package gateway

import (
	"context"

	appgateway "github.com/guliangiam/guliangiam/internal/app/gateway"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appgateway.Service
}

func (h Handler) ListServers(ctx context.Context, request ListServersRequest) (ListServersResponse, error) {
	items, total, err := h.Service.ListServers(ctx, request.toQuery())
	if err != nil {
		return ListServersResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) GetServer(ctx context.Context, organizationID, id string) (*ServerResponse, error) {
	server, err := h.Service.GetServer(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}

	return toServerResponse(server), nil
}

func (h Handler) CreateServer(ctx context.Context, request CreateServerRequest) (*CreateServerResponse, error) {
	server, err := h.Service.CreateServer(ctx, request.toCommand())
	if err != nil {
		return nil, err
	}

	return &CreateServerResponse{ID: server.ID, Status: "created"}, nil
}

func (h Handler) UpdateServer(ctx context.Context, organizationID string, request UpdateServerRequest) error {
	return h.Service.UpdateServer(ctx, organizationID, request.toCommand())
}

func (h Handler) DeleteServer(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteServer(ctx, organizationID, id)
}

func (h Handler) ListSites(ctx context.Context, request ListSitesRequest) (ListSitesResponse, error) {
	items, total, err := h.Service.ListSites(ctx, request.toQuery())
	if err != nil {
		return ListSitesResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) GetSite(ctx context.Context, organizationID, id string) (*SiteResponse, error) {
	site, err := h.Service.GetSite(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}

	return toSiteResponse(site), nil
}

func (h Handler) CreateSite(ctx context.Context, request CreateSiteRequest) (*CreateSiteResponse, error) {
	site, err := h.Service.CreateSite(ctx, request.toCommand())
	if err != nil {
		return nil, err
	}

	return &CreateSiteResponse{ID: site.ID, Status: "created"}, nil
}

func (h Handler) UpdateSite(ctx context.Context, organizationID string, request UpdateSiteRequest) error {
	return h.Service.UpdateSite(ctx, organizationID, request.toCommand())
}

func (h Handler) DeleteSite(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteSite(ctx, organizationID, id)
}

// Site SSL/Health/Alert handlers

func (h Handler) ProvisionSiteCert(ctx context.Context, organizationID, siteID string) (*appgateway.ProvisionCertResult, error) {
	return h.Service.ProvisionCert(ctx, organizationID, siteID)
}

func (h Handler) RenewSiteCert(ctx context.Context, organizationID, siteID string) (*appgateway.ProvisionCertResult, error) {
	return h.Service.RenewCert(ctx, organizationID, siteID)
}

func (h Handler) GetSiteCertStatus(ctx context.Context, organizationID, siteID string) (*appgateway.CertStatusResult, error) {
	return h.Service.GetCertStatus(ctx, organizationID, siteID)
}

func (h Handler) CheckSiteHealth(ctx context.Context, organizationID, siteID string) (*appgateway.HealthCheckResult, error) {
	return h.Service.CheckHealth(ctx, organizationID, siteID)
}

func (h Handler) GetSiteHealth(ctx context.Context, organizationID, siteID string) (*appgateway.HealthCheckResult, error) {
	return h.Service.GetHealthHistory(ctx, organizationID, siteID)
}

func (h Handler) GetGlobalSites(ctx context.Context) ([]*appgateway.SiteListItem, error) {
	return h.Service.GetGlobalSites(ctx)
}
