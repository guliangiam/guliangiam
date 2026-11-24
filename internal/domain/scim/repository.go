package scim

import (
	"context"
)

// ListQuery defines the query parameters for listing scim.
type ListQuery struct {
	Filter     string // SCIM filter expression
	Count      int    // Maximum results per page (default 100)
	StartIndex int    // 1-based index of the first result
	SortBy     string // Attribute to sort by
	SortOrder  string // "ascending" or "descending"
}

// Repository defines the interface for scim persistence operations.
type Repository interface { // ScimSchema operations
	GetSchemaBySchemaID(ctx context.Context, schemaID string) (*ScimSchema, error)
	ListSchemas(ctx context.Context, organizationID string, filter *ListQuery) ([]*ScimSchema, int64, error)
	CreateSchema(ctx context.Context, schema *ScimSchema) error
	UpdateSchema(ctx context.Context, schema *ScimSchema) error
	DeleteSchema(ctx context.Context, schemaID string) error

	// ScimUser operations
	GetUserByID(ctx context.Context, userID string) (*ScimUser, error)
	ListUsers(ctx context.Context, organizationID string, query *ListQuery) ([]*ScimUser, int64, error)
	CreateUser(ctx context.Context, user *ScimUser) error
	UpdateUser(ctx context.Context, user *ScimUser) error
	DeleteUser(ctx context.Context, userID string) error
	PatchUser(ctx context.Context, userID string, patches []PatchOperation) (*ScimUser, error)

	// ScimGroup operations
	GetGroupByID(ctx context.Context, groupID string) (*ScimGroup, error)
	ListGroups(ctx context.Context, organizationID string, query *ListQuery) ([]*ScimGroup, int64, error)
	CreateGroup(ctx context.Context, group *ScimGroup) error
	UpdateGroup(ctx context.Context, group *ScimGroup) error
	DeleteGroup(ctx context.Context, groupID string) error
	PatchGroup(ctx context.Context, groupID string, patches []PatchOperation) (*ScimGroup, error)
}
