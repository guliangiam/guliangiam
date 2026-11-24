package bootstrap

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/domain/providers"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
)

type stubSyslogLookup struct {
	provider *providers.Provider
	err      error
}

func (s stubSyslogLookup) FindAuditSyslogProvider(_ context.Context, _ string) (*providers.Provider, error) {
	return s.provider, s.err
}

func TestAuditSyslogObserver_NoProviderIsNoop(t *testing.T) {
	observer := newAuditSyslogObserver(stubSyslogLookup{})
	orgID := "org-1"
	item := &platformoutbox.OutboxItem{OrganizationID: &orgID, AggregateType: "users", EventType: "user.login"}
	require.NoError(t, observer.Observe(context.Background(), item))
}

func TestAuditSyslogObserver_WritesToProvider(t *testing.T) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	defer conn.Close()

	host, portText, err := net.SplitHostPort(conn.LocalAddr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)

	observer := newAuditSyslogObserver(stubSyslogLookup{provider: &providers.Provider{
		ID: "provider-1", Revision: 1, Enabled: true,
		ProviderType: providers.ProviderSyslog, Name: "audit-syslog",
		Host: host, Port: port, SubType: "UDP", Template: "auth", ContentType: "RFC 5424",
	}})

	orgID := "org-1"
	item := &platformoutbox.OutboxItem{
		OrganizationID: &orgID, AggregateType: "users", AggregateID: "user-1",
		EventType: "user.login", Payload: []byte(`{"username":"alice"}`),
	}
	require.NoError(t, observer.Observe(context.Background(), item))

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 2048)
	n, _, err := conn.ReadFrom(buf)
	require.NoError(t, err)
	assert.Contains(t, string(buf[:n]), "user.login")
	assert.Contains(t, string(buf[:n]), `"username":"alice"`)
}

func TestAuditSyslogObserver_InvalidConfigIsReported(t *testing.T) {
	observer := newAuditSyslogObserver(stubSyslogLookup{provider: &providers.Provider{
		ID: "provider-1", Revision: 1, Enabled: true,
		ProviderType: providers.ProviderSyslog, Host: "", Port: 0,
	}})
	orgID := "org-1"
	item := &platformoutbox.OutboxItem{OrganizationID: &orgID, AggregateType: "users", EventType: "user.login"}
	require.Error(t, observer.Observe(context.Background(), item))
}
