package auth

import (
	"context"
	"log/slog"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	mfachallengedomain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s *Service) GetLoginChallenge(ctx context.Context, command LoginChallengeCommand) (*LoginChallengeResponse, error) {
	challengeID := strings.TrimSpace(command.ChallengeID)
	if challengeID == "" {
		return nil, apperrors.NewInvalidArgument("challengeId is required")
	}
	// The challenge read is also an object-level operation: AC-PERM-004 keeps
	// /login-challenges/* out of the anonymous surface, so the caller's session
	// principal must own the challenge. Foreign and missing challenges share
	// one rejection shape (no existence oracle).
	sessionUserID := strings.TrimSpace(command.SessionUserID)
	sessionOrgID := strings.TrimSpace(command.SessionOrganizationID)
	if sessionUserID == "" || sessionOrgID == "" {
		return nil, invalidChallengeError()
	}

	var response *LoginChallengeResponse
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		challenge, err := tx.MfaChallenges().GetByID(ctx, challengeID)
		if err != nil {
			if appErr := apperrors.FromError(err); appErr.Code == apperrors.CodeNotFound {
				return invalidChallengeError()
			}
			return err
		}
		if challenge.UserID != sessionUserID || challenge.OrganizationID != sessionOrgID {
			return invalidChallengeError()
		}
		response = toLoginChallengeResponse(challenge)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (s *Service) ApproveLoginChallenge(ctx context.Context, command LoginChallengeCommand) (*LoginChallengeResponse, error) {
	return s.updateLoginChallengeState(ctx, command, mfachallengedomain.StateApproved)
}

func (s *Service) RejectLoginChallenge(ctx context.Context, command LoginChallengeCommand) (*LoginChallengeResponse, error) {
	return s.updateLoginChallengeState(ctx, command, mfachallengedomain.StateRejected)
}

func (s *Service) updateLoginChallengeState(ctx context.Context, command LoginChallengeCommand, state string) (*LoginChallengeResponse, error) {
	challengeID := strings.TrimSpace(command.ChallengeID)
	if challengeID == "" {
		return nil, apperrors.NewInvalidArgument("challengeId is required")
	}

	// Approve/reject are object-level actions on a challenge minted for one
	// user. The principal must come from the authenticated session; a caller
	// without one is rejected like a missing challenge (never an implicit
	// approval). The rejection shape is deliberately identical for foreign and
	// non-existent challenges so the response cannot be used as an existence
	// oracle.
	sessionUserID := strings.TrimSpace(command.SessionUserID)
	sessionOrgID := strings.TrimSpace(command.SessionOrganizationID)
	if sessionUserID == "" || sessionOrgID == "" {
		return nil, invalidChallengeError()
	}

	var response *LoginChallengeResponse
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		now := s.Clock.Now()
		challenge, err := tx.MfaChallenges().GetByID(ctx, challengeID)
		if err != nil {
			// A repository not-found is mapped to the same rejection as a
			// foreign challenge so the response cannot be used to probe which
			// challenge IDs exist. Infrastructure failures stay internal.
			if appErr := apperrors.FromError(err); appErr.Code == apperrors.CodeNotFound {
				return invalidChallengeError()
			}
			return err
		}
		if challenge.UserID != sessionUserID || challenge.OrganizationID != sessionOrgID {
			return invalidChallengeError()
		}
		if !challenge.ExpiresAt.After(now) {
			if err := tx.MfaChallenges().Delete(ctx, challenge.ID); err != nil {
				slog.WarnContext(ctx, "login challenge: delete expired challenge failed",
					"challenge_id", challenge.ID, "error", err)
			}
			if err := s.embedAuditOutboxPlain(
				tx,
				ctx,
				"",
				"",
				"",
				challenge.OrganizationID,
				sessionUserID,
				"mfa.challenge.deleted",
				loginMethodMFA,
				challenge.ID,
				loginMethodMFA,
				"mfa.challenge.deleted",
				map[string]any{},
			); err != nil {
				return err
			}
			return invalidChallengeError()
		}
		if challenge.State == mfachallengedomain.StateRejected && state == mfachallengedomain.StateApproved {
			return apperrors.NewConflict("rejected challenge cannot be approved")
		}

		challenge.State = state
		challenge.UpdatedAt = now
		challenge.Revision++
		if state == mfachallengedomain.StateApproved {
			challenge.VerifiedAt = &now
		}
		if state == mfachallengedomain.StateRejected {
			challenge.VerifiedAt = nil
		}
		if err := tx.MfaChallenges().Update(ctx, challenge); err != nil {
			return err
		}

		if err := s.embedAuditOutboxPlain(
			tx,
			ctx,
			"",
			"",
			"",
			challenge.OrganizationID,
			sessionUserID,
			"mfa.challenge.updated",
			loginMethodMFA,
			challenge.ID,
			loginMethodMFA,
			"mfa.challenge.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		response = toLoginChallengeResponse(challenge)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}

func toLoginChallengeResponse(challenge *mfachallengedomain.Challenge) *LoginChallengeResponse {
	if challenge == nil {
		return nil
	}
	return &LoginChallengeResponse{
		ID:         challenge.ID,
		State:      challenge.State,
		MethodType: challenge.MethodType,
		ExpiresAt:  challenge.ExpiresAt,
		VerifiedAt: challenge.VerifiedAt,
	}
}
