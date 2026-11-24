package messaging

import (
	"context"
	"crypto/md5" //nolint:gosec // signature spec mandates MD5
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// unismsTemplateSendURL is the 联麓 (shlianlu) template SMS endpoint.
const unismsTemplateSendURL = "https://apis.shlianlu.com/sms/trade/template/send"

// UnismsSmsSender sends SMS through 联麓信息 (Lianlu) template SMS API.
type UnismsSmsSender struct {
	MchID      string
	AppID      string
	AppKey     string
	TemplateID string
	client     *http.Client
}

// NewUnismsSmsSender creates a 联麓 template SMS sender.
func NewUnismsSmsSender(mchID, appID, appKey, templateID string) *UnismsSmsSender {
	return &UnismsSmsSender{
		MchID:      mchID,
		AppID:      appID,
		AppKey:     appKey,
		TemplateID: templateID,
		client:     ssrf.SafeHTTPClient(10 * time.Second),
	}
}

// unismsUnsignedParams lists the body fields excluded from the signature per
// the official 签名机制 doc (PhoneNumberSet/TemplateParamSet/Signature/...).
var unismsUnsignedParams = map[string]bool{
	"PhoneNumberSet":    true,
	"SessionContext":    true,
	"SessionContextSet": true,
	"ContextParamSet":   true,
	"TemplateParamSet":  true,
	"Signature":         true,
	"PhoneList":         true,
	"phoneSet":          true,
}

// unismsSignature sorts the signed params, joins key=value with & and appends
// &key=<appKey>, then MD5-hashes the string uppercased (联麓 MD5 签名).
func unismsSignature(params map[string]string, appKey string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if unismsUnsignedParams[k] || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(params[k])
	}
	sb.WriteString("&key=")
	sb.WriteString(appKey)
	sum := md5.Sum([]byte(sb.String())) //nolint:gosec // signature spec mandates MD5
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

// Send dispatches the verification code through the 联麓 template API.
func (s *UnismsSmsSender) Send(ctx context.Context, phone, code string) error {
	if s.MchID == "" || s.AppID == "" || s.AppKey == "" || s.TemplateID == "" {
		return apperrors.NewInvalidArgument("unisms mchId, appId, appKey and templateId are required")
	}

	params := map[string]string{
		"AppId":      s.AppID,
		"MchId":      s.MchID,
		"SignType":   "MD5",
		"TemplateId": s.TemplateID,
		"TimeStamp":  strconv.FormatInt(time.Now().UnixMilli(), 10),
		"Type":       "3",
		"Version":    "1.2.0",
	}
	body := map[string]any{
		"AppId":            params["AppId"],
		"MchId":            params["MchId"],
		"SignType":         params["SignType"],
		"TemplateId":       params["TemplateId"],
		"TimeStamp":        params["TimeStamp"],
		"Type":             params["Type"],
		"Version":          params["Version"],
		"PhoneNumberSet":   []string{phone},
		"TemplateParamSet": []string{code},
		"Signature":        unismsSignature(params, s.AppKey),
	}

	jsonData, err := jsonMarshal(body)
	if err != nil {
		return apperrors.WrapInternalf(err, "unisms: marshal")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, unismsTemplateSendURL, jsonReader(jsonData))
	if err != nil {
		return apperrors.WrapInternalf(err, "unisms: create request")
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "unisms: request")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return apperrors.NewTooManyRequests("unisms: rate limited (429)")
	}
	if resp.StatusCode >= 400 {
		return apperrors.NewInternalf("unisms: API error (status %d)", resp.StatusCode)
	}

	var out struct {
		Status  string `json:"status"`
		TaskID  string `json:"taskId"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return apperrors.WrapInternalf(err, "unisms: decode response")
	}
	// 联麓 returns status "00" for an accepted batch.
	if out.Status != "00" {
		return apperrors.NewInternalf("unisms: send failed (status %s, message %s)", out.Status, out.Message)
	}
	return nil
}

var _ SmsSender = (*UnismsSmsSender)(nil)
