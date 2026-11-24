package bootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-chi/chi/v5"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	wellknownmod "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/wellknown"
)

// advertisedEndpointFields are the discovery fields that carry a callable URL.
// Every one of them must resolve to a pattern the runtime router actually
// registers; a discovery document that publishes a path the router never
// mounted sends standard clients to a 401/403 instead of the protocol handler.
var advertisedEndpointFields = []string{
	"authorization_endpoint",
	"token_endpoint",
	"userinfo_endpoint",
	"jwks_uri",
	"registration_endpoint",
	"revocation_endpoint",
	"introspection_endpoint",
	"device_authorization_endpoint",
	"end_session_endpoint",
}

// TestDiscoveryEndpointsResolveToRegisteredRoutes is the regression guard for
// the drift where discovery advertised the root-level /oauth2/revoke and
// /oauth2/introspect paths while the handlers live under /api/v1/auth/oauth2/.
//
// It cross-checks all three construction sites against the same runtime route
// set the route contract gate uses, instead of restating the paths as
// literals: the app-layer document, the well-known fallback and the RFC 8414
// document each have to name a registered pattern.
func TestDiscoveryEndpointsResolveToRegisteredRoutes(t *testing.T) {
	router := NewChiRouter(&Runtime{})
	registered := map[string]bool{}
	if err := chi.Walk(router, func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		registered[normalizeRouteKey(pattern)] = true
		return nil
	}); err != nil {
		t.Fatalf("chi.Walk() error = %v", err)
	}

	const base = "https://issuer.example"
	appDocument, err := (&appauth.Service{}).GetOIDCDiscovery(context.Background(), base)
	if err != nil {
		t.Fatalf("GetOIDCDiscovery() error = %v", err)
	}

	documents := []struct {
		name      string
		endpoints map[string]string
	}{
		{
			name: "app-layer OIDC discovery",
			endpoints: map[string]string{
				"authorization_endpoint":        appDocument.AuthorizationEndpoint,
				"token_endpoint":                appDocument.TokenEndpoint,
				"userinfo_endpoint":             appDocument.UserinfoEndpoint,
				"jwks_uri":                      appDocument.JWKSEndpoint,
				"registration_endpoint":         appDocument.RegistrationEndpoint,
				"revocation_endpoint":           appDocument.RevocationEndpoint,
				"introspection_endpoint":        appDocument.IntrospectionEndpoint,
				"device_authorization_endpoint": appDocument.DeviceAuthorizationEndpoint,
				"end_session_endpoint":          appDocument.EndSessionEndpoint,
			},
		},
	}

	// Serve the discovery documents from the real wellknown handler. A bare chi
	// router keeps this test about endpoint derivation; the route set above
	// comes from the composed runtime router.
	discoveryRouter := chi.NewRouter()
	wellknownmod.RegisterChiRoutes(discoveryRouter, wellknownmod.Handler{IssuerURL: base})

	for _, path := range []string{
		"/.well-known/openid-configuration",
		"/.well-known/oauth-authorization-server",
	} {
		recorder := httptest.NewRecorder()
		discoveryRouter.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200: %s", path, recorder.Code, recorder.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatalf("GET %s body is not JSON: %v", path, err)
		}
		endpoints := map[string]string{}
		for _, field := range advertisedEndpointFields {
			if value, ok := payload[field].(string); ok && value != "" {
				endpoints[field] = value
			}
		}
		documents = append(documents, struct {
			name      string
			endpoints map[string]string
		}{name: path, endpoints: endpoints})
	}

	for _, document := range documents {
		if len(document.endpoints) == 0 {
			t.Fatalf("%s advertises no endpoint fields", document.name)
		}
		for field, rawURL := range document.endpoints {
			parsed, err := url.Parse(rawURL)
			if err != nil {
				t.Errorf("%s: %s=%q is not a URL: %v", document.name, field, rawURL, err)
				continue
			}
			if !registered[normalizeRouteKey(parsed.Path)] {
				t.Errorf("%s advertises %s=%s, but %s is not registered on the runtime router",
					document.name, field, rawURL, parsed.Path)
			}
		}
	}

	// The root-level OAuth2 face is deliberate and must stay reachable: the
	// userinfo route is mounted by wellknown.RegisterChiRoutes, not under
	// /api/v1/auth, so the guard must not mistake it for drift.
	if !registered["/oauth2/userinfo"] {
		t.Error("the runtime router no longer registers /oauth2/userinfo; discovery relies on it")
	}
	if registered["/oauth2/revoke"] || registered["/oauth2/introspect"] {
		t.Error("root-level /oauth2/revoke or /oauth2/introspect appeared; discovery must name the registered /api/v1/auth/oauth2/* paths")
	}
}
