package auth

import (
	"context"
	"log/slog"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	mfachallengedomain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/phonefmt"
)

func (s *Service) resolveLoginContext(ctx context.Context, tx appuow.UnitOfWork, command LoginCommand, now time.Time) (*orgdomain.Organization, *applicationdomain.Application, *userdomain.User, *mfachallengedomain.Challenge, error) {
	challengeID := strings.TrimSpace(command.ChallengeID)
	if challengeID == "" {
		organization, err := tx.Organizations().GetBySlug(ctx, command.OrganizationSlug)
		if err != nil {
			return nil, nil, nil, nil, err
		}

		var application *applicationdomain.Application
		if strings.TrimSpace(command.ApplicationID) == "" && strings.TrimSpace(command.ResponseType) == "" {
			var source applicationReferenceSource
			application, source, err = resolveApplicationReferenceWithSource(ctx, tx.Applications(), organization, organization.Slug)
			if err == nil {
				slog.InfoContext(ctx, "default application resolved",
					"organization_id", organization.ID, "application_id", application.ID, "source", string(source))
			}
		} else {
			application, err = resolveApplicationReference(ctx, tx.Applications(), organization, command.ApplicationID)
		}
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if application.OrganizationID != organization.ID {
			return nil, nil, nil, nil, apperrors.NewInvalidArgument("application does not belong to organization")
		}

		user, err := tx.Users().GetByLoginIdentifier(ctx, organization.ID, command.Username)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if strings.TrimSpace(command.Password) != "" {
			if err := verifyPassword(command.Password, user.Credentials.PasswordAlgo, user.Credentials.PasswordHash); err != nil {
				return nil, nil, nil, nil, err
			}
		} else if err := verifyLoginOtp(ctx, organization.ID, user, command); err != nil {
			return nil, nil, nil, nil, err
		}

		return organization, application, user, nil, nil
	}

	challenge, err := tx.MfaChallenges().GetByID(ctx, challengeID)
	if err != nil {
		return nil, nil, nil, nil, invalidChallengeError()
	}
	if !challenge.ExpiresAt.After(now) {
		if delErr := tx.MfaChallenges().Delete(ctx, challenge.ID); delErr != nil {
			slog.WarnContext(ctx, "failed to delete expired MFA challenge", "challenge_id", challenge.ID, "error", delErr)
		}
		return nil, nil, nil, nil, invalidChallengeError()
	}
	if challenge.State == mfachallengedomain.StateRejected {
		return nil, nil, nil, nil, invalidChallengeError()
	}

	organization, err := tx.Organizations().GetByID(ctx, challenge.OrganizationID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	application, err := tx.Applications().GetByID(ctx, challenge.ApplicationID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	user, err := tx.Users().GetByID(ctx, challenge.UserID)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	if command.OrganizationSlug != "" && command.OrganizationSlug != organization.Slug {
		return nil, nil, nil, nil, apperrors.NewInvalidArgument("challenge does not belong to organizationSlug")
	}
	if strings.TrimSpace(command.ApplicationID) != "" {
		// The challenge carries the UUID it was minted with, but the client may
		// still spell the same application as a slug or name.
		referencedApplication, refErr := resolveApplicationReference(ctx, tx.Applications(), organization, command.ApplicationID)
		if refErr != nil {
			return nil, nil, nil, nil, refErr
		}
		if referencedApplication.ID != application.ID {
			return nil, nil, nil, nil, apperrors.NewInvalidArgument("challenge does not belong to applicationId")
		}
	}
	if !identifierMatchesUser(user, command.Username) {
		return nil, nil, nil, nil, apperrors.NewInvalidArgument("challenge does not belong to username")
	}

	return organization, application, user, challenge, nil
}

// identifierMatchesUser reports whether the identifier supplied with an MFA
// challenge belongs to the challenged user. Password login accepts a username,
// an email or a phone, so the challenge follow-up must accept the same forms;
// otherwise a challenge minted for an email login would be rejected on the
// second step. An empty identifier keeps the historical behaviour of "no
// binding check".
func identifierMatchesUser(user *userdomain.User, identifier string) bool {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return true
	}
	if user == nil {
		return false
	}
	if identifier == user.Username {
		return true
	}
	if user.Profile.Email != "" && strings.EqualFold(identifier, user.Profile.Email) {
		return true
	}
	if user.Profile.Phone != "" && identifier == user.Profile.Phone {
		return true
	}
	return false
}

// loginOtpRecipient binds the login identifier to the user's own contact
// channel. The identifier doubles as the recipient the code was sent to, so a
// code harvested for another recipient can never authenticate this account.
func loginOtpRecipient(user *userdomain.User, identifier, verificationType string) (string, string, error) {
	identifier = strings.TrimSpace(identifier)
	if user == nil || identifier == "" {
		return "", "", apperrors.NewInvalidArgument("login verification recipient is invalid")
	}
	tableType, ok := loginVerificationTableType(verificationType)
	if !ok {
		return "", "", apperrors.NewInvalidArgument("login verification type is invalid")
	}
	switch tableType {
	case verificationTypeEmail:
		if user.Profile.Email == "" || !strings.EqualFold(identifier, user.Profile.Email) {
			return "", "", apperrors.NewInvalidArgument("login verification recipient is invalid")
		}
	case verificationTypePhone:
		if !phoneMatchesUser(identifier, user.Profile.Phone, user.Profile.CountryCode) {
			return "", "", apperrors.NewInvalidArgument("login verification recipient is invalid")
		}
	}
	return tableType, identifier, nil
}

// phoneMatchesUser accepts the same phone spellings as password login: exact
// match first, then an E.164 comparison using the profile country code hint.
func phoneMatchesUser(identifier, profilePhone, countryCode string) bool {
	profilePhone = strings.TrimSpace(profilePhone)
	if profilePhone == "" {
		return false
	}
	if identifier == profilePhone {
		return true
	}
	normalizedIdentifier, identifierErr := phonefmt.ToE164(identifier, countryCode)
	normalizedProfile, profileErr := phonefmt.ToE164(profilePhone, countryCode)
	return identifierErr == nil && profileErr == nil && normalizedIdentifier == normalizedProfile
}

// verifyLoginOtp consumes the one-time code for the resolved user. Every
// credential-level rejection collapses into the generic login failure so an
// attacker cannot distinguish a wrong code from an expired, replayed or
// cross-tenant code. Rate limiting and infrastructure failures stay visible.
func verifyLoginOtp(ctx context.Context, organizationID string, user *userdomain.User, command LoginCommand) error {
	tableType, recipient, err := loginOtpRecipient(user, command.Username, command.VerificationType)
	if err != nil {
		return genericLoginFailureError()
	}
	if err := consumeLoginVerificationCode(ctx, organizationID, tableType, recipient,
		strings.TrimSpace(command.VerificationCode), command.IPAddress); err != nil {
		if typed, ok := err.(*apperrors.Error); ok {
			switch typed.Code {
			case apperrors.CodeTooManyRequests, apperrors.CodeUnimplemented, apperrors.CodeInternal:
				return err
			}
		}
		return genericLoginFailureError()
	}
	return nil
}

func rejectServiceAccountPasswordLogin(ctx context.Context, tx appuow.UnitOfWork, organizationID, username string) error {
	username = strings.TrimSpace(username)
	if username == "" || tx.Workload() == nil {
		return nil
	}

	// Login already runs in its narrowly scoped anonymous system transaction.
	// The client_id lookup uses the globally unique credential; the name lookup
	// is constrained to the organization resolved from the login request.
	account, err := tx.Workload().GetAccountByClientID(ctx, username)
	if err == nil && account != nil {
		return apperrors.NewUnauthorized("invalid username or password")
	}
	if err != nil && !isWorkloadNotFound(err) {
		return apperrors.WrapInternal(err, "lookup service account by client_id")
	}

	account, err = tx.Workload().GetAccountByName(ctx, organizationID, username)
	if err == nil && account != nil {
		return apperrors.NewUnauthorized("invalid username or password")
	}
	if err != nil && !isWorkloadNotFound(err) {
		return apperrors.WrapInternal(err, "lookup service account by name")
	}
	return nil
}

func (s *Service) createLoginChallenge(ctx context.Context, tx appuow.UnitOfWork, organizationID, userID, applicationID string, now time.Time) (*mfachallengedomain.Challenge, error) {
	challengeID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return nil, apperrors.NewInternal("generate challenge id")
	}

	challenge := &mfachallengedomain.Challenge{
		ID:             challengeID,
		OrganizationID: organizationID,
		UserID:         userID,
		ApplicationID:  applicationID,
		State:          mfachallengedomain.StatePending,
		ExpiresAt:      now.Add(mfaChallengeTTL),
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}
	if err := tx.MfaChallenges().Create(ctx, challenge); err != nil {
		return nil, err
	}

	return challenge, nil
}
