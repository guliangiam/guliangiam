package saml

import (
	"strings"
	"testing"
	"time"
)

func validAssertionInput(signer *samlTestSigner, xml string) (input *ValidateSAMLAssertionInput, cfg ValidateSAMLAssertionConfig) {
	now := time.Now().UTC()
	input = &ValidateSAMLAssertionInput{
		ID:                  "_assert-1",
		Issuer:              "https://idp.example.com",
		IssueInstant:        now.Format(time.RFC3339),
		NotBefore:           now.Add(-time.Minute).Format(time.RFC3339),
		NotOnOrAfter:        now.Add(5 * time.Minute).Format(time.RFC3339),
		Audience:            "https://sp.example.com",
		Recipient:           "https://sp.example.com/acs",
		InResponseTo:        "_req-1",
		SignaturePresent:    true,
		ResponseDestination: "https://sp.example.com/acs",
	}
	cfg = ValidateSAMLAssertionConfig{
		ExpectedIssuer:       "https://idp.example.com",
		ExpectedAudience:     "https://sp.example.com",
		ExpectedRecipient:    "https://sp.example.com/acs",
		ExpectedDestination:  "https://sp.example.com/acs",
		ExpectedInResponseTo: "_req-1",
		ClockSkew:            5 * time.Minute,
		MaxAssertionAge:      15 * time.Minute,
		IdPSigningCert:       signer.cert,
		RawAssertion:         []byte(xml),
	}
	return input, cfg
}

func TestValidateSAMLAssertion_AcceptsSignedAssertion(t *testing.T) {
	signer := newSAMLTestSigner(t)
	xml := buildSAMLTestResponse(t, signer, defaultSAMLTestResponseOptions())
	input, cfg := validAssertionInput(signer, xml)
	if err := ValidateSAMLAssertion(input, cfg); err != nil {
		t.Fatalf("valid signed assertion rejected: %v", err)
	}
}

func TestValidateSAMLAssertion_RejectsMissingSignature(t *testing.T) {
	signer := newSAMLTestSigner(t)
	xml := buildSAMLTestResponse(t, signer, defaultSAMLTestResponseOptions())
	input, cfg := validAssertionInput(signer, xml)
	input.SignaturePresent = false
	if err := ValidateSAMLAssertion(input, cfg); err == nil {
		t.Fatal("assertion without a signature flag was accepted")
	}
}

func TestValidateSAMLAssertion_RejectsTamperedRawAssertion(t *testing.T) {
	signer := newSAMLTestSigner(t)
	xml := buildSAMLTestResponse(t, signer, defaultSAMLTestResponseOptions())
	input, cfg := validAssertionInput(signer, xml)
	cfg.RawAssertion = []byte(strings.ReplaceAll(xml, "user@example.com", "attacker@example.com"))
	if err := ValidateSAMLAssertion(input, cfg); err == nil {
		t.Fatal("tampered assertion was accepted")
	}
}

func TestValidateSAMLAssertion_RejectsFieldSubstitution(t *testing.T) {
	signer := newSAMLTestSigner(t)
	xml := buildSAMLTestResponse(t, signer, defaultSAMLTestResponseOptions())
	input, cfg := validAssertionInput(signer, xml)
	input.ID = "_other-assertion"
	if err := ValidateSAMLAssertion(input, cfg); err == nil {
		t.Fatal("fields not covered by the signature were accepted")
	}
}

func TestValidateSAMLAssertion_RejectsInResponseToMismatch(t *testing.T) {
	signer := newSAMLTestSigner(t)
	xml := buildSAMLTestResponse(t, signer, defaultSAMLTestResponseOptions())
	input, cfg := validAssertionInput(signer, xml)
	input.InResponseTo = "_other-request"
	// Keep the signed element in sync so the mismatch is decided by the
	// InResponseTo binding check, not by the signature/field consistency guard.
	opts := defaultSAMLTestResponseOptions()
	opts.inResponseTo = "_other-request"
	cfg.RawAssertion = []byte(buildSAMLTestResponse(t, signer, opts))
	if err := ValidateSAMLAssertion(input, cfg); err == nil {
		t.Fatal("InResponseTo mismatch was accepted")
	}
}

func TestValidateSAMLAssertion_RejectsExpiredAssertion(t *testing.T) {
	signer := newSAMLTestSigner(t)
	now := time.Now().UTC()
	opts := defaultSAMLTestResponseOptions()
	opts.notBefore = now.Add(-30 * time.Minute)
	opts.notOnOrAfter = now.Add(-20 * time.Minute)
	xml := buildSAMLTestResponse(t, signer, opts)
	input, cfg := validAssertionInput(signer, xml)
	input.NotBefore = opts.notBefore.Format(time.RFC3339)
	input.NotOnOrAfter = opts.notOnOrAfter.Format(time.RFC3339)
	if err := ValidateSAMLAssertion(input, cfg); err == nil {
		t.Fatal("expired assertion was accepted")
	}
}

func TestValidateSAMLAssertion_RejectsWrongAudience(t *testing.T) {
	signer := newSAMLTestSigner(t)
	opts := defaultSAMLTestResponseOptions()
	opts.audience = "https://other-sp.example.com"
	xml := buildSAMLTestResponse(t, signer, opts)
	input, cfg := validAssertionInput(signer, xml)
	input.Audience = opts.audience
	if err := ValidateSAMLAssertion(input, cfg); err == nil {
		t.Fatal("wrong audience was accepted")
	}
}

func TestValidateSAMLAssertion_RejectsDestinationMismatch(t *testing.T) {
	signer := newSAMLTestSigner(t)
	opts := defaultSAMLTestResponseOptions()
	opts.destination = "https://other-sp.example.com/acs"
	xml := buildSAMLTestResponse(t, signer, opts)
	input, cfg := validAssertionInput(signer, xml)
	input.ResponseDestination = opts.destination
	if err := ValidateSAMLAssertion(input, cfg); err == nil {
		t.Fatal("wrong Destination was accepted")
	}
}
