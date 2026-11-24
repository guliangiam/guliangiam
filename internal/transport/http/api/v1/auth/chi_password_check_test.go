package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	orgmock "github.com/guliangiam/guliangiam/internal/domain/organizations/mock"
	usermock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// R2-B02: /password/check must ignore caller-supplied organizationSlug and
// username and use the authenticated session instead — otherwise the endpoint
// is a cross-tenant password-guessing oracle. The gomock expectations are
// strict: any lookup with the body values fails the test.
func TestPasswordCheckOverridesBodyWithSessionContext(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuthUoW(ctrl)
	orgRepo := orgmock.NewMockRepository(ctrl)
	userRepo := usermock.NewMockRepository(ctrl)

	uow.EXPECT().Organizations().Return(orgRepo).AnyTimes()
	uow.EXPECT().Users().Return(userRepo).AnyTimes()
	orgRepo.EXPECT().GetBySlug(gomock.Any(), "session-org").
		Return(&orgdomain.Organization{ID: "org-1", Slug: "session-org"}, nil)
	userRepo.EXPECT().GetByUsername(gomock.Any(), "org-1", "sessionuser").
		Return(nil, apperrors.NewNotFound("user not found"))

	svc := &appauth.Service{UOW: uow, Clock: clock.SystemClock{}}
	router := chi.NewRouter()
	registerPasswordRoutes(router, Handler{Service: svc})

	body := `{"organizationSlug":"attacker-org","username":"victim","password":"guess-me"}`
	req := httptest.NewRequest(http.MethodPost, "/password/check", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(WithSessionContext(req.Context(), SessionContext{
		UserID:           "user-1",
		Username:         "sessionuser",
		OrganizationID:   "org-1",
		OrganizationSlug: "session-org",
	}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.Equal(t, false, result["valid"])
}

func TestPasswordCheckRequiresSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuthUoW(ctrl)

	svc := &appauth.Service{UOW: uow, Clock: clock.SystemClock{}}
	router := chi.NewRouter()
	registerPasswordRoutes(router, Handler{Service: svc})

	body := `{"organizationSlug":"attacker-org","username":"victim","password":"guess-me"}`
	req := httptest.NewRequest(http.MethodPost, "/password/check", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
