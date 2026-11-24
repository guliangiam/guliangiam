package syncer

import (
	"encoding/json"
	"net/http"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// SCIMSyncer syncs users and groups via SCIM 2.0 protocol.
// This is a "push" syncer: it pushes local users/groups to an external SCIM endpoint.
type SCIMSyncer struct {
	client *http.Client
}

// NewSCIMSyncer creates a new SCIM syncer.
func NewSCIMSyncer() *SCIMSyncer {
	return &SCIMSyncer{
		client: ssrf.SafeHTTPClient(30 * time.Second),
	}
}

// --- SCIM 2.0 Data Types ---

// scimUser represents a SCIM 2.0 User resource.
type scimUser struct {
	Schemas     []string    `json:"schemas"`
	ID          string      `json:"id,omitempty"`
	UserName    string      `json:"userName"`
	DisplayName string      `json:"displayName,omitempty"`
	Name        *scimName   `json:"name,omitempty"`
	Emails      []scimEmail `json:"emails,omitempty"`
	Active      *bool       `json:"active,omitempty"`
	Title       string      `json:"title,omitempty"`
	ExternalID  string      `json:"externalId,omitempty"`
}

type scimName struct {
	GivenName  string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
	Formatted  string `json:"formatted,omitempty"`
}

type scimEmail struct {
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"` // work, home, etc.
	Primary bool   `json:"primary,omitempty"`
}

// scimGroup represents a SCIM 2.0 Group resource.
type scimGroup struct {
	Schemas     []string     `json:"schemas"`
	ID          string       `json:"id,omitempty"`
	DisplayName string       `json:"displayName"`
	Members     []scimMember `json:"members,omitempty"`
	ExternalID  string       `json:"externalId,omitempty"`
}

type scimMember struct {
	Value   string `json:"value"`
	Display string `json:"display,omitempty"`
}

// scimListResponse is a generic SCIM list response.
type scimListResponse struct {
	TotalResults int             `json:"totalResults"`
	ItemsPerPage int             `json:"itemsPerPage"`
	StartIndex   int             `json:"startIndex"`
	Resources    json.RawMessage `json:"Resources"`
}

// scimPatchRequest represents a SCIM PATCH request.
type scimPatchRequest struct {
	Schemas    []string      `json:"schemas"`
	Operations []scimPatchOp `json:"Operations"`
}

type scimPatchOp struct {
	Op    string      `json:"op"`
	Path  string      `json:"path,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

// SyncUsers pushes users to the SCIM endpoint.
// For each user, it first tries to find an existing user by userName (GET with filter),
// then either updates (PUT/PATCH) or creates (POST) the user.
