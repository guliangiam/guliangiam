package audit

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSyslogSink_Validates(t *testing.T) {
	_, err := NewSyslogSink(SyslogConfig{Port: 514})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host is required")

	_, err = NewSyslogSink(SyslogConfig{Host: "127.0.0.1", Port: 0})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "port 0")

	_, err = NewSyslogSink(SyslogConfig{Host: "127.0.0.1", Port: 514, Protocol: "SCTP"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "protocol")

	_, err = NewSyslogSink(SyslogConfig{Host: "127.0.0.1", Port: 514, Facility: "custom"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "facility")

	_, err = NewSyslogSink(SyslogConfig{Host: "127.0.0.1", Port: 514, Format: "JSON"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "format")
}

func TestSyslogSink_FormatRFC5424(t *testing.T) {
	sink, err := NewSyslogSink(SyslogConfig{Host: "127.0.0.1", Port: 514})
	require.NoError(t, err)

	line := sink.format(Event{Type: "user.login", ActorID: "actor-1", Resource: "users", ResourceID: "user-1"})
	assert.Contains(t, line, "<38>1 ")
	assert.Contains(t, line, " user.login ")
	assert.Contains(t, line, `"actor":"actor-1"`)
}

func TestSyslogSink_FormatRFC3164(t *testing.T) {
	sink, err := NewSyslogSink(SyslogConfig{Host: "127.0.0.1", Port: 514, Format: "RFC 3164", Tag: "iam"})
	require.NoError(t, err)

	line := sink.format(Event{Type: "user.login", ActorID: "actor-1"})
	assert.Contains(t, line, "<38>")
	assert.Contains(t, line, "iam[")
	assert.NotContains(t, line, "1 20", "RFC 3164 must not emit the RFC 5424 version field")
}

func TestSyslogSink_MessageIDIsBounded(t *testing.T) {
	assert.Equal(t, "-", syslogMsgID(""))
	assert.Len(t, syslogMsgID("a.very.long.event.name.that.exceeds.the.rfc5424.limit"), 32)
}

// TestFrameMessage pins the OPEN-API-75 P10 deviation fix: TLS must use
// RFC 5425 §3.1.1 octet counting ("<LEN> SP MSG"), plain TCP keeps the
// RFC 6587 §3.2 LF-terminated non-transparent framing, UDP carries the bare
// message. The previous implementation sent LF framing over TLS too, which
// desynchronizes a spec-conformant receiver.
func TestFrameMessage(t *testing.T) {
	assert.Equal(t, "6 <14>hi", frameMessage("TLS", "<14>hi"))
	assert.Equal(t, "<14>hi\n", frameMessage("TCP", "<14>hi"))
	assert.Equal(t, "<14>hi", frameMessage("UDP", "<14>hi"))

	// The octet count must be the byte length of the message itself.
	line := "<14>1 2026-11-14T00:00:00Z host app pid - 中文"
	framed := frameMessage("TLS", line)
	assert.Equal(t, strconv.Itoa(len(line))+" "+line, framed)
}

func TestSyslogSink_SendsTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	host, port := splitHostPort(t, ln.Addr().String())
	sink, err := NewSyslogSink(SyslogConfig{Host: host, Port: port, Protocol: "TCP"})
	require.NoError(t, err)
	defer sink.Close()

	require.NoError(t, sink.Send(context.Background(), Event{Type: "test.event", ActorID: "actor"}))

	conn, err := ln.Accept()
	require.NoError(t, err)
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	require.NoError(t, err)

	// Plain TCP: exactly one LF-terminated message, no length prefix.
	payload := string(buf[:n])
	require.True(t, strings.HasSuffix(payload, "\n"), "TCP payload must end with LF")
	assert.Equal(t, 1, strings.Count(payload, "\n"))
	assert.True(t, strings.HasPrefix(payload, "<38>"), "TCP payload must not carry an octet-count prefix")
	assert.Contains(t, payload, "test.event")
}

func TestSyslogSink_SendsUDP(t *testing.T) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	defer conn.Close()

	host, port := splitHostPort(t, conn.LocalAddr().String())
	sink, err := NewSyslogSink(SyslogConfig{Host: host, Port: port})
	require.NoError(t, err)
	defer sink.Close()

	require.NoError(t, sink.Send(context.Background(), Event{Type: "test.event", ActorID: "actor"}))

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 2048)
	n, _, err := conn.ReadFrom(buf)
	require.NoError(t, err)
	assert.Contains(t, string(buf[:n]), "test.event")
}

func splitHostPort(t *testing.T, address string) (string, int) {
	t.Helper()
	host, portText, err := net.SplitHostPort(address)
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)
	return host, port
}
