package gateway

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
)

func toServer(row serverRow) domain.Server {
	return domain.Server{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Name:           row.Name,
		Endpoint:       row.Endpoint,
		Protocol:       row.Protocol,
		Status:         row.Status,
		Metadata:       row.Metadata,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		DeletedAt:      row.DeletedAt,
		Revision:       row.Revision,
	}
}

func toSite(row siteRow) domain.Site {
	return domain.Site{
		ID:                  row.ID,
		OrganizationID:      row.OrganizationID,
		Name:                row.Name,
		Host:                row.Host,
		Status:              row.Status,
		SslMode:             row.SslMode,
		CertID:              row.CertID,
		ForceHttps:          row.ForceHttps,
		EnableHealthCheck:   row.EnableHealthCheck,
		HealthCheckInterval: row.HealthCheckInterval,
		LastHealthStatus:    row.LastHealthStatus,
		LastHealthAt:        row.LastHealthAt,
		EnableAlert:         row.EnableAlert,
		AlertInterval:       row.AlertInterval,
		AlertTryTimes:       row.AlertTryTimes,
		AlertProviderIDs:    row.AlertProviderIDs,
		Metadata:            row.Metadata,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
		DeletedAt:           row.DeletedAt,
		Revision:            row.Revision,
	}
}
