package applications

import (
	"context"
	"testing"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Export 1.1 输出「有效 token endpoint 认证方式」：legacy 行（method 为空但
// 有 secret hash）不再导出空 method，避免导入后被归一为 public client。
func TestExportApplication_UsesEffectiveAuthMethodForLegacyRows(t *testing.T) {
	app := applicationStub("legacy-app")
	app.Slug = "legacy-export"
	app.TokenEndpointAuthMethod = ""
	app.ClientSecretHash = "stored-secret-hash"
	repo := &mockApplicationRepo{apps: []*applicationdomain.Application{app}}
	svc := Service{Clock: fakeClock{}, UOW: testUnitOfWork{appRepo: repo}}

	payload, err := svc.ExportApplication(context.Background(), "org1", "legacy-app")
	if err != nil {
		t.Fatalf("ExportApplication() error = %v", err)
	}
	if payload.Version != ExportVersion {
		t.Fatalf("export version = %q, want %q", payload.Version, ExportVersion)
	}
	if payload.Export.TokenEndpointAuthMethod != "client_secret_basic" {
		t.Fatalf("exported method = %q, want client_secret_basic (effective method)", payload.Export.TokenEndpointAuthMethod)
	}
}

// 未知导出版本拒绝导入（422），不会落到仓储。
func TestImportApplication_RejectsUnknownExportVersion(t *testing.T) {
	repo := &mockApplicationRepo{}
	svc := Service{Clock: fakeClock{}, UUID: &fakeUUIDGenerator{fixedID: "new-uuid"}, UOW: testUnitOfWork{appRepo: repo}}

	_, err := svc.ImportApplication(context.Background(), ImportPayload{
		Version:        "9.9",
		OrganizationID: "org1",
		Export: ApplicationExportDTO{
			Slug:                    "imported-app",
			DisplayName:             "Imported App",
			TokenEndpointAuthMethod: "none",
			RedirectURIs:            []string{"https://imp.example.com/cb"},
			GrantTypes:              []string{grantTypeAuthorizationCode},
		},
	})
	appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
	if appErr.Meta["field"] != "version" {
		t.Fatalf("meta[field] = %v, want version", appErr.Meta["field"])
	}
	if len(repo.apps) != 0 {
		t.Fatalf("rejected import still wrote %d applications", len(repo.apps))
	}
}

// 1.0（或空版本）保持向后兼容：旧文件仍可导入。
func TestImportApplication_AcceptsLegacyExportVersion(t *testing.T) {
	for _, version := range []string{"", ExportVersionLegacy} {
		t.Run("version_"+version, func(t *testing.T) {
			repo := &mockApplicationRepo{}
			svc := Service{Clock: fakeClock{}, UUID: &fakeUUIDGenerator{fixedID: "new-uuid"}, UOW: testUnitOfWork{appRepo: repo}}

			_, err := svc.ImportApplication(context.Background(), ImportPayload{
				Version:        version,
				OrganizationID: "org1",
				Export: ApplicationExportDTO{
					Slug:                    "imported-app",
					DisplayName:             "Imported App",
					TokenEndpointAuthMethod: "none",
					RedirectURIs:            []string{"https://imp.example.com/cb"},
					GrantTypes:              []string{grantTypeAuthorizationCode},
				},
			})
			if err != nil {
				t.Fatalf("ImportApplication(version=%q) error = %v", version, err)
			}
			if len(repo.apps) != 1 {
				t.Fatalf("imported applications = %d, want 1", len(repo.apps))
			}
		})
	}
}
