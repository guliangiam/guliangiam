package auth

import (
	"context"
	"testing"
	"time"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/domain/applications"
	consentdomain "github.com/guliangiam/guliangiam/internal/domain/consents"
	deviceauthcodesdomain "github.com/guliangiam/guliangiam/internal/domain/deviceauthcodes"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	rebac "github.com/guliangiam/guliangiam/internal/domain/rebac"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ---------------------------------------------------------------------------
// Extended test UoW with Consent and Token repos
// ---------------------------------------------------------------------------

// serviceTestUnitOfWork extends loginTestUnitOfWork with consent and token repos.
type serviceTestUnitOfWork struct {
	loginTestUnitOfWork
	consentRepo *testConsentRepository
	tokenRepo   *testTokenRepository
	deviceRepo  *testDeviceAuthCodeRepository
	// sessionRepo overrides the embedded repository. The embedded one answers
	// every lookup with NotFound, which is what most tests want; a test that needs
	// to resolve a session (the sid lookup) supplies its own.
	sessionRepo sessiondomain.Repository
}

func (u serviceTestUnitOfWork) Outbox() domainoutbox.Repository {
	if u.outboxRepo == nil {
		return nil
	}
	return u.outboxRepo
}
func (u serviceTestUnitOfWork) Sessions() sessiondomain.Repository {
	if u.sessionRepo != nil {
		return u.sessionRepo
	}
	return u.loginTestUnitOfWork.Sessions()
}

func (u serviceTestUnitOfWork) Consents() consentdomain.Repository {
	return u.consentRepo
}

func (u serviceTestUnitOfWork) Tokens() tokendomain.Repository {
	return u.tokenRepo
}

func (u serviceTestUnitOfWork) DeviceAuthCodes() deviceauthcodesdomain.Repository {
	if u.deviceRepo == nil {
		return nil
	}
	return u.deviceRepo
}

// All other repo methods are inherited from loginTestUnitOfWork.
// We need to re-delegate WithinTransaction to include the extended repos.

func (u serviceTestUnitOfWork) Rebac() rebac.Repository { return nil }
func (u serviceTestUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

// testConsentRepository is an in-memory consent repository for tests.
type testConsentRepository struct {
	consents map[string]*consentdomain.Consent // keyed by "userID:appID"
}

func consentKey(userID, appID string) string {
	return userID + ":" + appID
}

func (r *testConsentRepository) GetByUserAndApplication(ctx context.Context, userID, applicationID string) (*consentdomain.Consent, error) {
	if r.consents == nil {
		return nil, apperrors.NewNotFound("consent not found")
	}
	c, ok := r.consents[consentKey(userID, applicationID)]
	if !ok {
		return nil, apperrors.NewNotFound("consent not found")
	}
	return c, nil
}

func (r *testConsentRepository) CountActiveByApplication(ctx context.Context, applicationID string) (int64, error) {
	return 0, nil
}

func (r *testConsentRepository) Upsert(ctx context.Context, consent *consentdomain.Consent) error {
	if r.consents == nil {
		r.consents = make(map[string]*consentdomain.Consent)
	}
	r.consents[consentKey(consent.UserID, consent.ApplicationID)] = consent
	return nil
}

func (r *testConsentRepository) Revoke(ctx context.Context, userID, applicationID string) error {
	if r.consents != nil {
		delete(r.consents, consentKey(userID, applicationID))
	}
	return nil
}

// testTokenRepository is an in-memory token repository for tests.
type testTokenRepository struct {
	tokens         map[string]*tokendomain.Token        // keyed by id
	tokensByHash   map[string]*tokendomain.Token        // keyed by tokenHash
	refreshByHash  map[string]*tokendomain.RefreshToken // keyed by tokenHash
	createdRefresh []*tokendomain.RefreshToken          // recorded CreateRefresh calls
	revokeQueries  []tokendomain.RevokeQuery            // recorded RevokeByQuery criteria
}

func (r *testTokenRepository) GetByID(ctx context.Context, id string) (*tokendomain.Token, error) {
	if r.tokens == nil {
		return nil, apperrors.NewNotFound("token not found")
	}
	t, ok := r.tokens[id]
	if !ok {
		return nil, apperrors.NewNotFound("token not found")
	}
	return t, nil
}

func (r *testTokenRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*tokendomain.Token, error) {
	t, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if t.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("token not found")
	}
	return t, nil
}

func (r *testTokenRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*tokendomain.Token, error) {
	if r.tokensByHash == nil {
		return nil, apperrors.NewNotFound("token not found")
	}
	t, ok := r.tokensByHash[tokenHash]
	if !ok {
		return nil, apperrors.NewNotFound("token not found")
	}
	return t, nil
}

func (r *testTokenRepository) GetByDeviceSecretHash(ctx context.Context, deviceSecretHash string) (*tokendomain.Token, error) {
	return nil, apperrors.NewNotFound("token not found")
}

func (r *testTokenRepository) GetRefreshByTokenHash(ctx context.Context, tokenHash string) (*tokendomain.RefreshToken, error) {
	if r.refreshByHash == nil {
		return nil, apperrors.NewNotFound("refresh token not found")
	}
	rt, ok := r.refreshByHash[tokenHash]
	if !ok {
		return nil, apperrors.NewNotFound("refresh token not found")
	}
	return rt, nil
}

func (r *testTokenRepository) List(ctx context.Context, query tokendomain.ListQuery) ([]*tokendomain.Token, int64, error) {
	return nil, 0, nil
}

func (r *testTokenRepository) Create(ctx context.Context, token *tokendomain.Token) error {
	if r.tokens == nil {
		r.tokens = make(map[string]*tokendomain.Token)
	}
	r.tokens[token.ID] = token
	return nil
}

func (r *testTokenRepository) UpdateDeviceSecret(ctx context.Context, tokenID, deviceSecretHash string, expiresIn int) error {
	if r.tokens != nil {
		if t, ok := r.tokens[tokenID]; ok {
			t.DeviceSecretHash = deviceSecretHash
			t.DeviceSecretExpiresIn = expiresIn
		}
	}
	return nil
}

func (r *testTokenRepository) CreateRefresh(ctx context.Context, token *tokendomain.RefreshToken) error {
	r.createdRefresh = append(r.createdRefresh, token)
	return nil
}

func (r *testTokenRepository) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	if r.tokens != nil {
		if t, ok := r.tokens[id]; ok {
			t.RevokedAt = &revokedAt
		}
	}
	return nil
}

func (r *testTokenRepository) RevokeRefresh(ctx context.Context, id string, revokedAt time.Time) error {
	return nil
}

// RevokeByQuery records the criteria and applies the same predicates the SQL
// statements apply, so a test can assert both the filter that was sent and the
// tokens that actually survived.
func (r *testTokenRepository) RevokeByQuery(ctx context.Context, query tokendomain.RevokeQuery, revokedAt time.Time) (int64, error) {
	r.revokeQueries = append(r.revokeQueries, query)
	var revoked int64
	for _, token := range r.tokens {
		if query.OrganizationID != "" && token.OrganizationID != query.OrganizationID {
			continue
		}
		if query.UserID != "" && token.UserID != query.UserID {
			continue
		}
		if query.ApplicationID != "" && token.ApplicationID != query.ApplicationID {
			continue
		}
		if query.TokenPrefix != "" && token.TokenPrefix != query.TokenPrefix {
			continue
		}
		if token.RevokedAt == nil {
			at := revokedAt
			token.RevokedAt = &at
			revoked++
		}
	}
	return revoked, nil
}

func (r *testTokenRepository) RevokeByFamilyID(ctx context.Context, familyID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}

func (r *testTokenRepository) RevokeRefreshByUser(ctx context.Context, userID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}

func (r *testTokenRepository) RevokeByAuthorizationCodeID(ctx context.Context, authorizationCodeID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}

func (r *testTokenRepository) Delete(ctx context.Context, id string) error {
	if r.tokens != nil {
		delete(r.tokens, id)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Test helpers for creating extended UoW
// ---------------------------------------------------------------------------

func newServiceTestUow(org *orgdomain.Organization, user *userdomain.User, app *applications.Application) serviceTestUnitOfWork {
	base := newTestUow(org, user, app)
	return serviceTestUnitOfWork{
		loginTestUnitOfWork: base,
		consentRepo:         &testConsentRepository{},
		tokenRepo:           &testTokenRepository{},
	}
}

func newServiceTestService(uow serviceTestUnitOfWork) Service {
	return Service{
		Clock:         fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "019test-0000-0000-0000-000000000001"},
		UOW:           uow,
	}
}

// ---------------------------------------------------------------------------
// GetMaskedContact tests (contact.go)
// ---------------------------------------------------------------------------

func TestGetMaskedContact_SuccessWithEmail(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	result, err := svc.GetMaskedContact(context.Background(), testOrgSlug, testUsername)
	if err != nil {
		t.Fatalf("GetMaskedContact() error = %v, want nil", err)
	}
	if result.MaskedEmail == "" {
		t.Error("expected masked email to be non-empty")
	}
	// Email is testuser@example.com, should be masked as t*******@example.com
	if result.MaskedEmail != "t*******@example.com" {
		t.Errorf("MaskedEmail = %q, want t*******@example.com", result.MaskedEmail)
	}
}

func TestGetMaskedContact_SuccessWithPhone(t *testing.T) {
	org := newTestOrg()
	user := newTestUser(func(u *userdomain.User) {
		u.Profile.Phone = "13812345678"
	})
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	result, err := svc.GetMaskedContact(context.Background(), testOrgSlug, testUsername)
	if err != nil {
		t.Fatalf("GetMaskedContact() error = %v, want nil", err)
	}
	if result.MaskedPhone == "" {
		t.Error("expected masked phone to be non-empty")
	}
	if result.MaskedPhone != "138****5678" {
		t.Errorf("MaskedPhone = %q, want 138****5678", result.MaskedPhone)
	}
}

func TestGetMaskedContact_EmptyOrgSlug(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	_, err := svc.GetMaskedContact(context.Background(), "", testUsername)
	if err == nil {
		t.Fatal("expected error for empty org slug")
	}
}

func TestGetMaskedContact_EmptyUsername(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	_, err := svc.GetMaskedContact(context.Background(), testOrgSlug, "")
	if err == nil {
		t.Fatal("expected error for empty username")
	}
}

func TestGetMaskedContact_OrgNotFound(t *testing.T) {
	// Use an org that won't be found
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	// Request a non-existent org
	result, err := svc.GetMaskedContact(context.Background(), "nonexistent-org", testUsername)
	if err != nil {
		t.Fatalf("GetMaskedContact() error = %v, want nil (should not leak org existence)", err)
	}
	// Should return empty result to prevent enumeration
	if result.MaskedEmail != "" || result.MaskedPhone != "" {
		t.Error("expected empty masked contact for non-existent org")
	}
}

func TestGetMaskedContact_UserNotFound(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	// Request a non-existent user
	result, err := svc.GetMaskedContact(context.Background(), testOrgSlug, "nonexistent-user")
	if err != nil {
		t.Fatalf("GetMaskedContact() error = %v, want nil (should not leak user existence)", err)
	}
	if result.MaskedEmail != "" || result.MaskedPhone != "" {
		t.Error("expected empty masked contact for non-existent user")
	}
}

// ---------------------------------------------------------------------------
// GrantConsent tests (consent.go)
// ---------------------------------------------------------------------------

func TestGrantConsent_NewConsent(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp(func(a *applications.Application) {
		a.CustomScopes = []applications.ApplicationCustomScope{
			{Scope: "read", DisplayName: "Read", Description: "Read access"},
			{Scope: "write", DisplayName: "Write", Description: "Write access"},
		}
	})
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	result, err := svc.GrantConsent(context.Background(), GrantConsentCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		ApplicationID:    testAppID,
		GrantedScopes:    []string{"read", "write"},
	})
	if err != nil {
		t.Fatalf("GrantConsent() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("GrantConsent() result = nil, want non-nil")
	}
	if result.Status != "granted" {
		t.Errorf("Status = %q, want granted", result.Status)
	}
	if result.Consent.UserID != testUserID {
		t.Errorf("Consent.UserID = %q, want %q", result.Consent.UserID, testUserID)
	}
	if len(result.Consent.GrantedScopes) != 2 {
		t.Errorf("len(GrantedScopes) = %d, want 2", len(result.Consent.GrantedScopes))
	}
}

func TestGrantConsent_EmptyScopes(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp(func(a *applications.Application) {
		a.CustomScopes = []applications.ApplicationCustomScope{
			{Scope: "read", DisplayName: "Read", Description: "Read access"},
		}
	})
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	_, err := svc.GrantConsent(context.Background(), GrantConsentCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		ApplicationID:    testAppID,
		GrantedScopes:    []string{},
	})
	if err == nil {
		t.Fatal("expected error for empty scopes")
	}
}

func TestGrantConsent_InvalidScope(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp(func(a *applications.Application) {
		a.CustomScopes = []applications.ApplicationCustomScope{
			{Scope: "read", DisplayName: "Read", Description: "Read access"},
		}
	})
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	_, err := svc.GrantConsent(context.Background(), GrantConsentCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		ApplicationID:    testAppID,
		GrantedScopes:    []string{"admin"}, // not in custom scopes
	})
	if err == nil {
		t.Fatal("expected error for invalid scope")
	}
}

func TestGrantConsent_MissingOrgSlug(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	_, err := svc.GrantConsent(context.Background(), GrantConsentCommand{
		OrganizationSlug: "",
		Username:         testUsername,
		ApplicationID:    testAppID,
		GrantedScopes:    []string{"read"},
	})
	if err == nil {
		t.Fatal("expected error for missing org slug")
	}
}

func TestGrantConsent_MissingAppID(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	_, err := svc.GrantConsent(context.Background(), GrantConsentCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		ApplicationID:    "",
		GrantedScopes:    []string{"read"},
	})
	if err == nil {
		t.Fatal("expected error for missing app ID")
	}
}

// ---------------------------------------------------------------------------
// EvaluateConsent tests (consent.go)
// ---------------------------------------------------------------------------

func TestEvaluateConsent_NoExistingConsent(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp(func(a *applications.Application) {
		a.CustomScopes = []applications.ApplicationCustomScope{
			{Scope: "read", DisplayName: "Read", Description: "Read access"},
			{Scope: "write", DisplayName: "Write", Description: "Write access"},
		}
	})
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	result, err := svc.EvaluateConsent(context.Background(), EvaluateConsentCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		ApplicationID:    testAppID,
		RequestedScopes:  []string{"read", "write"},
	})
	if err != nil {
		t.Fatalf("EvaluateConsent() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("EvaluateConsent() result = nil")
	}
	if result.Status != "evaluated" {
		t.Errorf("Status = %q, want evaluated", result.Status)
	}
	// All scopes should be missing since no consent was granted
	if len(result.Evaluation.MissingScopes) != 2 {
		t.Errorf("len(MissingScopes) = %d, want 2", len(result.Evaluation.MissingScopes))
	}
	if !result.Evaluation.Required {
		t.Error("Expected Required = true when missing scopes exist")
	}
}

func TestEvaluateConsent_WithExistingConsent(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp(func(a *applications.Application) {
		a.CustomScopes = []applications.ApplicationCustomScope{
			{Scope: "read", DisplayName: "Read", Description: "Read access"},
			{Scope: "write", DisplayName: "Write", Description: "Write access"},
		}
	})
	uow := newServiceTestUow(org, user, app)

	// Pre-grant consent for "read"
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	uow.consentRepo.consents = map[string]*consentdomain.Consent{
		consentKey(testUserID, testAppID): {
			ID:            "consent-1",
			UserID:        testUserID,
			Username:      testUsername,
			ApplicationID: testAppID,
			GrantedScopes: []string{"read"},
			CreatedAt:     now,
			UpdatedAt:     now,
			Revision:      1,
		},
	}

	svc := newServiceTestService(uow)

	result, err := svc.EvaluateConsent(context.Background(), EvaluateConsentCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		ApplicationID:    testAppID,
		RequestedScopes:  []string{"read", "write"},
	})
	if err != nil {
		t.Fatalf("EvaluateConsent() error = %v, want nil", err)
	}
	// "read" is granted, "write" is missing
	if len(result.Evaluation.GrantedScopes) != 1 || result.Evaluation.GrantedScopes[0] != "read" {
		t.Errorf("GrantedScopes = %v, want [read]", result.Evaluation.GrantedScopes)
	}
	if len(result.Evaluation.MissingScopes) != 1 || result.Evaluation.MissingScopes[0] != "write" {
		t.Errorf("MissingScopes = %v, want [write]", result.Evaluation.MissingScopes)
	}
}

func TestEvaluateConsent_MissingOrgSlug(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	_, err := svc.EvaluateConsent(context.Background(), EvaluateConsentCommand{
		OrganizationSlug: "",
		Username:         testUsername,
		ApplicationID:    testAppID,
	})
	if err == nil {
		t.Fatal("expected error for missing org slug")
	}
}

// ---------------------------------------------------------------------------
// RevokeConsent tests (consent.go)
// ---------------------------------------------------------------------------

func TestRevokeConsent_PartialRevoke(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp(func(a *applications.Application) {
		a.CustomScopes = []applications.ApplicationCustomScope{
			{Scope: "read", DisplayName: "Read", Description: "Read access"},
			{Scope: "write", DisplayName: "Write", Description: "Write access"},
		}
	})
	uow := newServiceTestUow(org, user, app)

	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	uow.consentRepo.consents = map[string]*consentdomain.Consent{
		consentKey(testUserID, testAppID): {
			ID:               "consent-1",
			UserID:           testUserID,
			Username:         testUsername,
			OrganizationID:   testOrgID,
			OrganizationSlug: testOrgSlug,
			ApplicationID:    testAppID,
			ApplicationSlug:  testAppName,
			GrantedScopes:    []string{"read", "write"},
			CreatedAt:        now,
			UpdatedAt:        now,
			Revision:         1,
		},
	}

	svc := newServiceTestService(uow)

	result, err := svc.RevokeConsent(context.Background(), RevokeConsentCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		ApplicationID:    testAppID,
		GrantedScopes:    []string{"write"},
	})
	if err != nil {
		t.Fatalf("RevokeConsent() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("RevokeConsent() result = nil")
	}
	if result.Status != "revoked" {
		t.Errorf("Status = %q, want revoked", result.Status)
	}
	// "read" should still be granted
	if len(result.Consent.GrantedScopes) != 1 || result.Consent.GrantedScopes[0] != "read" {
		t.Errorf("GrantedScopes = %v, want [read]", result.Consent.GrantedScopes)
	}
}

func TestRevokeConsent_FullRevoke(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp(func(a *applications.Application) {
		a.CustomScopes = []applications.ApplicationCustomScope{
			{Scope: "read", DisplayName: "Read", Description: "Read access"},
		}
	})
	uow := newServiceTestUow(org, user, app)

	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	uow.consentRepo.consents = map[string]*consentdomain.Consent{
		consentKey(testUserID, testAppID): {
			ID:               "consent-1",
			UserID:           testUserID,
			Username:         testUsername,
			OrganizationID:   testOrgID,
			OrganizationSlug: testOrgSlug,
			ApplicationID:    testAppID,
			ApplicationSlug:  testAppName,
			GrantedScopes:    []string{"read"},
			CreatedAt:        now,
			UpdatedAt:        now,
			Revision:         1,
		},
	}

	svc := newServiceTestService(uow)

	result, err := svc.RevokeConsent(context.Background(), RevokeConsentCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		ApplicationID:    testAppID,
		GrantedScopes:    []string{"read"},
	})
	if err != nil {
		t.Fatalf("RevokeConsent() error = %v, want nil", err)
	}
	// All scopes revoked, consent should be fully revoked
	if result.Consent.RevokedAt == nil {
		t.Error("expected RevokedAt to be set when all scopes are revoked")
	}
}

// ---------------------------------------------------------------------------
// RevokeToken tests (token_revocation.go)
// ---------------------------------------------------------------------------

func TestRevokeToken_EmptyToken(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	// Per RFC 7009, empty token returns success
	err := svc.RevokeToken(context.Background(), RevokeTokenCommand{
		Token:    "",
		ClientID: testAppClientID,
	})
	if err != nil {
		t.Errorf("RevokeToken() error = %v, want nil for empty token", err)
	}
}

func TestRevokeToken_EmptyClientID(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	// RFC 7009 §2.2.1: missing client credentials → 401 invalid_client.
	err := svc.RevokeToken(context.Background(), RevokeTokenCommand{
		Token:    "some-token",
		ClientID: "",
	})
	protocolErr, ok := err.(interface {
		ProtocolCode() string
		HTTPStatus() int
	})
	if !ok || protocolErr.ProtocolCode() != "invalid_client" || protocolErr.HTTPStatus() != 401 {
		t.Errorf("RevokeToken() error = %v, want 401 invalid_client", err)
	}
}

func TestRevokeToken_ClientNotFound(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	// Unknown client must look identical to a wrong secret: 401 invalid_client.
	err := svc.RevokeToken(context.Background(), RevokeTokenCommand{
		Token:         "some-token",
		TokenTypeHint: "access_token",
		ClientID:      "nonexistent-client",
	})
	protocolErr, ok := err.(interface {
		ProtocolCode() string
		HTTPStatus() int
	})
	if !ok || protocolErr.ProtocolCode() != "invalid_client" || protocolErr.HTTPStatus() != 401 {
		t.Errorf("RevokeToken() error = %v, want 401 invalid_client", err)
	}
}

func TestHashTokenValueForRevocation(t *testing.T) {
	// Test that the hash function is deterministic
	token := "test-token-value"
	hash1 := hashTokenValueForRevocation(token)
	hash2 := hashTokenValueForRevocation(token)
	if hash1 != hash2 {
		t.Errorf("hash is not deterministic: %q != %q", hash1, hash2)
	}

	// Different tokens should produce different hashes
	hash3 := hashTokenValueForRevocation("different-token")
	if hash1 == hash3 {
		t.Error("different tokens should produce different hashes")
	}

	// Empty token should still produce a valid hash
	hash4 := hashTokenValueForRevocation("")
	if hash4 == "" {
		t.Error("empty token should produce a non-empty hash")
	}
}

// ---------------------------------------------------------------------------
// ExitImpersonate tests (service.go)
// ---------------------------------------------------------------------------

func TestExitImpersonate_Success(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	err := svc.ExitImpersonate(context.Background(), testOrgSlug, testUsername)
	if err != nil {
		t.Errorf("ExitImpersonate() error = %v, want nil", err)
	}
}

func TestExitImpersonate_OrgNotFound(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	err := svc.ExitImpersonate(context.Background(), "nonexistent-org", testUsername)
	if err == nil {
		t.Fatal("expected error for non-existent org")
	}
}

func TestExitImpersonate_UserNotFound(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	err := svc.ExitImpersonate(context.Background(), testOrgSlug, "nonexistent-user")
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

// ExitImpersonate must revoke only the tokens minted by ImpersonateUser ("imp"
// prefix). Revoking every token of the user is a forced logout: the target
// user's own sessions have to survive an exit-impersonate.
func TestExitImpersonate_RevokesOnlyImpersonationTokens(t *testing.T) {
	impToken := &tokendomain.Token{
		ID: "imp-token-1", UserID: testUserID, OrganizationID: testOrgID,
		ApplicationID: testAppID, TokenPrefix: "imp",
	}
	// Two tokens that must survive: the user's regular access token, and an
	// identical "imp"-prefixed token belonging to another organization.
	regularToken := &tokendomain.Token{
		ID: "at-token-1", UserID: testUserID, OrganizationID: testOrgID,
		ApplicationID: testAppID, TokenPrefix: "at",
	}
	foreignTenantToken := &tokendomain.Token{
		ID: "imp-token-2", UserID: testUserID, OrganizationID: "other-org",
		ApplicationID: testAppID, TokenPrefix: "imp",
	}
	tokenRepo := &testTokenRepository{tokens: map[string]*tokendomain.Token{
		impToken.ID: impToken, regularToken.ID: regularToken, foreignTenantToken.ID: foreignTenantToken,
	}}
	uow := newServiceTestUow(newTestOrg(), newTestUser(), newTestApp())
	uow.tokenRepo = tokenRepo
	svc := newServiceTestService(uow)

	if err := svc.ExitImpersonate(context.Background(), testOrgSlug, testUsername); err != nil {
		t.Fatalf("ExitImpersonate() error = %v, want nil", err)
	}

	if len(tokenRepo.revokeQueries) != 1 {
		t.Fatalf("RevokeByQuery calls = %d, want 1", len(tokenRepo.revokeQueries))
	}
	query := tokenRepo.revokeQueries[0]
	if query.TokenPrefix != "imp" {
		t.Errorf("RevokeByQuery TokenPrefix = %q, want imp", query.TokenPrefix)
	}
	if query.OrganizationID != testOrgID {
		t.Errorf("RevokeByQuery OrganizationID = %q, want %q", query.OrganizationID, testOrgID)
	}
	if query.UserID != testUserID {
		t.Errorf("RevokeByQuery UserID = %q, want %q", query.UserID, testUserID)
	}

	if impToken.RevokedAt == nil {
		t.Error("impersonation token was not revoked")
	}
	if regularToken.RevokedAt != nil {
		t.Error("regular access token must survive exit-impersonate")
	}
	if foreignTenantToken.RevokedAt != nil {
		t.Error("impersonation token of another organization must not be revoked")
	}
}

// ---------------------------------------------------------------------------
// RecordLoginFailure tests (account_lockout.go)
// ---------------------------------------------------------------------------

func TestRecordLoginFailure_NewAttempt(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	err := svc.RecordLoginFailure(context.Background(), testUserID, testOrgID)
	if err != nil {
		t.Errorf("RecordLoginFailure() error = %v, want nil", err)
	}

	// Verify that an attempt was recorded
	attempt, err := uow.lockoutAttempt.GetByUserID(context.Background(), testUserID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if attempt == nil {
		t.Fatal("expected lockout attempt to be recorded")
	}
}

func TestRecordLoginFailure_IncrementAttempt(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	// Record first failure
	_ = svc.RecordLoginFailure(context.Background(), testUserID, testOrgID)

	// Record second failure
	err := svc.RecordLoginFailure(context.Background(), testUserID, testOrgID)
	if err != nil {
		t.Errorf("RecordLoginFailure() error = %v, want nil", err)
	}

	// Verify attempt count increased
	attempt, _ := uow.lockoutAttempt.GetByUserID(context.Background(), testUserID)
	if attempt == nil {
		t.Fatal("expected lockout attempt to be recorded")
	}
}

// ---------------------------------------------------------------------------
// validateGrantedConsentScopes tests (consent.go)
// ---------------------------------------------------------------------------

func TestValidateGrantedConsentScopes_EmptyScopes(t *testing.T) {
	app := &applications.Application{
		CustomScopes: []applications.ApplicationCustomScope{
			{Scope: "read", DisplayName: "Read"},
		},
	}
	_, err := validateGrantedConsentScopes(app, []string{})
	if err == nil {
		t.Fatal("expected error for empty scopes")
	}
}

func TestValidateGrantedConsentScopes_NoCustomScopes(t *testing.T) {
	app := &applications.Application{}
	_, err := validateGrantedConsentScopes(app, []string{"read"})
	if err == nil {
		t.Fatal("expected error when app has no custom scopes")
	}
}

func TestValidateGrantedConsentScopes_ValidScopes(t *testing.T) {
	app := &applications.Application{
		CustomScopes: []applications.ApplicationCustomScope{
			{Scope: "read", DisplayName: "Read"},
			{Scope: "write", DisplayName: "Write"},
		},
	}
	scopes, err := validateGrantedConsentScopes(app, []string{"read"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scopes) != 1 || scopes[0] != "read" {
		t.Errorf("scopes = %v, want [read]", scopes)
	}
}

func TestValidateGrantedConsentScopes_InvalidScope(t *testing.T) {
	app := &applications.Application{
		CustomScopes: []applications.ApplicationCustomScope{
			{Scope: "read", DisplayName: "Read"},
		},
	}
	_, err := validateGrantedConsentScopes(app, []string{"admin"})
	if err == nil {
		t.Fatal("expected error for scope not in custom scopes")
	}
}

// ---------------------------------------------------------------------------
// filterConsentableScopes tests (consent.go)
// ---------------------------------------------------------------------------

func TestFilterConsentableScopes_EmptyRequested(t *testing.T) {
	customScopes := []applications.ApplicationCustomScope{
		{Scope: "read", DisplayName: "Read"},
	}
	result := filterConsentableScopes([]string{}, customScopes)
	if len(result) != 0 {
		t.Errorf("expected empty, got %v", result)
	}
}

func TestFilterConsentableScopes_EmptyCustomScopes(t *testing.T) {
	result := filterConsentableScopes([]string{"read"}, nil)
	if len(result) != 0 {
		t.Errorf("expected empty, got %v", result)
	}
}

func TestFilterConsentableScopes_MatchingScopes(t *testing.T) {
	customScopes := []applications.ApplicationCustomScope{
		{Scope: "read", DisplayName: "Read"},
		{Scope: "write", DisplayName: "Write"},
	}
	result := filterConsentableScopes([]string{"read", "admin", "write"}, customScopes)
	if len(result) != 2 {
		t.Errorf("expected 2 consentable scopes, got %d: %v", len(result), result)
	}
}

// ---------------------------------------------------------------------------
// getActiveGrantedScopes tests (consent.go)
// ---------------------------------------------------------------------------

func TestGetActiveGrantedScopes_NoConsent(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)
	svc := newServiceTestService(uow)

	scopes, err := svc.getActiveGrantedScopes(context.Background(), testUserID, testAppID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scopes) != 0 {
		t.Errorf("expected empty scopes, got %v", scopes)
	}
}

func TestGetActiveGrantedScopes_WithActiveConsent(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)

	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	uow.consentRepo.consents = map[string]*consentdomain.Consent{
		consentKey(testUserID, testAppID): {
			ID:            "consent-1",
			UserID:        testUserID,
			ApplicationID: testAppID,
			GrantedScopes: []string{"read", "write"},
			CreatedAt:     now,
			UpdatedAt:     now,
			Revision:      1,
		},
	}

	svc := newServiceTestService(uow)

	scopes, err := svc.getActiveGrantedScopes(context.Background(), testUserID, testAppID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scopes) != 2 {
		t.Errorf("expected 2 scopes, got %d: %v", len(scopes), scopes)
	}
}

func TestGetActiveGrantedScopes_WithRevokedConsent(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newServiceTestUow(org, user, app)

	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	uow.consentRepo.consents = map[string]*consentdomain.Consent{
		consentKey(testUserID, testAppID): {
			ID:            "consent-1",
			UserID:        testUserID,
			ApplicationID: testAppID,
			GrantedScopes: []string{"read"},
			CreatedAt:     now,
			UpdatedAt:     now,
			RevokedAt:     &now,
			Revision:      1,
		},
	}

	svc := newServiceTestService(uow)

	scopes, err := svc.getActiveGrantedScopes(context.Background(), testUserID, testAppID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scopes) != 0 {
		t.Errorf("expected empty scopes for revoked consent, got %v", scopes)
	}
}
