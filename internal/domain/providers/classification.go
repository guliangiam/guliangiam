package providers

import (
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type Family string

const (
	FamilyIdentity     Family = "identity"
	FamilyAI           Family = "ai"
	FamilyPayment      Family = "payment"
	FamilyNotification Family = "notification"
)

type Kind string
type Variant string

const (
	VariantSandbox    Variant = "sandbox"
	VariantProduction Variant = "production"
)

type Status string

const (
	StatusDraft    Status = "draft"
	StatusTesting  Status = "testing"
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

// Registration is the family-neutral control-plane view of a provider. During
// stage A it is persisted through the existing providers/provider_configs
// tables; native family rows remain the protocol-level detail records.
type Registration struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Family         Family
	Kind           Kind
	Variant        Variant
	Status         Status
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
	Revision       int64
}

// The value sets below are the single source for what the API accepts. They
// feed both the predicate helpers and the 422 `allowed` list, so a rejection
// can never name a value the predicate would accept (手册 4.14.3).
var (
	Families         = []Family{FamilyIdentity, FamilyAI, FamilyPayment, FamilyNotification}
	ProviderStatuses = []Status{StatusDraft, StatusTesting, StatusActive, StatusDisabled}
	Variants         = []Variant{VariantSandbox, VariantProduction}
)

// validStatusTransitions mirrors the control-plane lifecycle. It is read by the
// guard and by AllowedTransitions(), so the 422 `allowed` list is the graph
// itself rather than a second hand-written copy.
var validStatusTransitions = map[Status][]Status{
	StatusDraft:    {StatusTesting},
	StatusTesting:  {StatusActive},
	StatusActive:   {StatusDisabled},
	StatusDisabled: {StatusTesting},
}

func enumStrings[T ~string](values []T) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = string(value)
	}
	return out
}

func (f Family) Valid() bool {
	for _, allowed := range Families {
		if f == allowed {
			return true
		}
	}
	return false
}

func (s Status) Valid() bool {
	for _, allowed := range ProviderStatuses {
		if s == allowed {
			return true
		}
	}
	return false
}

// ValidateFamily rejects an unknown family with 422 and the accepted set. It is
// exported for callers that choose a family before a Registration exists
// (strategy lookup, native-detail loading).
func ValidateFamily(family Family) error {
	if family.Valid() {
		return nil
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"unsupported provider family: "+string(family),
		map[string]any{
			"field":   "family",
			"value":   string(family),
			"allowed": enumStrings(Families),
		},
	)
}

func (r *Registration) Normalize() {
	r.Family = Family(strings.ToLower(strings.TrimSpace(string(r.Family))))
	r.Kind = Kind(strings.ToLower(strings.TrimSpace(string(r.Kind))))
	r.Variant = Variant(strings.ToLower(strings.TrimSpace(string(r.Variant))))
	if r.Status == "" {
		r.Status = StatusDraft
	}
}

func (r Registration) Validate() error {
	if strings.TrimSpace(r.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(r.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	if strings.TrimSpace(r.DisplayName) == "" {
		return apperrors.NewInvalidArgument("displayName is required")
	}
	if !r.Family.Valid() {
		return ValidateFamily(r.Family)
	}
	if r.Kind == "" {
		return apperrors.NewInvalidArgument("kind is required")
	}
	if !r.Status.Valid() {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"invalid provider status: "+string(r.Status),
			map[string]any{
				"field":   "status",
				"value":   string(r.Status),
				"allowed": enumStrings(ProviderStatuses),
			},
		)
	}
	if r.Family == FamilyPayment {
		if r.Variant != VariantSandbox && r.Variant != VariantProduction {
			return apperrors.NewWithMeta(
				apperrors.CodeUnprocessableEntity,
				"payment variant must be sandbox or production",
				map[string]any{
					"field":   "variant",
					"value":   string(r.Variant),
					"allowed": enumStrings(Variants),
				},
			)
		}
	} else if r.Variant != "" {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"variant is not supported for "+string(r.Family)+" providers",
			map[string]any{"field": "variant", "value": string(r.Variant), "expected": ""},
		)
	}
	return nil
}

func (r Registration) CanTransitionTo(target Status) bool {
	if target == r.Status {
		return true
	}
	for _, allowed := range validStatusTransitions[r.Status] {
		if target == allowed {
			return true
		}
	}
	return false
}

// AllowedTransitions returns the statuses this registration may move to next.
func (r Registration) AllowedTransitions() []string {
	return enumStrings(validStatusTransitions[r.Status])
}

func (r *Registration) TransitionTo(target Status) error {
	if !target.Valid() {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"invalid provider status: "+string(target),
			map[string]any{
				"field":   "status",
				"value":   string(target),
				"allowed": enumStrings(ProviderStatuses),
			},
		)
	}
	if !r.CanTransitionTo(target) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"cannot move provider from "+string(r.Status)+" to "+string(target),
			map[string]any{
				"field":      "status",
				"value":      string(r.Status),
				"transition": string(target),
				"allowed":    r.AllowedTransitions(),
			},
		)
	}
	r.Status = target
	return nil
}

func (r Registration) IsRoutable() bool {
	return r.Status == StatusActive && r.DeletedAt == nil
}
