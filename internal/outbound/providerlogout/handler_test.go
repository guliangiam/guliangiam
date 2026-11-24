package providerlogout

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
	providerlogoutevent "github.com/guliangiam/guliangiam/internal/platform/providerlogout"
	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

type fakeProviderLoader struct {
	provider *providerdomain.Provider
	err      error
	gotID    string
}

func (l *fakeProviderLoader) GetProvider(_ context.Context, providerID string) (*providerdomain.Provider, error) {
	l.gotID = providerID
	return l.provider, l.err
}

type fakeTokenDecrypter struct {
	sealed string
	token  string
	err    error
}

func (d *fakeTokenDecrypter) DecryptField(ciphertext string) (string, error) {
	d.sealed = ciphertext
	return d.token, d.err
}

func providerLogoutItem(t *testing.T, providerID, sealedToken string) *platformoutbox.OutboxItem {
	t.Helper()
	event, err := providerlogoutevent.NewEvent(providerlogoutevent.Request{
		OrganizationID:    "org-001",
		SessionID:         "sess-001",
		ProviderID:        providerID,
		ProviderType:      "OAuth",
		SealedAccessToken: sealedToken,
		RequestedAt:       time.Now().UTC(),
	})
	require.NoError(t, err)
	return &platformoutbox.OutboxItem{
		ID:            event.ID,
		AggregateType: event.AggregateType,
		AggregateID:   event.AggregateID,
		EventType:     event.EventType,
		EventVersion:  event.EventVersion,
		Payload:       event.Payload,
	}
}

func loopbackClient(timeout time.Duration) *http.Client {
	return ssrf.SafeHTTPClientWithAllowlist(timeout, []string{"127.0.0.0/8"})
}

// TestHandlePostsTokenToProviderLogout keeps every request-shape assertion the
// former synchronous test made (token, client credentials, custom headers).
func TestHandlePostsTokenToProviderLogout(t *testing.T) {
	type capture struct {
		token        string
		clientID     string
		clientSecret string
		header       string
	}
	seen := make(chan capture, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		seen <- capture{
			token:        r.Form.Get("token"),
			clientID:     r.Form.Get("client_id"),
			clientSecret: r.Form.Get("client_secret"),
			header:       r.Header.Get("X-Tenant"),
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	loader := &fakeProviderLoader{provider: &providerdomain.Provider{
		ID:              "provider-001",
		ClientID:        "client-id",
		ClientSecret:    "client-secret",
		CustomLogoutUrl: server.URL + "/logout",
		HttpHeaders:     map[string]string{"X-Tenant": "acme"},
	}}
	decrypter := &fakeTokenDecrypter{token: "upstream-token"}
	handler := NewHandler(loader, decrypter, loopbackClient(5*time.Second))

	require.NoError(t, handler.Handle(context.Background(), providerLogoutItem(t, "provider-001", "sealed-upstream-token")))

	got := <-seen
	assert.Equal(t, "upstream-token", got.token)
	assert.Equal(t, "client-id", got.clientID)
	assert.Equal(t, "client-secret", got.clientSecret)
	assert.Equal(t, "acme", got.header)
	assert.Equal(t, "provider-001", loader.gotID)
	assert.Equal(t, "sealed-upstream-token", decrypter.sealed)
}

func TestHandleSkipsProviderWithoutLogoutURL(t *testing.T) {
	handler := NewHandler(
		&fakeProviderLoader{provider: &providerdomain.Provider{ID: "provider-001"}},
		&fakeTokenDecrypter{token: "upstream-token"},
		nil,
	)
	require.NoError(t, handler.Handle(context.Background(), providerLogoutItem(t, "provider-001", "sealed")))
}

func TestHandleMarksMalformedPayloadPermanent(t *testing.T) {
	handler := NewHandler(&fakeProviderLoader{}, &fakeTokenDecrypter{}, nil)
	err := handler.Handle(context.Background(), &platformoutbox.OutboxItem{Payload: []byte("not-json")})
	require.Error(t, err)
	assert.True(t, platformoutbox.IsPermanent(err))
}

func TestHandleMarksMissingPayloadFieldsPermanent(t *testing.T) {
	handler := NewHandler(&fakeProviderLoader{}, &fakeTokenDecrypter{}, nil)
	err := handler.Handle(context.Background(), &platformoutbox.OutboxItem{Payload: []byte(`{"providerId":"","sealedAccessToken":""}`)})
	require.Error(t, err)
	assert.True(t, platformoutbox.IsPermanent(err))
}

func TestHandleMarksUnknownProviderPermanent(t *testing.T) {
	handler := NewHandler(
		&fakeProviderLoader{err: apperrors.NewNotFound("provider not found")},
		&fakeTokenDecrypter{token: "upstream-token"},
		nil,
	)
	err := handler.Handle(context.Background(), providerLogoutItem(t, "provider-001", "sealed"))
	require.Error(t, err)
	assert.True(t, platformoutbox.IsPermanent(err))
}

func TestHandleRetriesProviderReadFailure(t *testing.T) {
	handler := NewHandler(
		&fakeProviderLoader{err: apperrors.NewInternal("database unavailable")},
		&fakeTokenDecrypter{token: "upstream-token"},
		nil,
	)
	err := handler.Handle(context.Background(), providerLogoutItem(t, "provider-001", "sealed"))
	require.Error(t, err)
	assert.False(t, platformoutbox.IsPermanent(err), "transient provider read failures must be retried")
}

func TestHandleMarksDecryptFailurePermanent(t *testing.T) {
	handler := NewHandler(
		&fakeProviderLoader{provider: &providerdomain.Provider{ID: "provider-001", CustomLogoutUrl: "https://idp.example.com/logout"}},
		&fakeTokenDecrypter{err: apperrors.NewInternal("cannot open token")},
		nil,
	)
	err := handler.Handle(context.Background(), providerLogoutItem(t, "provider-001", "sealed"))
	require.Error(t, err)
	assert.True(t, platformoutbox.IsPermanent(err))
}

func TestHandleRetryClassification(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		permanent bool
	}{
		{name: "server error is retryable", status: http.StatusInternalServerError, permanent: false},
		{name: "throttling is retryable", status: http.StatusTooManyRequests, permanent: false},
		{name: "request timeout is retryable", status: http.StatusRequestTimeout, permanent: false},
		{name: "bad request is permanent", status: http.StatusBadRequest, permanent: true},
		{name: "gone is permanent", status: http.StatusGone, permanent: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer server.Close()

			handler := NewHandler(
				&fakeProviderLoader{provider: &providerdomain.Provider{
					ID:              "provider-001",
					CustomLogoutUrl: server.URL + "/logout",
				}},
				&fakeTokenDecrypter{token: "upstream-token"},
				loopbackClient(5*time.Second),
			)
			err := handler.Handle(context.Background(), providerLogoutItem(t, "provider-001", "sealed"))
			require.Error(t, err)
			assert.Equal(t, tc.permanent, platformoutbox.IsPermanent(err), "status %d", tc.status)
		})
	}
}

// TestHandleSlowProviderIsRetryable proves a slow provider fails inside the
// worker (and is retried) rather than inside a logout request.
func TestHandleSlowProviderIsRetryable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	handler := NewHandler(
		&fakeProviderLoader{provider: &providerdomain.Provider{
			ID:              "provider-001",
			CustomLogoutUrl: server.URL + "/logout",
		}},
		&fakeTokenDecrypter{token: "upstream-token"},
		&http.Client{Timeout: 50 * time.Millisecond},
	)
	err := handler.Handle(context.Background(), providerLogoutItem(t, "provider-001", "sealed"))
	require.Error(t, err)
	assert.False(t, platformoutbox.IsPermanent(err))
}
