package rebac

import "context"

// Repository is the persistence contract for ReBAC relation namespaces and
// tuples. The database enforces a unique (org, namespace, subject, relation,
// object) key so duplicate CreateTuple calls are idempotent — callers do not
// need to check existence up front.
//
// Repository extends TupleStore so Evaluator can be satisfied by the same
// object (single interface for both CRUD and evaluator fetch paths).
type Repository interface {
	TupleStore

	CreateNamespace(ctx context.Context, ns *Namespace) error
	ListNamespaces(ctx context.Context, query ListNamespacesQuery) ([]*Namespace, int64, error)
	GetNamespace(ctx context.Context, id, organizationID string) (*Namespace, error)
	// GetNamespaceByName looks up a namespace by (organization_id, name).
	// Used by APIAuthorizationMiddleware to resolve the "default" namespace UUID
	// for ReBAC Check — the namespace_id column is UUID, so a hardcoded string
	// like "default" cannot be passed directly. Returns sql.ErrNoRows when missing.
	GetNamespaceByName(ctx context.Context, organizationID, name string) (*Namespace, error)
	DeleteNamespace(ctx context.Context, id, organizationID string) error

	CreateTuple(ctx context.Context, tuple *Tuple) error
	// CreateTuplesBatch inserts tuples that all share one (OrganizationID,
	// NamespaceID) with a single revision bump and one bulk statement
	// (BE-D-18). Per-key idempotency matches CreateTuple: conflicting rows
	// keep their stored zookie and are returned as-read.
	CreateTuplesBatch(ctx context.Context, tuples []*Tuple) error
	ListTuples(ctx context.Context, query ListTuplesQuery) ([]*Tuple, int64, error)
	DeleteTuple(ctx context.Context, id, organizationID string) error
	DeleteTuplesByNamespace(ctx context.Context, namespaceID, organizationID string) (int64, error)
}
