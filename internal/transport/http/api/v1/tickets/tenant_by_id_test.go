package tickets

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	apptickets "github.com/guliangiam/guliangiam/internal/app/tickets"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	domain "github.com/guliangiam/guliangiam/internal/domain/tickets"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tenantTicketRepo struct {
	tickets   []*domain.Ticket
	getCalls  int
	updateCnt int
	deleteCnt int
}

func (r *tenantTicketRepo) GetByID(_ context.Context, id, organizationID string) (*domain.Ticket, error) {
	r.getCalls++
	for _, t := range r.tickets {
		if t.ID == id && t.OrganizationID == organizationID {
			return t, nil
		}
	}
	return nil, apperrors.NewNotFound("ticket not found")
}

func (r *tenantTicketRepo) List(_ context.Context, _ domain.ListQuery) ([]*domain.Ticket, int64, error) {
	return r.tickets, int64(len(r.tickets)), nil
}

func (r *tenantTicketRepo) Create(_ context.Context, t *domain.Ticket) error {
	r.tickets = append(r.tickets, t)
	return nil
}

func (r *tenantTicketRepo) Update(_ context.Context, t *domain.Ticket) error {
	r.updateCnt++
	for i, item := range r.tickets {
		if item.ID == t.ID {
			r.tickets[i] = t
			return nil
		}
	}
	return apperrors.NewNotFound("ticket not found")
}

func (r *tenantTicketRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCnt++
	for i, t := range r.tickets {
		if t.ID == id && t.OrganizationID == organizationID {
			r.tickets = append(r.tickets[:i], r.tickets[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("ticket not found")
}

type tenantTicketRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *tenantTicketRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type tenantTicketUOW struct {
	appuow.UnitOfWork
	repo    domain.Repository
	records *tenantTicketRecords
}

func (u *tenantTicketUOW) Tickets() domain.Repository        { return u.repo }
func (u *tenantTicketUOW) Records() recordsdomain.Repository { return u.records }
func (u *tenantTicketUOW) Outbox() domainoutbox.Repository   { return nil }
func (u *tenantTicketUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func tenantTicketRouter(repo *tenantTicketRepo, records *tenantTicketRecords) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: apptickets.NewService(forgedOrgClock{}, forgedOrgUUID{}, &tenantTicketUOW{repo: repo, records: records})})
	return router
}

func tenantTicketRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func serveTenantTicketRequest(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

type readTrackingBody struct {
	reader io.Reader
	read   bool
}

func (b *readTrackingBody) Read(p []byte) (int, error) {
	b.read = true
	return b.reader.Read(p)
}

func foreignTenantTicket() *domain.Ticket {
	return &domain.Ticket{
		ID: "foreign-ticket", OrganizationID: forgedOrgRequestID,
		Name: "foreign", DisplayName: "Foreign", Status: domain.StatusOpen, Revision: 1,
		Description: "---\nAuthor: reporter\nTime: 2026-01-01T00:00:00Z\nsecret foreign message",
	}
}

func ownTenantTicket() *domain.Ticket {
	return &domain.Ticket{
		ID: "own-ticket", OrganizationID: forgedOrgSessionID,
		Name: "own", DisplayName: "Own", Status: domain.StatusOpen, Revision: 1,
	}
}

// TestTicketByID_ForeignTenantRejected pins every ticket by-ID face (detail,
// patch, delete and the messages child read) to the authenticated tenant: the
// foreign ticket is indistinguishable from a missing one and no write/audit
// happens.
func TestTicketByID_ForeignTenantRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/tickets/foreign-ticket?organizationId=" + forgedOrgRequestID, ""},
		{"patch", http.MethodPatch, "/api/v1/tickets/foreign-ticket?organizationId=" + forgedOrgRequestID, `{"displayName":"hijacked","revision":1}`},
		{"delete", http.MethodDelete, "/api/v1/tickets/foreign-ticket?organizationId=" + forgedOrgRequestID, ""},
		{"messages", http.MethodGet, "/api/v1/tickets/foreign-ticket/messages?organizationId=" + forgedOrgRequestID, ""},
		{"add-message", http.MethodPost, "/api/v1/tickets/foreign-ticket/messages?organizationId=" + forgedOrgRequestID, `{"author":"attacker","content":"injected"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &tenantTicketRepo{tickets: []*domain.Ticket{foreignTenantTicket()}}
			records := &tenantTicketRecords{}
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveTenantTicketRequest(t, tenantTicketRouter(repo, records), forgedOrgRequest(tt.method, tt.path, body))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign ticket must be indistinguishable from missing")
			assert.Zero(t, repo.updateCnt, "foreign tenant must not update the ticket")
			assert.Zero(t, repo.deleteCnt, "foreign tenant must not delete the ticket")
			assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
			require.Len(t, repo.tickets, 1, "foreign ticket row must remain untouched")
			assert.NotContains(t, recorder.Body.String(), "secret foreign message")
		})
	}
}

// TestTicketByID_WithoutTenantScopeRejected pins the fail-fast before the
// repository is touched.
func TestTicketByID_WithoutTenantScopeRejected(t *testing.T) {
	repo := &tenantTicketRepo{tickets: []*domain.Ticket{ownTenantTicket()}}
	records := &tenantTicketRecords{}
	router := tenantTicketRouter(repo, records)

	requests := map[string]*http.Request{
		"get":         tenantTicketRequest(http.MethodGet, "/api/v1/tickets/own-ticket", nil),
		"patch":       tenantTicketRequest(http.MethodPatch, "/api/v1/tickets/own-ticket", strings.NewReader(`{"displayName":"renamed"}`)),
		"delete":      tenantTicketRequest(http.MethodDelete, "/api/v1/tickets/own-ticket", nil),
		"messages":    tenantTicketRequest(http.MethodGet, "/api/v1/tickets/own-ticket/messages", nil),
		"add-message": tenantTicketRequest(http.MethodPost, "/api/v1/tickets/own-ticket/messages", strings.NewReader(`{"author":"agent","content":"reply"}`)),
	}
	for name, req := range requests {
		t.Run(name, func(t *testing.T) {
			recorder := serveTenantTicketRequest(t, router, req)
			assert.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
	assert.Zero(t, repo.updateCnt)
	assert.Zero(t, repo.deleteCnt)
	assert.Empty(t, records.created)
}

// TestTicketByID_WithoutTenantScopeDoesNotReadBody pins that the tenant
// fail-fast runs before JSON decoding: out-of-scope write bodies are never
// consumed.
func TestTicketByID_WithoutTenantScopeDoesNotReadBody(t *testing.T) {
	repo := &tenantTicketRepo{tickets: []*domain.Ticket{ownTenantTicket()}}
	records := &tenantTicketRecords{}
	router := tenantTicketRouter(repo, records)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"patch", http.MethodPatch, "/api/v1/tickets/own-ticket", `{"displayName":"renamed"}`},
		{"add-message", http.MethodPost, "/api/v1/tickets/own-ticket/messages", `{"author":"agent","content":"reply"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := &readTrackingBody{reader: strings.NewReader(tt.body)}
			recorder := serveTenantTicketRequest(t, router, tenantTicketRequest(tt.method, tt.path, body))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.False(t, body.read, "tenant fail-fast must precede body decoding")
		})
	}
	assert.Zero(t, repo.updateCnt)
	assert.Zero(t, repo.deleteCnt)
	assert.Empty(t, records.created)
}

// TestTicketByID_OwnTenantSucceeds pins that the same operations still work for
// the authenticated tenant and that write audits carry the ticket organization.
func TestTicketByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &tenantTicketRepo{tickets: []*domain.Ticket{ownTenantTicket()}}
		recorder := serveTenantTicketRequest(t, tenantTicketRouter(repo, &tenantTicketRecords{}),
			forgedOrgRequest(http.MethodGet, "/api/v1/tickets/own-ticket", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"organizationId":"`+forgedOrgSessionID+`"`)
	})

	t.Run("patch", func(t *testing.T) {
		repo := &tenantTicketRepo{tickets: []*domain.Ticket{ownTenantTicket()}}
		records := &tenantTicketRecords{}
		recorder := serveTenantTicketRequest(t, tenantTicketRouter(repo, records),
			forgedOrgRequest(http.MethodPatch, "/api/v1/tickets/own-ticket",
				strings.NewReader(`{"displayName":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		repo := &tenantTicketRepo{tickets: []*domain.Ticket{ownTenantTicket()}}
		records := &tenantTicketRecords{}
		recorder := serveTenantTicketRequest(t, tenantTicketRouter(repo, records),
			forgedOrgRequest(http.MethodDelete, "/api/v1/tickets/own-ticket", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID, "delete audit must carry the ticket organization")
	})

	t.Run("messages", func(t *testing.T) {
		repo := &tenantTicketRepo{tickets: []*domain.Ticket{ownTenantTicket()}}
		recorder := serveTenantTicketRequest(t, tenantTicketRouter(repo, &tenantTicketRecords{}),
			forgedOrgRequest(http.MethodGet, "/api/v1/tickets/own-ticket/messages", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("add-message", func(t *testing.T) {
		repo := &tenantTicketRepo{tickets: []*domain.Ticket{ownTenantTicket()}}
		records := &tenantTicketRecords{}
		recorder := serveTenantTicketRequest(t, tenantTicketRouter(repo, records),
			forgedOrgRequest(http.MethodPost, "/api/v1/tickets/own-ticket/messages",
				strings.NewReader(`{"author":"agent","content":"reply"}`)))

		require.Equal(t, http.StatusCreated, recorder.Code)
		assert.Equal(t, 1, repo.updateCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID)
	})
}
