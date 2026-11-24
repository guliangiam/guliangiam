package rebac

import "time"

// Namespace groups a set of relation definitions and the tuples that populate
// them. A single IAM org typically runs multiple namespaces (e.g. "iam" for
// user→role, "doc" for doc→folder), each with its own relation vocabulary so
// that the permission engine can reason about them independently.
type Namespace struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	// RelationDefs is a free-form JSONB bag describing the relation vocabulary:
	// {"roles": ["admin","member","viewer"], "permissions": ["read","write"]}.
	RelationDefs map[string]any `json:"relationDefs"`
	Version      int            `json:"version"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	Revision     int64          `json:"revision"`
}

// Tuple is one row of the relation table: subject [subject_relation] relation object.
// See ADR-024 (ReBAC Relation Tuple Authorization) for the full semantics.
type Tuple struct {
	ID              string    `json:"id"`
	OrganizationID  string    `json:"organizationId"`
	NamespaceID     string    `json:"namespaceId"`
	Subject         string    `json:"subject"`
	SubjectRelation *string   `json:"subjectRelation,omitempty"`
	Relation        string    `json:"relation"`
	Object          string    `json:"object"`
	Zookie          string    `json:"zookie"`
	CreatedAt       time.Time `json:"createdAt"`
}

type CreateNamespaceCommand struct {
	OrganizationID string
	Name           string
	DisplayName    string
	RelationDefs   map[string]any
}

type ListNamespacesQuery struct {
	OrganizationID string
	Page           int
	PageSize       int
}

type CreateTupleCommand struct {
	OrganizationID  string
	NamespaceID     string
	Subject         string
	SubjectRelation *string
	Relation        string
	Object          string
}

type ListTuplesQuery struct {
	OrganizationID string
	NamespaceID    string
	Page           int
	PageSize       int
}
