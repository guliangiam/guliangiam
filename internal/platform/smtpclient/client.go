package smtpclient

import (
	"context"
	"crypto/tls"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// DialContext opens an SMTP client using implicit TLS on port 465 and STARTTLS
// on port 587. A nil TLS config uses the secure platform default.
func DialContext(ctx context.Context, host string, port int, config *tls.Config) (*smtp.Client, error) {
	return dialContext(ctx, host, port, config, port == 465, port == 587)
}

// DialImplicitTLSContext supports implicit TLS on a non-standard SMTP port.
func DialImplicitTLSContext(ctx context.Context, host string, port int, config *tls.Config) (*smtp.Client, error) {
	return dialContext(ctx, host, port, config, true, false)
}

func dialContext(ctx context.Context, host string, port int, config *tls.Config, implicitTLS, startTLS bool) (*smtp.Client, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, apperrors.NewInvalidArgument("smtp host is required")
	}
	if port == 0 {
		port = 25
	}
	if port < 1 || port > 65535 {
		return nil, apperrors.NewInvalidArgument("smtp port is invalid")
	}

	if config == nil {
		config = &tls.Config{MinVersion: tls.VersionTLS12}
	} else {
		config = config.Clone()
		if config.MinVersion == 0 {
			config.MinVersion = tls.VersionTLS12
		}
	}
	if config.ServerName == "" {
		config.ServerName = host
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, apperrors.WrapUnavailablef(err, "smtp: connect to %s", addr)
	}
	if implicitTLS {
		tlsConn := tls.Client(conn, config)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return nil, apperrors.WrapUnavailablef(err, "smtp: implicit TLS handshake with %s", addr)
		}
		conn = tlsConn
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return nil, apperrors.WrapUnavailablef(err, "smtp: initialize client for %s", addr)
	}
	if startTLS {
		if err := client.StartTLS(config); err != nil {
			_ = client.Close()
			return nil, apperrors.WrapUnavailablef(err, "smtp: STARTTLS with %s", addr)
		}
	}
	return client, nil
}
