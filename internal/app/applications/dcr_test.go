package applications

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestValidateDCRRequest(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		err := validateDCRRequest(DCRRequest{
			RedirectURIs: []string{"https://example.com/cb"},
			GrantTypes:   []string{"authorization_code"},
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("missing_redirect_uris", func(t *testing.T) {
		err := validateDCRRequest(DCRRequest{})
		assertAppError(t, err, apperrors.CodeInvalidArgument)
	})
	t.Run("invalid_redirect_uri", func(t *testing.T) {
		err := validateDCRRequest(DCRRequest{
			RedirectURIs: []string{"not-a-url"},
		})
		assertAppError(t, err, apperrors.CodeUnprocessableEntity)
	})
	t.Run("wildcard_redirect_uri_unusable", func(t *testing.T) {
		err := validateDCRRequest(DCRRequest{
			RedirectURIs: []string{"https://*.example.com/cb"},
		})
		assertAppError(t, err, apperrors.CodeUnprocessableEntity)
	})
	t.Run("unsupported_grant_type", func(t *testing.T) {
		err := validateDCRRequest(DCRRequest{
			RedirectURIs: []string{"https://example.com/cb"},
			GrantTypes:   []string{"unsupported"},
		})
		appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
		if appErr.Meta["field"] != "grant_types" || appErr.Meta["allowed"] == nil {
			t.Fatalf("meta = %v, want field=grant_types + allowed", appErr.Meta)
		}
	})
	t.Run("unsupported_auth_method", func(t *testing.T) {
		err := validateDCRRequest(DCRRequest{
			RedirectURIs:            []string{"https://example.com/cb"},
			TokenEndpointAuthMethod: "invalid",
		})
		// RFC 7591 §3.2.2: metadata failures are 400 invalid_client_metadata.
		var metadataErr interface {
			HTTPStatus() int
			ProtocolCode() string
		}
		if !errors.As(err, &metadataErr) {
			t.Fatalf("err = %v, want invalid_client_metadata", err)
		}
		if metadataErr.HTTPStatus() != http.StatusBadRequest || metadataErr.ProtocolCode() != "invalid_client_metadata" {
			t.Fatalf("status/code = %d/%s", metadataErr.HTTPStatus(), metadataErr.ProtocolCode())
		}
	})

	t.Run("client_secret_jwt_rejected_as_metadata_error", func(t *testing.T) {
		err := validateDCRRequest(DCRRequest{
			RedirectURIs:            []string{"https://example.com/cb"},
			TokenEndpointAuthMethod: "client_secret_jwt",
		})
		var metadataErr interface {
			HTTPStatus() int
			ProtocolCode() string
		}
		if !errors.As(err, &metadataErr) {
			t.Fatalf("err = %v, want invalid_client_metadata", err)
		}
		if metadataErr.HTTPStatus() != http.StatusBadRequest || metadataErr.ProtocolCode() != "invalid_client_metadata" {
			t.Fatalf("status/code = %d/%s", metadataErr.HTTPStatus(), metadataErr.ProtocolCode())
		}
	})

	t.Run("mtls_methods_rejected_without_certificate_channel", func(t *testing.T) {
		// RFC 7591 has no client certificate metadata, so DCR cannot maintain
		// app_client_certificates. Accepting these would register a client that
		// can never authenticate at the token endpoint.
		for _, method := range []string{"tls_client_auth", "self_signed_tls_client_auth"} {
			err := validateDCRRequest(DCRRequest{
				RedirectURIs:            []string{"https://example.com/cb"},
				TokenEndpointAuthMethod: method,
			})
			var metadataErr interface {
				HTTPStatus() int
				ProtocolCode() string
			}
			if !errors.As(err, &metadataErr) {
				t.Fatalf("%s: err = %v, want invalid_client_metadata", method, err)
			}
			if metadataErr.HTTPStatus() != http.StatusBadRequest || metadataErr.ProtocolCode() != "invalid_client_metadata" {
				t.Fatalf("%s: status/code = %d/%s", method, metadataErr.HTTPStatus(), metadataErr.ProtocolCode())
			}
		}
	})

	t.Run("none_requires_authorization_code_for_pkce", func(t *testing.T) {
		err := validateDCRRequest(DCRRequest{
			RedirectURIs:            []string{"https://example.com/cb"},
			TokenEndpointAuthMethod: "none",
			GrantTypes:              []string{"client_credentials"},
		})
		var metadataErr interface {
			HTTPStatus() int
			ProtocolCode() string
		}
		if !errors.As(err, &metadataErr) || metadataErr.ProtocolCode() != "invalid_client_metadata" {
			t.Fatalf("err = %v, want invalid_client_metadata", err)
		}
	})
	t.Run("invalid_client_uri", func(t *testing.T) {
		err := validateDCRRequest(DCRRequest{
			RedirectURIs: []string{"https://example.com/cb"},
			ClientURI:    "not-a-url",
		})
		assertAppError(t, err, apperrors.CodeUnprocessableEntity)
	})
}

func TestApplicationToDCRResponse(t *testing.T) {
	now := time.Now()
	app := &domain.Application{
		ClientID:                "cid1",
		RedirectURIs:            []string{"https://example.com/cb"},
		GrantTypes:              []string{"authorization_code"},
		TokenEndpointAuthMethod: "client_secret_basic",
		DisplayName:             "My App",
		HomepageURL:             "https://example.com",
		CreatedAt:               now,
	}
	resp := applicationToDCRResponse(app)
	if resp.ClientID != "cid1" {
		t.Fatalf("got %s", resp.ClientID)
	}
	if resp.ClientName != "My App" {
		t.Fatalf("got %s", resp.ClientName)
	}
	if resp.ClientIDIssuedAt != now.Unix() {
		t.Fatalf("got %d", resp.ClientIDIssuedAt)
	}
}

func TestGenerateOAuthClientID(t *testing.T) {
	id, err := generateOAuthClientID()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(id, "gl_cid_") {
		t.Fatalf("expected gl_cid_ prefix, got: %s", id)
	}
	if len(id) <= len("gl_cid_") {
		t.Fatalf("entropy portion is empty, id=%s", id)
	}
}

func TestGenerateOAuthClientSecret(t *testing.T) {
	secret, err := generateOAuthClientSecret()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(secret, "gl_cs_") {
		t.Fatalf("expected gl_cs_ prefix, got: %s", secret)
	}
	if len(secret) <= len("gl_cs_") {
		t.Fatalf("entropy portion is empty, secret=%s", secret)
	}
}

// dcrRegistrationService builds a Service whose organization lookup can resolve
// the "built-in" slug, so registration reaches the validation under test.
func dcrRegistrationService(repo *mockApplicationRepo) Service {
	return Service{
		Clock: fixedApplicationClock{},
		UUID:  fixedApplicationUUID{},
		UOW: appTestUOW{
			appRepo: repo,
			orgRepo: &appTestOrgRepo{orgs: []*orgdomain.Organization{{ID: "org-1", Slug: "built-in"}}},
		},
	}
}

// DCR writes into the same app_oauth_config.redirect_uris VARCHAR(255)[] column
// as the console, so the same ceilings have to hold before the transaction is
// opened: a registration rejected only by the INSERT would surface as a 500
// with no field the caller can fix.
func TestDCRRegistration_AcceptsRedirectURIsAtColumnLimits(t *testing.T) {
	repo := &mockApplicationRepo{}
	service := dcrRegistrationService(repo)

	response, err := service.RegisterDynamicClient(context.Background(), DCRRequest{
		ClientName:       "Limit Client",
		RedirectURIs:     redirectURIsOfLength(maxRedirectURIs, maxRedirectURIEntryLength),
		OrganizationSlug: "built-in",
	})
	if err != nil {
		t.Fatalf("registration with %d redirect URIs of %d characters error = %v", maxRedirectURIs, maxRedirectURIEntryLength, err)
	}
	if len(response.RedirectURIs) != maxRedirectURIs {
		t.Fatalf("redirect_uris = %d, want %d", len(response.RedirectURIs), maxRedirectURIs)
	}
	if len(repo.apps) != 1 {
		t.Fatalf("registered clients = %d, want 1", len(repo.apps))
	}
}

func TestDCRRegistration_DefaultsTokenSigningMethodToRS256(t *testing.T) {
	repo := &mockApplicationRepo{}
	service := dcrRegistrationService(repo)

	if _, err := service.RegisterDynamicClient(context.Background(), DCRRequest{
		ClientName:       "N8a Client",
		RedirectURIs:     []string{"https://app.example/callback"},
		OrganizationSlug: "built-in",
	}); err != nil {
		t.Fatalf("registration error = %v", err)
	}
	if len(repo.apps) != 1 {
		t.Fatalf("registered clients = %d, want 1", len(repo.apps))
	}
	if repo.apps[0].TokenSigningMethod != domain.TokenSigningMethodRS256 {
		t.Fatalf("persisted TokenSigningMethod = %q, want %q", repo.apps[0].TokenSigningMethod, domain.TokenSigningMethodRS256)
	}
}

// private_key_jwt authenticates with the embedded JWKS; registering without one
// would mint a client that can never pass the token endpoint. The update path
// already enforced this, registration did not.
func TestDCRRegistration_RejectsPrivateKeyJWTWithoutJWKS(t *testing.T) {
	repo := &mockApplicationRepo{}
	service := dcrRegistrationService(repo)

	_, err := service.RegisterDynamicClient(context.Background(), DCRRequest{
		ClientName:              "PKJ Client",
		RedirectURIs:            []string{"https://app.example/callback"},
		TokenEndpointAuthMethod: "private_key_jwt",
		OrganizationSlug:        "built-in",
	})
	var metadataErr interface {
		HTTPStatus() int
		ProtocolCode() string
	}
	if !errors.As(err, &metadataErr) || metadataErr.ProtocolCode() != "invalid_client_metadata" {
		t.Fatalf("err = %v, want invalid_client_metadata", err)
	}
	if len(repo.apps) != 0 {
		t.Fatalf("rejected registration still wrote %d applications", len(repo.apps))
	}
}

func TestDCRRegistration_AcceptsPrivateKeyJWTWithJWKS(t *testing.T) {
	repo := &mockApplicationRepo{}
	service := dcrRegistrationService(repo)

	jwks := jwksDocument(t, publicRSAJWK(t, jwksTestRSAKey(t), "kid-1"))
	if _, err := service.RegisterDynamicClient(context.Background(), DCRRequest{
		ClientName:              "PKJ Client",
		RedirectURIs:            []string{"https://app.example/callback"},
		TokenEndpointAuthMethod: "private_key_jwt",
		OrganizationSlug:        "built-in",
		JWKS:                    json.RawMessage(jwks),
	}); err != nil {
		t.Fatalf("registration error = %v", err)
	}
	if len(repo.apps) != 1 {
		t.Fatalf("registered clients = %d, want 1", len(repo.apps))
	}
	if repo.apps[0].TokenEndpointAuthMethod != domain.TokenEndpointAuthMethodPrivateKeyJWT {
		t.Fatalf("persisted method = %q", repo.apps[0].TokenEndpointAuthMethod)
	}
}

func TestDCRRegistration_RejectsRedirectURIsBeyondColumnLimits(t *testing.T) {
	cases := []struct {
		name      string
		redirects []string
		wantMeta  map[string]any
	}{
		{
			name:      "too_many_entries",
			redirects: redirectURIsOfLength(maxRedirectURIs+1, 0),
			wantMeta:  map[string]any{metaKeyField: "redirect_uris", metaKeyMax: maxRedirectURIs},
		},
		{
			name:      "entry_longer_than_column",
			redirects: redirectURIsOfLength(1, maxRedirectURIEntryLength+1),
			wantMeta:  map[string]any{metaKeyField: "redirect_uris", metaKeyIndex: 0, metaKeyMax: maxRedirectURIEntryLength},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &mockApplicationRepo{}
			service := dcrRegistrationService(repo)

			_, err := service.RegisterDynamicClient(context.Background(), DCRRequest{
				RedirectURIs:     testCase.redirects,
				OrganizationSlug: "built-in",
			})
			appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
			for key, want := range testCase.wantMeta {
				if appErr.Meta[key] != want {
					t.Fatalf("meta[%q] = %v, want %v (meta = %+v)", key, appErr.Meta[key], want, appErr.Meta)
				}
			}
			if len(repo.apps) != 0 {
				t.Fatalf("rejected registration still wrote %d applications", len(repo.apps))
			}
		})
	}
}
