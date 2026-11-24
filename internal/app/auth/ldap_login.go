package auth

import (
	"context"
	stderrors "errors"
	"log/slog"
	"net"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	ldapdomain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	tenantctx "github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// LdapLoginCommand authenticates a user against an external LDAP directory.
type LdapLoginCommand struct {
	OrganizationSlug  string
	Username          string
	Password          string
	LdapID            string
	PreviousSessionID string // LDAP config ID; if empty, auto-detect from organization
	ApplicationID     string
	IPAddress         string
	UserAgent         string
	// CaptchaToken/CaptchaID carry the human-check response, enforced through
	// EnforceCaptcha once the organization is resolved (same gate as password
	// login; without it the LDAP channel would bypass the org captcha policy).
	CaptchaToken string
	CaptchaID    string
	// HandleLoggedIn parameters
	ResponseType string
	RedirectURI  string
	State        string
	Nonce        string
	Scope        []string
}

// LdapLoginResult contains the result of an LDAP login operation.
type LdapLoginResult struct {
	LoginResult
}

// ldapBindAndSearchFunc is the function variable that performs LDAP bind + search.
// It is wired to the actual LDAP client implementation in the bootstrap layer.
var ldapBindAndSearchFunc ldapBindAndSearchFuncType = defaultLdapBindAndSearch

type ldapBindAndSearchFuncType func(ctx context.Context, host string, port int, bindDN, bindPassword, baseDN string, useTLS bool, username, password, filter, attributeName string) (*LdapUserInfo, error)

// LdapUserInfo contains user info returned from an LDAP directory.
type LdapUserInfo struct {
	UID         string
	CommonName  string
	Email       string
	Phone       string
	Mobile      string
	Department  string
	Title       string
	EmployeeNum string
	DN          string
}

func defaultLdapBindAndSearch(ctx context.Context, host string, port int, bindDN, bindPassword, baseDN string, useTLS bool, username, password, filter, attributeName string) (*LdapUserInfo, error) {
	return nil, apperrors.NewInternal("LDAP login not configured: ldapBindAndSearchFunc not wired")
}

// SetLdapLoginAdapter wires the LDAP login adapter function.
func SetLdapLoginAdapter(fn ldapBindAndSearchFuncType) {
	if fn != nil {
		ldapBindAndSearchFunc = fn
	}
}

// LoginWithLDAP authenticates a user against an external LDAP directory.
// Flow: find LDAP config → bind + search → match/create local user → create session → handleLoggedIn.
func (s *Service) LoginWithLDAP(ctx context.Context, command LdapLoginCommand) (*LdapLoginResult, error) {
	if strings.TrimSpace(command.OrganizationSlug) == "" {
		return nil, apperrors.NewInvalidArgument("organizationSlug is required")
	}
	if strings.TrimSpace(command.Username) == "" {
		return nil, apperrors.NewInvalidArgument("username is required")
	}
	if strings.TrimSpace(command.Password) == "" {
		return nil, apperrors.NewInvalidArgument("password is required")
	}

	var result *LdapLoginResult
	// ADR-029 post_registration 在事务提交后触发（fail-open）。
	var registeredUser *userdomain.User
	// LDAP 登录发生在任何会话作用域之前：组织/应用/LDAP 配置都受 RLS 保护，
	// 必须像密码登录（login_flow.go）一样在系统作用域事务内解析（DEF-36）。
	err := s.UOW.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		now := s.Clock.Now()

		// 1. Resolve organization.
		organization, err := tx.Organizations().GetBySlug(ctx, command.OrganizationSlug)
		if err != nil {
			return apperrors.WrapInternal(err, "load organization")
		}
		// Human-check gate before any directory bind (anti-bot, mirrors Login/Signup).
		if err := s.EnforceCaptcha(ctx, organization.ID, command.CaptchaToken, command.CaptchaID, command.IPAddress); err != nil {
			return err
		}
		if organization.DisableSignin {
			return apperrors.NewForbidden("organization has sign-in disabled")
		}

		// 2. Resolve application.
		application, err := resolveApplicationReference(ctx, tx.Applications(), organization, command.ApplicationID)
		if err != nil {
			return err
		}
		if application.DisableSignin {
			return apperrors.NewForbidden("application has sign-in disabled")
		}

		// 3. Find the LDAP configuration.
		ldapConfig, err := s.resolveLdapConfig(ctx, tx, organization.ID, command.LdapID)
		if err != nil {
			return err
		}
		if !ldapConfig.IsEnabled {
			return apperrors.NewForbidden("LDAP provider is not enabled")
		}

		// 4. Authenticate against the LDAP directory.
		ldapUser, err := ldapBindAndSearchFunc(
			ctx,
			ldapConfig.Host,
			ldapConfig.Port,
			ldapConfig.BindDN,
			ldapConfig.BindPassword,
			ldapConfig.BaseDN,
			ldapConfig.EnableSsl,
			command.Username,
			command.Password,
			ldapConfig.Filter,
			ldapConfig.AttributeName,
		)
		if err != nil {
			slog.WarnContext(ctx, "LDAP authentication failed",
				"ldap_id", ldapConfig.ID,
				"username", command.Username,
				"error", err,
			)
			return ldapBindError(err)
		}

		if ldapUser == nil || strings.TrimSpace(ldapUser.UID) == "" {
			return apperrors.NewUnauthorized("LDAP user not found")
		}

		// 5. Match local user or auto-register.
		var user *userdomain.User

		// Try matching by LDAP identity (provider_id = ldap config ID, subject = ldapUser.UID).
		user, err = tx.Users().GetByProviderIdentity(ctx, ldapConfig.ID, ldapUser.UID)
		if err != nil {
			// Try matching by username in the organization.
			user, err = tx.Users().GetByUsername(ctx, organization.ID, command.Username)
			if err != nil {
				user = nil
			}
		}

		if user == nil {
			// Auto-register if the organization allows it.
			if organization.DisableSignup {
				return apperrors.NewForbidden("organization does not allow sign-up")
			}

			user, err = s.autoRegisterLdapUser(ctx, tx, organization, ldapConfig, ldapUser, now)
			if err != nil {
				return apperrors.WrapInternal(err, "auto-register LDAP user")
			}
			registeredUser = user
		} else {
			// Update the LDAP identity link if the user doesn't have one.
			if err := s.ensureLdapIdentity(ctx, tx, user, ldapConfig, ldapUser, now); err != nil {
				slog.WarnContext(ctx, "failed to link LDAP identity", "error", err, "user_id", user.ID)
			}
		}

		// 6. Check if user is allowed to login.
		if !isUserAllowedToLogin(user.Status, user.IsForbidden) {
			return apperrors.NewForbidden("user cannot sign in")
		}

		// 7. Reset login failures on successful authentication.
		_ = resetLoginFailuresWithTx(ctx, tx, user.ID)

		// ADR-029 pre_login (ldap channel): fail-closed policy gate before
		// the session is created. LDAP 凭据不进入 action input。
		if gateErr := s.enforceActionsPreLogin(ctx, loginMethodLDAP, organization.ID, user.ID, user.Username,
			application.ID, command.IPAddress, command.UserAgent); gateErr != nil {
			return gateErr
		}

		// 8. Create session.
		sessionID, err := s.UUIDGenerator.NewString()
		if err != nil {
			return apperrors.NewInternal("generate session id")
		}

		effectiveTTL := sessionTTL
		if !application.EnableSigninSession {
			effectiveTTL = shortSessionTTL
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
			ApplicationID:    application.ID,
			ApplicationSlug:  application.Slug,
			SessionState:     statusActive,
			DeviceInfo:       map[string]any{paramLoginMethod: loginMethodLDAP, "ldapId": ldapConfig.ID},
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
		// 9. Generate login response.
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

		loginResponse, err := s.handleLoggedIn(ctx, tx, loginCommand, organization, application, user, sid, now)
		if err != nil {
			return err
		}

		result = &LdapLoginResult{
			LoginResult: LoginResult{SessionID: sessionID, Response: *loginResponse},
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if registeredUser != nil {
		s.dispatchPostRegistration(ctx, registeredUser.OrganizationID, registeredUser.ID, registeredUser.Username,
			registeredUser.SignupApplicationID, "LDAP Auto Register")
	}

	return result, nil
}

// resolveLdapConfig finds the LDAP configuration for login.
func (s *Service) resolveLdapConfig(ctx context.Context, tx appuow.UnitOfWork, orgID, ldapID string) (*ldapdomain.Ldap, error) {
	if ldapID != "" {
		ldap, err := tx.Ldaps().GetByID(ctx, ldapID)
		if err != nil {
			return nil, apperrors.WrapInternal(err, "load LDAP config")
		}
		if ldap.OrganizationID != orgID {
			return nil, apperrors.NewInvalidArgument("LDAP config does not belong to organization")
		}
		return ldap, nil
	}

	// Auto-detect: find the first enabled LDAP config for this organization.
	ldaps, _, err := tx.Ldaps().List(ctx, ldapdomain.ListQuery{
		OrganizationID: orgID,
		PageSize:       10,
		IsEnabled:      boolPtr(true),
	})
	if err != nil {
		return nil, apperrors.WrapInternal(err, "list LDAP configs")
	}
	if len(ldaps) == 0 {
		return nil, apperrors.NewInvalidArgument("no enabled LDAP provider found for organization")
	}

	return ldaps[0], nil
}

// autoRegisterLdapUser creates a new user from LDAP info and links the identity.
func (s *Service) autoRegisterLdapUser(ctx context.Context, tx appuow.UnitOfWork, org *orgdomain.Organization, ldapConfig *ldapdomain.Ldap, ldapUser *LdapUserInfo, now time.Time) (*userdomain.User, error) {
	userID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return nil, apperrors.NewInternal("generate user id")
	}

	username := ldapUser.UID
	if username == "" {
		username = "ldap_user"
	}

	identityID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return nil, apperrors.NewInternal("generate identity id")
	}

	user := &userdomain.User{
		ID:             userID,
		OrganizationID: org.ID,
		Username:       username,
		UserType:       userTypeNormal,
		Status:         statusActive,
		IsAdmin:        false,
		IsForbidden:    false,
		Profile: userdomain.UserProfile{
			DisplayName: firstNonEmpty(ldapUser.CommonName, username),
			Email:       ldapUser.Email,
			Phone:       firstNonEmpty(ldapUser.Mobile, ldapUser.Phone),
		},
		CreatedAt: now,
		UpdatedAt: now,
		Revision:  1,
		Identities: []userdomain.UserIdentity{
			{
				ID:              identityID,
				UserID:          userID,
				ProviderID:      ldapConfig.ID,
				IdentityType:    loginMethodLDAP,
				Subject:         ldapUser.UID,
				ProviderAccount: firstNonEmpty(ldapUser.CommonName, username),
				ProfileSnapshot: map[string]any{
					"dn":          ldapUser.DN,
					contactEmail:  ldapUser.Email,
					"department":  ldapUser.Department,
					"title":       ldapUser.Title,
					"employeeNum": ldapUser.EmployeeNum,
				},
				LinkedAt: now,
			},
		},
	}

	if err := tx.Users().Create(ctx, user); err != nil {
		return nil, apperrors.WrapInternal(err, "create user")
	}

	return user, nil
}

// ensureLdapIdentity links an LDAP identity to an existing user if not already linked.
func (s *Service) ensureLdapIdentity(ctx context.Context, tx appuow.UnitOfWork, user *userdomain.User, ldapConfig *ldapdomain.Ldap, ldapUser *LdapUserInfo, now time.Time) error {
	for i := range user.Identities {
		identity := &user.Identities[i]
		if identity.ProviderID == ldapConfig.ID && identity.Subject == ldapUser.UID {
			return nil // Already linked
		}
	}

	identityID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return err
	}

	newIdentity := userdomain.UserIdentity{
		ID:              identityID,
		UserID:          user.ID,
		ProviderID:      ldapConfig.ID,
		IdentityType:    loginMethodLDAP,
		Subject:         ldapUser.UID,
		ProviderAccount: firstNonEmpty(ldapUser.CommonName, user.Username),
		ProfileSnapshot: map[string]any{
			"dn":          ldapUser.DN,
			contactEmail:  ldapUser.Email,
			"department":  ldapUser.Department,
			"title":       ldapUser.Title,
			"employeeNum": ldapUser.EmployeeNum,
		},
		LinkedAt: now,
	}

	return tx.Users().CreateIdentity(ctx, &newIdentity)
}

// ldapBindError 区分「目录不可达/超时」（依赖类 → 503）与「凭证错误」（401）。
// 旧实现把所有 bind 错误压成 401，导致 AC-PROTO-003 的 503 语义永远无法出现（DEF-37）。
func ldapBindError(err error) error {
	var netErr net.Error
	var opErr *net.OpError
	if stderrors.As(err, &netErr) || stderrors.As(err, &opErr) ||
		stderrors.Is(err, context.DeadlineExceeded) || stderrors.Is(err, context.Canceled) {
		return apperrors.NewUnavailable("LDAP directory is unavailable")
	}
	return apperrors.NewUnauthorized("LDAP authentication failed")
}

var _ = ldapdomain.Ldap{}
