package saml

import (
	"crypto/x509"
	"encoding/xml"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ClockSkew is the tolerance window for time-based SAML conditions (±5 min
// per common SP deployment defaults). Configurable at the caller site.
const DefaultClockSkew = 5 * time.Minute

// ValidateSPAuthnRequestInput is the decoded XML of a SAMLp AuthnRequest.
// Keep this in infra so app stays thin — no xml.Unmarshal inside use cases.
type ValidateSPAuthnRequestInput struct {
	XMLName                     xml.Name `xml:"AuthnRequest"`
	ID                          string   `xml:"ID,attr"`
	Version                     string   `xml:"Version,attr"`
	IssueInstant                string   `xml:"IssueInstant,attr"`
	Destination                 string   `xml:"Destination,attr"`
	AssertionConsumerServiceURL string   `xml:"AssertionConsumerServiceURL,attr"`
	ProtocolBinding             string   `xml:"ProtocolBinding,attr"`
	Issuer                      string   `xml:"Issuer"`
}

// ValidateSPAuthnRequestConfig carries the IdP's expected values for the
// seven SP-side AuthnRequest checks. Any mismatch → error.
type ValidateSPAuthnRequestConfig struct {
	ExpectedEntityID    string        // IdP entity ID (Issuer value we expect SP to send as Destination/ACS)
	ExpectedDestination string        // IdP SSO URL this request must target
	ExpectedACSURL      string        // SP ACS URL registered in metadata
	ExpectedBinding     string        // e.g. HTTP-POST
	MaxAge              time.Duration // max age of IssueInstant before we reject (spec ≈ 5 min)
	ClockSkew           time.Duration
}

// ValidateSPAuthnRequest runs the seven SP-side AuthnRequest checks of 设计报告 15.3
// (protocol anchors: SAML 2.0 Core §3.4.1 <AuthnRequest> attributes — ID,
// Version, IssueInstant, Destination, AssertionConsumerServiceURL,
// ProtocolBinding — every mismatch fails closed):
//  1. Version == "2.0"
//  2. ID non-empty and starts with "_" (SAML convention)
//  3. Issuer matches expected SP entity ID
//  4. Destination matches expected IdP SSO URL
//  5. AssertionConsumerServiceURL matches registered SP ACS URL
//  6. ProtocolBinding matches expected (if non-empty config)
//  7. IssueInstant within [now-maxAge-skew, now+skew] window
func ValidateSPAuthnRequest(in *ValidateSPAuthnRequestInput, cfg ValidateSPAuthnRequestConfig) error {
	if cfg.ClockSkew == 0 {
		cfg.ClockSkew = DefaultClockSkew
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = 5 * time.Minute
	}
	now := time.Now().UTC()

	if in.Version != "2.0" {
		return apperrors.NewInvalidArgumentf("authn request version=%q, expected 2.0", in.Version)
	}
	if in.ID == "" {
		return apperrors.NewInvalidArgument("authn request ID is empty")
	}
	if !startsWith(in.ID, "_") {
		return apperrors.NewInvalidArgumentf("authn request ID must start with '_' per SAML convention, got %q", in.ID)
	}
	if in.Issuer == "" {
		return apperrors.NewInvalidArgument("authn request Issuer is empty")
	}
	if cfg.ExpectedEntityID != "" && in.Issuer != cfg.ExpectedEntityID {
		return apperrors.NewInvalidArgumentf("authn request Issuer=%q, expected SP entity ID %q", in.Issuer, cfg.ExpectedEntityID)
	}
	if in.Destination == "" {
		return apperrors.NewInvalidArgument("authn request Destination is empty")
	}
	if cfg.ExpectedDestination != "" && in.Destination != cfg.ExpectedDestination {
		return apperrors.NewInvalidArgumentf("authn request Destination=%q, expected IdP SSO URL %q", in.Destination, cfg.ExpectedDestination)
	}
	if cfg.ExpectedACSURL != "" && in.AssertionConsumerServiceURL != cfg.ExpectedACSURL {
		return apperrors.NewInvalidArgumentf("authn request ACS URL=%q, expected registered %q", in.AssertionConsumerServiceURL, cfg.ExpectedACSURL)
	}
	if cfg.ExpectedBinding != "" && in.ProtocolBinding != cfg.ExpectedBinding {
		return apperrors.NewInvalidArgumentf("authn request ProtocolBinding=%q, expected %q", in.ProtocolBinding, cfg.ExpectedBinding)
	}
	if in.IssueInstant == "" {
		return apperrors.NewInvalidArgument("authn request IssueInstant is empty")
	}
	issueTime, err := time.Parse(time.RFC3339, in.IssueInstant)
	if err != nil {
		return apperrors.NewInvalidArgumentf("authn request IssueInstant not RFC3339: %v", err)
	}
	issueTime = issueTime.UTC()
	if now.Sub(issueTime) > cfg.MaxAge+cfg.ClockSkew {
		return apperrors.NewInvalidArgumentf("authn request IssueInstant=%s is too old (now=%s, maxAge=%s)",
			in.IssueInstant, now.Format(time.RFC3339), cfg.MaxAge)
	}
	if issueTime.Sub(now) > cfg.ClockSkew {
		return apperrors.NewInvalidArgumentf("authn request IssueInstant=%s is in the future (now=%s, skew=%s)",
			in.IssueInstant, now.Format(time.RFC3339), cfg.ClockSkew)
	}

	return nil
}

// ──────────────────────────────────────────────────────────────
// Assertion validation
// ──────────────────────────────────────────────────────────────

// ValidateSAMLAssertionInput carries the decoded fields we need from the
// IdP's <Assertion>. We don't need to re-parse the whole XML — the caller
// (samling_sp.go) already has its own XML struct.
type ValidateSAMLAssertionInput struct {
	ID                  string
	Issuer              string
	IssueInstant        string
	SubjectNameID       string
	NotBefore           string // from Conditions; optional
	NotOnOrAfter        string // from Conditions; optional
	Audience            string // single AudienceRestriction.Audience value
	Recipient           string // SubjectConfirmationData.Recipient
	InResponseTo        string // SubjectConfirmationData.InResponseTo; should match our AuthnRequest ID
	SignaturePresent    bool   // whether <ds:Signature> was present on the Assertion element
	ResponseDestination string // top-level Response Destination (trusted only when Response is signed)
}

// ValidateSAMLAssertionConfig carries the SP's expected values.
type ValidateSAMLAssertionConfig struct {
	ExpectedIssuer       string // IdP entity ID
	ExpectedAudience     string // our SP entity ID (we're the intended audience)
	ExpectedRecipient    string // our ACS URL
	ExpectedDestination  string // absolute URL the Response was delivered to
	ExpectedInResponseTo string // our AuthnRequest ID (if flow was request/response); "" means skip
	MaxAssertionAge      time.Duration
	ClockSkew            time.Duration
	// IdPSigningCert is the IdP's X.509 DER cert. If nil, signature validation
	// is skipped (test/dev only — production MUST supply it).
	IdPSigningCert *x509.Certificate
	// RawAssertion is needed for signature verification; pass nil to skip.
	RawAssertion []byte
}

// ValidateSAMLAssertion runs the five assertion checks of 设计报告 15.3
// (protocol anchors: SAML 2.0 Core §2.5.1 <Assertion>/<Conditions> and
// Web Browser SSO Profile §4.1.4.2 relying-party Destination/Recipient/
// InResponseTo validation; signature is verified before any field check):
//  1. Issuer matches expected IdP entity ID
//  2. NotBefore ≤ now + skew (assertion not yet valid)
//  3. NotOnOrAfter ≥ now - skew (assertion not expired)
//  4. AudienceRestriction contains our SP entity ID
//  5. SubjectConfirmationData.Recipient == our ACS URL
//     + Recipient.InResponseTo optional binding check (prevents response replay)
//     + signature (non-SAMLp: Sign) verification when IdPSigningCert supplied
func ValidateSAMLAssertion(in *ValidateSAMLAssertionInput, cfg ValidateSAMLAssertionConfig) error {
	if cfg.ClockSkew == 0 {
		cfg.ClockSkew = DefaultClockSkew
	}
	if cfg.MaxAssertionAge == 0 {
		cfg.MaxAssertionAge = 24 * time.Hour
	}
	now := time.Now().UTC()

	// Signature first: unsigned or unverifiable assertions must never reach the
	// field checks below.
	if !in.SignaturePresent {
		return apperrors.NewUnauthorized("assertion signature is missing")
	}
	if cfg.IdPSigningCert == nil || len(cfg.RawAssertion) == 0 {
		return apperrors.NewUnauthorized("assertion signature verification is not configured")
	}
	if err := verifyAssertionSignature(in, cfg); err != nil {
		return err
	}

	if in.Issuer == "" {
		return apperrors.NewInvalidArgument("assertion Issuer is empty")
	}
	if cfg.ExpectedIssuer != "" && in.Issuer != cfg.ExpectedIssuer {
		return apperrors.NewInvalidArgumentf("assertion Issuer=%q, expected IdP %q", in.Issuer, cfg.ExpectedIssuer)
	}

	if in.ID == "" {
		return apperrors.NewInvalidArgument("assertion ID is empty")
	}

	// IssueInstant freshness (a future or ancient assertion is never fresh).
	if in.IssueInstant != "" {
		issuedAt, err := time.Parse(time.RFC3339, in.IssueInstant)
		if err != nil {
			return apperrors.NewInvalidArgumentf("assertion IssueInstant not RFC3339: %v", err)
		}
		issuedAt = issuedAt.UTC()
		if issuedAt.Sub(now) > cfg.ClockSkew {
			return apperrors.NewInvalidArgumentf("assertion IssueInstant=%s is in the future (now=%s)",
				in.IssueInstant, now.Format(time.RFC3339))
		}
		if now.Sub(issuedAt) > cfg.MaxAssertionAge+cfg.ClockSkew {
			return apperrors.NewInvalidArgumentf("assertion IssueInstant=%s is too old (now=%s, maxAge=%s)",
				in.IssueInstant, now.Format(time.RFC3339), cfg.MaxAssertionAge)
		}
	}

	// NotBefore
	if in.NotBefore != "" {
		nb, err := time.Parse(time.RFC3339, in.NotBefore)
		if err != nil {
			return apperrors.NewInvalidArgumentf("assertion NotBefore not RFC3339: %v", err)
		}
		nb = nb.UTC()
		if now.Before(nb.Add(-cfg.ClockSkew)) {
			return apperrors.NewInvalidArgumentf("assertion not yet valid: NotBefore=%s, now=%s",
				in.NotBefore, now.Format(time.RFC3339))
		}
	}

	// NotOnOrAfter
	if in.NotOnOrAfter != "" {
		noa, err := time.Parse(time.RFC3339, in.NotOnOrAfter)
		if err != nil {
			return apperrors.NewInvalidArgumentf("assertion NotOnOrAfter not RFC3339: %v", err)
		}
		noa = noa.UTC()
		if now.After(noa.Add(cfg.ClockSkew)) {
			return apperrors.NewInvalidArgumentf("assertion expired: NotOnOrAfter=%s, now=%s",
				in.NotOnOrAfter, now.Format(time.RFC3339))
		}
	}

	// Audience
	if in.Audience == "" {
		return apperrors.NewInvalidArgument("assertion Audience is empty")
	}
	if cfg.ExpectedAudience != "" && in.Audience != cfg.ExpectedAudience {
		return apperrors.NewInvalidArgumentf("assertion Audience=%q, expected our SP entity ID %q",
			in.Audience, cfg.ExpectedAudience)
	}

	// Recipient
	if in.Recipient == "" {
		return apperrors.NewInvalidArgument("assertion Recipient is empty")
	}
	if cfg.ExpectedRecipient != "" && in.Recipient != cfg.ExpectedRecipient {
		return apperrors.NewInvalidArgumentf("assertion Recipient=%q, expected our ACS URL %q",
			in.Recipient, cfg.ExpectedRecipient)
	}

	// Destination (top-level Response attribute). Only enforced when the SP
	// configured the URL it expects the Response to be delivered to.
	if cfg.ExpectedDestination != "" {
		if in.ResponseDestination == "" {
			return apperrors.NewInvalidArgument("response Destination is empty")
		}
		if in.ResponseDestination != cfg.ExpectedDestination {
			return apperrors.NewInvalidArgumentf("response Destination=%q, expected our ACS URL %q",
				in.ResponseDestination, cfg.ExpectedDestination)
		}
	}

	// InResponseTo binding
	if cfg.ExpectedInResponseTo != "" {
		if in.InResponseTo == "" {
			return apperrors.NewInvalidArgumentf("assertion InResponseTo is empty but we expected %q (response replay possible)",
				cfg.ExpectedInResponseTo)
		}
		if in.InResponseTo != cfg.ExpectedInResponseTo {
			return apperrors.NewInvalidArgumentf("assertion InResponseTo=%q, expected our AuthnRequest ID %q",
				in.InResponseTo, cfg.ExpectedInResponseTo)
		}
	}

	return nil
}

// verifyAssertionSignature verifies the XML-DSIG signature over the Response or
// the nested Assertion and binds the caller-supplied fields to the element that
// was actually signed (prevents field/signature substitution).
func verifyAssertionSignature(in *ValidateSAMLAssertionInput, cfg ValidateSAMLAssertionConfig) error {
	_, root, err := parseSAMLResponseDocument(string(cfg.RawAssertion))
	if err != nil {
		return apperrors.WrapUnauthorized(err, "parse raw SAML response")
	}
	signedElement, responseSigned, err := verifySAMLResponseSignature(root, cfg.IdPSigningCert)
	if err != nil {
		return err
	}

	assertionElement := firstChild(signedElement, "Assertion")
	if !responseSigned {
		assertionElement = signedElement
	}
	if assertionElement == nil || assertionElement.Tag != "Assertion" {
		return apperrors.NewUnauthorized("SAML response has no signed Assertion")
	}

	signed := parseAssertionFields(assertionElement)
	if signed.id != in.ID {
		return apperrors.NewUnauthorized("SAML assertion ID does not match the signed element")
	}
	if signed.issuer != in.Issuer {
		return apperrors.NewUnauthorized("SAML assertion Issuer does not match the signed element")
	}
	if signed.recipient != in.Recipient {
		return apperrors.NewUnauthorized("SAML assertion Recipient does not match the signed element")
	}
	if signed.inResponseTo != in.InResponseTo {
		return apperrors.NewUnauthorized("SAML assertion InResponseTo does not match the signed element")
	}
	if in.Audience != "" && !containsString(signed.audiences, in.Audience) {
		return apperrors.NewUnauthorized("SAML assertion Audience does not match the signed element")
	}
	return nil
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
