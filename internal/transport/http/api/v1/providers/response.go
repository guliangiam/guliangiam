package providers

import (
	appproviders "github.com/guliangiam/guliangiam/internal/app/providers"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type ListProvidersResponse = presenter.ListResponse[appproviders.ProviderListItem]
type ProviderResponse = appproviders.ProviderDetail
type ProviderConfigResponse = appproviders.ProviderConfigDetail

type ProviderSecretStatusListResponse struct {
	Items []appproviders.ProviderSecretStatusItem `json:"items"`
}

type TestConnectionResponse struct {
	Connected bool   `json:"connected"`
	Error     string `json:"error,omitempty"`
}
