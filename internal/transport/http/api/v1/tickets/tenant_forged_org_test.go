package tickets

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	apptickets "github.com/guliangiam/guliangiam/internal/app/tickets"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/tickets"
	domainmock "github.com/guliangiam/guliangiam/internal/domain/tickets/mock"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	forgedOrgSessionID = "11111111-1111-1111-1111-111111111111"
	forgedOrgRequestID = "22222222-2222-2222-2222-222222222222"
)

type forgedOrgClock struct{}

func (forgedOrgClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type forgedOrgUUID struct{}

func (forgedOrgUUID) NewString() (string, error) { return "generated-ticket-id", nil }

func forgedOrgRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), forgedOrgSessionID, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: forgedOrgSessionID,
	})
	return req.WithContext(ctx)
}

func forgedOrgRouter(ctrl *gomock.Controller, repo *domainmock.MockRepository) chi.Router {
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	).AnyTimes()
	uow.EXPECT().Tickets().Return(repo).AnyTimes()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: apptickets.NewService(forgedOrgClock{}, forgedOrgUUID{}, uow)})
	return router
}

// TestListTickets_ForgedQueryOrganizationIDIgnored pins that the query value
// only requests a tenant; the authenticated scope wins.
func TestListTickets_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := domainmock.NewMockRepository(ctrl)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query domain.ListQuery) ([]*domain.Ticket, int64, error) {
			assert.Equal(t, forgedOrgSessionID, query.OrganizationID)
			return nil, 0, nil
		},
	)

	recorder := httptest.NewRecorder()
	forgedOrgRouter(ctrl, repo).ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/tickets?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestCreateTicket_ForgedBodyOrganizationIDIgnored pins that a regular caller
// cannot move a ticket write into another tenant through the JSON body.
func TestCreateTicket_ForgedBodyOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := domainmock.NewMockRepository(ctrl)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, ticket *domain.Ticket) error {
			assert.Equal(t, forgedOrgSessionID, ticket.OrganizationID)
			return nil
		},
	)

	recorder := httptest.NewRecorder()
	forgedOrgRouter(ctrl, repo).ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/tickets",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","name":"ticket","displayName":"Ticket"}`)))
	require.Equal(t, http.StatusCreated, recorder.Code)
}

// TestCreateTicket_WithoutTenantScopeRejected pins the fail-fast guard.
func TestCreateTicket_WithoutTenantScopeRejected(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tickets",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","name":"ticket","displayName":"Ticket"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: apptickets.Service{}})
	router.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}
