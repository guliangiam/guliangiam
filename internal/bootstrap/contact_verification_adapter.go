package bootstrap

import (
	"context"
	"time"

	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	appverifications "github.com/guliangiam/guliangiam/internal/app/verifications"
)

// contactVerificationAdapter bridges app/users to the verification service so
// neither application package has to import the other.
type contactVerificationAdapter struct {
	service *appverifications.Service
}

func newContactVerificationAdapter(service *appverifications.Service) appusers.ContactVerificationGateway {
	return contactVerificationAdapter{service: service}
}

func (a contactVerificationAdapter) Send(ctx context.Context, organizationID, channel, recipient string) (appusers.ContactVerificationTicket, error) {
	result, err := a.service.SendVerification(ctx, appverifications.SendVerificationCommand{
		OrganizationID: organizationID,
		Type:           channel,
		Recipient:      recipient,
	})
	if err != nil {
		return appusers.ContactVerificationTicket{}, err
	}
	ticket := appusers.ContactVerificationTicket{
		Channel:   channel,
		Recipient: recipient,
	}
	if !result.ExpiresAt.IsZero() {
		ticket.ExpiresAt = result.ExpiresAt
	} else {
		ticket.ExpiresAt = time.Now().Add(10 * time.Minute)
	}
	return ticket, nil
}

func (a contactVerificationAdapter) Consume(ctx context.Context, organizationID, channel, recipient, code string) error {
	return a.service.ConsumeVerificationCodeWithClientIP(ctx, organizationID, channel, recipient, code, "")
}
