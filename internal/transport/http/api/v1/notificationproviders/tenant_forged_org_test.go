package notificationproviders

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	apppkg "github.com/guliangiam/guliangiam/internal/app/notificationproviders"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	domainmock "github.com/guliangiam/guliangiam/internal/domain/notificationproviders/mock"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	trustedOrg = "11111111-1111-1111-1111-111111111111"
	forgedOrg  = "22222222-2222-2222-2222-222222222222"
)

func forgedOrgRequest(target string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	return req.WithContext(tenantctx.WithTenantScope(req.Context(), trustedOrg, "session-org", tenantctx.TenantFilterStrict))
}

// TestListNotificationProviders_ForgedQueryOrganizationIDIgnored pins the list.
func TestListNotificationProviders_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockInfraUoW(ctrl)
	repo := domainmock.NewMockRepository(ctrl)
	uow.EXPECT().NotificationProviders().Return(repo).AnyTimes()
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q domain.ListQuery) ([]*domain.NotificationProvider, int64, error) {
			assert.Equal(t, trustedOrg, q.OrganizationID)
			return nil, 0, nil
		})

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: apppkg.Service{UOW: uow}})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest("/api/v1/notification-providers?organizationId="+forgedOrg))
	require.Equal(t, http.StatusOK, recorder.Code)
}
