package rebac

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	guuid "github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rebacapp "github.com/guliangiam/guliangiam/internal/app/rebac"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domain "github.com/guliangiam/guliangiam/internal/domain/rebac"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

const (
	createNamespaceOrgID = "11111111-1111-1111-1111-111111111111"
	createNamespaceID    = "01890000-0000-7000-8000-000000000001"
)

type createNamespaceClock struct{ now time.Time }

func (c createNamespaceClock) Now() time.Time { return c.now }

type createNamespaceUUID struct{ id string }

func (g createNamespaceUUID) NewString() (string, error) { return g.id, nil }

// createNamespaceRepo is an in-memory Namespace repository. It records
// whether the write happened inside the UoW transaction so the test can pin
// the transaction boundary restored by the app use case.
type createNamespaceRepo struct {
	domain.Repository
	now               time.Time
	inTx              bool
	created           []*domain.Namespace
	createdInT        []bool
	tuples            []*domain.Tuple
	tuplesInTx        []bool
	deletedTuples     []string
	deletedNamespaces []string
	failTupleAt       int
	tupleCalls        int
}

func (r *createNamespaceRepo) CreateNamespace(_ context.Context, ns *domain.Namespace) error {
	ns.CreatedAt, ns.UpdatedAt = r.now, r.now
	ns.Revision = 1
	r.created = append(r.created, ns)
	r.createdInT = append(r.createdInT, r.inTx)
	return nil
}

func (r *createNamespaceRepo) CreateTuple(_ context.Context, tuple *domain.Tuple) error {
	r.tupleCalls++
	if r.failTupleAt > 0 && r.tupleCalls == r.failTupleAt {
		return context.DeadlineExceeded
	}
	tuple.CreatedAt = r.now
	tuple.Zookie = "1"
	r.tuples = append(r.tuples, tuple)
	r.tuplesInTx = append(r.tuplesInTx, r.inTx)
	return nil
}

// CreateTuplesBatch 与 CreateTuple 共用计数/失败注入，保持事务边界断言语义。
func (r *createNamespaceRepo) CreateTuplesBatch(_ context.Context, tuples []*domain.Tuple) error {
	for _, tuple := range tuples {
		r.tupleCalls++
		if r.failTupleAt > 0 && r.tupleCalls == r.failTupleAt {
			return context.DeadlineExceeded
		}
		tuple.CreatedAt = r.now
		tuple.Zookie = "1"
		r.tuples = append(r.tuples, tuple)
		r.tuplesInTx = append(r.tuplesInTx, r.inTx)
	}
	return nil
}

func (r *createNamespaceRepo) DeleteTuple(_ context.Context, id, _ string) error {
	r.deletedTuples = append(r.deletedTuples, id)
	return nil
}

func (r *createNamespaceRepo) DeleteNamespace(_ context.Context, id, _ string) error {
	r.deletedNamespaces = append(r.deletedNamespaces, id)
	return nil
}

func (r *createNamespaceRepo) DeleteTuplesByNamespace(context.Context, string, string) (int64, error) {
	return 0, nil
}

type createNamespaceRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *createNamespaceRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type createNamespaceOutbox struct {
	domainoutbox.Repository
	events []*domainoutbox.OutboxEvent
}

func (o *createNamespaceOutbox) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	o.events = append(o.events, event)
	return nil
}

type createNamespaceUOW struct {
	appuow.UnitOfWork
	repo    *createNamespaceRepo
	records *createNamespaceRecords
	outbox  *createNamespaceOutbox
}

func (u *createNamespaceUOW) Rebac() domain.Repository { return u.repo }
func (u *createNamespaceUOW) Records() recordsdomain.Repository {
	return u.records
}
func (u *createNamespaceUOW) Outbox() domainoutbox.Repository { return u.outbox }
func (u *createNamespaceUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	namespaceCount, tupleCount := len(u.repo.created), len(u.repo.tuples)
	tupleTxCount := len(u.repo.tuplesInTx)
	recordCount, eventCount := len(u.records.created), len(u.outbox.events)
	u.repo.inTx = true
	defer func() { u.repo.inTx = false }()
	err := fn(ctx, u)
	if err != nil {
		u.repo.created = u.repo.created[:namespaceCount]
		u.repo.tuples = u.repo.tuples[:tupleCount]
		u.repo.tuplesInTx = u.repo.tuplesInTx[:tupleTxCount]
		u.records.created = u.records.created[:recordCount]
		u.outbox.events = u.outbox.events[:eventCount]
	}
	return err
}

type createNamespaceHarness struct {
	repo    *createNamespaceRepo
	records *createNamespaceRecords
	outbox  *createNamespaceOutbox
	router  http.Handler
}

func newCreateNamespaceHarness() *createNamespaceHarness {
	return newRebacHarness(createNamespaceUUID{id: createNamespaceID})
}

func newRebacHarness(uid interface{ NewString() (string, error) }) *createNamespaceHarness {
	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	repo := &createNamespaceRepo{now: now}
	records := &createNamespaceRecords{}
	outbox := &createNamespaceOutbox{}
	uow := &createNamespaceUOW{repo: repo, records: records, outbox: outbox}
	service := rebacapp.NewService(createNamespaceClock{now: now}, uid, uow)

	router := testRouter()
	RegisterChiRoutes(router, NewHandler(repo, nil, &service))
	return &createNamespaceHarness{repo: repo, records: records, outbox: outbox, router: router}
}

type createTupleSequenceUUID struct{ next int }

func (g *createTupleSequenceUUID) NewString() (string, error) {
	g.next++
	return fmt.Sprintf("tuple-id-%d", g.next), nil
}

func createNamespaceRequest(orgID, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rebac/namespaces", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if orgID != "" {
		req = req.WithContext(tenantctx.WithTenantScope(req.Context(), orgID, "session-org", tenantctx.TenantFilterStrict))
	}
	return req
}

func createTupleRequest(orgID, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rebac/namespaces/namespace-id/tuples", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if orgID != "" {
		req = req.WithContext(tenantctx.WithTenantScope(req.Context(), orgID, "session-org", tenantctx.TenantFilterStrict))
	}
	return req
}

func TestCreateNamespace_UsesAppGeneratedIDAndLocation(t *testing.T) {
	h := newCreateNamespaceHarness()
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, createNamespaceRequest(createNamespaceOrgID,
		`{"name":"folder","displayName":"Folder","relationDefs":{"roles":["admin","viewer"]}}`))

	require.Equal(t, http.StatusCreated, rec.Code)
	location := rec.Header().Get("Location")
	require.NotEmpty(t, location, "create must always return Location")
	assert.Equal(t, "/api/v1/rebac/namespaces/"+createNamespaceID, location)

	var envelope struct {
		Namespace map[string]json.RawMessage `json:"namespace"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))

	wantFields := []string{
		"id", "organizationId", "name", "displayName",
		"relationDefs", "version", "createdAt", "updatedAt",
	}
	for _, field := range wantFields {
		assert.Contains(t, envelope.Namespace, field, "response contract field %q missing", field)
	}
	assert.NotContains(t, envelope.Namespace, "revision", "app-only field must not leak into the wire contract")

	var id string
	require.NoError(t, json.Unmarshal(envelope.Namespace["id"], &id))
	_, err := guuid.Parse(id)
	require.NoError(t, err, "created namespace id must be a valid UUID")
	assert.Equal(t, createNamespaceID, id)
	assert.Equal(t, "/api/v1/rebac/namespaces/"+id, location)

	var organizationID string
	require.NoError(t, json.Unmarshal(envelope.Namespace["organizationId"], &organizationID))
	assert.Equal(t, createNamespaceOrgID, organizationID, "tenant must come from the trusted scope")

	// The write went through the app use case: UUID generated there, repository
	// write inside the transaction, audit + outbox written in the same tx.
	require.Len(t, h.repo.created, 1)
	assert.True(t, h.repo.createdInT[0], "namespace insert must run inside WithinTransaction")
	assert.Equal(t, createNamespaceID, h.repo.created[0].ID)

	require.Len(t, h.records.created, 1)
	record := h.records.created[0]
	assert.Equal(t, "rebac.namespace.created", record.Action)
	assert.Equal(t, "namespace", record.ResourceType)
	assert.Equal(t, createNamespaceID, record.ResourceID)
	assert.Equal(t, createNamespaceOrgID, record.OrganizationID)

	require.Len(t, h.outbox.events, 1)
	event := h.outbox.events[0]
	assert.Equal(t, "rebac.namespace.created", event.EventType)
	assert.Equal(t, "namespace", event.AggregateType)
	assert.Equal(t, createNamespaceID, event.AggregateID)
	require.NotNil(t, event.OrganizationID)
	assert.Equal(t, createNamespaceOrgID, *event.OrganizationID)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(event.Payload, &payload))
	assert.Equal(t, createNamespaceID, payload["namespaceId"])
	assert.Equal(t, createNamespaceOrgID, payload["orgId"])
	assert.Equal(t, "folder", payload["name"])
}

func TestCreateNamespace_InvalidPayloadKeepsErrorContract(t *testing.T) {
	cases := []struct {
		name string
		org  string
		body string
	}{
		{name: "malformed json", org: createNamespaceOrgID, body: `{"name":`},
		{name: "unknown field", org: createNamespaceOrgID, body: `{"name":"folder","organizationId":"` + createNamespaceOrgID + `"}`},
		{name: "missing name", org: createNamespaceOrgID, body: `{"displayName":"Folder"}`},
		{name: "missing trusted tenant", body: `{"name":"folder"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newCreateNamespaceHarness()
			rec := httptest.NewRecorder()
			h.router.ServeHTTP(rec, createNamespaceRequest(tc.org, tc.body))

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, "invalid_argument", body.Error.Code)
			assert.Empty(t, h.repo.created, "invalid payload must not reach persistence")
			assert.Empty(t, h.records.created, "invalid payload must not produce an audit record")
			assert.Empty(t, h.outbox.events, "invalid payload must not produce an outbox event")
			assert.Empty(t, rec.Header().Get("Location"))
		})
	}
}

func TestCreateTuple_UsesAppTransactionAuditAndOutbox(t *testing.T) {
	h := newRebacHarness(createNamespaceUUID{id: "tuple-id-1"})
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, createTupleRequest(createNamespaceOrgID,
		`{"subject":"user:jane","relation":"viewer","object":"doc:project_alpha"}`))

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Len(t, h.repo.tuples, 1)
	assert.True(t, h.repo.tuplesInTx[0], "tuple insert must run inside the UoW transaction")
	require.Len(t, h.records.created, 1)
	assert.Equal(t, "rebac.tuple.created", h.records.created[0].Action)
	assert.Equal(t, "tuple-id-1", h.records.created[0].ResourceID)
	require.Len(t, h.outbox.events, 1)
	assert.Equal(t, "rebac.tuple.created", h.outbox.events[0].EventType)
}

func TestWriteTuples_IsAtomicAndAudited(t *testing.T) {
	h := newRebacHarness(&createTupleSequenceUUID{})
	req := WriteTuplesRequest{
		OrganizationID: createNamespaceOrgID,
		NamespaceID:    "namespace-id",
		Tuples: []TupleKeyJSON{
			{Subject: "user:jane", Relation: "viewer", Object: "doc:one"},
			{Subject: "user:jane", Relation: "viewer", Object: "doc:two"},
		},
	}

	resp, err := NewHandler(h.repo, nil, serviceForHarness(h, &createTupleSequenceUUID{})).WriteTuples(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, []string{"tuple-id-1", "tuple-id-2"}, resp.IDs)
	require.Len(t, h.repo.tuples, 2)
	assert.Equal(t, []bool{true, true}, h.repo.tuplesInTx)
	require.Len(t, h.records.created, 2)
	require.Len(t, h.outbox.events, 2)

	failed := newRebacHarness(&createTupleSequenceUUID{})
	failed.repo.failTupleAt = 2
	_, err = NewHandler(failed.repo, nil, serviceForHarness(failed, &createTupleSequenceUUID{})).WriteTuples(context.Background(), req)
	require.Error(t, err)
	assert.Empty(t, failed.repo.tuples, "failed batch must roll back earlier tuple writes")
	assert.Empty(t, failed.records.created, "failed batch must roll back embedded audit rows")
	assert.Empty(t, failed.outbox.events, "failed batch must roll back outbox events")
}

func serviceForHarness(h *createNamespaceHarness, uid interface{ NewString() (string, error) }) *rebacapp.Service {
	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	uow := &createNamespaceUOW{repo: h.repo, records: h.records, outbox: h.outbox}
	service := rebacapp.NewService(createNamespaceClock{now: now}, uid, uow)
	return &service
}

func TestDeleteTuple_UsesAppTransactionAuditAndOutbox(t *testing.T) {
	h := newRebacHarness(createNamespaceUUID{id: createNamespaceID})
	handler := NewHandler(h.repo, nil, serviceForHarness(h, createNamespaceUUID{id: createNamespaceID}))

	err := handler.DeleteTuple(context.Background(), "tuple-id", createNamespaceOrgID)
	require.NoError(t, err)
	assert.Equal(t, []string{"tuple-id"}, h.repo.deletedTuples)
	require.Len(t, h.records.created, 1)
	assert.Equal(t, "rebac.tuple.deleted", h.records.created[0].Action)
	require.Len(t, h.outbox.events, 1)
	assert.Equal(t, "rebac.tuple.deleted", h.outbox.events[0].EventType)
}

func TestDeleteNamespace_UsesAppTransactionAuditAndOutbox(t *testing.T) {
	h := newRebacHarness(createNamespaceUUID{id: createNamespaceID})
	handler := NewHandler(h.repo, nil, serviceForHarness(h, createNamespaceUUID{id: createNamespaceID}))

	err := handler.DeleteNamespace(context.Background(), createNamespaceID, createNamespaceOrgID)
	require.NoError(t, err)
	assert.Equal(t, []string{createNamespaceID}, h.repo.deletedNamespaces)
	require.Len(t, h.records.created, 1)
	assert.Equal(t, "rebac.namespace.deleted", h.records.created[0].Action)
	require.Len(t, h.outbox.events, 1)
	assert.Equal(t, "rebac.namespace.deleted", h.outbox.events[0].EventType)
}
