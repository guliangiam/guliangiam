package verifications

import "context"

type EmailSender interface {
	Send(ctx context.Context, to, subject, body string) error
}

type SmsSender interface {
	Send(ctx context.Context, phone, code string) error
}

// SmsCodeVerifier is the optional capability of a provider that generates and
// checks the verification code itself (Twilio Verify). When the resolved
// sender implements it, the stored verification keeps an empty code and the
// app delegates comparison to the provider.
type SmsCodeVerifier interface {
	CheckCode(ctx context.Context, phone, code string) (bool, error)
}

type SenderFactory interface {
	GetEmailSender(providerType string) (EmailSender, error)
	GetSmsSender(providerType string) (SmsSender, error)
}

// SenderResolver resolves the organization-scoped sender for a delivery
// channel. Implementations prefer the organization's configured provider row
// and only fall back to the deployment default when the organization has none.
type SenderResolver interface {
	ResolveEmailSender(ctx context.Context, organizationID string) (EmailSender, error)
	ResolveSmsSender(ctx context.Context, organizationID string) (SmsSender, error)
}

// ApplicationSenderResolver is the optional capability of a SenderResolver that
// understands application-scoped provider bindings. When the caller supplies an
// application id, the application's bound provider takes precedence over the
// organization default; an unbound application falls back to the organization
// row (and then the deployment default).
type ApplicationSenderResolver interface {
	ResolveEmailSenderForApplication(ctx context.Context, organizationID, applicationID string) (EmailSender, error)
	ResolveSmsSenderForApplication(ctx context.Context, organizationID, applicationID string) (SmsSender, error)
}

// ProviderIDSmsSender decorates an SmsSender with the provider row id and keeps
// the inner sender reachable, so provider-managed verification
// (SmsCodeVerifier, implemented by e.g. Twilio Verify) can still be detected
// through the decoration.
type ProviderIDSmsSender interface {
	SmsSender
	ProviderID() string
	InnerSmsSender() SmsSender
}

// ProviderIDEmailSender is the email counterpart of ProviderIDSmsSender.
type ProviderIDEmailSender interface {
	EmailSender
	ProviderID() string
	InnerEmailSender() EmailSender
}

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Type           string
	Status         string
	Recipient      string
}

// Repository is the verification persistence port.
type Repository interface {
	// GetByID is the system-scoped lookup used by the recipient/code send-and-
	// verify flows, which are outside the management by-ID tenant binding.
	// Management GET /verifications/{id} must use GetByIDForOrganization.
	GetByID(ctx context.Context, id string) (*Verification, error)
	// GetByIDForOrganization returns the verification only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Verification, error)
	List(ctx context.Context, query ListQuery) ([]*Verification, int64, error)
	Create(ctx context.Context, verification *Verification) error
	UpdateStatus(ctx context.Context, id string, status string) error
	GetActiveByRecipientAndCode(ctx context.Context, recipient string, code string, verificationType string) (*Verification, error)
	// GetActiveByRecipient returns the newest pending verification for a
	// recipient regardless of code. Provider-managed senders (those with an
	// empty stored code) need it because the code is generated upstream.
	GetActiveByRecipient(ctx context.Context, recipient string, verificationType string) (*Verification, error)
	// UpdateCode persists a provider-approved code so later checks in the same
	// flow take the normal code-match path.
	UpdateCode(ctx context.Context, id string, code string) error
}
