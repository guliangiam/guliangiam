//go:build integration

package applications

import (
	"context"
	"strings"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/testutil"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestApplicationRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	tc := testutil.SetupPostgres(ctx, t)
	defer tc.Cleanup(ctx)

	pool := tc.ConnectPool(ctx, t)
	repo := NewRepository(pool)

	now := time.Now()
	orgID := "01900000-0000-7000-8000-000000000200"

	t.Run("CreateAndGetByID", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		app := &domain.Application{
			ID:                        "01900000-0000-7000-8000-000000000001",
			OrganizationID:            orgID,
			Slug:                      "test-app",
			DisplayName:               "Test Application",
			Category:                  "web",
			Status:                    "active",
			ClientID:                  "client-test-001",
			ClientSecretHash:          "secrethash123",
			RedirectURIs:              []string{"https://example.com/callback"},
			GrantTypes:                []string{"authorization_code", "refresh_token"},
			TokenEndpointAuthMethod:   "client_secret_basic",
			ExpireInHours:             24,
			RefreshTokenExpireInHours: 168,
			CreatedAt:                 now,
			UpdatedAt:                 now,
			Revision:                  1,
		}

		if err := repo.Create(ctx, app); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		got, err := repo.GetByID(ctx, app.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.ID != app.ID {
			t.Errorf("got.ID = %q, want %q", got.ID, app.ID)
		}
		if got.DisplayName != app.DisplayName {
			t.Errorf("got.DisplayName = %q, want %q", got.DisplayName, app.DisplayName)
		}
		if got.ClientID != app.ClientID {
			t.Errorf("got.ClientID = %q, want %q", got.ClientID, app.ClientID)
		}
	})

	t.Run("GetByClientID", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		app := &domain.Application{
			ID:                      "01900000-0000-7000-8000-000000000002",
			OrganizationID:          orgID,
			Slug:                    "clientid-app",
			DisplayName:             "ClientID Test App",
			Status:                  "active",
			ClientID:                "unique-client-id-002",
			ClientSecretHash:        "secrethash456",
			TokenEndpointAuthMethod: "client_secret_basic",
			CreatedAt:               now,
			UpdatedAt:               now,
			Revision:                1,
		}
		if err := repo.Create(ctx, app); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		got, err := repo.GetByClientID(ctx, "unique-client-id-002")
		if err != nil {
			t.Fatalf("GetByClientID() error = %v", err)
		}
		if got.ID != app.ID {
			t.Errorf("got.ID = %q, want %q", got.ID, app.ID)
		}
	})

	t.Run("GetByID_NotFound", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		_, err := repo.GetByID(ctx, "01900000-0000-7000-8000-999999999999")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		appErr, ok := err.(*apperrors.Error)
		if !ok || appErr.Code != apperrors.CodeNotFound {
			t.Errorf("error = %v, want not_found", err)
		}
	})

	t.Run("List", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		for i := 0; i < 3; i++ {
			app := &domain.Application{
				ID:                      "01900000-0000-7000-8000-00000000001" + string(rune('0'+i)),
				OrganizationID:          orgID,
				Slug:                    "list-app-" + string(rune('a'+i)),
				DisplayName:             "App " + string(rune('A'+i)),
				Status:                  "active",
				ClientID:                "list-client-" + string(rune('0'+i)),
				ClientSecretHash:        "listsecret" + string(rune('0'+i)),
				TokenEndpointAuthMethod: "client_secret_basic",
				CreatedAt:               now,
				UpdatedAt:               now,
				Revision:                1,
			}
			if err := repo.Create(ctx, app); err != nil {
				t.Fatalf("Create(%d) error = %v", i, err)
			}
		}

		items, total, err := repo.List(ctx, domain.ListQuery{
			Page:           1,
			PageSize:       10,
			OrganizationID: orgID,
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
		if len(items) != 3 {
			t.Errorf("len(items) = %d, want 3", len(items))
		}
	})

	t.Run("Update", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		app := &domain.Application{
			ID:                      "01900000-0000-7000-8000-000000000003",
			OrganizationID:          orgID,
			Slug:                    "update-app",
			DisplayName:             "Before Update",
			Status:                  "active",
			ClientID:                "update-client-003",
			ClientSecretHash:        "updatesecret003",
			TokenEndpointAuthMethod: "client_secret_basic",
			CreatedAt:               now,
			UpdatedAt:               now,
			Revision:                1,
		}
		if err := repo.Create(ctx, app); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		app.DisplayName = "After Update"
		app.UpdatedAt = now.Add(time.Minute)
		if err := repo.Update(ctx, app); err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		got, err := repo.GetByID(ctx, app.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.DisplayName != "After Update" {
			t.Errorf("got.DisplayName = %q, want %q", got.DisplayName, "After Update")
		}
		if got.Revision != 2 {
			t.Errorf("got.Revision = %d, want 2", got.Revision)
		}
	})

	t.Run("ReplaceProviderBindings", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		app := &domain.Application{
			ID:                      "01900000-0000-7000-8000-000000000004",
			OrganizationID:          orgID,
			Slug:                    "binding-app",
			DisplayName:             "Binding Test App",
			Status:                  "active",
			ClientID:                "binding-client-004",
			ClientSecretHash:        "bindingsecret004",
			TokenEndpointAuthMethod: "client_secret_basic",
			CreatedAt:               now,
			UpdatedAt:               now,
			Revision:                1,
		}
		if err := repo.Create(ctx, app); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		bindings := []domain.ApplicationProviderBinding{
			{ApplicationID: app.ID, ProviderID: "01900000-0000-7000-8000-000000000070"},
			{ApplicationID: app.ID, ProviderID: "01900000-0000-7000-8000-000000000071"},
		}
		if err := repo.ReplaceProviderBindings(ctx, app.ID, bindings); err != nil {
			t.Fatalf("ReplaceProviderBindings() error = %v", err)
		}

		got, err := repo.GetByID(ctx, app.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if len(got.ProviderBindings) != 2 {
			t.Errorf("len(got.ProviderBindings) = %d, want 2", len(got.ProviderBindings))
		}
	})

	t.Run("Delete", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		app := &domain.Application{
			ID:                      "01900000-0000-7000-8000-000000000005",
			OrganizationID:          orgID,
			Slug:                    "delete-app",
			DisplayName:             "Delete Test App",
			Status:                  "active",
			ClientID:                "delete-client-005",
			ClientSecretHash:        "deletesecret005",
			TokenEndpointAuthMethod: "client_secret_basic",
			CreatedAt:               now,
			UpdatedAt:               now,
			Revision:                1,
		}
		if err := repo.Create(ctx, app); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if err := repo.Delete(ctx, app.ID, app.OrganizationID); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		_, err := repo.GetByID(ctx, app.ID)
		if err == nil {
			t.Fatal("expected error after delete, got nil")
		}
	})

	t.Run("SamlMetadataRoundTripMultiKB", func(t *testing.T) {
		if err := testutil.TruncateAllAndSeedOrganization(ctx, pool); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		if _, err := pool.Exec(ctx, `INSERT INTO guliangiam.organizations (id, slug, display_name)
			VALUES ($1, 'saml-metadata-org', 'SAML Metadata Org')
			ON CONFLICT (id) DO NOTHING`, orgID); err != nil {
			t.Fatalf("seed organization: %v", err)
		}

		metadata := `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://idp.example.com">` +
			strings.Repeat(`<KeyDescriptor use="signing"/>`, 512) + `</EntityDescriptor>`
		if len(metadata) < 4096 {
			t.Fatalf("fixture metadata is only %d bytes, want at least 4KB", len(metadata))
		}
		metadataURL := "https://idp.example.com/metadata?tenant=" + strings.Repeat("a", 512)

		app := &domain.Application{
			ID:                      "01900000-0000-7000-8000-000000000006",
			OrganizationID:          orgID,
			Slug:                    "saml-metadata-app",
			DisplayName:             "SAML Metadata App",
			Status:                  "active",
			ClientID:                "saml-metadata-client-006",
			ClientSecretHash:        "samlmetadatasecret006",
			TokenEndpointAuthMethod: "client_secret_basic",
			SamlMetadata:            metadata,
			SamlMetadataUrl:         metadataURL,
			CreatedAt:               now,
			UpdatedAt:               now,
			Revision:                1,
		}
		if err := repo.Create(ctx, app); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		got, err := repo.GetByID(ctx, app.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.SamlMetadata != metadata {
			t.Errorf("saml_metadata round-trip length = %d, want %d", len(got.SamlMetadata), len(metadata))
		}
		if got.SamlMetadataUrl != metadataURL {
			t.Errorf("saml_metadata_url round-trip length = %d, want %d", len(got.SamlMetadataUrl), len(metadataURL))
		}

		var storedMetadataLength, storedURLLength int
		err = pool.QueryRow(ctx, `SELECT length(saml_metadata), length(saml_metadata_url)
			FROM guliangiam.app_saml_config WHERE application_id = $1`, app.ID).
			Scan(&storedMetadataLength, &storedURLLength)
		if err != nil {
			t.Fatalf("query stored lengths: %v", err)
		}
		if storedMetadataLength != len(metadata) {
			t.Errorf("length(saml_metadata) = %d, want %d", storedMetadataLength, len(metadata))
		}
		if storedURLLength != len(metadataURL) {
			t.Errorf("length(saml_metadata_url) = %d, want %d", storedURLLength, len(metadataURL))
		}
	})
}
