package messaging

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAmazonSNSSender_Send_NilClient(t *testing.T) {
	sender := &AmazonSNSSender{} // no client initialized
	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client not initialized")
}

func TestAzureACSsmsSender_Send_ShortConnString(t *testing.T) {
	sender := NewAzureACSsmsSender("short", "+1234567890")
	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.Error(t, err)
}

func TestAzureACSsmsSender_Send_LongConnString(t *testing.T) {
	sender := NewAzureACSsmsSender("this-is-a-very-long-connection-string-value", "+1234567890")
	err := sender.Send(context.Background(), "+1234567890", "123456")
	assert.Error(t, err)
}

func TestMockSmsSender_Send_EmptyCode(t *testing.T) {
	sender := NewMockSmsSender()
	err := sender.Send(context.Background(), "+1234567890", "")
	require.NoError(t, err)
}
