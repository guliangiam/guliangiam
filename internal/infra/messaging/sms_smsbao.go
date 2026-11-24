package messaging

import (
	"context"
	"crypto/md5" //nolint:gosec // G501: SmsBao API mandates MD5 for the p parameter
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// SmsBaoSender sends SMS through SmsBao's HTTP gateway.
type SmsBaoSender struct {
	Username string
	APIKey   string
	SignName string
	Template string
	GoodsID  string
	APIURL   string
	client   *http.Client
}

// NewSmsBaoSender creates an SmsBao sender. Template uses one %s placeholder
// for the verification code; SmsBao expects the content already signed.
func NewSmsBaoSender(username, apiKey, signName, template, goodsID string) *SmsBaoSender {
	return &SmsBaoSender{
		Username: username,
		APIKey:   apiKey,
		SignName: signName,
		Template: template,
		GoodsID:  goodsID,
		APIURL:   "https://api.smsbao.com/sms",
		client:   ssrf.SafeHTTPClient(10 * time.Second),
	}
}

func (s *SmsBaoSender) Send(ctx context.Context, phone, code string) error {
	if s.Username == "" || s.APIKey == "" {
		return apperrors.NewInvalidArgument("smsbao username and apiKey are required")
	}
	mobile, err := smsBaoMobile(phone)
	if err != nil {
		return err
	}

	template := s.Template
	if template == "" {
		template = "%s"
	}
	content := "【" + s.SignName + "】" + fmt.Sprintf(template, code)

	query := url.Values{}
	query.Set("u", s.Username)
	query.Set("p", smsBaoPassword(s.APIKey))
	if s.GoodsID != "" {
		query.Set("g", s.GoodsID)
	}
	query.Set("m", mobile)
	query.Set("c", content)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.APIURL+"?"+query.Encode(), http.NoBody)
	if err != nil {
		return apperrors.WrapInternalf(err, "smsbao: create request")
	}

	response, err := s.client.Do(request)
	if err != nil {
		return apperrors.WrapInternalf(err, "smsbao: request")
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<10))
	if err != nil {
		return apperrors.WrapInternalf(err, "smsbao: read response")
	}
	return smsBaoStatusError(strings.TrimSpace(string(body)))
}

// smsBaoPassword implements the SmsBao gateway credential rule: the p parameter
// is the MD5 hash (32 hex chars) of the platform login password. A value that is
// already a 32-char hex digest (pre-hashed password or platform ApiKey) is sent
// unchanged. https://www.smsbao.com/openapi/213.html
func smsBaoPassword(key string) string {
	if isHex32(key) {
		return key
	}
	sum := md5.Sum([]byte(key)) //nolint:gosec // G501: SmsBao API mandates MD5 for the p parameter
	return hex.EncodeToString(sum[:])
}

func isHex32(s string) bool {
	if len(s) != 32 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func smsBaoMobile(phone string) (string, error) {
	trimmed := strings.TrimSpace(phone)
	if strings.HasPrefix(trimmed, "+86") {
		return strings.TrimPrefix(trimmed, "+86"), nil
	}
	if strings.HasPrefix(trimmed, "+") {
		return "", apperrors.NewInvalidArgument("smsbao does not support non-mainland phone numbers")
	}
	return trimmed, nil
}

func smsBaoStatusError(body string) error {
	switch body {
	case "0", "":
		return nil
	case "30":
		return apperrors.NewUnauthorized("smsbao rejected the credentials")
	case "40":
		return apperrors.NewUnauthorized("smsbao account does not exist")
	case "41":
		return apperrors.NewUnavailable("smsbao account is overdue")
	case "43":
		return apperrors.NewForbidden("smsbao IP address limit reached")
	case "50":
		return apperrors.NewInvalidArgument("smsbao content contains forbidden words")
	case "51":
		return apperrors.NewInvalidArgument("smsbao rejected the phone number")
	default:
		return apperrors.NewInternalf("smsbao send failed: status %s", body)
	}
}

var _ SmsSender = (*SmsBaoSender)(nil)
