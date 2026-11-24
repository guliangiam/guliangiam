package auth

import (
	"context"
	"log/slog"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	consentdomain "github.com/guliangiam/guliangiam/internal/domain/consents"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s *Service) GrantConsent(ctx context.Context, command GrantConsentCommand) (*ConsentResponse, error) {
	organization, user, application, scopes, err := s.resolveConsentContext(ctx, command.OrganizationSlug, command.Username, command.ApplicationID, command.GrantedScopes)
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now().UTC()
	var response *ConsentResponse
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// ADR-029 consent extension point: fail-closed gate before any
		// consent write. Deny renders 403.
		if gateErr := s.enforceActionsConsent(ctx, organization.ID, user.ID, user.Username, application.ID, scopes); gateErr != nil {
			return gateErr
		}

		existing, err := tx.Consents().GetByUserAndApplication(ctx, user.ID, application.ID)
		if err != nil {
			if typed, ok := err.(*apperrors.Error); !ok || typed.Code != apperrors.CodeNotFound {
				return err
			}
		}

		consent := existing
		if consent == nil {
			consentID, err := s.UUIDGenerator.NewString()
			if err != nil {
				return err
			}
			consent = &consentdomain.Consent{
				ID:               consentID,
				UserID:           user.ID,
				Username:         user.Username,
				OrganizationID:   organization.ID,
				OrganizationSlug: organization.Slug,
				ApplicationID:    application.ID,
				ApplicationSlug:  application.Slug,
				CreatedAt:        now,
				Revision:         1,
			}
		} else {
			consent.Revision++
		}

		consent.GrantedScopes = mergeScopes(consent.GrantedScopes, scopes)
		consent.UpdatedAt = now
		consent.RevokedAt = nil

		if err := tx.Consents().Upsert(ctx, consent); err != nil {
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
			"consent.granted",
			"consent",
			consent.ID,
			"consent",
			"consent.granted",
			map[string]any{},
		); err != nil {
			return err
		}
		response = &ConsentResponse{Status: "granted", Consent: toConsentDetail(consent)}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (s *Service) EvaluateConsent(ctx context.Context, command EvaluateConsentCommand) (*ConsentEvaluationResponse, error) {
	organization, user, application, err := s.resolveConsentActorContext(ctx, command.OrganizationSlug, command.Username, command.ApplicationID)
	if err != nil {
		return nil, err
	}

	requestedScopes := normalizeScopes(command.RequestedScopes)
	consentableScopes := filterConsentableScopes(requestedScopes, application.CustomScopes)
	grantedScopes, err := s.getActiveGrantedScopes(ctx, user.ID, application.ID)
	if err != nil {
		return nil, err
	}
	missingScopes := differenceScopes(consentableScopes, grantedScopes)

	return &ConsentEvaluationResponse{
		Status: "evaluated",
		Evaluation: ConsentEvaluationDetail{
			ApplicationID:     application.ID,
			ApplicationSlug:   application.Slug,
			RequestedScopes:   requestedScopes,
			ConsentableScopes: consentableScopes,
			GrantedScopes:     grantedScopes,
			MissingScopes:     missingScopes,
			Required:          len(application.CustomScopes) > 0 && len(missingScopes) > 0 && organization.ID != "",
		},
	}, nil
}

func (s *Service) RevokeConsent(ctx context.Context, command RevokeConsentCommand) (*ConsentResponse, error) {
	_, user, application, scopes, err := s.resolveConsentContext(ctx, command.OrganizationSlug, command.Username, command.ApplicationID, command.GrantedScopes)
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now().UTC()
	var response *ConsentResponse
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		consent, err := tx.Consents().GetByUserAndApplication(ctx, user.ID, application.ID)
		if err != nil {
			return err
		}
		if consent.RevokedAt != nil {
			return apperrors.NewNotFound("consent not found")
		}

		consent.GrantedScopes = subtractScopes(consent.GrantedScopes, scopes)
		consent.UpdatedAt = now
		consent.Revision++
		if len(consent.GrantedScopes) == 0 {
			consent.RevokedAt = &now
		}

		if err := tx.Consents().Upsert(ctx, consent); err != nil {
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
			"consent.granted",
			"consent",
			consent.ID,
			"consent",
			"consent.granted",
			map[string]any{},
		); err != nil {
			return err
		}
		// Withdrawing a scope has to actually withdraw the authority it granted.
		// Revoking only the consent row left the already-issued access and
		// refresh tokens working until they expired, so a user who revoked
		// access saw the third party keep using it (设计报告 14.17).
		//
		// The revocation runs inside the same transaction as the consent write:
		// either the consent is gone AND the tokens are dead, or neither moved.
		if err := s.revokeTokensForRevokedConsent(ctx, tx, user.ID, application.ID, now); err != nil {
			return err
		}

		// A session that outlives the consent would let the user silently
		// re-grant by revisiting the application; drop it too. Best effort: a
		// failure here must not undo the revocation that already happened.
		s.revokeSessionsForRevokedConsent(ctx, tx, user.ID, application, now, command.CurrentSessionID)

		response = &ConsentResponse{Status: "revoked", Consent: toConsentDetail(consent)}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}

// revokeTokensForRevokedConsent revokes every access token (and the refresh
// tokens paired with them) that the user holds for the application.
//
// It reuses the same repository call the logout fan-out uses, so there is one
// definition of "revoke this user's tokens for this application" rather than a
// second, subtly different SQL statement.
func (s *Service) revokeTokensForRevokedConsent(ctx context.Context, tx appuow.UnitOfWork, userID, applicationID string, now time.Time) error {
	affected, err := tx.Tokens().RevokeByQuery(ctx, tokendomain.RevokeQuery{
		UserID:        userID,
		ApplicationID: applicationID,
	}, now)
	if err != nil {
		return apperrors.WrapInternal(err, "revoke tokens for revoked consent")
	}

	slog.InfoContext(ctx, "consent revoked: tokens invalidated",
		"user_id", userID,
		"application_id", applicationID,
		"revoked_count", affected,
	)
	return nil
}

// revokeSessionsForRevokedConsent drops the user's active sessions for the
// application so the revoked consent cannot be bypassed by an existing session.
// It is best effort by design: the token revocation above is the security
// guarantee, and failing the whole request because a session row was already
// gone would report a successful revocation as an error.
func (s *Service) revokeSessionsForRevokedConsent(ctx context.Context, tx appuow.UnitOfWork, userID string, application *applicationdomain.Application, now time.Time, excludeSessionID string) {
	sessions, _, err := tx.Sessions().List(ctx, sessiondomain.ListQuery{
		UserID:        userID,
		ApplicationID: application.ID,
		ActiveOnly:    boolPtr(true),
		PageSize:      100,
	})
	if err != nil {
		slog.WarnContext(ctx, "consent revoked: could not list sessions", "error", err, "application_id", application.ID)
		return
	}

	for _, sess := range sessions {
		if excludeSessionID != "" && sess.ID == excludeSessionID {
			continue
		}
		if err := tx.Sessions().Revoke(ctx, sess.ID, now); err != nil {
			slog.WarnContext(ctx, "consent revoked: could not revoke session", "error", err, "session_id", sess.ID)
		}
	}
}

func (s *Service) resolveConsentContext(ctx context.Context, organizationSlug, username, applicationID string, grantedScopes []string) (*orgdomain.Organization, *userdomain.User, *applicationdomain.Application, []string, error) {
	organization, user, application, err := s.resolveConsentActorContext(ctx, organizationSlug, username, applicationID)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	scopes, err := validateGrantedConsentScopes(application, grantedScopes)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return organization, user, application, scopes, nil
}

func (s *Service) resolveConsentActorContext(ctx context.Context, organizationSlug, username, applicationID string) (*orgdomain.Organization, *userdomain.User, *applicationdomain.Application, error) {
	if strings.TrimSpace(organizationSlug) == "" || strings.TrimSpace(username) == "" {
		return nil, nil, nil, apperrors.NewUnauthorized("active session is required")
	}
	if strings.TrimSpace(applicationID) == "" {
		return nil, nil, nil, apperrors.NewInvalidArgument("applicationId is required")
	}

	organization, err := s.UOW.Organizations().GetBySlug(ctx, organizationSlug)
	if err != nil {
		return nil, nil, nil, apperrors.WrapInternalf(err, "lookup organization by slug %q", organizationSlug)
	}
	user, err := s.UOW.Users().GetByUsername(ctx, organization.ID, username)
	if err != nil {
		return nil, nil, nil, apperrors.WrapInternalf(err, "lookup user %q in organization %s", username, organization.ID)
	}
	application, err := resolveApplicationReference(ctx, s.UOW.Applications(), organization, applicationID)
	if err != nil {
		return nil, nil, nil, err
	}
	if application.OrganizationID != organization.ID {
		return nil, nil, nil, apperrors.NewInvalidArgument("application does not belong to organization")
	}

	return organization, user, application, nil
}

func normalizeScopes(scopes []string) []string {
	result := make([]string, 0, len(scopes))
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		trimmed := strings.TrimSpace(scope)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func mergeScopes(existing, added []string) []string {
	return normalizeScopes(append(append([]string(nil), existing...), added...))
}

func differenceScopes(left, right []string) []string {
	rightSet := make(map[string]struct{}, len(right))
	for _, scope := range right {
		rightSet[scope] = struct{}{}
	}
	result := make([]string, 0, len(left))
	for _, scope := range left {
		if _, ok := rightSet[scope]; ok {
			continue
		}
		result = append(result, scope)
	}
	return result
}

func subtractScopes(existing, revoked []string) []string {
	revokedSet := make(map[string]struct{}, len(revoked))
	for _, scope := range revoked {
		revokedSet[scope] = struct{}{}
	}
	remaining := make([]string, 0, len(existing))
	for _, scope := range existing {
		if _, ok := revokedSet[scope]; ok {
			continue
		}
		remaining = append(remaining, scope)
	}
	return remaining
}

func validateGrantedConsentScopes(application *applicationdomain.Application, scopes []string) ([]string, error) {
	normalized := normalizeScopes(scopes)
	if len(normalized) == 0 {
		return nil, apperrors.NewInvalidArgument("grantedScopes must not be empty")
	}
	if len(application.CustomScopes) == 0 {
		return nil, apperrors.NewInvalidArgument("application does not define custom scopes")
	}

	consentableScopes := filterConsentableScopes(normalized, application.CustomScopes)
	if len(consentableScopes) == 0 {
		return nil, apperrors.NewInvalidArgument("grantedScopes must include at least one application custom scope")
	}

	if invalidScopes := differenceScopes(normalized, consentableScopes); len(invalidScopes) > 0 {
		return nil, apperrors.NewWithMeta(apperrors.CodeInvalidArgument, "grantedScopes contains scopes not defined by application customScopes", map[string]any{"invalidScopes": invalidScopes})
	}

	return consentableScopes, nil
}

func filterConsentableScopes(requested []string, customScopes []applicationdomain.ApplicationCustomScope) []string {
	if len(requested) == 0 || len(customScopes) == 0 {
		return []string{}
	}

	customScopeSet := make(map[string]struct{}, len(customScopes))
	for _, scope := range customScopes {
		name := strings.TrimSpace(scope.Scope)
		if name == "" {
			continue
		}
		customScopeSet[name] = struct{}{}
	}

	consentableScopes := make([]string, 0, len(requested))
	for _, scope := range requested {
		if _, ok := customScopeSet[scope]; ok {
			consentableScopes = append(consentableScopes, scope)
		}
	}
	return consentableScopes
}

func (s *Service) getActiveGrantedScopes(ctx context.Context, userID, applicationID string) ([]string, error) {
	consent, err := s.UOW.Consents().GetByUserAndApplication(ctx, userID, applicationID)
	if err != nil {
		if typed, ok := err.(*apperrors.Error); ok && typed.Code == apperrors.CodeNotFound {
			return []string{}, nil
		}
		return nil, apperrors.WrapInternalf(err, "lookup consent for user %s application %s", userID, applicationID)
	}
	if consent.RevokedAt != nil {
		return []string{}, nil
	}
	return normalizeScopes(consent.GrantedScopes), nil
}

func toConsentDetail(consent *consentdomain.Consent) ConsentDetail {
	return ConsentDetail{
		ID:               consent.ID,
		UserID:           consent.UserID,
		Username:         consent.Username,
		OrganizationID:   consent.OrganizationID,
		OrganizationSlug: consent.OrganizationSlug,
		ApplicationID:    consent.ApplicationID,
		ApplicationSlug:  consent.ApplicationSlug,
		GrantedScopes:    append([]string(nil), consent.GrantedScopes...),
		CreatedAt:        consent.CreatedAt,
		UpdatedAt:        consent.UpdatedAt,
		RevokedAt:        consent.RevokedAt,
		Revision:         consent.Revision,
	}
}
