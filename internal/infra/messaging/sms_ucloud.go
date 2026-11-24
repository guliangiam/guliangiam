package messaging

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func NewUCloudSmsSender(publicKey, privateKey, projectID, signName, templateID string) *UCloudSmsSender {
	return &UCloudSmsSender{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		ProjectID:  projectID,
		SignName:   signName,
		TemplateID: templateID,
		client:     ssrf.SafeHTTPClient(10 * time.Second),
	}
}

// Send sends an SMS via UCLOUD USMS API.
func (s *UCloudSmsSender) Send(ctx context.Context, phone, code string) error {
	apiURL := "https://api.ucloud.cn/"

	params := url.Values{}
	params.Set("Action", "SendUSMSMessage")
	params.Set("PublicKey", s.PublicKey)
	params.Set("ProjectId", s.ProjectID)
	params.Set("SigContent", s.SignName)
	params.Set("TemplateId", s.TemplateID)
	params.Set("PhoneNumbers.0", phone)
	params.Set("TemplateParams.0", code)
	params.Set("Signature", ucloudSignature(params, s.PrivateKey))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, http.NoBody)
	if err != nil {
		return apperrors.WrapInternalf(err, "ucloud: create request")
	}
	req.URL.RawQuery = params.Encode()

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "ucloud: request")
	}
	defer resp.Body.Close()

	var result struct {
		RetCode int    `json:"RetCode"`
		Action  string `json:"Action"`
		Message string `json:"Message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return apperrors.WrapInternalf(err, "ucloud: decode response")
	}

	// USMS returns RetCode 0 on success; non-zero carries Message, even with HTTP 200.
	if result.RetCode != 0 {
		return apperrors.NewInternalf("ucloud: API error %d: %s", result.RetCode, result.Message)
	}
	if resp.StatusCode >= 400 {
		return apperrors.NewInternalf("ucloud: API error (status %d)", resp.StatusCode)
	}
	return nil
}

// ucloudSignature implements the UCloud API signature algorithm:
// sort parameters by key, concatenate key+value pairs (no escaping), append
// the PrivateKey, and take the lowercase hex SHA256 of the result.
// https://docs.ucloud.cn/api/usms-api/send_usms_message
func ucloudSignature(params url.Values, privateKey string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf strings.Builder
	for _, k := range keys {
		buf.WriteString(k)
		buf.WriteString(params.Get(k))
	}
	seed := buf.String() + privateKey
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

// Ensure UCloudSmsSender implements SmsSender.
var _ SmsSender = (*UCloudSmsSender)(nil)

// EmaySmsSender sends SMS via Emay (亿美短信).
type EmaySmsSender struct {
	APIURL       string
	SerialNumber string
	Password     string
	SignName     string
	client       *http.Client
}

// NewEmaySmsSender creates a new Emay SMS sender.
