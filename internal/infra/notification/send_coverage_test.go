package notification

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
)

// roundTripFunc implements http.RoundTripper for testing
type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func newTestClient(fn roundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}

func makeNotif() *Notification {
	return &Notification{Type: "test", Subject: "title", Content: "body", Recipient: "test"}
}

// recordedRequest captures the outgoing request for contract assertions.
type recordedRequest struct {
	method string
	url    *url.URL
	header http.Header
	body   []byte
}

// captureClient returns a client that records the request and replies with the
// given status/body.
func captureClient(statusCode int, respBody string) (*http.Client, *recordedRequest) {
	rec := &recordedRequest{}
	client := newTestClient(func(req *http.Request) *http.Response {
		rec.method = req.Method
		rec.url = req.URL
		rec.header = req.Header.Clone()
		if req.Body != nil {
			rec.body, _ = io.ReadAll(req.Body)
		}
		var body io.ReadCloser = http.NoBody
		if respBody != "" {
			body = io.NopCloser(strings.NewReader(respBody))
		}
		return &http.Response{StatusCode: statusCode, Body: body, Header: http.Header{}}
	})
	return client, rec
}

func decodeJSONBody(t *testing.T, rec *recordedRequest) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(rec.body, &m), "request body must be JSON: %s", string(rec.body))
	return m
}

// --- BarkNotifier Send ---

func TestBarkNotifier_Send_Success(t *testing.T) {
	n := NewBarkNotifier()
	client, rec := captureClient(http.StatusOK, "")
	n.HTTPClient = client
	notif := makeNotif()
	notif.Recipient = "devicekey"
	notif.Content = "hello world!"
	require.NoError(t, n.Send(context.Background(), notif))

	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "api.day.app", rec.url.Host)
	// Path segments must be percent-encoded on the wire, decoded back on read.
	assert.Contains(t, rec.url.EscapedPath(), "hello%20world")
	assert.Equal(t, "/devicekey/hello world!", rec.url.Path)
}

func TestBarkNotifier_Send_Non2xx(t *testing.T) {
	n := NewBarkNotifier()
	n.HTTPClient = newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: http.NoBody}
	})
	err := n.Send(context.Background(), makeNotif())
	assert.Error(t, err)
}

// --- CustomHTTPNotifier Send ---

func TestCustomHTTPNotifier_Send_Success(t *testing.T) {
	n := NewCustomHTTPNotifier("https://webhook.example.com/hook", map[string]string{"X-Auth": "token"})
	client, rec := captureClient(http.StatusOK, "")
	n.HTTPClient = client
	require.NoError(t, n.Send(context.Background(), makeNotif()))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "https://webhook.example.com/hook", rec.url.String())
	assert.Equal(t, "application/json", rec.header.Get("Content-Type"))
	assert.Equal(t, "token", rec.header.Get("X-Auth"))
	body := decodeJSONBody(t, rec)
	assert.Equal(t, "test", body["type"])
	assert.Equal(t, "test", body["recipient"])
	assert.Equal(t, "title", body["subject"])
	assert.Equal(t, "body", body["content"])
}

func TestCustomHTTPNotifier_Send_Non2xx(t *testing.T) {
	n := NewCustomHTTPNotifier("https://webhook.example.com", nil)
	n.HTTPClient = newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: http.NoBody}
	})
	err := n.Send(context.Background(), makeNotif())
	assert.Error(t, err)
}

// --- DingTalkNotifier Send ---

func TestDingTalkNotifier_Send_Success(t *testing.T) {
	n := NewDingTalkNotifier()
	client, rec := captureClient(http.StatusOK, `{"errcode":0,"errmsg":"ok"}`)
	n.HTTPClient = client
	notif := makeNotif()
	notif.Recipient = "https://oapi.dingtalk.com/robot/send?access_token=fake-token"
	require.NoError(t, n.Send(context.Background(), notif))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "oapi.dingtalk.com", rec.url.Host)
	assert.Equal(t, "/robot/send", rec.url.Path)
	assert.Equal(t, "fake-token", rec.url.Query().Get("access_token"))
	assert.Empty(t, rec.url.Query().Get("sign"), "no secret configured means no sign params")
	assert.Equal(t, "application/json", rec.header.Get("Content-Type"))
	body := decodeJSONBody(t, rec)
	assert.Equal(t, "text", body["msgtype"])
	text, ok := body["text"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "body", text["content"])
}

func TestDingTalkNotifier_Send_Sign(t *testing.T) {
	n := NewDingTalkNotifier()
	client, rec := captureClient(http.StatusOK, `{"errcode":0,"errmsg":"ok"}`)
	n.HTTPClient = client
	secret := "SECfake-sign-secret"
	notif := makeNotif()
	notif.Recipient = "https://oapi.dingtalk.com/robot/send?access_token=fake-token"
	notif.Metadata = map[string]any{"sign_secret": secret}
	require.NoError(t, n.Send(context.Background(), notif))

	q := rec.url.Query()
	timestamp := q.Get("timestamp")
	require.NotEmpty(t, timestamp, "加签 requires timestamp query param")
	sign := q.Get("sign")
	require.NotEmpty(t, sign, "加签 requires sign query param")

	// Recompute per the official algorithm: HMAC-SHA256(key=secret,
	// msg=timestamp+"\n"+secret), base64 — independent of the notifier helper.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "\n" + secret))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	assert.Equal(t, expected, sign)
}

func TestDingTalkNotifier_Send_ProgrammaticErrcode(t *testing.T) {
	n := NewDingTalkNotifier()
	n.HTTPClient = newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"errcode":310000,"errmsg":"keywords not in content"}`)),
		}
	})
	err := n.Send(context.Background(), makeNotif())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "310000")
}

// --- DiscordNotifier Send ---

func TestDiscordNotifier_Send_Success(t *testing.T) {
	n := NewDiscordNotifier()
	client, rec := captureClient(http.StatusNoContent, "")
	n.HTTPClient = client
	notif := makeNotif()
	notif.Recipient = "https://discord.com/api/webhooks/123456/fake-secret"
	require.NoError(t, n.Send(context.Background(), notif))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "discord.com", rec.url.Host)
	assert.Equal(t, "/api/webhooks/123456/fake-secret", rec.url.Path)
	assert.Equal(t, "application/json", rec.header.Get("Content-Type"))
	body := decodeJSONBody(t, rec)
	assert.Equal(t, "body", body["content"])
}

func TestDiscordNotifier_Send_Non2xx(t *testing.T) {
	n := NewDiscordNotifier()
	n.HTTPClient = newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusBadRequest, Body: http.NoBody}
	})
	err := n.Send(context.Background(), makeNotif())
	assert.Error(t, err)
}

// --- GoogleChatNotifier Send ---

func TestGoogleChatNotifier_Send_Success(t *testing.T) {
	n := NewGoogleChatNotifier("https://chat.googleapis.com/v1/spaces/fake/messages?key=fake-key")
	client, rec := captureClient(http.StatusOK, "")
	n.HTTPClient = client
	require.NoError(t, n.Send(context.Background(), makeNotif()))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "chat.googleapis.com", rec.url.Host)
	assert.Equal(t, "application/json", rec.header.Get("Content-Type"))
	body := decodeJSONBody(t, rec)
	assert.Equal(t, "*title*\nbody", body["text"], "Google Chat webhook payload uses the text field")
}

// --- LarkNotifier Send ---

func TestLarkNotifier_Send_Success(t *testing.T) {
	n := NewLarkNotifier()
	client, rec := captureClient(http.StatusOK, `{"code":0,"msg":"success"}`)
	n.HTTPClient = client
	notif := makeNotif()
	notif.Recipient = "https://open.feishu.cn/open-apis/bot/v2/hook/fake-hook-token"
	require.NoError(t, n.Send(context.Background(), notif))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "open.feishu.cn", rec.url.Host)
	assert.Equal(t, "/open-apis/bot/v2/hook/fake-hook-token", rec.url.Path)
	assert.Equal(t, "application/json", rec.header.Get("Content-Type"))
	body := decodeJSONBody(t, rec)
	assert.Equal(t, "text", body["msg_type"], "Lark uses msg_type, not msgtype")
	content, ok := body["content"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "body", content["text"])
	_, hasSign := body["sign"]
	assert.False(t, hasSign, "no secret configured means no sign in body")
}

func TestLarkNotifier_Send_Sign(t *testing.T) {
	n := NewLarkNotifier()
	client, rec := captureClient(http.StatusOK, `{"code":0,"msg":"success"}`)
	n.HTTPClient = client
	secret := "fake-lark-robot-secret"
	notif := makeNotif()
	notif.Recipient = "https://open.feishu.cn/open-apis/bot/v2/hook/fake-hook-token"
	notif.Metadata = map[string]any{"secret": secret}
	require.NoError(t, n.Send(context.Background(), notif))

	body := decodeJSONBody(t, rec)
	timestamp, ok := body["timestamp"].(string)
	require.True(t, ok, "签名校验 requires timestamp claim in body")
	sign, ok := body["sign"].(string)
	require.True(t, ok, "签名校验 requires sign claim in body")

	// Recompute per the official algorithm: HMAC-SHA256(key=timestamp+"\n"+
	// secret, data=""), base64 — independent of the notifier helper.
	mac := hmac.New(sha256.New, []byte(timestamp+"\n"+secret))
	mac.Write([]byte{})
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	assert.Equal(t, expected, sign)
}

func TestLarkNotifier_Send_ProgrammaticCode(t *testing.T) {
	n := NewLarkNotifier()
	n.HTTPClient = newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"code":19021,"msg":"sign match fail"}`)),
		}
	})
	err := n.Send(context.Background(), makeNotif())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "19021")
}

// --- LineNotifier Send ---

func TestLineNotifier_Send_Success(t *testing.T) {
	n := NewLineNotifier("fake-channel-access-token")
	client, rec := captureClient(http.StatusOK, `{"message_ids":["fake-id"]}`)
	n.HTTPClient = client
	notif := makeNotif()
	notif.Recipient = "U1234567890abcdef1234567890abcdef"
	require.NoError(t, n.Send(context.Background(), notif))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "api.line.me", rec.url.Host)
	assert.Equal(t, "/v2/bot/message/push", rec.url.Path, "LINE Notify was retired 2025-03-31; push API is the successor")
	assert.Equal(t, "application/json", rec.header.Get("Content-Type"))
	assert.Equal(t, "Bearer fake-channel-access-token", rec.header.Get("Authorization"))
	body := decodeJSONBody(t, rec)
	assert.Equal(t, notif.Recipient, body["to"])
	messages, ok := body["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 1)
	msg, ok := messages[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "text", msg["type"])
	assert.Equal(t, "body", msg["text"])
}

// --- MatrixNotifier Send ---

func TestMatrixNotifier_Send_Success(t *testing.T) {
	n := NewMatrixNotifier("https://matrix.example.com", "fake-access-token")
	client, rec := captureClient(http.StatusOK, `{"event_id":"$fakeevent"}`)
	n.HTTPClient = client
	notif := makeNotif()
	notif.Recipient = "!roomid:matrix.example.com"
	require.NoError(t, n.Send(context.Background(), notif))

	assert.Equal(t, http.MethodPut, rec.method, "Matrix message send is PUT")
	assert.Equal(t, "matrix.example.com", rec.url.Host)
	assert.Equal(t, "application/json", rec.header.Get("Content-Type"))
	assert.Equal(t, "Bearer fake-access-token", rec.header.Get("Authorization"))

	// PUT /_matrix/client/v3/rooms/{roomId}/send/m.room.message/{txnId}
	segments := strings.Split(strings.TrimPrefix(rec.url.Path, "/"), "/")
	require.Len(t, segments, 8)
	assert.Equal(t, []string{"_matrix", "client", "v3", "rooms", "send", "m.room.message"},
		[]string{segments[0], segments[1], segments[2], segments[3], segments[5], segments[6]})
	assert.Equal(t, "!roomid:matrix.example.com", segments[4], "roomId must round-trip through path escaping")
	assert.NotContains(t, segments[7], "%!d", "txnId must never render a Go format error")
	assert.True(t, strings.HasPrefix(segments[7], "guliangiam-"), "txnId prefix: %s", segments[7])

	body := decodeJSONBody(t, rec)
	assert.Equal(t, "m.text", body["msgtype"])
	assert.Equal(t, "body", body["body"])
}

func TestMatrixNotifier_Send_TxnIDUnique(t *testing.T) {
	n := NewMatrixNotifier("https://matrix.example.com", "fake-access-token")

	var txnIDs []string
	for i := 0; i < 2; i++ {
		client, rec := captureClient(http.StatusOK, `{"event_id":"$fakeevent"}`)
		n.HTTPClient = client
		notif := makeNotif()
		notif.Recipient = "!roomid:matrix.example.com"
		require.NoError(t, n.Send(context.Background(), notif))
		segments := strings.Split(strings.TrimPrefix(rec.url.Path, "/"), "/")
		txnIDs = append(txnIDs, segments[7])
	}
	assert.NotEqual(t, txnIDs[0], txnIDs[1], "txnId must be unique without a request ID in context")
}

func TestMatrixNotifier_Send_TxnIDUniquePerRequestID(t *testing.T) {
	// One request can fan out to several notifications; the shared requestID
	// must not collapse them into one txnId (Matrix would dedupe the rest).
	n := NewMatrixNotifier("https://matrix.example.com", "fake-access-token")
	ctx := context.WithValue(context.Background(), "requestID", "req-shared")

	var txnIDs []string
	for i := 0; i < 2; i++ {
		client, rec := captureClient(http.StatusOK, `{"event_id":"$fakeevent"}`)
		n.HTTPClient = client
		notif := makeNotif()
		notif.Recipient = "!roomid:matrix.example.com"
		require.NoError(t, n.Send(ctx, notif))
		segments := strings.Split(strings.TrimPrefix(rec.url.Path, "/"), "/")
		txnIDs = append(txnIDs, segments[7])
	}
	assert.NotEqual(t, txnIDs[0], txnIDs[1], "txnId must stay unique for repeated sends sharing one requestID")
	assert.True(t, strings.HasPrefix(txnIDs[0], "guliangiam-req-shared-"), "requestID must remain traceable in the txnId: %s", txnIDs[0])
}

// --- PushbulletNotifier Send ---

func TestPushbulletNotifier_Send_Success(t *testing.T) {
	n := NewPushbulletNotifier("fake-access-token")
	client, rec := captureClient(http.StatusOK, `{"type":"note"}`)
	n.HTTPClient = client
	notif := makeNotif()
	notif.Recipient = "user@example.com"
	require.NoError(t, n.Send(context.Background(), notif))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "api.pushbullet.com", rec.url.Host)
	assert.Equal(t, "/v2/pushes", rec.url.Path)
	assert.Equal(t, "fake-access-token", rec.header.Get("Access-Token"))
	body := decodeJSONBody(t, rec)
	assert.Equal(t, "note", body["type"])
	assert.Equal(t, "title", body["title"])
	assert.Equal(t, "body", body["body"])
	assert.Equal(t, "user@example.com", body["email"])
}

// --- PushoverNotifier Send ---

func TestPushoverNotifier_Send_Success(t *testing.T) {
	n := NewPushoverNotifier("fake-app-token")
	client, rec := captureClient(http.StatusOK, `{"status":1,"request":"fake"}`)
	n.HTTPClient = client
	notif := makeNotif()
	notif.Recipient = "fake-user-key"
	require.NoError(t, n.Send(context.Background(), notif))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "api.pushover.net", rec.url.Host)
	assert.Equal(t, "/1/messages.json", rec.url.Path)
	// Pushover accepts JSON with Content-Type: application/json per its API docs.
	assert.Equal(t, "application/json", rec.header.Get("Content-Type"))
	body := decodeJSONBody(t, rec)
	assert.Equal(t, "fake-app-token", body["token"])
	assert.Equal(t, "fake-user-key", body["user"])
	assert.Equal(t, "body", body["message"])
	assert.Equal(t, "title", body["title"])
}

// --- RedditNotifier Send ---

func TestRedditNotifier_Send_Success(t *testing.T) {
	n := NewRedditNotifierWithConfig("fake-access-token", "fake-client-id", "fake-client-secret", "fake-user")
	client, rec := captureClient(http.StatusOK, `{"json":{"was_sent":true}}`)
	n.HTTPClient = client
	notif := makeNotif()
	notif.Recipient = "target_user"
	require.NoError(t, n.Send(context.Background(), notif))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "oauth.reddit.com", rec.url.Host)
	assert.Equal(t, "/api/compose", rec.url.Path)
	assert.Equal(t, "Bearer fake-access-token", rec.header.Get("Authorization"))
	assert.Contains(t, rec.header.Get("User-Agent"), "fake-user", "Reddit requires a descriptive User-Agent")
	body := decodeJSONBody(t, rec)
	assert.Equal(t, "target_user", body["to"])
	assert.Equal(t, "title", body["subject"])
	assert.Equal(t, "body", body["text"])
}

// --- RocketChatNotifier Send ---

func TestRocketChatNotifier_Send_Success(t *testing.T) {
	n := NewRocketChatNotifier()
	client, rec := captureClient(http.StatusOK, "")
	n.HTTPClient = client
	notif := makeNotif()
	notif.Recipient = "https://rocketchat.example.com/hooks/fake/fake-token"
	require.NoError(t, n.Send(context.Background(), notif))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "application/json", rec.header.Get("Content-Type"))
	body := decodeJSONBody(t, rec)
	assert.Equal(t, "body", body["text"])
}

// --- SlackNotifier Send ---

func TestSlackNotifier_Send_Success(t *testing.T) {
	n := NewSlackNotifier()
	client, rec := captureClient(http.StatusOK, "ok")
	n.HTTPClient = client
	notif := makeNotif()
	notif.Recipient = "https://hooks.slack.com/services/fake/fake/fake"
	require.NoError(t, n.Send(context.Background(), notif))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "hooks.slack.com", rec.url.Host)
	assert.Equal(t, "application/json", rec.header.Get("Content-Type"))
	body := decodeJSONBody(t, rec)
	assert.Equal(t, "body", body["text"])
}

func TestSlackNotifier_Send_Non2xx(t *testing.T) {
	n := NewSlackNotifier()
	n.HTTPClient = newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: http.NoBody}
	})
	err := n.Send(context.Background(), makeNotif())
	assert.Error(t, err)
}

// --- TeamsNotifier Send ---

func TestTeamsNotifier_Send_Success(t *testing.T) {
	n := NewTeamsNotifier()
	client, rec := captureClient(http.StatusOK, "")
	n.HTTPClient = client
	notif := makeNotif()
	notif.Recipient = "https://fake.org/fake-webhook-url"
	require.NoError(t, n.Send(context.Background(), notif))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "application/json", rec.header.Get("Content-Type"))
	body := decodeJSONBody(t, rec)
	assert.Equal(t, "message", body["type"], "Teams workflow webhook adaptive card envelope")
	attachments, ok := body["attachments"].([]any)
	require.True(t, ok)
	require.Len(t, attachments, 1)
	attachment, ok := attachments[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "application/vnd.microsoft.card.adaptive", attachment["contentType"])
	content, ok := attachment["content"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "AdaptiveCard", content["type"])
	assert.Equal(t, "1.4", content["version"])
}

// --- TelegramNotifier Send ---

func TestTelegramNotifier_Send_Success(t *testing.T) {
	n := NewTelegramNotifier("123456:fake-bot-token")
	client, rec := captureClient(http.StatusOK, `{"ok":true,"result":{"message_id":1}}`)
	n.HTTPClient = client
	notif := makeNotif()
	notif.Recipient = "-1001234567890"
	require.NoError(t, n.Send(context.Background(), notif))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "api.telegram.org", rec.url.Host)
	assert.Equal(t, "/bot123456:fake-bot-token/sendMessage", rec.url.Path)
	assert.Equal(t, "application/json", rec.header.Get("Content-Type"))
	body := decodeJSONBody(t, rec)
	assert.Equal(t, "-1001234567890", body["chat_id"])
	assert.Equal(t, "body", body["text"])
}

func TestTelegramNotifier_Send_Non2xx(t *testing.T) {
	n := NewTelegramNotifier("bot-token")
	n.HTTPClient = newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusBadRequest, Body: http.NoBody}
	})
	err := n.Send(context.Background(), makeNotif())
	assert.Error(t, err)
}

// --- TwitterNotifier Send ---

func TestTwitterNotifier_Send_Success(t *testing.T) {
	n := NewTwitterNotifierWithConfig("fake-oauth2-token", "fake-api-key", "fake-api-secret")
	client, rec := captureClient(http.StatusCreated, `{"data":{"dm_event_id":"123"}}`)
	n.HTTPClient = client
	notif := makeNotif()
	notif.Recipient = "2244994945"
	require.NoError(t, n.Send(context.Background(), notif))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "api.twitter.com", rec.url.Host)
	assert.Equal(t, "/2/dm_conversations/with/2244994945/messages", rec.url.Path, "X API v2 DM endpoint")
	assert.Equal(t, "Bearer fake-oauth2-token", rec.header.Get("Authorization"))
	body := decodeJSONBody(t, rec)
	assert.Equal(t, "body", body["text"])
	assert.NotContains(t, string(rec.body), "event", "v1.1 event envelope must be gone")
}

// --- ViberNotifier Send ---

func TestViberNotifier_Send_Success(t *testing.T) {
	n := NewViberNotifierWithConfig("fake-auth-token", "FakeSender")
	client, rec := captureClient(http.StatusOK, `{"status":0,"status_message":"ok","message_token":123}`)
	n.HTTPClient = client
	notif := makeNotif()
	notif.Recipient = "fake-receiver-id"
	require.NoError(t, n.Send(context.Background(), notif))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "chatapi.viber.com", rec.url.Host)
	assert.Equal(t, "/pa/send_message", rec.url.Path)
	assert.Equal(t, "fake-auth-token", rec.header.Get("X-Viber-Auth-Token"))
	body := decodeJSONBody(t, rec)
	assert.Equal(t, "fake-receiver-id", body["receiver"])
	msg, ok := body["message"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "text", msg["type"])
	assert.Equal(t, "body", msg["text"])
}

func TestViberNotifier_Send_ProgrammaticStatus(t *testing.T) {
	n := NewViberNotifierWithConfig("fake-auth-token", "")
	n.HTTPClient = newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"status":2,"status_message":"Invalid token"}`)),
		}
	})
	err := n.Send(context.Background(), makeNotif())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status=2")
}

// --- WeComNotifier Send ---

func TestWeComNotifier_Send_Success(t *testing.T) {
	n := NewWeComNotifier()
	client, rec := captureClient(http.StatusOK, `{"errcode":0,"errmsg":"ok"}`)
	n.HTTPClient = client
	notif := makeNotif()
	notif.Recipient = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=fake-key"
	require.NoError(t, n.Send(context.Background(), notif))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "qyapi.weixin.qq.com", rec.url.Host)
	assert.Equal(t, "/cgi-bin/webhook/send", rec.url.Path)
	assert.Equal(t, "fake-key", rec.url.Query().Get("key"))
	assert.Equal(t, "application/json", rec.header.Get("Content-Type"))
	body := decodeJSONBody(t, rec)
	assert.Equal(t, "text", body["msgtype"])
	text, ok := body["text"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "body", text["content"])
}

func TestWeComNotifier_Send_ProgrammaticErrcode(t *testing.T) {
	n := NewWeComNotifier()
	n.HTTPClient = newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"errcode":93000,"errmsg":"invalid msgtype"}`)),
		}
	})
	err := n.Send(context.Background(), makeNotif())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "93000")
}

// --- WebPushNotifier Send ---

func TestWebPushNotifier_Send_VAPID(t *testing.T) {
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	rawPriv := base64.RawURLEncoding.EncodeToString(ensureBigBytes(serverKey.D, 32))
	rawPub := base64.RawURLEncoding.EncodeToString(elliptic.Marshal(elliptic.P256(), serverKey.X, serverKey.Y))
	n := NewWebPushNotifierWithConfig(rawPub, rawPriv, "ops@example.com")
	client, rec := captureClient(http.StatusCreated, "")
	n.HTTPClient = client

	notif := makeNotif()
	notif.Recipient = "https://push.example.com/p/abc123"
	notif.Metadata = map[string]any{
		"p256dh": base64.RawURLEncoding.EncodeToString(elliptic.Marshal(elliptic.P256(), clientKey.X, clientKey.Y)),
		"auth":   base64.RawURLEncoding.EncodeToString(make([]byte, 16)),
	}
	require.NoError(t, n.Send(context.Background(), notif))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "push.example.com", rec.url.Host)
	assert.Equal(t, "aes128gcm", rec.header.Get("Content-Encoding"))
	assert.Equal(t, "86400", rec.header.Get("TTL"))
	assert.Equal(t, "application/octet-stream", rec.header.Get("Content-Type"))

	// RFC 8292: Authorization: vapid t=${JWT}, k=${base64url public key}
	auth := rec.header.Get("Authorization")
	require.True(t, strings.HasPrefix(auth, "vapid t="), "VAPID Authorization header: %q", auth)
	parts := strings.SplitN(strings.TrimPrefix(auth, "vapid t="), ", k=", 2)
	require.Len(t, parts, 2)
	jwt := parts[0]
	kKey, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	assert.Equal(t, rawPub, base64.RawURLEncoding.EncodeToString(kKey))

	jwtParts := strings.Split(jwt, ".")
	require.Len(t, jwtParts, 3)
	headerBytes, err := base64.RawURLEncoding.DecodeString(jwtParts[0])
	require.NoError(t, err)
	var header map[string]string
	require.NoError(t, json.Unmarshal(headerBytes, &header))
	assert.Equal(t, "JWT", header["typ"])
	assert.Equal(t, "ES256", header["alg"])

	claimsBytes, err := base64.RawURLEncoding.DecodeString(jwtParts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(claimsBytes, &claims))
	assert.Equal(t, "https://push.example.com", claims["aud"], "aud is the push endpoint origin")
	assert.Equal(t, "mailto:ops@example.com", claims["sub"])
	exp, ok := claims["exp"].(float64)
	require.True(t, ok)
	assert.Greater(t, exp, float64(0))

	sigBytes, err := base64.RawURLEncoding.DecodeString(jwtParts[2])
	require.NoError(t, err)
	require.Len(t, sigBytes, 64, "JOSE ES256 signature is fixed-width r||s")
	r := new(big.Int).SetBytes(sigBytes[:32])
	s := new(big.Int).SetBytes(sigBytes[32:])
	digest := sha256.Sum256([]byte(jwtParts[0] + "." + jwtParts[1]))
	assert.True(t, ecdsa.Verify(&serverKey.PublicKey, digest[:], r, s), "VAPID JWT must verify against the configured public key")

	// RFC 8291 aes128gcm body header: salt(16) | rs(4) | idlen(1)=0
	require.Greater(t, len(rec.body), 21)
	assert.Equal(t, byte(0), rec.body[20])
}

func TestWebPushNotifier_Send_MissingSubscriptionKeys(t *testing.T) {
	n := NewWebPushNotifierWithConfig("fake-pub", base64.RawURLEncoding.EncodeToString(make([]byte, 32)), "ops@example.com")
	n.HTTPClient = newTestClient(func(req *http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusCreated, Body: http.NoBody}
	})
	err := n.Send(context.Background(), makeNotif())
	assert.Error(t, err)
}

// --- WebPush pure crypto functions ---
func TestHkdfExpand(t *testing.T) {
	prk := make([]byte, 32)
	for i := range prk {
		prk[i] = byte(i)
	}
	info := []byte("test-info")
	result, err := platformcrypto.Expand(prk, info, 32)
	require.NoError(t, err)
	assert.Len(t, result, 32)

	result64, err := platformcrypto.Expand(prk, info, 64)
	require.NoError(t, err)
	assert.Len(t, result64, 64)
	assert.Equal(t, result, result64[:32])
}

func TestEnsureBigBytes(t *testing.T) {
	small := big.NewInt(255)
	result := ensureBigBytes(small, 32)
	assert.Len(t, result, 32)
	assert.Equal(t, byte(255), result[31])

	large := new(big.Int).SetBytes(make([]byte, 32))
	result = ensureBigBytes(large, 32)
	assert.Len(t, result, 32)

	tooLarge := new(big.Int).SetBytes(make([]byte, 40))
	result = ensureBigBytes(tooLarge, 32)
	assert.Len(t, result, 32)
}

func TestGetMetadataString(t *testing.T) {
	notif := &Notification{}
	assert.Equal(t, "", getMetadataString(notif, "key"))

	notif.Metadata = map[string]any{"other": "value"}
	assert.Equal(t, "", getMetadataString(notif, "key"))

	notif.Metadata = map[string]any{"key": "value"}
	assert.Equal(t, "value", getMetadataString(notif, "key"))

	notif.Metadata = map[string]any{"key": 42}
	assert.Equal(t, "42", getMetadataString(notif, "key"))
}

func TestEncryptWebPushPayload_InvalidClientKey(t *testing.T) {
	_, err := encryptWebPushPayload([]byte("test"), []byte("invalid"), []byte("auth"))
	assert.Error(t, err)
}

func TestDerivePRK(t *testing.T) {
	authKey := make([]byte, 16)
	sharedSecret := make([]byte, 32)
	clientPubKey := make([]byte, 65)
	serverPubKey := make([]byte, 65)

	result, err := derivePRK(authKey, sharedSecret, clientPubKey, serverPubKey)
	require.NoError(t, err)
	assert.Len(t, result, 32)
}
