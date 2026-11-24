package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	orgmock "github.com/guliangiam/guliangiam/internal/domain/organizations/mock"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	tokensmock "github.com/guliangiam/guliangiam/internal/domain/tokens/mock"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	usermock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
)

// TestExitImpersonateRoute_RevokesTargetUsername guards the auth-face exit
// binding: ExitImpersonate takes a username, not the organization slug
// (Account.Owner). Passing Owner made GetByUsername 404 and left imp tokens
// live while the FE believed the exit succeeded.
func TestExitImpersonateRoute_RevokesTargetUsername(t *testing.T) {
	ctrl := gomock.NewController(t)
	authUoW := appuowmock.NewMockAuthUoW(ctrl)
	tx := appuowmock.NewMockUnitOfWork(ctrl)
	orgRepo := orgmock.NewMockRepository(ctrl)
	userRepo := usermock.NewMockRepository(ctrl)
	tokenRepo := tokensmock.NewMockRepository(ctrl)

	authUoW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, tx)
		}).Times(2)
	tx.EXPECT().Organizations().Return(orgRepo).AnyTimes()
	tx.EXPECT().Users().Return(userRepo).AnyTimes()
	tx.EXPECT().Tokens().Return(tokenRepo).AnyTimes()
	orgRepo.EXPECT().GetBySlug(gomock.Any(), "acme").
		Return(&orgdomain.Organization{ID: "org-1", Slug: "acme"}, nil).Times(2)
	userRepo.EXPECT().GetByUsername(gomock.Any(), "org-1", "bob").
		Return(&userdomain.User{ID: "user-target", Username: "bob", OrganizationID: "org-1"}, nil).Times(2)

	var revoked tokendomain.RevokeQuery
	tokenRepo.EXPECT().RevokeByQuery(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query tokendomain.RevokeQuery, _ time.Time) (int64, error) {
			revoked = query
			return 1, nil
		})

	svc := &appauth.Service{UOW: authUoW, Clock: clock.SystemClock{}}
	router := chi.NewRouter()
	registerSharedRoutes(router, Handler{Service: svc})

	session := SessionContext{
		UserID: "user-target", Username: "bob",
		OrganizationID: "org-1", OrganizationSlug: "acme",
		AccessToken: "imp_raw_token", Source: "impersonation",
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		router.ServeHTTP(w, r.WithContext(WithSessionContext(r.Context(), session)))
	})

	req := httptest.NewRequest(http.MethodPost, "/exit-impersonate", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	require.Equal(t, "org-1", revoked.OrganizationID)
	require.Equal(t, "user-target", revoked.UserID)
	require.Equal(t, "imp", revoked.TokenPrefix)
}
