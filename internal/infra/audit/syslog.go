package audit

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// SyslogConfig describes one RFC 5424 / RFC 3164 sink.
type SyslogConfig struct {
	Host     string
	Port     int
	Protocol string // UDP (default), TCP, TLS
	Facility string // auth (default) or any syslog facility name
	Format   string // RFC 5424 (default) or RFC 3164
	Tag      string // application tag, default "guliangiam"
}

var syslogNetworks = map[string]string{"UDP": "udp", "TCP": "tcp", "TLS": "tcp"}

var syslogFacilities = map[string]int{
	"kern": 0, "user": 1, "mail": 2, "daemon": 3, "auth": 4, "syslog": 5, "lpr": 6, "news": 7,
	"uucp": 8, "cron": 9, "authpriv": 10, "ftp": 11, "ntp": 12, "security": 13, "console": 14,
	"solaris-cron": 15, "local0": 16, "local1": 17, "local2": 18, "local3": 19, "local4": 20,
	"local5": 21, "local6": 22, "local7": 23,
}

const (
	syslogSeverityInformational = 6
	syslogWriteTimeout          = 5 * time.Second
	// syslogUDPQueryMax keeps a datagram below the 64 KiB UDP payload limit.
	syslogUDPQueryMax = 60000
)

// SyslogSink writes audit events to a syslog endpoint. Writes are synchronous
// and serialized: the outbox worker is already asynchronous, and a persistent
// connection makes an unreachable endpoint observable as a delivery error
// instead of a silently growing in-memory queue.
type SyslogSink struct {
	cfg      SyslogConfig
	network  string
	facility int
	address  string
	hostname string
	procID   string

	mu   sync.Mutex
	conn net.Conn
}

// NewSyslogSink validates the configuration and returns a sink. The connection
// is opened lazily on the first write so a record can be configured before the
// collector is reachable.
func NewSyslogSink(cfg SyslogConfig) (*SyslogSink, error) {
	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		return nil, apperrors.NewInvalidArgument("syslog host is required")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return nil, apperrors.NewInvalidArgumentf("syslog port %d is out of range", cfg.Port)
	}

	protocol := strings.ToUpper(strings.TrimSpace(cfg.Protocol))
	if protocol == "" {
		protocol = "UDP"
	}
	network, ok := syslogNetworks[protocol]
	if !ok {
		return nil, apperrors.NewInvalidArgumentf("unsupported syslog protocol %q (allowed: UDP, TCP, TLS)", cfg.Protocol)
	}

	facilityName := strings.ToLower(strings.TrimSpace(cfg.Facility))
	if facilityName == "" {
		facilityName = "auth"
	}
	facility, ok := syslogFacilities[facilityName]
	if !ok {
		return nil, apperrors.NewInvalidArgumentf("unsupported syslog facility %q", cfg.Facility)
	}

	format := strings.TrimSpace(cfg.Format)
	if format == "" {
		format = "RFC 5424"
	}
	if format != "RFC 5424" && format != "RFC 3164" {
		return nil, apperrors.NewInvalidArgumentf("unsupported syslog format %q (allowed: RFC 5424, RFC 3164)", cfg.Format)
	}

	tag := strings.TrimSpace(cfg.Tag)
	if tag == "" {
		tag = "guliangiam"
	}
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "-"
	}

	return &SyslogSink{
		cfg:      SyslogConfig{Host: host, Port: cfg.Port, Protocol: protocol, Facility: facilityName, Format: format, Tag: tag},
		network:  network,
		facility: facility,
		address:  net.JoinHostPort(host, strconv.Itoa(cfg.Port)),
		hostname: hostname,
		procID:   strconv.Itoa(os.Getpid()),
	}, nil
}

// Send formats and writes one event. The connection is reopened on failure so a
// restarted collector or a dropped UDP/TCP socket recovers on the next write.
func (s *SyslogSink) Send(ctx context.Context, event Event) error {
	payload := frameMessage(s.cfg.Protocol, s.format(event))

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.write(ctx, payload); err != nil {
		s.closeLocked()
		return err
	}
	return nil
}

// frameMessage applies transport framing to a syslog line.
// TLS MUST use octet counting ("<MSGLEN> SP MSG", RFC 5425 §3.1.1): the
// stream has no message delimiter, so an LF-terminated line would
// desynchronize the receiver's parser. Plain TCP keeps the legacy
// non-transparent framing (LF-terminated lines), which RFC 6587 §3.2 still
// allows alongside octet counting (§3.1). UDP (RFC 5426) carries exactly one
// message per datagram with no framing.
func frameMessage(protocol, line string) string {
	switch protocol {
	case "TLS":
		return fmt.Sprintf("%d %s", len(line), line)
	case "TCP":
		return line + "\n"
	default: // UDP
		return line
	}
}

// Close closes the persistent connection.
func (s *SyslogSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeLocked()
	return nil
}

func (s *SyslogSink) write(ctx context.Context, line string) error {
	if s.conn == nil {
		conn, err := s.dial(ctx)
		if err != nil {
			return apperrors.WrapInternalf(err, "syslog dial %s", s.address)
		}
		s.conn = conn
	}

	deadline := time.Now().Add(syslogWriteTimeout)
	_ = s.conn.SetWriteDeadline(deadline)
	if _, err := s.conn.Write([]byte(line)); err != nil {
		return apperrors.WrapInternalf(err, "syslog write %s", s.address)
	}
	return nil
}

func (s *SyslogSink) dial(ctx context.Context) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: syslogWriteTimeout}
	if s.cfg.Protocol == "TLS" {
		return tls.DialWithDialer(dialer, "tcp", s.address, &tls.Config{
			ServerName: s.cfg.Host,
			MinVersion: tls.VersionTLS12,
		})
	}
	return dialer.DialContext(ctx, s.network, s.address)
}

func (s *SyslogSink) closeLocked() {
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
}

// format renders the syslog line without transport framing. RFC 5424 is the
// default; RFC 3164 is kept for collectors that still expect the BSD header.
// Framing is added by frameMessage at send time.
func (s *SyslogSink) format(event Event) string {
	priority := s.facility*8 + syslogSeverityInformational
	message := strings.NewReplacer("\r", " ", "\n", " ").Replace(s.message(event))
	now := time.Now()

	var line string
	if s.cfg.Format == "RFC 3164" {
		line = fmt.Sprintf("<%d>%s %s %s[%s]: %s", priority, now.Format(time.Stamp), s.hostname, s.cfg.Tag, s.procID, message)
	} else {
		line = fmt.Sprintf("<%d>1 %s %s %s %s %s - %s", priority, now.Format(time.RFC3339),
			s.hostname, s.cfg.Tag, s.procID, syslogMsgID(event.Type), message)
	}

	if s.network == "udp" && len(line) > syslogUDPQueryMax {
		line = strings.ToValidUTF8(line[:syslogUDPQueryMax], "")
	}
	return line
}

// message serializes the event into a single structured line. The audit fields
// are stable so a collector can index them without parsing free text.
func (s *SyslogSink) message(event Event) string {
	fields := map[string]any{
		"type":       event.Type,
		"actor":      event.ActorID,
		"resource":   event.Resource,
		"resourceId": event.ResourceID,
	}
	if len(event.Payload) > 0 {
		fields["payload"] = event.Payload
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return fmt.Sprintf("type=%s actor=%s resource=%s resourceId=%s", event.Type, event.ActorID, event.Resource, event.ResourceID)
	}
	return string(encoded)
}

// syslogMsgID limits the RFC 5424 MSGID to 32 printable ASCII characters.
func syslogMsgID(action string) string {
	msgID := strings.Map(func(r rune) rune {
		if r < 33 || r > 126 {
			return '_'
		}
		return r
	}, action)
	if len(msgID) > 32 {
		msgID = msgID[:32]
	}
	if msgID == "" {
		return "-"
	}
	return msgID
}
