package applications

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appapplications "github.com/guliangiam/guliangiam/internal/app/applications"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	httpmw "github.com/guliangiam/guliangiam/internal/transport/http/middleware"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

type metaAuditAppRepo struct {
	domain.Repository
	created []*domain.Application
}

func (r *metaAuditAppRepo) GetByOrganizationAndSlug(context.Context, string, string) (*domain.Application, error) {
	return nil, apperrors.NewNotFound("application not found")
}

func (r *metaAuditAppRepo) HasDeletedWithOrgAndSlug(context.Context, string, string) (bool, error) {
	return false, nil
}

func (r *metaAuditAppRepo) Create(_ context.Context, application *domain.Application) error {
	r.created = append(r.created, application)
	return nil
}

type metaAuditRecords struct {
	domainrecords.Repository
	created []*domainrecords.Record
}

func (r *metaAuditRecords) Create(_ context.Context, record *domainrecords.Record) error {
	r.created = append(r.created, record)
	return nil
}

type metaAuditOutbox struct {
	domainoutbox.Repository
	events []*domainoutbox.OutboxEvent
}

func (r *metaAuditOutbox) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	r.events = append(r.events, event)
	return nil
}

// metaAuditUOW is a hand-written (non-gomock) unit of work so the audit writer
// is actually exercised: platform/outbox/providers/audit skips gomock handles.
type metaAuditUOW struct {
	appuow.UnitOfWork
	apps    *metaAuditAppRepo
	records *metaAuditRecords
	outbox  *metaAuditOutbox
}

func (u *metaAuditUOW) Applications() domain.Repository   { return u.apps }
func (u *metaAuditUOW) Records() domainrecords.Repository { return u.records }
func (u *metaAuditUOW) Outbox() domainoutbox.Repository   { return u.outbox }
func (u *metaAuditUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

// metaAuditRouter mounts the real metadata chain: chi request ids, then the
// route surface/ids stamp, then the platform request-metadata stamp.
func metaAuditRouter(uow appuow.AuthUoW) chi.Router {
	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	router.Use(route.RequestMetadataMiddleware())
	router.Use(httpmw.RequestMetadataMiddleware())
	RegisterChiRoutes(router, Handler{Service: appapplications.NewService(
		tenantRegressionClock{}, tenantRegressionUUID{}, uow)})
	return router
}

func metaAuditCreateRequest() *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/applications", strings.NewReader(
		`{"organizationId":"`+tenantRegressionSessionOrg+`","slug":"metadata-audit",`+
			`"displayName":"Metadata Audit","status":"active",`+
			`"clientId":"metadata-audit-client","tokenEndpointAuthMethod":"none",`+
			`"grantTypes":["authorization_code"]}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "metadata-audit/1.0")
	request.RemoteAddr = "203.0.113.10:1234"
	return request.WithContext(authtransport.WithSessionContext(request.Context(), authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: tenantRegressionSessionOrg,
	}))
}

// TestCreateApplication_AuditRecordCarriesRequestMetadata pins the identity
// domain's audit origin evidence end to end: the handler passes no metadata, so
// the record's ip/userAgent/surface/requestId can only come from the request
// metadata stamped on the context by the middleware chain.
func TestCreateApplication_AuditRecordCarriesRequestMetadata(t *testing.T) {
	uow := &metaAuditUOW{
		apps:    &metaAuditAppRepo{},
		records: &metaAuditRecords{},
		outbox:  &metaAuditOutbox{},
	}

	recorder := httptest.NewRecorder()
	metaAuditRouter(uow).ServeHTTP(recorder, metaAuditCreateRequest())
	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Len(t, uow.apps.created, 1)
	require.Len(t, uow.records.created, 1)

	record := uow.records.created[0]
	assert.Equal(t, tenantRegressionSessionOrg, record.OrganizationID)
	assert.NotEmpty(t, record.IP, "审计记录必须带上观测到的客户端 IP")
	assert.Equal(t, "203.0.113.10", record.IP)
	assert.Equal(t, "metadata-audit/1.0", record.UserAgent)
	assert.Equal(t, string(route.SurfaceManagement), record.Surface)
	assert.NotEmpty(t, record.RequestID)
}
