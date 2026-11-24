package applications

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
)

// importFailCreateRepo makes Create fail while keeping the read methods of
// mockApplicationRepo available for the pre-transaction slug check.
type importFailCreateRepo struct {
	*mockApplicationRepo
	err error
}

func (r *importFailCreateRepo) Create(context.Context, *domain.Application) error { return r.err }

// importTxUOW models commit/rollback: audit writes made inside the callback are
// staged in per-transaction repos and only published when the callback succeeds.
type importTxUOW struct {
	appuow.UnitOfWork
	appRepo   domain.Repository
	records   *auditWrapperRecords
	outbox    *auditWrapperOutbox
	commits   int
	rollbacks int
}

func (u *importTxUOW) Applications() domain.Repository   { return u.appRepo }
func (u *importTxUOW) Records() domainrecords.Repository { return u.records }
func (u *importTxUOW) Outbox() domainoutbox.Repository   { return u.outbox }

func (u *importTxUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	tx := &importTxUOW{
		appRepo: u.appRepo,
		records: &auditWrapperRecords{},
		outbox:  &auditWrapperOutbox{},
	}
	if err := fn(ctx, tx); err != nil {
		u.rollbacks++
		return err
	}
	u.records.created = append(u.records.created, tx.records.created...)
	u.outbox.events = append(u.outbox.events, tx.outbox.events...)
	u.commits++
	return nil
}

func newImportTxUOW(repo domain.Repository) *importTxUOW {
	return &importTxUOW{
		appRepo: repo,
		records: &auditWrapperRecords{},
		outbox:  &auditWrapperOutbox{},
	}
}

func TestImportApplication_WritesAuditEventInTransaction(t *testing.T) {
	uow := newImportTxUOW(&mockApplicationRepo{})
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  &fakeUUIDGenerator{fixedID: "imported-app-id", altID: "imported-client-id"},
		UOW:   uow,
	}

	const actorID = "019operator-import-0000-7000-8000-000000000001"
	app, err := svc.ImportApplication(context.Background(), ImportPayload{
		OrganizationID: "org-1",
		ActorID:        actorID,
		Version:        ExportVersion,
		Export: ApplicationExportDTO{
			Slug:        "imported-app",
			DisplayName: "Imported Application",
			GrantTypes:  []string{grantTypeAuthorizationCode},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 1, uow.commits)
	require.Len(t, uow.records.created, 1)

	record := uow.records.created[0]
	assert.Equal(t, "application.created", record.Action)
	assert.Equal(t, "Application", record.ResourceType)
	assert.Equal(t, app.ID, record.ResourceID)
	assert.Equal(t, "org-1", record.OrganizationID)
	assert.Equal(t, actorID, record.ActorID)
	assert.NotEmpty(t, record.ActorID)

	require.Len(t, uow.outbox.events, 1)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(uow.outbox.events[0].Payload, &payload))
	assert.Equal(t, "imported-app", payload["slug"])
	assert.Equal(t, "Imported Application", payload["displayName"])
	assert.Equal(t, "import", payload["source"])
	assert.Equal(t, ExportVersion, payload["exportVersion"])
}

func TestImportApplication_RollsBackCreateAndAuditOnFailure(t *testing.T) {
	repo := &importFailCreateRepo{
		mockApplicationRepo: &mockApplicationRepo{},
		err:                 errors.New("insert failed"),
	}
	uow := newImportTxUOW(repo)
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  &fakeUUIDGenerator{fixedID: "imported-app-id", altID: "imported-client-id"},
		UOW:   uow,
	}

	_, err := svc.ImportApplication(context.Background(), ImportPayload{
		OrganizationID: "org-1",
		ActorID:        "019operator-import-0000-7000-8000-000000000002",
		Version:        ExportVersion,
		Export: ApplicationExportDTO{
			Slug:        "imported-app",
			DisplayName: "Imported Application",
			GrantTypes:  []string{grantTypeAuthorizationCode},
		},
	})
	require.Error(t, err)
	assert.Equal(t, 1, uow.rollbacks)
	assert.Equal(t, 0, uow.commits)
	assert.Empty(t, uow.records.created, "audit record must roll back with the failed create")
	assert.Empty(t, uow.outbox.events, "audit event must roll back with the failed create")
	assert.Empty(t, repo.apps, "application must not be persisted")
}
