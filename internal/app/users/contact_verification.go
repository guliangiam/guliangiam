package users

import (
	"context"
	"strings"
	"time"

	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ContactVerificationChannel is the closed set of verifiable contact channels.
type ContactVerificationChannel string

const (
	ContactChannelEmail ContactVerificationChannel = "email"
	ContactChannelPhone ContactVerificationChannel = "phone"
)

// ContactVerificationTicket is what the caller may learn about a sent code.
// The recipient is always masked; the plaintext destination and the code never
// leave the server.
type ContactVerificationTicket struct {
	Channel   string    `json:"channel"`
	Recipient string    `json:"recipient"`
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
}

// ContactVerificationGateway delivers and consumes verification codes for the
// user detail page. It is implemented by the verifications application service
// through a bootstrap adapter so app/users does not import app/verifications.
type ContactVerificationGateway interface {
	Send(ctx context.Context, organizationID, channel, recipient string) (ContactVerificationTicket, error)
	Consume(ctx context.Context, organizationID, channel, recipient, code string) error
}

func (s *Service) SendUserContactVerification(ctx context.Context, organizationID, userID, channel string) (*ContactVerificationTicket, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	normalized, recipient, err := s.resolveContactRecipient(ctx, organizationID, userID, channel)
	if err != nil {
		return nil, err
	}
	if s.ContactVerification == nil {
		return nil, apperrors.NewUnavailable("contact verification provider is not configured")
	}

	ticket, err := s.ContactVerification.Send(ctx, organizationID, normalized, recipient)
	if err != nil {
		return nil, err
	}
	ticket.Channel = normalized
	ticket.Recipient = maskContact(normalized, recipient)
	return &ticket, nil
}

// VerifyUserContactVerification consumes the code and flips the profile flag.
// A code is only accepted for the address currently stored on the profile, so a
// stale code for an old address cannot mark the new address verified.
func (s *Service) VerifyUserContactVerification(ctx context.Context, organizationID, userID, channel, code string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	normalized, recipient, err := s.resolveContactRecipient(ctx, organizationID, userID, channel)
	if err != nil {
		return err
	}
	if strings.TrimSpace(code) == "" {
		return apperrors.NewInvalidArgument("code is required")
	}
	if s.ContactVerification == nil {
		return apperrors.NewUnavailable("contact verification provider is not configured")
	}
	if err := s.ContactVerification.Consume(ctx, organizationID, normalized, recipient, code); err != nil {
		return err
	}

	user, err := s.UOW.Users().GetByIDForOrganization(ctx, userID, organizationID)
	if err != nil {
		return err
	}
	emailVerified := user.Profile.EmailVerified
	phoneVerified := user.Profile.PhoneVerified
	switch normalized {
	case string(ContactChannelEmail):
		emailVerified = true
	case string(ContactChannelPhone):
		phoneVerified = true
	}
	return s.UOW.Users().SetContactVerifiedForOrganization(ctx, userID, organizationID, emailVerified, phoneVerified)
}

func (s *Service) resolveContactRecipient(ctx context.Context, organizationID, userID, channel string) (string, string, error) {
	normalized := strings.ToLower(strings.TrimSpace(channel))
	switch ContactVerificationChannel(normalized) {
	case ContactChannelEmail, ContactChannelPhone:
	default:
		return "", "", apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "channel must be 'email' or 'phone'", map[string]any{
			"field":   "channel",
			"allowed": []string{string(ContactChannelEmail), string(ContactChannelPhone)},
		})
	}
	if strings.TrimSpace(userID) == "" {
		return "", "", apperrors.NewInvalidArgument("userId is required")
	}
	user, err := s.UOW.Users().GetByIDForOrganization(ctx, userID, organizationID)
	if err != nil {
		return "", "", err
	}
	recipient := contactValue(user.Profile, ContactVerificationChannel(normalized))
	if recipient == "" {
		return "", "", apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, normalized+" is not set on this profile", map[string]any{
			"field": normalized,
		})
	}
	return normalized, recipient, nil
}

func contactValue(profile userdomain.UserProfile, channel ContactVerificationChannel) string {
	if channel == ContactChannelPhone {
		return strings.TrimSpace(profile.Phone)
	}
	return strings.TrimSpace(profile.Email)
}

// maskContact keeps just enough of an address for the operator to recognise it
// while never echoing the full value back through the API.
func maskContact(channel, value string) string {
	if value == "" {
		return ""
	}
	if channel == string(ContactChannelPhone) {
		if len(value) <= 4 {
			return strings.Repeat("*", len(value))
		}
		visible := 3
		if len(value) < 8 {
			visible = 1
		}
		return value[:visible] + strings.Repeat("*", len(value)-visible-4) + value[len(value)-4:]
	}
	at := strings.LastIndex(value, "@")
	if at <= 0 {
		return strings.Repeat("*", len(value))
	}
	local, domain := value[:at], value[at:]
	if len(local) <= 2 {
		return string(local[0]) + strings.Repeat("*", len(local)-1) + domain
	}
	return local[:2] + strings.Repeat("*", len(local)-2) + domain
}
