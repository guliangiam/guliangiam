package ssf

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	ssfdomain "github.com/guliangiam/guliangiam/internal/domain/ssf"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeReceiverRepo struct {
	receiver *ssfdomain.Receiver
}

func (f *fakeReceiverRepo) Create(context.Context, *ssfdomain.Receiver) error { return nil }
func (f *fakeReceiverRepo) GetByID(context.Context, string, string) (*ssfdomain.Receiver, error) {
	return f.receiver, nil
}
func (f *fakeReceiverRepo) ListByOrganization(context.Context, string) ([]*ssfdomain.Receiver, error) {
	return nil, nil
}
func (f *fakeReceiverRepo) Delete(context.Context, string, string) error { return nil }

type fakeSigner struct{ token string }

func (f *fakeSigner) SignToken(map[string]any, string) (string, error) { return f.token, nil }

func item(t *testing.T, receiverID, eventType string) *platformoutbox.OutboxItem {
	t.Helper()
	headers, err := json.Marshal(map[string]string{"receiver_id": receiverID, "organization_id": "org-1"})
	require.NoError(t, err)
	payload, err := json.Marshal(map[string]any{
		"type":    eventType,
		"issuer":  "https://op.example.com",
		"eventId": "evt-1",
	})
	require.NoError(t, err)
	return &platformoutbox.OutboxItem{Headers: headers, Payload: payload}
}

func TestHandler_DeliversSignedToken(t *testing.T) {
	var gotContentType, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		body := make([]byte, 64)
		n, _ := r.Body.Read(body)
		gotBody = string(body[:n])
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	repo := &fakeReceiverRepo{receiver: &ssfdomain.Receiver{
		ID: "r-1", OrganizationID: "org-1", ReceiverURL: server.URL, Status: "active",
		EventTypes: []byte(`[]`),
	}}
	handler := NewHandler(repo, &fakeSigner{token: "signed-token"}, server.Client())
	require.NoError(t, handler.Handle(context.Background(), item(t, "r-1", "https://schemas.openid.net/secevent/caep/event-type/session-revoked")))
	assert.Equal(t, "application/secevent+jwt", gotContentType)
	assert.Equal(t, "signed-token", gotBody)
}

func TestHandler_SkipsDisabledReceiver(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer server.Close()

	repo := &fakeReceiverRepo{receiver: &ssfdomain.Receiver{
		ID: "r-1", OrganizationID: "org-1", ReceiverURL: server.URL, Status: "disabled",
	}}
	handler := NewHandler(repo, &fakeSigner{token: "signed-token"}, server.Client())
	require.NoError(t, handler.Handle(context.Background(), item(t, "r-1", "event")))
	assert.False(t, called)
}

func TestHandler_RejectsUnsubscribedEvent(t *testing.T) {
	repo := &fakeReceiverRepo{receiver: &ssfdomain.Receiver{
		ID: "r-1", OrganizationID: "org-1", ReceiverURL: "https://receiver.example.com", Status: "active",
		EventTypes: []byte(`["https://schemas.openid.net/secevent/caep/event-type/session-revoked"]`),
	}}
	handler := NewHandler(repo, &fakeSigner{token: "signed-token"}, nil)
	require.Error(t, handler.Handle(context.Background(), item(t, "r-1", "https://schemas.openid.net/secevent/caep/event-type/credential-change")))
}
