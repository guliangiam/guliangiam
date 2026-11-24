package organizations

import (
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// PolicyMode is the declared inheritance behavior for one policy key.
type PolicyMode string

const (
	PolicyModeInherit  PolicyMode = "inherit"
	PolicyModeOverride PolicyMode = "override"
	PolicyModeLock     PolicyMode = "lock"
)

// PolicyInheritanceRule is one organization's declaration for a policy key.
type PolicyInheritanceRule struct {
	PolicyKey            string
	Mode                 PolicyMode
	SourceOrganizationID string
}

// PolicyChainNode is one organization in the root→target chain with its rules
// and the effective security-policy value declared by that organization.
type PolicyChainNode struct {
	OrganizationID string
	Rules          map[string]PolicyInheritanceRule
	Values         map[string]any
}

// PolicyResolutionStep records how one node affected the effective value.
type PolicyResolutionStep struct {
	OrganizationID            string
	Mode                      PolicyMode
	Value                     any
	Applied                   bool
	Rejected                  bool
	ValueSourceOrganizationID string
}

// PolicyResolution is the deterministic preview of one policy key.
type PolicyResolution struct {
	PolicyKey                     string
	EffectiveValue                any
	EffectiveSourceOrganizationID string
	Locked                        bool
	LockedByOrganizationID        string
	Steps                         []PolicyResolutionStep
}

// ResolvePolicy computes the effective value from a root→target chain.
// inherit keeps the ancestor value, override replaces it, lock replaces and
// pins it so deeper overrides are rejected (design/b2b §3).
func ResolvePolicy(chain []PolicyChainNode, policyKey string) (PolicyResolution, error) {
	if len(chain) == 0 {
		return PolicyResolution{}, apperrors.NewInvalidArgument("policy chain is empty")
	}
	key := strings.TrimSpace(policyKey)
	if key == "" {
		return PolicyResolution{}, apperrors.NewInvalidArgument("policyKey is required")
	}
	byID := make(map[string]PolicyChainNode, len(chain))
	for _, node := range chain {
		if node.OrganizationID == "" {
			return PolicyResolution{}, apperrors.NewInvalidArgument("organizationId is required")
		}
		byID[node.OrganizationID] = node
	}
	resolution := PolicyResolution{PolicyKey: key, Steps: make([]PolicyResolutionStep, 0, len(chain))}
	valueOf := func(node PolicyChainNode) (any, bool) {
		value, ok := node.Values[key]
		return value, ok
	}
	for index, node := range chain {
		rule, hasRule := node.Rules[key]
		mode := PolicyModeInherit
		if hasRule {
			mode = rule.Mode
		}
		step := PolicyResolutionStep{OrganizationID: node.OrganizationID, Mode: mode}
		value, hasValue := valueOf(node)
		// A lock in an ancestor wins over every deeper override; the deeper
		// declaration is recorded as rejected instead of silently ignored.
		if resolution.Locked {
			step.Rejected = hasRule || hasValue
			resolution.Steps = append(resolution.Steps, step)
			continue
		}
		switch mode {
		case PolicyModeLock:
			source := node.OrganizationID
			if rule.SourceOrganizationID != "" {
				if sourceNode, ok := byID[rule.SourceOrganizationID]; ok {
					if sourceValue, ok := sourceNode.Values[key]; ok {
						value, hasValue = sourceValue, true
					}
				}
				source = rule.SourceOrganizationID
			}
			if hasValue {
				resolution.EffectiveValue = value
				resolution.EffectiveSourceOrganizationID = source
				step.Value = value
				step.ValueSourceOrganizationID = source
			}
			resolution.Locked = true
			resolution.LockedByOrganizationID = node.OrganizationID
			step.Applied = true
		case PolicyModeOverride:
			if hasValue {
				resolution.EffectiveValue = value
				resolution.EffectiveSourceOrganizationID = node.OrganizationID
				step.Value = value
				step.ValueSourceOrganizationID = node.OrganizationID
				step.Applied = true
			}
		default: // inherit
			if index == 0 && hasValue && resolution.EffectiveSourceOrganizationID == "" {
				resolution.EffectiveValue = value
				resolution.EffectiveSourceOrganizationID = node.OrganizationID
				step.Value = value
				step.ValueSourceOrganizationID = node.OrganizationID
				step.Applied = true
			}
		}
		resolution.Steps = append(resolution.Steps, step)
	}
	return resolution, nil
}
