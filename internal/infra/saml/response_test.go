package saml

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type samlTestSigner struct {
	key     *rsa.PrivateKey
	cert    *x509.Certificate
	certPEM string
}

func newSAMLTestSigner(t *testing.T) *samlTestSigner {
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
	return &samlTestSigner{
		key:     key,
		cert:    cert,
		certPEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
	}
}

type samlTestResponseOptions struct {
	responseID   string
	assertionID  string
	issuer       string
	audience     string
	recipient    string
	destination  string
	inResponseTo string
	nameID       string
	notBefore    time.Time
	notOnOrAfter time.Time
	sign         bool
}

func defaultSAMLTestResponseOptions() samlTestResponseOptions {
	now := time.Now().UTC()
	return samlTestResponseOptions{
		responseID:   "_resp-1",
		assertionID:  "_assert-1",
		issuer:       "https://idp.example.com",
		audience:     "https://sp.example.com",
		recipient:    "https://sp.example.com/acs",
		destination:  "https://sp.example.com/acs",
		inResponseTo: "_req-1",
		nameID:       "user@example.com",
		notBefore:    now.Add(-time.Minute),
		notOnOrAfter: now.Add(5 * time.Minute),
		sign:         true,
	}
}

func buildSAMLTestResponse(t *testing.T, signer *samlTestSigner, opts samlTestResponseOptions) string {
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

	attrStatement := assertion.CreateElement("saml:AttributeStatement")
	emailAttr := attrStatement.CreateElement("saml:Attribute")
	emailAttr.CreateAttr("Name", "email")
	emailAttr.CreateElement("saml:AttributeValue").SetText("user@example.com")

	if opts.sign {
		signingContext, err := dsig.NewSigningContext(signer.key, [][]byte{signer.cert.Raw})
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

func validResponseValidationConfig(signer *samlTestSigner, replayStore ReplayStore) SAMLResponseValidationConfig {
	return SAMLResponseValidationConfig{
		IdPCertPEM:           signer.certPEM,
		ExpectedAudience:     "https://sp.example.com",
		ExpectedRecipient:    "https://sp.example.com/acs",
		ExpectedDestination:  "https://sp.example.com/acs",
		ExpectedIssuer:       "https://idp.example.com",
		ExpectedInResponseTo: "_req-1",
		ReplayStore:          replayStore,
		ReplayKeyPrefix:      "saml-response|org-1|provider-1",
	}
}

func assertSAMLReason(t *testing.T, err error, want string) {
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

func TestValidateSAMLResponse_AcceptsValidResponse(t *testing.T) {
	signer := newSAMLTestSigner(t)
	data, err := ValidateSAMLResponse(
		context.Background(),
		buildSAMLTestResponse(t, signer, defaultSAMLTestResponseOptions()),
		validResponseValidationConfig(signer, NewMemoryReplayStore()),
	)
	if err != nil {
		t.Fatalf("valid response rejected: %v", err)
	}
	if data.NameID != "user@example.com" || data.Issuer != "https://idp.example.com" {
		t.Fatalf("unexpected verified data: %+v", data)
	}
	if data.Attributes["email"] != "user@example.com" {
		t.Fatalf("email attribute missing: %+v", data.Attributes)
	}
}

func TestValidateSAMLResponse_RejectsTamperedAssertion(t *testing.T) {
	signer := newSAMLTestSigner(t)
	xml := buildSAMLTestResponse(t, signer, defaultSAMLTestResponseOptions())
	tampered := strings.ReplaceAll(xml, "user@example.com", "attacker@example.com")
	if tampered == xml {
		t.Fatal("tamper mutation did not change the response")
	}
	_, err := ValidateSAMLResponse(context.Background(), tampered, validResponseValidationConfig(signer, NewMemoryReplayStore()))
	assertSAMLReason(t, err, "signature_invalid")
}

func TestValidateSAMLResponse_RejectsUnsignedResponse(t *testing.T) {
	signer := newSAMLTestSigner(t)
	opts := defaultSAMLTestResponseOptions()
	opts.sign = false
	_, err := ValidateSAMLResponse(context.Background(), buildSAMLTestResponse(t, signer, opts),
		validResponseValidationConfig(signer, NewMemoryReplayStore()))
	assertSAMLReason(t, err, "signature_invalid")
}

func TestValidateSAMLResponse_RejectsReplay(t *testing.T) {
	signer := newSAMLTestSigner(t)
	cfg := validResponseValidationConfig(signer, NewMemoryReplayStore())
	xml := buildSAMLTestResponse(t, signer, defaultSAMLTestResponseOptions())

	if _, err := ValidateSAMLResponse(context.Background(), xml, cfg); err != nil {
		t.Fatalf("first consumption failed: %v", err)
	}
	_, err := ValidateSAMLResponse(context.Background(), xml, cfg)
	assertSAMLReason(t, err, "replay")
}

func TestValidateSAMLResponse_RejectsInResponseToMismatch(t *testing.T) {
	signer := newSAMLTestSigner(t)
	opts := defaultSAMLTestResponseOptions()
	opts.inResponseTo = "_other-request"
	_, err := ValidateSAMLResponse(context.Background(), buildSAMLTestResponse(t, signer, opts),
		validResponseValidationConfig(signer, NewMemoryReplayStore()))
	assertSAMLReason(t, err, "in_response_to_mismatch")
}

func TestValidateSAMLResponse_RejectsMissingInResponseTo(t *testing.T) {
	signer := newSAMLTestSigner(t)
	opts := defaultSAMLTestResponseOptions()
	opts.inResponseTo = ""
	_, err := ValidateSAMLResponse(context.Background(), buildSAMLTestResponse(t, signer, opts),
		validResponseValidationConfig(signer, NewMemoryReplayStore()))
	assertSAMLReason(t, err, "in_response_to_missing")
}

func TestValidateSAMLResponse_RejectsAudienceMismatch(t *testing.T) {
	signer := newSAMLTestSigner(t)
	opts := defaultSAMLTestResponseOptions()
	opts.audience = "https://other-sp.example.com"
	_, err := ValidateSAMLResponse(context.Background(), buildSAMLTestResponse(t, signer, opts),
		validResponseValidationConfig(signer, NewMemoryReplayStore()))
	assertSAMLReason(t, err, "audience_mismatch")
}

func TestValidateSAMLResponse_RejectsRecipientMismatch(t *testing.T) {
	signer := newSAMLTestSigner(t)
	opts := defaultSAMLTestResponseOptions()
	opts.recipient = "https://other-sp.example.com/acs"
	_, err := ValidateSAMLResponse(context.Background(), buildSAMLTestResponse(t, signer, opts),
		validResponseValidationConfig(signer, NewMemoryReplayStore()))
	assertSAMLReason(t, err, "recipient_mismatch")
}

func TestValidateSAMLResponse_RejectsDestinationMismatch(t *testing.T) {
	signer := newSAMLTestSigner(t)
	opts := defaultSAMLTestResponseOptions()
	opts.destination = "https://other-sp.example.com/acs"
	_, err := ValidateSAMLResponse(context.Background(), buildSAMLTestResponse(t, signer, opts),
		validResponseValidationConfig(signer, NewMemoryReplayStore()))
	assertSAMLReason(t, err, "destination_mismatch")
}

func TestValidateSAMLResponse_RejectsExpiredAssertion(t *testing.T) {
	signer := newSAMLTestSigner(t)
	now := time.Now().UTC()
	opts := defaultSAMLTestResponseOptions()
	opts.notBefore = now.Add(-30 * time.Minute)
	opts.notOnOrAfter = now.Add(-20 * time.Minute)
	_, err := ValidateSAMLResponse(context.Background(), buildSAMLTestResponse(t, signer, opts),
		validResponseValidationConfig(signer, NewMemoryReplayStore()))
	assertSAMLReason(t, err, "assertion_expired")
}

func TestValidateSAMLResponse_FailsClosedWithoutConfig(t *testing.T) {
	signer := newSAMLTestSigner(t)
	xml := buildSAMLTestResponse(t, signer, defaultSAMLTestResponseOptions())

	cases := []SAMLResponseValidationConfig{
		{IdPCertPEM: signer.certPEM, ExpectedRecipient: "https://sp.example.com/acs", ExpectedDestination: "https://sp.example.com/acs", ExpectedInResponseTo: "_req-1", ReplayStore: NewMemoryReplayStore()},
		{IdPCertPEM: signer.certPEM, ExpectedAudience: "https://sp.example.com", ExpectedDestination: "https://sp.example.com/acs", ExpectedInResponseTo: "_req-1", ReplayStore: NewMemoryReplayStore()},
		{IdPCertPEM: signer.certPEM, ExpectedAudience: "https://sp.example.com", ExpectedRecipient: "https://sp.example.com/acs", ExpectedInResponseTo: "_req-1", ReplayStore: NewMemoryReplayStore()},
		{IdPCertPEM: signer.certPEM, ExpectedAudience: "https://sp.example.com", ExpectedRecipient: "https://sp.example.com/acs", ExpectedDestination: "https://sp.example.com/acs", ReplayStore: NewMemoryReplayStore()},
		{IdPCertPEM: signer.certPEM, ExpectedAudience: "https://sp.example.com", ExpectedRecipient: "https://sp.example.com/acs", ExpectedDestination: "https://sp.example.com/acs", ExpectedInResponseTo: "_req-1"},
	}
	for i, cfg := range cases {
		if _, err := ValidateSAMLResponse(context.Background(), xml, cfg); err == nil {
			t.Fatalf("case %d: incomplete configuration was accepted", i)
		}
	}
}

func TestMemoryReplayStore_Reserve(t *testing.T) {
	store := NewMemoryReplayStore()
	reserved, err := store.Reserve(context.Background(), "key-1", time.Minute)
	if err != nil || !reserved {
		t.Fatalf("first reserve = (%v, %v), want (true, nil)", reserved, err)
	}
	reserved, err = store.Reserve(context.Background(), "key-1", time.Minute)
	if err != nil || reserved {
		t.Fatalf("second reserve = (%v, %v), want (false, nil)", reserved, err)
	}
}
