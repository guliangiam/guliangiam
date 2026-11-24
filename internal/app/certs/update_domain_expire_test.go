package certs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	certdomain "github.com/guliangiam/guliangiam/internal/domain/certs"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	recordsmock "github.com/guliangiam/guliangiam/internal/domain/records/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	domainExpireTestOrgID  = "org-1"
	domainExpireTestCertID = "cert-1"
)

// guardCertRepo 复刻 PG/MySQL 仓储的乐观锁语义：仓储以传入实体 Revision-1
// 作为旧值守卫，只有实体在读取后自增过 revision 才会命中。
type guardCertRepo struct {
	cert         *certdomain.Cert
	readRevision int64
	updates      []*certdomain.Cert
}

func (r *guardCertRepo) GetByID(ctx context.Context, id, organizationID string) (*certdomain.Cert, error) {
	if r.cert.ID != id || r.cert.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("cert not found")
	}
	r.readRevision = r.cert.Revision
	return r.cert, nil
}

func (r *guardCertRepo) List(ctx context.Context, query certdomain.ListQuery) ([]*certdomain.Cert, int64, error) {
	return []*certdomain.Cert{r.cert}, 1, nil
}

func (r *guardCertRepo) Create(ctx context.Context, cert *certdomain.Cert) error { return nil }

func (r *guardCertRepo) Update(ctx context.Context, cert *certdomain.Cert) error {
	snapshot := *cert
	r.updates = append(r.updates, &snapshot)
	if cert.Revision-1 != r.readRevision {
		return apperrors.NewConflict("cert revision conflict")
	}
	r.cert = cert
	return nil
}

func (r *guardCertRepo) Delete(ctx context.Context, id, organizationID string) error { return nil }

type captureOutbox struct {
	events []*domainoutbox.OutboxEvent
}

func (o *captureOutbox) Enqueue(ctx context.Context, item *domainoutbox.OutboxEvent) error {
	o.events = append(o.events, item)
	return nil
}

func (o *captureOutbox) EnqueueBatch(ctx context.Context, items []*domainoutbox.OutboxEvent) error {
	o.events = append(o.events, items...)
	return nil
}

// domainExpireUOW 覆盖 testUnitOfWork 的 Certs/Records/Outbox，使审计写入可断言。
type domainExpireUOW struct {
	testUnitOfWork
	repo    certdomain.Repository
	records recordsdomain.Repository
	outbox  domainoutbox.Repository
}

func (u domainExpireUOW) Certs() certdomain.Repository      { return u.repo }
func (u domainExpireUOW) Records() recordsdomain.Repository { return u.records }
func (u domainExpireUOW) Outbox() domainoutbox.Repository   { return u.outbox }

// 显式实现事务转发：内嵌 testUnitOfWork 的提升方法会把内嵌副本传给回调，
// 从而绕过上面的 Certs/Records/Outbox 覆盖。
func (u domainExpireUOW) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

type domainExpireFixture struct {
	svc     Service
	repo    *guardCertRepo
	outbox  *captureOutbox
	records *recordsdomain.Record
}

func newDomainExpireFixture(t *testing.T, now time.Time) *domainExpireFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	f := &domainExpireFixture{outbox: &captureOutbox{}}
	f.repo = &guardCertRepo{
		cert: &certdomain.Cert{
			ID:             domainExpireTestCertID,
			OrganizationID: domainExpireTestOrgID,
			Name:           "my-cert",
			Revision:       7,
			CreatedAt:      now.Add(-2 * time.Hour),
			UpdatedAt:      now.Add(-time.Hour),
		},
	}
	records := recordsmock.NewMockRepository(ctrl)
	records.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, record *recordsdomain.Record) error {
			f.records = record
			return nil
		},
	)
	f.svc = Service{
		Clock: fakeClock{fixedTime: now},
		UOW: domainExpireUOW{
			repo:    f.repo,
			records: records,
			outbox:  f.outbox,
		},
	}
	return f
}

func (f *domainExpireFixture) assertAudit(t *testing.T) {
	t.Helper()
	require.NotNil(t, f.records, "审计 Record 必须落库")
	assert.Equal(t, "cert.domain_expire_updated", f.records.Action)
	assert.Equal(t, "certificate", f.records.ResourceType)
	assert.Equal(t, domainExpireTestCertID, f.records.ResourceID)
	assert.Equal(t, domainExpireTestOrgID, f.records.OrganizationID)

	require.Len(t, f.outbox.events, 1, "审计 Outbox 事件必须落库")
	assert.Equal(t, "cert.domain_expire_updated", f.outbox.events[0].EventType)
	assert.Equal(t, "cert", f.outbox.events[0].AggregateType)
	assert.Equal(t, domainExpireTestCertID, f.outbox.events[0].AggregateID)
}

func TestUpdateDomainExpire_BumpsRevisionAndWritesAudit(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	f := newDomainExpireFixture(t, now)

	err := f.svc.UpdateDomainExpire(context.Background(), domainExpireTestOrgID, domainExpireTestCertID, "2027-01-01T00:00:00Z")
	require.NoError(t, err)

	require.Len(t, f.repo.updates, 1)
	received := f.repo.updates[0]
	assert.Equal(t, f.repo.readRevision+1, received.Revision, "仓储必须收到读取值+1 的 revision")
	assert.Equal(t, now, received.UpdatedAt, "UpdatedAt 必须刷新为当前时钟")
	require.NotNil(t, received.DomainExpireTime)
	assert.Equal(t, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), *received.DomainExpireTime)
	f.assertAudit(t)
}

func TestUpdateDomainExpire_ClearsDomainExpireTime(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	f := newDomainExpireFixture(t, now)
	existing := now.Add(24 * time.Hour)
	f.repo.cert.DomainExpireTime = &existing

	err := f.svc.UpdateDomainExpire(context.Background(), domainExpireTestOrgID, domainExpireTestCertID, "")
	require.NoError(t, err)

	require.Len(t, f.repo.updates, 1)
	assert.Nil(t, f.repo.updates[0].DomainExpireTime, "空值必须清空 domainExpireTime")
	assert.Equal(t, f.repo.readRevision+1, f.repo.updates[0].Revision)
	assert.Equal(t, now, f.repo.updates[0].UpdatedAt)
	f.assertAudit(t)
}
