package notification

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Contract pins for the five notification kinds added in OPEN-API-75 P6
// (2026-11-14). Each success case pins method/host/path/headers/payload
// against the provider's official API shape.

// --- NtfyNotifier Send ---

func TestNtfyNotifier_Send_Success(t *testing.T) {
	n := NewNtfyNotifier("https://ntfy.sh/", "my-topic", "fake-token")
	client, rec := captureClient(http.StatusOK, "")
	n.HTTPClient = client
	require.NoError(t, n.Send(context.Background(), makeNotif()))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "ntfy.sh", rec.url.Host)
	assert.Equal(t, "/my-topic", rec.url.Path, "trailing slash on server must not double the separator")
	assert.Equal(t, "body", string(rec.body), "ntfy publishes the raw body as the message")
	assert.Equal(t, "text/plain; charset=utf-8", rec.header.Get("Content-Type"))
	assert.Equal(t, "title", rec.header.Get("Title"))
	assert.Equal(t, "Bearer fake-token", rec.header.Get("Authorization"))
}

func TestNtfyNotifier_Send_TopicFallsBackToRecipient(t *testing.T) {
	n := NewNtfyNotifier("https://ntfy.example.com", "", "")
	client, rec := captureClient(http.StatusOK, "")
	n.HTTPClient = client
	notif := makeNotif()
	notif.Recipient = "fallback-topic"
	notif.Subject = ""
	require.NoError(t, n.Send(context.Background(), notif))

	assert.Equal(t, "/fallback-topic", rec.url.Path)
	assert.Empty(t, rec.header.Get("Title"), "empty subject means no Title header")
	assert.Empty(t, rec.header.Get("Authorization"), "public topic means no bearer token")
}

func TestNtfyNotifier_Send_MissingServer(t *testing.T) {
	n := NewNtfyNotifier("", "topic", "")
	assert.Error(t, n.Send(context.Background(), makeNotif()))
}

func TestNtfyNotifier_Send_Non2xx(t *testing.T) {
	n := NewNtfyNotifier("https://ntfy.sh", "topic", "")
	client, _ := captureClient(http.StatusForbidden, `{"error":"unauthorized"}`)
	n.HTTPClient = client
	err := n.Send(context.Background(), makeNotif())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

// --- ServerChanNotifier Send ---

func TestServerChanNotifier_Send_TurboSuccess(t *testing.T) {
	n := NewServerChanNotifier("SCT123456abcdefghijklmnopqrst")
	client, rec := captureClient(http.StatusOK, `{"code":0,"message":"","data":{"pushid":"1"}}`)
	n.HTTPClient = client
	require.NoError(t, n.Send(context.Background(), makeNotif()))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "sctapi.ftqq.com", rec.url.Host)
	assert.Equal(t, "/SCT123456abcdefghijklmnopqrst.send", rec.url.Path)
	assert.Equal(t, "application/x-www-form-urlencoded", rec.header.Get("Content-Type"))
	form, err := url.ParseQuery(string(rec.body))
	require.NoError(t, err)
	assert.Equal(t, "title", form.Get("title"))
	assert.Equal(t, "body", form.Get("desp"))
}

func TestServerChanNotifier_Send_SCTPShortKey(t *testing.T) {
	n := NewServerChanNotifier("sctp1234abcd")
	client, rec := captureClient(http.StatusOK, `{"code":0}`)
	n.HTTPClient = client
	require.NoError(t, n.Send(context.Background(), makeNotif()))

	assert.Equal(t, "1234.push.ft07.com", rec.url.Host, "sctp keys target the uid host")
	assert.Equal(t, "/send/sctp1234abcd.send", rec.url.Path)
}

func TestServerChanNotifier_Send_TitleCappedAt32Runes(t *testing.T) {
	n := NewServerChanNotifier("SCTkey")
	client, rec := captureClient(http.StatusOK, `{"code":0}`)
	n.HTTPClient = client
	notif := makeNotif()
	notif.Subject = ""
	notif.Content = strings.Repeat("字", 40)
	require.NoError(t, n.Send(context.Background(), notif))

	form, err := url.ParseQuery(string(rec.body))
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat("字", 32), form.Get("title"), "Turbo API limits title to 32 characters")
}

func TestServerChanNotifier_Send_ProgrammaticCode(t *testing.T) {
	n := NewServerChanNotifier("SCTkey")
	client, _ := captureClient(http.StatusOK, `{"code":1,"message":"invalid sendkey"}`)
	n.HTTPClient = client
	err := n.Send(context.Background(), makeNotif())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "code=1")
}

func TestServerChanNotifier_Send_MissingKey(t *testing.T) {
	n := NewServerChanNotifier("")
	assert.Error(t, n.Send(context.Background(), makeNotif()))
}

// --- PagerDutyNotifier Send ---

func TestPagerDutyNotifier_Send_Success(t *testing.T) {
	n := NewPagerDutyNotifier("fake-routing-key")
	client, rec := captureClient(http.StatusAccepted, `{"status":"success","message":"Event processed","dedup_key":"fake"}`)
	n.HTTPClient = client
	require.NoError(t, n.Send(context.Background(), makeNotif()))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "events.pagerduty.com", rec.url.Host)
	assert.Equal(t, "/v2/enqueue", rec.url.Path)
	assert.Equal(t, "application/json", rec.header.Get("Content-Type"))
	body := decodeJSONBody(t, rec)
	assert.Equal(t, "fake-routing-key", body["routing_key"])
	assert.Equal(t, "trigger", body["event_action"])
	payload, ok := body["payload"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "title", payload["summary"], "subject becomes the trigger summary")
	assert.Equal(t, "GuliangIAM", payload["source"])
	assert.Equal(t, "error", payload["severity"], "unset severity falls back to error")
}

func TestPagerDutyNotifier_Send_InvalidSeverityFallsBack(t *testing.T) {
	n := NewPagerDutyNotifierWithConfig("rk", "fatal", "iam.example.com")
	client, rec := captureClient(http.StatusAccepted, "")
	n.HTTPClient = client
	require.NoError(t, n.Send(context.Background(), makeNotif()))

	payload, ok := decodeJSONBody(t, rec)["payload"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "error", payload["severity"], "severity outside the closed set falls back to error")
	assert.Equal(t, "iam.example.com", payload["source"])
}

func TestPagerDutyNotifier_Send_Non2xx(t *testing.T) {
	n := NewPagerDutyNotifier("rk")
	client, _ := captureClient(http.StatusBadRequest, `{"error":{"message":"Event action was wrong"}}`)
	n.HTTPClient = client
	err := n.Send(context.Background(), makeNotif())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestPagerDutyNotifier_Send_MissingRoutingKey(t *testing.T) {
	n := NewPagerDutyNotifier("")
	assert.Error(t, n.Send(context.Background(), makeNotif()))
}

// --- OpsGenieNotifier Send ---

func TestOpsGenieNotifier_Send_Success(t *testing.T) {
	n := NewOpsGenieNotifier("fake-api-key")
	client, rec := captureClient(http.StatusAccepted, `{"result":"Request will be processed","messageId":"fake"}`)
	n.HTTPClient = client
	require.NoError(t, n.Send(context.Background(), makeNotif()))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "api.opsgenie.com", rec.url.Host)
	assert.Equal(t, "/v2/alerts", rec.url.Path)
	assert.Equal(t, "GenieKey fake-api-key", rec.header.Get("Authorization"))
	body := decodeJSONBody(t, rec)
	assert.Equal(t, "title", body["message"])
	assert.Equal(t, "body", body["description"])
}

func TestOpsGenieNotifier_Send_EUCluster(t *testing.T) {
	n := NewOpsGenieNotifierWithConfig("fake-api-key", OpsGenieEUAlertsURL)
	client, rec := captureClient(http.StatusAccepted, "")
	n.HTTPClient = client
	require.NoError(t, n.Send(context.Background(), makeNotif()))
	assert.Equal(t, "api.eu.opsgenie.com", rec.url.Host)
}

func TestOpsGenieNotifier_Send_MessageTruncatedTo130Runes(t *testing.T) {
	n := NewOpsGenieNotifier("fake-api-key")
	client, rec := captureClient(http.StatusAccepted, "")
	n.HTTPClient = client
	notif := makeNotif()
	notif.Subject = strings.Repeat("标", 150)
	require.NoError(t, n.Send(context.Background(), notif))

	body := decodeJSONBody(t, rec)
	assert.Equal(t, strings.Repeat("标", 130), body["message"], "Alert API rejects messages over 130 characters")
}

func TestOpsGenieNotifier_Send_MissingKey(t *testing.T) {
	n := NewOpsGenieNotifier("")
	assert.Error(t, n.Send(context.Background(), makeNotif()))
}

// --- GotifyNotifier Send ---

func TestGotifyNotifier_Send_Success(t *testing.T) {
	n := NewGotifyNotifier("https://gotify.example.com/", "fake-app-token")
	client, rec := captureClient(http.StatusOK, `{"id":1,"message":"body"}`)
	n.HTTPClient = client
	require.NoError(t, n.Send(context.Background(), makeNotif()))

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "gotify.example.com", rec.url.Host)
	assert.Equal(t, "/message", rec.url.Path, "trailing slash on base must not double the separator")
	assert.Equal(t, "fake-app-token", rec.header.Get("X-Gotify-Key"))
	body := decodeJSONBody(t, rec)
	assert.Equal(t, "body", body["message"])
	assert.Equal(t, "title", body["title"])
	assert.Equal(t, float64(5), body["priority"], "default priority is 5")
}

func TestGotifyNotifier_Send_CustomPriorityOmitsEmptyTitle(t *testing.T) {
	n := NewGotifyNotifierWithConfig("https://gotify.example.com", "", 8)
	client, rec := captureClient(http.StatusOK, "")
	n.HTTPClient = client
	notif := makeNotif()
	notif.Subject = ""
	require.NoError(t, n.Send(context.Background(), notif))

	assert.Empty(t, rec.header.Get("X-Gotify-Key"), "no token means no key header")
	body := decodeJSONBody(t, rec)
	assert.Equal(t, float64(8), body["priority"])
	_, hasTitle := body["title"]
	assert.False(t, hasTitle, "empty title is omitted from the payload")
}

func TestGotifyNotifier_Send_MissingServer(t *testing.T) {
	n := NewGotifyNotifier("", "token")
	assert.Error(t, n.Send(context.Background(), makeNotif()))
}

func TestGotifyNotifier_Send_Non2xx(t *testing.T) {
	n := NewGotifyNotifier("https://gotify.example.com", "token")
	client, _ := captureClient(http.StatusUnauthorized, `{"error":"Unauthorized"}`)
	n.HTTPClient = client
	err := n.Send(context.Background(), makeNotif())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}
