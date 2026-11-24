package messaging

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// AmazonSESEmailSender sends email through Amazon SES API v2 SendEmail
// (POST /v2/email/outbound-emails, SigV4 service=ses, aws-sdk-go-v2 sesv2).
// https://docs.aws.amazon.com/ses/latest/APIReference-V2/API_SendEmail.html
type AmazonSESEmailSender struct {
	Region string
	From   string
	client *sesv2.Client
}

// NewAmazonSESEmailSender creates an SES v2 sender from static credentials.
// endpoint is optional (tests / VPC endpoints); empty uses the regional
// default resolved by the SDK.
func NewAmazonSESEmailSender(ctx context.Context, accessKeyID, secretAccessKey, region, from, endpoint string) (*AmazonSESEmailSender, error) {
	if accessKeyID == "" || secretAccessKey == "" {
		return nil, apperrors.NewInvalidArgument("amazon ses accessKeyId/secretAccessKey are required")
	}
	if region == "" {
		return nil, apperrors.NewInvalidArgument("amazon ses region is required")
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
	)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "amazon ses load config")
	}
	opts := func(o *sesv2.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	}
	return &AmazonSESEmailSender{
		Region: region,
		From:   from,
		client: sesv2.NewFromConfig(cfg, opts),
	}, nil
}

// Send delivers one message via SES v2 SendEmail (simple text content).
func (s *AmazonSESEmailSender) Send(ctx context.Context, to, subject, body string) error {
	if s.client == nil {
		return apperrors.NewInternal("amazon ses: client not initialized")
	}
	if s.From == "" {
		return apperrors.NewInvalidArgument("amazon ses configuration set from address is required")
	}
	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(s.From),
		Destination:      &types.Destination{ToAddresses: []string{to}},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{Data: aws.String(subject)},
				Body:    &types.Body{Text: &types.Content{Data: aws.String(body)}},
			},
		},
	}
	if _, err := s.client.SendEmail(ctx, input); err != nil {
		return apperrors.WrapInternalf(err, "amazon ses: send email")
	}
	return nil
}

var _ EmailSender = (*AmazonSESEmailSender)(nil)
