package users

import (
	"encoding/base64"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
)

type UserListItem struct {
	ID                  string     `json:"id"`
	OrganizationID      string     `json:"organizationId"`
	Username            string     `json:"username"`
	DisplayName         string     `json:"displayName"`
	AvatarURL           string     `json:"avatarUrl"`
	Email               string     `json:"email"`
	Phone               string     `json:"phone"`
	UserType            string     `json:"userType"`
	Status              string     `json:"status"`
	IsAdmin             bool       `json:"isAdmin"`
	IsForbidden         bool       `json:"isForbidden"`
	IsDeleted           bool       `json:"isDeleted"`
	IsVerified          bool       `json:"isVerified"`
	IsOnline            bool       `json:"isOnline"`
	CreatedIp           string     `json:"createdIp,omitempty"`
	Tag                 string     `json:"tag"`
	Affiliation         string     `json:"affiliation"`
	RealName            string     `json:"realName"`
	Score               int        `json:"score"`
	Balance             float64    `json:"balance"`
	BalanceCredit       float64    `json:"balanceCredit"`
	BalanceCurrency     string     `json:"balanceCurrency"`
	Karma               int        `json:"karma"`
	Ranking             int        `json:"ranking"`
	SignupApplicationID string     `json:"signupApplicationId"`
	RegisterType        string     `json:"registerType"`
	RegisterSource      string     `json:"registerSource"`
	LastSigninAt        *time.Time `json:"lastSigninAt,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	Revision            int64      `json:"revision"`
}

type UserDetail struct {
	UserListItem
	ExternalID              string                 `json:"externalId"`
	SignupApplicationID     string                 `json:"signupApplicationId"`
	RegisterType            string                 `json:"registerType"`
	RegisterSource          string                 `json:"registerSource"`
	PreferredMfaType        string                 `json:"preferredMfaType"`
	RecoveryCodes           []string               `json:"recoveryCodes"`
	MfaSetupSummary         []map[string]any       `json:"mfaSetupSummary"`
	Profile                 map[string]any         `json:"profile"`
	CredentialSummary       map[string]any         `json:"credentialSummary"`
	Identities              []map[string]any       `json:"identities"`
	MfaMethods              []map[string]any       `json:"mfaMethods"`
	WebauthnCredentialCount int                    `json:"webauthnCredentialCount"`
	RoleBindings            []map[string]any       `json:"roleBindings"`
	GroupMemberships        []map[string]any       `json:"groupMemberships"`
	ConsentSummary          map[string]any         `json:"consentSummary"`
	BalanceCredit           float64                `json:"balanceCredit"`
	BalanceCurrency         string                 `json:"balanceCurrency"`
	IpWhitelist             string                 `json:"ipWhitelist"`
	Addresses               []domain.Address       `json:"addresses"`
	ManagedAccounts         []ManagedAccountItem   `json:"managedAccounts"`
	FaceIds                 []domain.FaceId        `json:"faceIds"`
	Cart                    []domain.ProductInfo   `json:"cart"`
	MfaItems                []MfaItemEntry         `json:"mfaItems"`
	MfaAccounts             []MfaAccountItem       `json:"mfaAccounts"`
	ApplicationScopes       []domain.ConsentRecord `json:"applicationScopes"`
	Custom1                 string                 `json:"custom1"`
	Custom2                 string                 `json:"custom2"`
	Custom3                 string                 `json:"custom3"`
	Custom4                 string                 `json:"custom4"`
	Custom5                 string                 `json:"custom5"`
	Custom6                 string                 `json:"custom6"`
	Custom7                 string                 `json:"custom7"`
	Custom8                 string                 `json:"custom8"`
	Custom9                 string                 `json:"custom9"`
	Custom10                string                 `json:"custom10"`
	Revision                int64                  `json:"revision"`
}

type ManagedAccountItem struct {
	Application    string `json:"application"`
	Username       string `json:"username"`
	PasswordSet    bool   `json:"passwordSet"`
	PasswordMasked string `json:"passwordMasked,omitempty"`
	SigninURL      string `json:"signinUrl"`
}

type MfaItemEntry struct {
	MfaType      string `json:"mfaType"`
	SecretSet    bool   `json:"secretSet"`
	SecretMasked string `json:"secretMasked,omitempty"`
}

type MfaAccountItem struct {
	AccountName     string `json:"accountName"`
	Issuer          string `json:"issuer"`
	SecretKeySet    bool   `json:"secretKeySet"`
	SecretKeyMasked string `json:"secretKeyMasked,omitempty"`
	Origin          string `json:"origin"`
}

// WebAuthnCredentialItem is the DTO for a WebAuthn credential in list responses.
type WebAuthnCredentialItem struct {
	ID              string   `json:"id"`
	CredentialID    string   `json:"credentialId"`
	Name            string   `json:"name"`
	AttestationType string   `json:"attestationType"`
	Transport       []string `json:"transport"`
	SignCount       int64    `json:"signCount"`
	Status          string   `json:"status"`
	AAGUID          string   `json:"aaguid"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
}

func toWebAuthnCredentialItem(cred *webauthndomain.Credential) WebAuthnCredentialItem {
	return WebAuthnCredentialItem{
		ID:              cred.ID,
		CredentialID:    base64.RawURLEncoding.EncodeToString(cred.CredentialID),
		Name:            cred.Name,
		AttestationType: cred.AttestationType,
		Transport:       cred.Transport,
		SignCount:       cred.SignCount,
		Status:          string(cred.Status),
		AAGUID:          cred.AAGUID,
		CreatedAt:       cred.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       cred.UpdatedAt.Format(time.RFC3339),
	}
}
