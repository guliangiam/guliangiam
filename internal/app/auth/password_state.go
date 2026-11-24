package auth

import (
	"context"
	"time"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Password change reasons carried in the gate error details. They are stable
// identifiers consumed by the frontend deep link and by audit, and they select
// the 403 error code: need_update_password → password_change_required,
// max_age_exceeded → password_expired.
const (
	PasswordChangeReasonNeedUpdatePassword = "need_update_password"
	PasswordChangeReasonMaxAgeExceeded     = "max_age_exceeded"
)

// PasswordState is the single decision returned by password state evaluation.
type PasswordState struct {
	MustChange bool
	Reason     string
}

// EvaluatePasswordState decides whether a credential must change its password.
// It is the only threshold source shared by login (transient response) and the
// request-time gate; callers must not re-derive max-age rules themselves.
//
// Precedence: an explicitly forced flag wins over max-age so the reason stays
// need_update_password when both conditions hold.
//
// Max-age is fail-closed: when the organization sets PasswordMaxAgeDays > 0 but
// PasswordUpdatedAt is NULL, the credential cannot prove it is within the age
// limit, so it is treated as max_age_exceeded. Users without a local password
// (has_password=false, e.g. SSO/LDAP) remain exempt forever, and
// PasswordMaxAgeDays=0 disables the max-age branch entirely.
func EvaluatePasswordState(cred userdomain.UserCredentials, policy orgdomain.PasswordPolicy, now time.Time) PasswordState {
	// SSO/LDAP-provisioned users without a local password are never gated.
	if !cred.HasPassword {
		return PasswordState{}
	}

	if cred.NeedUpdatePassword {
		return PasswordState{MustChange: true, Reason: PasswordChangeReasonNeedUpdatePassword}
	}

	if policy.PasswordMaxAgeDays > 0 {
		if cred.PasswordUpdatedAt == nil {
			return PasswordState{MustChange: true, Reason: PasswordChangeReasonMaxAgeExceeded}
		}
		// Max-age comparison lives in UserCredentials.IsPasswordExpired so there
		// is exactly one threshold implementation for both login and the gate.
		if cred.IsPasswordExpired(now, policy.PasswordMaxAgeDays) {
			return PasswordState{MustChange: true, Reason: PasswordChangeReasonMaxAgeExceeded}
		}
	}

	return PasswordState{}
}

// applyPasswordStateNotice decorates a successful login response with the
// password state without persisting anything. A forced flag is already stored
// in UserCredentials; max-age is reported through the PasswordExpired notice
// fields instead so the next requests can still produce password_expired.
func applyPasswordStateNotice(resp *LoginResponse, cred userdomain.UserCredentials, policy orgdomain.PasswordPolicy, state PasswordState) {
	if resp == nil {
		return
	}

	switch state.Reason {
	case PasswordChangeReasonNeedUpdatePassword:
		resp.NeedUpdatePassword = true
	case PasswordChangeReasonMaxAgeExceeded:
		resp.NeedUpdatePassword = false
		resp.PasswordExpired = true
		if cred.PasswordUpdatedAt != nil && policy.PasswordMaxAgeDays > 0 {
			expiresAt := cred.PasswordUpdatedAt.AddDate(0, 0, policy.PasswordMaxAgeDays)
			resp.PasswordExpiresAt = expiresAt.UTC().Format(time.RFC3339)
		}
	}
}

// PasswordStateQuery identifies the credential to evaluate for the gate.
type PasswordStateQuery struct {
	UserID         string
	OrganizationID string
}

// GetPasswordState loads the credential and the effective organization policy
// and evaluates them. It is used on cache misses only, never on every request.
func (s *Service) GetPasswordState(ctx context.Context, query PasswordStateQuery) (PasswordState, error) {
	if query.UserID == "" || query.OrganizationID == "" {
		return PasswordState{}, apperrors.NewInvalidArgument("userId and organizationId are required")
	}

	org, err := s.UOW.Organizations().GetByID(ctx, query.OrganizationID)
	if err != nil {
		return PasswordState{}, err
	}

	user, err := s.UOW.Users().GetByIDForOrganization(ctx, query.UserID, query.OrganizationID)
	if err != nil {
		return PasswordState{}, err
	}

	now := time.Now().UTC()
	if s.Clock != nil {
		now = s.Clock.Now()
	}

	return EvaluatePasswordState(user.Credentials, resolvePasswordPolicy(org), now), nil
}
