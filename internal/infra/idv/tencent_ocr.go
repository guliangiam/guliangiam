package idv

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	domainidv "github.com/guliangiam/guliangiam/internal/domain/idv"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	// tencentOCRProviderName is the provider registration key for Tencent
	// Cloud 文字识别 OCR.
	tencentOCRProviderName = "tencent_ocr"
	// tencentOCRDefaultEndpoint is the official API domain of 文字识别 OCR.
	tencentOCRDefaultEndpoint = "https://ocr.tencentcloudapi.com"
	// tencentOCRService is the credential-scope service name for signature v3.
	tencentOCRService = "ocr"
	// tencentOCRAPIVersion is the current OCR API version.
	tencentOCRAPIVersion = "2018-11-19"
	// tencentOCRDefaultRegion is the default OCR region.
	tencentOCRDefaultRegion = "ap-guangzhou"
	// tencentOCRMaxEncodedImageBytes bounds the base64 payload (官方限制:
	// 图片经 Base64 编码后不超过 10M). The transport layer enforces the
	// stricter 5MiB product limit; this is the last line of defence.
	tencentOCRMaxEncodedImageBytes = 10 << 20
)

// Tencent OCR actions (signature v3, POST JSON body, Version 2018-11-19,
// domain ocr.tencentcloudapi.com):
//   - IDCardOCR 身份证识别: https://cloud.tencent.com/document/product/866/33524
//   - MLIDPassportOCR 护照识别（多国多地区护照）:
//     https://cloud.tencent.com/document/product/866/37657
//   - DriverLicenseOCR 驾驶证识别: https://cloud.tencent.com/document/product/866/36213
//   - 签名方法 v3（TC3-HMAC-SHA256）:
//     https://cloud.tencent.com/document/product/1007/31324
const (
	tencentActionIDCardOCR        = "IDCardOCR"
	tencentActionMLIDPassportOCR  = "MLIDPassportOCR"
	tencentActionDriverLicenseOCR = "DriverLicenseOCR"
)

// TencentOCRRecognizer implements DocumentRecognizer for the Tencent Cloud
// 文字识别 OCR product. It shares the TC3-HMAC-SHA256 (signature v3) recipe
// with the faceid adapter but uses a different endpoint, service and version.
type TencentOCRRecognizer struct {
	SecretID  string
	SecretKey string
	Region    string
	Endpoint  string
	client    *http.Client
}

var _ DocumentRecognizer = (*TencentOCRRecognizer)(nil)

// NewTencentOCRRecognizer builds the OCR recognizer from tenant-scoped
// credentials. An empty endpoint uses the official domain and an empty region
// falls back to ap-guangzhou.
func NewTencentOCRRecognizer(config OCRConfig) *TencentOCRRecognizer {
	region := strings.TrimSpace(config.Region)
	if region == "" {
		region = tencentOCRDefaultRegion
	}
	endpoint := strings.TrimSpace(config.Endpoint)
	if endpoint == "" {
		endpoint = tencentOCRDefaultEndpoint
	}
	return &TencentOCRRecognizer{
		SecretID:  config.AccessKey,
		SecretKey: config.SecretKey,
		Region:    region,
		Endpoint:  strings.TrimRight(endpoint, "/"),
		client:    safeClient(),
	}
}

// RecognizeDocument encodes the image as Base64 and calls the official card
// OCR action for the document type. Tencent OCR ingests the image directly
// (ImageBase64); no download proxy is involved.
func (r *TencentOCRRecognizer) RecognizeDocument(ctx context.Context, documentType DocumentType, image DocumentImage) (*DocumentFields, error) {
	action, ok := tencentOCRAction(documentType)
	if !ok {
		return nil, apperrors.NewInvalidArgumentf("tencent_ocr: unsupported document type %q", string(documentType))
	}
	if len(image.Data) == 0 {
		return nil, apperrors.NewInvalidArgument("tencent_ocr: image body is empty")
	}
	encoded := base64.StdEncoding.EncodeToString(image.Data)
	if len(encoded) > tencentOCRMaxEncodedImageBytes {
		return nil, apperrors.NewInvalidArgument("tencent_ocr: image exceeds the 10MB base64 limit")
	}
	body := map[string]any{"ImageBase64": encoded}
	if side := tencentOCRCardSide(documentType, image.Side); side != "" {
		body["CardSide"] = side
	}
	if documentType == domainidv.DocumentIDCard || documentType == domainidv.DocumentDriverLicense {
		// 开启告警能力（翻拍/复印/PS 检测），不改变返回结构。
		body["Config"] = `{"DetectAlgo":true}`
	}

	raw, err := r.call(ctx, action, body)
	if err != nil {
		return nil, err
	}
	fields, err := parseTencentOCRResponse(documentType, raw)
	if err != nil {
		return nil, err
	}
	fields.IDType = string(documentType)
	if fields.FullName == "" && fields.IDNumber == "" {
		return nil, apperrors.NewUnavailable("tencent_ocr: the provider returned no readable identity fields")
	}
	return fields, nil
}

// tencentOCRAction maps a platform document type onto the official OCR action.
func tencentOCRAction(documentType DocumentType) (string, bool) {
	switch documentType {
	case domainidv.DocumentIDCard:
		return tencentActionIDCardOCR, true
	case domainidv.DocumentPassport:
		return tencentActionMLIDPassportOCR, true
	case domainidv.DocumentDriverLicense:
		return tencentActionDriverLicenseOCR, true
	default:
		return "", false
	}
}

// tencentOCRCardSide normalizes the optional side hint for the card actions
// that accept CardSide (身份证/驾驶证, FRONT/BACK); passports ignore it.
func tencentOCRCardSide(documentType DocumentType, side string) string {
	if documentType == domainidv.DocumentPassport {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(side)) {
	case "front":
		return "FRONT"
	case "back":
		return "BACK"
	default:
		return ""
	}
}

// tencentOCRDateLayouts are the date shapes Tencent card OCR may return:
// compact yyyyMMdd, Chinese 年月日, dotted and dashed variants.
var tencentOCRDateLayouts = []string{"20060102", "2006年1月2日", "2006.1.2", "2006-1-2", "2006/1/2"}

// normalizeTencentOCRDate converts provider dates to the canonical YYYY-MM-DD
// used by the DATE columns and the profile form. Values that cannot be parsed
// are dropped (empty) so a malformed OCR read never pollutes persisted data;
// 长期/长期有效 (no fixed expiry) also map to empty.
func normalizeTencentOCRDate(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "长期" || trimmed == "长期有效" {
		return ""
	}
	for _, layout := range tencentOCRDateLayouts {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return parsed.Format("2006-01-02")
		}
	}
	return ""
}

// normalizeTencentOCRSex maps the provider gender marks (男/女/M/F) to the
// platform values male/female; unknown values are dropped.
func normalizeTencentOCRSex(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "男", "M", "MALE":
		return "male"
	case "女", "F", "FEMALE":
		return "female"
	default:
		return ""
	}
}

// parseTencentOCRResponse decodes the flat Response object of each card OCR
// action into DocumentFields.
func parseTencentOCRResponse(documentType DocumentType, raw json.RawMessage) (*DocumentFields, error) {
	switch documentType {
	case domainidv.DocumentIDCard:
		var resp struct {
			Name         string `json:"Name"`
			Sex          string `json:"Sex"`
			Nation       string `json:"Nation"`
			Birth        string `json:"Birth"`
			Address      string `json:"Address"`
			IdNum        string `json:"IdNum"`
			Authority    string `json:"Authority"`
			ValidDate    string `json:"ValidDate"`
			AdvancedInfo string `json:"AdvancedInfo"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, apperrors.NewUnavailable("tencent_ocr: IDCardOCR response is not valid JSON")
		}
		fields := &DocumentFields{
			FullName:         strings.TrimSpace(resp.Name),
			IDNumber:         strings.TrimSpace(resp.IdNum),
			Gender:           normalizeTencentOCRSex(resp.Sex),
			Birthday:         normalizeTencentOCRDate(resp.Birth),
			Address:          strings.TrimSpace(resp.Address),
			IssuingAuthority: strings.TrimSpace(resp.Authority),
			Confidence:       tencentOCRConfidence(resp.AdvancedInfo),
		}
		if nation := strings.TrimSpace(resp.Nation); nation != "" && nation != "汉" {
			fields.Nationality = nation
		}
		if resp.ValidDate != "" {
			from, to, _ := strings.Cut(strings.TrimSpace(resp.ValidDate), "-")
			fields.ValidFrom = normalizeTencentOCRDate(from)
			fields.ValidTo = normalizeTencentOCRDate(to)
		}
		return fields, nil
	case domainidv.DocumentPassport:
		var resp struct {
			ID               string `json:"ID"`
			Name             string `json:"Name"`
			DateOfBirth      string `json:"DateOfBirth"`
			Sex              string `json:"Sex"`
			DateOfExpiration string `json:"DateOfExpiration"`
			IssuingCountry   string `json:"IssuingCountry"`
			Nationality      string `json:"Nationality"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, apperrors.NewUnavailable("tencent_ocr: MLIDPassportOCR response is not valid JSON")
		}
		fields := &DocumentFields{
			FullName:    strings.TrimSpace(resp.Name),
			IDNumber:    strings.TrimSpace(resp.ID),
			Birthday:    normalizeTencentOCRDate(resp.DateOfBirth),
			ValidTo:     normalizeTencentOCRDate(resp.DateOfExpiration),
			Gender:      normalizeTencentOCRSex(resp.Sex),
			Nationality: firstNonEmptyString(resp.Nationality, resp.IssuingCountry),
		}
		return fields, nil
	case domainidv.DocumentDriverLicense:
		var resp struct {
			Name             string `json:"Name"`
			Sex              string `json:"Sex"`
			Nationality      string `json:"Nationality"`
			Address          string `json:"Address"`
			DateOfBirth      string `json:"DateOfBirth"`
			Class            string `json:"Class"`
			StartDate        string `json:"StartDate"`
			EndDate          string `json:"EndDate"`
			CardCode         string `json:"CardCode"`
			IssuingAuthority string `json:"IssuingAuthority"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, apperrors.NewUnavailable("tencent_ocr: DriverLicenseOCR response is not valid JSON")
		}
		return &DocumentFields{
			FullName:         strings.TrimSpace(resp.Name),
			IDNumber:         strings.TrimSpace(resp.CardCode),
			Gender:           normalizeTencentOCRSex(resp.Sex),
			Birthday:         normalizeTencentOCRDate(resp.DateOfBirth),
			Nationality:      strings.TrimSpace(resp.Nationality),
			Address:          strings.TrimSpace(resp.Address),
			IssuingAuthority: strings.TrimSpace(resp.IssuingAuthority),
			ValidFrom:        normalizeTencentOCRDate(resp.StartDate),
			ValidTo:          normalizeTencentOCRDate(resp.EndDate),
		}, nil
	default:
		return nil, apperrors.NewInvalidArgumentf("tencent_ocr: unsupported document type %q", string(documentType))
	}
}

// tencentOCRConfidence maps the optional image quality score in the
// AdvancedInfo JSON string (Quality, 0-100) to 0-1. When the provider omits
// it the field stays zero, which the UI renders as "unknown".
func tencentOCRConfidence(advancedInfo string) float64 {
	trimmed := strings.TrimSpace(advancedInfo)
	if trimmed == "" {
		return 0
	}
	var info struct {
		Quality *float64 `json:"Quality"`
	}
	if err := json.Unmarshal([]byte(trimmed), &info); err != nil || info.Quality == nil {
		return 0
	}
	confidence := *info.Quality
	if confidence > 1 {
		confidence /= 100
	}
	if confidence < 0 {
		return 0
	}
	if confidence > 1 {
		return 1
	}
	return confidence
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// call executes one signed OCR request and returns the decoded Response
// object. Unlike faceid, OCR requires the Region public parameter.
func (r *TencentOCRRecognizer) call(ctx context.Context, action string, body map[string]any) (json.RawMessage, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "tencent_ocr: marshal request")
	}
	return postTencentCloudAPI(ctx, tencentCallParams{
		Client:    r.client,
		Endpoint:  r.Endpoint,
		Service:   tencentOCRService,
		Version:   tencentOCRAPIVersion,
		Region:    r.Region,
		Action:    action,
		Payload:   payload,
		SecretID:  r.SecretID,
		SecretKey: r.SecretKey,
		Provider:  tencentOCRProviderName,
	})
}
