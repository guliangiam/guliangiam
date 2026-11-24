package faceid

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// LocalUniFaceVerifier implements FaceVerifier against a self-hosted UniFace
// HTTP service (the same /v1/compare contract Casdoor's "Local UniFace" provider
// speaks).
//
// The service performs face comparison only; it has no liveness endpoint.
// VerifyLiveness therefore fails closed with `unimplemented` instead of
// reporting a pass, so a policy that requires liveness cannot be silently
// bypassed by selecting this provider.
type LocalUniFaceVerifier struct {
	Endpoint   string
	APIKey     string
	HTTPClient *http.Client
}

// NewLocalUniFaceVerifier creates a verifier. A blank endpoint fails at call
// time instead of construction so an operator can pre-configure the record
// before the local service is rolled out.
func NewLocalUniFaceVerifier(endpoint, apiKey string) *LocalUniFaceVerifier {
	return &LocalUniFaceVerifier{
		Endpoint:   strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		APIKey:     strings.TrimSpace(apiKey),
		HTTPClient: ssrf.SafeHTTPClient(30 * time.Second),
	}
}

// VerifyLiveness fails closed: the local service has no liveness endpoint.
func (v *LocalUniFaceVerifier) VerifyLiveness(_ context.Context, _, _ string) (*LivenessResult, error) {
	return nil, apperrors.NewUnimplemented("local uniface provider does not implement liveness detection")
}

type localUniFaceCompareRequest struct {
	ImageA string `json:"imageA"`
	ImageB string `json:"imageB"`
}

type localUniFaceCompareResponse struct {
	Matched bool    `json:"matched"`
	Score   float64 `json:"score"`
	Reason  string  `json:"reason"`
	Detail  string  `json:"detail"`
}

// CompareFace posts both images to the service and reports the match verdict.
func (v *LocalUniFaceVerifier) CompareFace(ctx context.Context, imageData, storedTemplate string) (*CompareResult, error) {
	if v.Endpoint == "" {
		return nil, apperrors.NewInvalidArgument("local uniface endpoint is empty")
	}

	body, err := json.Marshal(localUniFaceCompareRequest{ImageA: imageData, ImageB: storedTemplate})
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "local uniface encode request")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.Endpoint+"/v1/compare", bytes.NewReader(body))
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "local uniface compare request")
	}
	req.Header.Set("Content-Type", "application/json")
	if v.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+v.APIKey)
	}

	client := v.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "local uniface compare")
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "local uniface read response")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, apperrors.NewInternalf("local uniface compare failed with status %d: %s", resp.StatusCode, string(responseBody))
	}

	var result localUniFaceCompareResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, apperrors.WrapInternalf(err, "local uniface compare decode")
	}
	return &CompareResult{IsMatch: result.Matched, Confidence: result.Score}, nil
}

// compile-time assertion that the local verifier satisfies the shared port.
var _ FaceVerifier = (*LocalUniFaceVerifier)(nil)
