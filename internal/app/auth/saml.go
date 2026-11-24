package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// SAMLRequestCommand contains parameters for initiating a SAML login.
type SAMLRequestCommand struct {
	OrganizationSlug string
	ApplicationSlug  string
	SAMLRequest      string // Base64-encoded SAML AuthnRequest
	RelayState       string
}

// SAMLRequestResult contains the result of a SAML login initiation.
type SAMLRequestResult struct {
	RedirectURL string `json:"redirectUrl,omitempty"`
	SAMLRequest string `json:"samlRequest,omitempty"`
	RelayState  string `json:"relayState,omitempty"`
}

// SAMLResponseCommand contains parameters for generating a SAML response.
type SAMLResponseCommand struct {
	OrganizationSlug string
	Username         string
	ApplicationID    string
	SAMLRequest      string // Base64-encoded SAML AuthnRequest
	RelayState       string
}

// SAMLResponseResult contains the generated SAML response.
type SAMLResponseResult struct {
	SAMLResponse string `json:"samlResponse"`
	RelayState   string `json:"relayState,omitempty"`
	RedirectURL  string `json:"redirectUrl,omitempty"`
	Method       string `json:"method,omitempty"` // "GET" or "POST"
}

// HandleSAMLRedirect processes an incoming SAML AuthnRequest (SP-initiated SSO).
// It decodes the request and redirects to the frontend login page.
func (s *Service) HandleSAMLRedirect(ctx context.Context, command SAMLRequestCommand) (*SAMLRequestResult, error) {
	if strings.TrimSpace(command.SAMLRequest) == "" {
		return nil, apperrors.NewInvalidArgument("SAMLRequest is required")
	}

	if _, err := s.ParseSamlRequest(ctx, command.SAMLRequest); err != nil {
		return nil, apperrors.NewInvalidArgument("invalid SAML AuthnRequest")
	}

	// IdP role: RelayState belongs to the external SP and MUST be echoed
	// unchanged (SAML 2.0 §3.4). Signing or verifying here would break external
	// SP correlation. Backend signing only happens on the SP issuance path
	// (BuildSPAuthnRequest), where we generate the state ourselves.
	return &SAMLRequestResult{
		SAMLRequest: command.SAMLRequest,
		RelayState:  command.RelayState,
	}, nil
}

// GenerateSAMLResponse generates a signed SAML Response after user
// authentication. The AssertionConsumerServiceURL requested by the SP is only
// accepted when it matches the application's registered redirect URIs; the
// response is signed with the application's certificate and fails closed when
// no signing key is configured.
func (s *Service) GenerateSAMLResponse(ctx context.Context, command SAMLResponseCommand) (*SAMLResponseResult, error) {
	if strings.TrimSpace(command.SAMLRequest) == "" {
		return nil, apperrors.NewInvalidArgument("SAMLRequest is required")
	}
	if strings.TrimSpace(command.Username) == "" {
		return nil, apperrors.NewInvalidArgument("username is required")
	}

	organization, err := s.UOW.Organizations().GetBySlug(ctx, command.OrganizationSlug)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "lookup organization by slug %q", command.OrganizationSlug)
	}

	user, err := s.UOW.Users().GetByUsername(ctx, organization.ID, command.Username)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "lookup user %q in organization %s", command.Username, organization.ID)
	}

	application, err := resolveApplicationReference(ctx, s.UOW.Applications(), organization, command.ApplicationID)
	if err != nil {
		return nil, err
	}

	requestInfo, err := s.ParseSamlRequest(ctx, command.SAMLRequest)
	if err != nil {
		return nil, apperrors.NewInvalidArgument("invalid SAML AuthnRequest")
	}
	if requestInfo.ID == "" {
		return nil, apperrors.NewInvalidArgument("SAML AuthnRequest has no ID")
	}

	acsURL, err := resolveSAMLACSURL(requestInfo.ACSURL, application)
	if err != nil {
		if apperrors.FromError(err).Code == apperrors.CodeForbidden {
			s.recordSAMLResponseRejection(ctx, organization.ID, user.ID, application.ID, "acs_url_not_registered")
		}
		return nil, err
	}

	signedXML, err := s.GenerateSamlResponse(ctx, application, user, requestInfo.ID, acsURL)
	if err != nil {
		return nil, err
	}

	// Single base64 encoding point: the transport posts this value verbatim.
	return &SAMLResponseResult{
		SAMLResponse: base64.StdEncoding.EncodeToString([]byte(signedXML)),
		RelayState:   command.RelayState,
		RedirectURL:  acsURL,
		Method:       "POST",
	}, nil
}

// resolveSAMLACSURL applies the redirect whitelist to the SP-requested ACS URL.
// An empty request falls back to the first registered redirect URI; a non-empty
// request must match the registered set exactly (applications.MatchRedirectURI),
// so a signed assertion can never be delivered to an attacker-chosen endpoint.
func resolveSAMLACSURL(requested string, application *applicationdomain.Application) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		if len(application.RedirectURIs) == 0 {
			return "", apperrors.NewInvalidArgument("no Assertion Consumer Service URL registered for application")
		}
		return application.RedirectURIs[0], nil
	}
	if !applicationdomain.MatchRedirectURI(requested, application.RedirectURIs) {
		return "", apperrors.NewForbidden("AssertionConsumerServiceURL is not a registered redirect URI")
	}
	return requested, nil
}

// recordSAMLResponseRejection appends a best-effort audit row for a refused
// assertion destination. The rejection itself must still reach the caller, so
// an audit failure is logged, not returned.
func (s *Service) recordSAMLResponseRejection(ctx context.Context, organizationID, userID, applicationID, reason string) {
	if s.UOW == nil || s.UOW.Records() == nil {
		return
	}
	id, err := s.UUIDGenerator.NewString()
	if err != nil {
		slog.WarnContext(ctx, "saml rejection audit id generation failed", "error", err)
		return
	}
	record := &domainrecords.Record{
		ID:             id,
		OrganizationID: organizationID,
		ActorID:        userID,
		Action:         "protocol.saml.response_rejected",
		ResourceType:   "application",
		ResourceID:     applicationID,
		Surface:        domainrecords.SurfaceProtocol,
		Metadata:       map[string]any{"reason": reason},
		CreatedAt:      s.Clock.Now().UTC(),
	}
	if err := s.UOW.Records().Create(ctx, record); err != nil {
		slog.WarnContext(ctx, "saml rejection audit write failed", "error", err)
	}
}

// GetSAMLMetadata generates the IdP metadata XML for a given application.
func (s *Service) GetSAMLMetadata(ctx context.Context, organizationSlug, applicationID string) ([]byte, error) {
	organization, err := s.UOW.Organizations().GetBySlug(ctx, organizationSlug)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "lookup organization by slug %q for SAML metadata", organizationSlug)
	}

	application, err := resolveApplicationReference(ctx, s.UOW.Applications(), organization, applicationID)
	if err != nil {
		return nil, err
	}

	if application.OrganizationID != organization.ID {
		return nil, apperrors.NewInvalidArgument("application does not belong to organization")
	}

	cert := s.resolveApplicationCert(ctx, application)
	metadata, err := buildSamlMetadataEtree(application, cert, s.IssuerURL, s.Clock.Now().UTC())
	if err != nil {
		return nil, err
	}
	return []byte(metadata), nil
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// ─────────────────────────────────────────────
// SAML Single Logout (SLO)
// ─────────────────────────────────────────────

// SAMLSLOCommand contains parameters for SAML Single Logout.
type SAMLSLOCommand struct {
	ApplicationID    string
	OrganizationSlug string
	Username         string
	SessionIndex     string
}

// SAMLSLOResult contains the result of a SAML Single Logout request.
type SAMLSLOResult struct {
	LogoutRequest string `json:"logoutRequest,omitempty"`
	RedirectURL   string `json:"redirectUrl,omitempty"`
}

// HandleSPInitiatedSLO handles SP-initiated SAML Single Logout.
// It generates a SAML LogoutRequest and returns the IdP SLO redirect URL.
func (s *Service) HandleSPInitiatedSLO(ctx context.Context, command SAMLSLOCommand) (*SAMLSLOResult, error) {
	var org *orgdomain.Organization
	if strings.TrimSpace(command.OrganizationSlug) != "" {
		resolved, err := s.UOW.Organizations().GetBySlug(ctx, command.OrganizationSlug)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "lookup organization by slug %q for SAML SLO", command.OrganizationSlug)
		}
		org = resolved
	}
	app, err := resolveApplicationReference(ctx, s.UOW.Applications(), org, command.ApplicationID)
	if err != nil {
		return nil, err
	}

	// Check if the application has SAML SLO capability.
	// SLO URL is derived from the SAML metadata URL if not explicitly configured.
	sloURL := app.SamlMetadataUrl
	if sloURL == "" {
		return nil, apperrors.NewInvalidArgument("application does not support SAML Single Logout")
	}

	logoutRequestID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return nil, apperrors.WrapInternal(err, "generate SAML logout request id")
	}
	now := s.Clock.Now().UTC().Format(time.RFC3339)

	logoutRequestXML := fmt.Sprintf(`<samlp:LogoutRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="%s" Version="2.0" IssueInstant="%s" Destination="%s">
<saml:Issuer>guliangiam</saml:Issuer>
<saml:NameID Format="urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified">%s</saml:NameID>
<samlp:SessionIndex>%s</samlp:SessionIndex>
</samlp:LogoutRequest>`,
		escapeXML(logoutRequestID),
		now,
		escapeXML(sloURL),
		escapeXML(command.Username),
		escapeXML(command.SessionIndex),
	)

	encodedRequest := base64.StdEncoding.EncodeToString([]byte(logoutRequestXML))

	redirectURL := sloURL
	if strings.Contains(redirectURL, "?") {
		redirectURL += "&SAMLRequest=" + url.QueryEscape(encodedRequest)
	} else {
		redirectURL += "?SAMLRequest=" + url.QueryEscape(encodedRequest)
	}

	return &SAMLSLOResult{
		LogoutRequest: encodedRequest,
		RedirectURL:   redirectURL,
	}, nil
}
