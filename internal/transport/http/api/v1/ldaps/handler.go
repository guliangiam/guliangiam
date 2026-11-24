package ldaps

import (
	"context"

	appldaps "github.com/guliangiam/guliangiam/internal/app/ldaps"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appldaps.Service
}

func NewHandler(s appldaps.Service) Handler {
	return Handler{Service: s}
}

type ListLdapsResponse = presenter.ListResponse[appldaps.LdapListItem]
type LdapResponse = appldaps.LdapDetail

type ListLdapsRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

type GetLdapRequest struct {
	ID string
}

type CreateLdapRequest struct {
	OrganizationID      string            `json:"organizationId"`
	Name                string            `json:"name"`
	DisplayName         string            `json:"displayName"`
	ServerName          string            `json:"serverName,omitempty"`
	Host                string            `json:"host"`
	Port                int               `json:"port"`
	Admin               string            `json:"admin,omitempty"`
	BindDN              string            `json:"bindDn,omitempty"`
	BindPassword        string            `json:"bindPassword,omitempty"`
	BaseDN              string            `json:"baseDn,omitempty"`
	Type                string            `json:"type,omitempty"`
	AttributeName       string            `json:"attributeName,omitempty"`
	PasswordType        string            `json:"passwordType,omitempty"`
	Filter              string            `json:"filter,omitempty"`
	FilterFields        []string          `json:"filterFields,omitempty"`
	EnableSsl           bool              `json:"enableSsl"`
	AllowSelfSignedCert bool              `json:"allowSelfSignedCert"`
	EnableGroups        bool              `json:"enableGroups"`
	DefaultGroups       []string          `json:"defaultGroups,omitempty"`
	CustomAttributes    map[string]string `json:"customAttributes,omitempty"`
	IsEnabled           *bool             `json:"isEnabled,omitempty"`
	SyncInterval        int               `json:"syncInterval,omitempty"`
}

type PatchLdapRequest struct {
	DisplayName         *string           `json:"displayName,omitempty"`
	ServerName          *string           `json:"serverName,omitempty"`
	Host                *string           `json:"host,omitempty"`
	Port                *int              `json:"port,omitempty"`
	Admin               *string           `json:"admin,omitempty"`
	BindDN              *string           `json:"bindDn,omitempty"`
	BindPassword        *string           `json:"bindPassword,omitempty"`
	BaseDN              *string           `json:"baseDn,omitempty"`
	Type                *string           `json:"type,omitempty"`
	AttributeName       *string           `json:"attributeName,omitempty"`
	PasswordType        *string           `json:"passwordType,omitempty"`
	Filter              *string           `json:"filter,omitempty"`
	FilterFields        []string          `json:"filterFields,omitempty"`
	EnableSsl           *bool             `json:"enableSsl,omitempty"`
	AllowSelfSignedCert *bool             `json:"allowSelfSignedCert,omitempty"`
	EnableGroups        *bool             `json:"enableGroups,omitempty"`
	DefaultGroups       []string          `json:"defaultGroups,omitempty"`
	CustomAttributes    map[string]string `json:"customAttributes,omitempty"`
	IsEnabled           *bool             `json:"isEnabled,omitempty"`
	SyncInterval        *int              `json:"syncInterval,omitempty"`
	Revision            int64             `json:"revision,omitempty"`
}

func (r CreateLdapRequest) toCommand() appldaps.CreateLdapCommand {
	return appldaps.CreateLdapCommand{
		OrganizationID:      r.OrganizationID,
		Name:                r.Name,
		DisplayName:         r.DisplayName,
		ServerName:          r.ServerName,
		Host:                r.Host,
		Port:                r.Port,
		Admin:               r.Admin,
		BindDN:              r.BindDN,
		BindPassword:        r.BindPassword,
		BaseDN:              r.BaseDN,
		Type:                r.Type,
		AttributeName:       r.AttributeName,
		PasswordType:        r.PasswordType,
		Filter:              r.Filter,
		FilterFields:        r.FilterFields,
		EnableSsl:           r.EnableSsl,
		AllowSelfSignedCert: r.AllowSelfSignedCert,
		EnableGroups:        r.EnableGroups,
		DefaultGroups:       r.DefaultGroups,
		CustomAttributes:    r.CustomAttributes,
		IsEnabled:           r.IsEnabled,
		SyncInterval:        r.SyncInterval,
	}
}

func (r PatchLdapRequest) toCommand(id string) appldaps.PatchLdapCommand {
	return appldaps.PatchLdapCommand{
		ID:                  id,
		DisplayName:         r.DisplayName,
		ServerName:          r.ServerName,
		Host:                r.Host,
		Port:                r.Port,
		Admin:               r.Admin,
		BindDN:              r.BindDN,
		BindPassword:        r.BindPassword,
		BaseDN:              r.BaseDN,
		Type:                r.Type,
		AttributeName:       r.AttributeName,
		PasswordType:        r.PasswordType,
		Filter:              r.Filter,
		FilterFields:        r.FilterFields,
		EnableSsl:           r.EnableSsl,
		AllowSelfSignedCert: r.AllowSelfSignedCert,
		EnableGroups:        r.EnableGroups,
		DefaultGroups:       r.DefaultGroups,
		CustomAttributes:    r.CustomAttributes,
		IsEnabled:           r.IsEnabled,
		SyncInterval:        r.SyncInterval,
		Revision:            r.Revision,
	}
}

func (h Handler) List(ctx context.Context, request ListLdapsRequest) (ListLdapsResponse, error) {
	items, total, err := h.Service.ListLdaps(ctx, appldaps.ListLdapsQuery(request))
	if err != nil {
		return ListLdapsResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID string, request GetLdapRequest) (*LdapResponse, error) {
	response, err := h.Service.GetLdap(ctx, request.ID, organizationID)
	if err != nil {
		return nil, err
	}

	typed := LdapResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateLdapRequest) error {
	return h.Service.CreateLdap(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchLdapRequest) error {
	return h.Service.PatchLdap(ctx, organizationID, request.toCommand(id))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteLdap(ctx, organizationID, id)
}

type SyncLdapRequest struct {
	UUIDs []string `json:"uuids,omitempty"` // empty = sync all users
}

func (h Handler) Sync(ctx context.Context, organizationID, id string, request SyncLdapRequest) (*appldaps.LDAPSyncResult, error) {
	return h.Service.SyncLdap(ctx, id, request.UUIDs, organizationID)
}

func (h Handler) PreviewUsers(ctx context.Context, organizationID, id string) (*appldaps.LDAPPreviewUserResult, error) {
	return h.Service.PreviewUsers(ctx, id, organizationID)
}

func (h Handler) TestConnection(ctx context.Context, organizationID, id string) error {
	return h.Service.TestConnection(ctx, id, organizationID)
}
