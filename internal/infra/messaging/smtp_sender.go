package messaging

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/smtpclient"
)

var dialSMTPContext = smtpclient.DialContext

// SMTPSender sends emails via SMTP.
type SMTPSender struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// NewSMTPSender creates a new SMTP email sender.
func NewSMTPSender(host string, port int, username, password, from string) *SMTPSender {
	return &SMTPSender{Host: host, Port: port, Username: username, Password: password, From: from}
}

func (s *SMTPSender) Send(ctx context.Context, to, subject, body string) error {
	client, err := dialSMTPContext(ctx, s.Host, s.Port, nil)
	if err != nil {
		return err
	}
	defer client.Close()

	if s.Username != "" || s.Password != "" {
		auth := smtp.PlainAuth("", s.Username, s.Password, s.Host)
		if err := client.Auth(auth); err != nil {
			return apperrors.WrapUnavailablef(err, "smtp: authenticate with %s", s.Host)
		}
	}
	if err := client.Mail(s.From); err != nil {
		return apperrors.WrapUnavailablef(err, "smtp: set sender")
	}
	if err := client.Rcpt(to); err != nil {
		return apperrors.WrapUnavailablef(err, "smtp: set recipient")
	}
	writer, err := client.Data()
	if err != nil {
		return apperrors.WrapUnavailablef(err, "smtp: start message data")
	}

	msg := strings.Join([]string{
		"From: " + s.From,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")
	if _, err := fmt.Fprint(writer, msg); err != nil {
		_ = writer.Close()
		return apperrors.WrapUnavailablef(err, "smtp: write message data")
	}
	if err := writer.Close(); err != nil {
		return apperrors.WrapUnavailablef(err, "smtp: finish message data")
	}
	if err := client.Quit(); err != nil {
		return apperrors.WrapUnavailablef(err, "smtp: close session")
	}
	return nil
}
