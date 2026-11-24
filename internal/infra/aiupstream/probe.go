package aiupstream

import (
	"context"
	"errors"
	"net/http"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Probe issues an authenticated, read-only request against the provider's
// model catalogue. It reuses the adapter's own HTTP plumbing (base URL,
// credential header/query, SSRF-checked client, timeout), so a management
// plane connection test traverses the same credential path as Chat and
// Embedding calls.
func (a *OpenAICompatAdapter) Probe(ctx context.Context) (int, error) {
	req, err := a.buildHTTPRequest(ctx, http.MethodGet, "/models", nil)
	if err != nil {
		return 0, apperrors.NewInvalidArgument("openai_compat: probe request construction failed")
	}
	return doProbe(a.httpClient, req, "openai_compat")
}

func (a *AnthropicMessagesAdapter) Probe(ctx context.Context) (int, error) {
	req, err := a.buildHTTPRequest(ctx, http.MethodGet, "/v1/models", nil)
	if err != nil {
		return 0, apperrors.NewInvalidArgument("anthropic: probe request construction failed")
	}
	return doProbe(a.httpClient, req, "anthropic")
}

func (a *GeminiGenerateContentAdapter) Probe(ctx context.Context) (int, error) {
	req, err := a.buildHTTPRequest(ctx, http.MethodGet, geminiAPIPrefix, nil)
	if err != nil {
		// 构造失败时不回显底层错误：Gemini 的 URL 在构造前已带上 ?key=。
		return 0, apperrors.NewInvalidArgument("gemini: probe request construction failed")
	}
	return doProbe(a.httpClient, req, "gemini")
}

// doProbe executes the request and converts transport/upstream failures into
// credential-free platform errors. The raw error is deliberately not wrapped:
// *url.Error embeds the full request URL, which for Gemini carries the
// plaintext key in the query string.
func doProbe(client *http.Client, req *http.Request, family string) (int, error) {
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return 0, apperrors.NewUnavailablef("%s: probe cancelled or timed out", family)
		}
		return 0, apperrors.NewUnavailablef("%s: probe request failed", family)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return resp.StatusCode, probeStatusError(family, resp.StatusCode)
	}
	return resp.StatusCode, nil
}

// probeStatusError maps an upstream status to a typed error without echoing
// the response body, which an upstream could otherwise use to reflect the
// credential back into logs or API responses.
func probeStatusError(family string, status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusPaymentRequired:
		return apperrors.NewUnauthorizedf("%s: probe rejected credentials (upstream %d)", family, status)
	case http.StatusForbidden:
		return apperrors.NewForbiddenf("%s: probe forbidden (upstream %d)", family, status)
	case http.StatusTooManyRequests:
		return apperrors.NewTooManyRequestsf("%s: probe rate limited (upstream %d)", family, status)
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return apperrors.NewInvalidArgumentf("%s: probe rejected (upstream %d)", family, status)
	default:
		return apperrors.NewUnavailablef("%s: probe failed (upstream %d)", family, status)
	}
}
