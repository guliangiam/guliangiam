package iga

import (
	"context"
	"time"
)

type Repository interface {
	// Access requests (shared tickets/ticket_messages storage)
	ListAccessRequests(ctx context.Context, q ListAccessRequestsQuery) ([]*AccessRequest, int64, error)
	GetAccessRequest(ctx context.Context, orgID, id string, forUpdate bool) (*AccessRequest, error)
	CreateAccessRequest(ctx context.Context, r *AccessRequest) error
	UpdateAccessRequest(ctx context.Context, r *AccessRequest, expected RequestStatus) (bool, error)
	AddTicketMessage(ctx context.Context, m *TicketMessage) error
	ListTicketMessages(ctx context.Context, orgID, ticketID string) ([]*TicketMessage, error)
	ValidateAccessTargets(ctx context.Context, orgID, userID, roleID, applicationID, enforcerID string) (AccessTargetValidity, error)
	ListEffectiveRoleIDs(ctx context.Context, orgID, userID string, at time.Time) ([]string, error)
	ResolveDefaultEnforcer(ctx context.Context, orgID string) (string, error)

	// JIT artifacts and reclaim
	CreateJITBinding(ctx context.Context, b *JITBinding) error
	CreateJITPolicy(ctx context.Context, p *JITPolicy) error
	RevokeJITArtifacts(ctx context.Context, lifecycleID, policyID string) error
	RevokePrivilegedSessions(ctx context.Context, lifecycleID string, at time.Time) (int64, error)
	GetLifecycleForUpdate(ctx context.Context, orgID, id string) (*AccessLifecycle, error)
	ListExpiringLifecycle(ctx context.Context, orgID string, before time.Time, limit int) ([]*AccessLifecycle, error)
	CompleteLifecycle(ctx context.Context, orgID, id string, status LifecycleStatus, at time.Time) (bool, error)
	RecordLifecycleFailure(ctx context.Context, orgID, id, message string) error

	// Campaigns
	ListCampaigns(ctx context.Context, q ListCampaignsQuery) ([]*AccessCampaign, int64, error)
	GetCampaign(ctx context.Context, id, orgID string) (*AccessCampaign, error)
	CreateCampaign(ctx context.Context, c *AccessCampaign) error
	UpdateCampaignStatus(ctx context.Context, orgID, id string, status CampaignStatus) error
	DeleteCampaign(ctx context.Context, id, orgID string) error

	// SOD Rules
	ListSODRules(ctx context.Context, q ListSODRulesQuery) ([]*SODRule, int64, error)
	GetSODRule(ctx context.Context, id, orgID string) (*SODRule, error)
	CreateSODRule(ctx context.Context, r *SODRule) error
	UpdateSODRule(ctx context.Context, orgID, id, name, roleA, roleB string) error
	DeleteSODRule(ctx context.Context, id, orgID string) error

	// Lifecycle
	ListLifecycle(ctx context.Context, q ListLifecycleQuery) ([]*AccessLifecycle, int64, error)
	GetLifecycle(ctx context.Context, id, orgID string) (*AccessLifecycle, error)
	CreateLifecycle(ctx context.Context, l *AccessLifecycle) error
	UpdateLifecycleStatus(ctx context.Context, orgID, id string, status LifecycleStatus) error
	RevokeLifecycle(ctx context.Context, id, orgID string) error
	ExpireLifecycle(ctx context.Context, orgID string, before time.Time) (int64, error)
}
