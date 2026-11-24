package notification

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ServerChanNotifier sends messages through Server酱·Turbo (ftqq.com).
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方文档复核)：Turbo API 为
// POST https://sctapi.ftqq.com/{SendKey}.send，表单字段 title（必填，最长 32
// 字符）+ desp（Markdown 正文），成功信号是 JSON 体 code==0（HTTP 200 也可能
// 业务失败）。sctp 前缀的新短 key 必须发往
// https://{uid}.push.ft07.com/send/{SendKey}.send，uid 为 sctp 之后的连续
// 数字：https://sct.ftqq.com/docs/api
type ServerChanNotifier struct {
	SendKey    string
	HTTPClient *http.Client
}

// NewServerChanNotifier creates a new ServerChan notifier.
func NewServerChanNotifier(sendKey string) *ServerChanNotifier {
	return &ServerChanNotifier{
		SendKey:    sendKey,
		HTTPClient: ssrf.DefaultHTTPClient,
	}
}

// serverChanUID extracts the numeric uid embedded in an `sctp` short key
// (e.g. "sctp1234abcd" -> "1234"); legacy Turbo keys carry no uid.
func serverChanUID(sendKey string) string {
	if !strings.HasPrefix(sendKey, "sctp") {
		return ""
	}
	rest := sendKey[len("sctp"):]
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	return rest[:end]
}

func (n *ServerChanNotifier) endpoint() string {
	if uid := serverChanUID(n.SendKey); uid != "" {
		return "https://" + uid + ".push.ft07.com/send/" + n.SendKey + ".send"
	}
	return "https://sctapi.ftqq.com/" + n.SendKey + ".send"
}

func (n *ServerChanNotifier) Send(ctx context.Context, notif *Notification) error {
	if n.SendKey == "" {
		return apperrors.NewInvalidArgument("serverchan notification: sendKey is required")
	}

	title := strings.TrimSpace(notif.Subject)
	if title == "" {
		title = strings.TrimSpace(notif.Content)
	}
	if title == "" {
		title = "GuliangIAM"
	}
	title = firstRunes(title, 32)

	form := url.Values{}
	form.Set("title", title)
	form.Set("desp", notif.Content)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.endpoint(), strings.NewReader(form.Encode()))
	if err != nil {
		return apperrors.WrapInternalf(err, "serverchan notification: create request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "serverchan notification: send request")
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apperrors.NewInternalf("serverchan notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return checkProviderCodeBody("serverchan", string(respBody))
}

// firstRunes truncates s to at most max runes (Server酱 title and OpsGenie
// message limits are rune counts, not byte counts).
func firstRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

var _ Provider = (*ServerChanNotifier)(nil)
