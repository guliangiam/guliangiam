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
	"strconv"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// LarkNotifier sends notifications via Lark/Feishu robot webhook.
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核，无偏差)：自定义机器人
// 签名校验 stringToSign = timestamp(s)+"\n"+secret 作为 HMAC-SHA256 密钥、对空
// 数据签名后 base64，与官方文档一致：
// https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot
type LarkNotifier struct {
	HTTPClient *http.Client
}

// NewLarkNotifier creates a new Lark notifier.
func NewLarkNotifier() *LarkNotifier {
	return &LarkNotifier{
		HTTPClient: ssrf.DefaultHTTPClient,
	}
}

// larkPayload represents the JSON payload sent to Lark webhook.
type larkPayload struct {
	Timestamp string      `json:"timestamp,omitempty"`
	Sign      string      `json:"sign,omitempty"`
	MsgType   string      `json:"msg_type"`
	Content   larkContent `json:"content"`
}

// larkContent represents the text content in a Lark message.
type larkContent struct {
	Text string `json:"text"`
}

// larkSign computes the custom-bot 签名校验 value per the official spec:
// stringToSign = timestamp(s) + "\n" + secret; sign = base64(HMAC-SHA256(
// key=stringToSign, data="")).
func larkSign(secret, timestamp string) string {
	stringToSign := timestamp + "\n" + secret
	mac := hmac.New(sha256.New, []byte(stringToSign))
	mac.Write([]byte{})
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (n *LarkNotifier) Send(ctx context.Context, notif *Notification) error {
	if notif.Recipient == "" {
		return apperrors.NewInvalidArgument("lark notification: webhook URL is required")
	}

	payload := larkPayload{
		MsgType: messageTypeText,
		Content: larkContent{
			Text: notif.Content,
		},
	}
	secret := getMetadataString(notif, "sign_secret")
	if secret == "" {
		secret = getMetadataString(notif, "secret")
	}
	if secret != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		payload.Timestamp = timestamp
		payload.Sign = larkSign(secret, timestamp)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "lark notification: marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, notif.Recipient, bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "lark notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "lark notification: send request")
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return apperrors.NewInternalf("lark notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	// Lark answers HTTP 200 even for rejected messages; body code is the
	// authoritative result per the custom-bot spec.
	return checkProviderCodeBody("lark", string(respBody))
}
