package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// WeComAppNotifier delivers messages through a self-built WeCom application
// (应用消息), addressed to a WeCom user id. This is different from
// WeComNotifier, which posts to a group robot webhook.
//
// 官方文档：https://developer.work.weixin.qq.com/document/path/90236（发送应用消息）
// 与 https://developer.work.weixin.qq.com/document/path/91039（获取 access_token）。
type WeComAppNotifier struct {
	HTTPClient *http.Client
	// BaseURL lets tests point at a local server; production keeps the default.
	BaseURL string

	CorpID  string
	Secret  string
	AgentID int

	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
}

// NewWeComAppNotifier builds the notifier from the provider row config. The
// secret is the application secret (stored in the encrypted access_token
// column), corp_id/agent_id live in the non-secret config map.
func NewWeComAppNotifier(corpID, secret, agentID string) (*WeComAppNotifier, error) {
	id, err := strconv.Atoi(agentID)
	if err != nil || id <= 0 {
		return nil, apperrors.NewInvalidArgument("wecom_app notification: agent_id must be a positive integer")
	}
	if corpID == "" || secret == "" {
		return nil, apperrors.NewInvalidArgument("wecom_app notification: corp_id and secret are required")
	}
	return &WeComAppNotifier{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		BaseURL:    "https://qyapi.weixin.qq.com",
		CorpID:     corpID,
		Secret:     secret,
		AgentID:    id,
	}, nil
}

type weComTokenResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type weComAppPayload struct {
	ToUser  string     `json:"touser"`
	ToParty string     `json:"toparty"`
	ToTag   string     `json:"totag"`
	MsgType string     `json:"msgtype"`
	AgentID int        `json:"agentid"`
	Text    *weComText `json:"text,omitempty"`
	Safe    int        `json:"safe"`
}

func (n *WeComAppNotifier) Send(ctx context.Context, notif *Notification) error {
	if notif.Recipient == "" {
		return apperrors.NewInvalidArgument("wecom_app notification: recipient (WeCom user id) is required")
	}
	token, err := n.token(ctx)
	if err != nil {
		return err
	}

	payload := weComAppPayload{
		ToUser:  notif.Recipient,
		MsgType: messageTypeText,
		AgentID: n.AgentID,
		Text:    &weComText{Content: notif.Content},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "wecom_app notification: marshal payload")
	}
	endpoint := fmt.Sprintf("%s/cgi-bin/message/send?access_token=%s", n.BaseURL, url.QueryEscape(token))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "wecom_app notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "wecom_app notification: send request")
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return apperrors.NewInternalf("wecom_app notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return checkProviderCodeBody("wecom_app", string(respBody))
}

func (n *WeComAppNotifier) token(ctx context.Context) (string, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.accessToken != "" && time.Now().Before(n.expiresAt) {
		return n.accessToken, nil
	}

	endpoint := fmt.Sprintf("%s/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		n.BaseURL, url.QueryEscape(n.CorpID), url.QueryEscape(n.Secret))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "wecom_app notification: create token request")
	}
	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "wecom_app notification: token request")
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var parsed weComTokenResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", apperrors.WrapInternalf(err, "wecom_app notification: decode token response")
	}
	if parsed.ErrCode != 0 {
		return "", apperrors.NewInternalf("wecom_app notification: token rejected (%d): %s", parsed.ErrCode, parsed.ErrMsg)
	}
	if parsed.AccessToken == "" {
		return "", apperrors.NewInternal("wecom_app notification: empty access_token")
	}

	// Refresh two minutes early so a token cannot expire mid-flight.
	ttl := time.Duration(parsed.ExpiresIn) * time.Second
	if ttl > 2*time.Minute {
		ttl -= 2 * time.Minute
	}
	n.accessToken = parsed.AccessToken
	n.expiresAt = time.Now().Add(ttl)
	return n.accessToken, nil
}
