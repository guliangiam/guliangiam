package pgerror_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/pgerror"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func pgErr(code, constraint string) *pgconn.PgError {
	return &pgconn.PgError{
		Code:           code,
		Message:        "driver detail: duplicate key value violates unique constraint",
		Detail:         "Key (code)=(secret) already exists.",
		ConstraintName: constraint,
		TableName:      "coupons",
	}
}

func asAppError(t *testing.T, err error) *apperrors.Error {
	t.Helper()
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	return appErr
}

func TestMapConstraintErrors(t *testing.T) {
	cases := []struct {
		name       string
		pgCode     string
		wantCode   apperrors.Code
		wantStatus int
		retryable  bool
	}{
		{"unique_violation_409", pgerror.CodeUniqueViolation, apperrors.CodeConflict, http.StatusConflict, false},
		{"check_violation_422", pgerror.CodeCheckViolation, apperrors.CodeUnprocessableEntity, http.StatusUnprocessableEntity, false},
		{"foreign_key_violation_422", pgerror.CodeForeignKeyViolation, apperrors.CodeUnprocessableEntity, http.StatusUnprocessableEntity, false},
		{"invalid_text_representation_422", pgerror.CodeInvalidTextRepresentation, apperrors.CodeUnprocessableEntity, http.StatusUnprocessableEntity, false},
		{"serialization_failure_503", pgerror.CodeSerializationFailure, apperrors.CodeUnavailable, http.StatusServiceUnavailable, true},
		{"deadlock_detected_503", pgerror.CodeDeadlockDetected, apperrors.CodeUnavailable, http.StatusServiceUnavailable, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			driverErr := pgErr(tc.pgCode, "uq_coupons_code")
			mapped := pgerror.Map(driverErr, "uq_coupons_code")
			appErr := asAppError(t, mapped)

			assert.Equal(t, tc.wantCode, appErr.Code)
			assert.Equal(t, tc.wantStatus, appErr.HTTPStatus())
			assert.True(t, errors.Is(mapped, driverErr), "driver error must stay in the chain")
			assert.NotContains(t, appErr.Message, driverErr.Message)
			assert.NotContains(t, appErr.Message, driverErr.Detail)
			assert.NotContains(t, appErr.Message, "SQLSTATE")

			if tc.retryable {
				assert.Equal(t, true, appErr.Meta["retryable"])
			} else if tc.pgCode == pgerror.CodeInvalidTextRepresentation {
				assert.Nil(t, appErr.Meta)
			} else {
				assert.Equal(t, "uq_coupons_code", appErr.Meta["constraint"])
			}
		})
	}
}

func TestMapConstraintMetaRequiresAllowlist(t *testing.T) {
	driverErr := pgErr(pgerror.CodeUniqueViolation, "uq_coupons_code")

	appErr := asAppError(t, pgerror.Map(driverErr))
	assert.Nil(t, appErr.Meta, "without an allowlist the constraint must not leak")

	appErr = asAppError(t, pgerror.Map(driverErr, "uq_forms_org_name"))
	assert.Nil(t, appErr.Meta, "a constraint outside the caller's allowlist must not leak")
}

func TestMapConstraintMetaRejectsUnsafeIdentifier(t *testing.T) {
	unsafe := pgErr(pgerror.CodeCheckViolation, `ck_coupons_status"; DROP TABLE coupons`)
	appErr := asAppError(t, pgerror.Map(unsafe, unsafe.ConstraintName))
	assert.Nil(t, appErr.Meta)
}

func TestMapReturnsOtherErrorsUnchanged(t *testing.T) {
	driverErr := pgErr("08006", "")
	got := pgerror.Map(driverErr, "uq_coupons_code")
	assert.Same(t, driverErr, got, "unmapped SQLSTATE must keep the original error")

	networkErr := errors.New("connection reset")
	got = pgerror.Map(networkErr)
	assert.Same(t, networkErr, got, "non-PostgreSQL errors must not be rewrapped")
}

func TestMapNil(t *testing.T) {
	assert.NoError(t, pgerror.Map(nil))
}

func TestMapWrappedDriverError(t *testing.T) {
	driverErr := pgErr(pgerror.CodeUniqueViolation, "uq_forms_org_name")
	wrapped := fmt.Errorf("exec insert form: %w", driverErr)

	appErr := asAppError(t, pgerror.Map(wrapped, "uq_forms_org_name"))
	assert.Equal(t, apperrors.CodeConflict, appErr.Code)
	assert.True(t, errors.Is(appErr, driverErr))
}

func TestMapKeepsExistingApplicationError(t *testing.T) {
	driverErr := pgErr(pgerror.CodeCheckViolation, "ck_coupons_status")
	existing := apperrors.Wrap(apperrors.CodeInvalidArgument, driverErr, "invalid coupon body")

	got := pgerror.Map(existing)
	assert.Same(t, existing, got, "an application error keeps its intentional code")
}
