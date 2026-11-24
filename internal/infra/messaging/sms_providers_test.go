package messaging

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockSmsSender_Send(t *testing.T) {
	sender := NewMockSmsSender()

	err := sender.Send(context.Background(), "+1234567890", "123456")
	require.NoError(t, err)
}

func TestMockSmsSender_Send_EmptyPhone(t *testing.T) {
	sender := NewMockSmsSender()

	err := sender.Send(context.Background(), "", "123456")
	require.NoError(t, err)
}

func TestMockSmsSender_ImplementsInterface(t *testing.T) {
	// Compile-time check
	var _ SmsSender = (*MockSmsSender)(nil)
}

func TestCustomHttpSmsSender_DefaultMethod(t *testing.T) {
	sender := NewCustomHttpSmsSender(
		"http://localhost:9999/sms",
		"",
		nil,
		"",
	)

	assert.Equal(t, "", sender.HTTPMethod, "HTTPMethod should default to empty; defaults to POST in Send")
}

func TestCustomHttpSmsSender_WithHeaders(t *testing.T) {
	headers := map[string]string{
		"X-Custom-Auth": "bearer-token",
	}

	sender := NewCustomHttpSmsSender(
		"http://localhost:9999/sms",
		"POST",
		headers,
		`{"phone":"{{phone}}","code":"{{code}}"}`,
	)

	assert.Equal(t, "http://localhost:9999/sms", sender.APIURL)
	assert.Equal(t, "POST", sender.HTTPMethod)
	assert.Equal(t, "bearer-token", sender.Headers["X-Custom-Auth"])
}

func TestCustomHttpSmsSender_ImplementsInterface(t *testing.T) {
	// Compile-time check
	var _ SmsSender = (*CustomHttpSmsSender)(nil)
}

func TestAmazonSNSSender_ImplementsInterface(t *testing.T) {
	var _ SmsSender = (*AmazonSNSSender)(nil)
}

func TestAzureACSsmsSender_ImplementsInterface(t *testing.T) {
	var _ SmsSender = (*AzureACSsmsSender)(nil)
}

func TestInfobipSmsSender_ImplementsInterface(t *testing.T) {
	var _ SmsSender = (*InfobipSmsSender)(nil)
}

func TestUCloudSmsSender_ImplementsInterface(t *testing.T) {
	var _ SmsSender = (*UCloudSmsSender)(nil)
}

func TestEmaySmsSender_ImplementsInterface(t *testing.T) {
	var _ SmsSender = (*EmaySmsSender)(nil)
}

func TestOsonSmsSender_ImplementsInterface(t *testing.T) {
	var _ SmsSender = (*OsonSmsSender)(nil)
}

func TestBaiduSmsSender_ImplementsInterface(t *testing.T) {
	var _ SmsSender = (*BaiduSmsSender)(nil)
}

func TestSubmailSmsSender_ImplementsInterface(t *testing.T) {
	var _ SmsSender = (*SubmailSmsSender)(nil)
}

func TestMailtrapSmsSender_ImplementsInterface(t *testing.T) {
	var _ SmsSender = (*MailtrapSmsSender)(nil)
}
