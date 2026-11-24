package ldapsync

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/guliangiam/guliangiam/internal/domain/ldaps"
	ldapsmock "github.com/guliangiam/guliangiam/internal/domain/ldaps/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// ─── BE-B-17: sync config validation ──────────────────────────────────────

func TestService_CreateSyncConfig_RejectsNonPositiveInterval(t *testing.T) {
	t.Parallel()
	for _, interval := range []int{0, -5} {
		svc := NewService(newFakeSyncRepo(), nil, nil)
		_, err := svc.CreateSyncConfig(context.Background(), SyncConfigInput{
			OrganizationID:      "org-1",
			LdapID:              "ldap-1",
			SyncIntervalMinutes: interval,
		})
		// 语义校验（范围下限）按 4.14.1 返回 422。
		require.Equal(t, apperrors.CodeUnprocessableEntity, apperrors.FromError(err).Code)
	}
}

func TestService_CreateSyncConfig_RejectsEmptyLdapID(t *testing.T) {
	t.Parallel()
	repo := newFakeSyncRepo()
	svc := NewService(repo, nil, nil)
	_, err := svc.CreateSyncConfig(context.Background(), SyncConfigInput{
		OrganizationID:      "org-1",
		LdapID:              "  ",
		SyncIntervalMinutes: 30,
	})
	assertLdapSyncInvalidArgument(t, err)
	assert.Empty(t, repo.configs)
}

func TestService_CreateSyncConfig_ForeignLdapIsNotFound(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := newFakeSyncRepo()
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	ldapRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "ldap-foreign", "org-1").
		Return(nil, apperrors.NewNotFound("ldap not found"))

	svc := NewService(repo, ldapRepo, nil)
	_, err := svc.CreateSyncConfig(context.Background(), SyncConfigInput{
		OrganizationID:      "org-1",
		LdapID:              "ldap-foreign",
		SyncIntervalMinutes: 30,
	})
	assertLdapSyncNotFound(t, err)
	assert.Empty(t, repo.configs, "a foreign ldapId must not create a sync config")
}

func TestService_PatchSyncConfig_RejectsNonPositiveInterval(t *testing.T) {
	t.Parallel()
	repo := newFakeSyncRepo()
	repo.configs["c1"] = &ldaps.LdapSyncConfig{ID: "c1", OrganizationID: "org-1", SyncIntervalMinutes: 30}
	svc := NewService(repo, nil, nil)

	zero := 0
	err := svc.PatchSyncConfig(context.Background(), "org-1", "c1", SyncConfigPatch{SyncIntervalMinutes: &zero})
	// 语义校验（范围下限）按 4.14.1 返回 422。
	require.Equal(t, apperrors.CodeUnprocessableEntity, apperrors.FromError(err).Code)
	assert.Equal(t, 30, repo.configs["c1"].SyncIntervalMinutes)
}

// ─── BE-B-18: TriggerSync runs detached from the HTTP request ─────────────

// lockedSyncRepo guards the task slice shared between the async sync goroutine
// and the test observer.
type lockedSyncRepo struct {
	*fakeSyncRepo
	mu sync.Mutex
}

func (r *lockedSyncRepo) CreateSyncTask(ctx context.Context, task *ldaps.LdapSyncTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fakeSyncRepo.CreateSyncTask(ctx, task)
}

func (r *lockedSyncRepo) UpdateSyncTask(ctx context.Context, task *ldaps.LdapSyncTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fakeSyncRepo.UpdateSyncTask(ctx, task)
}

func (r *lockedSyncRepo) tasksSnapshot() []*ldaps.LdapSyncTask {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*ldaps.LdapSyncTask, len(r.tasks))
	copy(out, r.tasks)
	return out
}

// TestService_TriggerSync_AsyncDetachedFromRequest pins that the manual trigger
// returns while the directory sync is still running, and that cancelling the
// request context does not abort the background sync (it finishes and persists
// its outcome under the config's tenant scope).
func TestService_TriggerSync_AsyncDetachedFromRequest(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := &lockedSyncRepo{fakeSyncRepo: newFakeSyncRepo()}
	repo.configs["config-1"] = &ldaps.LdapSyncConfig{
		ID: "config-1", OrganizationID: "org-1", LdapID: "ldap-1", Enabled: true,
	}

	release := make(chan struct{})
	ldapRepo := ldapsmock.NewMockRepository(ctrl)
	ldapRepo.EXPECT().GetByID(gomock.Any(), "ldap-1").DoAndReturn(
		func(ctx context.Context, id string) (*ldaps.Ldap, error) {
			// The background sync must carry the config's trusted tenant,
			// not the (cancelled) request scope.
			scope := tenantctx.TenantScopeFromContext(ctx)
			assert.Equal(t, tenantctx.TenantFilterStrict, scope.Mode)
			assert.Equal(t, "org-1", scope.OrganizationID)
			<-release
			return nil, assert.AnError
		})

	svc := NewService(repo, ldapRepo, nil)

	reqCtx, cancelReq := context.WithCancel(context.Background())
	defer cancelReq()

	type outcome struct {
		view *SyncResultView
		err  error
	}
	done := make(chan outcome, 1)
	go func() {
		view, err := svc.TriggerSync(reqCtx, "org-1", "config-1")
		done <- outcome{view, err}
	}()

	select {
	case out := <-done:
		require.NoError(t, out.err)
		require.NotNil(t, out.view)
	case <-time.After(2 * time.Second):
		t.Fatal("TriggerSync blocked on the directory sync; it must return after queueing the task")
	}

	tasks := repo.tasksSnapshot()
	require.Len(t, tasks, 1)
	assert.Equal(t, "running", tasks[0].Status)

	// Simulate the HTTP client disconnecting, then let the sync finish.
	cancelReq()
	close(release)

	deadline := time.Now().Add(5 * time.Second)
	for {
		tasks = repo.tasksSnapshot()
		if len(tasks) == 1 && tasks[0].Status == "failed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("background sync did not finalize after request cancellation; tasks=%+v", tasks)
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.NotEmpty(t, tasks[0].ErrorMessage)
}

// ─── BE-B-4: scheduler tick must run under System scope ───────────────────

// scopeCapturingSyncRepo records the tenant scope seen by ListDueSyncConfigs.
type scopeCapturingSyncRepo struct {
	*fakeSyncRepo
	mu   sync.Mutex
	mode tenantctx.TenantFilterMode
	seen bool
}

func (r *scopeCapturingSyncRepo) ListDueSyncConfigs(ctx context.Context) ([]*ldaps.LdapSyncConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mode = tenantctx.TenantScopeFromContext(ctx).Mode
	r.seen = true
	return nil, nil
}

// TestScheduler_Tick_ListsDueConfigsUnderSystemScope pins the RLS fix: the
// FORCE-RLS ldap_sync_configs table is only readable by the background
// scheduler under System scope; a bare background context fails closed and
// silently returns zero rows.
func TestScheduler_Tick_ListsDueConfigsUnderSystemScope(t *testing.T) {
	t.Parallel()
	repo := &scopeCapturingSyncRepo{fakeSyncRepo: newFakeSyncRepo()}
	svc := NewService(repo, nil, nil)
	sched := NewScheduler(svc, time.Hour, nil)

	sched.tick()

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.True(t, repo.seen, "ListDueSyncConfigs was not called")
	assert.Equal(t, tenantctx.TenantFilterSystem, repo.mode)
}
