package saml

import (
	"context"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/beevik/etree"
)

// SAML 2.0 SP-side response consumption limits.
const (
	// defaultResponseMaxAge bounds how old an Assertion IssueInstant may be.
	// Assertions are consumed immediately after the IdP redirect; 15 minutes
	// leaves room for slow logins while keeping the replay window bounded.
	defaultResponseMaxAge = 15 * time.Minute
	// defaultReplayTTL covers the default assertion validity window plus skew.
	defaultReplayTTL = 10 * time.Minute
	// maxReplayTTL prevents a malformed far-future NotOnOrAfter from pinning
	// replay entries indefinitely.
	maxReplayTTL = 24 * time.Hour
)

// SAMLResponseValidationConfig carries every expectation the SP applies before
// it trusts an IdP assertion. Empty expectations are rejected: an SP that does
// not know its entity ID / ACS URL / AuthnRequest ID must not consume responses.
type SAMLResponseValidationConfig struct {
	// IdPCertPEM is the pinned IdP signing certificate (PEM).
	IdPCertPEM string
	// ExpectedAudience is our SP entity ID.
	ExpectedAudience string
	// ExpectedRecipient is our ACS URL (SubjectConfirmationData.Recipient).
	ExpectedRecipient string
	// ExpectedDestination is the URL the Response was delivered to.
	ExpectedDestination string
	// ExpectedIssuer is the IdP entity ID (optional; the pinned cert is the
	// primary trust anchor).
	ExpectedIssuer string
	// ExpectedInResponseTo is the ID of the AuthnRequest we issued.
	ExpectedInResponseTo string
	// MaxAge bounds the Assertion IssueInstant age (default 15m).
	MaxAge time.Duration
	// ClockSkew tolerates IdP/SP clock drift (default 5m).
	ClockSkew time.Duration
	// ReplayStore records consumed Response/Assertion IDs. Nil fails closed.
	ReplayStore ReplayStore
	// ReplayKeyPrefix namespaces replay keys with organization/provider scope.
	ReplayKeyPrefix string
	// ReplayTTL overrides the replay entry lifetime (default 10m).
	ReplayTTL time.Duration
	// Now overrides the clock in tests.
	Now func() time.Time
}

// SAMLResponseData is the verified content of a SAML Response.
type SAMLResponseData struct {
	ResponseID   string
	AssertionID  string
	InResponseTo string
	Issuer       string
	Destination  string
	Status       string
	NameID       string
	SessionIndex string
	// AuthnContextClassRef is the verified assertion's AuthnContextClassRef (acr
	// in OIDC terms). Empty when the IdP did not include one.
	AuthnContextClassRef string
	Attributes           map[string]string
}

// ValidateSAMLResponse runs the SAML 2.0 SP-side assertion checklist from
// design/auth/saml.md §15.3:
//
//  1. signature verification over the Response or the nested Assertion
//  2. signature presence (unsigned responses are rejected)
//  3. InResponseTo must equal the AuthnRequest ID we issued
//  4. Audience must contain our SP entity ID and Recipient must be our ACS URL
//  5. NotBefore / NotOnOrAfter window
//  6. Response/Assertion ID one-time consumption (replay cache)
//
// Every failure is returned as a fail-closed apperrors.Error with metadata that
// names the request ID / issuer / reason only — never assertion content.
func ValidateSAMLResponse(ctx context.Context, responseXML string, cfg SAMLResponseValidationConfig) (*SAMLResponseData, error) {
	if cfg.ClockSkew == 0 {
		cfg.ClockSkew = DefaultClockSkew
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = defaultResponseMaxAge
	}
	now := time.Now().UTC()
	if cfg.Now != nil {
		now = cfg.Now().UTC()
	}

	if err := requireResponseConfig(cfg); err != nil {
		return nil, err
	}

	_, root, err := parseSAMLResponseDocument(responseXML)
	if err != nil {
		return nil, err
	}
	cert, err := ParseIdPCertificate(cfg.IdPCertPEM)
	if err != nil {
		return nil, err
	}
	signedElement, responseSigned, err := verifySAMLResponseSignature(root, cert)
	if err != nil {
		return nil, decorateRejection(err, "", cfg.ExpectedInResponseTo, "signature_invalid")
	}

	response := root
	assertionElement := firstChild(signedElement, "Assertion")
	if responseSigned {
		response = signedElement
	} else {
		// Assertion-only signatures leave Response attributes unsigned; they
		// must not be used as trust bindings. The signed Assertion carries the
		// Audience/Recipient/InResponseTo values we depend on.
		assertionElement = signedElement
	}
	if assertionElement == nil {
		return nil, samlReject("assertion_missing", "", cfg.ExpectedInResponseTo, "SAML response has no Assertion")
	}

	status := samlStatusValue(root)
	if status != "Success" {
		return nil, samlReject("status_failure", "", cfg.ExpectedInResponseTo, "SAML response status is not Success")
	}

	assertion := parseAssertionFields(assertionElement)
	requestID := firstNonEmptyString(cfg.ExpectedInResponseTo, assertion.inResponseTo)

	if assertion.issuer == "" {
		return nil, samlReject("issuer_missing", "", requestID, "SAML assertion Issuer is empty")
	}
	if cfg.ExpectedIssuer != "" && assertion.issuer != cfg.ExpectedIssuer {
		return nil, samlReject("issuer_mismatch", assertion.issuer, requestID, "SAML assertion Issuer does not match the configured IdP")
	}
	if assertion.id == "" {
		return nil, samlReject("assertion_id_missing", assertion.issuer, requestID, "SAML assertion ID is empty")
	}

	// IssueInstant freshness.
	if assertion.issueInstant == "" {
		return nil, samlReject("issue_instant_missing", assertion.issuer, requestID, "SAML assertion IssueInstant is empty")
	}
	issuedAt, err := parseSAMLTime(assertion.issueInstant)
	if err != nil {
		return nil, samlReject("issue_instant_invalid", assertion.issuer, requestID, "SAML assertion IssueInstant is not RFC3339")
	}
	if issuedAt.Sub(now) > cfg.ClockSkew {
		return nil, samlReject("issue_instant_in_future", assertion.issuer, requestID, "SAML assertion IssueInstant is in the future")
	}
	if now.Sub(issuedAt) > cfg.MaxAge+cfg.ClockSkew {
		return nil, samlReject("issue_instant_too_old", assertion.issuer, requestID, "SAML assertion IssueInstant is too old")
	}

	// NotBefore.
	if assertion.notBefore != "" {
		notBefore, err := parseSAMLTime(assertion.notBefore)
		if err != nil {
			return nil, samlReject("not_before_invalid", assertion.issuer, requestID, "SAML assertion NotBefore is not RFC3339")
		}
		if now.Before(notBefore.Add(-cfg.ClockSkew)) {
			return nil, samlReject("assertion_not_yet_valid", assertion.issuer, requestID, "SAML assertion is not yet valid")
		}
	}

	// NotOnOrAfter: at least one of Conditions / SubjectConfirmationData must
	// carry a validity bound, and every present bound must still be open.
	if assertion.notOnOrAfter == "" && assertion.subjectNotOnOrAfter == "" {
		return nil, samlReject("not_on_or_after_missing", assertion.issuer, requestID, "SAML assertion has no NotOnOrAfter bound")
	}
	var expiresAt time.Time
	for _, raw := range []string{assertion.notOnOrAfter, assertion.subjectNotOnOrAfter} {
		if raw == "" {
			continue
		}
		notOnOrAfter, err := parseSAMLTime(raw)
		if err != nil {
			return nil, samlReject("not_on_or_after_invalid", assertion.issuer, requestID, "SAML assertion NotOnOrAfter is not RFC3339")
		}
		if now.After(notOnOrAfter.Add(cfg.ClockSkew)) {
			return nil, samlReject("assertion_expired", assertion.issuer, requestID, "SAML assertion is expired")
		}
		if notOnOrAfter.After(expiresAt) {
			expiresAt = notOnOrAfter
		}
	}

	// Audience.
	if !containsString(assertion.audiences, cfg.ExpectedAudience) {
		return nil, samlReject("audience_mismatch", assertion.issuer, requestID, "SAML assertion Audience does not contain our SP entity ID")
	}

	// Recipient.
	if assertion.recipient == "" {
		return nil, samlReject("recipient_missing", assertion.issuer, requestID, "SAML assertion SubjectConfirmationData.Recipient is empty")
	}
	if assertion.recipient != cfg.ExpectedRecipient {
		return nil, samlReject("recipient_mismatch", assertion.issuer, requestID, "SAML assertion Recipient does not match our ACS URL")
	}

	// Destination (top-level Response attribute). Assertion-only signatures
	// leave it unsigned, so it is enforced only when the Response is signed or
	// when a value is present.
	destination := response.SelectAttrValue("Destination", "")
	if responseSigned && destination == "" {
		return nil, samlReject("destination_missing", assertion.issuer, requestID, "SAML response Destination is empty")
	}
	if destination != "" && destination != cfg.ExpectedDestination {
		return nil, samlReject("destination_mismatch", assertion.issuer, requestID, "SAML response Destination does not match our ACS URL")
	}

	// InResponseTo: SubjectConfirmationData is inside the signed Assertion.
	if assertion.inResponseTo == "" {
		return nil, samlReject("in_response_to_missing", assertion.issuer, requestID, "SAML assertion InResponseTo is empty")
	}
	if assertion.inResponseTo != cfg.ExpectedInResponseTo {
		return nil, samlReject("in_response_to_mismatch", assertion.issuer, requestID, "SAML assertion InResponseTo does not match our AuthnRequest ID")
	}
	if responseSigned {
		responseInResponseTo := response.SelectAttrValue("InResponseTo", "")
		if responseInResponseTo != "" && responseInResponseTo != cfg.ExpectedInResponseTo {
			return nil, samlReject("in_response_to_mismatch", assertion.issuer, requestID, "SAML response InResponseTo does not match our AuthnRequest ID")
		}
	}

	// One-time consumption of the signed identifiers. Reservation is atomic and
	// happens after all structural checks so an invalid response cannot burn the
	// ID of a legitimate one.
	ttl := replayTTL(cfg, expiresAt, now)
	if err := reserveReplay(ctx, cfg, assertion.issuer, "assertion", assertion.id, ttl); err != nil {
		return nil, err
	}
	if responseSigned {
		if responseID := response.SelectAttrValue("ID", ""); responseID != "" {
			if err := reserveReplay(ctx, cfg, assertion.issuer, "response", responseID, ttl); err != nil {
				return nil, err
			}
		}
	}

	return &SAMLResponseData{
		ResponseID:           response.SelectAttrValue("ID", ""),
		AssertionID:          assertion.id,
		InResponseTo:         assertion.inResponseTo,
		Issuer:               assertion.issuer,
		Destination:          destination,
		Status:               status,
		NameID:               assertion.nameID,
		SessionIndex:         assertion.sessionIndex,
		AuthnContextClassRef: assertion.authnContext,
		Attributes:           assertion.attributes,
	}, nil
}

type samlAssertionFields struct {
	id                  string
	issueInstant        string
	issuer              string
	nameID              string
	sessionIndex        string
	authnContext        string
	attributes          map[string]string
	audiences           []string
	notBefore           string
	notOnOrAfter        string
	recipient           string
	inResponseTo        string
	subjectNotOnOrAfter string
}

func parseAssertionFields(assertion *etree.Element) samlAssertionFields {
	fields := samlAssertionFields{
		id:           assertion.SelectAttrValue("ID", ""),
		issueInstant: assertion.SelectAttrValue("IssueInstant", ""),
		issuer:       strings.TrimSpace(childText(assertion, "Issuer")),
		attributes:   map[string]string{},
	}

	if subject := firstChild(assertion, "Subject"); subject != nil {
		fields.nameID = strings.TrimSpace(childText(subject, "NameID"))
		if confirmation := bearerConfirmation(subject); confirmation != nil {
			if data := firstChild(confirmation, "SubjectConfirmationData"); data != nil {
				fields.recipient = data.SelectAttrValue("Recipient", "")
				fields.inResponseTo = data.SelectAttrValue("InResponseTo", "")
				fields.subjectNotOnOrAfter = data.SelectAttrValue("NotOnOrAfter", "")
			}
		}
	}

	if conditions := firstChild(assertion, "Conditions"); conditions != nil {
		fields.notBefore = conditions.SelectAttrValue("NotBefore", "")
		fields.notOnOrAfter = conditions.SelectAttrValue("NotOnOrAfter", "")
		for _, restriction := range conditions.ChildElements() {
			if restriction.Tag != "AudienceRestriction" {
				continue
			}
			for _, audience := range restriction.ChildElements() {
				if audience.Tag == "Audience" {
					value := strings.TrimSpace(audience.Text())
					if value != "" {
						fields.audiences = append(fields.audiences, value)
					}
				}
			}
		}
	}

	if authn := firstChild(assertion, "AuthnStatement"); authn != nil {
		fields.sessionIndex = authn.SelectAttrValue("SessionIndex", "")
		if authnContext := firstChild(authn, "AuthnContext"); authnContext != nil {
			if classRef := firstChild(authnContext, "AuthnContextClassRef"); classRef != nil {
				fields.authnContext = strings.TrimSpace(classRef.Text())
			}
		}
	}

	if attrStatement := firstChild(assertion, "AttributeStatement"); attrStatement != nil {
		for _, attr := range attrStatement.ChildElements() {
			if attr.Tag != "Attribute" {
				continue
			}
			name := strings.TrimSpace(attr.SelectAttrValue("Name", ""))
			if name == "" {
				continue
			}
			if value := firstChild(attr, "AttributeValue"); value != nil {
				fields.attributes[name] = strings.TrimSpace(value.Text())
			}
		}
	}

	return fields
}

// bearerConfirmation returns the bearer SubjectConfirmation (or the first one
// carrying SubjectConfirmationData).
func bearerConfirmation(subject *etree.Element) *etree.Element {
	var fallback *etree.Element
	for _, confirmation := range subject.ChildElements() {
		if confirmation.Tag != "SubjectConfirmation" {
			continue
		}
		if firstChild(confirmation, "SubjectConfirmationData") == nil {
			continue
		}
		if confirmation.SelectAttrValue("Method", "") == "urn:oasis:names:tc:SAML:2.0:cm:bearer" {
			return confirmation
		}
		if fallback == nil {
			fallback = confirmation
		}
	}
	return fallback
}

func childText(el *etree.Element, localName string) string {
	child := firstChild(el, localName)
	if child == nil {
		return ""
	}
	return child.Text()
}

func samlStatusValue(root *etree.Element) string {
	status := firstChild(root, "Status")
	if status == nil {
		return ""
	}
	code := firstDescendant(status, "StatusCode")
	if code == nil {
		return ""
	}
	value := strings.TrimSpace(code.SelectAttrValue("Value", ""))
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ":")
	return parts[len(parts)-1]
}

func parseSAMLTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func requireResponseConfig(cfg SAMLResponseValidationConfig) error {
	switch {
	case strings.TrimSpace(cfg.IdPCertPEM) == "":
		return samlConfigReject("IdP signing certificate is not configured")
	case strings.TrimSpace(cfg.ExpectedAudience) == "":
		return samlConfigReject("SP entity ID (Audience) is not configured")
	case strings.TrimSpace(cfg.ExpectedRecipient) == "":
		return samlConfigReject("ACS URL (Recipient) is not configured")
	case strings.TrimSpace(cfg.ExpectedDestination) == "":
		return samlConfigReject("ACS URL (Destination) is not configured")
	case strings.TrimSpace(cfg.ExpectedInResponseTo) == "":
		return samlConfigReject("expected InResponseTo (AuthnRequest ID) is not configured")
	case cfg.ReplayStore == nil:
		return samlConfigReject("SAML replay store is not configured")
	}
	return nil
}

func replayTTL(cfg SAMLResponseValidationConfig, expiresAt, now time.Time) time.Duration {
	ttl := cfg.ReplayTTL
	if ttl <= 0 {
		ttl = defaultReplayTTL
	}
	if !expiresAt.IsZero() {
		until := expiresAt.Add(cfg.ClockSkew).Sub(now)
		if until > ttl {
			ttl = until
		}
	}
	if ttl > maxReplayTTL {
		ttl = maxReplayTTL
	}
	return ttl
}

func reserveReplay(ctx context.Context, cfg SAMLResponseValidationConfig, issuer, kind, id string, ttl time.Duration) error {
	key := strings.Join([]string{strings.TrimSpace(cfg.ReplayKeyPrefix), kind, issuer, id}, "|")
	reserved, err := cfg.ReplayStore.Reserve(ctx, key, ttl)
	if err != nil {
		e := apperrors.WrapInternalf(err, "SAML replay store failure")
		e.Meta = map[string]any{"samlReason": "replay_store_error", "samlIssuer": issuer}
		return e
	}
	if !reserved {
		return samlReject("replay", issuer, cfg.ExpectedInResponseTo, "SAML response has already been consumed (replay)")
	}
	return nil
}

func samlReject(reason, issuer, requestID, message string) *apperrors.Error {
	e := apperrors.NewUnauthorized(message)
	e.Meta = map[string]any{"samlReason": reason}
	if issuer != "" {
		e.Meta["samlIssuer"] = issuer
	}
	if requestID != "" {
		e.Meta["samlRequestId"] = requestID
	}
	return e
}

func samlConfigReject(message string) *apperrors.Error {
	e := apperrors.NewInternal(message)
	e.Meta = map[string]any{"samlReason": "config_missing"}
	return e
}

// decorateRejection adds issuer/request metadata to an existing rejection
// without copying assertion content.
func decorateRejection(err error, issuer, requestID, reason string) error {
	e, ok := err.(*apperrors.Error)
	if !ok {
		e = apperrors.WrapUnauthorized(err, "SAML response rejected")
	}
	if e.Meta == nil {
		e.Meta = map[string]any{}
	}
	if _, exists := e.Meta["samlReason"]; !exists {
		e.Meta["samlReason"] = reason
	}
	if issuer != "" && e.Meta["samlIssuer"] == nil {
		e.Meta["samlIssuer"] = issuer
	}
	if requestID != "" && e.Meta["samlRequestId"] == nil {
		e.Meta["samlRequestId"] = requestID
	}
	return e
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
