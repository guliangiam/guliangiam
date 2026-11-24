// Package enumset holds the single definition of a closed value set for an
// enum-shaped API field, and the check that turns an outside value into a 422.
//
// Why it exists (手册 4.14.3「枚举的开放扩展规则」第 4 条；开放项注册表 OPEN-API-11)
// ----------------------------------------
// Enum-shaped fields used to be validated only for emptiness
// (`strings.TrimSpace(v) == ""`), so any string was accepted and written to the
// database. Two consequences, both real:
//
//   - The console shipped values the server could not resolve. Captcha providers
//     were stored as `Default`/`reCAPTCHA`/`Turnstile` while the captcha factory
//     registers `default`/`recaptcha`/`cloudflare`, so every provider created in
//     the console was unusable — and because the unresolved lookup surfaced as a
//     wrapped internal error, the login captcha endpoint answered 500.
//   - Nothing tied the accepted values to the values the rest of the system
//     handles, so the two lists could only drift further apart.
//
// The rule is asymmetric on purpose:
//
//   - **Input is closed** — an unknown value is rejected with 422. Rejecting is
//     the whole point: a value the server cannot act on must not be persisted,
//     because by the time the failure shows up (a login attempt, a delivery
//     attempt) the caller has no way to connect it to what they typed.
//   - **Output is open** — a client must tolerate values it does not know
//     (手册 4.14.3 item 2), so response-side consumers need a fallback. That is
//     the frontend's contract, not this package's.
//
// A Set never normalises. Silently folding `Default` into `default` would make
// the bad write invisible instead of fixing it, which is the defect this package
// exists to remove.
package enumset

import (
	"sort"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Set is an ordered, closed collection of allowed values for one field.
//
// A Set is immutable after construction and safe for concurrent use.
type Set struct {
	name   string
	values []string
	index  map[string]struct{}
}

// NewSet declares a closed set.
//
// The name is the stable identifier used in error metadata, tests and the
// cross-language gate; it is not a field path. Values keep their declared order,
// which is the order shown to a caller in the 422 `allowed` list and the order
// an option list should render in.
//
// It panics on a malformed declaration — fewer than two values, a blank value, a
// value with surrounding whitespace, or a duplicate. Those are programming
// errors in a package-level declaration, so they must fail while the binary (or
// its test) loads rather than turn into a runtime surprise per request. The
// panic is also what lets the gate trust the declarations: a set that exists at
// all is well-formed.
func NewSet(name string, values ...string) *Set {
	if strings.TrimSpace(name) == "" {
		panic("enumset: set name must not be blank")
	}
	if len(values) < 2 {
		panic("enumset: set " + name + " needs at least two values")
	}

	index := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			panic("enumset: set " + name + " has a blank value")
		}
		if strings.TrimSpace(value) != value {
			panic("enumset: set " + name + " value " + value + " has surrounding whitespace")
		}
		if _, dup := index[value]; dup {
			panic("enumset: set " + name + " declares " + value + " twice")
		}
		index[value] = struct{}{}
	}

	return &Set{
		name:   name,
		values: append([]string(nil), values...),
		index:  index,
	}
}

// Name returns the stable identifier of the set.
func (s *Set) Name() string { return s.name }

// Values returns the declared values in declaration order.
//
// The slice is a copy: a caller cannot widen the set through it.
func (s *Set) Values() []string { return append([]string(nil), s.values...) }

// Contains reports whether value is a member of the set.
//
// The comparison is exact. `Default` is not `default` — accepting either would
// hide a client/server disagreement instead of surfacing it.
func (s *Set) Contains(value string) bool {
	_, ok := s.index[value]
	return ok
}

// SortedValues returns the values in lexicographic order.
//
// Error metadata uses this form so the same set always renders the same string,
// which keeps tests and logs comparable.
func (s *Set) SortedValues() []string {
	sorted := s.Values()
	sort.Strings(sorted)
	return sorted
}

// JoinedValues returns the sorted values as a comma-separated list, for a human
// readable part of an error message.
func (s *Set) JoinedValues() string {
	return strings.Join(s.SortedValues(), ", ")
}

// Validate rejects a value outside the set with 422.
//
// 422 rather than 400 is deliberate (错误码注册表 §1): the request is well formed,
// a rule rejected it, and the caller can succeed by changing one field. The
// metadata carries the field name, the offending value and the allowed values,
// so the console can point at the field instead of printing a sentence.
func (s *Set) Validate(field, value string) error {
	if s.Contains(value) {
		return nil
	}
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		field+" must be one of: "+s.JoinedValues(),
		map[string]any{
			"field":   field,
			"value":   value,
			"allowed": s.SortedValues(),
			"set":     s.name,
		},
	)
}

// ValidateRequired rejects a blank value as missing and an unknown value as
// unprocessable.
//
// The two cases stay distinguishable on purpose: an absent value is the caller
// forgetting a field (400-class argument error), while an unknown value is the
// caller naming something that does not exist (422). Collapsing them would tell
// a client to retry a request that cannot succeed without a change.
func (s *Set) ValidateRequired(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return apperrors.NewInvalidArgument(field + " is required")
	}
	return s.Validate(field, value)
}

// ValidateOptional applies Validate to a value that may be absent, for PATCH
// semantics where nil means "leave unchanged".
//
// The nil case must not be treated as an empty string: PATCH omits a field to
// keep it, so validating the zero value would reject every partial update that
// does not mention the enum.
func (s *Set) ValidateOptional(field string, value *string) error {
	if value == nil {
		return nil
	}
	if strings.TrimSpace(*value) == "" {
		return apperrors.NewInvalidArgument(field + " must not be blank")
	}
	return s.Validate(field, *value)
}

// ValidateOptionalRequired applies ValidateRequired to a value that may be
// absent, for PATCH semantics: absent is allowed, present must be usable.
func (s *Set) ValidateOptionalRequired(field string, value *string) error {
	if value == nil {
		return nil
	}
	return s.ValidateRequired(field, *value)
}
