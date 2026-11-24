// Package sorting holds the one implementation of this project's list sorting
// contract (手册 4.8.1).
//
// The contract has three parts, and each of them was broken in a different way:
//
//  1. The whitelist maps **API camelCase field names to physical columns**. The
//     only whitelist that existed was keyed by snake_case database names with
//     every value equal to its key — so the comment claiming it "maps API field
//     names to database column names" described a function it did not perform,
//     and a client sending the documented `sortField=createdAt` silently got the
//     default order instead.
//  2. Any field outside the whitelist is **an error**, not a fallback. Falling
//     back to `ORDER BY created_at DESC` means a caller who asked for
//     `password_hash` (or simply misspelled a field) cannot tell that the order
//     it received is not the order it asked for.
//  3. Every ORDER BY ends with a **unique-key tiebreaker**. Without it, rows
//     sharing a timestamp have no defined order, so consecutive pages can repeat
//     or skip rows even though each page is internally consistent.
//
// A Spec is per-module because the columns differ; Resolve is shared so the
// direction parsing, the tiebreaker and the error shape cannot drift between
// modules.
package sorting

import (
	"sort"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/sqlguard"
)

// Direction is a validated sort direction.
type Direction string

const (
	Ascending  Direction = "ASC"
	Descending Direction = "DESC"

	// DefaultTiebreaker is the unique key appended to every ORDER BY. Every table
	// in this schema has an `id` primary key (the replay gate asserts the
	// `uuidv7()` default exists on all of them), so it is a safe default; a table
	// whose unique key is composite can override it per Spec.
	DefaultTiebreaker = "id"
)

// Spec is the per-module whitelist: API field name (camelCase) → physical column.
type Spec struct {
	// Fields is the whitelist. Keys are the names a client may send; values are
	// the column names that may be interpolated into SQL.
	Fields map[string]string
	// Tiebreaker is the unique-key column appended to every ORDER BY. Empty means
	// DefaultTiebreaker.
	Tiebreaker string
}

// NewSpec builds a Spec from a whitelist.
func NewSpec(fields map[string]string) Spec {
	return Spec{Fields: fields}
}

// AllowedFields returns the accepted API field names, sorted. Callers use it to
// tell a client what it may ask for, so the error message and the OpenAPI
// description can be derived from the same list.
func (s Spec) AllowedFields() []string {
	names := make([]string, 0, len(s.Fields))
	for name := range s.Fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// tiebreaker returns the unique-key column for this spec.
func (s Spec) tiebreaker() string {
	if strings.TrimSpace(s.Tiebreaker) == "" {
		return DefaultTiebreaker
	}
	return s.Tiebreaker
}

// Resolve turns a client-supplied field and direction into an ORDER BY clause:
// `<column> <DIR>, <tiebreaker> ASC`.
//
// An empty field means "the caller did not ask for an order", which is not an
// error: the caller then gets the module's own default, expressed as
// `fallbackColumn DESC`. An empty direction means ascending (手册 4.8.1). A field
// that is not in the whitelist, or a direction that is not asc/desc, is a 422 —
// the request is well-formed but semantically invalid, and answering with a
// silently different order is what made the old behaviour undiagnosable.
func (s Spec) Resolve(field, order, fallbackColumn string) (string, error) {
	field = strings.TrimSpace(field)
	order = strings.TrimSpace(order)

	direction, err := ParseDirection(order)
	if err != nil {
		return "", err
	}

	column := fallbackColumn
	if field != "" {
		column = s.Fields[field]
		if column == "" {
			return "", apperrors.NewWithMeta(
				apperrors.CodeUnprocessableEntity,
				"sortField must be one of: "+strings.Join(s.AllowedFields(), ", "),
				map[string]any{"field": "sortField", "value": field, "allowed": s.AllowedFields()},
			)
		}
	}

	clause := "ORDER BY " + column + " " + string(direction) + ", " + s.tiebreaker() + " ASC"
	if err := sqlguard.ValidOrderByClause(clause); err != nil {
		return "", apperrors.NewInternal("sql guard violation: " + err.Error())
	}
	return clause, nil
}

// ParseDirection validates sortOrder. It is exported because the boundary needs
// to reject a bad direction even when no field was supplied.
//
// Only the two documented values are accepted. The previous implementation
// compared against the lowercase literal `desc`, so `DESC`, `desc ` and
// `descending` all fell through to ASC — a client that wrote `sortOrder=DESC`
// received ascending results and no indication anything was wrong.
func ParseDirection(order string) (Direction, error) {
	switch strings.ToLower(strings.TrimSpace(order)) {
	case "", "asc":
		return Ascending, nil
	case "desc":
		return Descending, nil
	default:
		return "", apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"sortOrder must be asc or desc",
			map[string]any{"field": "sortOrder", "value": strings.TrimSpace(order)},
		)
	}
}
