package scim

import "time"

// SCIM 2.0 schema URNs.
const (
	SchemaUser         = "urn:ietf:params:scim:schemas:core:2.0:User"
	SchemaGroup        = "urn:ietf:params:scim:schemas:core:2.0:Group"
	SchemaListResponse = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	SchemaPatchOp      = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
)

// ScimListQuery represents SCIM list query parameters.
type ScimListQuery struct {
	StartIndex int    `json:"startIndex"`
	Count      int    `json:"count"`
	Filter     string `json:"filter,omitempty"`
	SortBy     string `json:"sortBy,omitempty"`
	SortOrder  string `json:"sortOrder,omitempty"`
}

// ScimListResponse is a generic SCIM list response.
type ScimListResponse[T any] struct {
	TotalResults int      `json:"totalResults"`
	ItemsPerPage int      `json:"itemsPerPage"`
	StartIndex   int      `json:"startIndex"`
	Schemas      []string `json:"schemas"`
	Resources    []T      `json:"Resources,omitempty"`
}

// ScimUser represents a SCIM 2.0 User resource.
type ScimUser struct {
	Schemas      []string    `json:"schemas"`
	ID           string      `json:"id,omitempty"`
	ExternalID   string      `json:"externalId,omitempty"`
	UserName     string      `json:"userName"`
	Name         *ScimName   `json:"name,omitempty"`
	DisplayName  string      `json:"displayName,omitempty"`
	Emails       []ScimEmail `json:"emails,omitempty"`
	PhoneNumbers []ScimPhone `json:"phoneNumbers,omitempty"`
	Active       bool        `json:"active"`
	Meta         *ScimMeta   `json:"meta,omitempty"`
}

// ScimName represents the name component of a SCIM User.
type ScimName struct {
	GivenName       string `json:"givenName,omitempty"`
	FamilyName      string `json:"familyName,omitempty"`
	MiddleName      string `json:"middleName,omitempty"`
	HonorificPrefix string `json:"honorificPrefix,omitempty"`
	HonorificSuffix string `json:"honorificSuffix,omitempty"`
	Formatted       string `json:"formatted,omitempty"`
}

// ScimEmail represents an email address in SCIM format.
type ScimEmail struct {
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"`
	Primary bool   `json:"primary,omitempty"`
}

// ScimPhone represents a phone number in SCIM format.
type ScimPhone struct {
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"`
	Primary bool   `json:"primary,omitempty"`
}

// ScimGroup represents a SCIM 2.0 Group resource.
type ScimGroup struct {
	Schemas     []string          `json:"schemas"`
	ID          string            `json:"id,omitempty"`
	DisplayName string            `json:"displayName"`
	Members     []ScimGroupMember `json:"members,omitempty"`
	Meta        *ScimMeta         `json:"meta,omitempty"`
}

// ScimGroupMember represents a member of a SCIM Group.
type ScimGroupMember struct {
	Value   string `json:"value"`
	Ref     string `json:"$ref,omitempty"`
	Display string `json:"display,omitempty"`
}

// ScimMeta represents SCIM resource metadata.
type ScimMeta struct {
	ResourceType string    `json:"resourceType"`
	Created      time.Time `json:"created,omitempty"`
	LastModified time.Time `json:"lastModified,omitempty"`
}

// ScimPatchOp represents a SCIM PATCH request.
type ScimPatchOp struct {
	Schemas    []string        `json:"schemas"`
	Operations []ScimOperation `json:"Operations"`
}

// ScimOperation represents a single SCIM PATCH operation.
type ScimOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

// ScimFilter represents a parsed SCIM filter expression.
type ScimFilter struct {
	Attribute string
	Operator  string
	Value     string
}
