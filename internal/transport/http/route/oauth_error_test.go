package route

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func decodeBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	body := map[string]any{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v (body=%q)", err, recorder.Body.String())
	}
	return body
}

func TestWriteOAuthError_Envelope(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteOAuthError(recorder, http.StatusBadRequest, OAuthErrInvalidGrant, "code expired")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	body := decodeBody(t, recorder)
	if body["error"] != "invalid_grant" {
		t.Errorf("error = %v, want invalid_grant", body["error"])
	}
	if body["error_description"] != "code expired" {
		t.Errorf("error_description = %v, want %q", body["error_description"], "code expired")
	}
	// The protocol namespace must never leak the application error envelope.
	if _, ok := body["error_code"]; ok {
		t.Errorf("unexpected application error field in protocol body: %v", body)
	}
}

func TestWriteOAuthError_OmitsEmptyDescription(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteOAuthError(recorder, http.StatusBadRequest, OAuthErrInvalidRequest, "")

	body := decodeBody(t, recorder)
	if _, ok := body["error_description"]; ok {
		t.Errorf("error_description must be omitted when empty, got %v", body)
	}
}

func TestWriteProtocolError_MapsApplicationCodes(t *testing.T) {
	cases := []struct {
		name         string
		err          error
		wantStatus   int
		wantCode     string
		wantChalleng bool
	}{
		{"invalid_argument", apperrors.NewInvalidArgument("bad scope"), http.StatusBadRequest, "invalid_request", false},
		{"not_found", apperrors.NewNotFound("no such client"), http.StatusNotFound, "invalid_request", false},
		{"unauthorized", apperrors.NewUnauthorized("no token"), http.StatusUnauthorized, "invalid_token", true},
		{"forbidden", apperrors.NewForbidden("scope missing"), http.StatusForbidden, "insufficient_scope", false},
		{"account_locked", apperrors.New(apperrors.CodeAccountLocked, "locked"), 423, "invalid_grant", false},
		{"too_many_requests", apperrors.New(apperrors.CodeTooManyRequests, "slow down"), http.StatusTooManyRequests, "temporarily_unavailable", false},
		{"internal", apperrors.NewInternal("boom"), http.StatusInternalServerError, "server_error", false},
		{"unavailable", apperrors.NewUnavailable("db down"), http.StatusServiceUnavailable, "temporarily_unavailable", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/exchange", http.NoBody)
			WriteProtocolError(recorder, request, tc.err)

			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tc.wantStatus)
			}

			body := decodeBody(t, recorder)
			if body["error"] != tc.wantCode {
				t.Errorf("error = %v, want %s", body["error"], tc.wantCode)
			}

			challenge := recorder.Header().Get("WWW-Authenticate") != ""
			if challenge != tc.wantChalleng {
				t.Errorf("WWW-Authenticate present = %v, want %v", challenge, tc.wantChalleng)
			}
		})
	}
}

func TestWriteProtocolError_DoesNotLeakInternalMessage(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/exchange", http.NoBody)
	WriteProtocolError(recorder, request, apperrors.NewInternal("SELECT * FROM guliangiam.users failed"))

	body := decodeBody(t, recorder)
	description, _ := body["error_description"].(string)
	if description == "" {
		t.Fatal("error_description must not be empty")
	}
	if description == "SELECT * FROM guliangiam.users failed" {
		t.Errorf("internal message leaked into the protocol body: %q", description)
	}
}

func TestWriteProtocolError_TooManyRequestsSetsRetryAfter(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/oauth2/register", http.NoBody)
	WriteProtocolError(recorder, request, apperrors.New(apperrors.CodeTooManyRequests, "slow down"))

	if recorder.Header().Get("Retry-After") == "" {
		t.Error("429 protocol response must carry Retry-After")
	}
}

func TestWriteError_DispatchesBySurface(t *testing.T) {
	t.Run("protocol face uses the standard envelope", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/exchange", http.NoBody)
		request = request.WithContext(WithRequestMetadata(request.Context(), RequestMetadata{
			Surface: SurfaceProtocol,
		}))
		WriteError(recorder, request, apperrors.NewInvalidArgument("bad code"))

		body := decodeBody(t, recorder)
		if body["error"] != "invalid_request" {
			t.Errorf("error = %v, want invalid_request", body["error"])
		}
		if _, ok := body["requestId"]; ok {
			t.Errorf("protocol body must not carry the application envelope: %v", body)
		}
	})

	t.Run("management face keeps the application envelope", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users", http.NoBody)
		request = request.WithContext(WithRequestMetadata(request.Context(), RequestMetadata{
			Surface: SurfaceManagement,
		}))
		WriteError(recorder, request, apperrors.NewInvalidArgument("bad sortBy"))

		body := decodeBody(t, recorder)
		envelope, ok := body["error"].(map[string]any)
		if !ok {
			t.Fatalf("management error body must nest the error object, got %v", body)
		}
		if envelope["code"] != "invalid_argument" {
			t.Errorf("code = %v, want invalid_argument", envelope["code"])
		}
	})
}

func TestWriteSCIMError_EnvelopeAndMediaType(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteSCIMError(recorder, http.StatusConflict, "user already exists")

	if got := recorder.Header().Get("Content-Type"); got != "application/scim+json" {
		t.Errorf("Content-Type = %q, want application/scim+json", got)
	}

	body := decodeBody(t, recorder)
	schemas, ok := body["schemas"].([]any)
	if !ok || len(schemas) != 1 || schemas[0] != scimErrorSchema {
		t.Errorf("schemas = %v, want [%s]", body["schemas"], scimErrorSchema)
	}
	if body["detail"] != "user already exists" {
		t.Errorf("detail = %v", body["detail"])
	}
	if body["status"] != "409" {
		t.Errorf("status = %v, want \"409\"", body["status"])
	}
}

func TestOAuthErrorFrom_NonApplicationError(t *testing.T) {
	status, code, description := OAuthErrorFrom(errors.New("not an app error"))
	if status != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", status)
	}
	if code != OAuthErrServerError {
		t.Errorf("code = %v, want server_error", code)
	}
	if description == "" {
		t.Error("description must not be empty")
	}
}

func TestSurfaceFor_ProtocolAuthEndpoints(t *testing.T) {
	protocol := []string{
		"/api/v1/auth/authorize",
		"/api/v1/auth/tokens/exchange",
		"/api/v1/auth/oauth2/revoke",
		"/api/v1/auth/cas/login",
		"/api/v1/auth/saml/metadata/{orgSlug}/{appSlug}",
	}
	for _, pattern := range protocol {
		if got := SurfaceFor(pattern); got != SurfaceProtocol {
			t.Errorf("SurfaceFor(%q) = %s, want protocol", pattern, got)
		}
	}

	authentication := []string{
		"/api/v1/auth/login",
		"/api/v1/auth/mfa/push/verify",
		"/api/v1/auth/password/reset",
	}
	for _, pattern := range authentication {
		if got := SurfaceFor(pattern); got != SurfaceAuthentication {
			t.Errorf("SurfaceFor(%q) = %s, want authentication", pattern, got)
		}
	}
}

// protocolCodedError is the shape the application layer uses for RFC codes that
// have no counterpart in the application code table (错误码注册表 §1). It exists in
// the test only to prove the writer honours the interface.
type protocolCodedError struct {
	code    string
	message string
}

func (e protocolCodedError) Error() string        { return e.message }
func (e protocolCodedError) ProtocolCode() string { return e.code }
func (e protocolCodedError) HTTPStatus() int      { return http.StatusBadRequest }

// TestOAuthErrorFrom_PrefersProtocolCode pins the rule introduced with the
// implicit-flow retirement: `unsupported_response_type` has no
// application equivalent, so the code an error carries wins over the mapping
// table.
func TestOAuthErrorFrom_PrefersProtocolCode(t *testing.T) {
	err := protocolCodedError{code: string(OAuthErrUnsupportedResponseType), message: "unsupported response type: token"}

	status, code, description := OAuthErrorFrom(err)
	if status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", status)
	}
	if code != OAuthErrUnsupportedResponseType {
		t.Errorf("code = %s, want %s", code, OAuthErrUnsupportedResponseType)
	}
	if description != "unsupported response type: token" {
		t.Errorf("description = %q", description)
	}
}

func TestStatusCodeFromError_UsesProtocolStatus(t *testing.T) {
	if got := StatusCodeFromError(protocolCodedError{code: string(OAuthErrUnsupportedResponseType)}); got != http.StatusBadRequest {
		t.Fatalf("StatusCodeFromError() = %d, want 400", got)
	}
}

func TestWriteError_ProtocolCodedError(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/authorize", http.NoBody)
	request = request.WithContext(WithRequestMetadata(request.Context(), RequestMetadata{
		Surface: SurfaceProtocol,
	}))

	WriteError(recorder, request, protocolCodedError{code: string(OAuthErrUnsupportedResponseType), message: "unsupported response type: token"})

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
	body := decodeBody(t, recorder)
	if body["error"] != "unsupported_response_type" {
		t.Errorf("error = %v, want unsupported_response_type", body["error"])
	}
	if _, ok := body["requestId"]; ok {
		t.Errorf("protocol body must not carry the application envelope: %v", body)
	}
}

// A WWW-Authenticate value is a quoted-string (RFC 9110 §5.6.4). An unescaped
// quote ends the value early, and a bare CR/LF makes the header invalid — both
// change the challenge the client sees without any error being raised.
func TestWriteAuthChallenge_QuotesAndEscapesParams(t *testing.T) {
	cases := []struct {
		name   string
		params []ChallengeParam
		want   string
	}{
		{"plain", []ChallengeParam{{Name: "error", Value: "invalid_token"}}, `Bearer error="invalid_token"`},
		{"quote is escaped", []ChallengeParam{{Name: "error", Value: `say "hi"`}}, `Bearer error="say \"hi\""`},
		{"backslash is escaped", []ChallengeParam{{Name: "error", Value: `a\b`}}, `Bearer error="a\\b"`},
		{"crlf is truncated", []ChallengeParam{{Name: "error", Value: "line1\r\ninjected"}}, `Bearer error="line1"`},
		{"empty value is omitted", []ChallengeParam{{Name: "error", Value: ""}}, `Bearer`},
		{"empty value is not emitted quoted", []ChallengeParam{{Name: "error", Value: "  "}}, `Bearer`},
		{"order is preserved", []ChallengeParam{
			{Name: "error", Value: "invalid_token"},
			{Name: "scope", Value: "openid"},
		}, `Bearer error="invalid_token", scope="openid"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			WriteAuthChallenge(recorder, ChallengeSchemeBearer, tc.params...)
			if got := recorder.Header().Get("WWW-Authenticate"); got != tc.want {
				t.Errorf("WWW-Authenticate = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWriteTokenChallenge_CarriesSchemeDescriptionAndScope(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteTokenChallenge(recorder, ChallengeSchemeBearer, OAuthErrInsufficientScope, "scope missing", "openid profile")

	want := `Bearer error="insufficient_scope", error_description="scope missing", scope="openid profile"`
	if got := recorder.Header().Get("WWW-Authenticate"); got != want {
		t.Errorf("WWW-Authenticate = %q, want %q", got, want)
	}
}

// The DPoP nonce challenge (14.27, [Target]) issues the same auth-params under a
// different scheme; the scheme must be the caller's, not a `Bearer` literal.
func TestWriteTokenChallenge_HonoursScheme(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteTokenChallenge(recorder, ChallengeSchemeDPoP, OAuthErrUseDPoPNonce, "", "")

	if got := recorder.Header().Get("WWW-Authenticate"); got != `DPoP error="use_dpop_nonce"` {
		t.Errorf("WWW-Authenticate = %q, want DPoP challenge", got)
	}
}

func TestWriteProtocolError_ChallengeCarriesDescription(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", http.NoBody)
	WriteProtocolError(recorder, request, apperrors.NewUnauthorized("token is missing"))

	challenge := recorder.Header().Get("WWW-Authenticate")
	if !strings.Contains(challenge, `error="invalid_token"`) {
		t.Errorf("challenge = %q, want invalid_token", challenge)
	}
	if !strings.Contains(challenge, `error_description="token is missing"`) {
		t.Errorf("challenge = %q must carry the description", challenge)
	}
}

func TestSafeProtocolDescription(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"4xx keeps the message", apperrors.NewInvalidArgument("bad scope"), "bad scope"},
		{"5xx is replaced", apperrors.NewInternal("SELECT 1"), "the authorization server encountered an unexpected condition"},
		{"429 is a 4xx and keeps its message", apperrors.New(apperrors.CodeTooManyRequests, "quota"), "quota"},
		{"4xx is truncated to one line", apperrors.NewInvalidArgument("first\r\nsecond"), "first"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SafeProtocolDescription(tc.err); got != tc.want {
				t.Errorf("SafeProtocolDescription() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWriteResourceError_KeepsCallerCode(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/oauth2/userinfo", http.NoBody)
	_ = request

	WriteResourceError(recorder, apperrors.NewForbidden("scope missing"), OAuthErrInvalidToken, http.StatusUnauthorized)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
	body := decodeBody(t, recorder)
	// The service said forbidden, but a protected resource answers 401
	// invalid_token (RFC 6750 §3.1); the caller's code wins.
	if body["error"] != "invalid_token" {
		t.Errorf("error = %v, want invalid_token", body["error"])
	}
	challenge := recorder.Header().Get("WWW-Authenticate")
	if !strings.Contains(challenge, `error="invalid_token"`) {
		t.Errorf("challenge = %q, want invalid_token", challenge)
	}
}

func TestWriteSCIMErrorWithType_ExplicitOverridesStatusMapping(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteSCIMErrorWithType(recorder, http.StatusBadRequest, "count exceeds the maximum of 100", "tooMany")

	body := decodeBody(t, recorder)
	if body["scimType"] != "tooMany" {
		t.Errorf("scimType = %v, want tooMany", body["scimType"])
	}

	recorder = httptest.NewRecorder()
	WriteSCIMErrorWithType(recorder, http.StatusBadRequest, "bad value", "")
	body = decodeBody(t, recorder)
	if body["scimType"] != "invalidValue" {
		t.Errorf("scimType = %v, want invalidValue fallback", body["scimType"])
	}
}
