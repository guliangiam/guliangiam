package idv

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

const (
	// tencentIDVProviderName is the provider registration key for Tencent
	// Cloud 人脸核身（慧眼 faceid）.
	tencentIDVProviderName = "tencent_idv"
	// tencentIDVDefaultEndpoint is the official API domain of 人脸核身 faceid.
	tencentIDVDefaultEndpoint = "https://faceid.tencentcloudapi.com"
	// tencentIDVService is the credential-scope service name used by the
	// TC3-HMAC-SHA256 (signature v3) algorithm.
	tencentIDVService = "faceid"
	// tencentIDVAPIVersion is the current faceid API version.
	tencentIDVAPIVersion = "2018-03-01"
	// tencentIDVSignatureHeader carries the shared-secret webhook signature
	// injected by the customer gateway. Tencent faceid pushes no signed
	// callback itself, so polling is the authoritative path (same fail-closed
	// contract as the Aliyun adapter).
	tencentIDVSignatureHeader = "X-Tc-Signature"
	// tencentIDVSignatureHeaderLegacy is the historical header spelling.
	tencentIDVSignatureHeaderLegacy = "X-Tencent-Signature"
)

// Tencent faceid API actions (signature v3, POST JSON body):
//   - DetectAuth 实名核身鉴权（发起核身，返回 BizToken 与 H5 启动 Url）:
//     https://cloud.tencent.com/document/product/1007/31816
//   - GetDetectInfoEnhanced 获取实名核身结果信息（适用所有版本）:
//     https://cloud.tencent.com/document/product/1007/41957
//   - 签名方法 v3（TC3-HMAC-SHA256）:
//     https://cloud.tencent.com/document/product/1007/31324
//   - 流程返回码表（ErrCode）:
//     https://cloud.tencent.com/document/product/1007/115315
const (
	tencentActionDetectAuth            = "DetectAuth"
	tencentActionGetDetectInfoEnhanced = "GetDetectInfoEnhanced"
)

// TencentIDVVerifier implements IdentityVerifier for Tencent Cloud 人脸核身
// （慧眼 faceid）. DetectAuth issues a BizToken plus the WeChat H5 launch Url,
// and GetDetectInfoEnhanced polls the final result with the tenant RuleId.
type TencentIDVVerifier struct {
	SecretID      string
	SecretKey     string
	Region        string
	Endpoint      string
	RuleID        string
	WebhookSecret string
	client        *http.Client
}

var _ IdentityVerifier = (*TencentIDVVerifier)(nil)

// NewTencentIDVVerifierWithOptions builds the faceid adapter. The endpoint is
// overridable for tests; an empty endpoint uses the official domain. faceid is
// a Region-less product, so Region is only sent when explicitly configured.
func NewTencentIDVVerifierWithOptions(secretID, secretKey, region, endpoint, ruleID, webhookSecret string) *TencentIDVVerifier {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = tencentIDVDefaultEndpoint
	}
	return &TencentIDVVerifier{
		SecretID:      strings.TrimSpace(secretID),
		SecretKey:     strings.TrimSpace(secretKey),
		Region:        strings.TrimSpace(region),
		Endpoint:      strings.TrimRight(endpoint, "/"),
		RuleID:        strings.TrimSpace(ruleID),
		WebhookSecret: strings.TrimSpace(webhookSecret),
		client:        safeClient(),
	}
}

// InitiateVerification calls DetectAuth and returns the BizToken as the
// verification reference plus the H5 launch Url as the redirect target.
func (v *TencentIDVVerifier) InitiateVerification(ctx context.Context, opts InitiateOptions) (*InitiateResult, error) {
	if strings.TrimSpace(v.RuleID) == "" {
		return nil, apperrors.NewInvalidArgument("tencent_idv: ruleId is not configured for this provider")
	}
	body := map[string]any{
		"RuleId": v.RuleID,
		"Name":   opts.FullName,
		"IdCard": opts.IDNumber,
	}
	if opts.RedirectURL != "" {
		body["RedirectUrl"] = opts.RedirectURL
	}
	if opts.UserID != "" {
		body["Extra"] = opts.UserID
	}
	var resp struct {
		BizToken string `json:"BizToken"`
		Url      string `json:"Url"`
	}
	if err := v.call(ctx, tencentActionDetectAuth, body, &resp); err != nil {
		return nil, err
	}
	if resp.BizToken == "" {
		return nil, apperrors.NewUnavailable("tencent_idv: DetectAuth returned an empty BizToken")
	}
	return &InitiateResult{
		VerificationID: resp.BizToken,
		RedirectURL:    resp.Url,
		SessionToken:   resp.BizToken,
	}, nil
}

// GetVerificationResult polls GetDetectInfoEnhanced and maps Text.ErrCode onto
// the domain status per the official return-code table: 0 → approved,
// 15 → expired (BizToken 过期), 14/16/17 → pending (流程未完成/重试上限),
// any other non-zero code → rejected with the provider ErrMsg.
func (v *TencentIDVVerifier) GetVerificationResult(ctx context.Context, verificationID string) (*VerificationResult, error) {
	if strings.TrimSpace(v.RuleID) == "" {
		return nil, apperrors.NewInvalidArgument("tencent_idv: ruleId is not configured for this provider")
	}
	var resp struct {
		Text string `json:"Text"`
	}
	err := v.call(ctx, tencentActionGetDetectInfoEnhanced, map[string]any{
		"BizToken": verificationID,
		"RuleId":   v.RuleID,
		"InfoType": "1",
	}, &resp)
	if err != nil {
		return nil, err
	}
	var text struct {
		ErrCode int    `json:"ErrCode"`
		ErrMsg  string `json:"ErrMsg"`
	}
	status := StatusPending
	if strings.TrimSpace(resp.Text) != "" {
		if err := json.Unmarshal([]byte(resp.Text), &text); err != nil {
			return nil, apperrors.NewUnavailable("tencent_idv: GetDetectInfoEnhanced Text is not valid JSON")
		}
		status = mapTencentFaceIDErrCode(text.ErrCode)
	}
	out := &VerificationResult{
		VerificationID: verificationID,
		Status:         status,
		ProviderName:   tencentIDVProviderName,
	}
	if out.Status == StatusRejected {
		out.Reason = strings.TrimSpace(text.ErrMsg)
	}
	return out, nil
}

// VerifyCallback handles a faceid result callback. Tencent pushes no signed
// callback, so when a webhook secret is configured the receiver must be
// fronted by a gateway that adds X-Tc-Signature (HMAC-SHA256 hex over the raw
// body). Without a configured secret the callback is rejected: polling
// GetDetectInfoEnhanced is the authoritative path.
func (v *TencentIDVVerifier) VerifyCallback(_ context.Context, payload []byte, headers map[string]string) (*VerificationResult, error) {
	if v.WebhookSecret == "" {
		// Fail closed: without a gateway-injected signature there is no way to
		// authenticate the caller, so the callback must be rejected.
		return nil, apperrors.NewForbidden("tencent_idv: callback secret is not configured; use GetDetectInfoEnhanced polling")
	}
	signature := headerValue(headers, tencentIDVSignatureHeader)
	if signature == "" {
		signature = headerValue(headers, tencentIDVSignatureHeaderLegacy)
	}
	if signature == "" {
		return nil, apperrors.NewUnauthorized("tencent_idv: callback signature header is missing")
	}
	mac := hmac.New(sha256.New, []byte(v.WebhookSecret))
	mac.Write(payload)
	if !hmac.Equal([]byte(signature), []byte(hex.EncodeToString(mac.Sum(nil)))) {
		return nil, apperrors.NewUnauthorized("tencent_idv: callback signature verification failed")
	}
	var callback struct {
		BizToken string `json:"BizToken"`
		ErrCode  *int   `json:"ErrCode"`
		ErrMsg   string `json:"ErrMsg"`
	}
	if err := decodeJSON(tencentIDVProviderName, payload, &callback); err != nil {
		return nil, err
	}
	if callback.BizToken == "" {
		return nil, apperrors.NewInvalidArgument("tencent_idv: callback is missing BizToken")
	}
	status := StatusPending
	if callback.ErrCode != nil {
		status = mapTencentFaceIDErrCode(*callback.ErrCode)
	}
	out := &VerificationResult{
		VerificationID: callback.BizToken,
		Status:         status,
		ProviderName:   tencentIDVProviderName,
	}
	if out.Status == StatusRejected {
		out.Reason = strings.TrimSpace(callback.ErrMsg)
	}
	return out, nil
}

// Probe calls GetDetectInfoEnhanced (read-only) with a random BizToken. A
// business-level error still proves the signature was accepted; only
// credential/authorization failures disqualify.
func (v *TencentIDVVerifier) Probe(ctx context.Context) error {
	if strings.TrimSpace(v.RuleID) == "" {
		return apperrors.NewInvalidArgument("tencent_idv: ruleId is not configured for this provider")
	}
	probeToken, err := platformuuid.V7Generator{}.NewString()
	if err != nil {
		return apperrors.WrapInternal(err, "tencent_idv: generate probe token")
	}
	var resp struct {
		Text string `json:"Text"`
	}
	err = v.call(ctx, tencentActionGetDetectInfoEnhanced, map[string]any{
		"BizToken": probeToken,
		"RuleId":   v.RuleID,
		"InfoType": "1",
	}, &resp)
	if err == nil {
		return nil
	}
	if isTencentAuthError(err) {
		return err
	}
	// InvalidParameter / BizTokenExpired / RuleIdNotExist all mean the request
	// was authenticated and understood; only transport/auth failures
	// disqualify.
	if isTencentBusinessError(err) {
		return nil
	}
	return err
}

// call executes one signed faceid request and decodes the Response object
// into out.
func (v *TencentIDVVerifier) call(ctx context.Context, action string, body map[string]any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return apperrors.WrapInternal(err, "tencent_idv: marshal request")
	}
	raw, err := postTencentCloudAPI(ctx, tencentCallParams{
		Client:    v.client,
		Endpoint:  v.Endpoint,
		Service:   tencentIDVService,
		Version:   tencentIDVAPIVersion,
		Region:    v.Region,
		Action:    action,
		Payload:   payload,
		SecretID:  v.SecretID,
		SecretKey: v.SecretKey,
		Provider:  tencentIDVProviderName,
	})
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return apperrors.WrapInternal(err, "tencent_idv: decode "+action+" response")
	}
	return nil
}

// mapTencentFaceIDErrCode maps a faceid DetectInfoText.ErrCode onto the domain
// status. The codes come from the official return-code table
// (https://cloud.tencent.com/document/product/1007/115315): 0 成功,
// 14 本次校验已完成, 15 Token 过期, 16 已达到重试上限, 17 本次流程未完成.
func mapTencentFaceIDErrCode(code int) VerificationStatus {
	switch code {
	case 0:
		return StatusApproved
	case 15:
		return StatusExpired
	case 14, 16, 17:
		return StatusPending
	default:
		return StatusRejected
	}
}

// tencentTC3Authorization computes the Tencent Cloud API 3.0 signature v3
// (TC3-HMAC-SHA256) Authorization header per
// https://cloud.tencent.com/document/product/1007/31324.
//
// The canonical request signs exactly the headers content-type, host and
// x-tc-action (lower-cased); the credential scope is
// <date>/<service>/tc3_request and the signing-key chain is
// TC3<SecretKey> → date → service → "tc3_request".
func tencentTC3Authorization(service, host, action, contentType string, timestamp int64, payload []byte, secretID, secretKey string) string {
	hashedPayload := sha256.Sum256(payload)
	canonicalHeaders := "content-type:" + contentType + "\n" +
		"host:" + host + "\n" +
		"x-tc-action:" + strings.ToLower(action) + "\n"
	signedHeaders := "content-type;host;x-tc-action"
	canonicalRequest := strings.Join([]string{
		http.MethodPost, "/", "", canonicalHeaders, signedHeaders, hex.EncodeToString(hashedPayload[:]),
	}, "\n")
	hashedCanonical := sha256.Sum256([]byte(canonicalRequest))
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	credentialScope := date + "/" + service + "/tc3_request"
	stringToSign := strings.Join([]string{
		"TC3-HMAC-SHA256",
		strconv.FormatInt(timestamp, 10),
		credentialScope,
		hex.EncodeToString(hashedCanonical[:]),
	}, "\n")
	secretDate := tencentHMAC([]byte("TC3"+secretKey), []byte(date))
	secretService := tencentHMAC(secretDate, []byte(service))
	secretSigning := tencentHMAC(secretService, []byte("tc3_request"))
	signature := hex.EncodeToString(tencentHMAC(secretSigning, []byte(stringToSign)))
	return "TC3-HMAC-SHA256 Credential=" + secretID + "/" + credentialScope +
		", SignedHeaders=" + signedHeaders + ", Signature=" + signature
}

func tencentHMAC(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

// tencentCallParams carries the inputs of one signed Tencent Cloud API 3.0
// JSON request shared by faceid and OCR.
type tencentCallParams struct {
	Client    *http.Client
	Endpoint  string
	Service   string
	Version   string
	Region    string
	Action    string
	Payload   []byte
	SecretID  string
	SecretKey string
	Provider  string
}

// postTencentCloudAPI signs and sends one API 3.0 JSON request and returns the
// decoded Response object. Tencent answers every business failure as HTTP 200
// (or 4xx) with Response.Error, so the error envelope is checked before the
// status code.
func postTencentCloudAPI(ctx context.Context, params tencentCallParams) (json.RawMessage, error) {
	parsed, err := url.Parse(params.Endpoint)
	if err != nil {
		return nil, apperrors.WrapInvalidArgumentf(err, "%s: invalid endpoint", params.Provider)
	}
	now := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, params.Endpoint+"/", bytes.NewReader(params.Payload))
	if err != nil {
		return nil, apperrors.WrapInternal(err, params.Provider+": build request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-TC-Action", params.Action)
	req.Header.Set("X-TC-Version", params.Version)
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(now.Unix(), 10))
	if params.Region != "" {
		req.Header.Set("X-TC-Region", params.Region)
	}
	req.Header.Set("Authorization", tencentTC3Authorization(params.Service, parsed.Host, params.Action, "application/json", now.Unix(), params.Payload, params.SecretID, params.SecretKey))

	resp, err := params.Client.Do(req)
	if err != nil {
		return nil, apperrors.WrapUnavailablef(err, "%s: %s request failed", params.Provider, params.Action)
	}
	defer resp.Body.Close()

	raw, err := readLimited(resp)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Response struct {
			Error *struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		if !isSuccessStatus(resp.StatusCode) {
			return nil, mapProviderHTTPError(params.Provider, resp.StatusCode)
		}
		return nil, apperrors.NewUnavailable(params.Provider + ": " + params.Action + " returned invalid JSON")
	}
	if envelope.Response.Error != nil {
		return nil, mapTencentCloudAPIError(params.Provider, envelope.Response.Error.Code, params.Action)
	}
	if !isSuccessStatus(resp.StatusCode) {
		return nil, mapProviderHTTPError(params.Provider, resp.StatusCode)
	}
	var wrapper struct {
		Response json.RawMessage `json:"Response"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil || len(wrapper.Response) == 0 {
		return nil, apperrors.NewUnavailable(params.Provider + ": " + params.Action + " response missing Response object")
	}
	return wrapper.Response, nil
}

// mapTencentCloudAPIError translates a Tencent Cloud API 3.0 business error
// code (Response.Error.Code) into the platform taxonomy. The provider prefix
// is passed in because the faceid and OCR adapters share this classifier.
func mapTencentCloudAPIError(provider, code, action string) error {
	switch {
	case strings.HasPrefix(code, "AuthFailure"):
		return apperrors.NewUnauthorizedf("%s: credential rejected (%s)", provider, code)
	case strings.HasPrefix(code, "UnauthorizedOperation"):
		return apperrors.NewForbiddenf("%s: %s denied access (%s)", provider, action, code)
	case strings.HasPrefix(code, "InvalidParameter"),
		strings.HasPrefix(code, "MissingParameter"):
		return apperrors.NewInvalidArgumentf("%s: %s rejected the request (%s)", provider, action, code)
	case strings.HasPrefix(code, "InternalError"),
		strings.HasPrefix(code, "FailedOperation"),
		strings.HasPrefix(code, "ResourceUnavailable"):
		return apperrors.NewUnavailablef("%s: %s failed (%s)", provider, action, code)
	case strings.HasPrefix(code, "LimitExceeded"):
		return apperrors.NewTooManyRequestsf("%s: %s rate limited (%s)", provider, action, code)
	default:
		return apperrors.NewUnavailablef("%s: %s failed (%s)", provider, action, code)
	}
}

func isTencentAuthError(err error) bool {
	appErr, ok := err.(*apperrors.Error)
	if !ok {
		return false
	}
	return appErr.Code == apperrors.CodeUnauthorized || appErr.Code == apperrors.CodeForbidden
}

func isTencentBusinessError(err error) bool {
	appErr, ok := err.(*apperrors.Error)
	if !ok {
		return false
	}
	return appErr.Code == apperrors.CodeInvalidArgument || appErr.Code == apperrors.CodeNotFound
}
