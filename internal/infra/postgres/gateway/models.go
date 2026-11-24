package gateway

import "time"

type serverRow struct {
	ID             string
	OrganizationID string
	Name           string
	Endpoint       string
	Protocol       string
	Status         string
	Metadata       map[string]any
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
	Revision       int64
}

type siteRow struct {
	ID                  string
	OrganizationID      string
	Name                string
	Host                string
	Status              string
	SslMode             string
	CertID              string
	ForceHttps          bool
	EnableHealthCheck   bool
	HealthCheckInterval int
	LastHealthStatus    string
	LastHealthAt        *time.Time
	EnableAlert         bool
	AlertInterval       int
	AlertTryTimes       int
	AlertProviderIDs    []string
	Metadata            map[string]any
	CreatedAt           time.Time
	UpdatedAt           time.Time
	DeletedAt           *time.Time
	Revision            int64
}
