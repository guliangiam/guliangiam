package organizations

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func TestRepository_Create_Success(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.Organization{}
	err := repo.Create(context.Background(), entity)
	require.NoError(t, err)
}

func TestRepository_Create_Error(t *testing.T) {
	mockDB := &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("db error")
		},
	}
	repo := NewRepository(mockDB)
	entity := &domain.Organization{}
	err := repo.Create(context.Background(), entity)
	require.Error(t, err)
}

// TestRepository_Create_InsertArgumentsMatchColumnOrder pins the core INSERT
// argument order to the declared column list. The previous defect passed
// ParentID (empty string for root organizations) into the created_at
// timestamptz slot, so PostgreSQL rejected every create with
// "invalid input syntax for type timestamp with time zone: \"\"".
//
// The test runs entirely against shared.MockDBTX: it needs no DSN and is not
// skipped when no database is available.
func TestRepository_Create_InsertArgumentsMatchColumnOrder(t *testing.T) {
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)

	tests := []struct {
		name     string
		parentID string
	}{
		{name: "root organization without parentId", parentID: ""},
		{name: "child organization with parentId", parentID: "01900000-0000-7000-8000-0000000000aa"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var coreSQL string
			var coreArgs []any
			execCount := 0
			mockDB := &shared.MockDBTX{
				ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					execCount++
					if strings.Contains(sql, "INSERT INTO guliangiam.organizations (") {
						coreSQL = sql
						coreArgs = append([]any(nil), args...)
					}
					return pgconn.NewCommandTag("INSERT 1"), nil
				},
			}

			entity := &domain.Organization{
				ID:                 "01900000-0000-7000-8000-000000000001",
				ParentID:           tt.parentID,
				Slug:               "acme",
				DisplayName:        "Acme",
				WebsiteURL:         "https://acme.example.com",
				LogoURL:            "https://acme.example.com/logo.png",
				LogoDarkURL:        "https://acme.example.com/logo-dark.png",
				FaviconURL:         "https://acme.example.com/favicon.ico",
				Status:             "active",
				InitScore:          42,
				DefaultAvatar:      "https://acme.example.com/avatar.png",
				DisableSignin:      true,
				EnableSoftDeletion: true,
				DisableSignup:      true,
				IpWhitelist:        "10.0.0.0/8",
				InitiateLoginURL:   "https://acme.example.com/login",
				RegionCode:         "cn-east-1",
				CreatedAt:          createdAt,
				UpdatedAt:          updatedAt,
				Revision:           1,
			}

			require.NoError(t, NewRepository(mockDB).Create(context.Background(), entity))
			require.NotEmpty(t, coreSQL, "organizations core INSERT was not executed")
			require.Equal(t, 5, execCount, "create must write the core row and the four sub-tables")

			columns := parseInsertColumns(t, coreSQL)
			require.Len(t, coreArgs, len(columns), "argument count must match INSERT column count")

			byColumn := make(map[string]any, len(columns))
			placeholders := make(map[string]int, len(columns))
			for i, column := range columns {
				byColumn[column] = coreArgs[i]
				placeholders[column] = i + 1
			}

			require.Equal(t, entity.ID, byColumn["id"])
			require.Equal(t, entity.Slug, byColumn["slug"])
			require.Equal(t, entity.DisplayName, byColumn["display_name"])
			require.Equal(t, entity.Status, byColumn["status"])
			require.EqualValues(t, entity.InitScore, byColumn["init_score"])
			require.Equal(t, entity.RegionCode, byColumn["region_code"])
			require.Equal(t, entity.ParentID, byColumn["parent_id"])

			// created_at / updated_at are timestamptz columns and must receive
			// time.Time values, never the parent id string.
			gotCreatedAt, ok := byColumn["created_at"].(time.Time)
			require.Truef(t, ok, "created_at must receive time.Time, got %T", byColumn["created_at"])
			require.True(t, gotCreatedAt.Equal(createdAt))

			gotUpdatedAt, ok := byColumn["updated_at"].(time.Time)
			require.Truef(t, ok, "updated_at must receive time.Time, got %T", byColumn["updated_at"])
			require.True(t, gotUpdatedAt.Equal(updatedAt))

			require.EqualValues(t, entity.Revision, byColumn["revision"])

			// parent_id is a nullable UUID column; the domain encodes "root" as "".
			// The statement must convert it to NULL instead of sending "" to uuid.
			parentConversion := regexp.MustCompile(`NULLIF\(\$(\d+), ''\)::uuid`)
			match := parentConversion.FindStringSubmatch(coreSQL)
			require.NotNil(t, match, "parent_id must be written through NULLIF($n, '')::uuid")
			require.Equal(t, strconv.Itoa(placeholders["parent_id"]), match[1])
		})
	}
}

// TestRepository_Create_NilStringSlicesBecomeEmptyArrays pins the 23502 fix:
// pgx encodes a nil []string as SQL NULL, and an explicit NULL bypasses the
// `DEFAULT '{}'` of the NOT NULL varchar[] columns in organization_auth_config
// (password_options $3, ldap_attributes $11). Create must bind empty arrays.
func TestRepository_Create_NilStringSlicesBecomeEmptyArrays(t *testing.T) {
	tests := []struct {
		name            string
		passwordOptions []string
		ldapAttributes  []string
		wantOptions     []string
		wantLdap        []string
	}{
		{name: "nil slices are normalized", passwordOptions: nil, ldapAttributes: nil, wantOptions: []string{}, wantLdap: []string{}},
		{name: "empty slices stay empty", passwordOptions: []string{}, ldapAttributes: []string{}, wantOptions: []string{}, wantLdap: []string{}},
		{name: "provided values pass through", passwordOptions: []string{"minLength", "requireUpper"}, ldapAttributes: []string{"memberOf"}, wantOptions: []string{"minLength", "requireUpper"}, wantLdap: []string{"memberOf"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var authArgs []any
			mockDB := &shared.MockDBTX{
				ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					if strings.Contains(sql, "organization_auth_config") {
						authArgs = append([]any(nil), args...)
					}
					return pgconn.NewCommandTag("INSERT 1"), nil
				},
			}

			entity := &domain.Organization{
				ID:              "01900000-0000-7000-8000-000000000001",
				PasswordOptions: tt.passwordOptions,
				LdapAttributes:  tt.ldapAttributes,
			}
			require.NoError(t, NewRepository(mockDB).Create(context.Background(), entity))

			require.Len(t, authArgs, 11, "organization_auth_config upsert argument count")
			require.Equal(t, tt.wantOptions, authArgs[2], "password_options must never bind as NULL")
			require.Equal(t, tt.wantLdap, authArgs[10], "ldap_attributes must never bind as NULL")
		})
	}
}

// parseInsertColumns extracts the column list of an INSERT statement.
func parseInsertColumns(t *testing.T, statement string) []string {
	t.Helper()
	start := strings.Index(statement, "(")
	end := strings.Index(statement, ") VALUES")
	require.Greater(t, start, 0)
	require.Greater(t, end, start)

	raw := strings.Split(statement[start+1:end], ",")
	columns := make([]string, 0, len(raw))
	for _, column := range raw {
		columns = append(columns, strings.TrimSpace(column))
	}
	return columns
}
