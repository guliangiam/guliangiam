package models

import "context"

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

type Repository interface {
	// GetByID is the system-scoped lookup used by the Casbin policy engine
	// (permissions/enforcers getModelText), which is outside the management
	// by-ID tenant binding. Management routes must use
	// GetByIDForOrganization so a caller-supplied id cannot cross tenants.
	GetByID(ctx context.Context, id string) (*Model, error)
	// GetByIDForOrganization returns the model only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Model, error)
	List(ctx context.Context, query ListQuery) ([]*Model, int64, error)
	Create(ctx context.Context, model *Model) error
	// Update keeps the entity signature so the written row is scoped by the
	// entity's own OrganizationID, not by a caller-supplied value.
	Update(ctx context.Context, model *Model) error
	// Delete removes the model only when it belongs to organizationID;
	// zero affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
}
