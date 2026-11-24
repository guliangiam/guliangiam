package gateway

import (
	"time"

	appgateway "github.com/guliangiam/guliangiam/internal/app/gateway"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type ServerResponse struct {
	ID             string         `json:"id"`
	OrganizationID string         `json:"organizationId"`
	Name           string         `json:"name"`
	Endpoint       string         `json:"endpoint"`
	Protocol       string         `json:"protocol"`
	Status         string         `json:"status"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	Revision       int64          `json:"revision"`
}

func toServerResponse(server *appgateway.ServerDTO) *ServerResponse {
	return &ServerResponse{
		ID:             server.ID,
		OrganizationID: server.OrganizationID,
		Name:           server.Name,
		Endpoint:       server.Endpoint,
		Protocol:       server.Protocol,
		Status:         server.Status,
		Metadata:       server.Metadata,
		CreatedAt:      server.CreatedAt,
		UpdatedAt:      server.UpdatedAt,
		Revision:       server.Revision,
	}
}

type ListServersResponse = presenter.ListResponse[*appgateway.ServerDTO]

type CreateServerResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type SiteResponse struct {
	ID                  string         `json:"id"`
	OrganizationID      string         `json:"organizationId"`
	Name                string         `json:"name"`
	Host                string         `json:"host"`
	Status              string         `json:"status"`
	SslMode             string         `json:"sslMode"`
	CertID              string         `json:"certId,omitempty"`
	ForceHttps          bool           `json:"forceHttps"`
	EnableHealthCheck   bool           `json:"enableHealthCheck"`
	HealthCheckInterval int            `json:"healthCheckInterval"`
	LastHealthStatus    string         `json:"lastHealthStatus"`
	LastHealthAt        *time.Time     `json:"lastHealthAt,omitempty"`
	EnableAlert         bool           `json:"enableAlert"`
	AlertInterval       int            `json:"alertInterval"`
	AlertTryTimes       int            `json:"alertTryTimes"`
	AlertProviderIDs    []string       `json:"alertProviderIds,omitempty"`
	Metadata            map[string]any `json:"metadata"`
	CreatedAt           time.Time      `json:"createdAt"`
	UpdatedAt           time.Time      `json:"updatedAt"`
	Revision            int64          `json:"revision"`
}

func toSiteResponse(site *appgateway.SiteDTO) *SiteResponse {
	return &SiteResponse{
		ID:                  site.ID,
		OrganizationID:      site.OrganizationID,
		Name:                site.Name,
		Host:                site.Host,
		Status:              site.Status,
		SslMode:             site.SslMode,
		CertID:              site.CertID,
		ForceHttps:          site.ForceHttps,
		EnableHealthCheck:   site.EnableHealthCheck,
		HealthCheckInterval: site.HealthCheckInterval,
		LastHealthStatus:    site.LastHealthStatus,
		LastHealthAt:        site.LastHealthAt,
		EnableAlert:         site.EnableAlert,
		AlertInterval:       site.AlertInterval,
		AlertTryTimes:       site.AlertTryTimes,
		AlertProviderIDs:    site.AlertProviderIDs,
		Metadata:            site.Metadata,
		CreatedAt:           site.CreatedAt,
		UpdatedAt:           site.UpdatedAt,
		Revision:            site.Revision,
	}
}

type ListSitesResponse = presenter.ListResponse[*appgateway.SiteDTO]

type CreateSiteResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}
