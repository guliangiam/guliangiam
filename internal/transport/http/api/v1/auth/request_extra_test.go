package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeTokenRevocationRequest_Form(t *testing.T) {
	form := url.Values{}
	form.Set("token", "abc123")
	form.Set("token_type_hint", "access_token")
	form.Set("client_id", "myapp")
	form.Set("client_secret", "secret")

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	cmd := decodeTokenRevocationRequest(r)
	assert.Equal(t, "abc123", cmd.Token)
	assert.Equal(t, "access_token", cmd.TokenTypeHint)
	assert.Equal(t, "myapp", cmd.ClientID)
	assert.Equal(t, "secret", cmd.ClientSecret)
}

func TestDecodeTokenRevocationRequest_BasicAuth(t *testing.T) {
	form := url.Values{}
	form.Set("token", "abc123")

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.SetBasicAuth("basicapp", "basicsecret")

	cmd := decodeTokenRevocationRequest(r)
	assert.Equal(t, "abc123", cmd.Token)
	assert.Equal(t, "basicapp", cmd.ClientID)
	assert.Equal(t, "basicsecret", cmd.ClientSecret)
}

func TestDecodeTokenRevocationRequest_EmptyForm(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	cmd := decodeTokenRevocationRequest(r)
	assert.Equal(t, "", cmd.Token)
}

func TestDecodeTokenIntrospectionRFCRequest_Form(t *testing.T) {
	form := url.Values{}
	form.Set("token", "abc123")
	form.Set("token_type_hint", "access_token")
	form.Set("client_id", "myapp")
	form.Set("client_secret", "secret")

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	cmd := decodeTokenIntrospectionRFCRequest(r)
	assert.Equal(t, "abc123", cmd.Token)
	assert.Equal(t, "access_token", cmd.TokenTypeHint)
	assert.Equal(t, "myapp", cmd.ClientID)
	assert.Equal(t, "secret", cmd.ClientSecret)
}

func TestDecodeTokenIntrospectionRFCRequest_BasicAuth(t *testing.T) {
	form := url.Values{}
	form.Set("token", "abc123")

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.SetBasicAuth("basicapp", "basicsecret")

	cmd := decodeTokenIntrospectionRFCRequest(r)
	assert.Equal(t, "abc123", cmd.Token)
	assert.Equal(t, "basicapp", cmd.ClientID)
	assert.Equal(t, "basicsecret", cmd.ClientSecret)
}

func TestDecodeTokenIntrospectionRFCRequest_BasicAuthOverridesForm(t *testing.T) {
	form := url.Values{}
	form.Set("token", "abc123")
	form.Set("client_id", "formapp")

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.SetBasicAuth("basicapp", "basicsecret")

	cmd := decodeTokenIntrospectionRFCRequest(r)
	// Form client_id is not empty, so BasicAuth doesn't override it
	assert.Equal(t, "formapp", cmd.ClientID)
}
