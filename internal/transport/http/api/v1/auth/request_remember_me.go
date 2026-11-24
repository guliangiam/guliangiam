package auth

import (
	"strings"
)

type RememberMeIssueRequest struct {
	ApplicationID string `json:"applicationId,omitempty"`
}

func (r RememberMeIssueRequest) Validate() error {
	return validateOptionalString("applicationId", r.ApplicationID)
}

type RememberMeExchangeRequest struct {
	Token string `json:"token,omitempty"`
}

func (r RememberMeExchangeRequest) Validate() error {
	return validateOptionalString("token", r.Token)
}

type RememberMeRevokeRequest struct {
	FamilyID string `json:"familyId,omitempty"`
}

func (r RememberMeRevokeRequest) Validate() error {
	return validateOptionalString("familyId", r.FamilyID)
}

func trimRememberMeToken(value string) string {
	return strings.TrimSpace(value)
}
