package ldapsync

import (
	"context"
	"testing"
	"time"

	ldapsmock "github.com/guliangiam/guliangiam/internal/domain/ldaps/mock"
	usersmock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/guliangiam/guliangiam/internal/domain/ldaps"
)

// ─── fakeSyncRepo implements ldaps.SyncRepository for testing ─────────────

type fakeSyncRepo struct {
	configs map[string]*ldaps.LdapSyncConfig
	tasks   []*ldaps.LdapSyncTask
}

func newFakeSyncRepo() *fakeSyncRepo {
	return &fakeSyncRepo{configs: make(map[string]*ldaps.LdapSyncConfig)}
}

func (r *fakeSyncRepo) GetSyncConfigByID(_ context.Context, id string) (*ldaps.LdapSyncConfig, error) {
	c, ok := r.configs[id]
	if !ok {
		return nil, errNotFound("sync config not found")
	}
	return c, nil
}

func (r *fakeSyncRepo) GetSyncConfigByIDForOrganization(_ context.Context, id, organizationID string) (*ldaps.LdapSyncConfig, error) {
	c, ok := r.configs[id]
	if !ok || c.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("sync config not found")
	}
	return c, nil
}

func (r *fakeSyncRepo) ListSyncConfigs(_ context.Context, organizationID string) ([]*ldaps.LdapSyncConfig, error) {
	var result []*ldaps.LdapSyncConfig
	for _, c := range r.configs {
		if c.OrganizationID == organizationID {
			result = append(result, c)
		}
	}
	return result, nil
}

func (r *fakeSyncRepo) CreateSyncConfig(_ context.Context, config *ldaps.LdapSyncConfig) error {
	r.configs[config.ID] = config
	return nil
}

func (r *fakeSyncRepo) UpdateSyncConfig(_ context.Context, config *ldaps.LdapSyncConfig) error {
	r.configs[config.ID] = config
	return nil
}

func (r *fakeSyncRepo) DeleteSyncConfig(_ context.Context, id, organizationID string) error {
	if c, ok := r.configs[id]; ok && c.OrganizationID == organizationID {
		delete(r.configs, id)
		return nil
	}
	return apperrors.NewNotFound("sync config not found")
}

func (r *fakeSyncRepo) CreateSyncTask(_ context.Context, task *ldaps.LdapSyncTask) error {
	r.tasks = append(r.tasks, task)
	return nil
}

func (r *fakeSyncRepo) UpdateSyncTask(_ context.Context, task *ldaps.LdapSyncTask) error {
	for i, t := range r.tasks {
		if t.ID == task.ID {
			r.tasks[i] = task
			return nil
		}
	}
	return errNotFound("task not found")
}

func (r *fakeSyncRepo) ListSyncTasks(_ context.Context, ldapID string, limit int) ([]*ldaps.LdapSyncTask, error) {
	return r.listTasks(ldapID, "", limit), nil
}

func (r *fakeSyncRepo) ListSyncTasksForOrganization(_ context.Context, ldapID, organizationID string, limit int) ([]*ldaps.LdapSyncTask, error) {
	return r.listTasks(ldapID, organizationID, limit), nil
}

func (r *fakeSyncRepo) listTasks(ldapID, organizationID string, limit int) []*ldaps.LdapSyncTask {
	var result []*ldaps.LdapSyncTask
	for _, t := range r.tasks {
		if t.LdapID != ldapID {
			continue
		}
		if organizationID != "" && t.OrganizationID != organizationID {
			continue
		}
		result = append(result, t)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func (r *fakeSyncRepo) GetLatestSyncTask(_ context.Context, ldapID string) (*ldaps.LdapSyncTask, error) {
	for i := len(r.tasks) - 1; i >= 0; i-- {
		if r.tasks[i].LdapID == ldapID {
			return r.tasks[i], nil
		}
	}
	return nil, errNotFound("no tasks found")
}

func (r *fakeSyncRepo) ListDueSyncConfigs(_ context.Context) ([]*ldaps.LdapSyncConfig, error) {
	var result []*ldaps.LdapSyncConfig
	for _, c := range r.configs {
		if c.Enabled {
			result = append(result, c)
		}
	}
	return result, nil
}

func (r *fakeSyncRepo) MarkSyncRunning(_ context.Context, id string) error { return nil }

func (r *fakeSyncRepo) MarkSyncComplete(_ context.Context, id, _ string) error {
	return nil
}

type notFoundError struct{ msg string }

func (e notFoundError) Error() string { return e.msg }

func errNotFound(msg string) *notFoundError { return &notFoundError{msg: msg} }

// ─── NewService ───────────────────────────────────────────────────────────

func TestNewService(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncRepo := newFakeSyncRepo()
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	userRepo := usersmock.NewMockRepository(ctrl)

	svc := NewService(syncRepo, ldapRepo, userRepo)
	assert.NotNil(t, svc)
}

// ─── CreateSyncConfig ─────────────────────────────────────────────────────

func TestService_CreateSyncConfig(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncRepo := newFakeSyncRepo()
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	userRepo := usersmock.NewMockRepository(ctrl)

	svc := NewService(syncRepo, ldapRepo, userRepo)

	// BE-B-17: creation resolves the referenced LDAP connection inside the
	// trusted tenant before persisting the config.
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "ldap-1", "org-1").
		Return(&ldaps.Ldap{ID: "ldap-1", OrganizationID: "org-1"}, nil)

	input := SyncConfigInput{
		OrganizationID:      "org-1",
		LdapID:              "ldap-1",
		SyncIntervalMinutes: 30,
		Enabled:             true,
	}

	id, err := svc.CreateSyncConfig(context.Background(), input)
	assert.NoError(t, err)
	assert.NotEmpty(t, id) // ID should be generated by the service
	assert.Contains(t, syncRepo.configs, id)
}

// ─── GetSyncConfig ────────────────────────────────────────────────────────

func TestService_GetSyncConfig(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncRepo := newFakeSyncRepo()
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	userRepo := usersmock.NewMockRepository(ctrl)

	syncRepo.configs["config-1"] = &ldaps.LdapSyncConfig{
		ID:             "config-1",
		OrganizationID: "org-1",
		LdapID:         "ldap-1",
	}

	svc := NewService(syncRepo, ldapRepo, userRepo)

	config, err := svc.GetSyncConfig(context.Background(), "org-1", "config-1")
	assert.NoError(t, err)
	assert.Equal(t, "config-1", config.ID)
	assert.Equal(t, "org-1", config.OrganizationID)
}

func TestService_GetSyncConfig_NotFound(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncRepo := newFakeSyncRepo()
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	userRepo := usersmock.NewMockRepository(ctrl)

	svc := NewService(syncRepo, ldapRepo, userRepo)

	_, err := svc.GetSyncConfig(context.Background(), "org-1", "nonexistent")
	assert.Error(t, err)
}

// ─── ListSyncConfigs ──────────────────────────────────────────────────────

func TestService_ListSyncConfigs(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncRepo := newFakeSyncRepo()
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	userRepo := usersmock.NewMockRepository(ctrl)

	syncRepo.configs["c1"] = &ldaps.LdapSyncConfig{ID: "c1", OrganizationID: "org-1", LdapID: "ldap-1"}
	syncRepo.configs["c2"] = &ldaps.LdapSyncConfig{ID: "c2", OrganizationID: "org-1", LdapID: "ldap-2"}

	svc := NewService(syncRepo, ldapRepo, userRepo)

	configs, err := svc.ListSyncConfigs(context.Background(), "org-1")
	assert.NoError(t, err)
	assert.Len(t, configs, 2)
}

func TestService_ListSyncConfigs_Empty(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncRepo := newFakeSyncRepo()
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	userRepo := usersmock.NewMockRepository(ctrl)

	svc := NewService(syncRepo, ldapRepo, userRepo)

	configs, err := svc.ListSyncConfigs(context.Background(), "org-1")
	assert.NoError(t, err)
	assert.Empty(t, configs)
}

// ─── UpdateSyncConfig ─────────────────────────────────────────────────────

func TestService_UpdateSyncConfig(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncRepo := newFakeSyncRepo()
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	userRepo := usersmock.NewMockRepository(ctrl)

	syncRepo.configs["c1"] = &ldaps.LdapSyncConfig{ID: "c1", OrganizationID: "org-1"}

	svc := NewService(syncRepo, ldapRepo, userRepo)

	enabled := true
	err := svc.PatchSyncConfig(context.Background(), "org-1", "c1", SyncConfigPatch{Enabled: &enabled})
	assert.NoError(t, err)
	assert.True(t, syncRepo.configs["c1"].Enabled)
}

// ─── DeleteSyncConfig ─────────────────────────────────────────────────────

func TestService_DeleteSyncConfig(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncRepo := newFakeSyncRepo()
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	userRepo := usersmock.NewMockRepository(ctrl)

	syncRepo.configs["c1"] = &ldaps.LdapSyncConfig{ID: "c1", OrganizationID: "org-1"}

	svc := NewService(syncRepo, ldapRepo, userRepo)

	err := svc.DeleteSyncConfig(context.Background(), "org-1", "c1")
	assert.NoError(t, err)
	_, ok := syncRepo.configs["c1"]
	assert.False(t, ok)
}

// ─── GetSyncLogs ──────────────────────────────────────────────────────────

func TestService_GetSyncLogs(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncRepo := newFakeSyncRepo()
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	userRepo := usersmock.NewMockRepository(ctrl)

	syncRepo.tasks = []*ldaps.LdapSyncTask{
		{ID: "t1", OrganizationID: "org-1", LdapID: "ldap-1", Status: "succeeded"},
		{ID: "t2", OrganizationID: "org-1", LdapID: "ldap-1", Status: "failed"},
	}
	syncRepo.configs["config-1"] = &ldaps.LdapSyncConfig{ID: "config-1", OrganizationID: "org-1", LdapID: "ldap-1"}

	svc := NewService(syncRepo, ldapRepo, userRepo)

	// GetSyncLogs takes a config ID; the LDAP ID is resolved from the config.
	tasks, err := svc.GetSyncLogs(context.Background(), "org-1", "config-1", 10)
	assert.NoError(t, err)
	assert.Len(t, tasks, 2)
}

// ─── StartScheduler / StopScheduler ───────────────────────────────────────

func TestService_StartStopScheduler(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	syncRepo := newFakeSyncRepo()
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	userRepo := usersmock.NewMockRepository(ctrl)

	svc := NewService(syncRepo, ldapRepo, userRepo)

	// Start scheduler
	svc.StartScheduler()
	assert.NotNil(t, svc.scheduler)

	// Starting again should be a no-op
	svc.StartScheduler()

	// Stop scheduler
	svc.StopScheduler()
	assert.Nil(t, svc.scheduler)

	// Stopping again should be a no-op
	svc.StopScheduler()
}

// ─── ldapSyncTaskFromResult (supplementary) ───────────────────────────────

func TestLdapSyncTaskFromResult_Succeeded(t *testing.T) {
	t.Parallel()
	now := time.Now()
	config := &ldaps.LdapSyncConfig{
		ID:             "config-002",
		OrganizationID: "org-002",
		LdapID:         "ldap-002",
	}
	result := &ldaps.LdapSyncResult{
		UsersSynced:  5,
		UsersCreated: 2,
		UsersUpdated: 3,
	}

	task := ldapSyncTaskFromResult("task-002", config, result, "manual", &now)

	assert.Equal(t, "succeeded", task.Status) // no failed users
	assert.Equal(t, "manual", task.TriggerType)
	assert.Equal(t, 5, task.UsersSynced)
	assert.Equal(t, 2, task.UsersCreated)
	assert.Equal(t, 3, task.UsersUpdated)
}
