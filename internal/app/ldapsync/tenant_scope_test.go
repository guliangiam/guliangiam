package ldapsync

import (
	"context"
	"errors"
	"testing"

	"github.com/guliangiam/guliangiam/internal/domain/ldaps"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
)

func assertLdapSyncInvalidArgument(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func assertLdapSyncNotFound(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// TestSyncConfigByID_RequiresOrganization pins the non-HTTP fail-fast:
// without a trusted tenant the app layer rejects before touching the repo.
func TestSyncConfigByID_RequiresOrganization(t *testing.T) {
	svc := NewService(newFakeSyncRepo(), nil, nil)
	ctx := context.Background()

	_, err := svc.GetSyncConfig(ctx, "  ", "config-1")
	assertLdapSyncInvalidArgument(t, err)

	err = svc.PatchSyncConfig(ctx, "", "config-1", SyncConfigPatch{})
	assertLdapSyncInvalidArgument(t, err)

	err = svc.DeleteSyncConfig(ctx, "", "config-1")
	assertLdapSyncInvalidArgument(t, err)

	_, err = svc.GetSyncLogs(ctx, "", "config-1", 10)
	assertLdapSyncInvalidArgument(t, err)

	_, err = svc.TriggerSync(ctx, "", "config-1")
	assertLdapSyncInvalidArgument(t, err)
}

// TestTriggerSync_ForeignTenantIsNotFound pins that the manual sync trigger
// resolves the config through the scoped read and creates no task for a
// foreign id.
func TestTriggerSync_ForeignTenantIsNotFound(t *testing.T) {
	repo := newFakeSyncRepo()
	repo.configs["config-1"] = ldapSyncConfigForTenant("config-1", "org-foreign")
	svc := NewService(repo, nil, nil)

	_, err := svc.TriggerSync(context.Background(), "org-trusted", "config-1")
	assertLdapSyncNotFound(t, err)
	require.Empty(t, repo.tasks, "a foreign config must not create a sync task")
}

// TestGetSyncConfig_ForeignTenantIsNotFound pins the scoped read.
func TestGetSyncConfig_ForeignTenantIsNotFound(t *testing.T) {
	repo := newFakeSyncRepo()
	repo.configs["config-1"] = ldapSyncConfigForTenant("config-1", "org-foreign")
	svc := NewService(repo, nil, nil)

	_, err := svc.GetSyncConfig(context.Background(), "org-trusted", "config-1")
	assertLdapSyncNotFound(t, err)
}

// TestPatchSyncConfig_ForeignTenantStopsBeforeUpdate pins that ownership is
// decided before mutation: a NotFound read leaves Update uncalled.
func TestPatchSyncConfig_ForeignTenantStopsBeforeUpdate(t *testing.T) {
	repo := newFakeSyncRepo()
	repo.configs["config-1"] = ldapSyncConfigForTenant("config-1", "org-foreign")
	svc := NewService(repo, nil, nil)

	enabled := true
	err := svc.PatchSyncConfig(context.Background(), "org-trusted", "config-1", SyncConfigPatch{Enabled: &enabled})
	assertLdapSyncNotFound(t, err)
	require.False(t, repo.configs["config-1"].Enabled)
}

// TestDeleteSyncConfig_ForeignTenantStopsBeforeDelete pins the pre-read.
func TestDeleteSyncConfig_ForeignTenantStopsBeforeDelete(t *testing.T) {
	repo := newFakeSyncRepo()
	repo.configs["config-1"] = ldapSyncConfigForTenant("config-1", "org-foreign")
	svc := NewService(repo, nil, nil)

	err := svc.DeleteSyncConfig(context.Background(), "org-trusted", "config-1")
	assertLdapSyncNotFound(t, err)
	require.Contains(t, repo.configs, "config-1")
}

// TestGetSyncLogs_ForeignTenantIsNotFound pins that logs are resolved through
// the tenant-scoped config read and then filtered by organization.
func TestGetSyncLogs_ForeignTenantIsNotFound(t *testing.T) {
	repo := newFakeSyncRepo()
	repo.configs["config-1"] = ldapSyncConfigForTenant("config-1", "org-foreign")
	svc := NewService(repo, nil, nil)

	_, err := svc.GetSyncLogs(context.Background(), "org-trusted", "config-1", 10)
	assertLdapSyncNotFound(t, err)
}

func ldapSyncConfigForTenant(id, org string) *ldaps.LdapSyncConfig {
	return &ldaps.LdapSyncConfig{ID: id, OrganizationID: org, LdapID: "ldap-" + id}
}
