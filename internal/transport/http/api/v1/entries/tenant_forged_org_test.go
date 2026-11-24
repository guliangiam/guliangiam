package entries

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appentries "github.com/guliangiam/guliangiam/internal/app/entries"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/entries"
	domainmock "github.com/guliangiam/guliangiam/internal/domain/entries/mock"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/proto"
)

const (
	forgedOrgSessionID = "11111111-1111-1111-1111-111111111111"
	forgedOrgRequestID = "22222222-2222-2222-2222-222222222222"
)

type forgedOrgClock struct{}

func (forgedOrgClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type forgedOrgUUID struct{}

func (forgedOrgUUID) NewString() (string, error) { return "generated-entry-id", nil }

func forgedOrgRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
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
	uow.EXPECT().Entries().Return(repo).AnyTimes()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appentries.NewService(forgedOrgClock{}, forgedOrgUUID{}, uow)})
	return router
}

// TestOtlpIngest_ForgedQueryOrganizationIDIgnored pins all three OTLP write
// endpoints: a forged organizationId must not inject entries into another
// tenant (entries:write would otherwise pollute foreign audit/forensics data).
func TestOtlpIngest_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	payload, err := proto.Marshal(&coltracepb.ExportTraceServiceRequest{})
	require.NoError(t, err)

	tests := []struct {
		name string
		path string
	}{
		{"traces", "/api/v1/entries/traces"},
		{"metrics", "/api/v1/entries/metrics"},
		{"logs", "/api/v1/entries/logs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := domainmock.NewMockRepository(ctrl)
			repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, entry *domain.Entry) error {
					assert.Equal(t, forgedOrgSessionID, entry.OrganizationID)
					return nil
				},
			)

			req := forgedOrgRequest(http.MethodPost,
				tt.path+"?organizationId="+forgedOrgRequestID+"&provider=collector",
				bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/x-protobuf;charset=utf-8")
			recorder := httptest.NewRecorder()
			forgedOrgRouter(ctrl, repo).ServeHTTP(recorder, req)
			require.Equal(t, http.StatusOK, recorder.Code)
		})
	}
}

// TestOtlpIngest_WithoutTenantScopeRejected pins the fail-fast guard so OTLP
// ingestion can never reach the repository with an empty organization.
func TestOtlpIngest_WithoutTenantScopeRejected(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entries/logs", bytes.NewReader(nil))
	req.Header.Set("Content-Type", "application/x-protobuf;charset=utf-8")
	recorder := httptest.NewRecorder()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appentries.NewService(forgedOrgClock{}, forgedOrgUUID{}, nil)})
	router.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}
