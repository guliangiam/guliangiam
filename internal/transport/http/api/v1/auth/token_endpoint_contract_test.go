package auth

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GAP-006: the token endpoint the discovery document publishes must be a route
// chi actually registered. Comparing the parsed path against the walked route
// table — rather than against another string literal — keeps the declaration and
// the registration from drifting apart in a future rename.
func TestDiscoveryTokenEndpointMatchesRegisteredChiRoute(t *testing.T) {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	RegisterChiRoutes(router, Handler{})

	registered := make(map[string]struct{})
	require.NoError(t, chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		registered[method+" "+route] = struct{}{}
		return nil
	}))

	document, err := (&appauth.Service{}).GetOIDCDiscovery(context.Background(), "https://issuer.example.com")
	require.NoError(t, err)

	parsed, err := url.Parse(document.TokenEndpoint)
	require.NoError(t, err)
	assert.Equal(t, appauth.TokenEndpointPath, parsed.Path)
	_, ok := registered[http.MethodPost+" "+parsed.Path]
	assert.True(t, ok, "discovery advertises %s but chi did not register it; routes=%v",
		parsed.Path, registered)
}
