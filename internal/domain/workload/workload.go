package workload

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Pool struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Description    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

type FederationTrustStatus string

const (
	TrustStatusActive   FederationTrustStatus = "active"
	TrustStatusDisabled FederationTrustStatus = "disabled"
	TrustStatusDeleted  FederationTrustStatus = "deleted"
)

type FederationTrust struct {
	ID             string
	OrganizationID string
	PoolID         string
	Name           string
	Issuer         string
	JWKSURI        string
	Audience       []string
	Subjects       []string
	Status         FederationTrustStatus
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

type ServiceAccountStatus string

const (
	SAStatusActive   ServiceAccountStatus = "active"
	SAStatusDisabled ServiceAccountStatus = "disabled"
	SAStatusDeleted  ServiceAccountStatus = "deleted"
)

type ServiceAccount struct {
	ID                    string
	OrganizationID        string
	PoolID                string
	Name                  string
	Description           string
	ClientID              string
	ClientSecretHash      string
	Scopes                []string
	AccessTokenTTLSeconds int
	LastRotatedAt         *time.Time
	Status                ServiceAccountStatus
	CreatedAt             time.Time
	UpdatedAt             time.Time
	Revision              int64
}

type ListPoolsQuery struct {
	OrganizationID string
	Page, PageSize int
}
type ListTrustsQuery struct {
	OrganizationID, PoolID string
	Page, PageSize         int
}
type ListAccountsQuery struct {
	OrganizationID, PoolID string
	Page, PageSize         int
}

type CreatePoolCommand struct {
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	Description    string `json:"description"`
}

type CreateTrustCommand struct {
	OrganizationID string   `json:"organizationId"`
	PoolID         string   `json:"poolId"`
	Name           string   `json:"name"`
	Issuer         string   `json:"issuer"`
	JWKSURI        string   `json:"jwksUri"`
	Audience       []string `json:"audience"`
	Subjects       []string `json:"subjects"`
}

type CreateAccountCommand struct {
	OrganizationID        string   `json:"organizationId"`
	PoolID                string   `json:"poolId"`
	Name                  string   `json:"name"`
	Description           string   `json:"description"`
	ClientSecretPlain     string   `json:"-"`
	Scopes                []string `json:"scopes"`
	AccessTokenTTLSeconds int      `json:"accessTokenTtlSeconds"`
}

type SetTrustStatusCommand struct {
	OrganizationID string                `json:"organizationId"`
	PoolID         string                `json:"poolId"`
	TrustID        string                `json:"id"`
	Status         FederationTrustStatus `json:"status"`
}

type DeleteTrustCommand struct {
	OrganizationID string `json:"organizationId"`
	PoolID         string `json:"poolId"`
	TrustID        string `json:"id"`
}

type SetAccountStatusCommand struct {
	OrganizationID string               `json:"organizationId"`
	PoolID         string               `json:"poolId"`
	AccountID      string               `json:"id"`
	Status         ServiceAccountStatus `json:"status"`
	Revision       int64                `json:"revision"`
}

type RotateAccountSecretCommand struct {
	OrganizationID    string `json:"organizationId"`
	PoolID            string `json:"poolId"`
	AccountID         string `json:"id"`
	Revision          int64  `json:"revision"`
	ClientSecretPlain string `json:"-"`
}

type DeleteAccountCommand struct {
	OrganizationID string `json:"organizationId"`
	PoolID         string `json:"poolId"`
	AccountID      string `json:"id"`
	Revision       int64  `json:"revision"`
}

func ValidateHTTPSURI(value string) error {
	if value == "" || hasWhitespaceOrControl(value) {
		return fmt.Errorf("HTTPS URI must not be empty or contain whitespace/control characters")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("invalid HTTPS URI: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" || parsed.Hostname() == "" {
		return fmt.Errorf("URI must be an absolute HTTPS URI")
	}
	return nil
}
