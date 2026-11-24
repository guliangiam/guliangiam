package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// WeChatOfficialNotifier delivers template messages through a WeChat Official
// Account (公众号模板消息) to a user's openid.
//
// 官方文档：https://developers.weixin.qq.com/doc/offiaccount/Message_Management/Template_Message_Interface.html
// access_token：https://developers.weixin.qq.com/doc/offiaccount/Basic_Information/Get_access_token.html
type WeChatOfficialNotifier struct {
	HTTPClient *http.Client
	// BaseURL lets tests point at a local server; production keeps the default.
	BaseURL string

	AppID      string
	AppSecret  string
	TemplateID string
	// Link is the default jump URL when the notification carries none.
	Link string

	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
}

// NewWeChatOfficialNotifier builds the notifier from the provider row config.
// The app secret is stored in the encrypted access_token column; app_id,
// template_id and the default link are non-secret config values.
func NewWeChatOfficialNotifier(appID, appSecret, templateID, link string) (*WeChatOfficialNotifier, error) {
	if appID == "" || appSecret == "" || templateID == "" {
		return nil, apperrors.NewInvalidArgument("wechat_official notification: app_id, secret and template_id are required")
	}
	return &WeChatOfficialNotifier{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		BaseURL:    "https://api.weixin.qq.com",
		AppID:      appID,
		AppSecret:  appSecret,
		TemplateID: templateID,
		Link:       link,
	}, nil
}

type weChatTokenResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type weChatTemplateValue struct {
	Value string `json:"value"`
	Color string `json:"color,omitempty"`
}

type weChatTemplatePayload struct {
	ToUser     string                         `json:"touser"`
	TemplateID string                         `json:"template_id"`
	URL        string                         `json:"url,omitempty"`
	Data       map[string]weChatTemplateValue `json:"data"`
}

func (n *WeChatOfficialNotifier) Send(ctx context.Context, notif *Notification) error {
	if notif.Recipient == "" {
		return apperrors.NewInvalidArgument("wechat_official notification: recipient (openid) is required")
	}
	token, err := n.token(ctx)
	if err != nil {
		return err
	}

	link := notif.MetadataString("link")
	if link == "" {
		link = n.Link
	}
	// Template keyword names are dictated by the template chosen in the WeChat
	// console; callers may override them through metadata["template_data"].
	data := map[string]weChatTemplateValue{
		"first":    {Value: notif.Subject},
		"keyword1": {Value: notif.Content},
		"remark":   {Value: time.Now().Format("2006-01-02 15:04")},
	}
	if overrides := notif.MetadataTemplateData(); len(overrides) > 0 {
		data = map[string]weChatTemplateValue{}
		for key, value := range overrides {
			data[key] = weChatTemplateValue{Value: value}
		}
	}

	payload := weChatTemplatePayload{
		ToUser:     notif.Recipient,
		TemplateID: n.TemplateID,
		URL:        link,
		Data:       data,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "wechat_official notification: marshal payload")
	}
	endpoint := fmt.Sprintf("%s/cgi-bin/message/template/send?access_token=%s", n.BaseURL, url.QueryEscape(token))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "wechat_official notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "wechat_official notification: send request")
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return apperrors.NewInternalf("wechat_official notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return checkProviderCodeBody("wechat_official", string(respBody))
}

func (n *WeChatOfficialNotifier) token(ctx context.Context) (string, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.accessToken != "" && time.Now().Before(n.expiresAt) {
		return n.accessToken, nil
	}

	endpoint := fmt.Sprintf("%s/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		n.BaseURL, url.QueryEscape(n.AppID), url.QueryEscape(n.AppSecret))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "wechat_official notification: create token request")
	}
	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "wechat_official notification: token request")
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var parsed weChatTokenResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", apperrors.WrapInternalf(err, "wechat_official notification: decode token response")
	}
	if parsed.AccessToken == "" {
		return "", apperrors.NewInternalf("wechat_official notification: token rejected (%d): %s", parsed.ErrCode, parsed.ErrMsg)
	}

	ttl := time.Duration(parsed.ExpiresIn) * time.Second
	if ttl > 2*time.Minute {
		ttl -= 2 * time.Minute
	}
	n.accessToken = parsed.AccessToken
	n.expiresAt = time.Now().Add(ttl)
	return n.accessToken, nil
}
