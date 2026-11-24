package scim

import (
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// ScimSchema represents a SCIM 2.0 Schema definition.
// See RFC 7643 Section 6: https://datatracker.ietf.org/doc/html/rfc7643#section-6
type ScimSchema struct {
	ID             string      `json:"id"`
	OrganizationID string      `json:"organizationId"`
	SchemaID       string      `json:"schemaId"`    // URN (e.g., "urn:ietf:params:scim:schemas:core:2.0:User")
	Name           string      `json:"name"`        // Human-readable name (e.g., "User")
	Description    string      `json:"description"` // Schema description
	Attributes     []Attribute `json:"attributes"`  // Schema attributes
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedAt      time.Time   `json:"updatedAt"`
	Revision       int64       `json:"revision"`
}

// Attribute defines a SCIM attribute.
type Attribute struct {
	Name           string      `json:"name"`
	Type           string      `json:"type"` // "string", "boolean", "integer", "dateTime", "binary", "reference", "complex"
	MultiValued    bool        `json:"multiValued"`
	Description    string      `json:"description"`
	Required       bool        `json:"required"`
	CaseExact      bool        `json:"caseExact"`
	Mutability     string      `json:"mutability"`     // "readOnly", "readWrite", "writeOnly", "immutable"
	Returned       string      `json:"returned"`       // "always", "never", "default", "request"
	Uniqueness     string      `json:"uniqueness"`     // "none", "server", "global"
	ReferenceTypes []string    `json:"referenceTypes"` // For reference types
	SubAttributes  []Attribute `json:"subAttributes"`  // For complex types
}

// ScimUser represents a SCIM User resource mapped to a User.
// See RFC 7643 Section 8.1: https://datatracker.ietf.org/doc/html/rfc7643#section-8.1
type ScimUser struct {
	ID                string            `json:"id"`
	OrganizationID    string            `json:"organizationId"`
	UserName          string            `json:"userName"`          // Unique identifier for the User
	Name              *Name             `json:"name"`              // The components of the user's name
	DisplayName       string            `json:"displayName"`       // The name of the User
	NickName          string            `json:"nickName"`          // The casual way to address the user
	ProfileURL        string            `json:"profileUrl"`        // A fully qualified URL to a page
	Title             string            `json:"title"`             // The user's title
	UserType          string            `json:"userType"`          // Used to identify the organization to user relationship
	PreferredLanguage string            `json:"preferredLanguage"` // User's preferred written or spoken language
	Locale            string            `json:"locale"`            // User's default location for purposes of localizing items
	Timezone          string            `json:"timezone"`          // User's time zone
	Active            *bool             `json:"active"`            // User's active status
	Password          string            `json:"password"`          // User's password (only for creation/updates)
	Emails            []Email           `json:"emails"`            // Email addresses for the user
	PhoneNumbers      []PhoneNumber     `json:"phoneNumbers"`      // Phone numbers for the user
	Addresses         []Address         `json:"addresses"`         // Physical mailing addresses for the user
	Groups            []GroupRef        `json:"groups"`            // Groups that the user belongs to
	Photos            []Photo           `json:"photos"`            // URLs of user's photos
	Entitlements      []Entitlement     `json:"entitlements"`      // A list of entitlements
	Roles             []Role            `json:"roles"`             // A list of roles
	X509Certificates  []X509Certificate `json:"x509Certificates"`  // A list of certificates

	// SCIM specific meta information
	Meta *Meta `json:"meta"`

	// Local fields
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Revision  int64     `json:"revision"`
}

// Name represents the components of a user's name.
type Name struct {
	GivenName       string `json:"givenName"`
	FamilyName      string `json:"familyName"`
	MiddleName      string `json:"middleName"`
	HonorificPrefix string `json:"honorificPrefix"`
	HonorificSuffix string `json:"honorificSuffix"`
	Formatted       string `json:"formatted"` // The full name, including all middle names, titles, and suffixes
}

// Email represents an email address.
type Email struct {
	Value       string `json:"value"`       // Email address
	Type        string `json:"type"`        // "work", "home", "other"
	Primary     bool   `json:"primary"`     // Indicates this is the primary email
	DisplayName string `json:"displayName"` // Human-readable name
}

// PhoneNumber represents a phone number.
type PhoneNumber struct {
	Value       string `json:"value"`       // Phone number
	Type        string `json:"type"`        // "work", "home", "mobile", "fax", "pager", "other"
	Primary     bool   `json:"primary"`     // Indicates this is the primary phone number
	DisplayName string `json:"displayName"` // Human-readable name
}

// Address represents a physical mailing address.
type Address struct {
	StreetAddress string `json:"streetAddress"` // The full street address component
	Locality      string `json:"locality"`      // The city or locality component
	Region        string `json:"region"`        // The state or region component
	PostalCode    string `json:"postalCode"`    // The zip code or postal code component
	Country       string `json:"country"`       // The country name component
	Formatted     string `json:"formatted"`     // The full mailing address
	Type          string `json:"type"`          // "work", "home", "other"
	Primary       bool   `json:"primary"`       // Indicates this is the primary address
}

// GroupRef represents a reference to a group.
type GroupRef struct {
	Value   string `json:"value"`   // Group ID
	Display string `json:"display"` // Group display name
	Ref     string `json:"$ref"`    // Reference URI
	Type    string `json:"type"`    // "Group"
}

// Photo represents a user's photo.
type Photo struct {
	Value string `json:"value"` // Photo URL
	Type  string `json:"type"`  // "photo", "thumbnail"
}

// Entitlement represents a user's entitlement.
type Entitlement struct {
	Value string `json:"value"` // Entitlement value
	Type  string `json:"type"`  // Entitlement type
}

// Role represents a user's role.
type Role struct {
	Value string `json:"value"` // Role value
	Type  string `json:"type"`  // Role type
}

// X509Certificate represents a certificate associated with a user.
type X509Certificate struct {
	Value string `json:"value"` // PEM encoded certificate
}

// Meta represents SCIM resource metadata.
type Meta struct {
	ResourceType string    `json:"resourceType"` // The name of the resource type
	Location     string    `json:"location"`     // The URI of the resource
	Created      time.Time `json:"created"`      // The time the resource was added
	LastModified time.Time `json:"lastModified"` // The most recent time the details of the resource were changed
	Version      string    `json:"version"`      // The version of the resource being returned
}

// ScimGroup represents a SCIM Group resource mapped to a Group.
// See RFC 7643 Section 8.2: https://datatracker.ietf.org/doc/html/rfc7643#section-8.2
type ScimGroup struct {
	ID             string   `json:"id"`
	OrganizationID string   `json:"organizationId"`
	DisplayName    string   `json:"displayName"` // A human-readable name for the Group
	Members        []Member `json:"members"`     // A list of members of the Group
	Meta           *Meta    `json:"meta"`

	// Local fields
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Revision  int64     `json:"revision"`
}

// Member represents a group member.
type Member struct {
	Value   string `json:"value"`   // User ID
	Display string `json:"display"` // User display name
	Ref     string `json:"$ref"`    // Reference URI
	Type    string `json:"type"`    // "User"
}

// PatchOperation represents a SCIM PATCH operation.
// See RFC 7644 Section 3.5.2: https://datatracker.ietf.org/doc/html/rfc7644#section-3.5.2
type PatchOperation struct {
	Op    string      `json:"op"`    // "add", "remove", "replace"
	Path  string      `json:"path"`  // SCIM path (e.g., "userName", "emails[type eq 'work']")
	Value interface{} `json:"value"` // New value
}

// Standard SCIM Schema IDs
const (
	SchemaUser           = "urn:ietf:params:scim:schemas:core:2.0:User"
	SchemaGroup          = "urn:ietf:params:scim:schemas:core:2.0:Group"
	SchemaEnterpriseUser = "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"
)

// NewScimUser creates a new SCIM User with defaults.
func NewScimUser(organizationID, username string) (*ScimUser, error) {
	now := time.Now()
	id, err := uuid.V7Generator{}.NewString()
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "generate scim user id")
	}
	return &ScimUser{
		ID:             id,
		OrganizationID: organizationID,
		UserName:       username,
		Active:         boolPtr(true),
		Meta: &Meta{
			ResourceType: "User",
			Created:      now,
			LastModified: now,
		},
		CreatedAt: now,
		UpdatedAt: now,
		Revision:  1,
	}, nil
}

// NewScimGroup creates a new SCIM Group with defaults.
func NewScimGroup(organizationID, displayName string) (*ScimGroup, error) {
	now := time.Now()
	id, err := uuid.V7Generator{}.NewString()
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "generate scim group id")
	}
	return &ScimGroup{
		ID:             id,
		OrganizationID: organizationID,
		DisplayName:    displayName,
		Members:        []Member{},
		Meta: &Meta{
			ResourceType: "Group",
			Created:      now,
			LastModified: now,
		},
		CreatedAt: now,
		UpdatedAt: now,
		Revision:  1,
	}, nil
}

// Helper functions

func boolPtr(b bool) *bool {
	return &b
}
