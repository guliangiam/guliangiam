package gateway

import "time"

type Site struct {
	ID                  string         `json:"id"`
	OrganizationID      string         `json:"organizationId"`
	Name                string         `json:"name"`
	Host                string         `json:"host"`
	Status              string         `json:"status"`
	SslMode             string         `json:"sslMode"`                    // off, auto, manual
	CertID              string         `json:"certId,omitempty"`           // FK to certs table
	ForceHttps          bool           `json:"forceHttps"`                 // redirect HTTP→HTTPS
	EnableHealthCheck   bool           `json:"enableHealthCheck"`          // enable periodic health checks
	HealthCheckInterval int            `json:"healthCheckInterval"`        // seconds between checks
	LastHealthStatus    string         `json:"lastHealthStatus,omitempty"` // healthy, unhealthy, unknown
	LastHealthAt        *time.Time     `json:"lastHealthAt,omitempty"`     // last health check time
	EnableAlert         bool           `json:"enableAlert"`                // alert on consecutive failures
	AlertInterval       int            `json:"alertInterval"`              // seconds between alert retries
	AlertTryTimes       int            `json:"alertTryTimes"`              // consecutive failures before alerting
	AlertProviderIDs    []string       `json:"alertProviderIds,omitempty"` // notification provider IDs
	Metadata            map[string]any `json:"metadata"`
	CreatedAt           time.Time      `json:"createdAt"`
	UpdatedAt           time.Time      `json:"updatedAt"`
	DeletedAt           *time.Time     `json:"deletedAt,omitempty"`
	Revision            int64          `json:"revision"`
}
