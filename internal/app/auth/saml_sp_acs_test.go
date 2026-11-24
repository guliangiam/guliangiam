package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const samlACSTestProviderID = "01a00000-0000-7000-8000-0000000000b1"

type samlACSTestProviderRepository struct {
	providerdomain.Repository
	provider *providerdomain.Provider
}

func (r samlACSTestProviderRepository) GetByID(context.Context, string) (*providerdomain.Provider, error) {
	if r.provider == nil {
		return nil, apperrors.NewNotFound("provider not found")
	}
	return r.provider, nil
}

func (r samlACSTestProviderRepository) GetByIDForOrganization(_ context.Context, id, organizationID string) (*providerdomain.Provider, error) {
	if r.provider == nil || r.provider.ID != id || r.provider.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("provider not found")
	}
	return r.provider, nil
}

type samlACSTestAppRepository struct {
	*loginTestAppRepository
	defaultApp *applicationdomain.Application
}

func (r samlACSTestAppRepository) GetByOrganizationAndSlug(_ context.Context, organizationID, slug string) (*applicationdomain.Application, error) {
	if r.defaultApp != nil && r.defaultApp.OrganizationID == organizationID && slug == "app-"+testOrgSlug {
		return r.defaultApp, nil
	}
	return nil, apperrors.NewNotFound("application not found")
}

type samlACSTestUoW struct {
	loginTestUnitOfWork
	providers providerdomain.Repository
	apps      applicationdomain.Repository
}

func (u samlACSTestUoW) Providers() providerdomain.Repository       { return u.providers }
func (u samlACSTestUoW) Applications() applicationdomain.Repository { return u.apps }

func (u samlACSTestUoW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

type samlACSTestReplayStore struct {
	mu   sync.Mutex
	keys map[string]bool
}

func newSAMLAcstestReplayStore() *samlACSTestReplayStore {
	return &samlACSTestReplayStore{keys: make(map[string]bool)}
}

func (s *samlACSTestReplayStore) Reserve(_ context.Context, key string, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.keys[key] {
		return false, nil
	}
	s.keys[key] = true
	return true, nil
}

// samlACSTestSigner emits the real v1 envelope shape so the app layer's
// 10-minute check is exercised, while letting tests reject the signature or
// move the issue time into the past.
type samlACSTestSigner struct {
	issuedAt time.Time
	reject   bool
}

func (s samlACSTestSigner) Sign(payload, _, _ string) (string, error) {
	return fmt.Sprintf("v1.%d.%s.%s", s.issuedAt.Unix(), strings.Repeat("a", 64), payload), nil
}

func (s samlACSTestSigner) Verify(signed, _, _ string) (string, error) {
	if s.reject {
		return "", errors.New("signature mismatch")
	}
	parts := strings.SplitN(signed, ".", 4)
	if len(parts) != 4 || parts[0] != "v1" {
		return "", errors.New("invalid envelope")
	}
	return parts[3], nil
}

func samlACSTestState(t *testing.T, signer samlACSTestSigner, orgSlug, providerID, nonce string) string {
	t.Helper()
	signed, err := signer.Sign(encodeSAMLSPStatePayload(orgSlug, providerID, nonce), orgSlug, providerID)
	require.NoError(t, err)
	return signed
}

func samlACSTestProvider() *providerdomain.Provider {
	return &providerdomain.Provider{
		ID:             samlACSTestProviderID,
		OrganizationID: testOrgID,
		Name:           "corp-saml",
		Category:       providerdomain.CategorySAML,
		ProviderType:   providerdomain.ProviderSAML,
		Enabled:        true,
		CallbackUrl:    "https://sp.example.com/api/v1/auth/saml/acs/test-org/" + samlACSTestProviderID,
	}
}

func newSAMLACSService(t *testing.T, provider *providerdomain.Provider) (Service, samlACSTestUoW, *samlACSTestReplayStore, samlACSTestSigner) {
	t.Helper()
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()

	base := newTestUow(org, user, app)
	uow := samlACSTestUoW{
		loginTestUnitOfWork: base,
		providers:           samlACSTestProviderRepository{provider: provider},
		apps: samlACSTestAppRepository{
			loginTestAppRepository: base.appRepo,
			defaultApp:             app,
		},
	}
	svc := newTestService(base)
	svc.UOW = uow
	store := newSAMLAcstestReplayStore()
	svc.SAMLReplayStore = store
	signer := samlACSTestSigner{issuedAt: svc.Clock.Now().UTC()}
	svc.RelayStateSigner = signer
	return svc, uow, store, signer
}

func validSAMLCommand(t *testing.T, signer samlACSTestSigner) CompleteSAMLSocialLoginCommand {
	t.Helper()
	return CompleteSAMLSocialLoginCommand{
		OrganizationSlug: testOrgSlug,
		ProviderID:       samlACSTestProviderID,
		SAMLResponse:     "base64-response",
		RelayState:       samlACSTestState(t, signer, testOrgSlug, samlACSTestProviderID, "nonce-1"),
		IPAddress:        "203.0.113.10",
		UserAgent:        "test-agent",
	}
}

func TestCompleteSAMLSocialLogin_SuccessCreatesSessionAndAMR(t *testing.T) {
	original := exchangeSocialCode
	exchangeSocialCode = func(context.Context, SocialProviderConfig, string) (*SocialUserInfo, error) {
		return &SocialUserInfo{
			ID:           "subject-1",
			Username:     "saml-user",
			Email:        "saml-user@example.com",
			AuthnContext: "urn:oasis:names:tc:SAML:2.0:ac:classes:MobileTwoFactorContract",
		}, nil
	}
	t.Cleanup(func() { exchangeSocialCode = original })

	svc, uow, _, signer := newSAMLACSService(t, samlACSTestProvider())
	result, err := svc.CompleteSAMLSocialLogin(context.Background(), validSAMLCommand(t, signer))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.SessionID)

	session := uow.sessRepo.sessions[result.SessionID]
	require.NotNil(t, session)
	assert.Equal(t, "mfa", session.AuthenticationLevel)
	assert.Equal(t, "urn:oasis:names:tc:SAML:2.0:ac:classes:MobileTwoFactorContract", session.DeviceInfo["acr"])
	assert.Equal(t, "mfa", session.DeviceInfo["amr"])
	assert.Equal(t, loginMethodSocial, session.DeviceInfo[paramLoginMethod])
	assert.Equal(t, string(providerdomain.ProviderSAML), session.DeviceInfo["providerType"])
}

func TestCompleteSAMLSocialLogin_RejectsReplayedState(t *testing.T) {
	original := exchangeSocialCode
	exchangeSocialCode = func(context.Context, SocialProviderConfig, string) (*SocialUserInfo, error) {
		return &SocialUserInfo{ID: "subject-1", Username: "saml-user", Email: "saml-user@example.com"}, nil
	}
	t.Cleanup(func() { exchangeSocialCode = original })

	svc, _, _, signer := newSAMLACSService(t, samlACSTestProvider())
	command := validSAMLCommand(t, signer)

	_, err := svc.CompleteSAMLSocialLogin(context.Background(), command)
	require.NoError(t, err)

	_, err = svc.CompleteSAMLSocialLogin(context.Background(), command)
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnauthorized, apperrors.FromError(err).Code)
}

func TestCompleteSAMLSocialLogin_RejectsExpiredState(t *testing.T) {
	svc, _, _, signer := newSAMLACSService(t, samlACSTestProvider())
	signer.issuedAt = svc.Clock.Now().UTC().Add(-11 * time.Minute)
	svc.RelayStateSigner = signer

	_, err := svc.CompleteSAMLSocialLogin(context.Background(), validSAMLCommand(t, signer))
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnauthorized, apperrors.FromError(err).Code)
}

func TestCompleteSAMLSocialLogin_RejectsCrossProviderState(t *testing.T) {
	svc, _, _, signer := newSAMLACSService(t, samlACSTestProvider())
	command := validSAMLCommand(t, signer)
	command.RelayState = samlACSTestState(t, signer, testOrgSlug, "01a00000-0000-7000-8000-0000000000b2", "nonce-1")

	_, err := svc.CompleteSAMLSocialLogin(context.Background(), command)
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnauthorized, apperrors.FromError(err).Code)
}

func TestCompleteSAMLSocialLogin_RejectsCrossOrganizationState(t *testing.T) {
	svc, _, _, signer := newSAMLACSService(t, samlACSTestProvider())
	command := validSAMLCommand(t, signer)
	command.RelayState = samlACSTestState(t, signer, "other-org", samlACSTestProviderID, "nonce-1")

	_, err := svc.CompleteSAMLSocialLogin(context.Background(), command)
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnauthorized, apperrors.FromError(err).Code)
}

func TestCompleteSAMLSocialLogin_RejectsInvalidSignature(t *testing.T) {
	svc, _, _, signer := newSAMLACSService(t, samlACSTestProvider())
	signer.reject = true
	svc.RelayStateSigner = signer

	_, err := svc.CompleteSAMLSocialLogin(context.Background(), validSAMLCommand(t, signer))
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnauthorized, apperrors.FromError(err).Code)
}

func TestCompleteSAMLSocialLogin_FailsClosedWithoutStateStore(t *testing.T) {
	svc, _, _, signer := newSAMLACSService(t, samlACSTestProvider())
	svc.SAMLReplayStore = nil

	_, err := svc.CompleteSAMLSocialLogin(context.Background(), validSAMLCommand(t, signer))
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInternal, apperrors.FromError(err).Code)
}

func TestCompleteSAMLSocialLogin_RejectsForeignProvider(t *testing.T) {
	provider := samlACSTestProvider()
	provider.OrganizationID = "00000000-0000-7000-8000-000000000999"
	svc, _, _, signer := newSAMLACSService(t, provider)

	_, err := svc.CompleteSAMLSocialLogin(context.Background(), validSAMLCommand(t, signer))
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

func TestCompleteSAMLSocialLogin_RejectsNonSAMLProvider(t *testing.T) {
	provider := samlACSTestProvider()
	provider.Category = providerdomain.CategoryOAuth
	provider.ProviderType = providerdomain.ProviderGitHub
	svc, _, _, signer := newSAMLACSService(t, provider)

	_, err := svc.CompleteSAMLSocialLogin(context.Background(), validSAMLCommand(t, signer))
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
}

func TestGetSocialRedirectURL_SAMLStateIsServerGeneratedAndBound(t *testing.T) {
	provider := samlACSTestProvider()
	svc, _, _, signer := newSAMLACSService(t, provider)

	var captured string
	original := getSocialRedirectURL
	getSocialRedirectURL = func(_ context.Context, _ SocialProviderConfig, state string) (string, error) {
		captured = state
		return "https://idp.example.com/sso", nil
	}
	t.Cleanup(func() { getSocialRedirectURL = original })

	_, err := svc.GetSocialRedirectURL(context.Background(), SocialAuthorizeCommand{
		ProviderID:       provider.ID,
		OrganizationSlug: testOrgSlug,
		State:            "attacker-supplied-state",
	})
	require.NoError(t, err)
	require.NotEmpty(t, captured)
	assert.NotContains(t, captured, "attacker-supplied-state")

	parts := strings.SplitN(captured, ".", 4)
	require.Len(t, parts, 4)
	state, ok := decodeSAMLSPStatePayload(parts[3])
	require.True(t, ok)
	assert.Equal(t, testOrgSlug, state.OrganizationSlug)
	assert.Equal(t, provider.ID, state.ProviderID)
	assert.NotEmpty(t, state.Nonce)
	assert.Equal(t, signer.issuedAt.Unix(), mustParseUnix(t, parts[1]))
}

func mustParseUnix(t *testing.T, value string) int64 {
	t.Helper()
	var unix int64
	_, err := fmt.Sscanf(value, "%d", &unix)
	require.NoError(t, err)
	return unix
}

// ── Legacy JSON /social/login SAML closure (SP-2) ──────────────────────────

func validSAMLSocialLoginCommand(t *testing.T, signer samlACSTestSigner, providerID string) SocialLoginCommand {
	t.Helper()
	return SocialLoginCommand{
		ProviderID:       providerID,
		Code:             "saml-response",
		State:            samlACSTestState(t, signer, testOrgSlug, providerID, "nonce-login"),
		OrganizationSlug: testOrgSlug,
		ApplicationID:    testAppID,
		IPAddress:        "203.0.113.10",
		UserAgent:        "test-agent",
	}
}

func samlSocialLoginFailure(t *testing.T, svc Service, command SocialLoginCommand) error {
	t.Helper()
	_, err := svc.LoginWithProvider(context.Background(), command)
	require.Error(t, err)
	require.Equal(t, apperrors.CodeUnauthorized, apperrors.FromError(err).Code)
	return err
}

func TestLoginWithProvider_SAMLRejectsMissingState(t *testing.T) {
	svc, _, _, _ := newSAMLACSService(t, samlACSTestProvider())
	samlSocialLoginFailure(t, svc, SocialLoginCommand{
		ProviderID:       samlACSTestProviderID,
		Code:             "saml-response",
		OrganizationSlug: testOrgSlug,
		ApplicationID:    testAppID,
	})
}

func TestLoginWithProvider_SAMLRejectsInvalidSignature(t *testing.T) {
	svc, _, _, signer := newSAMLACSService(t, samlACSTestProvider())
	command := validSAMLSocialLoginCommand(t, signer, samlACSTestProviderID)
	signer.reject = true
	svc.RelayStateSigner = signer
	samlSocialLoginFailure(t, svc, command)
}

func TestLoginWithProvider_SAMLRejectsExpiredState(t *testing.T) {
	svc, _, _, signer := newSAMLACSService(t, samlACSTestProvider())
	signer.issuedAt = svc.Clock.Now().UTC().Add(-11 * time.Minute)
	svc.RelayStateSigner = signer
	samlSocialLoginFailure(t, svc, validSAMLSocialLoginCommand(t, signer, samlACSTestProviderID))
}

func TestLoginWithProvider_SAMLRejectsCrossOrganizationState(t *testing.T) {
	svc, _, _, signer := newSAMLACSService(t, samlACSTestProvider())
	command := validSAMLSocialLoginCommand(t, signer, samlACSTestProviderID)
	command.State = samlACSTestState(t, signer, "other-org", samlACSTestProviderID, "nonce-login")
	samlSocialLoginFailure(t, svc, command)
}

func TestLoginWithProvider_SAMLRejectsCrossProviderState(t *testing.T) {
	svc, _, _, signer := newSAMLACSService(t, samlACSTestProvider())
	command := validSAMLSocialLoginCommand(t, signer, samlACSTestProviderID)
	command.State = samlACSTestState(t, signer, testOrgSlug, "01a00000-0000-7000-8000-0000000000b2", "nonce-login")
	samlSocialLoginFailure(t, svc, command)
}

func TestLoginWithProvider_SAMLRejectsReplayedState(t *testing.T) {
	original := exchangeSocialCode
	exchangeSocialCode = func(context.Context, SocialProviderConfig, string) (*SocialUserInfo, error) {
		return &SocialUserInfo{ID: "subject-1", Username: "saml-user", Email: "saml-user@example.com"}, nil
	}
	t.Cleanup(func() { exchangeSocialCode = original })

	svc, _, _, signer := newSAMLACSService(t, samlACSTestProvider())
	command := validSAMLSocialLoginCommand(t, signer, samlACSTestProviderID)

	_, err := svc.LoginWithProvider(context.Background(), command)
	require.NoError(t, err)

	samlSocialLoginFailure(t, svc, command)
}

func TestLoginWithProvider_SAMLValidStateCreatesSession(t *testing.T) {
	original := exchangeSocialCode
	exchangeSocialCode = func(context.Context, SocialProviderConfig, string) (*SocialUserInfo, error) {
		return &SocialUserInfo{
			ID:           "subject-1",
			Username:     "saml-user",
			Email:        "saml-user@example.com",
			AuthnContext: "urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport",
		}, nil
	}
	t.Cleanup(func() { exchangeSocialCode = original })

	svc, uow, _, signer := newSAMLACSService(t, samlACSTestProvider())
	result, err := svc.LoginWithProvider(context.Background(), validSAMLSocialLoginCommand(t, signer, samlACSTestProviderID))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.SessionID)

	session := uow.sessRepo.sessions[result.SessionID]
	require.NotNil(t, session)
	assert.Equal(t, loginMethodPassword, session.AuthenticationLevel)
	assert.Equal(t, loginMethodPassword, session.DeviceInfo["amr"])
}

func TestLoginWithProvider_NonSAMLDoesNotRequireSignedState(t *testing.T) {
	original := exchangeSocialCode
	exchangeSocialCode = func(context.Context, SocialProviderConfig, string) (*SocialUserInfo, error) {
		return &SocialUserInfo{ID: "oauth-subject", Username: "oauth-user", Email: "oauth-user@example.com"}, nil
	}
	t.Cleanup(func() { exchangeSocialCode = original })

	provider := samlACSTestProvider()
	provider.Category = providerdomain.CategoryOAuth
	provider.ProviderType = providerdomain.ProviderGitHub
	svc, uow, _, _ := newSAMLACSService(t, provider)
	// A rejecting signer and no replay store prove the SAML guard is not on the
	// OAuth path at all.
	svc.RelayStateSigner = samlACSTestSigner{reject: true}
	svc.SAMLReplayStore = nil

	result, err := svc.LoginWithProvider(context.Background(), SocialLoginCommand{
		ProviderID:       provider.ID,
		Code:             "oauth-code",
		State:            "plain-oauth-state",
		OrganizationSlug: testOrgSlug,
		ApplicationID:    testAppID,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.SessionID)
	require.NotNil(t, uow.sessRepo.sessions[result.SessionID])
}

func TestConsumeSAMLProviderState_NonSAMLIsNoOp(t *testing.T) {
	svc := Service{}
	cases := []*providerdomain.Provider{
		{Category: providerdomain.CategoryOAuth, ProviderType: providerdomain.ProviderGitHub},
		{Category: providerdomain.CategoryOIDC, ProviderType: providerdomain.ProviderOIDC},
		{Category: providerdomain.CategorySocial, ProviderType: providerdomain.ProviderWeChat},
	}
	for _, provider := range cases {
		require.NoError(t, svc.consumeSAMLProviderState(context.Background(), nil, provider, SocialLoginCommand{}))
	}
}
