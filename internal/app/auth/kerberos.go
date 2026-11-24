package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	kerberosdomain "github.com/guliangiam/guliangiam/internal/domain/kerberostickets"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// KerberosLoginCommand is the command for Kerberos login.
type KerberosLoginCommand struct {
	OrganizationSlug  string
	SPNEGOToken       string
	Realm             string
	Service           string
	ApplicationID     string
	PreviousSessionID string
	IPAddress         string
	UserAgent         string
	ResponseType      string
	RedirectURI       string
	State             string
	Nonce             string
	Scope             []string
}

// KerberosLoginResult is the result of a Kerberos login.
type KerberosLoginResult struct {
	LoginResult
	Username string `json:"username"`
	Realm    string `json:"realm"`
}

// KerberosLogin authenticates a user via Kerberos SPNEGO token.
func (s *Service) KerberosLogin(ctx context.Context, command KerberosLoginCommand) (*KerberosLoginResult, error) {
	if command.SPNEGOToken == "" {
		return nil, apperrors.NewInvalidArgument("SPNEGO token is required")
	}

	username, realm, err := s.parseSPNEGOPrincipal(command.SPNEGOToken)
	if err != nil {
		return nil, apperrors.NewUnauthorizedf("Kerberos SPNEGO validation failed: %v", err)
	}

	if command.Realm != "" {
		realm = strings.ToUpper(command.Realm)
	} else {
		realm = strings.ToUpper(realm)
	}

	var result *KerberosLoginResult
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		now := s.Clock.Now()

		organization, err := tx.Organizations().GetBySlug(ctx, command.OrganizationSlug)
		if err != nil {
			return apperrors.WrapInternal(err, "load organization")
		}
		if organization.DisableSignin {
			return apperrors.NewForbidden("organization has sign-in disabled")
		}

		var application *applicationdomain.Application
		if strings.TrimSpace(command.ApplicationID) != "" {
			resolved, err := resolveApplicationReference(ctx, tx.Applications(), organization, command.ApplicationID)
			if err != nil {
				return err
			}
			application = resolved
		}

		user, err := tx.Users().GetByUsername(ctx, organization.ID, username)
		if err != nil {
			slog.WarnContext(ctx, "Kerberos login: user not found",
				"username", username,
				"realm", realm,
				"organization_id", organization.ID,
			)
			return apperrors.NewUnauthorized("user not found for Kerberos principal")
		}

		if !isUserAllowedToLogin(user.Status, user.IsForbidden) {
			return apperrors.NewForbidden("user cannot sign in")
		}

		_ = resetLoginFailuresWithTx(ctx, tx, user.ID)

		sessionID, err := s.UUIDGenerator.NewString()
		if err != nil {
			return apperrors.NewInternal("generate session id")
		}

		effectiveTTL := sessionTTL
		if application != nil && !application.EnableSigninSession {
			effectiveTTL = shortSessionTTL
		}

		var appID, appSlug string
		if application != nil {
			appID = application.ID
			appSlug = application.Slug
		}

		sid, err := newSessionSid()
		if err != nil {
			return err
		}
		session := &sessiondomain.Session{
			ID:               sessionID,
			Sid:              sid,
			UserID:           user.ID,
			Username:         user.Username,
			OrganizationID:   organization.ID,
			OrganizationSlug: organization.Slug,
			ApplicationID:    appID,
			ApplicationSlug:  appSlug,
			SessionState:     statusActive,
			DeviceInfo:       map[string]any{paramLoginMethod: "kerberos", "realm": realm},
			IPAddress:        strings.TrimSpace(command.IPAddress),
			UserAgent:        strings.TrimSpace(command.UserAgent),
			ExpiresAt:        now.Add(effectiveTTL),
			CreatedAt:        now,
			LastSeenAt:       now,
			UpdatedAt:        now,
			Revision:         1,
		}
		if err := tx.Sessions().Create(ctx, session); err != nil {
			return err
		}

		if err := s.embedAuditOutboxPlain(
			tx,
			ctx,
			"",
			"",
			"",
			session.OrganizationID,
			session.UserID,
			"session.created",
			"session",
			session.ID,
			"session",
			"session.created",
			map[string]any{},
		); err != nil {
			return err
		}
		// S9: revoke old session carried by request (session fixation prevention)
		if strings.TrimSpace(command.PreviousSessionID) != "" {
			if err := revokePreviousSessionForRotation(ctx, tx.Sessions(), command.PreviousSessionID, now); err != nil {
				return err
			}
			if err := s.embedAuditOutboxPlain(
				tx,
				ctx,
				"",
				"",
				"",
				"",
				"",
				"session.revoked",
				"session",
				command.PreviousSessionID,
				"session",
				"session.revoked",
				map[string]any{},
			); err != nil {
				return err
			}
		}

		// 登录方式无关的会话策略与 signin 统计（排他登录 / 并发上限 / LastSignin*）。
		if err := s.applyLoginSessionPolicies(ctx, tx, user, application, organization.ID, session, now); err != nil {
			return err
		}
		var loginResponse *LoginResponse
		if application != nil {
			loginCommand := LoginCommand{
				OrganizationSlug: organization.Slug,
				Username:         user.Username,
				ApplicationID:    application.ID,
				ResponseType:     command.ResponseType,
				RedirectURI:      command.RedirectURI,
				State:            command.State,
				Nonce:            command.Nonce,
				Scope:            command.Scope,
				IPAddress:        command.IPAddress,
				UserAgent:        command.UserAgent,
			}
			loginResponse, err = s.handleLoggedIn(ctx, tx, loginCommand, organization, application, user, sid, now)
			if err != nil {
				return err
			}
		}

		result = &KerberosLoginResult{
			LoginResult: LoginResult{SessionID: sessionID},
			Username:    username,
			Realm:       realm,
		}
		if loginResponse != nil {
			result.Response = *loginResponse
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// validateSPNEGOToken is the function variable for SPNEGO token validation.
var validateSPNEGOToken = func(authHeader string) (username string, realm string, err error) {
	parts := strings.SplitN(authHeader, "@", 2)
	if len(parts) == 2 {
		return parts[0], strings.ToUpper(parts[1]), nil
	}
	if authHeader != "" {
		return authHeader, "", nil
	}
	return "", "", apperrors.NewInvalidArgument("invalid SPNEGO token format")
}

// WireKerberosAuthenticator wires the SPNEGO validation function.
func WireKerberosAuthenticator(validateFn func(authHeader string) (username string, realm string, err error)) {
	validateSPNEGOToken = validateFn
}

func (s *Service) parseSPNEGOPrincipal(token string) (realm, principal string, err error) {
	return validateSPNEGOToken(token)
}

// CreateKerberosTicket creates a Kerberos service ticket and persists it to the database.
func (s *Service) CreateKerberosTicket(ctx context.Context, userID, username, service, realm string, ttl time.Duration) (*kerberosdomain.Ticket, error) {
	now := s.Clock.Now()
	ticketID := fmt.Sprintf("KRBT-%d", now.UnixNano())
	ticketDBID, _ := platformuuid.V7Generator{}.NewString()

	ticket := &kerberosdomain.Ticket{
		ID:        ticketDBID,
		TicketID:  ticketID,
		UserID:    userID,
		Username:  username,
		Service:   service,
		Realm:     realm,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
	}

	if err := s.UOW.KerberosTickets().Create(ctx, ticket); err != nil {
		return nil, err
	}

	return ticket, nil
}

// ValidateKerberosTicket validates a Kerberos service ticket from the database.
func (s *Service) ValidateKerberosTicket(ctx context.Context, ticketID string) (*kerberosdomain.Ticket, error) {
	ticket, err := s.UOW.KerberosTickets().GetByTicketID(ctx, ticketID)
	if err != nil {
		return nil, apperrors.NewNotFoundf("kerberos ticket not found: %s", ticketID)
	}

	now := s.Clock.Now()
	if now.After(ticket.ExpiresAt) {
		_ = s.UOW.KerberosTickets().DeleteByTicketID(ctx, ticketID)
		return nil, apperrors.NewInvalidArgumentf("kerberos ticket expired: %s", ticketID)
	}

	return ticket, nil
}

// DeleteKerberosTicket removes a Kerberos ticket from the database.
func (s *Service) DeleteKerberosTicket(ctx context.Context, ticketID string) error {
	return s.UOW.KerberosTickets().DeleteByTicketID(ctx, ticketID)
}
