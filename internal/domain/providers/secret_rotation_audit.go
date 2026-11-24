package providers

import "time"

type SecretRotationAudit struct {
	ID         string
	ProviderID string
	SecretType string
	OldVersion int
	NewVersion int
	RotatedBy  string
	RotatedAt  time.Time
	Reason     string
}

type SecretRotationAuditRepository interface {
	Create(audit *SecretRotationAudit) error
	ListByProviderID(providerID string, limit int) ([]SecretRotationAudit, error)
}
