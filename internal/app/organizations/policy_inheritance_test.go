package organizations

import (
	"context"
	"testing"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePolicyInheritanceRepository map[string][]orgdomain.PolicyInheritanceRule

func (f fakePolicyInheritanceRepository) ListPolicyInheritance(_ context.Context, organizationID string) ([]orgdomain.PolicyInheritanceRule, error) {
	return f[organizationID], nil
}

func TestPreviewPolicyInheritance_RootToChild(t *testing.T) {
	service, repository := newTestService()
	repository.items["root"] = &orgdomain.Organization{
		ID: "root", Slug: "root", DisplayName: "Root",
		Security: orgdomain.OrganizationSecurityPolicy{PasswordPolicy: map[string]any{"minLength": 12}},
	}
	repository.items["child"] = &orgdomain.Organization{
		ID: "child", ParentID: "root", Slug: "child", DisplayName: "Child",
	}
	service.PolicyInheritance = fakePolicyInheritanceRepository{
		"root":  {{PolicyKey: "passwordPolicy", Mode: orgdomain.PolicyModeLock}},
		"child": {{PolicyKey: "passwordPolicy", Mode: orgdomain.PolicyModeOverride}},
	}

	preview, err := service.PreviewPolicyInheritance(context.Background(), "child", "passwordPolicy")
	require.NoError(t, err)
	assert.True(t, preview.Locked)
	assert.Equal(t, "root", preview.LockedByOrganizationID)
	assert.Equal(t, "root", preview.EffectiveSourceOrganizationID)
	assert.Equal(t, 12, preview.EffectiveValue.(map[string]any)["minLength"])
	require.Len(t, preview.Chain, 2)
	assert.True(t, preview.Chain[1].Rejected)
}

func TestPreviewPolicyInheritance_UnknownKey(t *testing.T) {
	service, _ := newTestService()
	_, err := service.PreviewPolicyInheritance(context.Background(), "org-1", "unknownPolicy")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
}

func TestPreviewPolicyInheritance_NotFound(t *testing.T) {
	service, _ := newTestService()
	_, err := service.PreviewPolicyInheritance(context.Background(), "missing", "passwordPolicy")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestUpdateSecurityPolicy_AncestorLockForbidden(t *testing.T) {
	service, repository := newTestService()
	repository.items["root"] = &orgdomain.Organization{
		ID: "root", Slug: "root", DisplayName: "Root",
		Security: orgdomain.OrganizationSecurityPolicy{PasswordPolicy: map[string]any{"minLength": 12}},
	}
	repository.items["child"] = &orgdomain.Organization{
		ID: "child", ParentID: "root", Slug: "child", DisplayName: "Child",
	}
	service.PolicyInheritance = fakePolicyInheritanceRepository{
		"root": {{PolicyKey: "passwordPolicy", Mode: orgdomain.PolicyModeLock}},
	}

	err := service.UpdateOrganizationSecurityPolicy(context.Background(), UpdateOrganizationSecurityPolicyCommand{
		OrganizationID: "child",
		PasswordPolicy: map[string]any{"minLength": 8},
	})
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeForbidden, appErr.Code)
}

func TestPreviewPolicyInheritance_Theme(t *testing.T) {
	service, repository := newTestService()
	repository.items["root"] = &orgdomain.Organization{
		ID: "root", Slug: "root", DisplayName: "Root",
		Theme: orgdomain.OrganizationThemeSettings{ThemeName: "root-theme", Tokens: map[string]any{"primaryColor": "#123456"}},
	}
	repository.items["child"] = &orgdomain.Organization{
		ID: "child", ParentID: "root", Slug: "child", DisplayName: "Child",
	}
	service.PolicyInheritance = fakePolicyInheritanceRepository{}
	preview, err := service.PreviewPolicyInheritance(context.Background(), "child", "theme")
	require.NoError(t, err)
	assert.Equal(t, "root", preview.EffectiveSourceOrganizationID)
	assert.Equal(t, "root-theme", preview.EffectiveValue.(map[string]any)["themeName"])
}

func TestUpdateTheme_AncestorLockForbidden(t *testing.T) {
	service, repository := newTestService()
	repository.items["root"] = &orgdomain.Organization{ID: "root", Slug: "root", DisplayName: "Root"}
	repository.items["child"] = &orgdomain.Organization{ID: "child", ParentID: "root", Slug: "child", DisplayName: "Child"}
	service.PolicyInheritance = fakePolicyInheritanceRepository{
		"root": {{PolicyKey: "theme", Mode: orgdomain.PolicyModeLock}},
	}
	err := service.UpdateOrganizationTheme(context.Background(), UpdateOrganizationThemeCommand{
		OrganizationID: "child", ThemeName: "child-theme",
	})
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeForbidden, appErr.Code)
}

func TestUpdateSettings_AncestorLockForbidden(t *testing.T) {
	service, repository := newTestService()
	repository.items["root"] = &orgdomain.Organization{ID: "root", Slug: "root", DisplayName: "Root"}
	repository.items["child"] = &orgdomain.Organization{ID: "child", ParentID: "root", Slug: "child", DisplayName: "Child"}
	service.PolicyInheritance = fakePolicyInheritanceRepository{
		"root": {{PolicyKey: "accountItems", Mode: orgdomain.PolicyModeLock}},
	}
	err := service.UpdateOrganizationSettings(context.Background(), UpdateOrganizationSettingsCommand{
		OrganizationID: "child",
	})
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeForbidden, appErr.Code)
}
