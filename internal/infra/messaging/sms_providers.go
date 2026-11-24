package messaging

import (
	"bytes"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/service/sns"
)

// jsonMarshal marshals v to JSON bytes.
func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// jsonReader creates a reader from JSON bytes.
func jsonReader(data []byte) *bytes.Reader {
	return bytes.NewReader(data)
}

// AmazonSNSSender sends SMS via Amazon SNS.
type AmazonSNSSender struct {
	AccessKeyID string
	SecretKey   string
	Region      string
	SenderID    string
	client      *sns.Client
}

// NewAmazonSNSSender creates a new Amazon SNS SMS sender.
