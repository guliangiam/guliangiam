package organizations

import (
	"context"
	"slices"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// PATCH languages must persist into organization_settings without wiping the
// settings keys that were loaded before the patch.
func TestPatchOrganization_LanguagesPersistedWithExistingSettings(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()

	created, err := svc.CreateOrganization(ctx, CreateOrganizationCommand{
		Slug:                 "patch-languages",
		DisplayName:          "Patch Languages",
		Status:               "active",
		Languages:            []string{"en"},
		DefaultApplicationID: "app-1",
	})
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v, want nil", err)
	}

	err = svc.PatchOrganization(ctx, PatchOrganizationCommand{
		ID:        created.ID,
		Languages: []string{"zh-CN"},
		Revision:  created.Revision,
	})
	if err != nil {
		t.Fatalf("PatchOrganization() error = %v, want nil", err)
	}

	if repo.settings == nil {
		t.Fatal("settings were not persisted")
	}
	if !slices.Equal(repo.settings.Languages, []string{"zh-CN"}) {
		t.Errorf("persisted languages = %v, want [zh-CN]", repo.settings.Languages)
	}
	if repo.settings.DefaultApplicationID != "app-1" {
		t.Errorf("persisted default application = %q, want app-1", repo.settings.DefaultApplicationID)
	}
}

func TestPatchOrganization_InvalidLanguageRejected(t *testing.T) {
	svc, _ := newTestService()

	err := svc.PatchOrganization(context.Background(), PatchOrganizationCommand{
		ID:        "org-1",
		Languages: []string{"xx"},
	})
	if err == nil {
		t.Fatal("PatchOrganization() error = nil, want unsupported language rejection")
	}

	appErr, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("error type = %T, want *apperrors.Error", err)
	}
	if appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeUnprocessableEntity)
	}
}
