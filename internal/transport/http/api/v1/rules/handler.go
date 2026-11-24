package rules

import (
	"context"

	apprules "github.com/guliangiam/guliangiam/internal/app/rules"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type Handler struct {
	Service apprules.Service
}

func (h Handler) List(ctx context.Context, request ListRulesRequest) (ListRulesResponse, error) {
	items, total, err := h.Service.ListRules(ctx, apprules.ListRulesQuery{
		Page:           request.Page,
		PageSize:       request.PageSize,
		OrganizationID: request.OrganizationID,
		Scope:          request.Scope,
		RuleType:       request.RuleType,
		IsEnabled:      request.IsEnabled,
	})
	if err != nil {
		return ListRulesResponse{}, err
	}

	return presenter.NewListResponse(items, request.Page, request.PageSize, total), nil
}

func (h Handler) Get(ctx context.Context, organizationID, id string) (*RuleResponse, error) {
	response, err := h.Service.GetRule(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}

	typed := RuleResponse(*response)
	return &typed, nil
}

func (h Handler) Create(ctx context.Context, request CreateRuleRequest) error {
	return h.Service.CreateRule(ctx, request.toCommand())
}

func (h Handler) Patch(ctx context.Context, organizationID, id string, request PatchRuleRequest) error {
	return h.Service.PatchRule(ctx, organizationID, request.toCommand(id))
}

func (h Handler) Delete(ctx context.Context, organizationID, id string) error {
	return h.Service.DeleteRule(ctx, organizationID, id)
}

// Validate validates rule expressions without creating the rule.
func (h Handler) Validate(ctx context.Context, ruleType string, conditions []string) (*apprules.ValidationResult, error) {
	validator := apprules.RuleValidator{}
	result := validator.Validate(ruleType, conditions)
	return result, nil
}
