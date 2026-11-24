package providers

import "time"

type ProviderConfig struct {
	ProviderID string
	ConfigType string
	Config     map[string]any
	UpdatedAt  time.Time
	Revision   int64
}
