package organizations

import (
	"context"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

var supportedPolicyKeys = map[string]bool{
	"passwordPolicy":          true,
	"mfaPolicy":               true,
	"ipPolicy":                true,
	"sessionPolicy":           true,
	"riskPolicy":              true,
	"lockoutPolicy":           true,
	"concurrentSessionPolicy": true,
	"passwordDeriveAlgorithm": true,
	"theme":                   true,
	"accountItems":            true,
	"mfaItems":                true,
	"languages":               true,
	"featureFlags":            true,
}

type PolicyInheritancePreviewStep struct {
	OrganizationID            string `json:"organizationId"`
	Mode                      string `json:"mode"`
	Applied                   bool   `json:"applied"`
	Rejected                  bool   `json:"rejected"`
	Value                     any    `json:"value,omitempty"`
	ValueSourceOrganizationID string `json:"valueSourceOrganizationId,omitempty"`
}

type PolicyInheritancePreview struct {
	PolicyKey                     string                         `json:"policyKey"`
	EffectiveValue                any                            `json:"effectiveValue,omitempty"`
	EffectiveSourceOrganizationID string                         `json:"effectiveSourceOrganizationId,omitempty"`
	Locked                        bool                           `json:"locked"`
	LockedByOrganizationID        string                         `json:"lockedByOrganizationId,omitempty"`
	Chain                         []PolicyInheritancePreviewStep `json:"chain"`
}

func policyValue(org *orgdomain.Organization, key string) (any, bool) {
	if org == nil {
		return nil, false
	}
	switch key {
	case "passwordPolicy":
		return nonEmptyPolicy(org.Security.PasswordPolicy)
	case "mfaPolicy":
		return nonEmptyPolicy(org.Security.MfaPolicy)
	case "ipPolicy":
		return nonEmptyPolicy(org.Security.IPPolicy)
	case "sessionPolicy":
		return nonEmptyPolicy(org.Security.SessionPolicy)
	case "riskPolicy":
		return nonEmptyPolicy(org.Security.RiskPolicy)
	case "lockoutPolicy":
		return nonEmptyPolicy(org.Security.LockoutPolicy)
	case "concurrentSessionPolicy":
		return nonEmptyPolicy(org.Security.ConcurrentSessionPolicy)
	case "passwordDeriveAlgorithm":
		if org.Security.PasswordDeriveAlgorithm == "" {
			return nil, false
		}
		return org.Security.PasswordDeriveAlgorithm, true
	case "theme":
		if org.Theme.ThemeName == "" && len(org.Theme.Tokens) == 0 && org.Theme.CustomCSS == "" && len(org.Theme.Assets) == 0 {
			return nil, false
		}
		return map[string]any{
			"themeName": org.Theme.ThemeName,
			"tokens":    org.Theme.Tokens,
			"customCss": org.Theme.CustomCSS,
			"assets":    org.Theme.Assets,
		}, true
	case "accountItems":
		if len(org.Settings.AccountItems) == 0 {
			return nil, false
		}
		return org.Settings.AccountItems, true
	case "mfaItems":
		if len(org.Settings.MfaItems) == 0 {
			return nil, false
		}
		return org.Settings.MfaItems, true
	case "languages":
		if len(org.Settings.Languages) == 0 {
			return nil, false
		}
		return org.Settings.Languages, true
	case "featureFlags":
		if len(org.Settings.FeatureFlags) == 0 {
			return nil, false
		}
		return org.Settings.FeatureFlags, true
	default:
		return nil, false
	}
}

func nonEmptyPolicy(value map[string]any) (any, bool) {
	if len(value) == 0 {
		return nil, false
	}
	return value, true
}

// PreviewPolicyInheritance resolves the effective value and lock state for one
// policy key across the target organization's root→target ancestor chain.
func (s *Service) PreviewPolicyInheritance(ctx context.Context, organizationID, policyKey string) (*PolicyInheritancePreview, error) {
	if organizationID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	if !supportedPolicyKeys[policyKey] {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			"unsupported policyKey", map[string]any{"field": "policyKey", "value": policyKey})
	}
	organizations := make([]*orgdomain.Organization, 0, orgdomain.MaxOrganizationTreeDepth)
	seen := make(map[string]bool, orgdomain.MaxOrganizationTreeDepth)
	currentID := organizationID
	for len(organizations) < orgdomain.MaxOrganizationTreeDepth && currentID != "" {
		organization, err := s.UOW.Organizations().GetByID(ctx, currentID)
		if err != nil {
			return nil, err
		}
		if seen[organization.ID] {
			return nil, apperrors.NewInternal("organization tree contains a cycle")
		}
		seen[organization.ID] = true
		organizations = append([]*orgdomain.Organization{organization}, organizations...)
		currentID = organization.ParentID
	}
	chain := make([]orgdomain.PolicyChainNode, 0, len(organizations))
	for _, organization := range organizations {
		rules := make([]orgdomain.PolicyInheritanceRule, 0)
		if s.PolicyInheritance != nil {
			loaded, err := s.PolicyInheritance.ListPolicyInheritance(ctx, organization.ID)
			if err != nil {
				return nil, err
			}
			rules = loaded
		}
		ruleMap := make(map[string]orgdomain.PolicyInheritanceRule, len(rules))
		for _, rule := range rules {
			ruleMap[rule.PolicyKey] = rule
		}
		value, hasValue := policyValue(organization, policyKey)
		values := map[string]any{}
		if hasValue {
			values[policyKey] = value
		}
		chain = append(chain, orgdomain.PolicyChainNode{
			OrganizationID: organization.ID,
			Rules:          ruleMap,
			Values:         values,
		})
	}
	resolution, err := orgdomain.ResolvePolicy(chain, policyKey)
	if err != nil {
		return nil, err
	}
	steps := make([]PolicyInheritancePreviewStep, 0, len(resolution.Steps))
	for _, step := range resolution.Steps {
		steps = append(steps, PolicyInheritancePreviewStep{
			OrganizationID:            step.OrganizationID,
			Mode:                      string(step.Mode),
			Applied:                   step.Applied,
			Rejected:                  step.Rejected,
			Value:                     step.Value,
			ValueSourceOrganizationID: step.ValueSourceOrganizationID,
		})
	}
	return &PolicyInheritancePreview{
		PolicyKey:                     resolution.PolicyKey,
		EffectiveValue:                resolution.EffectiveValue,
		EffectiveSourceOrganizationID: resolution.EffectiveSourceOrganizationID,
		Locked:                        resolution.Locked,
		LockedByOrganizationID:        resolution.LockedByOrganizationID,
		Chain:                         steps,
	}, nil
}
