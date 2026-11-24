package bootstrap

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	mw "github.com/guliangiam/guliangiam/internal/platform/middleware"
	platformmtls "github.com/guliangiam/guliangiam/internal/platform/mtls"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
)

// anonymousTokenEndpointPDP fails the test if Casbin is consulted: every request
// below targets the client-authenticated token surface, which the anonymous gate
// must answer before authorization runs.
type anonymousTokenEndpointPDP struct{ t *testing.T }

func (p anonymousTokenEndpointPDP) ResolveEnforcerUUID(_ context.Context, _ string) (string, error) {
	return "enforcer-anonymous", nil
}

func (p anonymousTokenEndpointPDP) Enforce(_ context.Context, _, _ string, _ ...interface{}) (bool, error) {
	p.t.Errorf("Casbin must not be consulted for client-authenticated token endpoints")
	return false, nil
}

// newAnonymousTokenEndpointHarness wraps the full v1 router in the production
// gates that decide reachability for a session-less caller: the PEP (anonymous
// gate) and CSRF (exemption predicate). The inner router still provides the real
// session middleware, routing, handlers and repositories.
func newAnonymousTokenEndpointHarness(t *testing.T) (chi.Router, http.Handler, *fakeApplicationRepository, *fakeTokenRepository) {
	t.Helper()
	router, _, _, applications, _, _, _, tokens := newTestV1Router(t)
	guarded := authtransport.APIAuthorizationMiddleware(anonymousTokenEndpointPDP{t: t}, nil, nil)(
		mw.CSRFTokenMiddlewareWithExemption(func(r *http.Request) bool {
			return authtransport.IsClientAuthenticatedProtocolEndpoint(r.Method, r.URL.Path)
		}, authtransport.HasValidatedBearerSession)(router),
	)
	return router, guarded, applications, tokens
}

func registerAnonymousMachineClients(t *testing.T, applications *fakeApplicationRepository) (*rsa.PrivateKey, *x509.Certificate) {
	t.Helper()

	secretHash := func(secret string) string {
		hash, err := platformcrypto.HashClientSecret(secret)
		if err != nil {
			t.Fatalf("hash client secret: %v", err)
		}
		return hash
	}
	base := func(id, clientID string) *applicationdomain.Application {
		return &applicationdomain.Application{
			ID:             id,
			OrganizationID: "org-1",
			Slug:           clientID,
			DisplayName:    clientID,
			Status:         "active",
			ClientID:       clientID,
			GrantTypes:     []string{"client_credentials"},
			CreatedAt:      fixedNow(),
			UpdatedAt:      fixedNow(),
			Revision:       1,
		}
	}

	basic := base("01a00000-0000-7000-8000-0000000000b1", "machine-basic")
	basic.ClientSecretHash = secretHash("basic-secret")
	basic.TokenEndpointAuthMethod = applicationdomain.TokenEndpointAuthMethodClientSecretBasic
	applications.items[basic.ID] = basic

	post := base("01a00000-0000-7000-8000-0000000000b2", "machine-post")
	post.ClientSecretHash = secretHash("post-secret")
	post.TokenEndpointAuthMethod = applicationdomain.TokenEndpointAuthMethodClientSecretPost
	applications.items[post.ID] = post

	jwtClient := base("01a00000-0000-7000-8000-0000000000b3", "machine-jwt")
	jwtClient.TokenEndpointAuthMethod = applicationdomain.TokenEndpointAuthMethodPrivateKeyJWT
	applications.items[jwtClient.ID] = jwtClient
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	jwkJSON, err := json.Marshal(map[string]string{
		"kty": "RSA",
		"use": "sig",
		"alg": "RS256",
		"kid": "machine-jwt-kid",
		"n":   base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes()),
	})
	if err != nil {
		t.Fatalf("marshal JWK: %v", err)
	}
	storeKey := fakeBootstrapClientSecretKey(jwtClient.OrganizationID, jwtClient.ID)
	fakeBootstrapClientJWKs[storeKey] = []*applicationdomain.ClientJWK{{
		ID:            "client-jwk-1",
		ApplicationID: jwtClient.ID,
		Kid:           "machine-jwt-kid",
		Alg:           "RS256",
		JWK:           string(jwkJSON),
		Status:        applicationdomain.ClientJWKStatusActive,
	}}
	t.Cleanup(func() { delete(fakeBootstrapClientJWKs, storeKey) })

	mtls := base("01a00000-0000-7000-8000-0000000000b4", "machine-mtls")
	mtls.TokenEndpointAuthMethod = applicationdomain.TokenEndpointAuthMethodSelfSignedTLSClientAuth
	certificate := generateSelfSignedCertificate(t, "machine-mtls")
	thumbprint, err := platformmtls.Thumbprint(certificate)
	if err != nil {
		t.Fatalf("certificate thumbprint: %v", err)
	}
	mtlsKey := fakeBootstrapClientSecretKey(mtls.OrganizationID, mtls.ID)
	notBefore := certificate.NotBefore
	notAfter := certificate.NotAfter
	fakeBootstrapClientCertificates[mtlsKey] = []*applicationdomain.ClientCertificate{{
		ID:                "client-cert-1",
		ApplicationID:     mtls.ID,
		OrganizationID:    mtls.OrganizationID,
		AuthMethod:        applicationdomain.TokenEndpointAuthMethodSelfSignedTLSClientAuth,
		FingerprintSHA256: thumbprint,
		NotBefore:         &notBefore,
		NotAfter:          &notAfter,
		Status:            applicationdomain.ClientCertificateStatusActive,
	}}
	t.Cleanup(func() { delete(fakeBootstrapClientCertificates, mtlsKey) })
	applications.items[mtls.ID] = mtls

	return privateKey, certificate
}

func generateSelfSignedCertificate(t *testing.T, commonName string) *x509.Certificate {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate certificate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    fixedNow().Add(-time.Hour),
		NotAfter:     fixedNow().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return certificate
}

func signAnonymousClientAssertion(t *testing.T, key *rsa.PrivateKey, clientID, audience string) string {
	t.Helper()
	header, err := json.Marshal(map[string]any{"alg": "RS256", "typ": "JWT", "kid": "machine-jwt-kid"})
	if err != nil {
		t.Fatalf("marshal assertion header: %v", err)
	}
	now := fixedNow()
	claims, err := json.Marshal(map[string]any{
		"iss": clientID,
		"sub": clientID,
		"aud": audience,
		"exp": now.Add(2 * time.Minute).Unix(),
		"iat": now.Unix(),
		"jti": "anonymous-" + clientID + "-" + now.Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatalf("marshal assertion claims: %v", err)
	}
	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign assertion: %v", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func postTokenForm(t *testing.T, handler http.Handler, form string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/exchange", strings.NewReader(form))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if mutate != nil {
		mutate(request)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// assertTokenHandlerReached fails when the response came from the session PEP or
// the CSRF double-submit check instead of the token handler, or when the client
// could not authenticate at all.
func assertTokenHandlerReached(t *testing.T, recorder *httptest.ResponseRecorder, mechanism string) {
	t.Helper()
	body := recorder.Body.String()
	if recorder.Code == http.StatusUnauthorized && strings.Contains(body, "authentication required") {
		t.Fatalf("%s: rejected by the session PEP before the token handler: %d %s", mechanism, recorder.Code, body)
	}
	if recorder.Code == http.StatusForbidden && strings.Contains(strings.ToLower(body), "csrf") {
		t.Fatalf("%s: rejected by CSRF before the token handler: %d %s", mechanism, recorder.Code, body)
	}
	if strings.Contains(body, `"invalid_client"`) {
		t.Fatalf("%s: client authentication did not succeed: %d %s", mechanism, recorder.Code, body)
	}
}

// assertUnsupportedClientCredentials asserts the handler refused the grant for
// the right reason: the client authenticated (no session PEP, no CSRF, no
// invalid_client) and client_credentials is reported as unsupported because no
// principal type can be persisted as a token subject yet.
func assertUnsupportedClientCredentials(t *testing.T, recorder *httptest.ResponseRecorder, mechanism string) {
	t.Helper()
	assertTokenHandlerReached(t, recorder, mechanism)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("%s: status = %d, want 400: %s", mechanism, recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"unsupported_grant_type"`) {
		t.Fatalf("%s: body must report unsupported_grant_type, got: %s", mechanism, recorder.Body.String())
	}
}

// TestTokenEndpointAcceptsAnonymousClients is the L4 regression the existing
// suite missed: every request below carries no session cookie, which is how a
// real OAuth2 client calls the token endpoint. client_credentials and mTLS
// clients must reach the handler and authenticate; the grant itself is
// answered with unsupported_grant_type until token subjects become principals
// (see the client_credentials commit and the Principal ADR).
func TestTokenEndpointAcceptsAnonymousClients(t *testing.T) {
	router, guarded, applications, _ := newAnonymousTokenEndpointHarness(t)
	privateKey, certificate := registerAnonymousMachineClients(t, applications)

	t.Run("client_secret_basic", func(t *testing.T) {
		recorder := postTokenForm(t, guarded, "grant_type=client_credentials&scope=orders%3Aread", func(r *http.Request) {
			r.SetBasicAuth("machine-basic", "basic-secret")
		})
		assertUnsupportedClientCredentials(t, recorder, "client_secret_basic")
	})

	t.Run("client_secret_post", func(t *testing.T) {
		recorder := postTokenForm(t, guarded,
			"grant_type=client_credentials&client_id=machine-post&client_secret=post-secret&scope=orders%3Aread", nil)
		assertUnsupportedClientCredentials(t, recorder, "client_secret_post")
	})

	t.Run("private_key_jwt", func(t *testing.T) {
		audience := "http://example.com/api/v1/auth/tokens/exchange"
		assertion := signAnonymousClientAssertion(t, privateKey, "machine-jwt", audience)
		recorder := postTokenForm(t, guarded,
			"grant_type=client_credentials&client_id=machine-jwt&client_assertion_type="+
				"urn%3Aietf%3Aparams%3Aoauth%3Aclient-assertion-type%3Ajwt-bearer&client_assertion="+assertion, nil)
		assertUnsupportedClientCredentials(t, recorder, "private_key_jwt")
	})

	t.Run("self_signed_tls_client_auth", func(t *testing.T) {
		recorder := postTokenForm(t, guarded, "grant_type=client_credentials&client_id=machine-mtls", func(r *http.Request) {
			r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{certificate}}
		})
		assertUnsupportedClientCredentials(t, recorder, "self_signed_tls_client_auth")
	})

	t.Run("none_with_pkce", func(t *testing.T) {
		verifier := "anonymous-pkce-verifier-1"
		sum := sha256.Sum256([]byte(verifier))
		challenge := base64.RawURLEncoding.EncodeToString(sum[:])

		// Step 1 is user-mediated: consent and authorize legitimately carry the
		// browser session (and are stopped by the password gate when gated).
		consentRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/consents/grant", strings.NewReader(
			`{"applicationId":"01a00000-0000-7000-8000-0000000000a1","grantedScopes":["profile"]}`,
		))
		consentRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
		consentRequest.Header.Set("Content-Type", "application/json")
		consentRecorder := httptest.NewRecorder()
		router.ServeHTTP(consentRecorder, consentRequest)
		if consentRecorder.Code != http.StatusOK {
			t.Fatalf("consent status = %d, body = %s", consentRecorder.Code, consentRecorder.Body.String())
		}

		authorizeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/authorize", strings.NewReader(
			`{"clientId":"portal-client","redirectUri":"https://portal.example.com/callback","responseType":"code",`+
				`"requestedScopes":["openid","profile","offline_access"],"state":"anonymous-state",`+
				`"codeChallenge":"`+challenge+`","codeChallengeMethod":"S256","resource":"https://api.example.com"}`,
		))
		authorizeRequest.AddCookie(&http.Cookie{Name: authtransport.SessionCookieName, Value: "session-1"})
		authorizeRequest.Header.Set("Content-Type", "application/json")
		authorizeRecorder := httptest.NewRecorder()
		router.ServeHTTP(authorizeRecorder, authorizeRequest)
		if authorizeRecorder.Code != http.StatusOK {
			t.Fatalf("authorize status = %d, body = %s", authorizeRecorder.Code, authorizeRecorder.Body.String())
		}
		var authorizeResponse struct {
			Code struct {
				Code string `json:"code"`
			} `json:"code"`
		}
		if err := json.Unmarshal(authorizeRecorder.Body.Bytes(), &authorizeResponse); err != nil {
			t.Fatalf("decode authorize response: %v", err)
		}
		if authorizeResponse.Code.Code == "" {
			t.Fatalf("authorize returned no code: %s", authorizeRecorder.Body.String())
		}

		// Step 2 is the client-to-AS exchange: no session cookie, no CSRF token.
		form := "grant_type=authorization_code&client_id=portal-client&code=" + authorizeResponse.Code.Code +
			"&redirect_uri=https%3A%2F%2Fportal.example.com%2Fcallback&code_verifier=" + verifier +
			"&resource=https%3A%2F%2Fapi.example.com"
		recorder := postTokenForm(t, guarded, form, nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("anonymous PKCE exchange status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		var exchangeResponse struct {
			AccessToken string `json:"accessToken"`
			TokenType   string `json:"tokenType"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &exchangeResponse); err != nil {
			t.Fatalf("decode exchange response: %v", err)
		}
		if !strings.HasPrefix(exchangeResponse.AccessToken, "gl_pat_") || exchangeResponse.TokenType != "Bearer" {
			t.Fatalf("unexpected exchange response: %s", recorder.Body.String())
		}
	})

	t.Run("unknown_client", func(t *testing.T) {
		recorder := postTokenForm(t, guarded, "grant_type=client_credentials", func(r *http.Request) {
			r.SetBasicAuth("ghost-client", "ghost-secret")
		})
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("unknown client status = %d, want 401: %s", recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), "invalid_client") {
			t.Fatalf("unknown client must reach the token handler's invalid_client, got: %s", recorder.Body.String())
		}
	})
}

func postProtocolForm(t *testing.T, handler http.Handler, path, form string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if mutate != nil {
		mutate(request)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// TestRevocationAndIntrospectionReachClientsWithoutSession is the reachability
// regression for the second half of the fix: RFC 7009 revocation and RFC 7662
// introspection authenticate the caller as a client, so a session-less request
// must reach the handler instead of stopping at the session PEP or CSRF.
//
// The expected answers follow the implemented RFC semantics: revocation always
// replies 200 (RFC 7009 §2.2) and introspection replies 200 with active=false
// for a token it cannot attribute to the calling client (RFC 7662 §2.2).
func TestRevocationAndIntrospectionReachClientsWithoutSession(t *testing.T) {
	_, guarded, applications, tokens := newAnonymousTokenEndpointHarness(t)
	registerAnonymousMachineClients(t, applications)

	const tokenValue = "gl_pat_anonymous_revoke"
	const tokenID = "token-anonymous-1"
	tokens.items[tokenID] = &tokendomain.Token{
		ID:             tokenID,
		UserID:         "user-1",
		Username:       "jane",
		OrganizationID: "org-1",
		ApplicationID:  "01a00000-0000-7000-8000-0000000000b1",
		TokenHash:      hashTokenValueForTest(tokenValue),
		Scopes:         []string{"orders:read"},
		ExpiresAt:      fixedNow().Add(time.Hour),
		CreatedAt:      fixedNow(),
		UpdatedAt:      fixedNow(),
		Revision:       1,
	}

	introspect := func(t *testing.T, clientID, clientSecret string) *httptest.ResponseRecorder {
		t.Helper()
		return postProtocolForm(t, guarded, "/api/v1/auth/oauth2/introspect", "token="+tokenValue,
			func(r *http.Request) { r.SetBasicAuth(clientID, clientSecret) })
	}

	// A live token introspects active with no session cookie in play.
	recorder := introspect(t, "machine-basic", "basic-secret")
	if recorder.Code != http.StatusOK {
		t.Fatalf("anonymous introspect status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"active":true`) {
		t.Fatalf("anonymous introspect must report the live token active, got: %s", recorder.Body.String())
	}

	// Revocation without a session: client_secret_basic is the whole credential.
	recorder = postProtocolForm(t, guarded, "/api/v1/auth/oauth2/revoke", "token="+tokenValue,
		func(r *http.Request) { r.SetBasicAuth("machine-basic", "basic-secret") })
	if recorder.Code != http.StatusOK {
		t.Fatalf("anonymous revoke status = %d, want 200 (RFC 7009): %s", recorder.Code, recorder.Body.String())
	}
	if tokens.items[tokenID].RevokedAt == nil {
		t.Fatal("anonymous revoke reached the handler but did not revoke the token")
	}

	// The revoked token is now inactive for the same client.
	recorder = introspect(t, "machine-basic", "basic-secret")
	if !strings.Contains(recorder.Body.String(), `"active":false`) {
		t.Fatalf("revoked token must introspect inactive, got: %s", recorder.Body.String())
	}

	// Unknown client: revocation answers 401 invalid_client (RFC 7009 §2.2.1) and
	// introspection stays 200 active=false (RFC 7662 permits either; implemented
	// semantics keep the token-response byte pattern non-enumerable).
	recorder = postProtocolForm(t, guarded, "/api/v1/auth/oauth2/revoke", "token="+tokenValue,
		func(r *http.Request) { r.SetBasicAuth("ghost-client", "ghost-secret") })
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unknown client revoke status = %d, want 401 invalid_client: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "invalid_client") {
		t.Fatalf("unknown client revoke body must be invalid_client, got: %s", recorder.Body.String())
	}
	recorder = introspect(t, "ghost-client", "ghost-secret")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unknown client introspect status = %d, want 401 invalid_client: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "invalid_client") {
		t.Fatalf("unknown client introspect body must be invalid_client, got: %s", recorder.Body.String())
	}
}
