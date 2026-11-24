package messaging

import (
	"context"
	"fmt"
	"net/http"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
)

func NewAmazonSNSSender(accessKeyID, secretKey, region, senderID string) *AmazonSNSSender {
	s := &AmazonSNSSender{
		AccessKeyID: accessKeyID,
		SecretKey:   secretKey,
		Region:      region,
		SenderID:    senderID,
	}

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretKey, "")),
	)
	if err == nil {
		s.client = sns.NewFromConfig(cfg)
	}

	return s
}

// Send sends an SMS via Amazon SNS Publish API.
func (s *AmazonSNSSender) Send(ctx context.Context, phone, code string) error {
	if s.client == nil {
		return apperrors.NewInternal("amazon sns: client not initialized")
	}

	message := fmt.Sprintf("Your verification code is: %s", code)

	input := &sns.PublishInput{
		PhoneNumber: aws.String(phone),
		Message:     aws.String(message),
	}

	if s.SenderID != "" {
		input.MessageAttributes = map[string]types.MessageAttributeValue{
			"AWS.SNS.SMS.SenderID": {
				DataType:    aws.String("String"),
				StringValue: aws.String(s.SenderID),
			},
		}
	}

	_, err := s.client.Publish(ctx, input)
	if err != nil {
		return apperrors.WrapInternalf(err, "amazon sns: publish")
	}
	return nil
}

// Ensure AmazonSNSSender implements SmsSender.
var _ SmsSender = (*AmazonSNSSender)(nil)

// AzureACSsmsSender sends SMS via Azure Communication Services.
type AzureACSsmsSender struct {
	ConnectionString string
	FromNumber       string
	client           *http.Client
}

// NewAzureACSsmsSender creates a new Azure ACS SMS sender.
