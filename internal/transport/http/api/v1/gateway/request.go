package gateway

import (
	appgateway "github.com/guliangiam/guliangiam/internal/app/gateway"
)

type ListServersRequest struct {
	Page           int    `json:"page"`
	PageSize       int    `json:"pageSize"`
	OrganizationID string `json:"organizationId"`
	Keyword        string `json:"keyword"`
	Status         string `json:"status"`
}

func (r ListServersRequest) toQuery() appgateway.ListServersQueryApp {
	return appgateway.ListServersQueryApp{
		Page:           r.Page,
		PageSize:       r.PageSize,
		OrganizationID: r.OrganizationID,
		Keyword:        r.Keyword,
		Status:         r.Status,
	}
}

type CreateServerRequest struct {
	OrganizationID string         `json:"organizationId"`
	Name           string         `json:"name"`
	Endpoint       string         `json:"endpoint"`
	Protocol       string         `json:"protocol"`
	Status         string         `json:"status"`
	Metadata       map[string]any `json:"metadata"`
}

func (r CreateServerRequest) toCommand() appgateway.CreateServerCommand {
	return appgateway.CreateServerCommand{
		OrganizationID: r.OrganizationID,
		Name:           r.Name,
		Endpoint:       r.Endpoint,
		Protocol:       r.Protocol,
		Status:         r.Status,
		Metadata:       r.Metadata,
	}
}

type UpdateServerRequest struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Endpoint string         `json:"endpoint"`
	Protocol string         `json:"protocol"`
	Status   string         `json:"status"`
	Metadata map[string]any `json:"metadata"`
	Revision int64          `json:"revision"`
}

func (r UpdateServerRequest) toCommand() appgateway.UpdateServerCommand {
	return appgateway.UpdateServerCommand{
		ID:       r.ID,
		Name:     r.Name,
		Endpoint: r.Endpoint,
		Protocol: r.Protocol,
		Status:   r.Status,
		Metadata: r.Metadata,
		Revision: r.Revision,
	}
}

type ListSitesRequest struct {
	Page           int    `json:"page"`
	PageSize       int    `json:"pageSize"`
	OrganizationID string `json:"organizationId"`
	Keyword        string `json:"keyword"`
	Status         string `json:"status"`
}

func (r ListSitesRequest) toQuery() appgateway.ListSitesQueryApp {
	return appgateway.ListSitesQueryApp{
		Page:           r.Page,
		PageSize:       r.PageSize,
		OrganizationID: r.OrganizationID,
		Keyword:        r.Keyword,
		Status:         r.Status,
	}
}

type CreateSiteRequest struct {
	OrganizationID      string         `json:"organizationId"`
	Name                string         `json:"name"`
	Host                string         `json:"host"`
	Status              string         `json:"status"`
	SslMode             string         `json:"sslMode,omitempty"`
	CertID              string         `json:"certId,omitempty"`
	ForceHttps          bool           `json:"forceHttps"`
	EnableHealthCheck   bool           `json:"enableHealthCheck"`
	HealthCheckInterval int            `json:"healthCheckInterval"`
	EnableAlert         bool           `json:"enableAlert"`
	AlertInterval       int            `json:"alertInterval"`
	AlertTryTimes       int            `json:"alertTryTimes"`
	AlertProviderIDs    []string       `json:"alertProviderIds,omitempty"`
	Metadata            map[string]any `json:"metadata"`
}

func (r CreateSiteRequest) toCommand() appgateway.CreateSiteCommand {
	return appgateway.CreateSiteCommand{
		OrganizationID:      r.OrganizationID,
		Name:                r.Name,
		Host:                r.Host,
		Status:              r.Status,
		SslMode:             r.SslMode,
		CertID:              r.CertID,
		ForceHttps:          r.ForceHttps,
		EnableHealthCheck:   r.EnableHealthCheck,
		HealthCheckInterval: r.HealthCheckInterval,
		EnableAlert:         r.EnableAlert,
		AlertInterval:       r.AlertInterval,
		AlertTryTimes:       r.AlertTryTimes,
		AlertProviderIDs:    r.AlertProviderIDs,
		Metadata:            r.Metadata,
	}
}

type UpdateSiteRequest struct {
	ID                  string         `json:"id"`
	Name                string         `json:"name"`
	Host                string         `json:"host"`
	Status              string         `json:"status"`
	SslMode             string         `json:"sslMode,omitempty"`
	CertID              string         `json:"certId,omitempty"`
	ForceHttps          bool           `json:"forceHttps"`
	EnableHealthCheck   bool           `json:"enableHealthCheck"`
	HealthCheckInterval int            `json:"healthCheckInterval"`
	EnableAlert         bool           `json:"enableAlert"`
	AlertInterval       int            `json:"alertInterval"`
	AlertTryTimes       int            `json:"alertTryTimes"`
	AlertProviderIDs    []string       `json:"alertProviderIds,omitempty"`
	Metadata            map[string]any `json:"metadata"`
	Revision            int64          `json:"revision"`
}

func (r UpdateSiteRequest) toCommand() appgateway.UpdateSiteCommand {
	return appgateway.UpdateSiteCommand{
		ID:                  r.ID,
		Name:                r.Name,
		Host:                r.Host,
		Status:              r.Status,
		SslMode:             r.SslMode,
		CertID:              r.CertID,
		ForceHttps:          r.ForceHttps,
		EnableHealthCheck:   r.EnableHealthCheck,
		HealthCheckInterval: r.HealthCheckInterval,
		EnableAlert:         r.EnableAlert,
		AlertInterval:       r.AlertInterval,
		AlertTryTimes:       r.AlertTryTimes,
		AlertProviderIDs:    r.AlertProviderIDs,
		Metadata:            r.Metadata,
		Revision:            r.Revision,
	}
}
