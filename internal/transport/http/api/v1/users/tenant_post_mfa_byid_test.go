package users

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	mfaPostCredentialID  = "Y3JlZC0x" // base64url("cred-1")
	mfaPostCredentialRaw = "cred-1"
)

var mfaPostCredentialBytes = []byte(mfaPostCredentialRaw)

// mfaPostRepo keeps the system-scoped read available (pre-fix behavior) and
// records every scoped lookup so a forged query organizationId is observable.
type mfaPostRepo struct {
	userdomain.Repository
	user *userdomain.User

	getByIDCalls       int
	scopedGetCalls     int
	scopedOrgs         []string
	updateCalls        int
	updateProfileCalls int
	replaceMfaCalls    int
}

func (r *mfaPostRepo) GetByID(_ context.Context, id string) (*userdomain.User, error) {
	r.getByIDCalls++
	if r.user == nil || r.user.ID != id {
		return nil, apperrors.NewNotFound("user not found")
	}
	return r.user, nil
}

func (r *mfaPostRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*userdomain.User, error) {
	r.scopedGetCalls++
	r.scopedOrgs = append(r.scopedOrgs, organizationID)
	if r.user == nil || r.user.ID != id || r.user.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("user not found")
	}
	return r.user, nil
}

func (r *mfaPostRepo) Update(context.Context, *userdomain.User) error {
	r.updateCalls++
	return nil
}

func (r *mfaPostRepo) UpdateForOrganization(_ context.Context, user *userdomain.User, organizationID string) error {
	r.updateCalls++
	if user.OrganizationID != organizationID {
		return apperrors.NewNotFound("user not found")
	}
	return nil
}

func (r *mfaPostRepo) UpdateProfile(context.Context, *userdomain.UserProfile) error {
	r.updateProfileCalls++
	return nil
}

func (r *mfaPostRepo) ReplaceMfaMethods(context.Context, string, []userdomain.UserMfaMethod) error {
	r.replaceMfaCalls++
	return nil
}

type mfaPostWebAuthn struct {
	webauthndomain.Repository
	cred              *webauthndomain.Credential
	getCalls          int
	updateStatusCalls int
	lastStatus        webauthndomain.CredentialStatus
}

func (r *mfaPostWebAuthn) GetByCredentialID(_ context.Context, credentialID []byte) (*webauthndomain.Credential, error) {
	r.getCalls++
	if r.cred == nil || !bytes.Equal(r.cred.CredentialID, credentialID) {
		return nil, apperrors.NewNotFound("credential not found")
	}
	return r.cred, nil
}

func (r *mfaPostWebAuthn) UpdateStatus(_ context.Context, _ string, status webauthndomain.CredentialStatus) error {
	r.updateStatusCalls++
	r.lastStatus = status
	if r.cred != nil {
		r.cred.Status = status
	}
	return nil
}

type mfaPostSessions struct {
	sessiondomain.Repository
	getByIDCalls      int
	revokeByUserCalls int
	createCalls       int
}

func (r *mfaPostSessions) GetByID(context.Context, string) (*sessiondomain.Session, error) {
	r.getByIDCalls++
	return nil, apperrors.NewNotFound("session not found")
}

func (r *mfaPostSessions) RevokeByUser(context.Context, string, time.Time) (int64, error) {
	r.revokeByUserCalls++
	return 1, nil
}

func (r *mfaPostSessions) Create(context.Context, *sessiondomain.Session) error {
	r.createCalls++
	return nil
}

type mfaPostTokens struct {
	tokendomain.Repository
	refreshCalls int
	revokeCalls  int
	queries      []tokendomain.RevokeQuery
}

func (r *mfaPostTokens) RevokeRefreshByUser(context.Context, string, time.Time) (int64, error) {
	r.refreshCalls++
	return 1, nil
}

func (r *mfaPostTokens) RevokeByQuery(_ context.Context, query tokendomain.RevokeQuery, _ time.Time) (int64, error) {
	r.revokeCalls++
	r.queries = append(r.queries, query)
	return 1, nil
}

type mfaPostUnitOfWork struct {
	appuow.UnitOfWork
	users    *mfaPostRepo
	webauthn *mfaPostWebAuthn
	sessions *mfaPostSessions
	tokens   *mfaPostTokens
	records  *postByIDRecords
	outbox   *postByIDOutbox
}

func (u *mfaPostUnitOfWork) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func (u *mfaPostUnitOfWork) Users() userdomain.Repository                   { return u.users }
func (u *mfaPostUnitOfWork) WebAuthnCredentials() webauthndomain.Repository { return u.webauthn }
func (u *mfaPostUnitOfWork) Sessions() sessiondomain.Repository             { return u.sessions }
func (u *mfaPostUnitOfWork) Tokens() tokendomain.Repository                 { return u.tokens }
func (u *mfaPostUnitOfWork) Records() recordsdomain.Repository              { return u.records }
func (u *mfaPostUnitOfWork) Outbox() domainoutbox.Repository                { return u.outbox }

// Organizations backs the server-owned MFA collections mirror, which reads the
// organization for the TOTP issuer / accountName derivation.
func (u *mfaPostUnitOfWork) Organizations() orgdomain.Repository { return mfaPostOrgs{} }

type mfaPostOrgs struct {
	orgdomain.Repository
}

func (mfaPostOrgs) GetByID(_ context.Context, id string) (*orgdomain.Organization, error) {
	return &orgdomain.Organization{ID: id, Slug: "built-in", DisplayName: "Built In"}, nil
}

type mfaPostHarness struct {
	repo     *mfaPostRepo
	webauthn *mfaPostWebAuthn
	sessions *mfaPostSessions
	tokens   *mfaPostTokens
	records  *postByIDRecords
	outbox   *postByIDOutbox
	router   chi.Router
}

func newMfaPostHarness(userOrg string, credentialStatus webauthndomain.CredentialStatus) *mfaPostHarness {
	h := &mfaPostHarness{
		repo: &mfaPostRepo{user: postByIDTargetUser(userOrg)},
		webauthn: &mfaPostWebAuthn{cred: &webauthndomain.Credential{
			ID:           "credential-row-1",
			UserID:       postByIDUserID,
			CredentialID: append([]byte(nil), mfaPostCredentialBytes...),
			Status:       credentialStatus,
		}},
		sessions: &mfaPostSessions{},
		tokens:   &mfaPostTokens{},
		records:  &postByIDRecords{},
		outbox:   &postByIDOutbox{},
	}
	uow := &mfaPostUnitOfWork{
		users: h.repo, webauthn: h.webauthn, sessions: h.sessions,
		tokens: h.tokens, records: h.records, outbox: h.outbox,
	}
	h.router = chi.NewRouter()
	RegisterChiRoutes(h.router, Handler{Service: appusers.Service{
		Clock: postByIDClock{}, UUID: postByIDUUID{}, UOW: uow,
	}})
	return h
}

// mfaPostRoute pins one batch-2 endpoint. wantJSON is the exact pre-change
// response body, so the success shape cannot drift while binding the tenant.
type mfaPostRoute struct {
	name              string
	path              string
	body              string
	initialStatus     webauthndomain.CredentialStatus
	wantJSON          string
	wantUpdate        int
	wantProfile       int
	wantReplaceMfa    int
	wantStatusCalls   int
	wantStatus        webauthndomain.CredentialStatus
	wantSessionRevoke int
	wantTokenRefresh  int
	wantTokenRevoke   int
	wantAudit         int
}

var mfaPostRoutes = []mfaPostRoute{
	{
		name:          "mfa-setup-verify",
		path:          "/api/v1/users/" + postByIDUserID + "/mfa-setup/verify",
		body:          `{"mfaType":"totp","passcode":"282760","secret":"JBSWY3DPEHPK3PXP"}`,
		wantJSON:      `{"status":"verified","msg":"resource verified"}`,
		initialStatus: webauthndomain.CredentialStatusActive,
	},
	{
		name:           "mfa-setup-enable",
		path:           "/api/v1/users/" + postByIDUserID + "/mfa-setup/enable",
		body:           `{"mfaType":"totp","secret":"JBSWY3DPEHPK3PXP","passcode":"282760","recoveryCodes":["rc-1"]}`,
		wantJSON:       `{"status":"enabled","msg":"resource enabled"}`,
		wantUpdate:     1,
		wantReplaceMfa: 1,
		wantAudit:      1,
		initialStatus:  webauthndomain.CredentialStatusActive,
	},
	{
		name:              "webauthn-revoke",
		path:              "/api/v1/users/" + postByIDUserID + "/webauthn-credentials/" + mfaPostCredentialID + "/revoke",
		initialStatus:     webauthndomain.CredentialStatusActive,
		wantJSON:          `{"status":"revoked","msg":"credential status updated"}`,
		wantStatusCalls:   1,
		wantStatus:        webauthndomain.CredentialStatusRevoked,
		wantSessionRevoke: 1,
		wantTokenRefresh:  1,
		wantTokenRevoke:   1,
		wantAudit:         1,
	},
	{
		name:              "webauthn-deactivate",
		path:              "/api/v1/users/" + postByIDUserID + "/webauthn-credentials/" + mfaPostCredentialID + "/deactivate",
		initialStatus:     webauthndomain.CredentialStatusActive,
		wantJSON:          `{"status":"deactivated","msg":"credential status updated"}`,
		wantStatusCalls:   1,
		wantStatus:        webauthndomain.CredentialStatusDeactivated,
		wantSessionRevoke: 1,
		wantTokenRefresh:  1,
		wantTokenRevoke:   1,
		wantAudit:         1,
	},
	{
		name:            "webauthn-restore",
		path:            "/api/v1/users/" + postByIDUserID + "/webauthn-credentials/" + mfaPostCredentialID + "/restore",
		initialStatus:   webauthndomain.CredentialStatusDeactivated,
		wantJSON:        `{"status":"active","msg":"credential status updated"}`,
		wantStatusCalls: 1,
		wantStatus:      webauthndomain.CredentialStatusActive,
		wantAudit:       1,
	},
}

func assertMfaPostNoWrites(t *testing.T, h *mfaPostHarness) {
	t.Helper()
	assert.Zero(t, h.repo.updateCalls)
	assert.Zero(t, h.repo.updateProfileCalls)
	assert.Zero(t, h.repo.replaceMfaCalls)
	assert.Zero(t, h.webauthn.updateStatusCalls)
	assert.Zero(t, h.sessions.revokeByUserCalls)
	assert.Zero(t, h.tokens.refreshCalls)
	assert.Zero(t, h.tokens.revokeCalls)
	assert.Empty(t, h.records.created)
	assert.Empty(t, h.outbox.enqueued)
}

// TestUserMfaPostByID_ForeignTenantRejected pins that all five batch-2 POST
// routes resolve the target user through the trusted tenant. A foreign user is
// 404, the credential is never read, and no write/audit side effect runs.
func TestUserMfaPostByID_ForeignTenantRejected(t *testing.T) {
	for _, tt := range mfaPostRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newMfaPostHarness(forgedOrgRequestID, tt.initialStatus)

			recorder := postByIDServe(h.router, postByIDRequest(http.MethodPost,
				tt.path+"?organizationId="+forgedOrgRequestID, tt.body, postByIDCallerID))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign user must look missing")
			assert.NotContains(t, recorder.Body.String(), "alice@example.com")
			assert.Zero(t, h.repo.getByIDCalls, "batch-2 routes must never use the system-scoped read")
			assert.NotEmpty(t, h.repo.scopedOrgs, "ownership must be resolved through the scoped lookup")
			for _, org := range h.repo.scopedOrgs {
				assert.Equal(t, forgedOrgSessionID, org, "forged query organizationId must be ignored")
			}
			assert.Zero(t, h.webauthn.getCalls, "the credential must not be read before parent ownership is proven")
			assertMfaPostNoWrites(t, h)
		})
	}
}

// TestUserMfaPostByID_WithoutTenantScopeRejected pins the fail-fast: no trusted
// organization means 400 before the repository is touched.
func TestUserMfaPostByID_WithoutTenantScopeRejected(t *testing.T) {
	for _, tt := range mfaPostRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newMfaPostHarness(forgedOrgSessionID, tt.initialStatus)

			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(http.MethodPost, tt.path, nil)
			}
			recorder := postByIDServe(h.router, req)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, h.repo.getByIDCalls)
			assert.Zero(t, h.repo.scopedGetCalls)
			assert.Zero(t, h.webauthn.getCalls)
			assertMfaPostNoWrites(t, h)
		})
	}
}

// TestUserMfaPostByID_OwnTenantSucceeds pins the unchanged 200 body and the
// exact write/audit counts inside the authenticated tenant, while the query
// tries to name another tenant.
func currentTotpCode() string {
	code, err := totp.GenerateCode("JBSWY3DPEHPK3PXP", time.Now().UTC())
	if err != nil {
		return ""
	}
	return code
}

func TestUserMfaPostByID_OwnTenantSucceeds(t *testing.T) {
	for _, tt := range mfaPostRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newMfaPostHarness(forgedOrgSessionID, tt.initialStatus)

			// The verify endpoint checks against wall-clock time, so the code
			// must be derived at request time instead of a static fixture.
			body := tt.body
			if tt.name == "mfa-setup-verify" {
				body = strings.Replace(body, "282760", currentTotpCode(), 1)
			}

			recorder := postByIDServe(h.router, postByIDRequest(http.MethodPost,
				tt.path+"?organizationId="+forgedOrgRequestID, body, postByIDCallerID))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.JSONEq(t, tt.wantJSON, recorder.Body.String(), "success response shape must not change")
			assert.Zero(t, h.repo.getByIDCalls, "management writes must use the scoped lookup")
			require.NotEmpty(t, h.repo.scopedOrgs)
			for _, org := range h.repo.scopedOrgs {
				assert.Equal(t, forgedOrgSessionID, org, "forged query organizationId must be ignored")
			}

			assert.Equal(t, tt.wantUpdate, h.repo.updateCalls)
			assert.Equal(t, tt.wantProfile, h.repo.updateProfileCalls)
			assert.Equal(t, tt.wantReplaceMfa, h.repo.replaceMfaCalls)
			assert.Equal(t, tt.wantStatusCalls, h.webauthn.updateStatusCalls)
			if tt.wantStatusCalls > 0 {
				assert.Equal(t, tt.wantStatus, h.webauthn.lastStatus)
			}
			assert.Equal(t, tt.wantSessionRevoke, h.sessions.revokeByUserCalls)
			assert.Equal(t, tt.wantTokenRefresh, h.tokens.refreshCalls)
			assert.Equal(t, tt.wantTokenRevoke, h.tokens.revokeCalls)

			require.Len(t, h.records.created, tt.wantAudit)
			require.Len(t, h.outbox.enqueued, tt.wantAudit)
			if tt.wantAudit == 1 {
				assert.Equal(t, forgedOrgSessionID, h.records.created[0].OrganizationID,
					"audit must carry the entity organization, never the built-in tenant")
			}
		})
	}
}

// TestUserMfaPostByID_ForeignCredentialRejected pins that the parent scoped
// check is not enough: a credential that belongs to another user in the same
// tenant stays invisible (404) and is never updated.
func TestUserMfaPostByID_ForeignCredentialRejected(t *testing.T) {
	h := newMfaPostHarness(forgedOrgSessionID, webauthndomain.CredentialStatusActive)
	h.webauthn.cred.UserID = postByIDOtherUserID

	recorder := postByIDServe(h.router, postByIDRequest(http.MethodPost,
		"/api/v1/users/"+postByIDUserID+"/webauthn-credentials/"+mfaPostCredentialID+"/revoke",
		"", postByIDCallerID))

	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Zero(t, h.webauthn.updateStatusCalls)
	assert.Empty(t, h.records.created)
	assert.Empty(t, h.outbox.enqueued)
}

// Guard compile-time drift: mfaPostUnitOfWork must keep satisfying UnitOfWork.
var _ appuow.UnitOfWork = (*mfaPostUnitOfWork)(nil)
