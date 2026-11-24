package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// DingTalkNotifier sends notifications via DingTalk robot webhook.
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核，无偏差)：自定义机器人
// 加签 stringToSign = timestamp(ms)+"\n"+secret、sign=urlEncode(base64(
// HMAC-SHA256(key=secret)))，与官方安全设置文档一致：
// https://open.dingtalk.com/document/orgapp/customize-robot-security-settings
type DingTalkNotifier struct {
	HTTPClient *http.Client
}

// NewDingTalkNotifier creates a new DingTalk notifier.
func NewDingTalkNotifier() *DingTalkNotifier {
	return &DingTalkNotifier{
		HTTPClient: ssrf.DefaultHTTPClient,
	}
}

// dingTalkPayload represents the JSON payload sent to DingTalk webhook.
type dingTalkPayload struct {
	MsgType string       `json:"msgtype"`
	Text    dingTalkText `json:"text"`
}

// dingTalkText represents the text content in a DingTalk message.
type dingTalkText struct {
	Content string `json:"content"`
}

// dingTalkSign computes the webhook 加签 signature per the official spec:
// stringToSign = timestamp(ms) + "\n" + secret, sign = urlEncode(base64(
// HMAC-SHA256(key=secret, stringToSign))). The URL escaping is delegated to
// url.Values.Encode when the query params are appended.
func dingTalkSign(secret, timestamp string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "\n" + secret))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// dingTalkWebhookURL appends timestamp and sign query params when a 加签
// secret is provided via metadata ("sign_secret", falling back to "secret").
func dingTalkWebhookURL(rawURL, secret string) (string, error) {
	if secret == "" {
		return rawURL, nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", apperrors.WrapInvalidArgumentf(err, "dingtalk notification: invalid webhook URL")
	}
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	q := u.Query()
	q.Set("timestamp", timestamp)
	q.Set("sign", dingTalkSign(secret, timestamp))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (n *DingTalkNotifier) Send(ctx context.Context, notif *Notification) error {
	if notif.Recipient == "" {
		return apperrors.NewInvalidArgument("dingtalk notification: webhook URL is required")
	}

	secret := getMetadataString(notif, "sign_secret")
	if secret == "" {
		secret = getMetadataString(notif, "secret")
	}
	endpoint, err := dingTalkWebhookURL(notif.Recipient, secret)
	if err != nil {
		return err
	}

	payload := dingTalkPayload{
		MsgType: messageTypeText,
		Text: dingTalkText{
			Content: notif.Content,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "dingtalk notification: marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "dingtalk notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "dingtalk notification: send request")
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return apperrors.NewInternalf("dingtalk notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	// DingTalk answers HTTP 200 even for rejected messages; errcode is the
	// authoritative result per the custom-robot spec.
	return checkProviderCodeBody("dingtalk", string(respBody))
}
