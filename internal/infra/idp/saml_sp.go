package idp

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/beevik/etree"

	"github.com/guliangiam/guliangiam/internal/infra/saml"
)

// SAMLIdPProvider implements a SAML Service Provider (SP) connector.
// It allows GuliangIAM to authenticate users against an external SAML IdP.
// The flow is:
//  1. GetRedirectURL generates a SAML AuthnRequest and redirects to the IdP SSO URL
//  2. ExchangeCode verifies the signed SAML Response (§15.3 checklist) and
//     enforces audience/recipient/validity/InResponseTo plus one-time Response IDs
//  3. GetUserInfo extracts user attributes from the SAML Response
type SAMLIdPProvider struct {
	BaseOAuthProvider
	ssoURL    string // IdP Single Sign-On URL
	issuer    string // SP Entity ID (our entity ID)
	idpCert   string // IdP signing certificate (PEM, for response verification)
	nameIDFmt string // NameID format to request

	expectedAudience     string
	expectedRecipient    string
	expectedDestination  string
	expectedIssuer       string
	expectedInResponseTo string
	replayStore          saml.ReplayStore
	replayKeyPrefix      string
	maxAge               time.Duration
	clockSkew            time.Duration
}

// NewSAMLIdPProvider creates a new SAML SP connector without per-callback
// context. ExchangeCode fails closed because no AuthnRequest ID is bound.
func NewSAMLIdPProvider(config *OAuthConfig) *SAMLIdPProvider {
	return NewSAMLIdPProviderWithOptions(config, ProviderOptions{})
}

// NewSAMLIdPProviderWithOptions creates a SAML SP connector with the
// SP-initiated callback context (AuthnRequest ID + replay store).
func NewSAMLIdPProviderWithOptions(config *OAuthConfig, options ProviderOptions) *SAMLIdPProvider {
	expectedInResponseTo := ""
	if state := strings.TrimSpace(options.SAMLState); state != "" {
		// buildAuthnRequest sets ID="_"+state, so the echoed RelayState binds
		// the response to the request we actually issued.
		expectedInResponseTo = "_" + state
	}
	replayKeyPrefix := strings.Join([]string{
		"saml-response",
		strings.TrimSpace(options.SAMLReplayOrgID),
		strings.TrimSpace(options.SAMLReplayProviderID),
	}, "|")

	return &SAMLIdPProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			config.Host, // SSO URL
			"",          // No token URL for SAML
			"",          // No user info URL for SAML
		),
		ssoURL:               config.Host,
		issuer:               config.ClientID,
		idpCert:              firstNonEmpty(config.IdPCert, config.ClientSecret),
		nameIDFmt:            "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
		expectedAudience:     firstNonEmpty(config.SAMLExpectedAudience, config.ClientID),
		expectedRecipient:    firstNonEmpty(config.SAMLExpectedRecipient, config.RedirectURI),
		expectedDestination:  firstNonEmpty(config.SAMLExpectedDestination, config.SAMLExpectedRecipient, config.RedirectURI),
		expectedIssuer:       strings.TrimSpace(config.IdPIssuer),
		expectedInResponseTo: expectedInResponseTo,
		replayStore:          options.SAMLReplayStore,
		replayKeyPrefix:      replayKeyPrefix,
		maxAge:               15 * time.Minute,
		clockSkew:            saml.DefaultClockSkew,
	}
}

// GetRedirectURL generates a SAML AuthnRequest and returns the redirect URL to the IdP.
func (p *SAMLIdPProvider) GetRedirectURL(state string) (string, error) {
	state = strings.TrimSpace(state)
	if state == "" {
		return "", apperrors.NewInvalidArgument("SAML state is required to bind the AuthnRequest ID")
	}
	if strings.TrimSpace(p.ssoURL) == "" {
		return "", apperrors.NewInvalidArgument("SAML IdP SSO URL is not configured")
	}
	if strings.TrimSpace(p.issuer) == "" {
		return "", apperrors.NewInvalidArgument("SAML SP entity ID is not configured")
	}
	if strings.TrimSpace(p.config.RedirectURI) == "" {
		return "", apperrors.NewInvalidArgument("SAML ACS URL is not configured")
	}

	authnRequest := p.buildAuthnRequest(state)
	requestXML, err := authnRequest.WriteToString()
	if err != nil {
		return "", apperrors.WrapInternalf(err, "serialize authn request")
	}

	// Base64 encode the AuthnRequest
	encoded := base64.StdEncoding.EncodeToString([]byte(requestXML))

	// Build the redirect URL with SAMLRequest parameter
	u, err := url.Parse(p.ssoURL)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "parse SSO URL")
	}

	q := u.Query()
	q.Set("SAMLRequest", encoded)
	q.Set("RelayState", state)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// buildAuthnRequest constructs a SAML AuthnRequest XML element.
func (p *SAMLIdPProvider) buildAuthnRequest(requestID string) *etree.Document {
	doc := etree.NewDocument()
	doc.CreateProcInst("xml", `version="1.0" encoding="UTF-8"`)

	authnRequest := doc.CreateElement("samlp:AuthnRequest")
	authnRequest.CreateAttr("xmlns:samlp", "urn:oasis:names:tc:SAML:2.0:protocol")
	authnRequest.CreateAttr("xmlns:saml", "urn:oasis:names:tc:SAML:2.0:assertion")
	authnRequest.CreateAttr("ID", "_"+requestID)
	authnRequest.CreateAttr("Version", "2.0")
	authnRequest.CreateAttr("IssueInstant", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	authnRequest.CreateAttr("Destination", p.ssoURL)
	authnRequest.CreateAttr("ProtocolBinding", "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST")
	authnRequest.CreateAttr("AssertionConsumerServiceURL", p.config.RedirectURI)

	// Issuer
	issuer := authnRequest.CreateElement("saml:Issuer")
	issuer.SetText(p.issuer)

	// NameIDPolicy
	nameIDPolicy := authnRequest.CreateElement("samlp:NameIDPolicy")
	nameIDPolicy.CreateAttr("Format", p.nameIDFmt)
	nameIDPolicy.CreateAttr("AllowCreate", "true")

	// RequestedAuthnContext
	reqAuthn := authnRequest.CreateElement("samlp:RequestedAuthnContext")
	reqAuthn.CreateAttr("Comparison", "exact")
	authnClass := reqAuthn.CreateElement("saml:AuthnContextClassRef")
	authnClass.SetText("urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport")

	return doc
}

// samlResponse holds the verified SAML Response data.
type samlResponse struct {
	ID           string
	AssertionID  string
	InResponseTo string
	Issuer       string
	NameID       string
	SessionIndex string
	AuthnContext string
	Attributes   map[string]string
	Status       string
}

// ExchangeCode processes the SAML Response from the IdP.
// In SAML, the "code" is the base64-encoded SAML Response.
func (p *SAMLIdPProvider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	// The "code" is the base64-encoded SAML Response
	decoded, err := base64.StdEncoding.DecodeString(code)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decode SAML response")
	}

	// The validator returns apperrors with samlReason/samlIssuer/samlRequestId
	// metadata. Return it unwrapped so the application layer can audit the
	// failure without losing the reason.
	resp, err := p.parseSAMLResponse(ctx, string(decoded))
	if err != nil {
		return nil, err
	}

	// Store NameID and attributes in the token for GetUserInfo to use
	// We encode attributes as key=value pairs in the Scope field
	scopeParts := []string{"nameid=" + resp.NameID}
	for k, v := range resp.Attributes {
		scopeParts = append(scopeParts, fmt.Sprintf("%s=%s", k, v))
	}

	return &OAuthToken{
		AccessToken:  resp.NameID,
		Scope:        strings.Join(scopeParts, "|"),
		AuthnContext: resp.AuthnContext,
	}, nil
}

// GetUserInfo extracts user information from the SAML Response data
// that was parsed during ExchangeCode.
func (p *SAMLIdPProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	// Parse attributes from the Scope field
	attrs := make(map[string]string)
	parts := strings.Split(token.Scope, "|")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			attrs[kv[0]] = kv[1]
		}
	}

	info := &UserInfo{
		ID:           token.AccessToken, // NameID
		AuthnContext: token.AuthnContext,
	}

	// Extract common SAML attributes
	if email, ok := attrs["email"]; ok {
		info.Email = email
	}
	if name, ok := attrs["name"]; ok {
		info.DisplayName = name
	}
	if displayName, ok := attrs["displayName"]; ok {
		info.DisplayName = displayName
	}
	if firstName, ok := attrs["firstName"]; ok {
		if lastName, ok := attrs["lastName"]; ok {
			info.DisplayName = firstName + " " + lastName
		} else {
			info.DisplayName = firstName
		}
	}
	if username, ok := attrs["username"]; ok {
		info.Username = username
	}
	if info.Username == "" && info.DisplayName != "" {
		info.Username = info.DisplayName
	}

	return info, nil
}

// parseSAMLResponse verifies and validates a SAML Response XML string.
func (p *SAMLIdPProvider) parseSAMLResponse(ctx context.Context, xmlStr string) (*samlResponse, error) {
	data, err := saml.ValidateSAMLResponse(ctx, xmlStr, saml.SAMLResponseValidationConfig{
		IdPCertPEM:           p.idpCert,
		ExpectedAudience:     p.expectedAudience,
		ExpectedRecipient:    p.expectedRecipient,
		ExpectedDestination:  p.expectedDestination,
		ExpectedIssuer:       p.expectedIssuer,
		ExpectedInResponseTo: p.expectedInResponseTo,
		MaxAge:               p.maxAge,
		ClockSkew:            p.clockSkew,
		ReplayStore:          p.replayStore,
		ReplayKeyPrefix:      p.replayKeyPrefix,
	})
	if err != nil {
		return nil, err
	}

	return &samlResponse{
		ID:           data.ResponseID,
		AssertionID:  data.AssertionID,
		InResponseTo: data.InResponseTo,
		Issuer:       data.Issuer,
		NameID:       data.NameID,
		SessionIndex: data.SessionIndex,
		AuthnContext: data.AuthnContextClassRef,
		Attributes:   data.Attributes,
		Status:       data.Status,
	}, nil
}

// Ensure SAMLIdPProvider implements Provider interface
var _ Provider = (*SAMLIdPProvider)(nil)
