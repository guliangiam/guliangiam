package gateway

import (
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
)

// ServerDTO is the data transfer object for gateway servers,
// decoupling consumers from the domain entity.
type ServerDTO struct {
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

// SiteDTO is the data transfer object for gateway sites,
// decoupling consumers from the domain entity.
type SiteDTO struct {
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
	LastHealthStatus    string         `json:"lastHealthStatus,omitempty"`
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

// ListServersQueryApp is the application-layer query type for listing servers,
// decoupling consumers from the domain query type.
type ListServersQueryApp struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Status         string
}

// ListSitesQueryApp is the application-layer query type for listing sites,
// decoupling consumers from the domain query type.
type ListSitesQueryApp struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Status         string
}

func toServerDTO(s *domain.Server) *ServerDTO {
	if s == nil {
		return nil
	}
	return &ServerDTO{
		ID:             s.ID,
		OrganizationID: s.OrganizationID,
		Name:           s.Name,
		Endpoint:       s.Endpoint,
		Protocol:       s.Protocol,
		Status:         s.Status,
		Metadata:       s.Metadata,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
		Revision:       s.Revision,
	}
}

func toSiteDTO(s *domain.Site) *SiteDTO {
	if s == nil {
		return nil
	}
	return &SiteDTO{
		ID:                  s.ID,
		OrganizationID:      s.OrganizationID,
		Name:                s.Name,
		Host:                s.Host,
		Status:              s.Status,
		SslMode:             s.SslMode,
		CertID:              s.CertID,
		ForceHttps:          s.ForceHttps,
		EnableHealthCheck:   s.EnableHealthCheck,
		HealthCheckInterval: s.HealthCheckInterval,
		LastHealthStatus:    s.LastHealthStatus,
		LastHealthAt:        s.LastHealthAt,
		EnableAlert:         s.EnableAlert,
		AlertInterval:       s.AlertInterval,
		AlertTryTimes:       s.AlertTryTimes,
		AlertProviderIDs:    append([]string(nil), s.AlertProviderIDs...),
		Metadata:            s.Metadata,
		CreatedAt:           s.CreatedAt,
		UpdatedAt:           s.UpdatedAt,
		Revision:            s.Revision,
	}
}
