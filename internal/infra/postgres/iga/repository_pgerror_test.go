package iga

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/iga"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func notFoundRowDB() *shared.MockDBTX {
	return &shared.MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
}

func TestRepository_GetCampaign_NotFound(t *testing.T) {
	_, err := NewRepository(notFoundRowDB()).GetCampaign(context.Background(), "missing", "org-1")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.Equal(t, 404, appErr.HTTPStatus())
}

func TestRepository_GetSODRule_NotFound(t *testing.T) {
	_, err := NewRepository(notFoundRowDB()).GetSODRule(context.Background(), "missing", "org-1")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.Equal(t, 404, appErr.HTTPStatus())
}

func pgErrorExecDB(code, constraint string) *shared.MockDBTX {
	return &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, &pgconn.PgError{
				Code:           code,
				Message:        "driver detail that must not reach the client",
				ConstraintName: constraint,
			}
		},
	}
}

func TestRepository_CreateCampaign_MapsConstraintErrors(t *testing.T) {
	cases := []struct {
		name       string
		pgCode     string
		constraint string
		wantStatus int
	}{
		{"duplicate name is 409", "23505", "uq_access_campaigns_org_name", 409},
		{"bad status is 422", "23514", "ck_access_campaigns_status", 422},
		{"unknown organization is 422", "23503", "fk_access_campaigns_organization_id_organizations", 422},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewRepository(pgErrorExecDB(tc.pgCode, tc.constraint)).
				CreateCampaign(context.Background(), &domain.AccessCampaign{})
			var appErr *apperrors.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, tc.wantStatus, appErr.HTTPStatus())
			assert.NotContains(t, appErr.Message, "driver detail")
		})
	}
}

func TestRepository_CreateSODRule_MapsConstraintErrors(t *testing.T) {
	cases := []struct {
		name       string
		pgCode     string
		constraint string
		wantStatus int
	}{
		{"duplicate name is 409", "23505", "uq_sod_rules_org_name", 409},
		{"duplicate role pair is 409", "23505", "uq_sod_rules_org_roles", 409},
		{"unknown role A is 422", "23503", "fk_sod_rules_role_a_id_roles", 422},
		{"unknown role B is 422", "23503", "fk_sod_rules_role_b_id_roles", 422},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewRepository(pgErrorExecDB(tc.pgCode, tc.constraint)).
				CreateSODRule(context.Background(), &domain.SODRule{})
			var appErr *apperrors.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, tc.wantStatus, appErr.HTTPStatus())
			assert.NotContains(t, appErr.Message, "driver detail")
			if tc.constraint != "" {
				assert.Equal(t, tc.constraint, appErr.Meta["constraint"])
			}
		})
	}
}

func TestRepository_UpdateSODRule_MapsConstraintErrors(t *testing.T) {
	err := NewRepository(pgErrorExecDB("23505", "uq_sod_rules_org_roles")).
		UpdateSODRule(context.Background(), "org-1", "sod-1", "n", "role-a", "role-b")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, 409, appErr.HTTPStatus())
}

func zeroRowsExecDB(tag string) *shared.MockDBTX {
	return &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(tag), nil
		},
	}
}

func TestRepository_CampaignAndSOD_UpdateDelete_ZeroRowsIsNotFound(t *testing.T) {
	cases := []struct {
		name string
		call func(Repository) error
	}{
		{"UpdateCampaignStatus", func(r Repository) error {
			return r.UpdateCampaignStatus(context.Background(), "org-1", "missing", domain.CampaignActive)
		}},
		{"DeleteCampaign", func(r Repository) error {
			return r.DeleteCampaign(context.Background(), "missing", "org-1")
		}},
		{"UpdateSODRule", func(r Repository) error {
			return r.UpdateSODRule(context.Background(), "org-1", "missing", "n", "a", "b")
		}},
		{"DeleteSODRule", func(r Repository) error {
			return r.DeleteSODRule(context.Background(), "missing", "org-1")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call(NewRepository(zeroRowsExecDB("UPDATE 0")))
			var appErr *apperrors.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
			assert.Equal(t, 404, appErr.HTTPStatus())
		})
	}
}

func TestRepository_DeleteSODRule_OneRowSucceeds(t *testing.T) {
	err := NewRepository(zeroRowsExecDB("DELETE 1")).DeleteSODRule(context.Background(), "sod-1", "org-1")
	require.NoError(t, err)
}
