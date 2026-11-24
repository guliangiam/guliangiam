package b2b

import (
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

// ── Request ─────────────────────────────────────────────────────────────────

type ClaimDomainRequest struct {
	OrganizationID string `json:"-"` // 从路径参数 /organizations/{orgId}/domains 注入
	Domain         string `json:"domain" validate:"required,dnsname"`
}

func (r ClaimDomainRequest) Validate() error {
	domain := strings.TrimSpace(r.Domain)
	if domain == "" {
		return apperrors.NewInvalidArgument("domain is required")
	}
	if len(domain) > 253 {
		return apperrors.NewInvalidArgument("domain exceeds maximum length")
	}
	return nil
}

type ListDomainsRequest struct {
	OrganizationID string `json:"-"`
	Page           int    `json:"-"`
	PageSize       int    `json:"-"`
	Status         string `json:"-"`
}

type DeleteDomainRequest struct {
	OrganizationID string `json:"-"`
	DomainID       string `json:"-"`
}

type VerifyDomainRequest struct {
	OrganizationID string `json:"-"`
	DomainID       string `json:"-"`
}

// ── Response ────────────────────────────────────────────────────────────────

type DomainResponse struct {
	ID                string     `json:"id"`
	OrganizationID    string     `json:"organizationId"`
	Domain            string     `json:"domain"`
	VerificationToken string     `json:"verificationToken"`
	Status            string     `json:"status"`
	VerifiedAt        *time.Time `json:"verifiedAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type ClaimDomainResponse struct {
	Domain DomainResponse `json:"domain"`
}

type ListDomainsResponse = presenter.ListResponse[DomainResponse]

type VerifyDomainResponse struct {
	Domain   DomainResponse `json:"domain"`
	Verified bool           `json:"verified"`
}
