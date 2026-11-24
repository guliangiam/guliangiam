package saml

import (
	"crypto/x509"
	"encoding/pem"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
)

// certificateStore implements dsig.X509CertificateStore with a single IdP certificate.
type certificateStore struct {
	certificates []*x509.Certificate
}

// Certificates returns the stored X.509 certificates used for signature verification.
func (s *certificateStore) Certificates() ([]*x509.Certificate, error) {
	return s.certificates, nil
}

// ParseIdPCertificate decodes a PEM-encoded X.509 certificate.
func ParseIdPCertificate(idpCertPEM string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(idpCertPEM)))
	if block == nil {
		return nil, apperrors.NewInternal("failed to decode IdP certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse IdP certificate")
	}
	return cert, nil
}

// VerifySAMLResponse verifies the XML digital signature of a SAML response
// against the provided IdP certificate (PEM-encoded).
// It returns nil on successful verification, or a descriptive error on failure.
func VerifySAMLResponse(responseXML, idpCertPEM string) error {
	_, _, err := VerifySAMLResponseElement(responseXML, idpCertPEM)
	return err
}

// VerifySAMLResponseElement verifies the enveloped XML signature of a SAML
// Response. It accepts either a signature covering the <Response> element or
// one covering the first nested <Assertion> element (SAML 2.0 §5.4.2 allows
// both; the SP must at least verify the Assertion).
//
// The returned element is the one the validation context actually verified —
// callers must read NameID/attributes/conditions from it, never from an
// independently parsed copy. responseSigned reports whether the signature
// covered the <Response> element (and therefore the top-level Destination /
// InResponseTo attributes).
func VerifySAMLResponseElement(responseXML, idpCertPEM string) (signedElement *etree.Element, responseSigned bool, err error) {
	cert, err := ParseIdPCertificate(idpCertPEM)
	if err != nil {
		return nil, false, err
	}
	_, root, err := parseSAMLResponseDocument(responseXML)
	if err != nil {
		return nil, false, err
	}
	return verifySAMLResponseSignature(root, cert)
}

// parseSAMLResponseDocument parses a SAML Response document and returns its root.
func parseSAMLResponseDocument(responseXML string) (*etree.Document, *etree.Element, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(responseXML); err != nil {
		return nil, nil, apperrors.WrapInternalf(err, "parse SAML response XML")
	}
	root := doc.Root()
	if root == nil {
		return nil, nil, apperrors.NewUnauthorized("empty SAML response")
	}
	if root.Tag != "Response" {
		return nil, nil, apperrors.NewUnauthorized("SAML document is not a Response")
	}
	return doc, root, nil
}

// verifySAMLResponseSignature validates the signature on the Response element
// or, when the Response itself is unsigned, on its first Assertion child.
func verifySAMLResponseSignature(root *etree.Element, cert *x509.Certificate) (*etree.Element, bool, error) {
	if signatureElement(root) != nil {
		signed, err := validateSignedElement(root, cert)
		if err != nil {
			return nil, false, err
		}
		return signed, true, nil
	}

	if assertion := firstChild(root, "Assertion"); assertion != nil && signatureElement(assertion) != nil {
		signed, err := validateSignedElement(assertion, cert)
		if err != nil {
			return nil, false, err
		}
		return signed, false, nil
	}

	return nil, false, apperrors.NewUnauthorized("SAML response is not signed")
}

// validateSignedElement rejects weak (SHA-1) signature/digest algorithms before
// handing the element to the validation context, then verifies the enveloped
// signature against the pinned IdP certificate.
func validateSignedElement(el *etree.Element, cert *x509.Certificate) (*etree.Element, error) {
	if err := rejectWeakSignatureAlgorithms(signatureElement(el)); err != nil {
		return nil, err
	}

	store := &certificateStore{certificates: []*x509.Certificate{cert}}
	validationContext := dsig.NewDefaultValidationContext(store)
	signed, err := validationContext.Validate(el)
	if err != nil {
		return nil, apperrors.WrapUnauthorized(err, "SAML signature verification failed")
	}
	if signed == nil {
		return nil, apperrors.NewUnauthorized("SAML signature verification returned no signed element")
	}
	return signed, nil
}

// signatureElement returns the first direct <ds:Signature> child of el.
func signatureElement(el *etree.Element) *etree.Element {
	if el == nil {
		return nil
	}
	for _, child := range el.ChildElements() {
		if child.Tag != "Signature" {
			continue
		}
		// Prefix "ds" or an explicit xmldsig namespace declaration.
		if child.Space == "ds" || child.SelectAttrValue("xmlns", "") == dsig.Namespace {
			return child
		}
	}
	return nil
}

// rejectWeakSignatureAlgorithms enforces the design requirement to disable
// SHA-1 for SAML signature and digest algorithms.
func rejectWeakSignatureAlgorithms(sig *etree.Element) error {
	if sig == nil {
		return apperrors.NewUnauthorized("SAML signature element is missing")
	}
	signedInfo := firstChild(sig, "SignedInfo")
	if signedInfo == nil {
		return apperrors.NewUnauthorized("SAML signature has no SignedInfo")
	}
	if method := firstChild(signedInfo, "SignatureMethod"); method != nil {
		if isSHA1Algorithm(method.SelectAttrValue("Algorithm", "")) {
			return apperrors.NewUnauthorized("SAML signature uses the disabled SHA-1 algorithm")
		}
	}
	for _, ref := range signedInfo.ChildElements() {
		if ref.Tag != "Reference" {
			continue
		}
		if digest := firstChild(ref, "DigestMethod"); digest != nil {
			if isSHA1Algorithm(digest.SelectAttrValue("Algorithm", "")) {
				return apperrors.NewUnauthorized("SAML signature digest uses the disabled SHA-1 algorithm")
			}
		}
	}
	return nil
}

func isSHA1Algorithm(algorithm string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(algorithm)), "sha1")
}

// firstChild returns the first direct child element with the given local name.
func firstChild(el *etree.Element, localName string) *etree.Element {
	if el == nil {
		return nil
	}
	for _, child := range el.ChildElements() {
		if child.Tag == localName {
			return child
		}
	}
	return nil
}

// firstDescendant returns the first descendant element with the given local
// name (excluding el itself).
func firstDescendant(el *etree.Element, localName string) *etree.Element {
	if el == nil {
		return nil
	}
	for _, child := range el.ChildElements() {
		if child.Tag == localName {
			return child
		}
		if found := firstDescendant(child, localName); found != nil {
			return found
		}
	}
	return nil
}
