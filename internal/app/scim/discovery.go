package scim

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ServiceProviderConfig represents the SCIM 2.0 Service Provider Configuration.
type ServiceProviderConfig struct {
	Schemas               []string                   `json:"schemas"`
	ID                    string                     `json:"id,omitempty"`
	Name                  string                     `json:"name,omitempty"`
	Patch                 *SCIMConfigFeature         `json:"patch"`
	Bulk                  *SCIMBulkFeature           `json:"bulk"`
	Filter                *SCIMFilterFeature         `json:"filter"`
	ChangePassword        *SCIMConfigFeature         `json:"changePassword"`
	Sort                  *SCIMConfigFeature         `json:"sort"`
	ETag                  *SCIMConfigFeature         `json:"etag"`
	AuthenticationSchemes []SCIMAuthenticationScheme `json:"authenticationSchemes"`
}

type SCIMConfigFeature struct {
	Supported bool `json:"supported"`
}

type SCIMBulkFeature struct {
	Supported      bool `json:"supported"`
	MaxOperations  int  `json:"maxOperations"`
	MaxPayloadSize int  `json:"maxPayloadSize"`
}

type SCIMFilterFeature struct {
	Supported  bool `json:"supported"`
	MaxResults int  `json:"maxResults"`
}

type SCIMAuthenticationScheme struct {
	Type             string `json:"type"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	SpecURI          string `json:"specUri,omitempty"`
	DocumentationURI string `json:"documentationUri,omitempty"`
	Primary          bool   `json:"primary"`
}

// ResourceType represents a SCIM 2.0 Resource Type.
type ResourceType struct {
	Schemas     []string `json:"schemas"`
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Endpoint    string   `json:"endpoint"`
	Description string   `json:"description"`
	Schema      string   `json:"schema"`
}

// Schema represents a SCIM 2.0 Schema.
type Schema struct {
	Schemas     []string          `json:"schemas"`
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Attributes  []SchemaAttribute `json:"attributes"`
}

type SchemaAttribute struct {
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	MultiValued    bool              `json:"multiValued"`
	Description    string            `json:"description"`
	Required       bool              `json:"required"`
	CaseExact      bool              `json:"caseExact"`
	Mutability     string            `json:"mutability"`
	Returned       string            `json:"returned"`
	Uniqueness     string            `json:"uniqueness"`
	SubAttributes  []SchemaAttribute `json:"subAttributes,omitempty"`
	ReferenceTypes []string          `json:"referenceTypes,omitempty"`
}

// HandleServiceProviderConfig returns the SCIM 2.0 Service Provider Configuration.
func HandleServiceProviderConfig(w http.ResponseWriter, r *http.Request) {
	config := ServiceProviderConfig{
		Schemas: []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		Patch:   &SCIMConfigFeature{Supported: true},
		Bulk: &SCIMBulkFeature{
			Supported:      false,
			MaxOperations:  0,
			MaxPayloadSize: 0,
		},
		Filter: &SCIMFilterFeature{
			Supported:  true,
			MaxResults: 100,
		},
		// 密码管理走管理面 API，SCIM 面不提供改密操作（design/protocol/scim.md）。
		ChangePassword: &SCIMConfigFeature{Supported: false},
		// sortBy/sortOrder 已由 resolveScimSort + 仓储白名单消费
		//（scim.md「ServiceProviderConfig 宣告约束」；OPEN-API-87 已关闭）。
		Sort: &SCIMConfigFeature{Supported: true},
		ETag: &SCIMConfigFeature{Supported: false},
		// 仅宣告 Bearer：BearerTokenAuth 中间件只接受 Bearer、拒绝 Basic，
		// 宣告 httpbasic 会误导按发现文档自动选认证方式的客户端。
		AuthenticationSchemes: []SCIMAuthenticationScheme{
			{
				Type:        "oauthbearertoken",
				Name:        "OAuth Bearer Token",
				Description: "Authentication via OAuth 2.0 Bearer Token",
				Primary:     true,
			},
		},
	}

	w.Header().Set("Content-Type", "application/scim+json")
	if err := json.NewEncoder(w).Encode(config); err != nil {
		slog.ErrorContext(r.Context(), "scim discovery: encode service provider config", "error", err)
	}
}

// HandleResourceTypes returns the SCIM 2.0 Resource Types.
func HandleResourceTypes(w http.ResponseWriter, r *http.Request) {
	resourceTypes := []ResourceType{
		{
			Schemas:     []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"},
			ID:          resourceTypeUser,
			Name:        resourceTypeUser,
			Endpoint:    "/scim/v2/Users",
			Description: "User Account",
			Schema:      "urn:ietf:params:scim:schemas:core:2.0:User",
		},
		{
			Schemas:     []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"},
			ID:          resourceTypeGroup,
			Name:        resourceTypeGroup,
			Endpoint:    "/scim/v2/Groups",
			Description: resourceTypeGroup,
			Schema:      "urn:ietf:params:scim:schemas:core:2.0:Group",
		},
	}

	w.Header().Set("Content-Type", "application/scim+json")
	if err := json.NewEncoder(w).Encode(resourceTypes); err != nil {
		slog.ErrorContext(r.Context(), "scim discovery: encode resource types", "error", err)
	}
}

// HandleSchemas returns the SCIM 2.0 Schemas.
func HandleSchemas(w http.ResponseWriter, r *http.Request) {
	schemas := []Schema{
		{
			Schemas:     []string{"urn:ietf:params:scim:schemas:core:2.0:Schema"},
			ID:          "urn:ietf:params:scim:schemas:core:2.0:User",
			Name:        resourceTypeUser,
			Description: "User Account",
			Attributes: []SchemaAttribute{
				{Name: scimAttrUserName, Type: schemaTypeString, Required: true, Mutability: mutabilityReadWrite, Returned: returnedDefault, Uniqueness: "server"},
				{Name: "name", Type: "complex", Mutability: mutabilityReadWrite, Returned: returnedDefault, SubAttributes: []SchemaAttribute{
					{Name: "givenName", Type: schemaTypeString, Mutability: mutabilityReadWrite, Returned: returnedDefault},
					{Name: "familyName", Type: schemaTypeString, Mutability: mutabilityReadWrite, Returned: returnedDefault},
				}},
				{Name: "displayName", Type: schemaTypeString, Mutability: mutabilityReadWrite, Returned: returnedDefault},
				{Name: scimAttrEmails, Type: schemaTypeString, MultiValued: true, Mutability: mutabilityReadWrite, Returned: returnedDefault},
				{Name: scimAttrActive, Type: "boolean", Mutability: mutabilityReadWrite, Returned: returnedDefault},
			},
		},
		{
			Schemas:     []string{"urn:ietf:params:scim:schemas:core:2.0:Schema"},
			ID:          "urn:ietf:params:scim:schemas:core:2.0:Group",
			Name:        resourceTypeGroup,
			Description: resourceTypeGroup,
			Attributes: []SchemaAttribute{
				{Name: "displayName", Type: schemaTypeString, Mutability: mutabilityReadWrite, Returned: returnedDefault},
				{Name: scimAttrMembers, Type: "complex", MultiValued: true, Mutability: mutabilityReadWrite, Returned: returnedDefault, SubAttributes: []SchemaAttribute{
					{Name: "value", Type: schemaTypeString, Description: "ID of the member", Mutability: "immutable", Returned: returnedDefault},
					{Name: "display", Type: schemaTypeString, Mutability: mutabilityReadWrite, Returned: returnedDefault},
				}},
			},
		},
	}

	w.Header().Set("Content-Type", "application/scim+json")
	if err := json.NewEncoder(w).Encode(schemas); err != nil {
		slog.ErrorContext(r.Context(), "scim discovery: encode schemas", "error", err)
	}
}
