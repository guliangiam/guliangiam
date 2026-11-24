package bootstrap

import (
	"context"

	appidv "github.com/guliangiam/guliangiam/internal/app/idv"
	appusers "github.com/guliangiam/guliangiam/internal/app/users"
)

// idvUserAdapter connects the users application service to the IDV service
// without making app/users import app/idv (or vice versa).
type idvUserAdapter struct {
	idv *appidv.Service
}

func (a idvUserAdapter) InitiateUserVerification(ctx context.Context, request appusers.IdentityVerificationRequest) (appusers.IdentityVerificationTicket, error) {
	if a.idv == nil {
		return appusers.IdentityVerificationTicket{}, nil
	}
	userID := request.UserID
	outcome, err := a.idv.Initiate(ctx, appidv.InitiateCommand{
		OrganizationID: request.OrganizationID,
		UserID:         &userID,
		ReturnURL:      request.ReturnURL,
		IDType:         request.IDType,
		FullName:       request.FullName,
		IDNumber:       request.IDNumber,
		ActorID:        request.ActorID,
		MetaInfo:       request.MetaInfo,
		Country:        request.Country,
		Action:         request.Action,
	})
	if err != nil {
		return appusers.IdentityVerificationTicket{}, err
	}
	return appusers.IdentityVerificationTicket{
		VerificationID: outcome.VerificationID,
		Provider:       outcome.Provider,
		RedirectURL:    outcome.RedirectURL,
		ExpiresAt:      outcome.ExpiresAt,
	}, nil
}
