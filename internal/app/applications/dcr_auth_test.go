package applications

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// These tests pin RFC 7591 §2.3 / §3.2.1 as implemented for 设计报告:
// the read/update/delete operations on a dynamically registered client are
// authenticated with the registration_access_token issued at registration.
// Before this, all three were unauthenticated — anyone who knew a client_id
// could read, rewrite or delete somebody else's registered client.

// dcrTestService builds a Service over an in-memory application repository that
// holds one dynamically registered client.
func dcrTestService(t *testing.T, registrationToken string) Service {
	t.Helper()

	// The registration token hash lives outside the wide application row (it is
	// written by a narrow statement), so the fake keeps it in a side map — the
	// same shape as the real schema.
	repo := &mockApplicationRepo{registrationHashes: map[string]string{}}
	repo.apps = append(repo.apps, &domain.Application{
		ID:             "app-1",
		OrganizationID: "org-1",
		Slug:           "dcr-abc",
		DisplayName:    "DCR Client",
		Category:       "dcr",
		Status:         "active",
		ClientID:       "client-abc",
		RedirectURIs:   []string{"https://rp.example.com/cb"},
		GrantTypes:     []string{"authorization_code"},
		CreatedAt:      time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC),
	})
	repo.registrationHashes["client-abc"] = platformcrypto.HashTokenValue(registrationToken)

	service := Service{
		Clock: fixedApplicationClock{},
		UUID:  fixedApplicationUUID{},
		UOW:   appTestUOW{appRepo: repo, orgRepo: &appTestOrgRepo{}},
	}
	return service
}

type fixedApplicationClock struct{}

func (fixedApplicationClock) Now() time.Time {
	return time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
}

type fixedApplicationUUID struct{}

func (fixedApplicationUUID) NewString() (string, error) { return "generated-id", nil }

func TestDCR_ManagementRequiresRegistrationAccessToken(t *testing.T) {
	const token = "gl_rat_correct-token"
	service := dcrTestService(t, token)

	cases := []struct {
		name string
		call func() error
	}{
		{
			name: "read without token",
			call: func() error {
				_, err := service.GetDynamicClient(context.Background(), "client-abc", "")
				return err
			},
		},
		{
			name: "read with wrong token",
			call: func() error {
				_, err := service.GetDynamicClient(context.Background(), "client-abc", "gl_rat_wrong")
				return err
			},
		},
		{
			name: "update without token",
			call: func() error {
				_, err := service.UpdateDynamicClient(context.Background(), "client-abc", "", DCRRequest{RedirectURIs: []string{"https://rp.example.com/cb"}})
				return err
			},
		},
		{
			name: "update with wrong token",
			call: func() error {
				_, err := service.UpdateDynamicClient(context.Background(), "client-abc", "gl_rat_wrong", DCRRequest{RedirectURIs: []string{"https://rp.example.com/cb"}})
				return err
			},
		},
		{
			name: "delete without token",
			call: func() error {
				return service.DeleteDynamicClient(context.Background(), "client-abc", "")
			},
		},
		{
			name: "delete with wrong token",
			call: func() error {
				return service.DeleteDynamicClient(context.Background(), "client-abc", "gl_rat_wrong")
			},
		},
		{
			name: "unknown client id",
			call: func() error {
				_, err := service.GetDynamicClient(context.Background(), "no-such-client", token)
				return err
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.call()
			if err == nil {
				t.Fatal("expected the call to be rejected")
			}
			appErr, ok := err.(*apperrors.Error)
			if !ok || appErr.Code != apperrors.CodeUnauthorized {
				t.Fatalf("error = %v, want unauthorized", err)
			}
		})
	}
}

// TestDCR_UnknownClientDoesNotLeakExistence pins the enumeration guard: a wrong
// client_id and a wrong token must be indistinguishable, or the endpoint tells
// an attacker which clients exist.
func TestDCR_UnknownClientDoesNotLeakExistence(t *testing.T) {
	const token = "gl_rat_correct-token"
	service := dcrTestService(t, token)

	_, unknownErr := service.GetDynamicClient(context.Background(), "no-such-client", token)
	_, wrongTokenErr := service.GetDynamicClient(context.Background(), "client-abc", "gl_rat_wrong")

	unknownAppErr, ok := unknownErr.(*apperrors.Error)
	if !ok {
		t.Fatalf("unknown client error = %v", unknownErr)
	}
	wrongTokenAppErr, ok := wrongTokenErr.(*apperrors.Error)
	if !ok {
		t.Fatalf("wrong token error = %v", wrongTokenErr)
	}

	if unknownAppErr.Code != wrongTokenAppErr.Code || unknownAppErr.Message != wrongTokenAppErr.Message {
		t.Fatalf("responses differ and leak existence: %v vs %v", unknownAppErr, wrongTokenAppErr)
	}
}

func TestDCR_ManagementSucceedsWithTheIssuedToken(t *testing.T) {
	const token = "gl_rat_correct-token"
	service := dcrTestService(t, token)
	ctx := context.Background()

	response, err := service.GetDynamicClient(ctx, "client-abc", token)
	if err != nil {
		t.Fatalf("GetDynamicClient() error = %v", err)
	}
	if response.ClientID != "client-abc" {
		t.Fatalf("client_id = %q", response.ClientID)
	}
	// The management URI is how a client finds its own record again (RFC 7591 §3.2.1).
	if response.RegistrationClientURI != "/oauth2/register/client-abc" {
		t.Fatalf("registration_client_uri = %q", response.RegistrationClientURI)
	}

	if err := service.DeleteDynamicClient(ctx, "client-abc", token); err != nil {
		t.Fatalf("DeleteDynamicClient() error = %v", err)
	}
}

// TestDCRUpdate_InheritedMTLSRequiresRegisteredCertificate pins the update half
// of the mTLS closure: an update that omits token_endpoint_auth_method inherits
// the current method, so a client already using mTLS must still have a matching
// certificate registered through the console. Explicitly switching to mTLS is
// rejected earlier by validateDCRRequest for both register and update.
func TestDCRUpdate_InheritedMTLSRequiresRegisteredCertificate(t *testing.T) {
	const token = "gl_rat_correct-token"
	fakeClientCertificateStore = map[string][]*domain.ClientCertificate{}
	t.Cleanup(func() { fakeClientCertificateStore = map[string][]*domain.ClientCertificate{} })

	service := dcrTestService(t, token)
	repo := service.UOW.(appTestUOW).appRepo
	repo.apps[0].TokenEndpointAuthMethod = domain.TokenEndpointAuthMethodTLSClientAuth

	request := DCRRequest{RedirectURIs: []string{"https://rp.example.com/cb"}}

	// A certificate bound to the other mTLS method cannot authenticate the
	// inherited tls_client_auth method, so it must not satisfy the check.
	fakeClientCertificateStore[fakeClientSecretKey("org-1", "app-1")] = []*domain.ClientCertificate{
		{
			ID:             "cert-mismatch",
			ApplicationID:  "app-1",
			OrganizationID: "org-1",
			AuthMethod:     domain.TokenEndpointAuthMethodSelfSignedTLSClientAuth,
			Status:         domain.ClientCertificateStatusActive,
		},
	}

	_, err := service.UpdateDynamicClient(context.Background(), "client-abc", token, request)
	var metadataErr interface {
		HTTPStatus() int
		ProtocolCode() string
	}
	if !errors.As(err, &metadataErr) {
		t.Fatalf("update without a matching certificate error = %v, want invalid_client_metadata", err)
	}
	if metadataErr.HTTPStatus() != http.StatusBadRequest || metadataErr.ProtocolCode() != "invalid_client_metadata" {
		t.Fatalf("status/code = %d/%s", metadataErr.HTTPStatus(), metadataErr.ProtocolCode())
	}

	fakeClientCertificateStore[fakeClientSecretKey("org-1", "app-1")] = []*domain.ClientCertificate{
		{
			ID:             "cert-1",
			ApplicationID:  "app-1",
			OrganizationID: "org-1",
			AuthMethod:     domain.TokenEndpointAuthMethodTLSClientAuth,
			Status:         domain.ClientCertificateStatusActive,
		},
	}

	response, err := service.UpdateDynamicClient(context.Background(), "client-abc", token, request)
	if err != nil {
		t.Fatalf("update with a registered certificate error = %v", err)
	}
	if response.TokenEndpointAuthMethod != domain.TokenEndpointAuthMethodTLSClientAuth {
		t.Fatalf("token_endpoint_auth_method = %q", response.TokenEndpointAuthMethod)
	}
}

// TestDCRUpdate_SecretMethodRequiresExistingSecret pins the sibling of the mTLS
// guard: the DCR update request carries no secret material, so switching a
// client to client_secret_basic/post is only valid when the client already holds
// a usable secret. Without the check the endpoint persists a client that can
// never authenticate at the token endpoint.
func TestDCRUpdate_SecretMethodRequiresExistingSecret(t *testing.T) {
	const token = "gl_rat_correct-token"
	fakeClientSecretStore = map[string][]*domain.ClientSecret{}
	t.Cleanup(func() { fakeClientSecretStore = map[string][]*domain.ClientSecret{} })

	service := dcrTestService(t, token)

	request := DCRRequest{
		RedirectURIs:            []string{"https://rp.example.com/cb"},
		TokenEndpointAuthMethod: domain.TokenEndpointAuthMethodClientSecretBasic,
	}

	_, err := service.UpdateDynamicClient(context.Background(), "client-abc", token, request)
	var metadataErr interface {
		HTTPStatus() int
		ProtocolCode() string
	}
	if !errors.As(err, &metadataErr) {
		t.Fatalf("update without a stored secret error = %v, want invalid_client_metadata", err)
	}
	if metadataErr.HTTPStatus() != http.StatusBadRequest || metadataErr.ProtocolCode() != "invalid_client_metadata" {
		t.Fatalf("status/code = %d/%s", metadataErr.HTTPStatus(), metadataErr.ProtocolCode())
	}

	fakeClientSecretStore[fakeClientSecretKey("org-1", "app-1")] = []*domain.ClientSecret{
		{
			ID:            "secret-1",
			ApplicationID: "app-1",
			Status:        domain.ClientSecretStatusActive,
		},
	}

	response, err := service.UpdateDynamicClient(context.Background(), "client-abc", token, request)
	if err != nil {
		t.Fatalf("update with a usable secret error = %v", err)
	}
	if response.TokenEndpointAuthMethod != domain.TokenEndpointAuthMethodClientSecretBasic {
		t.Fatalf("token_endpoint_auth_method = %q", response.TokenEndpointAuthMethod)
	}
}

// TestDCR_RegistrationIssuesRegistrationAccessToken covers the issuing half:
// without it the client has no credential and the endpoints are unreachable.
func TestDCR_RegistrationIssuesRegistrationAccessToken(t *testing.T) {
	repo := &mockApplicationRepo{registrationHashes: map[string]string{}}
	service := Service{
		Clock: fixedApplicationClock{},
		UUID:  fixedApplicationUUID{},
		UOW: appTestUOW{
			appRepo: repo,
			orgRepo: &appTestOrgRepo{orgs: []*orgdomain.Organization{{ID: "org-1", Slug: "built-in"}}},
		},
	}

	response, err := service.RegisterDynamicClient(context.Background(), DCRRequest{
		ClientName:       "New RP",
		RedirectURIs:     []string{"https://new-rp.example.com/cb"},
		OrganizationSlug: "built-in",
	})
	if err != nil {
		t.Fatalf("RegisterDynamicClient() error = %v", err)
	}

	if len(response.RegistrationAccessToken) < len("gl_rat_")+32 {
		t.Fatalf("registration_access_token looks too short: %q", response.RegistrationAccessToken)
	}
	if response.RegistrationClientURI == "" {
		t.Fatal("registration_client_uri is empty")
	}

	// The plaintext must not be stored: the repository holds only its hash.
	if len(repo.registrationHashes) != 1 {
		t.Fatalf("expected exactly one stored registration hash, got %d", len(repo.registrationHashes))
	}
	for clientID, stored := range repo.registrationHashes {
		if stored == response.RegistrationAccessToken {
			t.Fatal("registration_access_token was stored in plaintext")
		}
		if stored != platformcrypto.HashTokenValue(response.RegistrationAccessToken) {
			t.Fatalf("stored hash does not match the issued token for %s", clientID)
		}
	}

	// The issued credential must actually work, or registration produced a
	// client nobody can manage.
	if _, err := service.GetDynamicClient(context.Background(), response.ClientID, response.RegistrationAccessToken); err != nil {
		t.Fatalf("GetDynamicClient() with the issued token error = %v", err)
	}
}
