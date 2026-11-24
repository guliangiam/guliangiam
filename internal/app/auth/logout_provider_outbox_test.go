package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	providersmock "github.com/guliangiam/guliangiam/internal/domain/providers/mock"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	providerlogoutevent "github.com/guliangiam/guliangiam/internal/platform/providerlogout"
)

type captureProviderLogoutOutbox struct {
	events  []*domainoutbox.OutboxEvent
	enqueue error
}

func (o *captureProviderLogoutOutbox) Enqueue(_ context.Context, item *domainoutbox.OutboxEvent) error {
	o.events = append(o.events, item)
	return o.enqueue
}

func providerBoundSession() *sessiondomain.Session {
	return &sessiondomain.Session{
		ID:             "sess-001",
		UserID:         "user-001",
		OrganizationID: "org-001",
		DeviceInfo: map[string]any{
			sessiondomain.DeviceInfoProviderID:          "provider-001",
			sessiondomain.DeviceInfoProviderAccessToken: "sealed-upstream-token",
			"providerType": "OAuth",
		},
	}
}

// TestLogoutEnqueuesProviderLogoutInsteadOfCallingProvider asserts the new
// async boundary: the request path revokes locally and writes one Outbox event,
// while a slow provider endpoint is never contacted from the logout goroutine.
func TestLogoutEnqueuesProviderLogoutInsteadOfCallingProvider(t *testing.T) {
	hit := make(chan struct{}, 1)
	slowProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		select {
		case hit <- struct{}{}:
		default:
		}
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer slowProvider.Close()

	ctrl := gomock.NewController(t)
	providerRepo := providersmock.NewMockRepository(ctrl)
	// The request path must neither read nor call the provider.
	providerRepo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Times(0)

	repo := &mockSessionRepoForLogout{session: providerBoundSession()}
	outbox := &captureProviderLogoutOutbox{}
	svc := Service{
		Clock:  fakeClock{fixedTime: time.Now()},
		UOW:    logoutTestUOW{sessions: repo, providers: providerRepo},
		Outbox: outbox,
	}

	start := time.Now()
	err := svc.Logout(context.Background(), LogoutCommand{SessionID: "sess-001"})
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Less(t, elapsed, time.Second, "logout must not wait on the provider endpoint")
	select {
	case <-hit:
		t.Fatal("logout contacted the provider endpoint synchronously")
	default:
	}
	assert.True(t, repo.revokeCalled, "local session must be revoked")

	require.Len(t, outbox.events, 1)
	event := outbox.events[0]
	assert.Equal(t, providerlogoutevent.AggregateType, event.AggregateType)
	assert.Equal(t, providerlogoutevent.EventType, event.EventType)
	assert.Equal(t, providerlogoutevent.EventVersion, event.EventVersion)
	assert.Equal(t, "sess-001", event.AggregateID)
	require.NotNil(t, event.OrganizationID)
	assert.Equal(t, "org-001", *event.OrganizationID)

	var payload providerlogoutevent.Payload
	require.NoError(t, json.Unmarshal(event.Payload, &payload))
	assert.Equal(t, "provider-001", payload.ProviderID)
	assert.Equal(t, "OAuth", payload.ProviderType)
	assert.Equal(t, "sess-001", payload.SessionID)
	assert.Equal(t, "sealed-upstream-token", payload.SealedAccessToken)
	assert.Equal(t, "sess-001", payload.IdempotencyKey)
}

// TestLogoutSucceedsWhenProviderLogoutEnqueueFails keeps the existing contract:
// the revoke already happened, so a best-effort enqueue failure must not fail
// the logout response.
func TestLogoutSucceedsWhenProviderLogoutEnqueueFails(t *testing.T) {
	repo := &mockSessionRepoForLogout{session: providerBoundSession()}
	outbox := &captureProviderLogoutOutbox{enqueue: apperrors.NewInternal("outbox unavailable")}
	svc := Service{
		Clock:  fakeClock{fixedTime: time.Now()},
		UOW:    logoutTestUOW{sessions: repo},
		Outbox: outbox,
	}

	err := svc.Logout(context.Background(), LogoutCommand{SessionID: "sess-001"})
	require.NoError(t, err)
	assert.True(t, repo.revokeCalled)
}

// TestLogoutSkipsProviderLogoutWithoutSealedToken covers sessions created while
// no KEK was configured: the token is absent, so there is nothing to deliver.
func TestLogoutSkipsProviderLogoutWithoutSealedToken(t *testing.T) {
	session := &sessiondomain.Session{
		ID:             "sess-001",
		UserID:         "user-001",
		OrganizationID: "org-001",
		DeviceInfo: map[string]any{
			sessiondomain.DeviceInfoProviderID: "provider-001",
		},
	}
	repo := &mockSessionRepoForLogout{session: session}
	outbox := &captureProviderLogoutOutbox{}
	svc := Service{
		Clock:  fakeClock{fixedTime: time.Now()},
		UOW:    logoutTestUOW{sessions: repo},
		Outbox: outbox,
	}

	require.NoError(t, svc.Logout(context.Background(), LogoutCommand{SessionID: "sess-001"}))
	assert.True(t, repo.revokeCalled)
	assert.Empty(t, outbox.events)
}
