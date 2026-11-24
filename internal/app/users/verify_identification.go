package users

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// IdentityVerificationRequest carries the claimed identity from the profile.
// It is forwarded to providers that compare the claimed document (Aliyun);
// hosted-flow providers (Jumio) only need the user reference.
type IdentityVerificationRequest struct {
	OrganizationID string
	UserID         string
	ReturnURL      string
	FullName       string
	IDType         string
	IDNumber       string
	ActorID        string
	// MetaInfo is the device environment JSON from the frontend SDK
	// (getMetaInfo). Aliyun fail-closes without the real client value.
	MetaInfo string
	// Country selects the domestic vs overseas route (CHN forces domestic).
	Country string
	// Action selects the face-verification semantics; empty defaults to
	// domestic real-name verification.
	Action string
}

// IdentityVerificationMeta carries the optional client-supplied context for a
// verification session. It is passed variadically so existing callers that
// only supply the claimed identity keep working.
type IdentityVerificationMeta struct {
	MetaInfo  string
	Country   string
	ReturnURL string
	Action    string
}

// IdentityVerificationTicket describes the initiated provider session.
type IdentityVerificationTicket struct {
	VerificationID  string
	Provider        string
	RedirectURL     string
	ExpiresAt       string
	AlreadyVerified bool
}

// IdentityVerificationGateway is implemented by the IDV application service
// and injected at the composition root.
type IdentityVerificationGateway interface {
	InitiateUserVerification(ctx context.Context, request IdentityVerificationRequest) (IdentityVerificationTicket, error)
}

// VerifyUserIdentification no longer marks a user verified on self-attestation.
// It validates the stored profile and starts a real provider session; the
// verified flag is only flipped when the provider returns an approval.
func (s *Service) VerifyUserIdentification(ctx context.Context, organizationID, userID string, meta ...IdentityVerificationMeta) (IdentityVerificationTicket, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return IdentityVerificationTicket{}, err
	}
	if s.IdentityVerification == nil {
		return IdentityVerificationTicket{}, apperrors.NewUnavailable("identity verification provider is not configured")
	}
	var m IdentityVerificationMeta
	if len(meta) > 0 {
		m = meta[0]
	}

	var user *userdomain.User
	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		loaded, err := tx.Users().GetByIDForOrganization(ctx, userID, organizationID)
		if err != nil {
			return err
		}
		user = loaded
		return nil
	}); err != nil {
		return IdentityVerificationTicket{}, err
	}

	if user.IsVerified {
		return IdentityVerificationTicket{AlreadyVerified: true}, nil
	}
	if user.Profile.RealName == "" || user.Profile.IDCardType == "" || user.Profile.IDCard == "" {
		return IdentityVerificationTicket{}, apperrors.NewInvalidArgument("real name, ID card type, and ID card number are required for verification")
	}

	return s.IdentityVerification.InitiateUserVerification(ctx, IdentityVerificationRequest{
		OrganizationID: organizationID,
		UserID:         userID,
		ReturnURL:      m.ReturnURL,
		FullName:       user.Profile.RealName,
		IDType:         user.Profile.IDCardType,
		IDNumber:       user.Profile.IDCard,
		MetaInfo:       m.MetaInfo,
		Country:        m.Country,
		Action:         m.Action,
	})
}
