package syncers

import (
	"context"

	appsyncers "github.com/guliangiam/guliangiam/internal/app/syncers"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appsyncers.Service
}

func NewHandler(s appsyncers.Service) Handler {
	return Handler{Service: s}
}

type ListSyncersResponse = presenter.ListResponse[appsyncers.SyncerListItem]
type SyncerResponse = appsyncers.SyncerDetail

type ListSyncersRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

type CreateSyncerRequest struct {
	OrganizationID   string `json:"organizationId"`
	Name             string `json:"name"`
	DisplayName      string `json:"displayName"`
	Type             string `json:"type"`
	DatabaseType     string `json:"databaseType,omitempty"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	Database         string `json:"database,omitempty"`
	TableName        string `json:"tableName,omitempty"`
	Username         string `json:"username,omitempty"`
	Password         string `json:"password,omitempty"`
	SslMode          string `json:"sslMode,omitempty"`
	SshType          string `json:"sshType,omitempty"`
	SshHost          string `json:"sshHost,omitempty"`
	SshPort          int    `json:"sshPort,omitempty"`
	SshUsername      string `json:"sshUsername,omitempty"`
	SshPassword      string `json:"sshPassword,omitempty"`
	SshCertID        string `json:"sshCertId,omitempty"`
	IsEnabled        *bool  `json:"isEnabled,omitempty"`
	SyncInterval     int    `json:"syncInterval,omitempty"`
	FieldMapping     string `json:"fieldMapping,omitempty"`
	ColumnMapping    string `json:"columnMapping,omitempty"`
	IsReadOnly       bool   `json:"isReadOnly,omitempty"`
	Affinity         string `json:"affinity,omitempty"`
	AffiliationTable string `json:"affiliationTable,omitempty"`
	AvatarBaseUrl    string `json:"avatarBaseUrl,omitempty"`
	ErrorText        string `json:"errorText,omitempty"`
}

type PatchSyncerRequest struct {
	DisplayName      *string `json:"displayName,omitempty"`
	Host             *string `json:"host,omitempty"`
	Port             *int    `json:"port,omitempty"`
	Database         *string `json:"database,omitempty"`
	TableName        *string `json:"tableName,omitempty"`
	Username         *string `json:"username,omitempty"`
	Password         *string `json:"password,omitempty"`
	IsEnabled        *bool   `json:"isEnabled,omitempty"`
	SyncInterval     *int    `json:"syncInterval,omitempty"`
	FieldMapping     *string `json:"fieldMapping,omitempty"`
	DatabaseType     *string `json:"databaseType,omitempty"`
	SslMode          *string `json:"sslMode,omitempty"`
	SshType          *string `json:"sshType,omitempty"`
	SshHost          *string `json:"sshHost,omitempty"`
	SshPort          *int    `json:"sshPort,omitempty"`
	SshUsername      *string `json:"sshUsername,omitempty"`
	SshPassword      *string `json:"sshPassword,omitempty"`
	SshCertID        *string `json:"sshCertId,omitempty"`
	ColumnMapping    *string `json:"columnMapping,omitempty"`
	IsReadOnly       *bool   `json:"isReadOnly,omitempty"`
	Affinity         *string `json:"affinity,omitempty"`
	AffiliationTable *string `json:"affiliationTable,omitempty"`
	AvatarBaseUrl    *string `json:"avatarBaseUrl,omitempty"`
	ErrorText        *string `json:"errorText,omitempty"`
	Revision         int64   `json:"revision,omitempty"`
}

func (r CreateSyncerRequest) toCommand() appsyncers.CreateSyncerCommand {
	return appsyncers.CreateSyncerCommand{
		OrganizationID:   r.OrganizationID,
		Name:             r.Name,
		DisplayName:      r.DisplayName,
		Type:             r.Type,
		DatabaseType:     r.DatabaseType,
		Host:             r.Host,
		Port:             r.Port,
		Database:         r.Database,
		TableName:        r.TableName,
		Username:         r.Username,
		Password:         r.Password,
		SslMode:          r.SslMode,
		SshType:          r.SshType,
		SshHost:          r.SshHost,
		SshPort:          r.SshPort,
		SshUsername:      r.SshUsername,
		SshPassword:      r.SshPassword,
		SshCertID:        r.SshCertID,
		IsEnabled:        r.IsEnabled,
		SyncInterval:     r.SyncInterval,
		FieldMapping:     r.FieldMapping,
		ColumnMapping:    r.ColumnMapping,
		IsReadOnly:       r.IsReadOnly,
		Affinity:         r.Affinity,
		AffiliationTable: r.AffiliationTable,
		AvatarBaseUrl:    r.AvatarBaseUrl,
		ErrorText:        r.ErrorText,
	}
}

func (r PatchSyncerRequest) toCommand(id string) appsyncers.PatchSyncerCommand {
	return appsyncers.PatchSyncerCommand{
		ID:               id,
		DisplayName:      r.DisplayName,
		Host:             r.Host,
		Port:             r.Port,
		Database:         r.Database,
		TableName:        r.TableName,
		Username:         r.Username,
		Password:         r.Password,
		IsEnabled:        r.IsEnabled,
		SyncInterval:     r.SyncInterval,
		FieldMapping:     r.FieldMapping,
		DatabaseType:     r.DatabaseType,
		SslMode:          r.SslMode,
		SshType:          r.SshType,
		SshHost:          r.SshHost,
		SshPort:          r.SshPort,
		SshUsername:      r.SshUsername,
		SshPassword:      r.SshPassword,
		SshCertID:        r.SshCertID,
		ColumnMapping:    r.ColumnMapping,
		IsReadOnly:       r.IsReadOnly,
		Affinity:         r.Affinity,
		AffiliationTable: r.AffiliationTable,
		AvatarBaseUrl:    r.AvatarBaseUrl,
		ErrorText:        r.ErrorText,
		Revision:         r.Revision,
	}
}

func (h Handler) List(ctx context.Context, request ListSyncersRequest) (ListSyncersResponse, error) {
	items, total, err := h.Service.ListSyncers(ctx, appsyncers.ListSyncersQuery(request))
	if err != nil {
		return ListSyncersResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID, id string) (*SyncerResponse, error) {
	response, err := h.Service.GetSyncer(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}

	typed := SyncerResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateSyncerRequest) error {
	return h.Service.CreateSyncer(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchSyncerRequest) error {
	return h.Service.PatchSyncer(ctx, organizationID, request.toCommand(id))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteSyncer(ctx, organizationID, id)
}

func (h Handler) Run(ctx context.Context, organizationID, id string) (*appsyncers.SyncerRunResult, error) {
	return h.Service.RunSyncerForOrganization(ctx, organizationID, id)
}

func (h Handler) TestConnection(ctx context.Context, organizationID, id string) error {
	return h.Service.TestConnectionForOrganization(ctx, organizationID, id)
}
