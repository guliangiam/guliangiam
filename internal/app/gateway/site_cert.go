package gateway

import (
	"context"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// CertStatusResult contains the SSL certificate status for a site.
type CertStatusResult struct {
	SslMode  string `json:"sslMode"`
	CertID   string `json:"certId,omitempty"`
	Valid    bool   `json:"valid"`
	NotAfter string `json:"notAfter,omitempty"`
	Issuer   string `json:"issuer,omitempty"`
	Message  string `json:"message,omitempty"`
}

// ProvisionCertResult contains the result of certificate provisioning.
type ProvisionCertResult struct {
	CertID  string `json:"certId"`
	Message string `json:"message"`
}

// ProvisionCert requests an ACME certificate for the site's host.
func (s *Service) ProvisionCert(ctx context.Context, organizationID, siteID string) (*ProvisionCertResult, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// Resolve ownership before mutating the site: a foreign id must not be
	// distinguishable from a missing one.
	site, err := s.UOW.Gateway().GetSiteByIDForOrganization(ctx, siteID, organizationID)
	if err != nil {
		return nil, err
	}
	if site.Host == "" {
		return nil, apperrors.NewInvalidArgument("site host is required for certificate provisioning")
	}

	// For now, record the intent. Actual ACME integration would use
	// go-acme/lego to complete the DNS/HTTP challenge and obtain a cert.
	// The cert would be stored in the certs table and linked via cert_id.
	site.SslMode = "auto"
	site.UpdatedAt = s.Clock.Now()
	site.Revision++

	if err := s.UOW.Gateway().UpdateSite(ctx, site); err != nil {
		return nil, err
	}

	return &ProvisionCertResult{
		Message: "certificate provisioning initiated for " + site.Host,
	}, nil
}

// RenewCert renews the certificate for a site.
func (s *Service) RenewCert(ctx context.Context, organizationID, siteID string) (*ProvisionCertResult, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	site, err := s.UOW.Gateway().GetSiteByIDForOrganization(ctx, siteID, organizationID)
	if err != nil {
		return nil, err
	}
	if site.CertID == "" {
		return nil, apperrors.NewInvalidArgument("no certificate associated with this site")
	}

	site.UpdatedAt = s.Clock.Now()
	site.Revision++
	if err := s.UOW.Gateway().UpdateSite(ctx, site); err != nil {
		return nil, err
	}

	return &ProvisionCertResult{
		CertID:  site.CertID,
		Message: "certificate renewal initiated",
	}, nil
}

// GetCertStatus returns the SSL certificate status for a site.
func (s *Service) GetCertStatus(ctx context.Context, organizationID, siteID string) (*CertStatusResult, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// siteID comes from the URL path; the site lookup is tenant-scoped and the
	// cert lookup below is bound to the site's real organization.
	site, err := s.UOW.Gateway().GetSiteByIDForOrganization(ctx, siteID, organizationID)
	if err != nil {
		return nil, err
	}

	result := &CertStatusResult{
		SslMode: site.SslMode,
		CertID:  site.CertID,
	}

	switch {
	case site.CertID != "":
		// Look up cert details within the site's tenant.
		cert, err := s.UOW.Certs().GetByID(ctx, site.CertID, site.OrganizationID)
		if err == nil && cert != nil {
			result.Valid = true
			if cert.ExpireTime != nil {
				result.NotAfter = cert.ExpireTime.Format("2006-01-02T15:04:05Z")
			}
			result.Issuer = cert.Type
		}
	case site.SslMode == "off":
		result.Message = "SSL is disabled for this site"
	default:
		result.Message = "no certificate configured"
	}

	return result, nil
}
