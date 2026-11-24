package verifications

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appverifications "github.com/guliangiam/guliangiam/internal/app/verifications"
	verificationsdomain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// BE-A-03: verify endpoint must use the trusted client IP, never raw XFF
// ---------------------------------------------------------------------------

type notFoundVerificationRepo struct {
	verificationsdomain.Repository
}

func (notFoundVerificationRepo) GetActiveByRecipient(context.Context, string, string) (*verificationsdomain.Verification, error) {
	return nil, apperrors.NewNotFound("verification not found")
}

type verifyTestUoW struct {
	appuow.AuthUoW
}

func (verifyTestUoW) Verifications() verificationsdomain.Repository {
	return notFoundVerificationRepo{}
}

// Rotating X-Forwarded-For from an untrusted socket peer must not reset the
// per-IP attempt budget: every request below carries a fresh XFF and a fresh
// recipient, so only the socket IP can tie them together. Pre-fix the handler
// counted each spoofed XFF as a new IP and never returned 429.
func TestVerifyCode_RotatingXFFCannotEscapeIPBudget(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "")

	limiter := appverifications.NewRateLimiter()
	t.Cleanup(limiter.Stop)
	handler := Handler{Service: appverifications.Service{
		UOW:         verifyTestUoW{},
		RateLimiter: limiter,
	}}
	router := testRouter()
	RegisterChiRoutes(router, handler)

	for i := 0; i < 10; i++ {
		body := fmt.Sprintf(`{"type":"email","recipient":"user%d@example.com","code":"000000"}`, i)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verification-code/verify", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", i+1))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Result().StatusCode, "attempt %d should be an invalid-code 400", i+1)
	}

	body := `{"type":"email","recipient":"final@example.com","code":"000000"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verification-code/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.254")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Result().StatusCode,
		"11th attempt from the same socket must hit the per-IP budget despite rotating XFF")
}

// ---------------------------------------------------------------------------
// BE-A-04: send endpoint stacks IP + recipient budgets
// ---------------------------------------------------------------------------

// The nil-service handler panics inside the use case (recovered to 500), but
// the stacked rate limiters run first: once a bucket is empty the response is
// 429 regardless of handler outcome.
func TestSendVerificationCode_RecipientBudgetAcrossRotatingIPs(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "")
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verification-code/send",
			bytes.NewBufferString(`{"type":"email","recipient":"victim@example.com"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = fmt.Sprintf("198.51.100.%d:40000", i+1)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusInternalServerError, rec.Result().StatusCode, "send %d should reach the (nil) handler", i+1)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verification-code/send",
		bytes.NewBufferString(`{"type":"email","recipient":"victim@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.254:40000"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Result().StatusCode,
		"6th send to the same recipient must be denied even from a fresh IP")
}

func TestSendVerificationCode_IPBudgetAcrossRotatingXFF(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "")
	router := testRouter()
	RegisterChiRoutes(router, newHandlerWithNilService())

	for i := 0; i < 5; i++ {
		body := fmt.Sprintf(`{"type":"email","recipient":"user%d@example.com"}`, i)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verification-code/send", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", i+1))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusInternalServerError, rec.Result().StatusCode, "send %d should reach the (nil) handler", i+1)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verification-code/send",
		bytes.NewBufferString(`{"type":"email","recipient":"final@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.254")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Result().StatusCode,
		"6th send from the same socket must be denied despite rotating XFF and recipients")
}

// ---------------------------------------------------------------------------
// sendRecipientRateLimitKey must peek without consuming the body
// ---------------------------------------------------------------------------

func TestSendRecipientRateLimitKey_PreservesBody(t *testing.T) {
	payload := `{"type":"email","recipient":"  User@Example.com "}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verification-code/send", bytes.NewBufferString(payload))

	key := sendRecipientRateLimitKey(req)
	assert.Equal(t, "verification-send:email:user@example.com", key)

	rest, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	assert.Equal(t, payload, string(rest), "handler must still decode the exact original body")
}

func TestSendRecipientRateLimitKey_InvalidBodyFallsBackToIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verification-code/send", bytes.NewBufferString(`not-json`))
	assert.Empty(t, sendRecipientRateLimitKey(req))
	rest, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	assert.Equal(t, "not-json", string(rest))
}
