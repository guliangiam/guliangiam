package keys

import (
	"context"
	"time"

	appkeys "github.com/guliangiam/guliangiam/internal/app/keys"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appkeys.Service
}

func NewHandler(s appkeys.Service) Handler {
	return Handler{Service: s}
}

type ListKeysResponse = presenter.ListResponse[appkeys.KeyListItem]
type KeyResponse = appkeys.KeyDetail

type ListKeysRequest struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	KeyType                 *string
	IsEnabled               *bool
}

type CreateKeyRequest struct {
	OrganizationID string     `json:"organizationId"`
	Name           string     `json:"name"`
	DisplayName    string     `json:"displayName"`
	KeyType        string     `json:"keyType"`
	PublicKey      string     `json:"publicKey"`
	PrivateKey     string     `json:"privateKey,omitempty"`
	Algorithm      string     `json:"algorithm,omitempty"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
	IsEnabled      bool       `json:"isEnabled"`
}

type PatchKeyRequest struct {
	DisplayName *string    `json:"displayName,omitempty"`
	PublicKey   *string    `json:"publicKey,omitempty"`
	PrivateKey  *string    `json:"privateKey,omitempty"`
	Algorithm   *string    `json:"algorithm,omitempty"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	IsEnabled   *bool      `json:"isEnabled,omitempty"`
	Revision    int64      `json:"revision,omitempty"`
}

func (h Handler) List(ctx context.Context, request ListKeysRequest) (ListKeysResponse, error) {
	items, total, err := h.Service.ListKeys(ctx, appkeys.ListKeysQuery{
		Page: request.Page, PageSize: request.PageSize, OrganizationID: request.OrganizationID,
		Keyword: request.Keyword, KeyType: request.KeyType, IsEnabled: request.IsEnabled,
	})
	if err != nil {
		return ListKeysResponse{}, err
	}
	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID, id string) (*KeyResponse, error) {
	result, err := h.Service.GetKey(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	typed := KeyResponse(*result)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateKeyRequest) error {
	return h.Service.CreateKey(ctx, appkeys.CreateKeyCommand{
		OrganizationID: request.OrganizationID, Name: request.Name, DisplayName: request.DisplayName,
		KeyType: request.KeyType, PublicKey: request.PublicKey, PrivateKey: request.PrivateKey,
		Algorithm: request.Algorithm, ExpiresAt: request.ExpiresAt, IsEnabled: request.IsEnabled,
	})
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchKeyRequest) error {
	return h.Service.PatchKey(ctx, organizationID, appkeys.PatchKeyCommand{
		OrganizationID: organizationID, ID: id, DisplayName: request.DisplayName, PublicKey: request.PublicKey,
		PrivateKey: request.PrivateKey, Algorithm: request.Algorithm, ExpiresAt: request.ExpiresAt,
		IsEnabled: request.IsEnabled, Revision: request.Revision,
	})
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteKey(ctx, organizationID, id)
}
