package adapters

import (
	"context"

	appadapters "github.com/guliangiam/guliangiam/internal/app/adapters"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct{ Service appadapters.Service }

func NewHandler(s appadapters.Service) Handler { return Handler{Service: s} }

type ListAdaptersResponse = presenter.ListResponse[appadapters.AdapterListItem]
type AdapterResponse = appadapters.AdapterDetail

type ListAdaptersRequest struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	IsEnabled               *bool
	DatabaseType            string
}
type GetAdapterRequest struct{ ID string }
type CreateAdapterRequest struct {
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	Description    string `json:"description,omitempty"`
	Type           string `json:"type,omitempty"`
	Server         string `json:"server,omitempty"`
	Database       string `json:"database,omitempty"`
	TableName      string `json:"tableName,omitempty"`
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
	// Phase 2 fields
	Owner        string `json:"owner,omitempty"`
	UseSameDb    bool   `json:"useSameDb,omitempty"`
	DatabaseType string `json:"databaseType,omitempty"`
	Host         string `json:"host,omitempty"`
	Port         *int   `json:"port,omitempty"`
	DbUser       string `json:"dbUser,omitempty"`
	DbPassword   string `json:"dbPassword,omitempty"`
	DatabaseName string `json:"databaseName,omitempty"`
	Policy       string `json:"policy,omitempty"`
	IsEnabled    bool   `json:"isEnabled,omitempty"`
}
type PatchAdapterRequest struct {
	DisplayName *string `json:"displayName,omitempty"`
	Description *string `json:"description,omitempty"`
	Type        *string `json:"type,omitempty"`
	Server      *string `json:"server,omitempty"`
	Database    *string `json:"database,omitempty"`
	TableName   *string `json:"tableName,omitempty"`
	Username    *string `json:"username,omitempty"`
	Password    *string `json:"password,omitempty"`
	Revision    int64   `json:"revision,omitempty"`
	// Phase 2 fields
	Owner        *string `json:"owner,omitempty"`
	UseSameDb    *bool   `json:"useSameDb,omitempty"`
	DatabaseType *string `json:"databaseType,omitempty"`
	Host         *string `json:"host,omitempty"`
	Port         *int    `json:"port,omitempty"`
	DbUser       *string `json:"dbUser,omitempty"`
	DbPassword   *string `json:"dbPassword,omitempty"`
	DatabaseName *string `json:"databaseName,omitempty"`
	Policy       *string `json:"policy,omitempty"`
	IsEnabled    *bool   `json:"isEnabled,omitempty"`
}

func (h Handler) List(ctx context.Context, req ListAdaptersRequest) (ListAdaptersResponse, error) {
	items, total, err := h.Service.ListAdapters(ctx, appadapters.ListAdaptersQuery(req))
	if err != nil {
		return ListAdaptersResponse{}, err
	}
	return presenter.NewListResponse(items, req.Page, req.PageSize, total), nil
}
func (h Handler) Get(ctx context.Context, organizationID string, req GetAdapterRequest) (*AdapterResponse, error) {
	r, err := h.Service.GetAdapter(ctx, organizationID, req.ID)
	if err != nil {
		return nil, err
	}
	t := AdapterResponse(*r)
	return &t, nil
}
func (h Handler) Create(ctx context.Context, req CreateAdapterRequest) error {
	return h.Service.CreateAdapter(ctx, appadapters.CreateAdapterCommand{
		OrganizationID: req.OrganizationID, Name: req.Name, DisplayName: req.DisplayName,
		Description: req.Description, Type: req.Type, Server: req.Server,
		Database: req.Database, TableName: req.TableName, Username: req.Username, Password: req.Password,
		Owner: req.Owner, UseSameDb: req.UseSameDb, DatabaseType: req.DatabaseType,
		Host: req.Host, Port: req.Port, DbUser: req.DbUser, DbPassword: req.DbPassword,
		DatabaseName: req.DatabaseName, Policy: req.Policy, IsEnabled: req.IsEnabled,
	})
}
func (h Handler) Patch(ctx context.Context, organizationID, id string, req PatchAdapterRequest) error {
	return h.Service.PatchAdapter(ctx, organizationID, appadapters.PatchAdapterCommand{
		ID: id, DisplayName: req.DisplayName, Description: req.Description, Type: req.Type,
		Server: req.Server, Database: req.Database, TableName: req.TableName,
		Username: req.Username, Password: req.Password, Revision: req.Revision,
		Owner: req.Owner, UseSameDb: req.UseSameDb, DatabaseType: req.DatabaseType,
		Host: req.Host, Port: req.Port, DbUser: req.DbUser, DbPassword: req.DbPassword,
		DatabaseName: req.DatabaseName, Policy: req.Policy, IsEnabled: req.IsEnabled,
	})
}
func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteAdapter(ctx, organizationID, id)
}

type TestConnectionRequest struct {
	DatabaseType string `json:"databaseType"`
	Host         string `json:"host"`
	Port         int    `json:"port,omitempty"`
	Username     string `json:"username,omitempty"`
	Password     string `json:"password,omitempty"`
	DatabaseName string `json:"databaseName,omitempty"`
}
type TestConnectionResponse struct {
	Connected bool   `json:"connected"`
	Error     string `json:"error,omitempty"`
}

func (h Handler) TestConnection(ctx context.Context, organizationID, id string, req TestConnectionRequest) (*TestConnectionResponse, error) {
	result, err := h.Service.TestConnection(ctx, organizationID, id, appadapters.TestConnectionCommand{
		DatabaseType: req.DatabaseType, Host: req.Host, Port: req.Port,
		Username: req.Username, Password: req.Password, DatabaseName: req.DatabaseName,
	})
	if err != nil {
		return nil, err
	}
	return &TestConnectionResponse{Connected: result.Connected, Error: result.Error}, nil
}

// ─── Policy operations ──────────────────────────────────────────

type PolicyRequest struct {
	PType string   `json:"pType"`
	Rules []string `json:"rules"`
}

type UpdatePolicyRequest struct {
	OldPolicy []string `json:"oldPolicy"`
	NewPolicy []string `json:"newPolicy"`
}

func (h Handler) GetPolicies(ctx context.Context, organizationID, id string) ([][]string, error) {
	return h.Service.GetPolicies(ctx, organizationID, id)
}
func (h Handler) GetFilteredPolicies(ctx context.Context, organizationID, id string, fieldIndex int, fieldValues []string) ([][]string, error) {
	return h.Service.GetFilteredPolicies(ctx, organizationID, id, fieldIndex, fieldValues...)
}
func (h Handler) AddPolicy(ctx context.Context, organizationID, id string, req PolicyRequest) error {
	return h.Service.AddPolicy(ctx, organizationID, id, append([]string{req.PType}, req.Rules...))
}
func (h Handler) RemovePolicy(ctx context.Context, organizationID, id string, req PolicyRequest) error {
	return h.Service.RemovePolicy(ctx, organizationID, id, append([]string{req.PType}, req.Rules...))
}
func (h Handler) UpdatePolicy(ctx context.Context, organizationID, id string, req UpdatePolicyRequest) error {
	return h.Service.UpdatePolicy(ctx, organizationID, id, req.OldPolicy, req.NewPolicy)
}
