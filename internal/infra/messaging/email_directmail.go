package messaging

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// DirectMailEmailSender sends transactional email through Aliyun Direct Mail's
// SingleSendMail RPC API (API version 2015-11-23, HMAC-SHA1 RPC signature —
// the same signing scheme as AlibabaSmsSender).
// https://help.aliyun.com/zh/direct-mail/api-dm-2015-11-23-singlesendmail
type DirectMailEmailSender struct {
	AccessKeyID     string
	AccessKeySecret string
	AccountName     string // verified sending address (AccountName)
	RegionID        string // e.g. cn-hangzhou; empty falls back to cn-hangzhou
	Endpoint        string // default https://dm.aliyuncs.com/
	client          *http.Client
}

// NewDirectMailEmailSender creates an Aliyun Direct Mail sender.
func NewDirectMailEmailSender(accessKeyID, accessKeySecret, accountName, regionID, endpoint string) *DirectMailEmailSender {
	if endpoint == "" {
		endpoint = "https://dm.aliyuncs.com/"
	}
	if regionID == "" {
		regionID = "cn-hangzhou"
	}
	return &DirectMailEmailSender{
		AccessKeyID:     accessKeyID,
		AccessKeySecret: accessKeySecret,
		AccountName:     accountName,
		RegionID:        regionID,
		Endpoint:        endpoint,
		client:          ssrf.DefaultHTTPClient,
	}
}

// Send delivers one email via SingleSendMail (plain-text body).
func (s *DirectMailEmailSender) Send(ctx context.Context, to, subject, body string) error {
	if s.AccessKeyID == "" || s.AccessKeySecret == "" {
		return apperrors.NewInvalidArgument("directmail accessKeyId/accessKeySecret are required")
	}
	if s.AccountName == "" {
		return apperrors.NewInvalidArgument("directmail accountName (sender) is required")
	}
	params := map[string]string{
		"AccessKeyId":      s.AccessKeyID,
		"Action":           "SingleSendMail",
		"AddressType":      "1",
		"AccountName":      s.AccountName,
		"Format":           "JSON",
		"RegionId":         s.RegionID,
		"ReplyToAddress":   "false",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   generateNonce(),
		"SignatureVersion": "1.0",
		"Subject":          subject,
		"TextBody":         body,
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"ToAddress":        to,
		"Version":          "2015-11-23",
	}
	params["Signature"] = signRequest(params, s.AccessKeySecret)

	values := make(url.Values)
	for k, v := range params {
		values.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.Endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return apperrors.WrapUnavailablef(err, "directmail send")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return apperrors.WrapUnavailablef(err, "directmail read response")
	}
	if resp.StatusCode >= 300 {
		var apiErr struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		}
		_ = json.Unmarshal(raw, &apiErr)
		return apperrors.NewInternalf("directmail send failed: %s %s", apiErr.Code, apiErr.Message)
	}
	return nil
}

var _ EmailSender = (*DirectMailEmailSender)(nil)
