package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	domainworkload "github.com/guliangiam/guliangiam/internal/domain/workload"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type authTestWorkloadRepository struct {
	domainworkload.Repository
	accounts []*domainworkload.ServiceAccount
	trusts   []*domainworkload.FederationTrust
}

func (r *authTestWorkloadRepository) GetAccountByClientID(_ context.Context, clientID string) (*domainworkload.ServiceAccount, error) {
	for _, account := range r.accounts {
		if account.ClientID == clientID {
			return account, nil
		}
	}
	return nil, apperrors.NewNotFound("service account not found")
}

func (r *authTestWorkloadRepository) GetAccountByName(_ context.Context, orgID, name string) (*domainworkload.ServiceAccount, error) {
	for _, account := range r.accounts {
		if account.OrganizationID == orgID && account.Name == name {
			return account, nil
		}
	}
	return nil, apperrors.NewNotFound("service account not found")
}

func (r *authTestWorkloadRepository) FindActiveTrustsByIssuer(_ context.Context, issuer string) ([]*domainworkload.FederationTrust, error) {
	var result []*domainworkload.FederationTrust
	for _, trust := range r.trusts {
		if trust.Issuer == issuer && trust.Status == domainworkload.TrustStatusActive {
			result = append(result, trust)
		}
	}
	return result, nil
}

type authTestRecordsRepository struct {
	domainrecords.Repository
	created []*domainrecords.Record
}

func (r *authTestRecordsRepository) Create(_ context.Context, record *domainrecords.Record) error {
	r.created = append(r.created, record)
	return nil
}

type authTestOutboxRepository struct {
	events []*domainoutbox.OutboxEvent
}

func (r *authTestOutboxRepository) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	r.events = append(r.events, event)
	return nil
}

func (r *authTestOutboxRepository) EnqueueBatch(_ context.Context, events []*domainoutbox.OutboxEvent) error {
	r.events = append(r.events, events...)
	return nil
}

type authWLITestUOW struct {
	serviceTestUnitOfWork
	records           *authTestRecordsRepository
	outbox            *authTestOutboxRepository
	transactionScopes []tenantctx.TenantScope
}

func (u *authWLITestUOW) Records() domainrecords.Repository { return u.records }
func (u *authWLITestUOW) Outbox() domainoutbox.Repository   { return u.outbox }
func (u *authWLITestUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	u.transactionScopes = append(u.transactionScopes, tenantctx.TenantScopeFromContext(ctx))
	return fn(ctx, u)
}

func newAuthWLITestUOW(workloadRepo *authTestWorkloadRepository) *authWLITestUOW {
	base := newServiceTestUow(newTestOrg(), newTestUser(), newTestApp())
	base.workloadRepo = workloadRepo
	return &authWLITestUOW{
		serviceTestUnitOfWork: base,
		records:               &authTestRecordsRepository{},
		outbox:                &authTestOutboxRepository{},
	}
}

func newClientCredentialsFixture(t *testing.T, account *domainworkload.ServiceAccount, application *applicationdomain.Application) (Service, *authWLITestUOW) {
	t.Helper()
	accounts := []*domainworkload.ServiceAccount{}
	if account != nil {
		accounts = append(accounts, account)
	}
	uow := newAuthWLITestUOW(&authTestWorkloadRepository{accounts: accounts})
	if application != nil {
		uow.appRepo.byClientID = map[string]*applicationdomain.Application{application.ClientID: application}
	}
	return Service{
		Clock:         fakeClock{fixedTime: time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "wli-access-token-id"},
		UOW:           uow,
	}, uow
}

func mustClientSecretHash(t *testing.T, secret string) string {
	t.Helper()
	hash, err := platformcrypto.HashClientSecret(secret)
	require.NoError(t, err)
	return hash
}

func requireClientCredentialsProtocolError(t *testing.T, err error, code string, status int) {
	t.Helper()
	require.Error(t, err)
	var protocolErr apperrors.ProtocolCodeError
	require.ErrorAs(t, err, &protocolErr)
	assert.Equal(t, code, protocolErr.ProtocolCode())
	var statusErr interface{ HTTPStatus() int }
	require.ErrorAs(t, err, &statusErr)
	assert.Equal(t, status, statusErr.HTTPStatus())
}

// A service account can authenticate but cannot become a token subject:
// access_tokens.user_id carries FK -> users(id) and the service account id is
// not a user id (23503), while application_id is never set (NOT NULL). The
// grant is therefore reported as unsupported until token subjects are modeled
// as principals. See
// .trae/documents/client-auth-reachability/adr-principal-model.md.
func TestExchangeClientCredentials_ServiceAccountIsUnsupportedUntilPrincipalsLand(t *testing.T) {
	const secret = "service-account-secret"
	account := &domainworkload.ServiceAccount{
		ID: "sa-1", OrganizationID: testOrgID, PoolID: "pool-1", Name: "ci-bot",
		ClientID: "gl_cid_ci", ClientSecretHash: mustClientSecretHash(t, secret),
		Scopes: []string{"orders:read", "orders:write"}, AccessTokenTTLSeconds: 1200,
		Status: domainworkload.SAStatusActive,
	}
	application := &applicationdomain.Application{
		ID: testAppID, OrganizationID: testOrgID, Slug: "application-fallback", ClientID: account.ClientID,
		ClientSecretHash: mustClientSecretHash(t, "application-secret"), GrantTypes: []string{clientCredentialsGrantType},
	}
	service, uow := newClientCredentialsFixture(t, account, application)

	response, err := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{
		ClientID: account.ClientID, ClientSecret: secret, Scope: "orders:read orders:read",
	})
	assert.Nil(t, response)
	requireClientCredentialsProtocolError(t, err, "unsupported_grant_type", http.StatusBadRequest)

	assert.Empty(t, uow.tokenRepo.tokens)
	assert.Empty(t, uow.tokenRepo.createdRefresh)
	assert.Empty(t, uow.records.created)
	assert.Empty(t, uow.outbox.events)
	require.Len(t, uow.transactionScopes, 1)
	assert.Equal(t, tenantctx.TenantFilterSystem, uow.transactionScopes[0].Mode)
}

func TestExchangeClientCredentials_ServiceAccountAuthenticationFailuresNeverFallBack(t *testing.T) {
	const secret = "service-account-secret"
	validHash := mustClientSecretHash(t, secret)
	tests := []struct {
		name   string
		status domainworkload.ServiceAccountStatus
		hash   string
		secret string
	}{
		{name: "wrong secret", status: domainworkload.SAStatusActive, hash: validHash, secret: "wrong"},
		{name: "missing secret", status: domainworkload.SAStatusActive, hash: validHash, secret: ""},
		{name: "empty hash", status: domainworkload.SAStatusActive, hash: "", secret: secret},
		{name: "disabled", status: domainworkload.SAStatusDisabled, hash: validHash, secret: secret},
		{name: "deleted", status: domainworkload.SAStatusDeleted, hash: validHash, secret: secret},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			account := &domainworkload.ServiceAccount{
				ID: "sa-1", OrganizationID: testOrgID, Name: "ci-bot", ClientID: "gl_cid_ci",
				ClientSecretHash: test.hash, AccessTokenTTLSeconds: 900, Status: test.status,
			}
			application := &applicationdomain.Application{
				ID: testAppID, OrganizationID: testOrgID, ClientID: account.ClientID,
				ClientSecretHash: mustClientSecretHash(t, secret), GrantTypes: []string{clientCredentialsGrantType},
			}
			service, uow := newClientCredentialsFixture(t, account, application)
			response, err := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{
				ClientID: account.ClientID, ClientSecret: test.secret,
			})
			assert.Nil(t, response)
			requireClientCredentialsProtocolError(t, err, "invalid_client", http.StatusUnauthorized)
			assert.Empty(t, uow.tokenRepo.tokens)
			assert.Empty(t, uow.records.created)
			assert.Empty(t, uow.outbox.events)
			assert.Len(t, uow.transactionScopes, 1)
		})
	}
}

func TestExchangeClientCredentials_ServiceAccountScopeAndTTLBoundaries(t *testing.T) {
	const secret = "service-account-secret"
	hash := mustClientSecretHash(t, secret)
	t.Run("scope must be subset", func(t *testing.T) {
		for _, test := range []struct {
			name      string
			allowed   []string
			requested string
		}{
			{name: "empty account scopes grants no permission", requested: "orders:read"},
			{name: "outside allowed scopes", allowed: []string{"orders:read"}, requested: "orders:write"},
		} {
			t.Run(test.name, func(t *testing.T) {
				account := &domainworkload.ServiceAccount{
					ID: "sa-scope", OrganizationID: testOrgID, Name: "scope-bot", ClientID: "gl_cid_scope",
					ClientSecretHash: hash, Scopes: test.allowed, AccessTokenTTLSeconds: 900, Status: domainworkload.SAStatusActive,
				}
				service, uow := newClientCredentialsFixture(t, account, nil)
				_, err := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{
					ClientID: account.ClientID, ClientSecret: secret, Scope: test.requested,
				})
				requireClientCredentialsProtocolError(t, err, "invalid_scope", http.StatusBadRequest)
				assert.Empty(t, uow.tokenRepo.tokens)
			})
		}
	})

	for _, ttl := range []int{-1, 59, 3601} {
		t.Run(fmt.Sprintf("invalid TTL %d", ttl), func(t *testing.T) {
			account := &domainworkload.ServiceAccount{
				ID: "sa-ttl", OrganizationID: testOrgID, Name: "ttl-bot", ClientID: "gl_cid_ttl",
				ClientSecretHash: hash, AccessTokenTTLSeconds: ttl, Status: domainworkload.SAStatusActive,
			}
			service, uow := newClientCredentialsFixture(t, account, nil)
			response, err := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{
				ClientID: account.ClientID, ClientSecret: secret,
			})
			assert.Nil(t, response)
			require.Error(t, err)
			assert.Empty(t, uow.tokenRepo.tokens)
		})
	}
}

func TestExchangeClientCredentials_ApplicationFallbackAndProtocolErrors(t *testing.T) {
	const secret = "application-secret"
	hash := mustClientSecretHash(t, secret)
	newApplication := func() *applicationdomain.Application {
		return &applicationdomain.Application{
			ID: testAppID, OrganizationID: testOrgID, Slug: "confidential", ClientID: "application-client",
			ClientSecretHash: hash, GrantTypes: []string{clientCredentialsGrantType},
		}
	}

	// An application authenticates, but its token cannot be persisted either:
	// user_id stays empty and the INSERT binds $2::uuid, which PostgreSQL
	// rejects as 22P02. The honest answer is that the grant is unsupported.
	t.Run("authenticated application is unsupported", func(t *testing.T) {
		application := newApplication()
		service, uow := newClientCredentialsFixture(t, nil, application)
		response, err := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{
			ClientID: application.ClientID, ClientSecret: secret, Scope: "openid profile",
		})
		assert.Nil(t, response)
		requireClientCredentialsProtocolError(t, err, "unsupported_grant_type", http.StatusBadRequest)
		assert.Empty(t, uow.tokenRepo.tokens)
		assert.Empty(t, uow.records.created)
		assert.Empty(t, uow.outbox.events)
	})

	t.Run("public client", func(t *testing.T) {
		application := newApplication()
		application.ClientSecretHash = ""
		application.TokenEndpointAuthMethod = applicationdomain.TokenEndpointAuthMethodNone
		service, _ := newClientCredentialsFixture(t, nil, application)
		_, err := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{ClientID: application.ClientID})
		requireClientCredentialsProtocolError(t, err, "unsupported_grant_type", http.StatusBadRequest)
	})

	t.Run("grant disabled", func(t *testing.T) {
		application := newApplication()
		application.GrantTypes = []string{"authorization_code"}
		service, _ := newClientCredentialsFixture(t, nil, application)
		_, err := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{ClientID: application.ClientID, ClientSecret: secret})
		requireClientCredentialsProtocolError(t, err, "unsupported_grant_type", http.StatusBadRequest)
	})

	t.Run("authentication failed", func(t *testing.T) {
		application := newApplication()
		service, _ := newClientCredentialsFixture(t, nil, application)
		_, err := service.ExchangeClientCredentials(context.Background(), ExchangeClientCredentialsCommand{ClientID: application.ClientID, ClientSecret: "wrong"})
		requireClientCredentialsProtocolError(t, err, "invalid_client", http.StatusUnauthorized)
	})
}

func TestPasswordLogin_RejectsServiceAccountIdentifiersWithoutCreatingLoginState(t *testing.T) {
	for _, status := range []domainworkload.ServiceAccountStatus{
		domainworkload.SAStatusActive, domainworkload.SAStatusDisabled, domainworkload.SAStatusDeleted,
	} {
		for _, identifier := range []string{"client_id", "name"} {
			t.Run(string(status)+"/"+identifier, func(t *testing.T) {
				account := &domainworkload.ServiceAccount{
					ID: "sa-login", OrganizationID: testOrgID, Name: "ci-bot", ClientID: "gl_cid_login", Status: status,
				}
				base := newTestUow(newTestOrg(), newTestUser(), newTestApp())
				base.workloadRepo = &authTestWorkloadRepository{accounts: []*domainworkload.ServiceAccount{account}}
				service := newTestService(base)
				username := account.Name
				if identifier == "client_id" {
					username = account.ClientID
				}
				result, err := service.Login(context.Background(), LoginCommand{
					OrganizationSlug: testOrgSlug, Username: username, Password: "not-a-user-password", ApplicationID: testAppID,
				})
				assert.Nil(t, result)
				require.Error(t, err)
				var appErr *apperrors.Error
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, apperrors.CodeUnauthorized, appErr.Code)
				assert.Equal(t, "invalid username or password", appErr.Message)
				assert.Empty(t, base.sessRepo.sessions)
				assert.Empty(t, base.challengeRepo.items)
			})
		}
	}
}

func TestPasswordLogin_ServiceAccountRepositoryDoesNotAffectOrdinaryUser(t *testing.T) {
	base := newTestUow(newTestOrg(), newTestUser(), newTestApp())
	base.workloadRepo = &authTestWorkloadRepository{accounts: []*domainworkload.ServiceAccount{{
		ID: "sa-other", OrganizationID: testOrgID, Name: "ci-bot", ClientID: "gl_cid_other", Status: domainworkload.SAStatusActive,
	}}}
	service := newTestService(base)
	result, err := service.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug, Username: testUsername, Password: testPassword, ApplicationID: testAppID,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, LoginStatusSuccess, result.Response.Status)
}

func federationClaims(subject string) map[string]interface{} {
	return map[string]interface{}{
		"iss": federationTestIssuer, "sub": subject, "aud": "guliangiam",
		"exp": float64(time.Now().Add(time.Hour).Unix()),
	}
}

const federationTestIssuer = "https://issuer.example.com"

func newFederationTestService(resolver FederationTrustResolver, verifier ExternalJWTVerifier) (Service, *authWLITestUOW) {
	uow := newAuthWLITestUOW(&authTestWorkloadRepository{})
	return Service{
		Clock:              fakeClock{fixedTime: time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)},
		UUIDGenerator:      fakeUUIDGenerator{fixedID: "federation-token"},
		UOW:                uow,
		federationResolver: resolver,
		federationVerifier: verifier,
	}, uow
}

func TestResolveJWTSubject_FederationCandidatesRequireCompleteMatch(t *testing.T) {
	const issuer = "https://issuer.example.com"
	resolver := newStubTrustResolver()
	resolver.Add(&domainworkload.FederationTrust{
		ID: "other-organization", OrganizationID: "org-other", Issuer: issuer, Audience: []string{"guliangiam"},
		Subjects: []string{"repo:org/*"}, Status: domainworkload.TrustStatusActive,
	})
	resolver.Add(&domainworkload.FederationTrust{
		ID: "tenant-a", OrganizationID: testOrgID, Issuer: issuer, Audience: []string{"guliangiam"},
		Subjects: []string{"repo:org/*"}, Status: domainworkload.TrustStatusActive,
	})
	resolver.Add(&domainworkload.FederationTrust{
		ID: "tenant-b", OrganizationID: testOrgID, Issuer: issuer, Audience: []string{"guliangiam"},
		Subjects: []string{"repo:org/*"}, Status: domainworkload.TrustStatusActive,
	})
	var verified []string
	verifier := &stubJWTVerifier{verifyFunc: func(_ string, trust *domainworkload.FederationTrust) (map[string]interface{}, error) {
		verified = append(verified, trust.ID)
		if trust.ID == "tenant-a" {
			return nil, errors.New("signature mismatch")
		}
		return federationClaims("repo:org/api"), nil
	}}
	service, uow := newFederationTestService(resolver, verifier)
	token := makeFederationJWT(issuer, "repo:org/api", time.Now().Add(time.Hour))
	subject, err := service.resolveJWTSubjectTokenForOrganization(context.Background(), token, testOrgID)
	require.NoError(t, err)
	assert.Equal(t, "repo:org/api", subject.UserID)
	assert.Equal(t, []string{"tenant-a", "tenant-b"}, verified)
	assert.Empty(t, uow.records.created)
	assert.Empty(t, uow.outbox.events)
}

func TestResolveJWTSubject_FederationRejectionAuditAndOutbox(t *testing.T) {
	const issuer = "https://issuer.example.com"
	validClaims := func() (map[string]interface{}, error) { return federationClaims("repo:org/api"), nil }
	tests := []struct {
		name              string
		mutateTrust       func(*domainworkload.FederationTrust)
		verify            func() (map[string]interface{}, error)
		reason            string
		expectedVerifyUse int
	}{
		{name: "subject outside whitelist", verify: func() (map[string]interface{}, error) {
			return federationClaims("repo:other/api"), nil
		}, reason: "subject_not_allowed", expectedVerifyUse: 1},
		{name: "audience mismatch", verify: func() (map[string]interface{}, error) {
			return nil, errors.New("audience mismatch")
		}, reason: "jwt_verification_failed", expectedVerifyUse: 1},
		{name: "bad signature", verify: func() (map[string]interface{}, error) {
			return nil, errors.New("bad signature")
		}, reason: "jwt_verification_failed", expectedVerifyUse: 1},
		{name: "empty audience", mutateTrust: func(trust *domainworkload.FederationTrust) {
			trust.Audience = nil
		}, verify: validClaims, reason: "audience_empty"},
		{name: "empty subjects", mutateTrust: func(trust *domainworkload.FederationTrust) {
			trust.Subjects = nil
		}, verify: validClaims, reason: "subjects_empty"},
		{name: "disabled", mutateTrust: func(trust *domainworkload.FederationTrust) {
			trust.Status = domainworkload.TrustStatusDisabled
		}, verify: validClaims, reason: "trust_inactive"},
		{name: "deleted", mutateTrust: func(trust *domainworkload.FederationTrust) {
			trust.Status = domainworkload.TrustStatusDeleted
		}, verify: validClaims, reason: "trust_inactive"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			trust := &domainworkload.FederationTrust{
				ID: "trust-1", OrganizationID: testOrgID, Issuer: issuer, Audience: []string{"guliangiam"},
				Subjects: []string{"repo:org/*"}, Status: domainworkload.TrustStatusActive,
			}
			if test.mutateTrust != nil {
				test.mutateTrust(trust)
			}
			resolver := newStubTrustResolver()
			resolver.Add(trust)
			resolver.includeInactive = true
			verifyCalls := 0
			verifier := &stubJWTVerifier{verifyFunc: func(_ string, _ *domainworkload.FederationTrust) (map[string]interface{}, error) {
				verifyCalls++
				return test.verify()
			}}
			service, uow := newFederationTestService(resolver, verifier)
			rawToken := makeFederationJWT(issuer, "repo:org/api", time.Now().Add(time.Hour))
			subject, err := service.resolveJWTSubjectTokenForOrganization(context.Background(), rawToken, testOrgID)
			assert.Nil(t, subject)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "federation JWT verification failed")
			assert.Equal(t, test.expectedVerifyUse, verifyCalls)
			assert.Empty(t, uow.tokenRepo.tokens)

			require.Len(t, uow.records.created, 1)
			record := uow.records.created[0]
			assert.Equal(t, "wli.federation.rejected", record.Action)
			assert.Equal(t, testOrgID, record.OrganizationID)
			require.Len(t, uow.outbox.events, 1)
			event := uow.outbox.events[0]
			assert.Equal(t, "wli.federation.rejected", event.EventType)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(event.Payload, &payload))
			assert.Equal(t, issuer, payload["issuer"])
			assert.Equal(t, test.reason, payload["reason"])
			assert.NotContains(t, string(event.Payload), rawToken)
			assert.NotContains(t, string(event.Payload), "client_secret")
			assert.NotContains(t, string(event.Payload), "hash")
		})
	}
}

func TestConfigureFederation_UsesSystemUOWResolver(t *testing.T) {
	const issuer = "https://issuer.example.com"
	repo := &authTestWorkloadRepository{trusts: []*domainworkload.FederationTrust{{
		ID: "trust-1", OrganizationID: "org-a", Issuer: issuer, Audience: []string{"guliangiam"},
		Subjects: []string{"repo:org/*"}, Status: domainworkload.TrustStatusActive,
	}}}
	uow := newAuthWLITestUOW(repo)
	service := Service{
		Clock: fakeClock{fixedTime: time.Now()}, UUIDGenerator: fakeUUIDGenerator{fixedID: "id"}, UOW: uow,
	}
	service.ConfigureFederation(&stubJWTVerifier{verifiedClaims: federationClaims("repo:org/api")})
	require.NotNil(t, service.federationResolver)
	require.NotNil(t, service.federationVerifier)
	token := makeFederationJWT(issuer, "repo:org/api", time.Now().Add(time.Hour))
	subject, err := service.resolveJWTSubjectToken(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, "repo:org/api", subject.UserID)
	require.Len(t, uow.transactionScopes, 1)
	assert.Equal(t, tenantctx.TenantFilterSystem, uow.transactionScopes[0].Mode)

	service.ConfigureFederation(nil)
	assert.Nil(t, service.federationResolver)
	assert.Nil(t, service.federationVerifier)
}
