package providers

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/platform/smtpclient"
)

func TestEmailConnectionImplicitTLS(t *testing.T) {
	httpServer := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	certificate := httpServer.TLS.Certificates[0]
	httpServer.Close()
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS12,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	originalDial := dialSMTPContext
	dialSMTPContext = func(ctx context.Context, host string, port int, _ *tls.Config) (*smtp.Client, error) {
		return smtpclient.DialImplicitTLSContext(ctx, host, port, &tls.Config{InsecureSkipVerify: true}) // #nosec G402 -- local test certificate
	}
	t.Cleanup(func() { dialSMTPContext = originalDial })

	serverResult := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverResult <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		protocol := textproto.NewConn(conn)
		if err := protocol.PrintfLine("220 localhost ESMTP"); err != nil {
			serverResult <- err
			return
		}
		for {
			line, err := protocol.ReadLine()
			if err == io.EOF {
				serverResult <- nil
				return
			}
			if err != nil {
				serverResult <- err
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
				if err := protocol.PrintfLine("250-localhost"); err != nil {
					serverResult <- err
					return
				}
				if err := protocol.PrintfLine("250 AUTH PLAIN"); err != nil {
					serverResult <- err
					return
				}
			case strings.HasPrefix(line, "AUTH PLAIN "):
				decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(line, "AUTH PLAIN "))
				if err != nil || string(decoded) != "\x00user\x00pass" {
					serverResult <- fmt.Errorf("unexpected SMTP authentication payload")
					return
				}
				if err := protocol.PrintfLine("235 authenticated"); err != nil {
					serverResult <- err
					return
				}
			default:
				serverResult <- fmt.Errorf("unexpected SMTP command: %s", line)
				return
			}
		}
	}()

	host, rawPort, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		t.Fatal(err)
	}
	result, err := testEmailConnection(context.Background(), &domain.Provider{
		Host: host, Port: port, Admin: "user", Password: "pass",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Connected {
		t.Fatalf("implicit TLS probe failed: %s", result.Error)
	}
	if err := <-serverResult; err != nil {
		t.Fatal(err)
	}
}
