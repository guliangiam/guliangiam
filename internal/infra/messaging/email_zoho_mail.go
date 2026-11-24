package messaging

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// ZohoMailEmailSender sends mail through the Zoho Mail Send Message API
// (POST https://mail.zoho.com/api/accounts/{accountId}/messages, header
// Authorization: Zoho-oauthtoken <access_token>).
// https://www.zoho.com/mail/help/api/post-send-an-email.html
//
// Boundary: the token is supplied pre-minted (Password column); the OAuth
// app-consent exchange that produces/refreshes it is a Zoho-side flow and is
// not implemented here. accountId comes from the AppId column; the data-center
// domain (mail.zoho.eu, mail.zoho.in, …) can be overridden via the Host/
// endpoint column.
type ZohoMailEmailSender struct {
	AccessToken string
	AccountID   string
	From        string // fromAddress (must belong to the mailbox)
	BaseURL     string // default https://mail.zoho.com
	client      *http.Client
}

// NewZohoMailEmailSender creates a Zoho Mail sender.
func NewZohoMailEmailSender(accessToken, accountID, from, baseURL string) *ZohoMailEmailSender {
	if baseURL == "" {
		baseURL = "https://mail.zoho.com"
	}
	return &ZohoMailEmailSender{
		AccessToken: accessToken,
		AccountID:   accountID,
		From:        from,
		BaseURL:     strings.TrimRight(baseURL, "/"),
		client:      ssrf.DefaultHTTPClient,
	}
}

// Send delivers one message via POST /api/accounts/{accountId}/messages.
func (s *ZohoMailEmailSender) Send(ctx context.Context, to, subject, body string) error {
	if s.AccessToken == "" {
		return apperrors.NewInvalidArgument("zoho mail OAuth access token is required")
	}
	if s.AccountID == "" {
		return apperrors.NewInvalidArgument("zoho mail accountId is required")
	}
	if s.From == "" {
		return apperrors.NewInvalidArgument("zoho mail fromAddress is required")
	}
	form := url.Values{}
	form.Set("fromAddress", s.From)
	form.Set("toEmail", to) // multiple recipients are comma-separated per vendor spec
	form.Set("subject", subject)
	form.Set("content", body)

	endpoint := s.BaseURL + "/api/accounts/" + url.PathEscape(s.AccountID) + "/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Zoho-oauthtoken "+s.AccessToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapUnavailablef(err, "zoho mail send")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return apperrors.WrapUnavailablef(err, "zoho mail read response")
	}
	if resp.StatusCode >= 300 {
		return apperrors.NewInternalf("zoho mail send failed: status %d", resp.StatusCode)
	}
	// Zoho returns HTTP 200 with status:"error" on business failures.
	var parsed struct {
		Status string `json:"status"`
		Data   struct {
			ErrorCode string `json:"errorCode"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err == nil && parsed.Status != "" && parsed.Status != "success" {
		return apperrors.NewInternalf("zoho mail send failed: %s %s", parsed.Status, parsed.Data.ErrorCode)
	}
	return nil
}

var _ EmailSender = (*ZohoMailEmailSender)(nil)
