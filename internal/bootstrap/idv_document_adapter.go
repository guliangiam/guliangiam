package bootstrap

import (
	"context"

	appidv "github.com/guliangiam/guliangiam/internal/app/idv"
	appusers "github.com/guliangiam/guliangiam/internal/app/users"
)

// idvDocumentRecognizer connects the users application service to the IDV OCR
// use case without making app/users import app/idv.
type idvDocumentRecognizer struct {
	idv *appidv.Service
}

func (a idvDocumentRecognizer) RecognizeIdentityDocument(ctx context.Context, request appusers.IdentityDocumentRecognitionRequest) (appusers.IdentityDocumentFields, error) {
	if a.idv == nil {
		return appusers.IdentityDocumentFields{}, nil
	}
	fields, err := a.idv.RecognizeDocument(ctx, appidv.RecognizeDocumentCommand{
		OrganizationID: request.OrganizationID,
		UserID:         request.UserID,
		DocumentType:   request.DocumentType,
		Side:           request.Side,
		MediaType:      request.MediaType,
		Data:           request.Data,
	})
	if err != nil {
		return appusers.IdentityDocumentFields{}, err
	}
	if fields == nil {
		return appusers.IdentityDocumentFields{}, nil
	}
	return appusers.IdentityDocumentFields{
		FullName:         fields.FullName,
		IDNumber:         fields.IDNumber,
		IDType:           fields.IDType,
		Gender:           fields.Gender,
		Birthday:         fields.Birthday,
		Nationality:      fields.Nationality,
		Address:          fields.Address,
		IssuingAuthority: fields.IssuingAuthority,
		ValidFrom:        fields.ValidFrom,
		ValidTo:          fields.ValidTo,
		Confidence:       fields.Confidence,
	}, nil
}
