package idv

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	domainidv "github.com/guliangiam/guliangiam/internal/domain/idv"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	aliyunOCRDefaultEndpoint = "https://ocr-api.cn-hangzhou.aliyuncs.com"
	aliyunOCRAPIVersion      = "2021-07-07"
	aliyunOCRProviderName    = "aliyun_ocr"
	aliyunOCRDefaultRegion   = "cn-hangzhou"
	// Aliyun accepts documents up to 10MiB. The transport layer enforces the
	// stricter 5MiB product limit; this is the last line of defence.
	aliyunOCRMaxImageBytes = 10 << 20
)

// AliyunOCRRecognizer implements DocumentRecognizer for the Aliyun OCR
// 2021-07-07 product. It shares the RPC V1 HMAC-SHA1 signing recipe with the
// CloudAuth adapter but uses a different endpoint and API version.
type AliyunOCRRecognizer struct {
	AccessKeyID     string
	AccessKeySecret string
	RegionID        string
	Endpoint        string
	client          *http.Client
}

var _ DocumentRecognizer = (*AliyunOCRRecognizer)(nil)

func NewAliyunOCRRecognizer(config OCRConfig) *AliyunOCRRecognizer {
	region := strings.TrimSpace(config.Region)
	if region == "" {
		region = aliyunOCRDefaultRegion
	}
	endpoint := strings.TrimSpace(config.Endpoint)
	if endpoint == "" {
		endpoint = aliyunOCRDefaultEndpoint
	}
	return &AliyunOCRRecognizer{
		AccessKeyID:     config.AccessKey,
		AccessKeySecret: config.SecretKey,
		RegionID:        region,
		Endpoint:        strings.TrimRight(endpoint, "/"),
		client:          safeClient(),
	}
}

func (r *AliyunOCRRecognizer) RecognizeDocument(ctx context.Context, documentType DocumentType, image DocumentImage) (*DocumentFields, error) {
	action, ok := aliyunOCRAction(documentType)
	if !ok {
		return nil, apperrors.NewInvalidArgumentf("aliyun_ocr: unsupported document type %q", string(documentType))
	}
	if len(image.Data) == 0 {
		return nil, apperrors.NewInvalidArgument("aliyun_ocr: image body is empty")
	}
	if len(image.Data) > aliyunOCRMaxImageBytes {
		return nil, apperrors.NewInvalidArgument("aliyun_ocr: image exceeds the 10MiB limit")
	}

	result, err := r.call(ctx, action, image.Data)
	if err != nil {
		return nil, err
	}
	payload, err := decodeOCRPayload(result)
	if err != nil {
		return nil, err
	}

	fields := &DocumentFields{IDType: string(documentType)}
	scope := collectOCRObjects(payload)
	fields.FullName = firstOCRString(scope, "name", "fullName", "chineseName", "passportName")
	fields.IDNumber = firstOCRString(scope, "idNumber", "passportNumber", "licenseNumber", "idNo", "number")
	fields.Gender = firstOCRString(scope, "sex", "gender")
	fields.Birthday = firstOCRString(scope, "birthDate", "birthday", "dateOfBirth", "birth")
	fields.Nationality = firstOCRString(scope, "nationality", "country")
	fields.Address = firstOCRString(scope, "address", "addressLine")
	fields.IssuingAuthority = firstOCRString(scope, "issueAuthority", "issuingAuthority", "authority")
	fields.ValidFrom, fields.ValidTo = resolveOCRValidity(scope)
	fields.Confidence = resolveOCRConfidence(payload)

	if fields.FullName == "" && fields.IDNumber == "" {
		return nil, apperrors.NewUnavailable("aliyun_ocr: the provider returned no readable identity fields")
	}
	return fields, nil
}

func (r *AliyunOCRRecognizer) call(ctx context.Context, action string, image []byte) (map[string]any, error) {
	params := map[string]string{
		"AccessKeyId":      r.AccessKeyID,
		"Action":           action,
		"Format":           "JSON",
		"RegionId":         r.RegionID,
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   mustUUID(),
		"SignatureVersion": "1.0",
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"Version":          aliyunOCRAPIVersion,
	}
	params["Signature"] = aliyunRPCSignature(http.MethodPost, params, r.AccessKeySecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.Endpoint+"/?"+aliyunCanonicalQuery(params), bytes.NewReader(image))
	if err != nil {
		return nil, apperrors.WrapInternal(err, "aliyun_ocr: build request")
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Accept", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, apperrors.WrapUnavailablef(err, "aliyun_ocr: %s request failed", action)
	}
	defer resp.Body.Close()

	raw, err := readLimited(resp)
	if err != nil {
		return nil, err
	}
	if !isSuccessStatus(resp.StatusCode) {
		if code, message := aliyunRPCErrorCode(raw); code != "" {
			return nil, mapAliyunErrorCode(aliyunOCRProviderName, code, message, action)
		}
		return nil, mapProviderHTTPError(aliyunOCRProviderName, resp.StatusCode)
	}

	var result map[string]any
	if err := decodeJSON(aliyunOCRProviderName, raw, &result); err != nil {
		return nil, err
	}
	if code, ok := result["Code"].(string); ok && code != "" && code != "200" {
		return nil, mapAliyunErrorCode(aliyunOCRProviderName, code, "", action)
	}
	return result, nil
}

func aliyunOCRAction(documentType DocumentType) (string, bool) {
	switch documentType {
	case domainidv.DocumentIDCard:
		return "RecognizeIdcard", true
	case domainidv.DocumentPassport:
		return "RecognizePassport", true
	case domainidv.DocumentDriverLicense:
		return "RecognizeDriverLicense", true
	default:
		return "", false
	}
}

// aliyunCanonicalQuery renders signed parameters into the exact query string
// covered by the signature (percent-encoding as required by Aliyun RPC V1).
func aliyunCanonicalQuery(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, aliyunPercentEncode(key)+"="+aliyunPercentEncode(params[key]))
	}
	return strings.Join(pairs, "&")
}

// decodeOCRPayload unwraps the double-encoded Data field of the OCR response.
func decodeOCRPayload(result map[string]any) (map[string]any, error) {
	raw, ok := result["Data"].(string)
	if !ok || strings.TrimSpace(raw) == "" {
		return nil, apperrors.NewUnavailable("aliyun_ocr: response is missing Data")
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, apperrors.NewUnavailable("aliyun_ocr: Data is not valid JSON")
	}
	return payload, nil
}

// collectOCRObjects flattens the provider payload into every nested object that
// may carry fields. Id cards nest face/back, passports and licenses are flat or
// nested under data, and the shape differs per action.
func collectOCRObjects(payload map[string]any) []map[string]any {
	objects := []map[string]any{payload}
	queue := []map[string]any{payload}
	for depth := 0; depth < 3 && len(queue) > 0; depth++ {
		next := make([]map[string]any, 0, len(queue))
		for _, object := range queue {
			for _, key := range []string{"data", "face", "back", "front", "info", "result"} {
				child, ok := object[key].(map[string]any)
				if !ok {
					continue
				}
				objects = append(objects, child)
				next = append(next, child)
			}
		}
		queue = next
	}
	return objects
}

func firstOCRString(objects []map[string]any, keys ...string) string {
	for _, object := range objects {
		for _, key := range keys {
			value, ok := object[key]
			if !ok {
				continue
			}
			if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

func resolveOCRValidity(objects []map[string]any) (string, string) {
	if from := firstOCRString(objects, "validFrom", "validStart", "startDate", "issueDate"); from != "" {
		return from, firstOCRString(objects, "validTo", "validEnd", "endDate", "expiryDate")
	}
	period := firstOCRString(objects, "validPeriod", "validityPeriod", "validDate", "period")
	if period == "" {
		return "", ""
	}
	normalized := strings.NewReplacer("至", "-", "—", "-", "–", "-").Replace(period)
	parts := strings.Split(normalized, "-")
	if len(parts) < 2 {
		return strings.TrimSpace(period), ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

// resolveOCRConfidence maps the optional quality score (0-100) to 0-1. When the
// provider omits it the field stays zero, which the UI renders as "unknown".
func resolveOCRConfidence(payload map[string]any) float64 {
	for _, object := range collectOCRObjects(payload) {
		warning, ok := object["warning"].(map[string]any)
		if !ok {
			continue
		}
		score := numericOCRValue(warning["qualityScore"])
		if score <= 0 {
			continue
		}
		if score > 1 {
			score /= 100
		}
		return score
	}
	return 0
}

func numericOCRValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}
