package idp

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"

	"github.com/guliangiam/guliangiam/internal/infra/saml"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// samlSignerFixture provides a throwaway IdP key/cert pair for signature tests.
type samlSignerFixture struct {
	key     *rsa.PrivateKey
	cert    *x509.Certificate
	certPEM string
}

func newSAMLSignerFixture(t *testing.T) *samlSignerFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test idp"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return &samlSignerFixture{
		key:     key,
		cert:    cert,
		certPEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
	}
}

type samlResponseOptions struct {
	responseID   string
	assertionID  string
	issuer       string
	audience     string
	recipient    string
	destination  string
	inResponseTo string
	nameID       string
	email        string
	notBefore    time.Time
	notOnOrAfter time.Time
	authnContext string
	sign         bool
}

func defaultSAMLResponseOptions() samlResponseOptions {
	now := time.Now().UTC()
	return samlResponseOptions{
		responseID:   "_resp-1",
		assertionID:  "_assert-1",
		issuer:       "https://idp.example.com",
		audience:     "https://sp.example.com",
		recipient:    "https://sp.example.com/acs",
		destination:  "https://sp.example.com/acs",
		inResponseTo: "_state-1",
		nameID:       "user@example.com",
		email:        "user@example.com",
		notBefore:    now.Add(-time.Minute),
		notOnOrAfter: now.Add(5 * time.Minute),
		sign:         true,
	}
}

// buildSAMLResponseXML returns a SAML 2.0 Response carrying one bearer assertion.
// When opts.sign is true the whole Response element is enveloped-signed.
func buildSAMLResponseXML(t *testing.T, fixture *samlSignerFixture, opts samlResponseOptions) string {
	t.Helper()
	now := time.Now().UTC()

	root := &etree.Element{Space: "samlp", Tag: "Response"}
	root.CreateAttr("xmlns:samlp", "urn:oasis:names:tc:SAML:2.0:protocol")
	root.CreateAttr("xmlns:saml", "urn:oasis:names:tc:SAML:2.0:assertion")
	root.CreateAttr("ID", opts.responseID)
	root.CreateAttr("Version", "2.0")
	root.CreateAttr("IssueInstant", now.Format(time.RFC3339))
	root.CreateAttr("Destination", opts.destination)
	if opts.inResponseTo != "" {
		root.CreateAttr("InResponseTo", opts.inResponseTo)
	}

	root.CreateElement("saml:Issuer").SetText(opts.issuer)
	status := root.CreateElement("samlp:Status")
	status.CreateElement("samlp:StatusCode").CreateAttr("Value", "urn:oasis:names:tc:SAML:2.0:status:Success")

	assertion := root.CreateElement("saml:Assertion")
	assertion.CreateAttr("ID", opts.assertionID)
	assertion.CreateAttr("Version", "2.0")
	assertion.CreateAttr("IssueInstant", now.Format(time.RFC3339))
	assertion.CreateElement("saml:Issuer").SetText(opts.issuer)

	subject := assertion.CreateElement("saml:Subject")
	subject.CreateElement("saml:NameID").SetText(opts.nameID)
	confirmation := subject.CreateElement("saml:SubjectConfirmation")
	confirmation.CreateAttr("Method", "urn:oasis:names:tc:SAML:2.0:cm:bearer")
	data := confirmation.CreateElement("saml:SubjectConfirmationData")
	data.CreateAttr("Recipient", opts.recipient)
	data.CreateAttr("NotOnOrAfter", opts.notOnOrAfter.Format(time.RFC3339))
	if opts.inResponseTo != "" {
		data.CreateAttr("InResponseTo", opts.inResponseTo)
	}

	conditions := assertion.CreateElement("saml:Conditions")
	conditions.CreateAttr("NotBefore", opts.notBefore.Format(time.RFC3339))
	conditions.CreateAttr("NotOnOrAfter", opts.notOnOrAfter.Format(time.RFC3339))
	restriction := conditions.CreateElement("saml:AudienceRestriction")
	restriction.CreateElement("saml:Audience").SetText(opts.audience)

	authn := assertion.CreateElement("saml:AuthnStatement")
	authn.CreateAttr("AuthnInstant", now.Format(time.RFC3339))
	authn.CreateAttr("SessionIndex", "_session-1")
	if opts.authnContext != "" {
		authnContext := authn.CreateElement("saml:AuthnContext")
		authnContext.CreateElement("saml:AuthnContextClassRef").SetText(opts.authnContext)
	}

	attrStatement := assertion.CreateElement("saml:AttributeStatement")
	emailAttr := attrStatement.CreateElement("saml:Attribute")
	emailAttr.CreateAttr("Name", "email")
	emailAttr.CreateElement("saml:AttributeValue").SetText(opts.email)

	if opts.sign {
		signingContext, err := dsig.NewSigningContext(fixture.key, [][]byte{fixture.cert.Raw})
		if err != nil {
			t.Fatalf("create signing context: %v", err)
		}
		signingContext.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")
		signingContext.Hash = crypto.SHA256
		signed, err := signingContext.SignEnveloped(root)
		if err != nil {
			t.Fatalf("sign response: %v", err)
		}
		root = signed
	}

	doc := etree.NewDocument()
	doc.SetRoot(root)
	out, err := doc.WriteToString()
	if err != nil {
		t.Fatalf("serialize response: %v", err)
	}
	return out
}

func newTestSAMLProvider(fixture *samlSignerFixture, replayStore saml.ReplayStore) *SAMLIdPProvider {
	return NewSAMLIdPProviderWithOptions(&OAuthConfig{
		ClientID:    "https://sp.example.com",
		RedirectURI: "https://sp.example.com/acs",
		Host:        "https://idp.example.com/sso",
		IdPCert:     fixture.certPEM,
		IdPIssuer:   "https://idp.example.com",
	}, ProviderOptions{
		SAMLState:            "state-1",
		SAMLReplayStore:      replayStore,
		SAMLReplayOrgID:      "org-1",
		SAMLReplayProviderID: "provider-1",
	})
}

func exchangeSAMLResponse(provider *SAMLIdPProvider, signedXML string) error {
	_, err := provider.ExchangeCode(context.Background(), base64.StdEncoding.EncodeToString([]byte(signedXML)))
	return err
}

func requireSAMLReason(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected rejection reason %q, got nil error", want)
	}
	appErr, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("expected *apperrors.Error, got %T", err)
	}
	if got := appErr.Meta["samlReason"]; got != want {
		t.Fatalf("rejection reason = %v, want %q (message: %s)", got, want, appErr.Message)
	}
}

func TestSAMLProvider_AcceptsValidResponse(t *testing.T) {
	fixture := newSAMLSignerFixture(t)
	provider := newTestSAMLProvider(fixture, saml.NewMemoryReplayStore())

	if err := exchangeSAMLResponse(provider, buildSAMLResponseXML(t, fixture, defaultSAMLResponseOptions())); err != nil {
		t.Fatalf("valid response rejected: %v", err)
	}
}

func TestSAMLProvider_ExtractsAuthnContext(t *testing.T) {
	fixture := newSAMLSignerFixture(t)
	provider := newTestSAMLProvider(fixture, saml.NewMemoryReplayStore())

	opts := defaultSAMLResponseOptions()
	opts.authnContext = "urn:oasis:names:tc:SAML:2.0:ac:classes:MobileTwoFactorContract"
	signed := buildSAMLResponseXML(t, fixture, opts)
	token, err := provider.ExchangeCode(context.Background(), base64.StdEncoding.EncodeToString([]byte(signed)))
	if err != nil {
		t.Fatalf("valid response rejected: %v", err)
	}
	info, err := provider.GetUserInfo(context.Background(), token)
	if err != nil {
		t.Fatalf("GetUserInfo() error = %v", err)
	}
	if info.AuthnContext != opts.authnContext {
		t.Fatalf("AuthnContext = %q, want %q", info.AuthnContext, opts.authnContext)
	}
}

func TestSAMLProvider_RejectsTamperedSignedResponse(t *testing.T) {
	fixture := newSAMLSignerFixture(t)
	provider := newTestSAMLProvider(fixture, saml.NewMemoryReplayStore())

	signed := buildSAMLResponseXML(t, fixture, defaultSAMLResponseOptions())
	tampered := strings.ReplaceAll(signed, "user@example.com", "attacker@example.com")
	if tampered == signed {
		t.Fatal("tamper mutation did not change the response")
	}
	requireSAMLReason(t, exchangeSAMLResponse(provider, tampered), "signature_invalid")
}

func TestSAMLProvider_RejectsUnsignedResponse(t *testing.T) {
	fixture := newSAMLSignerFixture(t)
	provider := newTestSAMLProvider(fixture, saml.NewMemoryReplayStore())

	opts := defaultSAMLResponseOptions()
	opts.sign = false
	requireSAMLReason(t, exchangeSAMLResponse(provider, buildSAMLResponseXML(t, fixture, opts)), "signature_invalid")
}

func TestSAMLProvider_RejectsWrongAudience(t *testing.T) {
	fixture := newSAMLSignerFixture(t)
	provider := newTestSAMLProvider(fixture, saml.NewMemoryReplayStore())

	opts := defaultSAMLResponseOptions()
	opts.audience = "https://other-sp.example.com"
	requireSAMLReason(t, exchangeSAMLResponse(provider, buildSAMLResponseXML(t, fixture, opts)), "audience_mismatch")
}

func TestSAMLProvider_RejectsWrongRecipient(t *testing.T) {
	fixture := newSAMLSignerFixture(t)
	provider := newTestSAMLProvider(fixture, saml.NewMemoryReplayStore())

	opts := defaultSAMLResponseOptions()
	opts.recipient = "https://other-sp.example.com/acs"
	requireSAMLReason(t, exchangeSAMLResponse(provider, buildSAMLResponseXML(t, fixture, opts)), "recipient_mismatch")
}

func TestSAMLProvider_RejectsWrongDestination(t *testing.T) {
	fixture := newSAMLSignerFixture(t)
	provider := newTestSAMLProvider(fixture, saml.NewMemoryReplayStore())

	opts := defaultSAMLResponseOptions()
	opts.destination = "https://other-sp.example.com/acs"
	requireSAMLReason(t, exchangeSAMLResponse(provider, buildSAMLResponseXML(t, fixture, opts)), "destination_mismatch")
}

func TestSAMLProvider_RejectsExpiredAssertion(t *testing.T) {
	fixture := newSAMLSignerFixture(t)
	provider := newTestSAMLProvider(fixture, saml.NewMemoryReplayStore())

	now := time.Now().UTC()
	opts := defaultSAMLResponseOptions()
	opts.notBefore = now.Add(-30 * time.Minute)
	opts.notOnOrAfter = now.Add(-20 * time.Minute)
	requireSAMLReason(t, exchangeSAMLResponse(provider, buildSAMLResponseXML(t, fixture, opts)), "assertion_expired")
}

func TestSAMLProvider_RejectsFutureAssertion(t *testing.T) {
	fixture := newSAMLSignerFixture(t)
	provider := newTestSAMLProvider(fixture, saml.NewMemoryReplayStore())

	now := time.Now().UTC()
	opts := defaultSAMLResponseOptions()
	opts.notBefore = now.Add(10 * time.Minute)
	opts.notOnOrAfter = now.Add(20 * time.Minute)
	requireSAMLReason(t, exchangeSAMLResponse(provider, buildSAMLResponseXML(t, fixture, opts)), "assertion_not_yet_valid")
}

func TestSAMLProvider_RejectsReplayedResponse(t *testing.T) {
	fixture := newSAMLSignerFixture(t)
	replayStore := saml.NewMemoryReplayStore()
	provider := newTestSAMLProvider(fixture, replayStore)

	signed := buildSAMLResponseXML(t, fixture, defaultSAMLResponseOptions())
	if err := exchangeSAMLResponse(provider, signed); err != nil {
		t.Fatalf("first exchange failed: %v", err)
	}
	requireSAMLReason(t, exchangeSAMLResponse(provider, signed), "replay")
}

func TestSAMLProvider_RejectsInResponseToMismatch(t *testing.T) {
	fixture := newSAMLSignerFixture(t)
	provider := newTestSAMLProvider(fixture, saml.NewMemoryReplayStore())

	opts := defaultSAMLResponseOptions()
	opts.inResponseTo = "_other-request"
	requireSAMLReason(t, exchangeSAMLResponse(provider, buildSAMLResponseXML(t, fixture, opts)), "in_response_to_mismatch")
}

func TestSAMLProvider_RejectsMissingInResponseTo(t *testing.T) {
	fixture := newSAMLSignerFixture(t)
	provider := newTestSAMLProvider(fixture, saml.NewMemoryReplayStore())

	opts := defaultSAMLResponseOptions()
	opts.inResponseTo = ""
	requireSAMLReason(t, exchangeSAMLResponse(provider, buildSAMLResponseXML(t, fixture, opts)), "in_response_to_missing")
}

func TestSAMLProvider_RejectsWrongIssuer(t *testing.T) {
	fixture := newSAMLSignerFixture(t)
	provider := newTestSAMLProvider(fixture, saml.NewMemoryReplayStore())

	opts := defaultSAMLResponseOptions()
	opts.issuer = "https://other-idp.example.com"
	requireSAMLReason(t, exchangeSAMLResponse(provider, buildSAMLResponseXML(t, fixture, opts)), "issuer_mismatch")
}

func TestSAMLProvider_GetRedirectURLBindsStateToRequestID(t *testing.T) {
	fixture := newSAMLSignerFixture(t)
	provider := newTestSAMLProvider(fixture, saml.NewMemoryReplayStore())

	redirect, err := provider.GetRedirectURL("state-1")
	if err != nil {
		t.Fatalf("GetRedirectURL failed: %v", err)
	}
	if !strings.Contains(redirect, "RelayState=state-1") {
		t.Fatalf("redirect URL does not carry RelayState: %s", redirect)
	}

	decoded := decodeSAMLRequestID(t, redirect)
	if decoded != "_state-1" {
		t.Fatalf("AuthnRequest ID = %q, want %q", decoded, "_state-1")
	}
}

func decodeSAMLRequestID(t *testing.T, redirect string) string {
	t.Helper()
	query, err := url.Parse(redirect)
	if err != nil {
		t.Fatalf("parse redirect URL: %v", err)
	}
	encoded := query.Query().Get("SAMLRequest")
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode SAMLRequest: %v", err)
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromString(string(raw)); err != nil {
		t.Fatalf("parse SAMLRequest XML: %v", err)
	}
	root := doc.Root()
	if root == nil {
		t.Fatal("empty SAMLRequest")
	}
	return root.SelectAttrValue("ID", "")
}
