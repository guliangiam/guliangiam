package saml

import (
	domainauth "github.com/guliangiam/guliangiam/internal/domain/auth"
)

// Validator implements domainauth.SAMLValidator by delegating to the
// package-level validation functions. This adapter keeps infra/saml a thin
// pure-logic module while app layer depends only on the domain port interface.
type Validator struct{}

// NewValidator creates a new SAML Validator adapter.
func NewValidator() *Validator {
	return &Validator{}
}

// Compile-time check that Validator satisfies the domain port.
var _ domainauth.SAMLValidator = (*Validator)(nil)

func (v *Validator) ValidateSPAuthnRequest(in *domainauth.SAMLSPAuthnRequestInput, cfg domainauth.SAMLSPAuthnRequestConfig) error {
	return ValidateSPAuthnRequest(
		&ValidateSPAuthnRequestInput{
			XMLName:                     in.XMLName,
			ID:                          in.ID,
			Version:                     in.Version,
			IssueInstant:                in.IssueInstant,
			Destination:                 in.Destination,
			AssertionConsumerServiceURL: in.AssertionConsumerServiceURL,
			ProtocolBinding:             in.ProtocolBinding,
			Issuer:                      in.Issuer,
		},
		ValidateSPAuthnRequestConfig{
			ExpectedEntityID:    cfg.ExpectedEntityID,
			ExpectedDestination: cfg.ExpectedDestination,
			ExpectedACSURL:      cfg.ExpectedACSURL,
			ExpectedBinding:     cfg.ExpectedBinding,
			MaxAge:              cfg.MaxAge,
			ClockSkew:           cfg.ClockSkew,
		},
	)
}

func (v *Validator) ValidateSAMLAssertion(in *domainauth.SAMLAssertionInput, cfg domainauth.SAMLAssertionConfig) error {
	return ValidateSAMLAssertion(
		&ValidateSAMLAssertionInput{
			ID:                  in.ID,
			Issuer:              in.Issuer,
			IssueInstant:        in.IssueInstant,
			SubjectNameID:       in.SubjectNameID,
			NotBefore:           in.NotBefore,
			NotOnOrAfter:        in.NotOnOrAfter,
			Audience:            in.Audience,
			Recipient:           in.Recipient,
			InResponseTo:        in.InResponseTo,
			SignaturePresent:    in.SignaturePresent,
			ResponseDestination: in.ResponseDestination,
		},
		ValidateSAMLAssertionConfig{
			ExpectedIssuer:       cfg.ExpectedIssuer,
			ExpectedAudience:     cfg.ExpectedAudience,
			ExpectedRecipient:    cfg.ExpectedRecipient,
			ExpectedDestination:  cfg.ExpectedDestination,
			ExpectedInResponseTo: cfg.ExpectedInResponseTo,
			MaxAssertionAge:      cfg.MaxAssertionAge,
			ClockSkew:            cfg.ClockSkew,
			IdPSigningCert:       cfg.IdPSigningCert,
			RawAssertion:         cfg.RawAssertion,
		},
	)
}
