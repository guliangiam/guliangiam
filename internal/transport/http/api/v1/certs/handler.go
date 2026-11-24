package certs

import (
	"context"

	appcerts "github.com/guliangiam/guliangiam/internal/app/certs"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service appcerts.Service
}

func NewHandler(service appcerts.Service) Handler {
	return Handler{Service: service}
}

func (h Handler) List(ctx context.Context, request ListCertsRequest) (ListCertsResponse, error) {
	items, total, err := h.Service.ListCerts(ctx, appcerts.ListCertsQuery(request))
	if err != nil {
		return ListCertsResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID, id string) (*CertResponse, error) {
	response, err := h.Service.GetCert(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}

	typed := CertResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateCertRequest) error {
	return h.Service.CreateCert(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchCertRequest) error {
	command := request.toCommand(id)
	command.OrganizationID = organizationID
	return h.Service.PatchCert(ctx, command)
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteCert(ctx, organizationID, id)
}

// UpdateDomainExpire updates the domain expiration time for a certificate.
func (h Handler) UpdateDomainExpire(ctx context.Context, organizationID, id, domainExpireTime string) error {
	return h.Service.UpdateDomainExpire(ctx, organizationID, id, domainExpireTime)
}

// CheckDomainExpiration checks the domain expiration for a certificate by parsing its NotAfter.
func (h Handler) CheckDomainExpiration(ctx context.Context, organizationID, id string) (*appcerts.CheckDomainExpirationResult, error) {
	return h.Service.CheckDomainExpiration(ctx, organizationID, id)
}
