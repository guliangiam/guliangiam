package users

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	orgmock "github.com/guliangiam/guliangiam/internal/domain/organizations/mock"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	sessionsmock "github.com/guliangiam/guliangiam/internal/domain/sessions/mock"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	usermock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// The self-service face takes the caller identity ONLY from the trusted session.
// These constants are the session principal; every ownership EXPECT below matches
// them, so a handler that leaked a body/query userID or organizationId into the
// app layer would fail the mock contract instead of silently operating on another
// principal.
const (
	selfSvcOrgID      = "org-self"
	selfSvcUserID     = "user-self"
	selfSvcForeignOrg = "org-foreign"
	selfSvcTotpSecret = "JBSWY3DPEHPK3PXP"
)

type selfServiceHarness struct {
	router   chi.Router
	users    *usermock.MockRepository
	orgs     *orgmock.MockRepository
	sessions *sessionsmock.MockRepository
}

func newSelfServiceHarness(ctrl *gomock.Controller, configure ...func(*appusers.Service)) *selfServiceHarness {
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	users := usermock.NewMockRepository(ctrl)
	orgs := orgmock.NewMockRepository(ctrl)
	sessions := sessionsmock.NewMockRepository(ctrl)

	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	).AnyTimes()
	uow.EXPECT().Users().Return(users).AnyTimes()
	uow.EXPECT().Organizations().Return(orgs).AnyTimes()
	uow.EXPECT().Sessions().Return(sessions).AnyTimes()
	// GetUser 回填 webauthnCredentialCount；零值 stub 返回空列表。
	uow.EXPECT().WebAuthnCredentials().Return(&delGetWebAuthn{}).AnyTimes()

	service := appusers.Service{
		Clock: clock.SystemClock{}, UUID: platformuuid.V7Generator{}, UOW: uow,
	}
	for _, fn := range configure {
		fn(&service)
	}
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return &selfServiceHarness{router: router, users: users, orgs: orgs, sessions: sessions}
}

func withSelfServiceSession(req *http.Request) *http.Request {
	return req.WithContext(authtransport.WithSessionContext(req.Context(), authtransport.SessionContext{
		UserID: selfSvcUserID, OrganizationID: selfSvcOrgID, OrganizationSlug: "self-org", Username: "self-user",
	}))
}

func selfServiceRequest(method, path, body string) *http.Request {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	return req
}

func (h *selfServiceHarness) serve(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func selfSvcUser() *userdomain.User {
	return &userdomain.User{
		ID:             selfSvcUserID,
		OrganizationID: selfSvcOrgID,
		Username:       "self-user",
		Revision:       1,
	}
}

func selfSvcOrg() *orgdomain.Organization {
	return &orgdomain.Organization{ID: selfSvcOrgID, Slug: "self-org", DisplayName: "Self Org"}
}

func currentSelfSvcTotpCode(t *testing.T) string {
	t.Helper()
	code, err := totp.GenerateCode(selfSvcTotpSecret, time.Now().UTC())
	require.NoError(t, err)
	return code
}

// TestSelfService_RequireSession pins the fail-close contract for all twelve
// endpoints: with no session context the handler answers 401 before any app work.
func TestSelfService_RequireSession(t *testing.T) {
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v1/auth/mfa-setup/initiate", `{"mfaType":"totp"}`},
		{http.MethodPost, "/api/v1/auth/mfa-setup/verify", `{"mfaType":"totp","passcode":"123456","secret":"` + selfSvcTotpSecret + `"}`},
		{http.MethodPost, "/api/v1/auth/mfa-setup/enable", `{"mfaType":"totp","passcode":"123456","secret":"` + selfSvcTotpSecret + `","recoveryCodes":["rc-1"]}`},
		{http.MethodPut, "/api/v1/auth/mfa-methods", `{"revision":1,"items":[]}`},
		{http.MethodPut, "/api/v1/auth/preferred-mfa", `{"mfaType":"totp","revision":1}`},
		{http.MethodDelete, "/api/v1/auth/identities/id-1", ""},
		{http.MethodGet, "/api/v1/auth/consents", ""},
		{http.MethodGet, "/api/v1/auth/profile", ""},
		{http.MethodPatch, "/api/v1/auth/profile", `{"displayName":"New Name"}`},
		{http.MethodPost, "/api/v1/auth/verify-identification", ""},
		{http.MethodPost, "/api/v1/auth/faceids", ""},
		{http.MethodDelete, "/api/v1/auth/faceids", ""},
	}
	for _, tt := range cases {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			h := newSelfServiceHarness(ctrl)
			rec := h.serve(selfServiceRequest(tt.method, tt.path, tt.body))
			assert.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
		})
	}
}

// TestSelfService_EmptySessionUserRejected pins the TrimSpace fail-close: a session
// that carries no user id is treated as unauthenticated.
func TestSelfService_EmptySessionUserRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := newSelfServiceHarness(ctrl)
	req := selfServiceRequest(http.MethodGet, "/api/v1/auth/consents", "")
	req = req.WithContext(authtransport.WithSessionContext(req.Context(), authtransport.SessionContext{
		UserID: "   ", OrganizationID: selfSvcOrgID, Username: "self-user",
	}))
	rec := h.serve(req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
}

// TestSelfService_InitiateMfaSetup proves the success shape and that the target
// principal comes from the session even when the query forges another
// organizationId (the EXPECT matcher pins the session values).
func TestSelfService_InitiateMfaSetup(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := newSelfServiceHarness(ctrl)

	h.users.EXPECT().GetByIDForOrganization(gomock.Any(), selfSvcUserID, selfSvcOrgID).Return(selfSvcUser(), nil)
	h.orgs.EXPECT().GetByID(gomock.Any(), selfSvcOrgID).Return(selfSvcOrg(), nil)

	rec := h.serve(withSelfServiceSession(selfServiceRequest(http.MethodPost,
		"/api/v1/auth/mfa-setup/initiate?organizationId="+selfSvcForeignOrg, `{"mfaType":"totp"}`)))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp InitiateUserMfaSetupResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Secret)
	assert.NotEmpty(t, resp.URL)
	assert.NotEmpty(t, resp.RecoveryCodes)
}

// TestSelfService_ForgedBodyUserRejected pins that the strict decoder refuses a
// body that tries to name another principal: there is no userID/organizationId
// field on the DTO, so such a payload is a 400 and never reaches the app layer.
func TestSelfService_ForgedBodyUserRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := newSelfServiceHarness(ctrl)

	body := `{"mfaType":"totp","userID":"attacker","organizationId":"` + selfSvcForeignOrg + `"}`
	rec := h.serve(withSelfServiceSession(selfServiceRequest(http.MethodPost, "/api/v1/auth/mfa-setup/initiate", body)))
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestSelfService_VerifyMfaSetup(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := newSelfServiceHarness(ctrl)

	h.users.EXPECT().GetByIDForOrganization(gomock.Any(), selfSvcUserID, selfSvcOrgID).Return(selfSvcUser(), nil)

	body := `{"mfaType":"totp","passcode":"` + currentSelfSvcTotpCode(t) + `","secret":"` + selfSvcTotpSecret + `"}`
	rec := h.serve(withSelfServiceSession(selfServiceRequest(http.MethodPost, "/api/v1/auth/mfa-setup/verify", body)))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"status":"verified","msg":"resource verified"}`, rec.Body.String())
}

func TestSelfService_EnableMfaSetup(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := newSelfServiceHarness(ctrl)

	h.users.EXPECT().GetByIDForOrganization(gomock.Any(), selfSvcUserID, selfSvcOrgID).Return(selfSvcUser(), nil)
	h.orgs.EXPECT().GetByID(gomock.Any(), selfSvcOrgID).Return(selfSvcOrg(), nil)
	h.users.EXPECT().UpdateForOrganization(gomock.Any(), gomock.Any(), selfSvcOrgID).Return(nil)
	h.users.EXPECT().ReplaceMfaMethods(gomock.Any(), selfSvcUserID, gomock.Any()).Return(nil)

	body := `{"mfaType":"totp","passcode":"` + currentSelfSvcTotpCode(t) + `","secret":"` + selfSvcTotpSecret + `","recoveryCodes":["rc-1"]}`
	rec := h.serve(withSelfServiceSession(selfServiceRequest(http.MethodPost, "/api/v1/auth/mfa-setup/enable", body)))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"status":"enabled","msg":"resource enabled"}`, rec.Body.String())
}

// TestSelfService_EnableMfaSetup_RotatesSessionCookie covers the S9-sensitive path:
// when the caller presents an opaque session cookie, enable rotates it and writes
// the new session cookie back.
func TestSelfService_EnableMfaSetup_RotatesSessionCookie(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := newSelfServiceHarness(ctrl)

	h.users.EXPECT().GetByIDForOrganization(gomock.Any(), selfSvcUserID, selfSvcOrgID).Return(selfSvcUser(), nil)
	h.orgs.EXPECT().GetByID(gomock.Any(), selfSvcOrgID).Return(selfSvcOrg(), nil)
	h.users.EXPECT().UpdateForOrganization(gomock.Any(), gomock.Any(), selfSvcOrgID).Return(nil)
	h.users.EXPECT().ReplaceMfaMethods(gomock.Any(), selfSvcUserID, gomock.Any()).Return(nil)
	h.sessions.EXPECT().GetByID(gomock.Any(), "prev-session").Return(&sessiondomain.Session{
		ID: "prev-session", UserID: selfSvcUserID, Username: "self-user", OrganizationID: selfSvcOrgID,
	}, nil)
	h.sessions.EXPECT().Revoke(gomock.Any(), "prev-session", gomock.Any()).Return(nil)
	h.sessions.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	body := `{"mfaType":"totp","passcode":"` + currentSelfSvcTotpCode(t) + `","secret":"` + selfSvcTotpSecret + `","recoveryCodes":["rc-1"]}`
	req := withSelfServiceSession(selfServiceRequest(http.MethodPost, "/api/v1/auth/mfa-setup/enable", body))
	req.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "prev-session"})
	rec := h.serve(req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"status":"enabled","msg":"resource enabled"}`, rec.Body.String())
	var rotated string
	for _, c := range rec.Result().Cookies() {
		if c.Name == authtransport.SessionCookieName && c.Value != "prev-session" {
			rotated = c.Value
		}
	}
	assert.NotEmpty(t, rotated, "enable must write back a rotated session cookie")
}

func TestSelfService_ReplaceMfaMethods(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := newSelfServiceHarness(ctrl)

	user := selfSvcUser()
	user.TotpSecret = selfSvcTotpSecret
	user.MfaMethods = []userdomain.UserMfaMethod{
		{ID: "m-1", UserID: selfSvcUserID, MethodType: "totp", Enabled: true, Verified: true},
	}
	h.users.EXPECT().GetByIDForOrganization(gomock.Any(), selfSvcUserID, selfSvcOrgID).Return(user, nil)
	h.orgs.EXPECT().GetByID(gomock.Any(), selfSvcOrgID).Return(selfSvcOrg(), nil)
	h.users.EXPECT().UpdateForOrganization(gomock.Any(), gomock.Any(), selfSvcOrgID).Return(nil)
	h.users.EXPECT().ReplaceMfaMethods(gomock.Any(), selfSvcUserID, gomock.Any()).Return(nil)

	body := `{"revision":1,"items":[{"id":"m-1","methodType":"totp","enabled":true,"verified":false,"displayName":"App"}]}`
	rec := h.serve(withSelfServiceSession(selfServiceRequest(http.MethodPut, "/api/v1/auth/mfa-methods", body)))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"status":"updated","msg":"resource updated"}`, rec.Body.String())
}

func TestSelfService_UpdatePreferredMfa(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := newSelfServiceHarness(ctrl)

	user := selfSvcUser()
	user.MfaMethods = []userdomain.UserMfaMethod{
		{ID: "m-1", UserID: selfSvcUserID, MethodType: "totp", Enabled: true, Verified: true},
	}
	h.users.EXPECT().GetByIDForOrganization(gomock.Any(), selfSvcUserID, selfSvcOrgID).Return(user, nil)
	h.users.EXPECT().UpdateForOrganization(gomock.Any(), gomock.Any(), selfSvcOrgID).Return(nil)

	rec := h.serve(withSelfServiceSession(selfServiceRequest(http.MethodPut, "/api/v1/auth/preferred-mfa",
		`{"mfaType":"totp","revision":1}`)))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"status":"updated","msg":"resource updated"}`, rec.Body.String())
}

// TestSelfService_DeleteIdentity covers the 204 success and ownership: an identity
// id that is not in the session user's aggregate is indistinguishable from missing
// (404) and never reaches DeleteIdentity.
func TestSelfService_DeleteIdentity(t *testing.T) {
	t.Run("own identity", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		h := newSelfServiceHarness(ctrl)
		user := selfSvcUser()
		user.Identities = []userdomain.UserIdentity{{ID: "id-1", UserID: selfSvcUserID, ProviderID: "google", Subject: "sub"}}
		h.users.EXPECT().GetByIDForOrganization(gomock.Any(), selfSvcUserID, selfSvcOrgID).Return(user, nil)
		h.users.EXPECT().DeleteIdentity(gomock.Any(), selfSvcUserID, "id-1").Return(nil)

		rec := h.serve(withSelfServiceSession(selfServiceRequest(http.MethodDelete, "/api/v1/auth/identities/id-1", "")))
		assert.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
		assert.Empty(t, rec.Body.String())
	})

	t.Run("foreign identity is missing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		h := newSelfServiceHarness(ctrl)
		user := selfSvcUser()
		user.Identities = []userdomain.UserIdentity{{ID: "id-1", UserID: selfSvcUserID}}
		h.users.EXPECT().GetByIDForOrganization(gomock.Any(), selfSvcUserID, selfSvcOrgID).Return(user, nil)
		// DeleteIdentity must NOT be called for an identity outside the aggregate.

		rec := h.serve(withSelfServiceSession(selfServiceRequest(http.MethodDelete, "/api/v1/auth/identities/id-other", "")))
		assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	})
}

// TestSelfService_ListConsents covers the success shape and that a forged query
// organizationId is ignored: ownership resolves through the session tenant.
func TestSelfService_ListConsents(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := newSelfServiceHarness(ctrl)

	user := selfSvcUser()
	user.ApplicationScopes = []userdomain.ConsentRecord{
		{Application: "app-1", Scopes: []string{"read", "write"}, GrantedAt: "2026-01-01T00:00:00Z"},
	}
	h.users.EXPECT().GetByIDForOrganization(gomock.Any(), selfSvcUserID, selfSvcOrgID).Return(user, nil)

	rec := h.serve(withSelfServiceSession(selfServiceRequest(http.MethodGet,
		"/api/v1/auth/consents?organizationId="+selfSvcForeignOrg, "")))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var items []appusers.UserConsentItem
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &items))
	require.Len(t, items, 1)
	assert.Equal(t, "app-1", items[0].ApplicationID)
	assert.Equal(t, []string{"read", "write"}, items[0].GrantedScopes)
}

// TestSelfService_ForeignTenantRejected pins that the session tenant is the only
// trusted scope: when the session user does not belong to the session org (the
// scoped lookup fails), the endpoint answers 404 and performs no write.
func TestSelfService_ForeignTenantRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := newSelfServiceHarness(ctrl)

	h.users.EXPECT().GetByIDForOrganization(gomock.Any(), selfSvcUserID, selfSvcOrgID).
		Return(nil, apperrors.NewNotFound("user not found"))

	rec := h.serve(withSelfServiceSession(selfServiceRequest(http.MethodGet, "/api/v1/auth/consents", "")))
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

// ---------------------------------------------------------------------------
// Batch 2: self profile read/merge, real-name verification, FaceID enroll/revoke
// ---------------------------------------------------------------------------

// selfSvcProfileUser is the stored aggregate the merge tests overlay onto. The
// contact fields carry synthetic values and verified flags that must survive
// every self-service profile edit untouched.
func selfSvcProfileUser() *userdomain.User {
	user := selfSvcUser()
	user.Profile = userdomain.UserProfile{
		UserID:        selfSvcUserID,
		DisplayName:   "Old Name",
		Bio:           "keep me",
		Email:         "self@example.com",
		EmailVerified: true,
		Phone:         "+8613800000000",
		PhoneVerified: true,
		Properties: map[string]any{
			"homepage":  "https://old.example.com",
			"education": "bachelor",
			"language":  "zh-CN",
		},
	}
	return user
}

// TestSelfService_GetProfile pins the read contract: full UserDetail shape,
// no-store caching, and the session tenant wins over a forged query org.
func TestSelfService_GetProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := newSelfServiceHarness(ctrl)

	h.users.EXPECT().GetByIDForOrganization(gomock.Any(), selfSvcUserID, selfSvcOrgID).Return(selfSvcProfileUser(), nil)
	h.orgs.EXPECT().GetByID(gomock.Any(), selfSvcOrgID).Return(selfSvcOrg(), nil)

	rec := h.serve(withSelfServiceSession(selfServiceRequest(http.MethodGet,
		"/api/v1/auth/profile?organizationId="+selfSvcForeignOrg, "")))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	var detail appusers.UserDetail
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &detail))
	assert.Equal(t, selfSvcUserID, detail.ID)
	assert.Equal(t, int64(1), detail.Revision)
	assert.Equal(t, "Old Name", detail.Profile["displayName"])
	assert.Equal(t, "self@example.com", detail.Profile["email"])
	assert.Equal(t, true, detail.Profile["emailVerified"])
}

// TestSelfService_GetProfile_ForeignTenantRejected: the scoped lookup fails →
// 404, never another tenant's profile.
func TestSelfService_GetProfile_ForeignTenantRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := newSelfServiceHarness(ctrl)
	h.users.EXPECT().GetByIDForOrganization(gomock.Any(), selfSvcUserID, selfSvcOrgID).
		Return(nil, apperrors.NewNotFound("user not found"))

	rec := h.serve(withSelfServiceSession(selfServiceRequest(http.MethodGet, "/api/v1/auth/profile", "")))
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

// TestSelfService_UpdateProfile_RejectsContactsAndForgedIdentity pins the
// strict-decoder boundary: email/phone (contact binding lives on /auth/account)
// and any body-supplied userID/organizationId are 400 before any app work.
func TestSelfService_UpdateProfile_RejectsContactsAndForgedIdentity(t *testing.T) {
	cases := []string{
		`{"email":"attacker@example.com"}`,
		`{"phone":"+8613900000000"}`,
		`{"displayName":"x","userID":"attacker","organizationId":"` + selfSvcForeignOrg + `"}`,
		`{"displayName":"x","id":"attacker"}`,
	}
	for _, body := range cases {
		t.Run(body, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			h := newSelfServiceHarness(ctrl)
			// No repository EXPECT: a payload that reaches the app layer fails
			// the mock contract instead of silently writing.
			rec := h.serve(withSelfServiceSession(selfServiceRequest(http.MethodPatch, "/api/v1/auth/profile", body)))
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		})
	}
}

// TestSelfService_UpdateProfile_PartialMerge proves the merge semantics end to
// end: only the supplied fields change; email/phone and their verified flags,
// untouched columns and stored property keys survive; revision increments.
func TestSelfService_UpdateProfile_PartialMerge(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := newSelfServiceHarness(ctrl)

	h.users.EXPECT().GetByIDForOrganization(gomock.Any(), selfSvcUserID, selfSvcOrgID).Return(selfSvcProfileUser(), nil)
	var savedUser *userdomain.User
	h.users.EXPECT().UpdateForOrganization(gomock.Any(), gomock.Any(), selfSvcOrgID).DoAndReturn(
		func(_ context.Context, user *userdomain.User, _ string) error {
			savedUser = user
			return nil
		})
	var savedProfile *userdomain.UserProfile
	h.users.EXPECT().UpdateProfile(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, profile *userdomain.UserProfile) error {
			savedProfile = profile
			return nil
		})

	body := `{"displayName":"New Name","title":"Engineer","properties":{"homepage":"","extra":"kept"}}`
	rec := h.serve(withSelfServiceSession(selfServiceRequest(http.MethodPatch,
		"/api/v1/auth/profile?organizationId="+selfSvcForeignOrg, body)))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"status":"updated","msg":"resource updated"}`, rec.Body.String())
	require.NotNil(t, savedProfile)
	assert.Equal(t, "New Name", savedProfile.DisplayName)
	assert.Equal(t, "keep me", savedProfile.Bio)
	assert.Equal(t, "self@example.com", savedProfile.Email)
	assert.True(t, savedProfile.EmailVerified)
	assert.Equal(t, "+8613800000000", savedProfile.Phone)
	assert.True(t, savedProfile.PhoneVerified)
	assert.Equal(t, "Engineer", savedProfile.Properties["title"])
	assert.Equal(t, "bachelor", savedProfile.Properties["education"])
	assert.Equal(t, "zh-CN", savedProfile.Properties["language"])
	assert.Equal(t, "kept", savedProfile.Properties["extra"])
	assert.NotContains(t, savedProfile.Properties, "homepage", "explicit empty string deletes the key")
	require.NotNil(t, savedUser)
	assert.Equal(t, int64(2), savedUser.Revision)
}

// TestSelfService_UpdateProfile_ForeignTenantRejected: scoped lookup fails →
// 404 and no write is attempted.
func TestSelfService_UpdateProfile_ForeignTenantRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := newSelfServiceHarness(ctrl)
	h.users.EXPECT().GetByIDForOrganization(gomock.Any(), selfSvcUserID, selfSvcOrgID).
		Return(nil, apperrors.NewNotFound("user not found"))

	rec := h.serve(withSelfServiceSession(selfServiceRequest(http.MethodPatch, "/api/v1/auth/profile", `{"displayName":"x"}`)))
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

// selfIDVGateway is a stub IdentityVerificationGateway capturing the request.
type selfIDVGateway struct {
	ticket  appusers.IdentityVerificationTicket
	request appusers.IdentityVerificationRequest
	called  bool
}

func (g *selfIDVGateway) InitiateUserVerification(_ context.Context, request appusers.IdentityVerificationRequest) (appusers.IdentityVerificationTicket, error) {
	g.called = true
	g.request = request
	return g.ticket, nil
}

// TestSelfService_VerifyIdentification_AlreadyVerified: a verified user gets the
// terminal action response and the provider is never called.
func TestSelfService_VerifyIdentification_AlreadyVerified(t *testing.T) {
	ctrl := gomock.NewController(t)
	gateway := &selfIDVGateway{}
	h := newSelfServiceHarness(ctrl, func(s *appusers.Service) { s.IdentityVerification = gateway })

	user := selfSvcUser()
	user.IsVerified = true
	h.users.EXPECT().GetByIDForOrganization(gomock.Any(), selfSvcUserID, selfSvcOrgID).Return(user, nil)

	rec := h.serve(withSelfServiceSession(selfServiceRequest(http.MethodPost, "/api/v1/auth/verify-identification", "")))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"status":"verified","msg":"resource verified"}`, rec.Body.String())
	assert.False(t, gateway.called)
	assert.Empty(t, rec.Result().Cookies(), "verification must never issue a session cookie")
}

// TestSelfService_VerifyIdentification_Pending pins the ticket data shape and
// that the claimed identity comes from the stored profile, not the payload.
func TestSelfService_VerifyIdentification_Pending(t *testing.T) {
	ctrl := gomock.NewController(t)
	gateway := &selfIDVGateway{ticket: appusers.IdentityVerificationTicket{
		VerificationID: "ver-1",
		Provider:       "stub",
		RedirectURL:    "https://provider.example.com/session/ver-1",
		ExpiresAt:      "2026-01-01T00:00:00Z",
	}}
	h := newSelfServiceHarness(ctrl, func(s *appusers.Service) { s.IdentityVerification = gateway })

	user := selfSvcUser()
	user.Profile = userdomain.UserProfile{
		UserID:     selfSvcUserID,
		RealName:   "Synthetic Test Name",
		IDCardType: "id_card",
		IDCard:     "TEST-DOC-0001",
	}
	h.users.EXPECT().GetByIDForOrganization(gomock.Any(), selfSvcUserID, selfSvcOrgID).Return(user, nil)

	rec := h.serve(withSelfServiceSession(selfServiceRequest(http.MethodPost,
		"/api/v1/auth/verify-identification", `{"metaInfo":"{}","country":"CHN"}`)))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		Status string         `json:"status"`
		Msg    string         `json:"msg"`
		Data   map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "pending", resp.Status)
	assert.Equal(t, "identity verification session created", resp.Msg)
	assert.Equal(t, "ver-1", resp.Data["verificationId"])
	assert.Equal(t, "stub", resp.Data["provider"])
	assert.Equal(t, "https://provider.example.com/session/ver-1", resp.Data["redirectUrl"])
	assert.Equal(t, "2026-01-01T00:00:00Z", resp.Data["expiresAt"])
	require.True(t, gateway.called)
	assert.Equal(t, selfSvcOrgID, gateway.request.OrganizationID)
	assert.Equal(t, selfSvcUserID, gateway.request.UserID)
	assert.Equal(t, "Synthetic Test Name", gateway.request.FullName)
	assert.Empty(t, rec.Result().Cookies(), "verification must never issue a session cookie")
}

// selfFaceGateway is a stub FaceIDEnrollmentGateway capturing the request.
type selfFaceGateway struct {
	faceID  userdomain.FaceId
	request appusers.FaceIDEnrollmentRequest
}

func (g *selfFaceGateway) EnrollFaceID(_ context.Context, request appusers.FaceIDEnrollmentRequest) (userdomain.FaceId, error) {
	g.request = request
	return g.faceID, nil
}

// selfFaceTemplateRepo captures the sealed template writes; the embedded nil
// interface is never exercised beyond Enroll/RevokeByUser in these tests.
type selfFaceTemplateRepo struct {
	userdomain.FaceTemplateRepository
	enrolled []*userdomain.FaceTemplate
	revoked  []string
}

func (r *selfFaceTemplateRepo) Enroll(_ context.Context, template *userdomain.FaceTemplate) error {
	r.enrolled = append(r.enrolled, template)
	return nil
}

func (r *selfFaceTemplateRepo) RevokeByUser(_ context.Context, _, userID string, _ time.Time) error {
	r.revoked = append(r.revoked, userID)
	return nil
}

// selfFaceUoW adds the narrow FaceTemplates capability the app asserts for.
// The transaction callback receives the wrapper (not the inner mock) so the
// capability survives into the tx; Records/Outbox are stubbed nil because the
// wrapper is not gomock-detected by the audit embed.
type selfFaceUoW struct {
	*appuowmock.MockUnitOfWork
	templates userdomain.FaceTemplateRepository
}

func (u selfFaceUoW) FaceTemplates() userdomain.FaceTemplateRepository { return u.templates }

type selfFaceHarness struct {
	router    chi.Router
	users     *usermock.MockRepository
	templates *selfFaceTemplateRepo
	gateway   *selfFaceGateway
}

func newSelfFaceHarness(ctrl *gomock.Controller) *selfFaceHarness {
	inner := appuowmock.NewMockUnitOfWork(ctrl)
	users := usermock.NewMockRepository(ctrl)
	templates := &selfFaceTemplateRepo{}
	gateway := &selfFaceGateway{faceID: userdomain.FaceId{ID: "face-1", CreatedAt: "2026-01-01T00:00:00Z"}}
	uow := selfFaceUoW{MockUnitOfWork: inner, templates: templates}

	inner.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	).AnyTimes()
	inner.EXPECT().Users().Return(users).AnyTimes()
	inner.EXPECT().Records().Return(nil).AnyTimes()
	inner.EXPECT().Outbox().Return(nil).AnyTimes()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appusers.Service{
		Clock: clock.SystemClock{}, UUID: platformuuid.V7Generator{}, UOW: uow,
		FaceIDEnrollment: gateway,
	}})
	return &selfFaceHarness{router: router, users: users, templates: templates, gateway: gateway}
}

func (h *selfFaceHarness) serve(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

// selfFaceMultipartRequest builds the multipart upload body. withFile=false
// omits the file part entirely (missing-file rejection path).
func selfFaceMultipartRequest(t *testing.T, method, path string, content []byte, withFile bool) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if withFile {
		part, err := writer.CreateFormFile("file", "face.png")
		require.NoError(t, err)
		_, err = part.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// pngFaceBytes carries the PNG magic so http.DetectContentType resolves a real
// media type when the part header is application/octet-stream.
var pngFaceBytes = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR synthetic-face")

// TestSelfService_EnrollFaceID_RejectsBadUpload pins the shared multipart
// contract: missing file, empty file and oversize file are all 400 before any
// repository or provider work.
func TestSelfService_EnrollFaceID_RejectsBadUpload(t *testing.T) {
	cases := []struct {
		name     string
		content  []byte
		withFile bool
	}{
		{"missing file", nil, false},
		{"empty file", []byte{}, true},
		{"oversize file", make([]byte, maxFaceImageBytes+1), true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			h := newSelfFaceHarness(ctrl)
			req := withSelfServiceSession(selfFaceMultipartRequest(t, http.MethodPost, "/api/v1/auth/faceids", tt.content, tt.withFile))
			rec := h.serve(req)
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.Empty(t, h.templates.enrolled)
		})
	}
}

// TestSelfService_EnrollFaceID_Success pins the 201 shape, the session-bound
// ownership (pre-check + under-lock reads both match the session principal)
// and the sealed template write.
func TestSelfService_EnrollFaceID_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := newSelfFaceHarness(ctrl)

	user := selfSvcUser()
	user.IsVerified = true
	h.users.EXPECT().GetByIDForOrganization(gomock.Any(), selfSvcUserID, selfSvcOrgID).Return(user, nil).Times(2)
	h.users.EXPECT().LockByID(gomock.Any(), selfSvcUserID).Return(nil)
	var marker userdomain.UserCollections
	h.users.EXPECT().UpdateCollectionsForOrganization(gomock.Any(), selfSvcUserID, selfSvcOrgID, gomock.Any()).DoAndReturn(
		func(_ context.Context, _, _ string, collections userdomain.UserCollections) error {
			marker = collections
			return nil
		})

	req := selfFaceMultipartRequest(t, http.MethodPost, "/api/v1/auth/faceids?organizationId="+selfSvcForeignOrg, pngFaceBytes, true)
	rec := h.serve(withSelfServiceSession(req))

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var item FaceIDItem
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &item))
	assert.Equal(t, "face-1", item.ID)
	assert.Equal(t, "2026-01-01T00:00:00Z", item.CreatedAt)
	assert.Equal(t, selfSvcOrgID, h.gateway.request.OrganizationID)
	assert.Equal(t, selfSvcUserID, h.gateway.request.UserID)
	assert.Equal(t, "image/png", h.gateway.request.MimeType)
	require.Len(t, h.templates.enrolled, 1)
	assert.Equal(t, selfSvcUserID, h.templates.enrolled[0].UserID)
	assert.Equal(t, userdomain.FaceTemplateStatusActive, h.templates.enrolled[0].Status)
	require.Len(t, marker.FaceIds, 1)
	assert.Equal(t, "face-1", marker.FaceIds[0].ID)
	assert.Empty(t, marker.FaceIds[0].ImageUrl, "legacy marker must not carry a plaintext image URL")
}

// TestSelfService_RevokeFaceID_Success pins the 204 contract and that the
// revocation targets the session principal only.
func TestSelfService_RevokeFaceID_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	h := newSelfFaceHarness(ctrl)

	user := selfSvcUser()
	user.IsVerified = true
	h.users.EXPECT().GetByIDForOrganization(gomock.Any(), selfSvcUserID, selfSvcOrgID).Return(user, nil)
	var marker userdomain.UserCollections
	h.users.EXPECT().UpdateCollectionsForOrganization(gomock.Any(), selfSvcUserID, selfSvcOrgID, gomock.Any()).DoAndReturn(
		func(_ context.Context, _, _ string, collections userdomain.UserCollections) error {
			marker = collections
			return nil
		})

	rec := h.serve(withSelfServiceSession(selfServiceRequest(http.MethodDelete, "/api/v1/auth/faceids", "")))

	assert.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Empty(t, rec.Body.String())
	assert.Equal(t, []string{selfSvcUserID}, h.templates.revoked)
	assert.Empty(t, marker.FaceIds, "legacy face id marker must be cleared")
}
