package idv

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

const (
	aliyunDefaultEndpoint     = "https://cloudauth.aliyuncs.com"
	aliyunAPIVersion          = "2019-03-07"
	aliyunProviderName        = "aliyun_idv"
	aliyunDefaultModel        = "LIVENESS"
	aliyunDefaultProduct      = "ID_PRO"
	aliyunFaceContrastProduct = "PV_FV"
)

// AliyunIDVVerifier implements IdentityVerifier for Aliyun CloudAuth
// (实人认证). CloudAuth is an RPC-style API: every call is a form-encoded POST
// signed with the RPC V1 HMAC-SHA1 recipe (the previous ACS3 attempt signed a
// JSON body, which Aliyun rejects).
//
// Calibration (OPEN-API-75 P5, 2026-11-14, 已按官方文档复核，无偏差):
//   - InitFaceVerify 初始化人脸核身请求（2019-03-07）:
//     https://next.api.aliyun.com/document/Cloudauth/2019-03-07/InitFaceVerify
//   - DescribeFaceVerify 获取认证结果（2019-03-07）:
//     https://next.api.aliyun.com/document/Cloudauth/2019-03-07/DescribeFaceVerify
//   - RPC V1 签名机制: https://help.aliyun.com/document_detail/315597.html
type AliyunIDVVerifier struct {
	AccessKeyID     string
	AccessKeySecret string
	RegionID        string
	Endpoint        string
	SceneID         string
	ProductCode     string
	Model           string
	WebhookSecret   string
	client          *http.Client
}

var _ IdentityVerifier = (*AliyunIDVVerifier)(nil)

func NewAliyunIDVVerifier(accessKeyID, accessKeySecret, regionID, webhookSecret string) *AliyunIDVVerifier {
	return NewAliyunIDVVerifierWithOptions(accessKeyID, accessKeySecret, regionID, webhookSecret, "", "", "", "")
}

func NewAliyunIDVVerifierWithOptions(accessKeyID, accessKeySecret, regionID, webhookSecret, endpoint, sceneID, productCode, model string) *AliyunIDVVerifier {
	if strings.TrimSpace(regionID) == "" {
		regionID = "cn-shanghai"
	}
	if strings.TrimSpace(endpoint) == "" {
		endpoint = aliyunDefaultEndpoint
	}
	if strings.TrimSpace(productCode) == "" {
		productCode = aliyunDefaultProduct
	}
	if strings.TrimSpace(model) == "" {
		model = aliyunDefaultModel
	}
	return &AliyunIDVVerifier{
		AccessKeyID:     accessKeyID,
		AccessKeySecret: accessKeySecret,
		RegionID:        regionID,
		Endpoint:        strings.TrimRight(endpoint, "/"),
		SceneID:         sceneID,
		ProductCode:     productCode,
		Model:           model,
		WebhookSecret:   webhookSecret,
		client:          safeClient(),
	}
}

// InitiateVerification routes by the face action (design §3):
//   - real_name_verify_cn (default): domestic ID_PRO/ID_PLUS authoritative
//     real-name verification (InitFaceVerify 2019-03-07).
//   - login_verify: domestic PV_FV 活体人脸验证 — liveness + 1:1 comparison
//     against the enrolled face (FaceContrastPictureUrl). LR_FR is the
//     native-SDK equivalent of the same scheme; the Web/H5 product is PV_FV.
//   - real_name_verify_overseas: Cloudauth-intl eKYC_PRO (InitializeV2).
func (v *AliyunIDVVerifier) InitiateVerification(ctx context.Context, opts InitiateOptions) (*InitiateResult, error) {
	switch opts.Action {
	case ActionRealNameVerifyOverseas:
		return v.initEKYC(ctx, opts)
	case ActionLoginVerify:
		return v.initFaceContrast(ctx, opts)
	default:
		return v.initRealName(ctx, opts)
	}
}

// domesticInitParams validates the inputs every domestic InitFaceVerify
// product requires and builds the shared parameter set. MetaInfo must be the
// real client value from the frontend getMetaInfo() — the official docs warn
// that a hardcoded value can prevent CertifyUrl issuance, so it is fail-closed.
func (v *AliyunIDVVerifier) domesticInitParams(opts InitiateOptions) (map[string]string, error) {
	if strings.TrimSpace(v.SceneID) == "" {
		return nil, apperrors.NewInvalidArgument("aliyun_idv: sceneId is not configured for this provider")
	}
	if strings.TrimSpace(opts.BizSeq) == "" {
		return nil, apperrors.NewInvalidArgument("aliyun_idv: bizSeq (OuterOrderNo) is required")
	}
	if strings.TrimSpace(opts.MetaInfo) == "" {
		return nil, apperrors.NewInvalidArgument("aliyun_idv: metaInfo from the frontend SDK is required")
	}
	if strings.TrimSpace(opts.RedirectURL) == "" {
		return nil, apperrors.NewInvalidArgument("aliyun_idv: returnUrl is required")
	}
	params := map[string]string{
		"SceneId":      v.SceneID,
		"OuterOrderNo": opts.BizSeq,
		"Model":        v.Model,
		"MetaInfo":     opts.MetaInfo,
		"ReturnUrl":    opts.RedirectURL,
	}
	if urlType := aliyunCertifyURLType(opts.MetaInfo); urlType != "" {
		params["CertifyUrlType"] = urlType
	}
	return params, nil
}

// aliyunCertifyURLType derives WEB/H5 from the MetaInfo deviceType so the
// liveness algorithm matches the device; unknown values let the provider
// auto-detect from MetaInfo.
func aliyunCertifyURLType(metaInfo string) string {
	var meta struct {
		DeviceType string `json:"deviceType"`
	}
	if err := json.Unmarshal([]byte(metaInfo), &meta); err != nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(meta.DeviceType)) {
	case "web":
		return "WEB"
	case "h5":
		return "H5"
	default:
		return ""
	}
}

func (v *AliyunIDVVerifier) initRealName(ctx context.Context, opts InitiateOptions) (*InitiateResult, error) {
	params, err := v.domesticInitParams(opts)
	if err != nil {
		return nil, err
	}
	product := v.ProductCode
	if product != "ID_PLUS" {
		product = aliyunDefaultProduct
	}
	params["ProductCode"] = product
	certType := strings.ToUpper(strings.TrimSpace(opts.IDType))
	if certType == "" {
		certType = "IDENTITY_CARD"
	}
	params["CertType"] = certType
	if opts.UserID != "" {
		params["UserId"] = opts.UserID
	}
	if opts.FullName != "" {
		params["CertName"] = opts.FullName
	}
	if opts.IDNumber != "" {
		params["CertNo"] = opts.IDNumber
	}
	return v.initDomestic(ctx, params, product)
}

func (v *AliyunIDVVerifier) initFaceContrast(ctx context.Context, opts InitiateOptions) (*InitiateResult, error) {
	params, err := v.domesticInitParams(opts)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(opts.TargetFaceURL) == "" {
		return nil, apperrors.NewInvalidArgument("aliyun_idv: login verification requires the enrolled face reference (targetFaceUrl)")
	}
	if strings.TrimSpace(opts.UserID) == "" {
		return nil, apperrors.NewInvalidArgument("aliyun_idv: login verification requires userId")
	}
	params["ProductCode"] = aliyunFaceContrastProduct
	params["CertType"] = "IDENTITY_CARD"
	params["UserId"] = opts.UserID
	params["FaceContrastPictureUrl"] = opts.TargetFaceURL
	// The enrolled photo is a user upload, not an SDK-captured frame; the
	// official guidance recommends allowing crop/alignment for such images.
	params["Crop"] = "T"
	return v.initDomestic(ctx, params, aliyunFaceContrastProduct)
}

func (v *AliyunIDVVerifier) initDomestic(ctx context.Context, params map[string]string, product string) (*InitiateResult, error) {
	result, err := v.call(ctx, "InitFaceVerify", params)
	if err != nil {
		return nil, err
	}

	var resultObject struct {
		CertifyID  string `json:"certifyId"`
		CertifyURL string `json:"certifyUrl"`
	}
	if raw, ok := result["ResultObject"].(string); ok && raw != "" {
		if err := json.Unmarshal([]byte(raw), &resultObject); err != nil {
			return nil, apperrors.NewUnavailable("aliyun_idv: InitFaceVerify ResultObject is not valid JSON")
		}
	}
	if resultObject.CertifyID == "" || resultObject.CertifyURL == "" {
		return nil, apperrors.NewUnavailable("aliyun_idv: InitFaceVerify returned no certifyId/certifyUrl")
	}

	return &InitiateResult{
		VerificationID: resultObject.CertifyID,
		RedirectURL:    resultObject.CertifyURL,
		SessionToken:   resultObject.CertifyID,
		ProductCode:    product,
	}, nil
}

func (v *AliyunIDVVerifier) GetVerificationResult(ctx context.Context, verificationID string) (*VerificationResult, error) {
	if isAliyunIntlProduct(v.ProductCode) {
		// Cloudauth-intl CheckResult requires MerchantBizId alongside the
		// TransactionId; only the order-aware entry point can supply it.
		return nil, apperrors.NewInvalidArgument("aliyun_idv: international result query requires the merchant biz sequence")
	}
	if strings.TrimSpace(v.SceneID) == "" {
		return nil, apperrors.NewInvalidArgument("aliyun_idv: sceneId is not configured for this provider")
	}
	result, err := v.call(ctx, "DescribeFaceVerify", map[string]string{"CertifyId": verificationID, "SceneId": v.SceneID})
	if err != nil {
		return nil, err
	}

	var object struct {
		Passed       string `json:"passed"`
		SubCode      string `json:"subCode"`
		MaterialInfo string `json:"materialInfo"`
		Identity     struct {
			CertifyID string `json:"certifyId"`
		} `json:"identityInfo"`
	}
	if raw, ok := result["ResultObject"].(string); ok && raw != "" {
		if err := json.Unmarshal([]byte(raw), &object); err != nil {
			return nil, apperrors.NewUnavailable("aliyun_idv: DescribeFaceVerify ResultObject is not valid JSON")
		}
	}

	status := StatusPending
	switch strings.ToUpper(object.Passed) {
	case "T":
		status = StatusApproved
	case "F":
		status = StatusRejected
	}

	out := &VerificationResult{
		VerificationID: verificationID,
		Status:         status,
		ProviderName:   aliyunProviderName,
		GatewayCode:    object.SubCode,
	}
	if status == StatusRejected {
		out.Reason = mapAliyunSubCode(object.SubCode)
	}
	if score := aliyunVerifyScore(object.MaterialInfo); score > 0 {
		out.Confidence = score
	}
	return out, nil
}

// GetVerificationResultWithOrder implements domain.OrderResultQuerier. For
// international products it calls Cloudauth-intl CheckResult with the
// persisted MerchantBizId; domestic products ignore the sequence.
func (v *AliyunIDVVerifier) GetVerificationResultWithOrder(ctx context.Context, verificationID, merchantBizID string) (*VerificationResult, error) {
	if isAliyunIntlProduct(v.ProductCode) {
		return v.checkEKYCResult(ctx, verificationID, merchantBizID)
	}
	return v.GetVerificationResult(ctx, verificationID)
}

// aliyunVerifyScore extracts the live-face vs enrolled-face comparison score
// (MaterialInfo.facialPictureFront.verifyScore) when the scene retains
// material. Absent or malformed material simply yields no score.
func aliyunVerifyScore(materialInfo string) float64 {
	if strings.TrimSpace(materialInfo) == "" {
		return 0
	}
	var material struct {
		FacialPictureFront struct {
			VerifyScore float64 `json:"verifyScore"`
		} `json:"facialPictureFront"`
	}
	if err := json.Unmarshal([]byte(materialInfo), &material); err != nil {
		return 0
	}
	return material.FacialPictureFront.VerifyScore
}

// VerifyCallback handles a CloudAuth result callback. CloudAuth has no native
// callback signature, so when a webhook secret is configured the receiver must
// be fronted by a gateway that adds X-Acs-Signature (HMAC-SHA256 hex). Without
// a configured secret the callback is rejected: polling DescribeFaceVerify is
// the authoritative path.
func (v *AliyunIDVVerifier) VerifyCallback(_ context.Context, payload []byte, headers map[string]string) (*VerificationResult, error) {
	if v.WebhookSecret == "" {
		// Fail closed: without a gateway-injected signature there is no way to
		// authenticate the caller, so the callback must be rejected.
		return nil, apperrors.NewForbidden("aliyun_idv: callback secret is not configured; use DescribeFaceVerify polling")
	}
	signature := headerValue(headers, "X-Acs-Signature")
	if signature == "" {
		return nil, apperrors.NewUnauthorized("aliyun_idv: callback signature header is missing")
	}
	mac := hmac.New(sha256.New, []byte(v.WebhookSecret))
	mac.Write(payload)
	if !hmac.Equal([]byte(signature), []byte(hex.EncodeToString(mac.Sum(nil)))) {
		return nil, apperrors.NewUnauthorized("aliyun_idv: callback signature verification failed")
	}

	var callback struct {
		CertifyID string `json:"CertifyId"`
		Passed    string `json:"Passed"`
		SubCode   string `json:"SubCode"`
	}
	if err := decodeJSON(aliyunProviderName, payload, &callback); err != nil {
		return nil, err
	}
	if callback.CertifyID == "" {
		return nil, apperrors.NewInvalidArgument("aliyun_idv: callback is missing CertifyId")
	}
	status := StatusPending
	switch strings.ToUpper(callback.Passed) {
	case "T":
		status = StatusApproved
	case "F":
		status = StatusRejected
	}
	out := &VerificationResult{
		VerificationID: callback.CertifyID,
		Status:         status,
		ProviderName:   aliyunProviderName,
	}
	if status == StatusRejected {
		out.Reason = mapAliyunSubCode(callback.SubCode)
	}
	return out, nil
}

// Probe calls DescribeFaceVerify (read-only) with a random CertifyId. A
// business-level "not found" response still proves the signature was accepted.
func (v *AliyunIDVVerifier) Probe(ctx context.Context) error {
	if isAliyunIntlProduct(v.ProductCode) {
		return v.probeIntl(ctx)
	}
	if strings.TrimSpace(v.SceneID) == "" {
		return apperrors.NewInvalidArgument("aliyun_idv: sceneId is not configured for this provider")
	}
	probeID, err := platformuuid.V7Generator{}.NewString()
	if err != nil {
		return apperrors.WrapInternal(err, "aliyun_idv: generate probe id")
	}
	_, err = v.call(ctx, "DescribeFaceVerify", map[string]string{"CertifyId": probeID, "SceneId": v.SceneID})
	if err == nil {
		return nil
	}
	if isAliyunAuthError(err) {
		return err
	}
	// InvalidParameter / NotFound / invalid CertifyId all mean the request was
	// authenticated and understood; only transport/auth failures disqualify.
	if isAliyunBusinessError(err) {
		return nil
	}
	return err
}

func (v *AliyunIDVVerifier) call(ctx context.Context, action string, business map[string]string) (map[string]any, error) {
	return v.rpcCall(ctx, v.Endpoint, aliyunAPIVersion, "200", action, business)
}

// rpcCall performs one RPC V1 form-encoded POST. successCode differs between
// the domestic Cloudauth ("200") and Cloudauth-intl ("Success") products.
func (v *AliyunIDVVerifier) rpcCall(ctx context.Context, endpoint, version, successCode, action string, business map[string]string) (map[string]any, error) {
	params := map[string]string{
		"AccessKeyId":      v.AccessKeyID,
		"Action":           action,
		"Format":           "JSON",
		"RegionId":         v.RegionID,
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   mustUUID(),
		"SignatureVersion": "1.0",
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"Version":          version,
	}
	for key, value := range business {
		if value != "" {
			params[key] = value
		}
	}
	params["Signature"] = aliyunRPCSignature(http.MethodPost, params, v.AccessKeySecret)

	form := url.Values{}
	for key, value := range params {
		form.Set(key, value)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, apperrors.WrapInternal(err, "aliyun_idv: build request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, apperrors.WrapUnavailablef(err, "aliyun_idv: %s request failed", action)
	}
	defer resp.Body.Close()

	raw, err := readLimited(resp)
	if err != nil {
		return nil, err
	}
	if !isSuccessStatus(resp.StatusCode) {
		if code, message := aliyunRPCErrorCode(raw); code != "" {
			return nil, mapAliyunErrorCode(aliyunProviderName, code, message, action)
		}
		return nil, mapProviderHTTPError(aliyunProviderName, resp.StatusCode)
	}

	var result map[string]any
	if err := decodeJSON(aliyunProviderName, raw, &result); err != nil {
		return nil, err
	}
	if code, ok := result["Code"].(string); ok && code != "" && code != successCode {
		message, _ := result["Message"].(string)
		return nil, mapAliyunErrorCode(aliyunProviderName, code, message, action)
	}
	return result, nil
}

// aliyunRPCSignature builds the RPC V1 signature:
// StringToSign = POST&%2F&percentEncode(sorted canonical query)
// Signature    = Base64(HMAC-SHA1(accessKeySecret + "&", StringToSign))
func aliyunRPCSignature(method string, params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		if key == "Signature" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, aliyunPercentEncode(key)+"="+aliyunPercentEncode(params[key]))
	}
	canonical := strings.Join(pairs, "&")
	stringToSign := method + "&" + aliyunPercentEncode("/") + "&" + aliyunPercentEncode(canonical)

	mac := hmac.New(sha1.New, []byte(secret+"&"))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// aliyunPercentEncode follows RFC 3986 as required by Aliyun RPC signing.
func aliyunPercentEncode(value string) string {
	encoded := url.QueryEscape(value)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}

func mustUUID() string {
	id, err := platformuuid.V7Generator{}.NewString()
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return id
}

// mapAliyunErrorCode translates an Aliyun RPC error code into the platform
// taxonomy. The provider prefix is passed in because the OCR and CloudAuth
// adapters share this classifier. The raw provider code travels in Meta: the
// unrecognized codes collapse into a generic 503, and without the original
// code the UI cannot tell "RAM not permitted / product not activated" apart
// from a real outage. Aliyun error codes are public machine codes, echoing
// them does not expose provider prose.
func mapAliyunErrorCode(provider, code, message, action string) error {
	meta := map[string]any{"provider": provider, "providerCode": code}
	switch {
	case strings.HasPrefix(code, "SignatureDoesNotMatch"),
		strings.HasPrefix(code, "IncompleteSignature"),
		strings.HasPrefix(code, "InvalidAccessKeyId"),
		strings.HasPrefix(code, "InvalidAccessKeySecret"):
		return apperrors.NewWithMeta(apperrors.CodeUnauthorized, fmt.Sprintf("%s: credential rejected (%s)", provider, code), meta)
	case strings.HasPrefix(code, "Forbidden"),
		strings.HasPrefix(code, "NoPermission"):
		return apperrors.NewWithMeta(apperrors.CodeForbidden, fmt.Sprintf("%s: %s denied access (%s)", provider, action, code), meta)
	case strings.HasPrefix(code, "Throttling"), strings.HasPrefix(code, "ServiceUnavailable"):
		return apperrors.NewWithMeta(apperrors.CodeUnavailable, fmt.Sprintf("%s: %s throttled or unavailable (%s)", provider, action, code), meta)
	case strings.HasPrefix(code, "InvalidParameter"),
		strings.HasPrefix(code, "MissingParameter"),
		strings.HasPrefix(code, "InvalidCertifyId"):
		return apperrors.NewWithMeta(apperrors.CodeInvalidArgument, fmt.Sprintf("%s: %s rejected the request (%s)", provider, action, code), meta)
	default:
		return apperrors.NewWithMeta(apperrors.CodeUnavailable, fmt.Sprintf("%s: %s failed (%s)", provider, action, code), meta)
	}
}

func isAliyunAuthError(err error) bool {
	appErr, ok := err.(*apperrors.Error)
	if !ok {
		return false
	}
	return appErr.Code == apperrors.CodeUnauthorized || appErr.Code == apperrors.CodeForbidden
}

func isAliyunBusinessError(err error) bool {
	appErr, ok := err.(*apperrors.Error)
	if !ok {
		return false
	}
	return appErr.Code == apperrors.CodeInvalidArgument || appErr.Code == apperrors.CodeNotFound
}

func mapAliyunSubCode(code string) string {
	switch code {
	case "200":
		return "face comparison failed"
	case "201":
		return "liveness detection failed"
	case "202":
		return "ID document not recognized"
	case "203":
		return "face and document do not match"
	default:
		return fmt.Sprintf("verification failed (code: %s)", code)
	}
}
