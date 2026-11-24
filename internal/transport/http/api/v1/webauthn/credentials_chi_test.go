package webauthn

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	usermock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	webauthnmock "github.com/guliangiam/guliangiam/internal/domain/webauthn/mock"
	apiauth "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newCredentialTestRouter(t *testing.T, ctrl *gomock.Controller) (chi.Router, *webauthnmock.MockRepository, *usermock.MockRepository) {
	t.Helper()
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	webAuthnRepo := webauthnmock.NewMockRepository(ctrl)
	userRepo := usermock.NewMockRepository(ctrl)
	uow.EXPECT().WebAuthnCredentials().Return(webAuthnRepo).AnyTimes()
	uow.EXPECT().Users().Return(userRepo).AnyTimes()

	router := chi.NewRouter()
	RegisterChiRoutes(router, &Handler{AuthService: &appauth.Service{UOW: uow}})
	return router, webAuthnRepo, userRepo
}

func withCredentialSession(request *http.Request) *http.Request {
	return request.WithContext(apiauth.WithSessionContext(request.Context(), apiauth.SessionContext{
		UserID: "user-1", OrganizationID: "org-1", OrganizationSlug: "acme", Username: "victim",
	}))
}

func TestCredentialsRoutes_RequireSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, _, _ := newCredentialTestRouter(t, ctrl)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/auth/webauthn/credentials", nil))
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestCredentialsRoutes_ListOwnCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, webAuthnRepo, userRepo := newCredentialTestRouter(t, ctrl)

	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").
		Return(&userdomain.User{ID: "user-1", OrganizationID: "org-1", Username: "victim"}, nil)
	webAuthnRepo.EXPECT().ListByUserID(gomock.Any(), "user-1").Return([]*webauthndomain.Credential{{
		ID: "cred-1", UserID: "user-1", CredentialID: []byte{1, 2}, Name: "Laptop",
		Status: webauthndomain.CredentialStatusActive, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}}, nil)

	recorder := httptest.NewRecorder()
	request := withCredentialSession(httptest.NewRequest(http.MethodGet, "/api/v1/auth/webauthn/credentials", nil))
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var credentials []appauth.WebAuthnCredentialSummary
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &credentials))
	require.Len(t, credentials, 1)
	assert.Equal(t, "cred-1", credentials[0].ID)
	assert.Equal(t, "active", credentials[0].Status)
}

func TestCredentialsRoutes_DeleteOnlyOwnCredential(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, webAuthnRepo, userRepo := newCredentialTestRouter(t, ctrl)

	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "user-1", "org-1").
		Return(&userdomain.User{ID: "user-1", OrganizationID: "org-1", Username: "victim"}, nil).Times(2)
	webAuthnRepo.EXPECT().ListByUserID(gomock.Any(), "user-1").Return([]*webauthndomain.Credential{
		{ID: "cred-1", UserID: "user-1"},
	}, nil).Times(2)
	webAuthnRepo.EXPECT().UpdateStatus(gomock.Any(), "cred-1", webauthndomain.CredentialStatusRevoked).Return(nil).Times(1)

	okRecorder := httptest.NewRecorder()
	okRequest := withCredentialSession(httptest.NewRequest(http.MethodDelete, "/api/v1/auth/webauthn/credentials/cred-1", nil))
	router.ServeHTTP(okRecorder, okRequest)
	assert.Equal(t, http.StatusNoContent, okRecorder.Code)

	foreignRecorder := httptest.NewRecorder()
	foreignRequest := withCredentialSession(httptest.NewRequest(http.MethodDelete, "/api/v1/auth/webauthn/credentials/cred-other", nil))
	router.ServeHTTP(foreignRecorder, foreignRequest)
	assert.Equal(t, http.StatusNotFound, foreignRecorder.Code, foreignRecorder.Body.String())
}

var _ = context.Background
