package applications

import (
	"context"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// The export DTO carries no client material (no JWKS, certificate or secret),
// so importing private_key_jwt or either mTLS method would create a client the
// token endpoint can never authenticate. The import path must reject those with
// an identifiable 422 before the repository is touched.
func TestImportApplication_RejectsMethodsThatNeedClientMaterial(t *testing.T) {
	cases := []string{
		"private_key_jwt",
		"tls_client_auth",
		"self_signed_tls_client_auth",
	}

	for _, method := range cases {
		t.Run(method, func(t *testing.T) {
			repo := &mockApplicationRepo{}
			svc := Service{Clock: fakeClock{}, UUID: &fakeUUIDGenerator{fixedID: "new-uuid"}, UOW: testUnitOfWork{appRepo: repo}}

			_, err := svc.ImportApplication(context.Background(), ImportPayload{
				OrganizationID: "org1",
				Export: ApplicationExportDTO{
					Slug:                    "imported-app",
					DisplayName:             "Imported App",
					TokenEndpointAuthMethod: method,
					RedirectURIs:            []string{"https://imp.example.com/cb"},
					GrantTypes:              []string{grantTypeAuthorizationCode},
				},
			})
			appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
			if appErr.Meta[metaKeyField] != "tokenEndpointAuthMethod" {
				t.Fatalf("meta[field] = %v, want tokenEndpointAuthMethod (meta = %+v)", appErr.Meta[metaKeyField], appErr.Meta)
			}
			if appErr.Meta[metaKeyValue] != method {
				t.Fatalf("meta[value] = %v, want %s", appErr.Meta[metaKeyValue], method)
			}
			if appErr.Meta["expected"] != "client_material_via_console" {
				t.Fatalf("meta[expected] = %v, want client_material_via_console", appErr.Meta["expected"])
			}
			if len(repo.apps) != 0 {
				t.Fatalf("rejected import still wrote %d applications", len(repo.apps))
			}
		})
	}
}

// client_secret_jwt is outside the supported closed set (OPEN-API-60); an
// export carrying it is rejected instead of being silently normalized to none.
func TestImportApplication_RejectsUnsupportedMethod(t *testing.T) {
	repo := &mockApplicationRepo{}
	svc := Service{Clock: fakeClock{}, UUID: &fakeUUIDGenerator{fixedID: "new-uuid"}, UOW: testUnitOfWork{appRepo: repo}}

	_, err := svc.ImportApplication(context.Background(), ImportPayload{
		OrganizationID: "org1",
		Export: ApplicationExportDTO{
			Slug:                    "imported-app",
			DisplayName:             "Imported App",
			TokenEndpointAuthMethod: "client_secret_jwt",
			RedirectURIs:            []string{"https://imp.example.com/cb"},
			GrantTypes:              []string{grantTypeAuthorizationCode},
		},
	})
	appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
	if appErr.Meta["allowed"] == nil {
		t.Fatalf("meta[allowed] is missing: %+v", appErr.Meta)
	}
	if len(repo.apps) != 0 {
		t.Fatalf("rejected import still wrote %d applications", len(repo.apps))
	}
}

func TestImportApplication_AcceptsSupportedMethods(t *testing.T) {
	cases := []string{
		"",
		"none",
		"client_secret_basic",
		"client_secret_post",
	}

	for _, method := range cases {
		t.Run("method_"+method, func(t *testing.T) {
			repo := &mockApplicationRepo{}
			svc := Service{Clock: fakeClock{}, UUID: &fakeUUIDGenerator{fixedID: "new-uuid"}, UOW: testUnitOfWork{appRepo: repo}}

			app, err := svc.ImportApplication(context.Background(), ImportPayload{
				OrganizationID: "org1",
				Export: ApplicationExportDTO{
					Slug:                    "imported-app",
					DisplayName:             "Imported App",
					TokenEndpointAuthMethod: method,
					RedirectURIs:            []string{"https://imp.example.com/cb"},
					GrantTypes:              []string{grantTypeAuthorizationCode},
				},
			})
			if err != nil {
				t.Fatalf("ImportApplication() error = %v", err)
			}
			if app.TokenEndpointAuthMethod != method {
				t.Fatalf("persisted method = %q, want %q", app.TokenEndpointAuthMethod, method)
			}
			if len(repo.apps) != 1 {
				t.Fatalf("imported applications = %d, want 1", len(repo.apps))
			}
		})
	}
}

// Normalization is applied before the closed-set check, so an export written
// with uppercase metadata still lands on the lowercase enum the token endpoint
// compares against.
func TestImportApplication_NormalizesMethodCase(t *testing.T) {
	repo := &mockApplicationRepo{}
	svc := Service{Clock: fakeClock{}, UUID: &fakeUUIDGenerator{fixedID: "new-uuid"}, UOW: testUnitOfWork{appRepo: repo}}

	app, err := svc.ImportApplication(context.Background(), ImportPayload{
		OrganizationID: "org1",
		Export: ApplicationExportDTO{
			Slug:                    "imported-app",
			DisplayName:             "Imported App",
			TokenEndpointAuthMethod: "Client_Secret_Basic",
			RedirectURIs:            []string{"https://imp.example.com/cb"},
			GrantTypes:              []string{grantTypeAuthorizationCode},
		},
	})
	if err != nil {
		t.Fatalf("ImportApplication() error = %v", err)
	}
	if app.TokenEndpointAuthMethod != "client_secret_basic" {
		t.Fatalf("persisted method = %q, want client_secret_basic", app.TokenEndpointAuthMethod)
	}
}
