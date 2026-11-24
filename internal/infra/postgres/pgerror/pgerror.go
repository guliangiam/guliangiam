// Package pgerror is the single place where PostgreSQL driver errors become
// platform errors. Repositories call Map at the write boundary; without it a
// unique/check/foreign-key violation or an invalid text representation reaches
// route.WriteError as a raw *pgconn.PgError, which PresentError renders as an
// opaque 500 instead of a caller-actionable 409/422/503.
//
// Mapping (errors.yaml discipline: 400 = unparseable request, 422 = parsed but
// semantically rejected):
//
//	23505 unique_violation          -> 409 conflict
//	23514 check_violation           -> 422 unprocessable_entity
//	23503 foreign_key_violation     -> 422 unprocessable_entity
//	22P02 invalid_text_representation -> 422 unprocessable_entity
//	40001 serialization_failure     -> 503 unavailable (retryable)
//	40P01 deadlock_detected         -> 503 unavailable (retryable)
//
// Everything else is returned unchanged so the original error (and any %w
// wrapper) keeps its identity and the existing 500/log path.
//
// 22P02 answers 422, not 400: the repositories run after transport binding, so
// a value that cannot be cast (typically a malformed UUID column argument) is a
// semantic field error the caller fixes, not an unparseable body. 400 stays
// reserved for malformed JSON/missing fields. The impact is that clients that
// branched on 400 for a bad UUID must branch on 422; no error code changes.
package pgerror

import (
	"errors"
	"regexp"

	"github.com/jackc/pgx/v5/pgconn"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	// CodeUniqueViolation is PostgreSQL SQLSTATE 23505.
	CodeUniqueViolation = "23505"
	// CodeCheckViolation is PostgreSQL SQLSTATE 23514.
	CodeCheckViolation = "23514"
	// CodeForeignKeyViolation is PostgreSQL SQLSTATE 23503.
	CodeForeignKeyViolation = "23503"
	// CodeInvalidTextRepresentation is PostgreSQL SQLSTATE 22P02.
	CodeInvalidTextRepresentation = "22P02"
	// CodeSerializationFailure is PostgreSQL SQLSTATE 40001.
	CodeSerializationFailure = "40001"
	// CodeDeadlockDetected is PostgreSQL SQLSTATE 40P01.
	CodeDeadlockDetected = "40P01"
)

// safeConstraintName keeps a constraint identifier from carrying schema SQL or
// quoted text into response metadata. The allowlist is still the primary gate.
var safeConstraintName = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// Map translates a PostgreSQL constraint/transaction error into a platform
// error. A nil error and any non-PostgreSQL error are returned unchanged.
//
// allowedConstraints is the caller's whitelist of constraint names whose
// identifier may be exposed in Meta. Passing none keeps driver detail out of
// the response entirely; the original PgError stays in the error chain for
// server-side logging.
func Map(err error, allowedConstraints ...string) error {
	if err == nil {
		return nil
	}

	// An application error already carries an intentional code; do not replace
	// it just because a driver error sits in its cause chain.
	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		return err
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr == nil {
		return err
	}

	switch pgErr.Code {
	case CodeUniqueViolation:
		return withConstraint(apperrors.Wrap(
			apperrors.CodeConflict, err,
			"resource conflicts with an existing record",
		), pgErr, allowedConstraints)
	case CodeCheckViolation:
		return withConstraint(apperrors.Wrap(
			apperrors.CodeUnprocessableEntity, err,
			"request violates a data constraint",
		), pgErr, allowedConstraints)
	case CodeForeignKeyViolation:
		return withConstraint(apperrors.Wrap(
			apperrors.CodeUnprocessableEntity, err,
			"request references a related resource that does not exist",
		), pgErr, allowedConstraints)
	case CodeInvalidTextRepresentation:
		return apperrors.Wrap(
			apperrors.CodeUnprocessableEntity, err,
			"request contains a value with an invalid format",
		)
	case CodeSerializationFailure, CodeDeadlockDetected:
		mapped := apperrors.Wrap(
			apperrors.CodeUnavailable, err,
			"database conflict, retry the request",
		)
		mapped.Meta = map[string]any{"retryable": true}
		return mapped
	default:
		return err
	}
}

func withConstraint(mapped *apperrors.Error, pgErr *pgconn.PgError, allowed []string) *apperrors.Error {
	name := pgErr.ConstraintName
	if name == "" || !safeConstraintName.MatchString(name) {
		return mapped
	}
	for _, candidate := range allowed {
		if candidate == name {
			mapped.Meta = map[string]any{"constraint": name}
			return mapped
		}
	}
	return mapped
}
