package organizations

import (
	"context"

	appapplications "github.com/guliangiam/guliangiam/internal/app/applications"
	apporganizations "github.com/guliangiam/guliangiam/internal/app/organizations"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service            apporganizations.Service
	ApplicationService appapplications.Service
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

func (h Handler) List(ctx context.Context, request ListOrganizationsRequest) (ListOrganizationsResponse, error) {
	items, total, err := h.Service.ListOrganizations(ctx, apporganizations.ListOrganizationsQuery(request))
	if err != nil {
		return ListOrganizationsResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, request GetOrganizationRequest) (*OrganizationResponse, error) {
	response, err := h.Service.GetOrganization(ctx, request.ID)
	if err != nil {
		return nil, err
	}

	typed := OrganizationResponse(*response)
	return &typed, nil
}

func (h Handler) GetBySlug(ctx context.Context, request GetOrganizationBySlugRequest) (*OrganizationResponse, error) {
	response, err := h.Service.GetOrganizationBySlug(ctx, request.Slug)
	if err != nil {
		return nil, err
	}

	typed := OrganizationResponse(*response)
	return &typed, nil
}

// GetBySlugPublic serves the anonymous /by-slug and /resolve discovery routes.
// It returns the minimal OrganizationPublicDetail: no credential fields.
func (h Handler) GetBySlugPublic(ctx context.Context, request GetOrganizationBySlugRequest) (*apporganizations.OrganizationPublicDetail, error) {
	response, err := h.Service.GetOrganizationBySlugPublic(ctx, request.Slug)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (h Handler) Create(ctx context.Context, request CreateOrganizationRequest) (*CreateOrganizationResponse, error) {
	command := request.toCommand()
	command.ActorID = operatorID(ctx)
	org, err := h.Service.CreateOrganization(ctx, apporganizations.CreateOrganizationCommand(command))
	if err != nil {
		return nil, err
	}

	return &CreateOrganizationResponse{ID: org.ID, Status: "created"}, nil
}

func (h Handler) Patch(ctx context.Context, id string, request PatchOrganizationRequest) error {
	return h.Service.PatchOrganization(ctx, request.toCommand(id))
}

// Delete soft-deletes an organization.
func (h Handler) Delete(ctx context.Context, id string) error {
	return h.Service.DeleteOrganization(ctx, apporganizations.DeleteOrganizationCommand{ID: id})
}

func (h Handler) UpdateSettings(ctx context.Context, id string, request UpdateOrganizationSettingsRequest) error {
	return h.Service.UpdateOrganizationSettings(ctx, request.toCommand(id))
}

func (h Handler) UpdateTheme(ctx context.Context, id string, request UpdateOrganizationThemeRequest) error {
	return h.Service.UpdateOrganizationTheme(ctx, request.toCommand(id))
}

func (h Handler) UpdateSecurityPolicy(ctx context.Context, id string, request UpdateOrganizationSecurityPolicyRequest) error {
	return h.Service.UpdateOrganizationSecurityPolicy(ctx, request.toCommand(id))
}

// ListDeploymentRegions returns the region registry for the organization form.
func (h Handler) ListDeploymentRegions(ctx context.Context) ([]apporganizations.DeploymentRegionDTO, error) {
	return h.Service.ListDeploymentRegions(ctx)
}

// UpdateConfiguration saves core + settings + theme + security policy in one transaction.
func (h Handler) UpdateConfiguration(ctx context.Context, id string, request UpdateOrganizationConfigurationRequest) error {
	return h.Service.UpdateOrganizationConfiguration(ctx, request.toCommand(id))
}

func (h Handler) GetSettings(ctx context.Context, id string) (map[string]any, error) {
	org, err := h.Service.GetOrganization(ctx, id)
	if err != nil {
		return nil, err
	}
	return org.Settings, nil
}

func (h Handler) GetTheme(ctx context.Context, id string) (map[string]any, error) {
	org, err := h.Service.GetOrganization(ctx, id)
	if err != nil {
		return nil, err
	}
	return org.Theme, nil
}

func (h Handler) GetSecurityPolicy(ctx context.Context, id string) (map[string]any, error) {
	org, err := h.Service.GetOrganization(ctx, id)
	if err != nil {
		return nil, err
	}
	return org.SecurityPolicySummary, nil
}

// ListOrganizationApplications lists applications belonging to a specific organization.
func (h Handler) ListOrganizationApplications(ctx context.Context, orgID string, page int, pageSize int) (ListOrganizationApplicationsResponse, error) {
	items, total, err := h.ApplicationService.ListApplications(ctx, appapplications.ListApplicationsQuery{
		Page:           page,
		PageSize:       pageSize,
		OrganizationID: orgID,
	})
	if err != nil {
		return ListOrganizationApplicationsResponse{}, err
	}

	return presenter.NewListResponse(items, page, pageSize, total), nil
}

// GetDefaultApplication returns the default application for the given organization.
func (h Handler) GetDefaultApplication(ctx context.Context, orgID string) (*appapplications.ApplicationDetail, error) {
	return h.ApplicationService.GetDefaultApplication(ctx, orgID)
}

// GetOrganizationNames returns a lightweight list of organization names.
func (h Handler) GetOrganizationNames(ctx context.Context) ([]apporganizations.OrganizationNameItem, error) {
	return h.Service.GetOrganizationNames(ctx)
}

// PreviewPolicyInheritance resolves the effective security-policy value and
// lock state across the organization's ancestor chain (design/b2b §3).
func (h Handler) PreviewPolicyInheritance(ctx context.Context, organizationID, policyKey string) (*apporganizations.PolicyInheritancePreview, error) {
	return h.Service.PreviewPolicyInheritance(ctx, organizationID, policyKey)
}
