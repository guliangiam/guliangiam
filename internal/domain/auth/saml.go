package auth

import (
	"context"
	"crypto/x509"
	"encoding/xml"
	"time"
)

// ReplayStore is the one-time reservation port for SP-side SAML state. It is
// deliberately identical in shape to infra/saml.ReplayStore so the application
// layer can depend on the port without importing the infrastructure package.
//
// Reserve must be atomic: it returns reserved=true only for the first sighting
// of key within ttl. A false, nil result means the key was already consumed
// (replay). An error is a hard failure and callers must fail closed.
type ReplayStore interface {
	Reserve(ctx context.Context, key string, ttl time.Duration) (reserved bool, err error)
}

// RelayStateSigner signs and verifies SAML RelayState values with a key that
// never leaves the server.
//
// The browser must not hold this key: a key compiled into the frontend bundle is
// readable by every visitor, so any signature it produces can be forged
// (审计 SEC-A01). Implementations live in infra/saml/.
type RelayStateSigner interface {
	// Sign binds relayState to the organization/application pair and stamps an
	// issue time, returning an opaque envelope safe to hand to the browser.
	Sign(relayState, organizationSlug, applicationSlug string) (string, error)
	// Verify checks the envelope's signature, TTL and org/app binding, and
	// returns the original relayState on success.
	Verify(signed, organizationSlug, applicationSlug string) (string, error)
}

// SAMLValidator is the domain port for SAML protocol validation.
// Implementations live in infra/saml/ — app layer depends only on this interface.
type SAMLValidator interface {
	// ValidateSPAuthnRequest validates an SP-initiated AuthnRequest per SAML 2.0 §3.4.1.
	ValidateSPAuthnRequest(in *SAMLSPAuthnRequestInput, cfg SAMLSPAuthnRequestConfig) error
	// ValidateSAMLAssertion validates an IdP's Assertion per SAML 2.0 §2.3.3.
	ValidateSAMLAssertion(in *SAMLAssertionInput, cfg SAMLAssertionConfig) error
}

// ──────────────────────────────────────────────────────────────
// AuthnRequest validation types
// ──────────────────────────────────────────────────────────────

// SAMLSPAuthnRequestInput is the decoded XML of a SAMLp AuthnRequest.
type SAMLSPAuthnRequestInput struct {
	XMLName                     xml.Name `xml:"AuthnRequest"`
	ID                          string   `xml:"ID,attr"`
	Version                     string   `xml:"Version,attr"`
	IssueInstant                string   `xml:"IssueInstant,attr"`
	Destination                 string   `xml:"Destination,attr"`
	AssertionConsumerServiceURL string   `xml:"AssertionConsumerServiceURL,attr"`
	ProtocolBinding             string   `xml:"ProtocolBinding,attr"`
	Issuer                      string   `xml:"Issuer"`
}

// SAMLSPAuthnRequestConfig carries the expected values for AuthnRequest validation.
type SAMLSPAuthnRequestConfig struct {
	ExpectedEntityID    string
	ExpectedDestination string
	ExpectedACSURL      string
	ExpectedBinding     string
	MaxAge              time.Duration
	ClockSkew           time.Duration
}

// ──────────────────────────────────────────────────────────────
// Assertion validation types
// ──────────────────────────────────────────────────────────────

// SAMLAssertionInput carries the decoded fields from the IdP's <Assertion>.
type SAMLAssertionInput struct {
	ID               string
	Issuer           string
	IssueInstant     string
	SubjectNameID    string
	NotBefore        string
	NotOnOrAfter     string
	Audience         string
	Recipient        string
	InResponseTo     string
	SignaturePresent bool
	// ResponseDestination is the top-level <Response Destination> value. It is
	// only trustworthy when the Response element itself is signed.
	ResponseDestination string
}

// SAMLAssertionConfig carries the SP's expected values for assertion validation.
type SAMLAssertionConfig struct {
	ExpectedIssuer       string
	ExpectedAudience     string
	ExpectedRecipient    string
	ExpectedDestination  string
	ExpectedInResponseTo string
	MaxAssertionAge      time.Duration
	ClockSkew            time.Duration
	IdPSigningCert       *x509.Certificate
	RawAssertion         []byte
}
