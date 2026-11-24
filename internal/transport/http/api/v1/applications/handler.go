package applications

import (
	"context"

	appapplications "github.com/guliangiam/guliangiam/internal/app/applications"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

// TokenVerifier 是本包对 JWT 验证能力的本地接口依赖。
// 由 bootstrap/wire_transport.go 注入 infra/jwt.Issuer 具体实现。
type TokenVerifier interface {
	ValidateToken(tokenString string) (map[string]interface{}, error)
}

type Handler struct {
	Service   appapplications.Service
	JWTIssuer TokenVerifier
}

func (h Handler) List(ctx context.Context, request ListApplicationsRequest) (ListApplicationsResponse, error) {
	items, total, err := h.Service.ListApplications(ctx, appapplications.ListApplicationsQuery(request))
	if err != nil {
		return ListApplicationsResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, request GetApplicationRequest) (*ApplicationResponse, error) {
	response, err := h.Service.GetApplication(ctx, request.OrganizationID, request.ID)
	if err != nil {
		return nil, err
	}

	typed := ApplicationResponse(*response)
	return &typed, nil
}

func (h Handler) GetDeleteSummary(ctx context.Context, request GetApplicationRequest) (*ApplicationDeleteSummaryResponse, error) {
	response, err := h.Service.GetApplicationDeleteSummary(ctx, request.OrganizationID, request.ID)
	if err != nil {
		return nil, err
	}

	typed := ApplicationDeleteSummaryResponse(*response)
	return &typed, nil
}

// operatorID returns the authenticated session's user id, or "" for system
// paths that carry no session. Callers write it into the command so the audit
// provider can attribute the write to the operator.
func operatorID(ctx context.Context) string {
	if session, ok := authtransport.SessionContextFromContext(ctx); ok {
		return session.UserID
	}
	return ""
}

func (h Handler) Create(ctx context.Context, request CreateApplicationRequest) (*CreateApplicationResponse, error) {
	command := request.toCommand()
	command.ActorID = operatorID(ctx)
	result, err := h.Service.CreateApplication(ctx, command)
	if err != nil {
		return nil, err
	}

	return &CreateApplicationResponse{
		ID:                 result.Application.ID,
		Status:             "created",
		ClientSecretPlain:  result.ClientSecretPlain,
		ClientSecretPrefix: result.ClientSecretPrefix,
	}, nil
}

// RotateClientSecret 轮换应用的 client secret（仅 basic/post 方式）。
// 旧 secret 在宽限期内仍可验签，新明文只在本次响应出现。
func (h Handler) RotateClientSecret(ctx context.Context, organizationID, id string) (*RotateClientSecretResponse, error) {
	result, err := h.Service.RotateClientSecret(ctx, appapplications.RotateClientSecretCommand{
		ID:             id,
		OrganizationID: organizationID,
		ActorID:        operatorID(ctx),
	})
	if err != nil {
		return nil, err
	}
	return &RotateClientSecretResponse{
		ClientSecretPlain:  result.ClientSecretPlain,
		ClientSecretPrefix: result.ClientSecretPrefix,
		RotatedAt:          result.RotatedAt,
		GraceUntil:         result.GraceUntil,
	}, nil
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchApplicationRequest) (*PatchApplicationResponse, error) {
	command := request.toCommand(id)
	command.OrganizationID = organizationID
	command.ActorID = operatorID(ctx)
	revision, err := h.Service.PatchApplicationWithRevision(ctx, command)
	if err != nil {
		return nil, err
	}
	return &PatchApplicationResponse{Status: "updated", Revision: revision}, nil
}

func (h Handler) Delete(ctx context.Context, request GetApplicationRequest) error {
	return h.Service.DeleteApplication(ctx, appapplications.DeleteApplicationCommand{
		ID:             request.ID,
		OrganizationID: request.OrganizationID,
		ActorID:        operatorID(ctx),
	})
}

func (h Handler) GetProviderBindings(ctx context.Context, request GetApplicationRequest) (*ApplicationProviderBindingsResponse, error) {
	response, err := h.Service.GetApplication(ctx, request.OrganizationID, request.ID)
	if err != nil {
		return nil, err
	}

	return &ApplicationProviderBindingsResponse{Items: toProviderBindingResponseItems(response.ProviderIDs)}, nil
}

func (h Handler) ReplaceProviderBindings(ctx context.Context, organizationID, id string, request ReplaceApplicationProviderBindingsRequest) (*ReplaceProviderBindingsResponse, error) {
	revision, err := h.Service.ReplaceProviderBindingsWithRevision(ctx, organizationID, id, operatorID(ctx), request.toCommands())
	if err != nil {
		return nil, err
	}
	return &ReplaceProviderBindingsResponse{Status: "updated", Revision: revision}, nil
}

// GetClientCertificates 返回应用当前活跃的客户端证书元数据（不含证书原文）。
func (h Handler) GetClientCertificates(ctx context.Context, organizationID, id string) (*ApplicationClientCertificatesResponse, error) {
	views, err := h.Service.ListClientCertificates(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	items := make([]ApplicationClientCertificateResponseItem, 0, len(views))
	for _, view := range views {
		items = append(items, ApplicationClientCertificateResponseItem{
			FingerprintSHA256: view.FingerprintSHA256,
			NotAfter:          view.NotAfter,
			SelfSigned:        view.SelfSigned,
			Status:            view.Status,
		})
	}
	return &ApplicationClientCertificatesResponse{Items: items}, nil
}

// ReplaceClientCertificates 用请求集合替换全部活跃证书绑定。
func (h Handler) ReplaceClientCertificates(ctx context.Context, organizationID, id string, request ReplaceApplicationClientCertificatesRequest) error {
	return h.Service.ReplaceClientCertificates(ctx, request.toCommand(organizationID, id, operatorID(ctx)))
}

func (h Handler) Export(ctx context.Context, organizationID, id string) (*appapplications.ExportPayload, error) {
	return h.Service.ExportApplication(ctx, organizationID, id)
}

func (h Handler) Import(ctx context.Context, request ImportApplicationRequest) (*CreateApplicationResponse, error) {
	app, err := h.Service.ImportApplication(ctx, appapplications.ImportPayload{
		OrganizationID: request.OrganizationID,
		ActorID:        operatorID(ctx),
		Version:        request.Version,
		Export:         request.Export,
	})
	if err != nil {
		return nil, err
	}

	return &CreateApplicationResponse{ID: app.ID, Status: "imported"}, nil
}

func toProviderBindingResponseItems(providerIDs []string) []ApplicationProviderBindingResponseItem {
	items := make([]ApplicationProviderBindingResponseItem, 0, len(providerIDs))
	for _, providerID := range providerIDs {
		items = append(items, ApplicationProviderBindingResponseItem{ProviderID: providerID})
	}
	return items
}

// GetByClientID returns the anonymous pre-login discovery projection.
func (h Handler) GetByClientID(ctx context.Context, clientID string) (*ApplicationPublicResponse, error) {
	response, err := h.Service.GetApplicationPublicByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	typed := ApplicationPublicResponse(*response)
	return &typed, nil
}

// Resolve looks up an application by organization slug and application slug.
// Like by-client-id, it serves the anonymous public discovery projection.
func (h Handler) Resolve(ctx context.Context, ownerSlug, appSlug string) (*ApplicationPublicResponse, error) {
	response, err := h.Service.GetApplicationByOrgAndSlug(ctx, ownerSlug, appSlug)
	if err != nil {
		return nil, err
	}
	typed := ApplicationPublicResponse(*response)
	return &typed, nil
}

// GetUserApplication returns the default application for the current user's organization
// as a full ApplicationDetail, matching the frontend getUserApplication() expectation.
func (h Handler) GetUserApplication(ctx context.Context, organizationID string) (*appapplications.ApplicationDetail, error) {
	return h.Service.GetDefaultApplication(ctx, organizationID)
}

// GetApplicationNames returns a lightweight list of application names.
func (h Handler) GetApplicationNames(ctx context.Context) ([]appapplications.ApplicationNameItem, error) {
	return h.Service.GetApplicationNames(ctx)
}
