package gateway

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// HealthCheckResult contains the result of a health check.
type HealthCheckResult struct {
	Status    string    `json:"status"` // healthy, unhealthy, unknown
	CheckedAt time.Time `json:"checkedAt"`
	Latency   string    `json:"latency,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// CheckHealth performs an HTTP health check against the site's host.
func (s *Service) CheckHealth(ctx context.Context, organizationID, siteID string) (*HealthCheckResult, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// The tenant decision must precede the outbound probe: a foreign site id
	// must not be able to make the server touch an arbitrary host.
	site, err := s.UOW.Gateway().GetSiteByIDForOrganization(ctx, siteID, organizationID)
	if err != nil {
		return nil, err
	}
	if site.Host == "" {
		return &HealthCheckResult{
			Status:    healthStatusUnknown,
			CheckedAt: s.Clock.Now(),
			Error:     "site host is not configured",
		}, nil
	}

	// Build URL for health check
	checkURL := site.Host
	if site.SslMode != "off" && site.ForceHttps {
		if checkURL != "" && checkURL[:5] != "https" {
			checkURL = "https://" + checkURL
		}
	} else {
		if checkURL != "" && checkURL[:4] != "http" {
			checkURL = "http://" + checkURL
		}
	}

	// SSRF 前置校验
	if err := ssrf.ValidateURL(checkURL); err != nil {
		return &HealthCheckResult{
			Status:    healthStatusUnhealthy,
			CheckedAt: s.Clock.Now(),
			Error:     err.Error(),
		}, nil
	}

	// 组合：SafeTransport 提供 SSRF 防护；TLS 默认校验证书，
	// 仅当站点显式声明 Metadata["insecureSkipTlsVerify"]=true（自签内网端点）才跳过，
	// 避免全局关闭证书校验被中间人劫持健康检查结果。
	transport := ssrf.SafeTransport(10 * time.Second)
	if tlsSkipVerifyOptIn(site.Metadata) {
		// 跳过校验是站点级显式开关：SafeTransport 仍会拒绝私有/回环目标，
		// 且这里只放宽证书链校验，最低 TLS 版本仍保持 1.2；使用前记录告警以便审计。
		slog.WarnContext(ctx, "site health check skips TLS verification",
			"site_id", site.ID, "url", checkURL)
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true, // #nosec G402 -- per-site opt-in, private targets blocked by SafeTransport
			MinVersion:         tls.VersionTLS12,
		}
	}
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
	}

	start := s.Clock.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, http.NoBody)
	if err != nil {
		return &HealthCheckResult{
			Status:    healthStatusUnhealthy,
			CheckedAt: start,
			Error:     err.Error(),
		}, nil
	}

	resp, err := client.Do(req)
	latency := s.Clock.Now().Sub(start)
	if err != nil {
		site.LastHealthStatus = healthStatusUnhealthy
		site.LastHealthAt = &start
		site.UpdatedAt = s.Clock.Now()
		site.Revision++
		if updateErr := s.UOW.Gateway().UpdateSite(ctx, site); updateErr != nil {
			return nil, apperrors.WrapInternalf(updateErr, "update unhealthy site health status")
		}

		return &HealthCheckResult{
			Status:    healthStatusUnhealthy,
			CheckedAt: start,
			Latency:   latency.String(),
			Error:     err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	status := healthStatusHealthy
	if resp.StatusCode >= 400 {
		status = healthStatusUnhealthy
	}

	site.LastHealthStatus = status
	now := s.Clock.Now()
	site.LastHealthAt = &now
	site.UpdatedAt = now
	site.Revision++
	if err := s.UOW.Gateway().UpdateSite(ctx, site); err != nil {
		return nil, apperrors.WrapInternalf(err, "update site health status")
	}

	// Evaluate alert conditions
	if status == healthStatusUnhealthy && site.EnableAlert && len(site.AlertProviderIDs) > 0 {
		s.evaluateAlerts(ctx, site)
	}

	return &HealthCheckResult{
		Status:    status,
		CheckedAt: now,
		Latency:   latency.String(),
	}, nil
}

// tlsSkipVerifyOptIn 仅当站点 Metadata 显式声明 insecureSkipTlsVerify=true 时
// 才允许跳过 TLS 证书校验（自签内网端点场景）。默认严格校验。
func tlsSkipVerifyOptIn(metadata map[string]any) bool {
	allowSelfSigned, _ := metadata["insecureSkipTlsVerify"].(bool)
	return allowSelfSigned
}

// GetHealthHistory returns the latest health status for a site.
func (s *Service) GetHealthHistory(ctx context.Context, organizationID, siteID string) (*HealthCheckResult, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// siteID comes from the URL path; the lookup is tenant-scoped so a foreign
	// id is indistinguishable from a missing one.
	site, err := s.UOW.Gateway().GetSiteByIDForOrganization(ctx, siteID, organizationID)
	if err != nil {
		return nil, err
	}
	return &HealthCheckResult{
		Status: site.LastHealthStatus,
		CheckedAt: func() time.Time {
			if site.LastHealthAt != nil {
				return *site.LastHealthAt
			}
			return time.Time{}
		}(),
	}, nil
}

// GetGlobalSites returns all sites across organizations (admin only).
func (s *Service) GetGlobalSites(ctx context.Context) ([]*SiteListItem, error) {
	// 全局列表无 org 参数：ListSites 传空 OrganizationID，SQL 不加 org 条件；返回范围由调用方作用域（platform_admin 会话为 super_admin）与 RLS 决定，service 不提升作用域。
	sites, err := platformpagination.CollectAll(func(page, pageSize int) ([]*domain.Site, int64, error) {
		return s.UOW.Gateway().ListSites(ctx, domain.ListSitesQuery{Page: page, PageSize: pageSize})
	})
	if err != nil {
		return nil, err
	}
	items := make([]*SiteListItem, 0, len(sites))
	for _, site := range sites {
		items = append(items, &SiteListItem{
			ID:             site.ID,
			OrganizationID: site.OrganizationID,
			Name:           site.Name,
			Host:           site.Host,
			Status:         site.Status,
			LastHealth:     site.LastHealthStatus,
			CreatedAt:      site.CreatedAt,
		})
	}
	return items, nil
}

// SiteListItem is a lightweight view for global site listing.
type SiteListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	Host           string    `json:"host"`
	Status         string    `json:"status"`
	LastHealth     string    `json:"lastHealthStatus,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}
