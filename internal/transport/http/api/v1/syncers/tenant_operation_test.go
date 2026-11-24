package syncers

import (
	"context"
	"net/http"
	"testing"

	appsyncers "github.com/guliangiam/guliangiam/internal/app/syncers"
	domain "github.com/guliangiam/guliangiam/internal/domain/syncers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// idorSyncerEngine counts how often the operational routes reach the sync
// engine. A rejected cross-tenant request must never touch it.
type idorSyncerEngine struct {
	testConnectionCalls int
	syncUsersCalls      int
	syncGroupsCalls     int
}

func (e *idorSyncerEngine) SyncUsers(context.Context, *domain.SyncEngineConfig) (*domain.SyncEngineResult, error) {
	e.syncUsersCalls++
	return &domain.SyncEngineResult{TotalUsers: 1, AddedUsers: 1}, nil
}

func (e *idorSyncerEngine) SyncGroups(context.Context, *domain.SyncEngineConfig) (*domain.SyncEngineResult, error) {
	e.syncGroupsCalls++
	return &domain.SyncEngineResult{TotalGroups: 1, AddedGroups: 1}, nil
}

func (e *idorSyncerEngine) TestConnection(context.Context, *domain.SyncEngineConfig) error {
	e.testConnectionCalls++
	return nil
}

type idorSyncerFactory struct{ engine domain.SyncEngine }

func (f idorSyncerFactory) GetEngine(string) (domain.SyncEngine, error) { return f.engine, nil }

func newSyncerOperationService(repo domain.Repository) (appsyncers.Service, *idorSyncerEngine, *idorSyncerRecords) {
	engine := &idorSyncerEngine{}
	records := &idorSyncerRecords{}
	service := appsyncers.NewServiceWithFactory(
		idorSyncerClock{}, idorSyncerUUID{},
		&idorSyncerUOW{repo: repo, records: records},
		idorSyncerFactory{engine: engine},
	)
	return service, engine, records
}

func syncerOperationPaths() []struct {
	name   string
	suffix string
} {
	return []struct {
		name   string
		suffix string
	}{
		{"run", "/run"},
		{"test-connection", "/test-connection"},
	}
}

// TestSyncerOperation_WithoutTenantScopeRejected pins the transport fail-fast:
// run/test-connection resolve the trusted tenant before the app layer, so a
// request without an authenticated tenant is 400 and never reaches the engine.
func TestSyncerOperation_WithoutTenantScopeRejected(t *testing.T) {
	for _, op := range syncerOperationPaths() {
		t.Run(op.name, func(t *testing.T) {
			repo := &idorSyncerRepo{syncers: []*domain.Syncer{idorSyncer(idorSyncerOwnID, idorSyncerTenantOrgID)}}
			service, engine, records := newSyncerOperationService(repo)

			target := "/api/v1/syncers/" + idorSyncerOwnID + op.suffix
			recorder := serveSyncerIDOR(t, idorSyncerRouter(service), idorSyncerBareRequest(http.MethodPost, target, nil))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, engine.testConnectionCalls)
			assert.Zero(t, engine.syncUsersCalls)
			assert.Zero(t, engine.syncGroupsCalls)
			assert.Empty(t, repo.lastScopedID, "an empty tenant must not reach the repository")
			assert.Empty(t, records.created)
		})
	}
}

// TestSyncerOperation_ForeignTenantRejected pins that a foreign syncer id is
// indistinguishable from a missing one: 404, no engine call, no update, no
// audit record, and the repository receives the authenticated tenant rather
// than the forged organizationId query value.
func TestSyncerOperation_ForeignTenantRejected(t *testing.T) {
	for _, op := range syncerOperationPaths() {
		t.Run(op.name, func(t *testing.T) {
			repo := &idorSyncerRepo{syncers: []*domain.Syncer{idorSyncer(idorSyncerForeignID, idorSyncerForeignOrgID)}}
			service, engine, records := newSyncerOperationService(repo)

			target := "/api/v1/syncers/" + idorSyncerForeignID + op.suffix + "?organizationId=" + idorSyncerForeignOrgID
			recorder := serveSyncerIDOR(t, idorSyncerRouter(service), idorSyncerRequest(http.MethodPost, target, nil))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign syncer must be indistinguishable from missing")
			assert.Zero(t, engine.testConnectionCalls, "foreign tenant must not open a connection")
			assert.Zero(t, engine.syncUsersCalls, "foreign tenant must not run a sync")
			assert.Zero(t, engine.syncGroupsCalls)
			assert.Zero(t, repo.updateCnt, "foreign tenant must not update the syncer")
			assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
			assert.Equal(t, idorSyncerTenantOrgID, repo.lastScopedOrgID, "repository must receive the trusted session tenant")
			require.Len(t, repo.syncers, 1, "foreign syncer row must remain untouched")
		})
	}
}

// TestSyncerOperation_OwnTenantSucceeds pins that operational routes still
// work for the authenticated tenant after the binding, and that a run audits
// the syncer's real organization.
func TestSyncerOperation_OwnTenantSucceeds(t *testing.T) {
	t.Run("run", func(t *testing.T) {
		repo := &idorSyncerRepo{syncers: []*domain.Syncer{idorSyncer(idorSyncerOwnID, idorSyncerTenantOrgID)}}
		service, engine, records := newSyncerOperationService(repo)

		recorder := serveSyncerIDOR(t, idorSyncerRouter(service),
			idorSyncerRequest(http.MethodPost, "/api/v1/syncers/"+idorSyncerOwnID+"/run", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, engine.syncUsersCalls)
		assert.Equal(t, 1, engine.syncGroupsCalls)
		assert.Equal(t, 1, repo.updateCnt, "own tenant run must persist lastSyncAt")
		require.Len(t, records.created, 1)
		assert.Equal(t, idorSyncerTenantOrgID, records.created[0].OrganizationID)
	})

	t.Run("test-connection", func(t *testing.T) {
		repo := &idorSyncerRepo{syncers: []*domain.Syncer{idorSyncer(idorSyncerOwnID, idorSyncerTenantOrgID)}}
		service, engine, _ := newSyncerOperationService(repo)

		recorder := serveSyncerIDOR(t, idorSyncerRouter(service),
			idorSyncerRequest(http.MethodPost, "/api/v1/syncers/"+idorSyncerOwnID+"/test-connection", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, engine.testConnectionCalls)
	})
}
