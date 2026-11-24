package faceid

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // G505: Alibaba Cloud signature v1.0 mandates HMAC-SHA1
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	"github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	// aliyunFaceBodyDefaultEndpoint is the default Alibaba Cloud Face Body API endpoint.
	aliyunFaceBodyDefaultEndpoint = "https://facebody.cn-shanghai.aliyuncs.com"

	// aliyunAPISVersion is the API version for Alibaba Cloud Face Body API.
	aliyunAPISVersion = "2019-12-30"

	// aliyunSignatureMethod is the signature method for Alibaba Cloud API.
	aliyunSignatureMethod = "HMAC-SHA1"

	// aliyunSignatureVersion is the signature version.
	aliyunSignatureVersion = "1.0"

	// compareThreshold is the minimum confidence score for face comparison.
	// CompareFace returns Confidence on a 0-100 scale.
	compareThreshold = 80.0
)

// Calibration (OPEN-API-75 P5, 2026-11-14, official docs reviewed):
//   - 人脸比对 CompareFace（2019-12-30，请求参数 ImageURLA/ImageURLB，返回 Data.Confidence 0-100）:
//     https://next.api.aliyun.com/document/facebody/2019-12-30/CompareFace
//   - 人脸活体检测 DetectLivingFace（2019-12-30，请求参数 Tasks[].ImageURL，
//     返回 Data.Elements[].Results[].{Suggestion,Rate}）:
//     https://next.api.aliyun.com/document/facebody/2019-12-30/DetectLivingFace
//   - RPC V1 签名（HMAC-SHA1，percentEncode + StringToSign=GET&%2F&...）:
//     https://help.aliyun.com/document_detail/315597.html
//     本实现的 percentEncode/签名链路已按该文档复核，无偏差。
//
// 注意：Face Body 原始 RPC 仅接受可访问的图像 URL（本地/任意地域文件需先上传
// 上海 OSS，见文档「文件 URL 处理」），不支持直接提交 base64；此前实现的
// Action 名（CompareFaceFaces/DetectFaceLiving）与参数名均与官方文档不符。
// 另：阿里云已发布视觉智能开放平台（VIAPI）线上服务下线公告
// https://help.aliyun.com/zh/viapi/product-overview/september-2026/ ，新接入
// 建议优先使用实人认证（aliyun_idv/tencent_idv）等仍在维护的产品。

// ErrFaceNotLive is returned when liveness detection fails.
var ErrFaceNotLive = errors.NewUnauthorized("face liveness detection failed")

// ErrFaceNotMatch is returned when face comparison fails.
var ErrFaceNotMatch = errors.NewUnauthorized("face comparison failed")

// AliyunVerifier implements FaceVerifier using Alibaba Cloud Face Body API.
type AliyunVerifier struct {
	AccessKeyID     string
	AccessKeySecret string
	Endpoint        string
	HTTPClient      *http.Client
}

// NewAliyunVerifier creates a new AliyunVerifier instance.
func NewAliyunVerifier(accessKeyID, accessKeySecret, endpoint string) *AliyunVerifier {
	if endpoint == "" {
		endpoint = aliyunFaceBodyDefaultEndpoint
	}
	return &AliyunVerifier{
		AccessKeyID:     accessKeyID,
		AccessKeySecret: accessKeySecret,
		Endpoint:        endpoint,
		HTTPClient:      ssrf.SafeHTTPClient(30 * time.Second),
	}
}

// VerifyLiveness performs liveness detection with the official DetectLivingFace
// action (2019-12-30). The raw RPC accepts only an accessible image URL
// (Tasks[].ImageURL); base64 payloads must be uploaded to OSS first and are
// therefore rejected here. The verdict comes from
// Data.Elements[0].Results[0].Suggestion ("pass" = live) with Rate (0-100) as
// the score.
func (v *AliyunVerifier) VerifyLiveness(ctx context.Context, imageData, deviceInfo string) (*LivenessResult, error) {
	// Device info is passed as a hint but not required by the API.
	_ = deviceInfo
	imageURL, err := faceBodyImageURL(imageData)
	if err != nil {
		return nil, err
	}
	params := map[string]string{
		"Action":           "DetectLivingFace",
		"Version":          aliyunAPISVersion,
		"Tasks.1.ImageURL": imageURL,
	}

	respBody, err := v.callAPI(ctx, params)
	if err != nil {
		return nil, errors.WrapInternalf(err, "aliyun liveness detection")
	}

	var result struct {
		RequestID string `json:"RequestId"`
		Data      struct {
			Elements []struct {
				Results []struct {
					Suggestion string  `json:"Suggestion"`
					Rate       float64 `json:"Rate"`
				} `json:"Results"`
			} `json:"Elements"`
		} `json:"Data"`
		Code    string `json:"Code"`
		Message string `json:"Message"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, errors.WrapInternalf(err, "aliyun liveness decode")
	}

	if result.Code != "" && result.Code != "200" {
		return nil, errors.NewInternalf("aliyun liveness error: code=%s message=%s", result.Code, result.Message)
	}

	sessionID := result.RequestID
	if sessionID == "" {
		sessionID = fmt.Sprintf("aliyun-live-%d", time.Now().UnixNano())
	}
	var suggestion string
	var rate float64
	if len(result.Data.Elements) > 0 && len(result.Data.Elements[0].Results) > 0 {
		suggestion = strings.ToLower(result.Data.Elements[0].Results[0].Suggestion)
		rate = result.Data.Elements[0].Results[0].Rate
	}

	return &LivenessResult{
		IsLive:    suggestion == "pass",
		Score:     rate,
		SessionID: sessionID,
	}, nil
}

// CompareFace compares two face images with the official CompareFace action
// (2019-12-30). Both inputs must be accessible image URLs (ImageURLA /
// ImageURLB); the returned Data.Confidence is a 0-100 similarity score.
func (v *AliyunVerifier) CompareFace(ctx context.Context, imageData, storedTemplate string) (*CompareResult, error) {
	imageURL, err := faceBodyImageURL(imageData)
	if err != nil {
		return nil, err
	}
	storedURL, err := faceBodyImageURL(storedTemplate)
	if err != nil {
		return nil, err
	}
	params := map[string]string{
		"Action":    "CompareFace",
		"Version":   aliyunAPISVersion,
		"ImageURLA": imageURL,
		"ImageURLB": storedURL,
	}

	respBody, err := v.callAPI(ctx, params)
	if err != nil {
		return nil, errors.WrapInternalf(err, "aliyun face compare")
	}

	var result struct {
		Data struct {
			Confidence float64 `json:"Confidence"`
		} `json:"Data"`
		Code    string `json:"Code"`
		Message string `json:"Message"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, errors.WrapInternalf(err, "aliyun face compare decode")
	}

	if result.Code != "" && result.Code != "200" {
		return nil, errors.NewInternalf("aliyun face compare error: code=%s message=%s", result.Code, result.Message)
	}

	return &CompareResult{
		IsMatch:    result.Data.Confidence >= compareThreshold,
		Confidence: result.Data.Confidence,
	}, nil
}

// faceBodyImageURL validates that a Face Body input is an accessible image
// URL. The raw RPC does not accept base64 image data (see the 「文件 URL
// 处理」 section of the CompareFace docs).
func faceBodyImageURL(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed, nil
	}
	return "", errors.NewInvalidArgument("aliyun facebody: image input must be an accessible image URL; base64 upload requires an OSS object URL")
}

// callAPI makes an authenticated API call to Alibaba Cloud using the HMAC-SHA1 signature method.
func (v *AliyunVerifier) callAPI(ctx context.Context, params map[string]string) ([]byte, error) {
	// Add common parameters.
	params["AccessKeyId"] = v.AccessKeyID
	params["SignatureMethod"] = aliyunSignatureMethod
	params["SignatureVersion"] = aliyunSignatureVersion
	params["SignatureNonce"] = fmt.Sprintf("%d", time.Now().UnixNano())
	params["Timestamp"] = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	params["Format"] = "JSON"

	// Compute signature.
	signature := v.computeSignature("GET", params)
	params["Signature"] = signature

	// Build the query string with all parameters including signature.
	values := make(url.Values, len(params))
	for k, val := range params {
		values.Set(k, val)
	}

	reqURL := fmt.Sprintf("%s/?%s", v.Endpoint, values.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, errors.WrapInternalf(err, "create request")
	}

	resp, err := v.HTTPClient.Do(req)
	if err != nil {
		return nil, errors.WrapInternalf(err, "http request")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.WrapInternalf(err, "read response")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.NewInternalf("aliyun api status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// computeSignature computes the HMAC-SHA1 signature for Alibaba Cloud API.
func (v *AliyunVerifier) computeSignature(method string, params map[string]string) string {
	// Sort parameter keys.
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build canonicalized query string.
	var buf bytes.Buffer
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(percentEncode(k))
		buf.WriteByte('=')
		buf.WriteString(percentEncode(params[k]))
	}
	canonicalizedQueryString := buf.String()

	// Construct string to sign.
	// StringToSign = HTTPMethod + "&" + percentEncode("/") + "&" + percentEncode(CanonicalizedQueryString)
	stringToSign := method + "&" + percentEncode("/") + "&" + percentEncode(canonicalizedQueryString)

	// Sign with HMAC-SHA1.
	key := v.AccessKeySecret + "&"
	mac := hmac.New(sha1.New, []byte(key))
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return signature
}

// percentEncode encodes a string per Alibaba Cloud specifications.
// It is similar to URL encoding but uses uppercase hex and encodes
// additional characters: + → %20, * → %2A, ~ → %7E.
func percentEncode(s string) string {
	encoded := url.QueryEscape(s)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}
