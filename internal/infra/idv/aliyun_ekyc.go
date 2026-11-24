package idv

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// Cloudauth-intl (国际站实人认证) constants. The international products use a
// different API version, endpoint and success code ("Success" instead of the
// domestic "200"), but share the RPC V1 HMAC-SHA1 signing recipe.
//
// Calibration (2026-11-18, official docs):
//   - InitializeV2: https://www.alibabacloud.com/help/zh/id-verification/ai-real-person-verification/initializev2
//   - CheckResult:  https://www.alibabacloud.com/help/zh/id-verification/ai-real-person-verification/checkresult
const (
	aliyunIntlDefaultEndpoint = "https://cloudauth-intl.cn-shanghai.aliyuncs.com"
	aliyunIntlAPIVersion      = "2022-08-09"
	aliyunEKYCProduct         = "eKYC_PRO"
)

var _ OrderResultQuerier = (*AliyunIDVVerifier)(nil)

// isAliyunIntlProduct reports whether the product code belongs to the
// Cloudauth-intl product family (result queries route to CheckResult).
func isAliyunIntlProduct(productCode string) bool {
	switch strings.ToUpper(strings.TrimSpace(productCode)) {
	case "EKYC_PRO", "EKYC", "EKYC_NDI", "FACE_IDU", "FACE_VERIFY", "FACE_LIVENESS_PRO", "ID_OCR_PRO":
		return true
	default:
		return false
	}
}

// intlEndpoint keeps an operator-configured endpoint override but otherwise
// uses the international gateway (the constructor defaults to the domestic
// one, which cannot serve InitializeV2/CheckResult).
func (v *AliyunIDVVerifier) intlEndpoint() string {
	if v.Endpoint != "" && v.Endpoint != aliyunDefaultEndpoint {
		return v.Endpoint
	}
	return aliyunIntlDefaultEndpoint
}

func (v *AliyunIDVVerifier) callIntl(ctx context.Context, action string, business map[string]string) (map[string]any, error) {
	return v.rpcCall(ctx, v.intlEndpoint(), aliyunIntlAPIVersion, "Success", action, business)
}

// initEKYC starts an overseas real-name session via InitializeV2. Required
// inputs are fail-closed: MerchantBizId (our BizSeq), MetaInfo from the
// frontend SDK, MerchantUserId and the Web ReturnUrl. DocType prefers the
// explicit document type, then the ISO country code, then the global default.
func (v *AliyunIDVVerifier) initEKYC(ctx context.Context, opts InitiateOptions) (*InitiateResult, error) {
	if strings.TrimSpace(opts.BizSeq) == "" {
		return nil, apperrors.NewInvalidArgument("aliyun_idv: bizSeq (MerchantBizId) is required")
	}
	if strings.TrimSpace(opts.MetaInfo) == "" {
		return nil, apperrors.NewInvalidArgument("aliyun_idv: metaInfo from the frontend SDK is required")
	}
	if strings.TrimSpace(opts.UserID) == "" {
		return nil, apperrors.NewInvalidArgument("aliyun_idv: userId (MerchantUserId) is required")
	}
	if strings.TrimSpace(opts.RedirectURL) == "" {
		return nil, apperrors.NewInvalidArgument("aliyun_idv: returnUrl is required")
	}
	product := v.ProductCode
	if !isAliyunIntlProduct(product) {
		product = aliyunEKYCProduct
	}
	docType := strings.ToUpper(strings.TrimSpace(opts.IDType))
	if docType == "" {
		docType = strings.ToUpper(strings.TrimSpace(opts.Country))
	}
	if docType == "" {
		docType = "GLB"
	}
	params := map[string]string{
		"ProductCode":    product,
		"MerchantBizId":  opts.BizSeq,
		"MerchantUserId": opts.UserID,
		"MetaInfo":       opts.MetaInfo,
		"DocType":        docType,
		"ReturnUrl":      opts.RedirectURL,
		"Model":          v.Model,
		"SecurityLevel":  "02",
	}
	result, err := v.callIntl(ctx, "InitializeV2", params)
	if err != nil {
		return nil, err
	}

	var object struct {
		TransactionID  string `json:"TransactionId"`
		TransactionURL string `json:"TransactionUrl"`
	}
	if err := decodeAliyunNested(result["Result"], &object); err != nil {
		return nil, apperrors.NewUnavailable("aliyun_idv: InitializeV2 Result is not valid JSON")
	}
	if object.TransactionID == "" || object.TransactionURL == "" {
		return nil, apperrors.NewUnavailable("aliyun_idv: InitializeV2 returned no transactionId/transactionUrl")
	}
	return &InitiateResult{
		VerificationID: object.TransactionID,
		RedirectURL:    object.TransactionURL,
		SessionToken:   object.TransactionID,
		ProductCode:    product,
	}, nil
}

// checkEKYCResult queries CheckResult. MerchantBizId must match the value
// sent to InitializeV2 and TransactionId must come from server-side storage,
// never from the client. Code=Success only means the call succeeded; the
// verdict is Passed + SubCode.
func (v *AliyunIDVVerifier) checkEKYCResult(ctx context.Context, transactionID, merchantBizID string) (*VerificationResult, error) {
	if strings.TrimSpace(merchantBizID) == "" {
		return nil, apperrors.NewInvalidArgument("aliyun_idv: international result query requires the merchant biz sequence")
	}
	result, err := v.callIntl(ctx, "CheckResult", map[string]string{
		"MerchantBizId": merchantBizID,
		"TransactionId": transactionID,
	})
	if err != nil {
		return nil, err
	}

	var object struct {
		Passed      string `json:"Passed"`
		SubCode     string `json:"SubCode"`
		ExtFaceInfo string `json:"ExtFaceInfo"`
	}
	if err := decodeAliyunNested(result["Result"], &object); err != nil {
		return nil, apperrors.NewUnavailable("aliyun_idv: CheckResult Result is not valid JSON")
	}

	status := StatusPending
	switch strings.ToUpper(object.Passed) {
	case "Y":
		status = StatusApproved
	case "N":
		status = StatusRejected
	}
	out := &VerificationResult{
		VerificationID: transactionID,
		Status:         status,
		ProviderName:   aliyunProviderName,
		GatewayCode:    object.SubCode,
	}
	if status == StatusRejected {
		out.Reason = mapAliyunEKYCSubCode(object.SubCode)
	}
	if score := ekycFaceScore(object.ExtFaceInfo); score > 0 {
		out.Confidence = score
	}
	return out, nil
}

// probeIntl proves credential acceptance with a read-only CheckResult call
// using random identifiers; a business-level rejection still means the
// signature was accepted.
func (v *AliyunIDVVerifier) probeIntl(ctx context.Context) error {
	probeID, err := platformuuid.V7Generator{}.NewString()
	if err != nil {
		return apperrors.WrapInternal(err, "aliyun_idv: generate probe id")
	}
	_, err = v.callIntl(ctx, "CheckResult", map[string]string{
		"MerchantBizId": probeID,
		"TransactionId": probeID,
	})
	if err == nil || isAliyunBusinessError(err) {
		return nil
	}
	return err
}

// decodeAliyunNested decodes a provider field that is documented as a JSON
// string but may arrive as an already-decoded object.
func decodeAliyunNested(raw any, dst any) error {
	switch value := raw.(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return nil
		}
		return json.Unmarshal([]byte(value), dst)
	case map[string]any:
		encoded, err := json.Marshal(value)
		if err != nil {
			return err
		}
		return json.Unmarshal(encoded, dst)
	default:
		return nil
	}
}

// ekycFaceScore extracts ExtFaceInfo.faceComparisonScore (0~100, provider
// threshold >= 90). Absent or malformed material simply yields no score.
func ekycFaceScore(extFaceInfo string) float64 {
	if strings.TrimSpace(extFaceInfo) == "" {
		return 0
	}
	var face struct {
		FaceComparisonScore float64 `json:"faceComparisonScore"`
	}
	if err := json.Unmarshal([]byte(extFaceInfo), &face); err != nil {
		return 0
	}
	return face.FaceComparisonScore
}

func mapAliyunEKYCSubCode(code string) string {
	switch code {
	case "204":
		return "face and document photo do not match"
	case "205":
		return "liveness attack detected"
	case "206":
		return "device environment risk detected"
	case "212":
		return "document spoofing risk detected"
	default:
		return fmt.Sprintf("verification failed (code: %s)", code)
	}
}
