package scim

import (
	"time"

	"github.com/google/uuid"
)

// ScimSchemaModel represents the database schema for scim_schemas table.
type ScimSchemaModel struct {
	ID             uuid.UUID `db:"id"`
	OrganizationID uuid.UUID `db:"organization_id"`
	SchemaID       string    `db:"schema_id"`
	Name           string    `db:"name"`
	Description    *string   `db:"description"`
	Attributes     []byte    `db:"attributes"` // JSONB stored as bytes
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	Revision       int64     `db:"revision"`
}

// ScimUserModel represents a user record (we'll use existing users table).
// This is a lightweight wrapper for mapping operations.
type ScimUserModel struct {
	ID             uuid.UUID `db:"id"`
	OrganizationID uuid.UUID `db:"organization_id"`
	Username       string    `db:"username"`
	DisplayName    *string   `db:"display_name"`
	Email          *string   `db:"email"`
	Phone          *string   `db:"phone"`
	IsActive       bool      `db:"is_active"`
	IsAdmin        bool      `db:"is_admin"`
	IsForbidden    bool      `db:"is_forbidden"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	Revision       int64     `db:"revision"`
}

// ScimGroupModel represents a group record (we'll use existing groups table).
type ScimGroupModel struct {
	ID             uuid.UUID `db:"id"`
	OrganizationID uuid.UUID `db:"organization_id"`
	Name           string    `db:"name"`
	DisplayName    string    `db:"display_name"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	Revision       int64     `db:"revision"`
}

// ScimGroupMemberModel represents a group member (from user_group_memberships table).
type ScimGroupMemberModel struct {
	UserID         uuid.UUID `db:"user_id"`
	GroupID        uuid.UUID `db:"group_id"`
	OrganizationID uuid.UUID `db:"organization_id"`
	JoinedAt       time.Time `db:"joined_at"`
}
