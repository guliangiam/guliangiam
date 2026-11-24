package saml

import (
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
)

// SignResponse signs a SAML Response XML string using the provided RSA key and certificate.
// It adds an enveloped XML-DSIG signature to the Assertion element.
func SignResponse(responseXML string, key *rsa.PrivateKey, cert *x509.Certificate) (string, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(responseXML); err != nil {
		return "", apperrors.WrapInternalf(err, "parse SAML response XML")
	}

	// Find the Assertion element to sign.
	assertion := doc.FindElement("//saml:Assertion")
	if assertion == nil {
		return "", apperrors.NewNotFound("saml:Assertion element not found in response")
	}

	assertionID := assertion.SelectAttrValue("ID", "")
	if assertionID == "" {
		return "", apperrors.NewNotFound("Assertion element has no ID attribute")
	}

	// Create the signing context using the modern API.
	signingContext, err := dsig.NewSigningContext(key, [][]byte{cert.Raw})
	if err != nil {
		return "", apperrors.WrapInternalf(err, "create signing context")
	}
	signingContext.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")

	// Sign the Assertion element.
	signedElement, err := signingContext.SignEnveloped(assertion)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "sign SAML assertion")
	}

	// Replace the original Assertion with the signed version.
	parent := assertion.Parent()
	if parent != nil {
		for i, child := range parent.Child {
			if child == assertion {
				parent.Child[i] = signedElement
				break
			}
		}
	}

	return doc.WriteToString()
}

// BuildSignedSAMLResponse creates a complete SAML Response with optional XML-DSIG signature.
func BuildSignedSAMLResponse(responseID, assertionID, issueInstant, notOnOrAfter, inResponseTo, acsURL string,
	userAttributes map[string]string, nameID, issuer string,
	key *rsa.PrivateKey, cert *x509.Certificate) (string, error) {

	// Build unsigned response first.
	unsignedXML := buildUnsignedSAMLResponse(responseID, assertionID, issueInstant, notOnOrAfter,
		inResponseTo, acsURL, userAttributes, nameID, issuer)

	// If no signing key, return unsigned.
	if key == nil || cert == nil {
		return unsignedXML, nil
	}

	// Sign the response.
	return SignResponse(unsignedXML, key, cert)
}

func buildUnsignedSAMLResponse(responseID, assertionID, issueInstant, notOnOrAfter, inResponseTo, acsURL string,
	attributes map[string]string, nameID, issuer string) string {

	attrsXML := ""
	for name, value := range attributes {
		attrsXML += fmt.Sprintf(`
      <saml:Attribute Name="%s" NameFormat="urn:oasis:names:tc:SAML:2.0:attrname-format:basic">
        <saml:AttributeValue>%s</saml:AttributeValue>
      </saml:Attribute>`, escapeXML(name), escapeXML(value))
	}

	return fmt.Sprintf(`<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="%s" Version="2.0" IssueInstant="%s" Destination="%s" InResponseTo="%s">
  <saml:Issuer>%s</saml:Issuer>
  <samlp:Status>
    <samlp:StatusCode Value="urn:oasis:names:tc:SAML:2.0:status:Success"/>
  </samlp:Status>
  <saml:Assertion xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema" ID="%s" Version="2.0" IssueInstant="%s">
    <saml:Issuer>%s</saml:Issuer>
    <saml:Subject>
      <saml:NameID Format="urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress">%s</saml:NameID>
      <saml:SubjectConfirmation Method="urn:oasis:names:tc:SAML:2.0:cm:bearer">
        <saml:SubjectConfirmationData NotOnOrAfter="%s" Recipient="%s" InResponseTo="%s"/>
      </saml:SubjectConfirmation>
    </saml:Subject>
    <saml:Conditions NotBefore="%s" NotOnOrAfter="%s">
      <saml:AudienceRestriction>
        <saml:Audience>%s</saml:Audience>
      </saml:AudienceRestriction>
    </saml:Conditions>
    <saml:AuthnStatement AuthnInstant="%s" SessionIndex="%s">
      <saml:AuthnContext>
        <saml:AuthnContextClassRef>urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport</saml:AuthnContextClassRef>
      </saml:AuthnContext>
    </saml:AuthnStatement>
    <saml:AttributeStatement>%s
    </saml:AttributeStatement>
  </saml:Assertion>
</samlp:Response>`,
		responseID, issueInstant, acsURL, inResponseTo,
		escapeXML(issuer),
		assertionID, issueInstant,
		escapeXML(issuer),
		escapeXML(nameID),
		notOnOrAfter, acsURL, inResponseTo,
		issueInstant, notOnOrAfter,
		escapeXML(issuer),
		issueInstant, assertionID,
		attrsXML,
	)
}

// BuildSAMLMetadataWithKey creates SAML IdP metadata including the signing certificate.
func BuildSAMLMetadataWithKey(entityID, ssoRedirectURL, ssoPostURL string, cert *x509.Certificate) []byte {
	certB64 := ""
	if cert != nil {
		certB64 = base64.StdEncoding.EncodeToString(cert.Raw)
	}

	keyDescriptor := ""
	if certB64 != "" {
		keyDescriptor = fmt.Sprintf(`
    <md:KeyDescriptor use="signing">
      <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
        <ds:X509Data>
          <ds:X509Certificate>%s</ds:X509Certificate>
        </ds:X509Data>
      </ds:KeyInfo>
    </md:KeyDescriptor>`, certB64)
	}

	metadata := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="%s" validUntil="%s">
  <md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">%s
    <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
    <md:NameIDFormat>urn:oasis:names:tc:SAML:2.0:nameid-format:transient</md:NameIDFormat>
    <md:NameIDFormat>urn:oasis:names:tc:SAML:2.0:nameid-format:persistent</md:NameIDFormat>
    <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="%s"/>
    <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="%s"/>
  </md:IDPSSODescriptor>
</md:EntityDescriptor>`,
		escapeXML(entityID),
		time.Now().Add(365*24*time.Hour).Format(time.RFC3339),
		keyDescriptor,
		escapeXML(ssoRedirectURL),
		escapeXML(ssoPostURL),
	)

	return []byte(metadata)
}

// Ensure crypto import is used.
var _ crypto.Signer = (*rsa.PrivateKey)(nil)

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
