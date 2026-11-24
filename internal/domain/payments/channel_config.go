package payments

import (
	"fmt"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// PayChannelConfig versions a tenant's credentials for a specific payment
// channel. Four-state machine: draft (initial) → testing (self-verifying) →
// active (live) / disabled (shut down). Versioning means rotation produces a
// new row rather than updating in place — the old config row is soft-deleted
// so history survives and rollback is a one-row-query.
const (
	ChannelConfigStatusDraft    = "draft"
	ChannelConfigStatusTesting  = "testing"
	ChannelConfigStatusActive   = "active"
	ChannelConfigStatusDisabled = "disabled"
)

var validConfigTransitions = map[string][]string{
	ChannelConfigStatusDraft:    {ChannelConfigStatusTesting, ChannelConfigStatusDisabled},
	ChannelConfigStatusTesting:  {ChannelConfigStatusActive, ChannelConfigStatusDraft, ChannelConfigStatusDisabled},
	ChannelConfigStatusActive:   {ChannelConfigStatusDisabled},
	ChannelConfigStatusDisabled: {ChannelConfigStatusDraft},
}

// channelEnvironments / channelConfigStatuses are the closed sets Validate
// reports in `allowed`.
var channelEnvironments = []string{"sandbox", "production"}

var channelConfigStatuses = []string{
	string(ChannelConfigStatusDraft), string(ChannelConfigStatusTesting),
	string(ChannelConfigStatusActive), string(ChannelConfigStatusDisabled),
}

type PayChannelConfig struct {
	ID                string     `json:"id"`
	OrganizationID    string     `json:"organizationId"`
	ProviderID        string     `json:"providerId,omitempty"`
	Channel           string     `json:"channel"`
	Environment       string     `json:"environment"`
	ConfigSealed      string     `json:"-"` // encrypted, never exposed
	ConfigFingerprint string     `json:"configFingerprint"`
	ConfigSet         []string   `json:"configSet,omitempty"`
	VersionNo         int        `json:"versionNo"`
	Status            string     `json:"status"`
	TestPassedAt      *time.Time `json:"testPassedAt,omitempty"`
	ActivatedAt       *time.Time `json:"activatedAt,omitempty"`
	DisabledAt        *time.Time `json:"disabledAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
	DeletedAt         *time.Time `json:"deletedAt,omitempty"`
	Revision          int64      `json:"revision"`
}

func (c *PayChannelConfig) Validate() error {
	if c.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if c.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if c.Channel == "" {
		return apperrors.NewInvalidArgument("channel is required")
	}
	if c.Environment == "" {
		c.Environment = "production"
	}
	if c.Environment != "sandbox" && c.Environment != "production" {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid environment: %s", c.Environment),
			map[string]any{"field": "environment", "value": c.Environment, "allowed": append([]string{}, channelEnvironments...)},
		)
	}
	if c.Status == "" {
		c.Status = ChannelConfigStatusDraft
	}
	switch c.Status {
	case ChannelConfigStatusDraft, ChannelConfigStatusTesting,
		ChannelConfigStatusActive, ChannelConfigStatusDisabled:
	default:
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("invalid config status: %s", c.Status),
			map[string]any{"field": "status", "value": c.Status, "allowed": append([]string{}, channelConfigStatuses...)},
		)
	}
	if c.ConfigFingerprint == "" {
		return apperrors.NewInvalidArgument("configFingerprint is required")
	}
	if c.VersionNo < 1 {
		c.VersionNo = 1
	}
	return nil
}

func (c *PayChannelConfig) CanTransitionTo(newStatus string) bool {
	for _, s := range validConfigTransitions[c.Status] {
		if s == newStatus {
			return true
		}
	}
	return false
}

func (c *PayChannelConfig) TransitionTo(newStatus string) error {
	if newStatus == c.Status {
		return nil
	}
	if !c.CanTransitionTo(newStatus) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("cannot move channel config from %s to %s", c.Status, newStatus),
			map[string]any{"field": "status", "value": c.Status, "allowed": append([]string{}, validConfigTransitions[c.Status]...)},
		)
	}
	c.Status = newStatus
	switch newStatus {
	case ChannelConfigStatusActive:
		t := time.Now()
		c.ActivatedAt = &t
	case ChannelConfigStatusDisabled:
		t := time.Now()
		c.DisabledAt = &t
	}
	return nil
}

func (c *PayChannelConfig) IsUsable() bool {
	return c.Status == ChannelConfigStatusActive && c.DeletedAt == nil
}
