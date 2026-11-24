package authorizationcodes

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/authorizationcodes"
)

func toAuthorizationCode(row authorizationCodeRow) *domain.AuthorizationCode {
	// Code is intentionally left empty: the plaintext credential exists only in
	// the redirect response built at creation time, never in the database or in a
	// redeemed row.
	return &domain.AuthorizationCode{
		ID:               row.ID,
		UserID:           row.UserID,
		Username:         row.Username,
		OrganizationID:   row.OrganizationID,
		OrganizationSlug: row.OrganizationSlug,
		ApplicationID:    row.ApplicationID,
		ApplicationSlug:  row.ApplicationSlug,
		RedirectURI:      row.RedirectURI,
		Scopes:           row.Scopes,
		State:            row.State,
		Nonce:            row.Nonce,
		CodeChallenge:    row.CodeChallenge,
		ChallengeMethod:  row.ChallengeMethod,
		Resource:         row.Resource,
		Sid:              row.Sid,
		AuthTime:         row.AuthTime,
		ExpiresAt:        row.ExpiresAt,
		UsedAt:           row.UsedAt,
		CreatedAt:        row.CreatedAt,
	}
}
