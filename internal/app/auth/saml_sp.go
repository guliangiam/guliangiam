package auth

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	domainauth "github.com/guliangiam/guliangiam/internal/domain/auth"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
)

// spRelayStateFieldCount is the number of fields carried by the signed
// RelayState envelope on the SP-initiated path:
// requestID|spEntityID|spACSURL|originalRelayState.
const (
	spRelayStateFieldCount = 4
	spRelayStateSeparator  = "|"
	xmlDSigNamespace       = "http://www.w3.org/2000/09/xmldsig#"
)

// SPInitiatedSAMLCommand contains parameters for SP-initiated SAML login.
type SPInitiatedSAMLCommand struct {
	OrganizationSlug string
	ApplicationSlug  string
	// SP metadata for constructing the AuthnRequest
	SPEntityID   string
	SPACSURL     string
	SPEntityCert string // Base64-encoded X.509 certificate of the SP
	RelayState   string
}

// SPInitiatedSAMLResult contains the result for SP-initiated SAML login.
type SPInitiatedSAMLResult struct {
	SAMLRequest string `json:"samlRequest"` // Base64-encoded AuthnRequest to send to IdP
	RedirectURL string `json:"redirectUrl"` // IdP SSO URL to redirect the user to
	RelayState  string `json:"relayState,omitempty"`
}

// BuildSPAuthnRequestCommand contains parameters for building a SP-initiated AuthnRequest.
type BuildSPAuthnRequestCommand struct {
	OrganizationSlug string
	ApplicationSlug  string
	SPEntityID       string
	SPACSURL         string
	RelayState       string
}

// BuildSPAuthnRequestResult contains the built AuthnRequest for SP-initiated SAML.
type BuildSPAuthnRequestResult struct {
	SAMLRequest string `json:"samlRequest"` // Base64-encoded AuthnRequest
	RequestID   string `json:"requestId"`   // The ID of the AuthnRequest
	SSOURL      string `json:"ssoUrl"`      // IdP SSO URL
	RelayState  string `json:"relayState,omitempty"`
}

// HandleSPInitiatedACSCommand handles the ACS callback from an SP-initiated SAML flow.
type HandleSPInitiatedACSCommand struct {
	OrganizationSlug string
	ApplicationSlug  string
	SAMLResponse     string // Base64-encoded SAML Response from the SP
	RelayState       string
	IPAddress        string
	UserAgent        string
}

// HandleSPInitiatedACSResult contains the result from the ACS callback.
type HandleSPInitiatedACSResult struct {
	User         *userdomain.User
	Organization string
	Application  *applicationdomain.Application
	NameID       string
	Attributes   map[string]string
	SessionIndex string
}

// BuildSPAuthnRequest constructs a SAML AuthnRequest that our IdP will process
// for SP-initiated SSO. This allows an external SP to redirect users to our IdP.
func (s *Service) BuildSPAuthnRequest(ctx context.Context, command BuildSPAuthnRequestCommand) (*BuildSPAuthnRequestResult, error) {
	if strings.TrimSpace(command.SPEntityID) == "" {
		return nil, apperrors.NewInvalidArgument("spEntityId is required")
	}
	if strings.TrimSpace(command.SPACSURL) == "" {
		return nil, apperrors.NewInvalidArgument("spAcsUrl is required")
	}

	// Resolve the application to get the IdP configuration.
	var application *applicationdomain.Application
	if strings.TrimSpace(command.ApplicationSlug) != "" && strings.TrimSpace(command.OrganizationSlug) != "" {
		org, err := s.UOW.Organizations().GetBySlug(ctx, command.OrganizationSlug)
		if err == nil {
			application, err = s.UOW.Applications().GetByOrganizationAndSlug(ctx, org.ID, command.ApplicationSlug)
			if err != nil {
				return nil, err
			}
		}
	}

	// Generate a unique request ID.
	requestID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return nil, apperrors.NewInternal("generate request id")
	}
	// SAML request IDs must start with "_" per spec.
	requestID = "_" + requestID

	now := s.Clock.Now().UTC()
	issueInstant := now.Format("2006-01-02T15:04:05Z")

	// Build the AuthnRequest XML.
	authnRequest := fmt.Sprintf(`<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="%s" Version="2.0" IssueInstant="%s" Destination="%s" AssertionConsumerServiceURL="%s" ProtocolBinding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST">
  <saml:Issuer>%s</saml:Issuer>
  <samlp:NameIDPolicy Format="urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress" AllowCreate="true"/>
</samlp:AuthnRequest>`,
		requestID,
		issueInstant,
		escapeXML(buildIdPSSOURL(command.OrganizationSlug, application)),
		escapeXML(command.SPACSURL),
		escapeXML(command.SPEntityID),
	)

	// Base64-encode the request.
	encodedRequest := base64.StdEncoding.EncodeToString([]byte(authnRequest))

	// Determine the IdP SSO URL.
	ssoURL := buildIdPSSOURL(command.OrganizationSlug, application)

	// RelayState is generated by us when we act as the SP, so it must be signed
	// with the server-held key before it travels through the browser. The
	// signature binds the organization/application pair, the AuthnRequest ID and
	// the SP's entity/ACS values, and expires (SEC-A01). The envelope therefore
	// carries everything HandleSPInitiatedACS needs to bind the response to the
	// request we issued.
	if s.RelayStateSigner == nil {
		return nil, apperrors.NewInternal("relay state signer not configured")
	}
	signed, err := s.RelayStateSigner.Sign(
		encodeSPRelayState(requestID, command.SPEntityID, command.SPACSURL, command.RelayState),
		command.OrganizationSlug, command.ApplicationSlug,
	)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "sign relay state")
	}

	return &BuildSPAuthnRequestResult{
		SAMLRequest: encodedRequest,
		RequestID:   requestID,
		SSOURL:      ssoURL,
		RelayState:  signed,
	}, nil
}

// encodeSPRelayState packs the request binding values into the signed payload.
// The original RelayState keeps its last position so a value containing the
// separator survives SplitN.
func encodeSPRelayState(requestID, spEntityID, spACSURL, relayState string) string {
	return strings.Join([]string{requestID, spEntityID, spACSURL, relayState}, spRelayStateSeparator)
}

// decodeSPRelayState reverses encodeSPRelayState. ok is false when any
// request-binding field is missing.
func decodeSPRelayState(payload string) (requestID, spEntityID, spACSURL, relayState string, ok bool) {
	parts := strings.SplitN(payload, spRelayStateSeparator, spRelayStateFieldCount)
	if len(parts) != spRelayStateFieldCount {
		return "", "", "", "", false
	}
	requestID = strings.TrimSpace(parts[0])
	spEntityID = strings.TrimSpace(parts[1])
	spACSURL = strings.TrimSpace(parts[2])
	relayState = parts[3]
	if requestID == "" || spEntityID == "" || spACSURL == "" {
		return "", "", "", "", false
	}
	return requestID, spEntityID, spACSURL, relayState, true
}

// HandleSPInitiatedACS processes the SAML Response from an SP-initiated flow.
// In an SP-initiated flow, the SP sends an AuthnRequest to our IdP, the user
// authenticates, and the IdP sends a SAML Response back to the SP's ACS URL.
// This method handles the case where the SP is acting as both the requester and
// the consumer of the SAML assertion.
func (s *Service) HandleSPInitiatedACS(ctx context.Context, command HandleSPInitiatedACSCommand) (*HandleSPInitiatedACSResult, error) {
	if strings.TrimSpace(command.SAMLResponse) == "" {
		return nil, apperrors.NewInvalidArgument("SAMLResponse is required")
	}

	// The SP verifies the RelayState it issued before trusting the assertion.
	// The signed envelope carries the AuthnRequest ID plus the SP entity ID and
	// ACS URL, so InResponseTo/Audience/Recipient are bound to the request we
	// actually issued instead of being left empty (which skips replay checks).
	if strings.TrimSpace(command.RelayState) == "" {
		return nil, apperrors.NewInvalidArgument("RelayState is required to bind the SAML response")
	}
	if s.RelayStateSigner == nil {
		return nil, apperrors.NewInternal("relay state signer not configured")
	}
	payload, err := s.RelayStateSigner.Verify(command.RelayState, command.OrganizationSlug, command.ApplicationSlug)
	if err != nil {
		return nil, apperrors.NewInvalidArgument("invalid RelayState")
	}
	requestID, spEntityID, spACSURL, _, ok := decodeSPRelayState(payload)
	if !ok {
		return nil, apperrors.NewInvalidArgument("invalid RelayState")
	}

	// Decode the SAML Response.
	data, err := base64.StdEncoding.DecodeString(command.SAMLResponse)
	if err != nil {
		data, err = base64.URLEncoding.DecodeString(command.SAMLResponse)
		if err != nil {
			return nil, apperrors.NewInvalidArgument("invalid base64 encoding in SAMLResponse")
		}
	}

	// Parse the SAML Response XML.
	var samlResp samlResponseXML
	if err := xml.Unmarshal(data, &samlResp); err != nil {
		return nil, apperrors.NewInvalidArgumentf("invalid SAML Response XML: %v", err)
	}

	// Resolve the organization.
	organization, err := s.UOW.Organizations().GetBySlug(ctx, command.OrganizationSlug)
	if err != nil {
		return nil, err
	}

	// Resolve the application. The IdP signing certificate is pinned in its
	// metadata; without it the response cannot be verified, so fail closed.
	application, err := s.UOW.Applications().GetByOrganizationAndSlug(ctx, organization.ID, command.ApplicationSlug)
	if err != nil {
		return nil, err
	}
	if application == nil || strings.TrimSpace(application.SamlMetadata) == "" {
		return nil, apperrors.NewUnauthorized("SAML IdP metadata is not configured for the application")
	}
	idpCert, err := samlSigningCertificate(application.SamlMetadata)
	if err != nil {
		return nil, apperrors.NewUnauthorized("SAML IdP signing certificate is not configured")
	}

	// ── S7: 五项 Assertion 校验（via domain port interface）──
	if s.SAMLValidator == nil {
		return nil, apperrors.NewInternal("SAMLValidator not configured")
	}
	assertionCfg := domainauth.SAMLAssertionConfig{
		ExpectedAudience:     spEntityID,
		ExpectedRecipient:    spACSURL,
		ExpectedDestination:  spACSURL,
		ExpectedInResponseTo: requestID,
		ExpectedIssuer:       extractIdPIssuerFromMetadata(application.SamlMetadata),
		ClockSkew:            5 * time.Minute,
		MaxAssertionAge:      15 * time.Minute,
		IdPSigningCert:       idpCert,
		RawAssertion:         data,
	}
	assertionInput := &domainauth.SAMLAssertionInput{
		ID:                  samlResp.Assertion.ID,
		Issuer:              samlResp.Assertion.Issuer,
		IssueInstant:        samlResp.Assertion.IssueInstant,
		NotBefore:           samlResp.Assertion.Conditions.NotBefore,
		NotOnOrAfter:        samlResp.Assertion.Conditions.NotOnOrAfter,
		Recipient:           samlResp.Assertion.Subject.SubjectConfirmation.Data.Recipient,
		InResponseTo:        samlResp.Assertion.Subject.SubjectConfirmation.Data.InResponseTo,
		SignaturePresent:    samlSignaturePresent(data),
		ResponseDestination: samlResp.Destination,
	}
	if len(samlResp.Assertion.Conditions.AudienceRestriction.Audience) > 0 {
		assertionInput.Audience = samlResp.Assertion.Conditions.AudienceRestriction.Audience[0]
	}
	if err := s.SAMLValidator.ValidateSAMLAssertion(assertionInput, assertionCfg); err != nil {
		audit.SlogAudit(ctx, audit.EventAuthLoginFailed,
			"saml.request_id", requestID,
			"saml.issuer", assertionInput.Issuer,
			"saml.reason", "assertion_validation_failed")
		return nil, apperrors.NewUnauthorizedf("SAML assertion validation failed: %s", err.Error())
	}

	// Extract the NameID and attributes from the assertion.
	nameID := samlResp.Assertion.Subject.NameID.Value
	attributes := extractSAMLAttributes(samlResp.Assertion.AttributeStatement)

	// Find the user by NameID (typically email).
	user, err := findUserByNameID(ctx, s.UOW, organization.ID, nameID)
	if err != nil {
		return nil, apperrors.NewUnauthorizedf("user not found for NameID %q", nameID)
	}

	return &HandleSPInitiatedACSResult{
		User:         user,
		Organization: organization.ID,
		Application:  application,
		NameID:       nameID,
		Attributes:   attributes,
		SessionIndex: samlResp.Assertion.AuthnStatement.SessionIndex,
	}, nil
}

// GenerateSPMetadata generates SP metadata for external IdPs to consume.
func (s *Service) GenerateSPMetadata(ctx context.Context, command SPInitiatedSAMLCommand) ([]byte, error) {
	if strings.TrimSpace(command.SPEntityID) == "" {
		return nil, apperrors.NewInvalidArgument("spEntityId is required")
	}
	if strings.TrimSpace(command.SPACSURL) == "" {
		return nil, apperrors.NewInvalidArgument("spAcsUrl is required")
	}

	now := s.Clock.Now().UTC()
	validUntil := now.Add(365 * 24 * time.Hour)

	var certSection string
	if strings.TrimSpace(command.SPEntityCert) != "" {
		certSection = fmt.Sprintf(`
    <md:KeyDescriptor use="signing">
      <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
        <ds:X509Data>
          <ds:X509Certificate>%s</ds:X509Certificate>
        </ds:X509Data>
      </ds:KeyInfo>
    </md:KeyDescriptor>`,
			strings.TrimSpace(command.SPEntityCert),
		)
	}

	metadata := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="%s" validUntil="%s">
  <md:SPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">%s
    <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
    <md:NameIDFormat>urn:oasis:names:tc:SAML:2.0:nameid-format:transient</md:NameIDFormat>
    <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="%s" index="0" isDefault="true"/>
    <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="%s" index="1"/>
  </md:SPSSODescriptor>
</md:EntityDescriptor>`,
		escapeXML(command.SPEntityID),
		validUntil.Format("2006-01-02T15:04:05Z"),
		certSection,
		escapeXML(command.SPACSURL),
		escapeXML(command.SPACSURL),
	)

	return []byte(metadata), nil
}

// samlResponseXML is the XML structure for parsing a SAML Response.
type samlResponseXML struct {
	XMLName      xml.Name `xml:"Response"`
	ID           string   `xml:"ID,attr"`
	InResponseTo string   `xml:"InResponseTo,attr,omitempty"`
	IssueInstant string   `xml:"IssueInstant,attr,omitempty"`
	Destination  string   `xml:"Destination,attr,omitempty"`
	Assertion    samlAssertionXML
}

type samlAssertionXML struct {
	XMLName            xml.Name `xml:"Assertion"`
	ID                 string   `xml:"ID,attr"`
	IssueInstant       string   `xml:"IssueInstant,attr,omitempty"`
	Issuer             string   `xml:"Issuer"`
	Subject            samlSubjectXML
	Conditions         samlConditionsXML
	AttributeStatement struct {
		XMLName    xml.Name `xml:"AttributeStatement"`
		Attributes []struct {
			Name   string `xml:"Name,attr"`
			Values []struct {
				Value string `xml:",chardata"`
			} `xml:"AttributeValue"`
		} `xml:"Attribute"`
	} `xml:"AttributeStatement"`
	AuthnStatement struct {
		XMLName      xml.Name `xml:"AuthnStatement"`
		SessionIndex string   `xml:"SessionIndex,attr,omitempty"`
	} `xml:"AuthnStatement"`
}

type samlSubjectXML struct {
	XMLName xml.Name `xml:"Subject"`
	NameID  struct {
		XMLName xml.Name `xml:"NameID"`
		Value   string   `xml:",chardata"`
		Format  string   `xml:"Format,attr,omitempty"`
	} `xml:"NameID"`
	SubjectConfirmation struct {
		XMLName xml.Name `xml:"SubjectConfirmation"`
		Method  string   `xml:"Method,attr,omitempty"`
		Data    struct {
			XMLName      xml.Name `xml:"SubjectConfirmationData"`
			Recipient    string   `xml:"Recipient,attr,omitempty"`
			InResponseTo string   `xml:"InResponseTo,attr,omitempty"`
			NotOnOrAfter string   `xml:"NotOnOrAfter,attr,omitempty"`
		} `xml:"SubjectConfirmationData"`
	} `xml:"SubjectConfirmation"`
}

type samlConditionsXML struct {
	XMLName             xml.Name `xml:"Conditions"`
	NotBefore           string   `xml:"NotBefore,attr,omitempty"`
	NotOnOrAfter        string   `xml:"NotOnOrAfter,attr,omitempty"`
	AudienceRestriction struct {
		XMLName  xml.Name `xml:"AudienceRestriction"`
		Audience []string `xml:"Audience"`
	} `xml:"AudienceRestriction"`
}

// extractSAMLAttributes extracts attributes from a SAML assertion.
func extractSAMLAttributes(attrStmt struct {
	XMLName    xml.Name `xml:"AttributeStatement"`
	Attributes []struct {
		Name   string `xml:"Name,attr"`
		Values []struct {
			Value string `xml:",chardata"`
		} `xml:"AttributeValue"`
	} `xml:"Attribute"`
}) map[string]string {
	attrs := make(map[string]string)
	for _, attr := range attrStmt.Attributes {
		if len(attr.Values) > 0 {
			attrs[attr.Name] = attr.Values[0].Value
		}
	}
	return attrs
}

// samlSigningCertificate extracts the first X.509 certificate from the
// IDPSSODescriptor KeyDescriptor list of a SAML metadata document.
func samlSigningCertificate(metadata string) (*x509.Certificate, error) {
	type metadataDocument struct {
		IDPSSO struct {
			KeyDescriptors []struct {
				KeyInfo struct {
					X509Data struct {
						Certificates []string `xml:"X509Certificate"`
					} `xml:"X509Data"`
				} `xml:"KeyInfo"`
			} `xml:"KeyDescriptor"`
		} `xml:"IDPSSODescriptor"`
	}

	var parsed metadataDocument
	if err := xml.Unmarshal([]byte(metadata), &parsed); err != nil {
		return nil, err
	}
	for _, descriptor := range parsed.IDPSSO.KeyDescriptors {
		for _, encoded := range descriptor.KeyInfo.X509Data.Certificates {
			der, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(encoded), ""))
			if err != nil {
				continue
			}
			cert, err := x509.ParseCertificate(der)
			if err == nil {
				return cert, nil
			}
		}
	}
	return nil, apperrors.NewUnauthorized("no signing certificate in SAML metadata")
}

// samlSignaturePresent reports whether the raw SAML document carries a
// ds:Signature element. This is a structural assertion only — infra/saml
// performs the actual cryptographic verification over the same bytes.
func samlSignaturePresent(raw []byte) bool {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	for {
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local == "Signature" &&
			(start.Name.Space == xmlDSigNamespace || start.Name.Space == "") {
			return true
		}
	}
}

// findUserByNameID finds a user by their NameID (typically email or username).
func findUserByNameID(ctx context.Context, uow appuow.AuthUoW, orgID, nameID string) (*userdomain.User, error) {
	// NameID resolution must scan the whole tenant: a single PageSize:1000 read
	// was clamped to 100 users, so any user outside the first page was
	// indistinguishable from a missing account.
	users, err := platformpagination.CollectAll(func(page, pageSize int) ([]*userdomain.User, int64, error) {
		return uow.Users().List(ctx, userdomain.ListQuery{OrganizationID: orgID, Page: page, PageSize: pageSize})
	})
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		if strings.EqualFold(user.Profile.Email, nameID) {
			return user, nil
		}
		if strings.EqualFold(user.Username, nameID) {
			return user, nil
		}
	}

	return nil, apperrors.NewNotFound("user not found")
}

// buildIdPSSOURL constructs the IdP SSO URL for a given application.
func buildIdPSSOURL(orgSlug string, app *applicationdomain.Application) string {
	if app != nil {
		return fmt.Sprintf("/api/v1/auth/saml/redirect/%s/%s", orgSlug, app.ID)
	}
	return fmt.Sprintf("/api/v1/auth/saml/redirect/%s", orgSlug)
}

// extractIdPIssuerFromMetadata parses a SAML entity descriptor XML and extracts
// the IdP entity ID — used to bind assertions to the expected IdP Issuer.
func extractIdPIssuerFromMetadata(metadata string) string {
	type ed struct {
		XMLName  xml.Name `xml:"EntityDescriptor"`
		EntityID string   `xml:"entityID,attr"`
		IDPSSO   struct {
			XMLName xml.Name `xml:"IDPSSODescriptor"`
		} `xml:"IDPSSODescriptor"`
	}
	var e ed
	if err := xml.Unmarshal([]byte(metadata), &e); err != nil {
		return ""
	}
	// Only trust entity descriptor that actually declares IDPSSODescriptor
	if e.EntityID != "" {
		return e.EntityID
	}
	return ""
}
