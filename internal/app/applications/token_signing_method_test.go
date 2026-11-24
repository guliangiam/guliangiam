package applications

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// token_signing_method is NOT NULL DEFAULT 'RS256'; an entity zero value must
// never reach the repository as '' (GM N8a).

func TestNewApplication_DefaultsTokenSigningMethodToRS256(t *testing.T) {
	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UUID: &fakeUUIDGenerator{fixedID: "app-1"}}

	app, err := svc.NewApplication(CreateApplicationCommand{
		OrganizationID: "org-1",
		Slug:           "my-app",
		DisplayName:    "My App",
		Status:         "active",
	})
	if err != nil {
		t.Fatalf("NewApplication() error = %v", err)
	}
	if app.TokenSigningMethod != domain.TokenSigningMethodRS256 {
		t.Errorf("TokenSigningMethod = %q, want %q", app.TokenSigningMethod, domain.TokenSigningMethodRS256)
	}
}

func TestCreateApplication_MinimalBodyDefaultsTokenSigningMethod(t *testing.T) {
	repo := &mockApplicationRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  &fakeUUIDGenerator{fixedID: "app-1", altID: "client-1"},
		UOW:   testUnitOfWork{appRepo: repo},
	}

	if _, err := svc.CreateApplication(context.Background(), CreateApplicationCommand{
		OrganizationID: "org-1",
		Slug:           "my-app",
		DisplayName:    "My App",
		Status:         "active",
		GrantTypes:     []string{grantTypeAuthorizationCode},
	}); err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	if len(repo.apps) != 1 {
		t.Fatalf("persisted applications = %d, want 1", len(repo.apps))
	}
	if repo.apps[0].TokenSigningMethod != domain.TokenSigningMethodRS256 {
		t.Errorf("persisted TokenSigningMethod = %q, want %q", repo.apps[0].TokenSigningMethod, domain.TokenSigningMethodRS256)
	}
}

func TestNewApplication_RespectsExplicitTokenSigningMethod(t *testing.T) {
	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UUID: &fakeUUIDGenerator{fixedID: "app-1"}}

	app, err := svc.NewApplication(CreateApplicationCommand{
		OrganizationID:     "org-1",
		Slug:               "my-app",
		DisplayName:        "My App",
		Status:             "active",
		TokenSigningMethod: "  " + domain.TokenSigningMethodSM2SM3 + "  ",
	})
	if err != nil {
		t.Fatalf("NewApplication() error = %v", err)
	}
	if app.TokenSigningMethod != domain.TokenSigningMethodSM2SM3 {
		t.Errorf("TokenSigningMethod = %q, want %q", app.TokenSigningMethod, domain.TokenSigningMethodSM2SM3)
	}
}

func TestValidateCreateApplicationCommand_TokenSigningMethod(t *testing.T) {
	t.Run("blank_allowed", func(t *testing.T) {
		command := validApplicationCommandForLimits()
		command.TokenSigningMethod = "   "
		if err := validateCreateApplicationCommand(command); err != nil {
			t.Fatalf("blank tokenSigningMethod must be accepted: %v", err)
		}
	})

	t.Run("unsupported_rejected", func(t *testing.T) {
		command := validApplicationCommandForLimits()
		command.TokenSigningMethod = "ES256"
		err := validateCreateApplicationCommand(command)
		appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
		if appErr.Meta["field"] != "tokenSigningMethod" || appErr.Meta["value"] != "ES256" {
			t.Fatalf("meta = %+v", appErr.Meta)
		}
	})
}

func TestPatchApplication_BlankTokenSigningMethodNormalizesToRS256(t *testing.T) {
	for _, existing := range []string{"", domain.TokenSigningMethodSM2SM3} {
		t.Run("from_"+existing, func(t *testing.T) {
			now := time.Now()
			repo := &mockApplicationRepo{apps: []*domain.Application{{
				ID:                 "app-1",
				OrganizationID:     "org-1",
				Slug:               "test-app",
				DisplayName:        "Test App",
				Status:             "active",
				TokenSigningMethod: existing,
				Revision:           1,
				CreatedAt:          now,
				UpdatedAt:          now,
			}}}
			svc := Service{
				Clock: fakeClock{fixedTime: now},
				UUID:  &fakeUUIDGenerator{fixedID: "uuid-1"},
				UOW:   testUnitOfWork{appRepo: repo},
			}

			blank := "   "
			if err := svc.PatchApplication(context.Background(), PatchApplicationCommand{
				ID:                 "app-1",
				OrganizationID:     "org-1",
				TokenSigningMethod: &blank,
			}); err != nil {
				t.Fatalf("PatchApplication() error = %v", err)
			}
			if repo.apps[0].TokenSigningMethod != domain.TokenSigningMethodRS256 {
				t.Errorf("TokenSigningMethod = %q, want %q", repo.apps[0].TokenSigningMethod, domain.TokenSigningMethodRS256)
			}
		})
	}
}

func TestPatchApplication_RejectsUnsupportedTokenSigningMethod(t *testing.T) {
	unsupported := "ES256"
	svc := Service{}
	err := svc.PatchApplication(context.Background(), PatchApplicationCommand{
		ID:                 "app-1",
		OrganizationID:     "org-1",
		TokenSigningMethod: &unsupported,
	})
	appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
	if appErr.Meta["field"] != "tokenSigningMethod" || appErr.Meta["value"] != unsupported {
		t.Fatalf("meta = %+v", appErr.Meta)
	}
}

// Import lands in the same column create validates, so a payload carrying an
// out-of-enum method must be rejected before the INSERT — otherwise the value
// would be stored and later fail the enum CHECK (GM N8a).
func TestImportApplication_RejectsUnsupportedTokenSigningMethod(t *testing.T) {
	repo := &mockApplicationRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  &fakeUUIDGenerator{fixedID: "app-1", altID: "client-1"},
		UOW:   testUnitOfWork{appRepo: repo},
	}

	_, err := svc.ImportApplication(context.Background(), ImportPayload{
		OrganizationID: "org-1",
		Version:        ExportVersion,
		Export: ApplicationExportDTO{
			Slug:               "imported-app",
			DisplayName:        "Imported App",
			Status:             "active",
			TokenSigningMethod: "ES256",
		},
	})
	appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
	if appErr.Meta["field"] != "tokenSigningMethod" || appErr.Meta["value"] != "ES256" {
		t.Fatalf("meta = %+v", appErr.Meta)
	}
	if len(repo.apps) != 0 {
		t.Fatalf("persisted applications = %d, want 0", len(repo.apps))
	}
}

func TestImportApplication_NormalizesBlankTokenSigningMethod(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		value string
		want  string
	}{
		{"empty", "", domain.TokenSigningMethodRS256},
		{"whitespace", "   ", domain.TokenSigningMethodRS256},
		{"explicit", domain.TokenSigningMethodSM2SM3, domain.TokenSigningMethodSM2SM3},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &mockApplicationRepo{}
			svc := Service{
				Clock: fakeClock{fixedTime: time.Now()},
				UUID:  &fakeUUIDGenerator{fixedID: "app-1", altID: "client-1"},
				UOW:   testUnitOfWork{appRepo: repo},
			}

			app, err := svc.ImportApplication(context.Background(), ImportPayload{
				OrganizationID: "org-1",
				Version:        ExportVersion,
				Export: ApplicationExportDTO{
					Slug:                    "imported-app",
					DisplayName:             "Imported App",
					Status:                  "active",
					TokenEndpointAuthMethod: "none",
					GrantTypes:              []string{grantTypeAuthorizationCode},
					TokenSigningMethod:      testCase.value,
				},
			})
			if err != nil {
				t.Fatalf("ImportApplication() error = %v", err)
			}
			if app.TokenSigningMethod != testCase.want {
				t.Errorf("TokenSigningMethod = %q, want %q", app.TokenSigningMethod, testCase.want)
			}
		})
	}
}
