package idv

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// maxResponseBytes caps provider response bodies. Identity providers return
// small JSON documents; anything larger is treated as a protocol error so a
// hostile endpoint cannot exhaust memory.
const maxResponseBytes = 1 << 20

// defaultTimeout bounds every outbound identity-provider call.
const defaultTimeout = 15 * time.Second

func safeClient() *http.Client {
	return ssrf.SafeHTTPClient(defaultTimeout)
}

// readLimited reads at most maxResponseBytes and rejects an oversized body.
func readLimited(resp *http.Response) ([]byte, error) {
	limited := io.LimitReader(resp.Body, maxResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "read provider response")
	}
	if len(body) > maxResponseBytes {
		return nil, apperrors.NewUnavailable("provider response exceeds the 1MiB limit")
	}
	return body, nil
}

// aliyunRPCErrorCode extracts Code/Message from an Aliyun RPC error body so a
// 4xx status keeps its diagnostic code instead of being flattened to "not
// found" (Aliyun answers access-key failures with HTTP 404).
func aliyunRPCErrorCode(body []byte) (string, string) {
	var payload struct {
		Code    string `json:"Code"`
		Message string `json:"Message"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", ""
	}
	return strings.TrimSpace(payload.Code), strings.TrimSpace(payload.Message)
}

// mapProviderHTTPError translates a provider HTTP status into the platform
// error taxonomy. The upstream body is never echoed verbatim: provider error
// payloads can contain request identifiers and personal data.
func mapProviderHTTPError(provider string, status int) error {
	switch {
	case status == http.StatusBadRequest:
		return apperrors.NewInvalidArgumentf("%s rejected the request (400)", provider)
	case status == http.StatusUnauthorized:
		return apperrors.NewUnauthorizedf("%s rejected the credential (401)", provider)
	case status == http.StatusForbidden:
		return apperrors.NewForbiddenf("%s denied access (403)", provider)
	case status == http.StatusNotFound:
		return apperrors.NewNotFoundf("%s resource not found (404)", provider)
	case status == http.StatusConflict:
		return apperrors.NewConflictf("%s reported a conflict (409)", provider)
	case status == http.StatusTooManyRequests:
		return apperrors.NewTooManyRequestsf("%s rate limited the request (429)", provider)
	case status >= 500:
		return apperrors.NewUnavailablef("%s is unavailable (%d)", provider, status)
	default:
		return apperrors.NewUnavailablef("%s returned an unexpected status %d", provider, status)
	}
}

// decodeJSON decodes a provider response with unknown fields allowed and
// rejects trailing data so a broken payload cannot pass silently.
func decodeJSON(provider string, body []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(out); err != nil {
		return apperrors.WrapInvalidArgumentf(err, "%s returned invalid JSON", provider)
	}
	if decoder.More() {
		return apperrors.NewInvalidArgumentf("%s returned trailing JSON content", provider)
	}
	return nil
}

func headerValue(headers map[string]string, name string) string {
	for key, value := range headers {
		if strings.EqualFold(key, name) {
			return value
		}
	}
	return ""
}

func isSuccessStatus(status int) bool {
	return status >= 200 && status < 300
}

func newJSONRequest(ctx context.Context, method, url string, body []byte) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	return req, nil
}
