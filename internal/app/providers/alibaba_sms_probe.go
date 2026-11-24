package providers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	platformssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// Alibaba Cloud SMS (Dysmsapi 2017-05-25) read-only verification endpoints.
//
// Test connection deliberately uses GetSmsSign + GetSmsTemplate instead of
// SendSms: both are read-only queries, send no SMS and incur no fee. A real
// delivery test still requires an explicit phone number and real credentials.
const (
	alibabaSMSEndpoint     = "https://dysmsapi.aliyuncs.com/"
	alibabaSMSAPIVersion   = "2017-05-25"
	alibabaSMSProbeTimeout = 10 * time.Second
)

type alibabaSMSProbePayload struct {
	Code           string `json:"Code"`
	Message        string `json:"Message"`
	RequestID      string `json:"RequestId"`
	SignName       string `json:"SignName"`
	SignStatus     int    `json:"SignStatus"`
	TemplateCode   string `json:"TemplateCode"`
	TemplateStatus int    `json:"TemplateStatus"`
}

// probeAlibabaSMS authenticates with the configured AccessKey pair and verifies
// that the configured signature and template exist and are approved. It never
// calls SendSms.
func probeAlibabaSMS(ctx context.Context, client *http.Client, endpoint string, p *domain.Provider) (*TestConnectionResult, error) {
	if p == nil {
		return &TestConnectionResult{Connected: false, Error: "alibaba sms provider detail is missing"}, nil
	}
	accessKeyID := strings.TrimSpace(p.Admin)
	accessKeySecret := strings.TrimSpace(p.Password)
	signName := strings.TrimSpace(p.Sender)
	templateCode := strings.TrimSpace(p.Template)
	if accessKeyID == "" || accessKeySecret == "" || signName == "" || templateCode == "" {
		// validateSMSProviderConfig runs before this function; keep a defensive
		// guard so a future caller cannot silently skip the required fields.
		return &TestConnectionResult{
			Connected: false,
			Error:     "alibaba sms configuration is incomplete: accessKeyId, accessKeySecret, signName and templateCode are required",
		}, nil
	}
	if client == nil {
		client = platformssrf.DefaultHTTPClient
	}
	if strings.TrimSpace(endpoint) == "" {
		endpoint = alibabaSMSEndpoint
	}

	started := time.Now()
	signPayload, result := alibabaSMSCall(ctx, client, endpoint, "GetSmsSign", map[string]string{
		"SignName": signName,
	}, accessKeyID, accessKeySecret)
	result.LatencyMs = time.Since(started).Milliseconds()
	if result.Error != "" {
		return result, nil
	}
	if signPayload.SignStatus != 1 {
		result.Connected = false
		result.Error = fmt.Sprintf("alibaba sms authenticated, but signName %q is not approved (SignStatus=%d); SendSms requires SignStatus=1", signName, signPayload.SignStatus)
		return result, nil
	}

	templatePayload, templateResult := alibabaSMSCall(ctx, client, endpoint, "GetSmsTemplate", map[string]string{
		"TemplateCode": templateCode,
	}, accessKeyID, accessKeySecret)
	// Report both probes as one verification cost.
	result.LatencyMs = time.Since(started).Milliseconds()
	result.StatusCode = templateResult.StatusCode
	if templateResult.Error != "" {
		result.Connected = false
		result.Error = templateResult.Error
		return result, nil
	}

	if templatePayload.TemplateStatus != 1 {
		result.Connected = false
		result.Error = fmt.Sprintf("alibaba sms authenticated, but templateCode %q is not approved (TemplateStatus=%d); SendSms requires TemplateStatus=1", templateCode, templatePayload.TemplateStatus)
		return result, nil
	}

	result.Connected = true
	result.Error = ""
	return result, nil
}

// alibabaSMSCall signs and executes one read-only Dysmsapi RPC request. The
// returned result only carries transport/API failures; business status checks
// stay with the caller.
func alibabaSMSCall(
	ctx context.Context,
	client *http.Client,
	endpoint, action string,
	businessParams map[string]string,
	accessKeyID, accessKeySecret string,
) (*alibabaSMSProbePayload, *TestConnectionResult) {
	params := map[string]string{
		"AccessKeyId":      accessKeyID,
		"Action":           action,
		"Format":           "JSON",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   alibabaSMSNonce(),
		"SignatureVersion": "1.0",
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"Version":          alibabaSMSAPIVersion,
	}
	for key, value := range businessParams {
		params[key] = value
	}

	canonical := alibabaSMSCanonicalQuery(params)
	signature := alibabaSMSSignature(http.MethodGet, params, accessKeySecret)
	requestURL := strings.TrimRight(endpoint, "/") + "/?" + canonical + "&Signature=" + alibabaSMSPercentEncode(signature)

	requestCtx, cancel := context.WithTimeout(ctx, alibabaSMSProbeTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		return nil, &TestConnectionResult{Connected: false, Error: "alibaba sms auth probe request could not be built"}
	}

	response, err := client.Do(request)
	if err != nil {
		// Do not surface err.Error(): the signed URL contains the AccessKeyId.
		return nil, &TestConnectionResult{Connected: false, Error: "alibaba sms auth probe failed: endpoint unreachable"}
	}
	defer response.Body.Close()

	result := &TestConnectionResult{StatusCode: response.StatusCode}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		result.Error = "alibaba sms auth probe failed: response body could not be read"
		return nil, result
	}

	var payload alibabaSMSProbePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		result.Error = fmt.Sprintf("alibaba sms auth probe returned HTTP %d with an unreadable body", response.StatusCode)
		return nil, result
	}
	if payload.Code != "OK" {
		result.Error = alibabaSMSProbeError(payload.Code, payload.Message)
		return nil, result
	}
	return &payload, result
}

// alibabaSMSProbeError maps the vendor error code to an actionable message.
// Keep the raw code in the text so support can search the official error list.
func alibabaSMSProbeError(code, message string) string {
	switch code {
	case "InvalidAccessKeyId.NotFound", "InvalidAccessKeyID.NotFound":
		return "alibaba sms authentication failed: accessKeyId does not exist (InvalidAccessKeyId.NotFound)"
	case "InvalidAccessKeyId.Inactive", "InvalidAccessKeyID.Inactive":
		return "alibaba sms authentication failed: accessKeyId is disabled (InvalidAccessKeyId.Inactive)"
	case "InvalidAccessKeySecret":
		return "alibaba sms authentication failed: accessKeySecret is invalid (InvalidAccessKeySecret)"
	case "SignatureDoesNotMatch", "IncompleteSignature":
		return fmt.Sprintf("alibaba sms authentication failed: signature mismatch, check accessKeySecret (%s)", code)
	case "InvalidTimeStamp.Expired":
		return "alibaba sms authentication failed: request timestamp expired (InvalidTimeStamp.Expired); check the host clock"
	case "SignatureNonceUsed":
		return "alibaba sms authentication failed: signature nonce was reused (SignatureNonceUsed)"
	case "isp.RAM_PERMISSION_DENY", "Forbidden.RAM":
		return fmt.Sprintf("alibaba sms signature was accepted, but the AccessKey lacks permission for the read-only probe (%s); sign/template configuration is NOT verified", code)
	case "isv.PRODUCT_UN_SUBSCRIPT", "isv.PRODUCT_UNSUBSCRIBE":
		return fmt.Sprintf("alibaba sms account has not activated the SMS product (%s)", code)
	case "isv.OUT_OF_SERVICE":
		return "alibaba sms account is suspended due to insufficient balance (isv.OUT_OF_SERVICE)"
	case "isv.AMOUNT_NOT_ENOUGH":
		return "alibaba sms account balance is insufficient (isv.AMOUNT_NOT_ENOUGH)"
	case "isv.ACCOUNT_NOT_EXISTS":
		return "alibaba sms account does not exist (isv.ACCOUNT_NOT_EXISTS); check the AccessKey owner"
	case "isv.SMS_SIGNATURE_ILLEGAL", "isv.SMS_SIGN_ILLEGAL", "isv.SIGN_NAME_ILLEGAL":
		return fmt.Sprintf("alibaba sms signName is not usable on this account (%s); apply for and approve the signature first", code)
	case "isv.SMS_TEMPLATE_ILLEGAL", "isv.TEMPLATE_PARAMS_ILLEGAL":
		return fmt.Sprintf("alibaba sms templateCode is not usable on this account (%s); apply for and approve the template first", code)
	default:
		if message == "" {
			return fmt.Sprintf("alibaba sms auth probe failed: code=%s", code)
		}
		return fmt.Sprintf("alibaba sms auth probe failed: code=%s message=%s", code, message)
	}
}

// alibabaSMSCanonicalQuery builds the sorted, percent-encoded query string used
// for both the request URL and the string-to-sign.
func alibabaSMSCanonicalQuery(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for i, key := range keys {
		if i > 0 {
			builder.WriteByte('&')
		}
		builder.WriteString(alibabaSMSPercentEncode(key))
		builder.WriteByte('=')
		builder.WriteString(alibabaSMSPercentEncode(params[key]))
	}
	return builder.String()
}

// alibabaSMSSignature implements the legacy RPC HMAC-SHA1 signature
// (SignatureVersion=1.0), which the existing AlibabaSmsSender in this
// repository already uses.
func alibabaSMSSignature(method string, params map[string]string, accessKeySecret string) string {
	stringToSign := method + "&" + alibabaSMSPercentEncode("/") + "&" + alibabaSMSPercentEncode(alibabaSMSCanonicalQuery(params))
	mac := hmac.New(sha1.New, []byte(accessKeySecret+"&"))
	_, _ = mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// alibabaSMSPercentEncode implements RFC 3986 percent-encoding as required by
// Alibaba Cloud: space becomes %20, '*' becomes %2A, '~' stays unescaped.
func alibabaSMSPercentEncode(value string) string {
	const upperHex = "0123456789ABCDEF"
	var builder strings.Builder
	builder.Grow(len(value))
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			builder.WriteByte(c)
			continue
		}
		builder.WriteByte('%')
		builder.WriteByte(upperHex[c>>4])
		builder.WriteByte(upperHex[c&0x0F])
	}
	return builder.String()
}

func alibabaSMSNonce() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// rand.Read failure is catastrophic; the timestamp still keeps collisions unlikely.
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}
