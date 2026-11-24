package auth

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/beevik/etree"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	certdomain "github.com/guliangiam/guliangiam/internal/domain/certs"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	dsig "github.com/russellhaering/goxmldsig"
)

// SamlRequestInfo contains parsed information from a SAML AuthnRequest.
type SamlRequestInfo struct {
	ID           string `json:"id"`
	IssueInstant string `json:"issueInstant"`
	Version      string `json:"version"`
	Issuer       string `json:"issuer"`
	ACSURL       string `json:"acsUrl"`
	Destination  string `json:"destination"`
}

// samlAuthnRequestXML is the XML structure for decoding a SAML AuthnRequest.
type samlAuthnRequestXMLFull struct {
	XMLName      xml.Name `xml:"AuthnRequest"`
	ID           string   `xml:"ID,attr"`
	IssueInstant string   `xml:"IssueInstant,attr"`
	Version      string   `xml:"Version,attr"`
	Destination  string   `xml:"Destination,attr,omitempty"`
	Issuer       string   `xml:"Issuer,omitempty"`
	ACSURL       string   `xml:"AssertionConsumerServiceURL,attr,omitempty"`
}

// GenerateSamlMetadata generates the SAML IdP EntityDescriptor XML for the given application.
// It includes the signing certificate from the application's associated cert, and
// SingleSignOnService endpoints for both HTTP-Redirect and HTTP-POST bindings.
func (s *Service) GenerateSamlMetadata(ctx context.Context, applicationID string) (string, error) {
	application, err := resolveApplicationReference(ctx, s.UOW.Applications(), nil, applicationID)
	if err != nil {
		return "", err
	}

	cert := s.resolveApplicationCert(ctx, application)
	return buildSamlMetadataEtree(application, cert, s.IssuerURL, s.Clock.Now().UTC())
}

// GenerateSamlMetadataBySlug generates SAML IdP metadata by organization and application slug.
func (s *Service) GenerateSamlMetadataBySlug(ctx context.Context, orgSlug, appSlug string) (string, error) {
	org, err := s.UOW.Organizations().GetBySlug(ctx, orgSlug)
	if err != nil {
		return "", apperrors.NewNotFoundf("organization not found: %s", orgSlug)
	}

	app, err := s.UOW.Applications().GetByOrganizationAndSlug(ctx, org.ID, appSlug)
	if err != nil {
		return "", apperrors.NewNotFoundf("application not found: %s/%s", orgSlug, appSlug)
	}

	cert := s.resolveApplicationCert(ctx, app)
	return buildSamlMetadataEtree(app, cert, s.IssuerURL, s.Clock.Now().UTC())
}

// GenerateSamlResponse generates a signed SAML 2.0 Response XML for the authenticated user.
// It constructs the full SAML Response with Assertion, Conditions, AuthnStatement,
// and AttributeStatement, then signs it using the application's certificate.
func (s *Service) GenerateSamlResponse(ctx context.Context, app *applicationdomain.Application, user *userdomain.User, requestID, destination string) (string, error) {
	if requestID == "" {
		return "", apperrors.NewInvalidArgument("requestID is required")
	}
	if destination == "" {
		return "", apperrors.NewInvalidArgument("destination is required")
	}

	now := s.Clock.Now().UTC()
	responseID := fmt.Sprintf("_%s", s.generateID())
	assertionID := fmt.Sprintf("_%s", s.generateID())
	notOnOrAfter := now.Add(5 * time.Minute)

	// Build the SAML Response using etree.
	samlResponse := buildSamlResponseEtree(responseID, assertionID, now, notOnOrAfter,
		requestID, destination, app, user)

	// Attempt to sign the response.
	cert := s.resolveApplicationCert(ctx, app)
	privateKey := s.resolveApplicationPrivateKey(ctx, app)

	signedXML, err := signSamlResponseWithKey(samlResponse, privateKey, cert)
	if err != nil {
		return "", apperrors.WrapInternal(err, "sign SAML response")
	}

	return signedXML, nil
}

// maxSAMLRequestBytes 限制解码/解压后的 SAML AuthnRequest 体积：deflate 可把
// 极小报文膨胀成超大数据，限制读取量可避免内存/CPU 被解压炸弹耗尽。
const maxSAMLRequestBytes = 512 * 1024

// ParseSamlRequest parses a Base64-encoded SAML AuthnRequest, handling both
// deflate-compressed and plain XML formats.
func (s *Service) ParseSamlRequest(ctx context.Context, request string) (*SamlRequestInfo, error) {
	requestBytes, err := decodeSAMLMessage(request)
	if err != nil {
		return nil, err
	}

	var req samlAuthnRequestXMLFull
	if err := xml.Unmarshal(requestBytes, &req); err != nil {
		return nil, apperrors.NewInvalidArgument("failed to parse SAML AuthnRequest XML")
	}

	return &SamlRequestInfo{
		ID:           req.ID,
		IssueInstant: req.IssueInstant,
		Version:      req.Version,
		Issuer:       req.Issuer,
		ACSURL:       req.ACSURL,
		Destination:  req.Destination,
	}, nil
}

// decodeSAMLMessage decodes a Base64-encoded SAML protocol message (AuthnRequest
// or LogoutRequest), accepting standard/URL-safe base64 and both plain XML and
// HTTP-Redirect deflate compression.
func decodeSAMLMessage(encoded string) ([]byte, error) {
	if strings.TrimSpace(encoded) == "" {
		return nil, apperrors.NewInvalidArgument("SAML request is required")
	}

	// Normalize base64 padding.
	normalized := strings.ReplaceAll(encoded, " ", "+")

	data, err := base64.StdEncoding.DecodeString(normalized)
	if err != nil {
		// Try URL-safe encoding.
		data, err = base64.URLEncoding.DecodeString(normalized)
		if err != nil {
			return nil, apperrors.NewInvalidArgument("invalid base64 encoding in SAML request")
		}
	}

	if bytes.Contains(data, []byte("xmlns")) {
		if len(data) > maxSAMLRequestBytes {
			return nil, apperrors.NewInvalidArgument("SAML request exceeds size limit")
		}
		return data, nil
	}

	// Deflate-compressed：用 LimitReader 防止解压炸弹（极小压缩包解出超大 XML）。
	var buffer bytes.Buffer
	rdr := flate.NewReader(bytes.NewReader(data))
	n, err := io.Copy(&buffer, io.LimitReader(rdr, maxSAMLRequestBytes+1))
	if err != nil {
		rdr.Close()
		return nil, apperrors.NewInvalidArgument("failed to decompress SAML request")
	}
	rdr.Close()
	if n > maxSAMLRequestBytes {
		return nil, apperrors.NewInvalidArgument("SAML request exceeds size limit")
	}
	return buffer.Bytes(), nil
}

// resolveApplicationCert attempts to find a certificate for the given application.
func (s *Service) resolveApplicationCert(ctx context.Context, app *applicationdomain.Application) *x509.Certificate {
	if s.UOW.Certs() == nil {
		return nil
	}
	certs, _, err := s.UOW.Certs().List(ctx, certdomain.ListQuery{
		OrganizationID: app.OrganizationID,
		PageSize:       10,
		Scope:          tokenTypeJWT,
	})
	if err != nil || len(certs) == 0 {
		return nil
	}

	// Find a cert that has both certificate and private key.
	for _, c := range certs {
		if c.Certificate == "" || c.PrivateKey == "" {
			continue
		}
		block, _ := pem.Decode([]byte(c.Certificate))
		if block == nil {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		return cert
	}
	return nil
}

// resolveApplicationPrivateKey attempts to find the RSA private key for the given application.
func (s *Service) resolveApplicationPrivateKey(ctx context.Context, app *applicationdomain.Application) *rsa.PrivateKey {
	if s.UOW.Certs() == nil {
		return nil
	}
	certs, _, err := s.UOW.Certs().List(ctx, certdomain.ListQuery{
		OrganizationID: app.OrganizationID,
		PageSize:       10,
		Scope:          tokenTypeJWT,
	})
	if err != nil || len(certs) == 0 {
		return nil
	}

	for _, c := range certs {
		if c.PrivateKey == "" {
			continue
		}
		block, _ := pem.Decode([]byte(c.PrivateKey))
		if block == nil {
			continue
		}
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			// Try PKCS8.
			keyInterface, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err2 != nil {
				continue
			}
			key, ok := keyInterface.(*rsa.PrivateKey)
			if !ok {
				continue
			}
			return key
		}
		return key
	}
	return nil
}

// generateID creates a unique identifier for SAML elements.
func (s *Service) generateID() string {
	id, err := s.UUIDGenerator.NewString()
	if err != nil {
		// Fallback to timestamp-based ID.
		return fmt.Sprintf("%d", s.Clock.Now().UnixNano())
	}
	return id
}

// buildSamlMetadataEtree constructs a SAML IdP EntityDescriptor XML using etree.
// issuerBase is the configured public issuer URL; endpoint Locations must be
// absolute so external SPs can consume the metadata, and an unconfigured issuer
// fails closed rather than publishing relative URLs.
func buildSamlMetadataEtree(app *applicationdomain.Application, cert *x509.Certificate, issuerBase string, now time.Time) (string, error) {
	issuerBase = strings.TrimSuffix(strings.TrimSpace(issuerBase), "/")
	if issuerBase == "" {
		return "", apperrors.NewInternal("SAML metadata requires a configured issuer URL")
	}

	doc := etree.NewDocument()

	entityDesc := doc.CreateElement("md:EntityDescriptor")
	entityDesc.CreateAttr("xmlns:md", "urn:oasis:names:tc:SAML:2.0:metadata")
	entityDesc.CreateAttr("xmlns:ds", "http://www.w3.org/2000/09/xmldsig#")
	entityDesc.CreateAttr("entityID", app.Slug)
	entityDesc.CreateAttr("validUntil", now.Add(365*24*time.Hour).Format(time.RFC3339))

	idpSSODesc := entityDesc.CreateElement("md:IDPSSODescriptor")
	idpSSODesc.CreateAttr("WantAuthnRequestsSigned", "false")
	idpSSODesc.CreateAttr("protocolSupportEnumeration", "urn:oasis:names:tc:SAML:2.0:protocol")

	// Add signing key descriptor if cert is available.
	if cert != nil {
		keyDesc := idpSSODesc.CreateElement("md:KeyDescriptor")
		keyDesc.CreateAttr("use", "signing")
		keyInfo := keyDesc.CreateElement("ds:KeyInfo")
		x509Data := keyInfo.CreateElement("ds:X509Data")
		x509Cert := x509Data.CreateElement("ds:X509Certificate")
		x509Cert.SetText(base64.StdEncoding.EncodeToString(cert.Raw))
	}

	// NameID formats.
	for _, format := range []string{
		"urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
		"urn:oasis:names:tc:SAML:2.0:nameid-format:persistent",
		"urn:oasis:names:tc:SAML:2.0:nameid-format:transient",
	} {
		nameIDFormat := idpSSODesc.CreateElement("md:NameIDFormat")
		nameIDFormat.SetText(format)
	}

	// SSO endpoint (absolute URL on the configured issuer).
	ssoURL := issuerBase + "/api/v1/saml/sso/" + app.ID

	redirectService := idpSSODesc.CreateElement("md:SingleSignOnService")
	redirectService.CreateAttr("Binding", "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect")
	redirectService.CreateAttr("Location", ssoURL)

	postService := idpSSODesc.CreateElement("md:SingleSignOnService")
	postService.CreateAttr("Binding", "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST")
	postService.CreateAttr("Location", ssoURL)

	// SAML attributes.
	attrs := []struct {
		Name         string
		FriendlyName string
	}{
		{"Email", "E-Mail"},
		{"DisplayName", fieldDisplayName},
		{"Name", "Name"},
		{"Roles", "Roles"},
	}
	for _, attr := range attrs {
		attrEl := idpSSODesc.CreateElement("md:Attribute")
		attrEl.CreateAttr("xmlns", "urn:oasis:names:tc:SAML:2.0:assertion")
		attrEl.CreateAttr("Name", attr.Name)
		attrEl.CreateAttr("NameFormat", "urn:oasis:names:tc:SAML:2.0:attrname-format:basic")
		attrEl.CreateAttr("FriendlyName", attr.FriendlyName)
	}

	xmlStr, err := doc.WriteToString()
	if err != nil {
		return "", apperrors.WrapInternal(err, "serialize SAML metadata")
	}
	return xmlStr, nil
}

// buildSamlResponseEtree constructs a SAML 2.0 Response element using etree.
func buildSamlResponseEtree(responseID, assertionID string, now, notOnOrAfter time.Time,
	inResponseTo, destination string, app *applicationdomain.Application, user *userdomain.User) *etree.Element {

	issueInstant := now.Format(time.RFC3339)
	notOnOrAfterStr := notOnOrAfter.Format(time.RFC3339)

	// Root Response element.
	samlResponse := &etree.Element{
		Space: "samlp",
		Tag:   "Response",
	}
	samlResponse.CreateAttr("xmlns:samlp", "urn:oasis:names:tc:SAML:2.0:protocol")
	samlResponse.CreateAttr("xmlns:saml", "urn:oasis:names:tc:SAML:2.0:assertion")
	samlResponse.CreateAttr("ID", responseID)
	samlResponse.CreateAttr("Version", "2.0")
	samlResponse.CreateAttr("IssueInstant", issueInstant)
	samlResponse.CreateAttr("Destination", destination)
	samlResponse.CreateAttr("InResponseTo", inResponseTo)

	// Issuer.
	samlResponse.CreateElement("saml:Issuer").SetText(app.Slug)

	// Status.
	status := samlResponse.CreateElement("samlp:Status")
	status.CreateElement("samlp:StatusCode").CreateAttr("Value", "urn:oasis:names:tc:SAML:2.0:status:Success")

	// Assertion.
	assertion := samlResponse.CreateElement("saml:Assertion")
	assertion.CreateAttr("xmlns:xsi", "http://www.w3.org/2001/XMLSchema-instance")
	assertion.CreateAttr("xmlns:xs", "http://www.w3.org/2001/XMLSchema")
	assertion.CreateAttr("ID", assertionID)
	assertion.CreateAttr("Version", "2.0")
	assertion.CreateAttr("IssueInstant", issueInstant)
	assertion.CreateElement("saml:Issuer").SetText(app.Slug)

	// Subject.
	subject := assertion.CreateElement("saml:Subject")
	nameIDValue := user.Username
	nameIDFormat := "urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified"
	if user.Profile.Email != "" {
		nameIDValue = user.Profile.Email
		nameIDFormat = "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"
	}
	nameID := subject.CreateElement("saml:NameID")
	nameID.CreateAttr("Format", nameIDFormat)
	nameID.SetText(nameIDValue)

	subjectConfirmation := subject.CreateElement("saml:SubjectConfirmation")
	subjectConfirmation.CreateAttr("Method", "urn:oasis:names:tc:SAML:2.0:cm:bearer")
	subjectConfirmationData := subjectConfirmation.CreateElement("saml:SubjectConfirmationData")
	subjectConfirmationData.CreateAttr("NotOnOrAfter", notOnOrAfterStr)
	subjectConfirmationData.CreateAttr("Recipient", destination)
	subjectConfirmationData.CreateAttr("InResponseTo", inResponseTo)

	// Conditions.
	conditions := assertion.CreateElement("saml:Conditions")
	conditions.CreateAttr("NotBefore", issueInstant)
	conditions.CreateAttr("NotOnOrAfter", notOnOrAfterStr)
	audienceRestriction := conditions.CreateElement("saml:AudienceRestriction")
	audienceRestriction.CreateElement("saml:Audience").SetText(app.Slug)

	// AuthnStatement.
	authnStatement := assertion.CreateElement("saml:AuthnStatement")
	authnStatement.CreateAttr("AuthnInstant", issueInstant)
	authnStatement.CreateAttr("SessionIndex", assertionID)
	authnStatement.CreateAttr("SessionNotOnOrAfter", notOnOrAfterStr)
	authnContext := authnStatement.CreateElement("saml:AuthnContext")
	authnContext.CreateElement("saml:AuthnContextClassRef").SetText("urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport")

	// AttributeStatement.
	attributeStatement := assertion.CreateElement("saml:AttributeStatement")

	// Email attribute.
	emailAttr := attributeStatement.CreateElement("saml:Attribute")
	emailAttr.CreateAttr("Name", "Email")
	emailAttr.CreateAttr("NameFormat", "urn:oasis:names:tc:SAML:2.0:attrname-format:basic")
	emailAttrValue := emailAttr.CreateElement("saml:AttributeValue")
	emailAttrValue.CreateAttr("xsi:type", "xs:string")
	emailAttrValue.SetText(user.Profile.Email)

	// Name attribute.
	nameAttr := attributeStatement.CreateElement("saml:Attribute")
	nameAttr.CreateAttr("Name", "Name")
	nameAttr.CreateAttr("NameFormat", "urn:oasis:names:tc:SAML:2.0:attrname-format:basic")
	nameAttrValue := nameAttr.CreateElement("saml:AttributeValue")
	nameAttrValue.CreateAttr("xsi:type", "xs:string")
	nameAttrValue.SetText(user.Username)

	// DisplayName attribute.
	displayNameAttr := attributeStatement.CreateElement("saml:Attribute")
	displayNameAttr.CreateAttr("Name", "DisplayName")
	displayNameAttr.CreateAttr("NameFormat", "urn:oasis:names:tc:SAML:2.0:attrname-format:basic")
	displayNameAttrValue := displayNameAttr.CreateElement("saml:AttributeValue")
	displayNameAttrValue.CreateAttr("xsi:type", "xs:string")
	displayNameAttrValue.SetText(user.Profile.DisplayName)

	// Roles attribute.
	rolesAttr := attributeStatement.CreateElement("saml:Attribute")
	rolesAttr.CreateAttr("Name", "Roles")
	rolesAttr.CreateAttr("NameFormat", "urn:oasis:names:tc:SAML:2.0:attrname-format:basic")
	for _, rb := range user.RoleBindings {
		roleValue := rolesAttr.CreateElement("saml:AttributeValue")
		roleValue.CreateAttr("xsi:type", "xs:string")
		roleValue.SetText(rb.RoleID)
	}

	return samlResponse
}

// signSamlResponseWithKey signs a SAML Response element using XML-DSIG with the provided key and certificate.
// If either key or cert is nil, returns an error — unsigned SAML responses are a security risk.
func signSamlResponseWithKey(samlResponse *etree.Element, key *rsa.PrivateKey, cert *x509.Certificate) (string, error) {
	if key == nil || cert == nil {
		return "", apperrors.NewInternalf("SAML signing requires both private key and certificate: key=%v cert=%v", key != nil, cert != nil)
	}

	// Sign the Assertion element.
	assertion := samlResponse.FindElement("./saml:Assertion")
	if assertion != nil {
		// The detached assertion subtree must carry the namespace declarations
		// it uses; exclusive C14N does not inherit them from the Response root.
		for _, attr := range samlResponse.Attr {
			if attr.Space != "xmlns" && attr.Key != "xmlns" {
				continue
			}
			declared := false
			for _, existing := range assertion.Attr {
				if existing.Space == attr.Space && existing.Key == attr.Key {
					declared = true
					break
				}
			}
			if !declared {
				name := attr.Key
				if attr.Space != "" {
					name = attr.Space + ":" + attr.Key
				}
				assertion.CreateAttr(name, attr.Value)
			}
		}
		signingContext, err := dsig.NewSigningContext(key, [][]byte{cert.Raw})
		if err != nil {
			return "", apperrors.WrapInternal(err, "create signing context")
		}
		signingContext.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")

		signedAssertion, err := signingContext.SignEnveloped(assertion)
		if err != nil {
			return "", apperrors.WrapInternal(err, "sign SAML assertion")
		}

		// Replace assertion in response.
		parent := assertion.Parent()
		if parent != nil {
			for i, child := range parent.Child {
				if child == assertion {
					parent.Child[i] = signedAssertion
					break
				}
			}
		}
	}

	// Sign the Response element.
	signingContext, err := dsig.NewSigningContext(key, [][]byte{cert.Raw})
	if err != nil {
		return "", apperrors.WrapInternal(err, "create response signing context")
	}
	signingContext.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")
	signingContext.Hash = crypto.SHA256

	signedResponse, err := signingContext.SignEnveloped(samlResponse)
	if err != nil {
		return "", apperrors.WrapInternal(err, "sign SAML response element")
	}

	doc := etree.NewDocument()
	doc.SetRoot(signedResponse)
	return doc.WriteToString()
}

// Ensure crypto import is used.
var _ crypto.Signer = (*rsa.PrivateKey)(nil)
