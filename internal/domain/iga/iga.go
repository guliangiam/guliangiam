package iga

import "time"

// ─── Access Campaign ───────────────────────────────────────────────────────

type CampaignStatus string

const (
	CampaignDraft     CampaignStatus = "draft"
	CampaignActive    CampaignStatus = "active"
	CampaignCompleted CampaignStatus = "completed"
	CampaignCancelled CampaignStatus = "cancelled"
)

// AccessCampaign is a time-boxed access review drive (ADR-025). States are
// enforced by CHECK constraint ck_access_campaigns_status.
type AccessCampaign struct {
	ID             string
	OrganizationID string
	Name           string
	Status         CampaignStatus
	StartDate      *time.Time
	EndDate        *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

// ─── SOD Rule ──────────────────────────────────────────────────────────────

// SODRule encodes "role A and role B must not be held by the same subject"
// (uq_sod_rules_org_roles prevents duplicate declarations).
type SODRule struct {
	ID             string
	OrganizationID string
	Name           string
	RoleAID        string
	RoleBID        string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

// ─── Access Lifecycle ──────────────────────────────────────────────────────

type LifecycleStatus string

const (
	LifecycleActive  LifecycleStatus = "active"
	LifecycleExpired LifecycleStatus = "expired"
	LifecycleRevoked LifecycleStatus = "revoked"
)

// AccessLifecycle is a user→role grant with TTL. The index on expires_at
// WHERE status='active' lets a background job find expiring grants fast.
type AccessLifecycle struct {
	ID             string
	OrganizationID string
	UserID         string
	RoleID         string
	TicketID       string
	EnforcerID     string
	Resource       string
	Action         string
	PolicyID       string
	GrantedAt      time.Time
	StartsAt       time.Time
	ExpiresAt      time.Time
	RevokedAt      *time.Time
	Status         LifecycleStatus
	RetryCount     int
	LastError      string
	CreatedAt      time.Time
	Revision       int64
}

// ─── Queries ───────────────────────────────────────────────────────────────

type ListCampaignsQuery struct {
	OrganizationID string
	Status         *CampaignStatus
	Page, PageSize int
}
type ListSODRulesQuery struct {
	OrganizationID string
	Page, PageSize int
}
type ListLifecycleQuery struct {
	OrganizationID string
	UserID         *string
	RoleID         *string
	Status         *LifecycleStatus
	Page, PageSize int
}

// ─── Commands ──────────────────────────────────────────────────────────────

type CreateCampaignCommand struct {
	OrganizationID, Name string
	StartDate, EndDate   *time.Time
}
type UpdateCampaignCommand struct {
	OrganizationID, ID string
	Status             CampaignStatus
}
type CreateSODRuleCommand struct{ OrganizationID, Name, RoleAID, RoleBID string }
type RevokeLifecycleCommand struct{ OrganizationID, ID string } // revoke a single grant
type ExpireLifecycleCommand struct {
	OrganizationID string
	Before         time.Time
} // bulk expire

// RequestStatus is the access-request workflow persisted in tickets.status.
type RequestStatus string

const (
	RequestSubmitted      RequestStatus = "submitted"
	RequestPendingManager RequestStatus = "pending_manager"
	RequestPendingSAP     RequestStatus = "pending_sap"
	RequestApproved       RequestStatus = "approved"
	RequestProvisioned    RequestStatus = "provisioned"
	RequestRejected       RequestStatus = "rejected"
	RequestWithdrawn      RequestStatus = "withdrawn"
)

// AccessRequest is the IGA projection of the shared tickets aggregate.
type AccessRequest struct {
	ID                string
	OrganizationID    string
	RequesterID       string
	RoleID            string
	ApplicationID     string
	EnforcerID        string
	Resource          string
	Action            string
	Reason            string
	Status            RequestStatus
	StartsAt          time.Time
	ExpiresAt         time.Time
	ManagerApproverID string
	SAPApproverID     string
	LifecycleID       string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Revision          int64
}

type ListAccessRequestsQuery struct {
	OrganizationID string
	RequesterID    string
	Status         *RequestStatus
	Page, PageSize int
}

type TicketMessage struct {
	ID        string
	TicketID  string
	AuthorID  string
	Action    string
	Content   string
	CreatedAt time.Time
}

type AccessTargetValidity struct {
	UserValid, RoleValid, ApplicationValid, EnforcerValid bool
	Subject, OrganizationSlug                             string
}

func (v AccessTargetValidity) Valid() bool {
	return v.UserValid && v.RoleValid && v.ApplicationValid && v.EnforcerValid
}

type JITBinding struct {
	UserID, RoleID, LifecycleID string
	StartsAt, ExpiresAt         time.Time
}

type JITPolicy struct {
	ID, EnforcerID, LifecycleID string
	UserID, Subject             string
	OrganizationSlug            string
	Resource, Action            string
	StartsAt, ExpiresAt         time.Time
}
