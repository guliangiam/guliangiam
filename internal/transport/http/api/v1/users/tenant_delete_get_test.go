package users

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appsessions "github.com/guliangiam/guliangiam/internal/app/sessions"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	transactiondomain "github.com/guliangiam/guliangiam/internal/domain/transactions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	delGetCredentialIDB64 = "Y3JlZC0x" // base64url("cred-1")
	delGetCredentialRowID = "credential-row-1"
)

var delGetCredentialBytes = []byte("cred-1")

// delGetUserRepo models a system-scoped read that returns any row; the scoped
// read records the organization it was asked for.
type delGetUserRepo struct {
	userdomain.Repository
	user           *userdomain.User
	getByIDCalls   int
	scopedGetCalls int
	scopedOrgs     []string
}

func (r *delGetUserRepo) GetByID(_ context.Context, id string) (*userdomain.User, error) {
	r.getByIDCalls++
	if r.user == nil || r.user.ID != id {
		return nil, apperrors.NewNotFound("user not found")
	}
	return r.user, nil
}

func (r *delGetUserRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*userdomain.User, error) {
	r.scopedGetCalls++
	r.scopedOrgs = append(r.scopedOrgs, organizationID)
	if r.user == nil || r.user.ID != id || r.user.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("user not found")
	}
	return r.user, nil
}

type delGetWebAuthn struct {
	webauthndomain.Repository
	credentials []*webauthndomain.Credential
	getCalls    int
	listCalls   int
	deleteCalls int
	deletedIDs  []string
}

func (r *delGetWebAuthn) GetByCredentialID(_ context.Context, credentialID []byte) (*webauthndomain.Credential, error) {
	r.getCalls++
	for _, cred := range r.credentials {
		if bytes.Equal(cred.CredentialID, credentialID) {
			return cred, nil
		}
	}
	return nil, apperrors.NewNotFound("credential not found")
}

func (r *delGetWebAuthn) ListByUserID(_ context.Context, userID string) ([]*webauthndomain.Credential, error) {
	r.listCalls++
	var result []*webauthndomain.Credential
	for _, cred := range r.credentials {
		if cred.UserID == userID {
			result = append(result, cred)
		}
	}
	return result, nil
}

func (r *delGetWebAuthn) Delete(_ context.Context, id string) error {
	r.deleteCalls++
	r.deletedIDs = append(r.deletedIDs, id)
	return nil
}

type delGetSessions struct {
	sessiondomain.Repository
	listCalls         int
	listQuery         sessiondomain.ListQuery
	revokeByUserCalls int
}

func (r *delGetSessions) List(_ context.Context, query sessiondomain.ListQuery) ([]*sessiondomain.Session, int64, error) {
	r.listCalls++
	r.listQuery = query
	return []*sessiondomain.Session{{
		ID: "session-1", UserID: postByIDUserID, OrganizationID: forgedOrgSessionID,
		Username: "target-user", SessionState: "active",
		CreatedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(0, 0).UTC(),
	}}, 1, nil
}

func (r *delGetSessions) RevokeByUser(context.Context, string, time.Time) (int64, error) {
	r.revokeByUserCalls++
	return 1, nil
}

type delGetTransactions struct {
	transactiondomain.Repository
	listCalls int
	listQuery transactiondomain.ListQuery
}

func (r *delGetTransactions) List(_ context.Context, query transactiondomain.ListQuery) ([]*transactiondomain.Transaction, int64, error) {
	r.listCalls++
	r.listQuery = query
	return []*transactiondomain.Transaction{{
		ID: "txn-1", UserID: postByIDUserID, OrganizationID: forgedOrgSessionID,
		TransactionType: "recharge", Amount: 10, Currency: "CNY", Status: "completed",
		CreatedAt: time.Unix(0, 0).UTC(),
	}}, 1, nil
}

type delGetTokens struct {
	tokendomain.Repository
	refreshCalls int
	revokeCalls  int
}

func (r *delGetTokens) RevokeRefreshByUser(context.Context, string, time.Time) (int64, error) {
	r.refreshCalls++
	return 1, nil
}

func (r *delGetTokens) RevokeByQuery(context.Context, tokendomain.RevokeQuery, time.Time) (int64, error) {
	r.revokeCalls++
	return 1, nil
}

type delGetUnitOfWork struct {
	appuow.UnitOfWork
	users    *delGetUserRepo
	webauthn *delGetWebAuthn
	sessions *delGetSessions
	txns     *delGetTransactions
	tokens   *delGetTokens
	records  *postByIDRecords
	outbox   *postByIDOutbox
}

func (u *delGetUnitOfWork) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func (u *delGetUnitOfWork) Users() userdomain.Repository                   { return u.users }
func (u *delGetUnitOfWork) WebAuthnCredentials() webauthndomain.Repository { return u.webauthn }
func (u *delGetUnitOfWork) Sessions() sessiondomain.Repository             { return u.sessions }
func (u *delGetUnitOfWork) Transactions() transactiondomain.Repository     { return u.txns }
func (u *delGetUnitOfWork) Tokens() tokendomain.Repository                 { return u.tokens }
func (u *delGetUnitOfWork) Records() recordsdomain.Repository              { return u.records }
func (u *delGetUnitOfWork) Outbox() domainoutbox.Repository                { return u.outbox }

type delGetHarness struct {
	repo     *delGetUserRepo
	webauthn *delGetWebAuthn
	sessions *delGetSessions
	txns     *delGetTransactions
	tokens   *delGetTokens
	records  *postByIDRecords
	outbox   *postByIDOutbox
	router   chi.Router
}

func newDelGetHarness(ownerOrg string) *delGetHarness {
	user := postByIDTargetUser(ownerOrg)
	user.ApplicationScopes = []userdomain.ConsentRecord{{
		Application: "app-1", Scopes: []string{"read", "write"},
	}}
	h := &delGetHarness{
		repo: &delGetUserRepo{user: user},
		webauthn: &delGetWebAuthn{credentials: []*webauthndomain.Credential{{
			ID: delGetCredentialRowID, UserID: postByIDUserID,
			CredentialID: append([]byte(nil), delGetCredentialBytes...),
			Name:         "security-key", Status: webauthndomain.CredentialStatusActive,
		}}},
		sessions: &delGetSessions{},
		txns:     &delGetTransactions{},
		tokens:   &delGetTokens{},
		records:  &postByIDRecords{},
		outbox:   &postByIDOutbox{},
	}
	uow := &delGetUnitOfWork{
		users: h.repo, webauthn: h.webauthn, sessions: h.sessions,
		txns: h.txns, tokens: h.tokens, records: h.records, outbox: h.outbox,
	}
	h.router = chi.NewRouter()
	RegisterChiRoutes(h.router, Handler{
		Service: appusers.Service{
			Clock: postByIDClock{}, UUID: postByIDUUID{}, UOW: uow,
		},
		SessionService: appsessions.Service{Clock: postByIDClock{}, UOW: uow},
	})
	return h
}

var delGetReadRoutes = []struct {
	name string
	path string
}{
	{"webauthn-credentials", "/api/v1/users/" + postByIDUserID + "/webauthn-credentials"},
	{"sessions", "/api/v1/users/" + postByIDUserID + "/sessions"},
	{"transactions", "/api/v1/users/" + postByIDUserID + "/transactions"},
	{"consents", "/api/v1/users/" + postByIDUserID + "/consents"},
}

func assertDelGetNoChildReads(t *testing.T, h *delGetHarness) {
	t.Helper()
	assert.Zero(t, h.webauthn.getCalls)
	assert.Zero(t, h.webauthn.listCalls)
	assert.Zero(t, h.sessions.listCalls)
	assert.Zero(t, h.txns.listCalls)
}

func assertDelGetNoWrites(t *testing.T, h *delGetHarness) {
	t.Helper()
	assert.Zero(t, h.webauthn.deleteCalls)
	assert.Zero(t, h.sessions.revokeByUserCalls)
	assert.Zero(t, h.tokens.refreshCalls)
	assert.Zero(t, h.tokens.revokeCalls)
	assert.Empty(t, h.records.created)
	assert.Empty(t, h.outbox.enqueued)
}

// TestUserReadByID_ForeignTenantRejected pins that every GET sub-resource
// resolves the parent user through the trusted tenant. A foreign user is 404
// and no child query runs.
func TestUserReadByID_ForeignTenantRejected(t *testing.T) {
	for _, tt := range delGetReadRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newDelGetHarness(forgedOrgRequestID)

			recorder := postByIDServe(h.router, postByIDRequest(http.MethodGet,
				tt.path+"?organizationId="+forgedOrgRequestID, "", postByIDCallerID))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign user must look missing")
			assert.NotContains(t, recorder.Body.String(), "alice@example.com")
			assert.Zero(t, h.repo.getByIDCalls, "read routes must never use the system-scoped lookup")
			require.NotEmpty(t, h.repo.scopedOrgs, "ownership must be resolved through the scoped lookup")
			for _, org := range h.repo.scopedOrgs {
				assert.Equal(t, forgedOrgSessionID, org, "forged query organizationId must be ignored")
			}
			assertDelGetNoChildReads(t, h)
		})
	}
}

// TestUserReadByID_WithoutTenantScopeRejected pins the fail-fast: an empty
// trusted tenant is a 400 before any repository read.
func TestUserReadByID_WithoutTenantScopeRejected(t *testing.T) {
	for _, tt := range delGetReadRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newDelGetHarness(forgedOrgSessionID)

			recorder := byIDServe(h.router, httptest.NewRequest(http.MethodGet, tt.path, nil))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, h.repo.getByIDCalls)
			assert.Zero(t, h.repo.scopedGetCalls)
			assertDelGetNoChildReads(t, h)
		})
	}
}

// TestUserReadByID_OwnTenantSucceeds pins that the four GET sub-resources still
// return their payload inside the authenticated tenant while the query tries to
// name another tenant.
func TestUserReadByID_OwnTenantSucceeds(t *testing.T) {
	for _, tt := range delGetReadRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newDelGetHarness(forgedOrgSessionID)

			recorder := postByIDServe(h.router, postByIDRequest(http.MethodGet,
				tt.path+"?organizationId="+forgedOrgRequestID, "", postByIDCallerID))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Zero(t, h.repo.getByIDCalls, "management reads must use the scoped lookup")
			require.NotEmpty(t, h.repo.scopedOrgs)
			for _, org := range h.repo.scopedOrgs {
				assert.Equal(t, forgedOrgSessionID, org, "forged query organizationId must be ignored")
			}

			switch tt.name {
			case "webauthn-credentials":
				assert.Equal(t, 1, h.webauthn.listCalls)
				assert.Contains(t, recorder.Body.String(), delGetCredentialRowID)
			case "sessions":
				assert.Equal(t, 1, h.sessions.listCalls)
				assert.Equal(t, forgedOrgSessionID, h.sessions.listQuery.OrganizationID)
				assert.Equal(t, postByIDUserID, h.sessions.listQuery.UserID)
			case "transactions":
				assert.Equal(t, 1, h.txns.listCalls)
				assert.Equal(t, forgedOrgSessionID, h.txns.listQuery.OrganizationID)
				assert.Equal(t, postByIDUserID, h.txns.listQuery.UserID)
			case "consents":
				assert.Contains(t, recorder.Body.String(), `"applicationId":"app-1"`)
			}
		})
	}
}

// TestUserReadByID_PlatformAdminCanTargetTenant pins the documented exception:
// a platform scope keeps the explicit cross-tenant target as the predicate.
func TestUserReadByID_PlatformAdminCanTargetTenant(t *testing.T) {
	for _, tt := range delGetReadRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newDelGetHarness(forgedOrgRequestID)

			recorder := byIDServe(h.router, byIDSuperAdminRequest(http.MethodGet,
				tt.path+"?organizationId="+forgedOrgRequestID))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.NotEmpty(t, h.repo.scopedOrgs)
			assert.Equal(t, forgedOrgRequestID, h.repo.scopedOrgs[0],
				"platform admin's explicit organization is the predicate")
		})
	}
}

// TestUserCredentialDelete_ForeignTenantRejected pins that the parent user is
// resolved through the trusted tenant before the credential row is touched.
func TestUserCredentialDelete_ForeignTenantRejected(t *testing.T) {
	h := newDelGetHarness(forgedOrgRequestID)

	recorder := postByIDServe(h.router, postByIDRequest(http.MethodDelete,
		delGetCredentialPath()+"?organizationId="+forgedOrgRequestID, "", postByIDCallerID))

	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Zero(t, h.repo.getByIDCalls)
	require.NotEmpty(t, h.repo.scopedOrgs)
	assert.Equal(t, forgedOrgSessionID, h.repo.scopedOrgs[0], "forged query organizationId must be ignored")
	assert.Zero(t, h.webauthn.getCalls, "the credential must not be read before parent ownership is proven")
	assertDelGetNoWrites(t, h)
}

// TestUserCredentialDelete_WithoutTenantScopeRejected pins the fail-fast.
func TestUserCredentialDelete_WithoutTenantScopeRejected(t *testing.T) {
	h := newDelGetHarness(forgedOrgSessionID)

	recorder := byIDServe(h.router, httptest.NewRequest(http.MethodDelete, delGetCredentialPath(), nil))

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Zero(t, h.repo.getByIDCalls)
	assert.Zero(t, h.repo.scopedGetCalls)
	assert.Zero(t, h.webauthn.getCalls)
	assertDelGetNoWrites(t, h)
}

// TestUserCredentialDelete_OwnTenantSucceeds pins the success path, the delete,
// and that the audit event carries the parent user's real organization.
func TestUserCredentialDelete_OwnTenantSucceeds(t *testing.T) {
	h := newDelGetHarness(forgedOrgSessionID)

	recorder := postByIDServe(h.router, postByIDRequest(http.MethodDelete,
		delGetCredentialPath()+"?organizationId="+forgedOrgRequestID, "", postByIDCallerID))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.JSONEq(t, `{"status":"deleted","msg":"resource deleted"}`, recorder.Body.String())
	assert.Zero(t, h.repo.getByIDCalls)
	require.NotEmpty(t, h.repo.scopedOrgs)
	assert.Equal(t, forgedOrgSessionID, h.repo.scopedOrgs[0], "forged query organizationId must be ignored")
	assert.Equal(t, 1, h.webauthn.getCalls)
	assert.Equal(t, 1, h.webauthn.deleteCalls)
	assert.Equal(t, []string{delGetCredentialRowID}, h.webauthn.deletedIDs)
	assert.Equal(t, 1, h.sessions.revokeByUserCalls, "deletion cascades sessions")
	assert.Equal(t, 1, h.tokens.refreshCalls, "deletion cascades refresh tokens")
	assert.Equal(t, 1, h.tokens.revokeCalls, "deletion cascades access tokens")
	require.Len(t, h.records.created, 1)
	require.Len(t, h.outbox.enqueued, 1)
	assert.Equal(t, forgedOrgSessionID, h.records.created[0].OrganizationID,
		"audit must carry the entity organization, never the built-in tenant")
}

// TestUserCredentialDelete_ForeignCredentialRejected pins the ownership check:
// a credential bound to another user in the same tenant stays invisible.
func TestUserCredentialDelete_ForeignCredentialRejected(t *testing.T) {
	h := newDelGetHarness(forgedOrgSessionID)
	h.webauthn.credentials[0].UserID = postByIDOtherUserID

	recorder := postByIDServe(h.router, postByIDRequest(http.MethodDelete,
		delGetCredentialPath(), "", postByIDCallerID))

	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Equal(t, 1, h.webauthn.getCalls)
	assert.Zero(t, h.webauthn.deleteCalls)
	assertDelGetNoWrites(t, h)
}

func delGetCredentialPath() string {
	return "/api/v1/users/" + postByIDUserID + "/webauthn-credentials/" + delGetCredentialIDB64
}

// Guard compile-time drift: both services must keep accepting delGetUnitOfWork.
var (
	_ appuow.UnitOfWork = (*delGetUnitOfWork)(nil)
	_ appuow.AuthUoW    = (*delGetUnitOfWork)(nil)
)
