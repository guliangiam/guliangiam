package messaging

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/smtpclient"
)

func TestSMTPSenderImplicitTLS(t *testing.T) {
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
		authenticated := false
		mailFrom := false
		recipient := false
		for {
			line, err := protocol.ReadLine()
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
				authenticated = true
				if err := protocol.PrintfLine("235 authenticated"); err != nil {
					serverResult <- err
					return
				}
			case line == "MAIL FROM:<sender@example.com>":
				mailFrom = true
				if err := protocol.PrintfLine("250 sender accepted"); err != nil {
					serverResult <- err
					return
				}
			case line == "RCPT TO:<recipient@example.com>":
				recipient = true
				if err := protocol.PrintfLine("250 recipient accepted"); err != nil {
					serverResult <- err
					return
				}
			case line == "DATA":
				if !authenticated || !mailFrom || !recipient {
					serverResult <- fmt.Errorf("DATA arrived before AUTH/MAIL/RCPT")
					return
				}
				if err := protocol.PrintfLine("354 send message"); err != nil {
					serverResult <- err
					return
				}
				lines, err := protocol.ReadDotLines()
				if err != nil {
					serverResult <- err
					return
				}
				message := strings.Join(lines, "\n")
				if !strings.Contains(message, "Subject: verification") || !strings.Contains(message, "code-123") {
					serverResult <- fmt.Errorf("message content was not preserved")
					return
				}
				if err := protocol.PrintfLine("250 queued"); err != nil {
					serverResult <- err
					return
				}
			case line == "QUIT":
				if err := protocol.PrintfLine("221 bye"); err != nil {
					serverResult <- err
					return
				}
				serverResult <- nil
				return
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
	sender := NewSMTPSender(host, port, "user", "pass", "sender@example.com")
	if err := sender.Send(context.Background(), "recipient@example.com", "verification", "code-123"); err != nil {
		t.Fatal(err)
	}
	if err := <-serverResult; err != nil {
		t.Fatal(err)
	}
}
