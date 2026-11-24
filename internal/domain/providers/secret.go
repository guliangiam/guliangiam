package providers

import "time"

type ProviderSecretStatus struct {
	ID            string
	ProviderID    string
	SecretType    string
	Status        string
	LastRotatedAt time.Time
	ExpiresAt     time.Time
	UpdatedAt     time.Time
	Version       int64
}
