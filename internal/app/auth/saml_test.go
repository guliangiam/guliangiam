package auth

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/guliangiam/guliangiam/internal/domain/applications"
	certdomain "github.com/guliangiam/guliangiam/internal/domain/certs"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// ---------------------------------------------------------------------------
// SAML test helpers
// ---------------------------------------------------------------------------

const (
	samlTestOrgID  = "01a00000-0000-7000-8000-0000000000b1"
	samlTestUserID = "01a00000-0000-7000-8000-0000000000b2"
	samlTestAppID  = "01a00000-0000-7000-8000-0000000000b3"
)

func newSamlTestOrg() *orgdomain.Organization {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	return &orgdomain.Organization{
		ID:        samlTestOrgID,
		Slug:      "saml-org",
		UpdatedAt: now,
		Security:  orgdomain.OrganizationSecurityPolicy{OrganizationID: samlTestOrgID},
	}
}

func newSamlTestUser() *userdomain.User {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	return &userdomain.User{
		ID:             samlTestUserID,
		OrganizationID: samlTestOrgID,
		Username:       "samluser",
		CreatedAt:      now,
		UpdatedAt:      now,
		Profile: userdomain.UserProfile{
			UserID:      samlTestUserID,
			DisplayName: "SAML User",
			Email:       "samluser@example.com",
			FirstName:   "SAML",
			LastName:    "User",
		},
	}
}

func newSamlTestApp() *applications.Application {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	return &applications.Application{
		ID:             samlTestAppID,
		OrganizationID: samlTestOrgID,
		Slug:           "saml-app",
		ClientID:       "saml-client-id",
		RedirectURIs:   []string{"https://sp.example.com/acs"},
		UpdatedAt:      now,
	}
}

func encodeSAMLAuthnRequest(id, issuer, acsURL string) string {
	request := fmt.Sprintf(`<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="%s" Version="2.0" IssueInstant="2026-05-15T10:00:00Z" AssertionConsumerServiceURL="%s"><saml:Issuer>%s</saml:Issuer></samlp:AuthnRequest>`,
		id, acsURL, issuer)
	return base64.StdEncoding.EncodeToString([]byte(request))
}

// ---------------------------------------------------------------------------
// SAML tests
// ---------------------------------------------------------------------------

func TestSAML_HandleSAMLRedirect_MissingRequest(t *testing.T) {
	svc := Service{
		Clock:         fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "test-id"},
	}

	result, err := svc.HandleSAMLRedirect(context.Background(), SAMLRequestCommand{
		OrganizationSlug: "saml-org",
		ApplicationSlug:  "saml-app",
		SAMLRequest:      "",
		RelayState:       "relay-123",
	})
	if err == nil {
		t.Fatal("expected error for empty SAMLRequest")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("expected invalid_argument, got %s", appErr.Code)
	}
}

func TestSAML_HandleSAMLRedirect_ValidRequest(t *testing.T) {
	svc := Service{
		Clock:         fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "test-id"},
	}

	encoded := encodeSAMLAuthnRequest("req-123", "https://sp.example.com", "https://sp.example.com/acs")

	result, err := svc.HandleSAMLRedirect(context.Background(), SAMLRequestCommand{
		OrganizationSlug: "saml-org",
		ApplicationSlug:  "saml-app",
		SAMLRequest:      encoded,
		RelayState:       "relay-123",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.SAMLRequest != encoded {
		t.Error("expected SAMLRequest to be preserved in result")
	}
	if result.RelayState != "relay-123" {
		t.Errorf("expected relay-123, got %s", result.RelayState)
	}
}

func TestSAML_HandleSAMLRedirect_InvalidBase64(t *testing.T) {
	svc := Service{
		Clock:         fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "test-id"},
	}

	result, err := svc.HandleSAMLRedirect(context.Background(), SAMLRequestCommand{
		OrganizationSlug: "saml-org",
		ApplicationSlug:  "saml-app",
		SAMLRequest:      "not-valid-base64!!!",
		RelayState:       "",
	})
	if err == nil {
		t.Fatal("expected error for invalid base64 SAMLRequest")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
}

func TestSAML_GenerateSAMLResponse_MissingRequest(t *testing.T) {
	svc := Service{
		Clock:         fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "test-id"},
	}

	result, err := svc.GenerateSAMLResponse(context.Background(), SAMLResponseCommand{
		OrganizationSlug: "saml-org",
		Username:         "samluser",
		ApplicationID:    samlTestAppID,
		SAMLRequest:      "",
		RelayState:       "relay-123",
	})
	if err == nil {
		t.Fatal("expected error for empty SAMLRequest")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("expected invalid_argument, got %s", appErr.Code)
	}
}

func TestSAML_GenerateSAMLResponse_MissingUsername(t *testing.T) {
	svc := Service{
		Clock:         fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "test-id"},
	}

	encoded := encodeSAMLAuthnRequest("req-123", "https://sp.example.com", "https://sp.example.com/acs")

	result, err := svc.GenerateSAMLResponse(context.Background(), SAMLResponseCommand{
		OrganizationSlug: "saml-org",
		Username:         "",
		ApplicationID:    samlTestAppID,
		SAMLRequest:      encoded,
		RelayState:       "relay-123",
	})
	if err == nil {
		t.Fatal("expected error for empty username")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("expected invalid_argument, got %s", appErr.Code)
	}
}

func TestSAML_GetSAMLMetadata(t *testing.T) {
	org := newSamlTestOrg()
	user := newSamlTestUser()
	app := newSamlTestApp()
	uow := samlCertTestUoW{loginTestUnitOfWork: newTestUow(org, user, app), certs: &samlTestCertRepo{}}
	svc := newTestService(uow.loginTestUnitOfWork)
	svc.UOW = uow
	svc.IssuerURL = "https://iam.example.com"

	metadata, err := svc.GetSAMLMetadata(context.Background(), "saml-org", samlTestAppID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(metadata) == 0 {
		t.Fatal("expected non-empty metadata")
	}
	metadataStr := string(metadata)
	if !strings.Contains(metadataStr, "EntityDescriptor") {
		t.Error("expected EntityDescriptor in metadata")
	}
	if !strings.Contains(metadataStr, samlTestAppID) {
		t.Error("expected application ID in metadata")
	}
	if !strings.Contains(metadataStr, "https://iam.example.com/api/v1/saml/sso/"+samlTestAppID) {
		t.Error("expected absolute SSO endpoint Location on the configured issuer")
	}
}

func TestSAML_GetSAMLMetadata_FailsClosedWithoutIssuer(t *testing.T) {
	org := newSamlTestOrg()
	user := newSamlTestUser()
	app := newSamlTestApp()
	uow := samlCertTestUoW{loginTestUnitOfWork: newTestUow(org, user, app), certs: &samlTestCertRepo{}}
	svc := newTestService(uow.loginTestUnitOfWork)
	svc.UOW = uow
	svc.IssuerURL = ""

	_, err := svc.GetSAMLMetadata(context.Background(), "saml-org", samlTestAppID)
	if err == nil {
		t.Fatal("expected error when no issuer URL is configured")
	}
	if apperrors.FromError(err).Code != apperrors.CodeInternal {
		t.Errorf("expected internal, got %s", apperrors.FromError(err).Code)
	}
}

func TestSAML_GetSAMLMetadata_AppNotBelongToOrg(t *testing.T) {
	org := newSamlTestOrg()
	user := newSamlTestUser()
	app := newSamlTestApp()
	app.OrganizationID = "different-org" // mismatch
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	_, err := svc.GetSAMLMetadata(context.Background(), "saml-org", samlTestAppID)
	if err == nil {
		t.Fatal("expected error when app does not belong to org")
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("expected invalid_argument, got %s", appErr.Code)
	}
}

func TestSAML_EscapeXML(t *testing.T) {
	if v := escapeXML("hello"); v != "hello" {
		t.Errorf("expected hello, got %s", v)
	}
	if v := escapeXML("<b>"); v != "&lt;b&gt;" {
		t.Errorf("expected &lt;b&gt;, got %s", v)
	}
	if v := escapeXML(`"quote"`); v != "&quot;quote&quot;" {
		t.Errorf("expected &quot;quote&quot;, got %s", v)
	}
	if v := escapeXML("a&b"); v != "a&amp;b" {
		t.Errorf("expected a&amp;b, got %s", v)
	}
	if v := escapeXML("it's"); v != "it&apos;s" {
		t.Errorf("expected it&apos;s, got %s", v)
	}
}

// ---------------------------------------------------------------------------
// RelayState signing (SEC-A01)
// ---------------------------------------------------------------------------

// fakeRelayStateSigner mirrors the envelope contract without crypto so the app
// layer's wiring and fail-close behaviour can be asserted in isolation.
type fakeRelayStateSigner struct{}

func (fakeRelayStateSigner) Sign(state, org, app string) (string, error) {
	if state == "" {
		return "", errors.New("empty state")
	}
	return "signed:" + org + ":" + app + ":" + state, nil
}

func (fakeRelayStateSigner) Verify(signed, org, app string) (string, error) {
	prefix := "signed:" + org + ":" + app + ":"
	if !strings.HasPrefix(signed, prefix) {
		return "", errors.New("invalid relay state")
	}
	return strings.TrimPrefix(signed, prefix), nil
}

func TestSAML_BuildSPAuthnRequest_SignsRelayState(t *testing.T) {
	org := newSamlTestOrg()
	user := newSamlTestUser()
	app := newSamlTestApp()
	svc := newTestService(newTestUow(org, user, app))
	svc.RelayStateSigner = fakeRelayStateSigner{}

	result, err := svc.BuildSPAuthnRequest(context.Background(), BuildSPAuthnRequestCommand{
		OrganizationSlug: "saml-org",
		SPEntityID:       "https://sp.example.com",
		SPACSURL:         "https://sp.example.com/acs",
		RelayState:       "relay-123",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.RelayState != "signed:saml-org::_"+testSessID+"|https://sp.example.com|https://sp.example.com/acs|relay-123" {
		t.Fatalf("expected signed relay state carrying the request binding, got %q", result.RelayState)
	}
}

func TestSAML_BuildSPAuthnRequest_FailsClosedWithoutSigner(t *testing.T) {
	org := newSamlTestOrg()
	user := newSamlTestUser()
	app := newSamlTestApp()
	svc := newTestService(newTestUow(org, user, app))

	_, err := svc.BuildSPAuthnRequest(context.Background(), BuildSPAuthnRequestCommand{
		OrganizationSlug: "saml-org",
		SPEntityID:       "https://sp.example.com",
		SPACSURL:         "https://sp.example.com/acs",
		RelayState:       "relay-123",
	})
	if err == nil {
		t.Fatal("expected error when the relay state signer is not configured")
	}
	if apperrors.FromError(err).Code != apperrors.CodeInternal {
		t.Fatalf("expected internal error, got %s", apperrors.FromError(err).Code)
	}
}

func TestSAML_HandleSPInitiatedACS_RejectsInvalidRelayState(t *testing.T) {
	svc := Service{
		Clock:            fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator:    fakeUUIDGenerator{fixedID: "test-id"},
		RelayStateSigner: fakeRelayStateSigner{},
	}

	_, err := svc.HandleSPInitiatedACS(context.Background(), HandleSPInitiatedACSCommand{
		OrganizationSlug: "saml-org",
		ApplicationSlug:  "saml-app",
		SAMLResponse:     base64.StdEncoding.EncodeToString([]byte("<Response/>")),
		RelayState:       "forged-state",
	})
	if err == nil {
		t.Fatal("expected error for an unsigned relay state")
	}
	if apperrors.FromError(err).Code != apperrors.CodeInvalidArgument {
		t.Fatalf("expected invalid_argument, got %s", apperrors.FromError(err).Code)
	}
}

// ---------------------------------------------------------------------------
// Signed issuance + ACS whitelist (BE-01 / BE-02 / BE-04)
// ---------------------------------------------------------------------------

// samlTestCertRepo serves the application signing certificate/key pair.
type samlTestCertRepo struct {
	certdomain.Repository
	certs []*certdomain.Cert
}

func (r *samlTestCertRepo) List(_ context.Context, _ certdomain.ListQuery) ([]*certdomain.Cert, int64, error) {
	return r.certs, int64(len(r.certs)), nil
}

// samlCertTestUoW adds a certificate repository to the login test UoW.
type samlCertTestUoW struct {
	loginTestUnitOfWork
	certs certdomain.Repository
}

func (u samlCertTestUoW) Certs() certdomain.Repository { return u.certs }

func newSamlTestKeyPair(t *testing.T) (*rsa.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "saml-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return key, cert
}

func samlTestCertPEM(cert *x509.Certificate) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))
}

func samlTestKeyPEM(key *rsa.PrivateKey) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
}

func TestSAML_ResolveSAMLACSURL(t *testing.T) {
	app := newSamlTestApp()

	// Empty request falls back to the first registered URI.
	acs, err := resolveSAMLACSURL("", app)
	if err != nil || acs != "https://sp.example.com/acs" {
		t.Fatalf("expected registered fallback, got %q / %v", acs, err)
	}

	// Exact registered match is accepted.
	acs, err = resolveSAMLACSURL("https://sp.example.com/acs", app)
	if err != nil || acs != "https://sp.example.com/acs" {
		t.Fatalf("expected registered match, got %q / %v", acs, err)
	}

	// Unregistered destination is refused.
	_, err = resolveSAMLACSURL("https://evil.example.com/acs", app)
	if apperrors.FromError(err).Code != apperrors.CodeForbidden {
		t.Fatalf("expected forbidden for unregistered ACS, got %v", err)
	}

	// No registered URIs and no request → nothing to deliver to.
	empty := newSamlTestApp()
	empty.RedirectURIs = nil
	_, err = resolveSAMLACSURL("", empty)
	if apperrors.FromError(err).Code != apperrors.CodeInvalidArgument {
		t.Fatalf("expected invalid_argument without registered ACS, got %v", err)
	}
}

func TestSAML_GenerateSAMLResponse_RejectsUnregisteredACS(t *testing.T) {
	org := newSamlTestOrg()
	user := newSamlTestUser()
	app := newSamlTestApp()
	svc := newTestService(newTestUow(org, user, app))

	encoded := encodeSAMLAuthnRequest("req-123", "https://sp.example.com", "https://evil.example.com/acs")
	_, err := svc.GenerateSAMLResponse(context.Background(), SAMLResponseCommand{
		OrganizationSlug: "saml-org",
		Username:         "samluser",
		ApplicationID:    samlTestAppID,
		SAMLRequest:      encoded,
	})
	if apperrors.FromError(err).Code != apperrors.CodeForbidden {
		t.Fatalf("expected forbidden for unregistered ACS URL, got %v", err)
	}
}

func TestSAML_GenerateSAMLResponse_SignsAndEncodesOnce(t *testing.T) {
	org := newSamlTestOrg()
	user := newSamlTestUser()
	app := newSamlTestApp()
	key, cert := newSamlTestKeyPair(t)
	uow := samlCertTestUoW{
		loginTestUnitOfWork: newTestUow(org, user, app),
		certs:               &samlTestCertRepo{certs: []*certdomain.Cert{{Certificate: samlTestCertPEM(cert), PrivateKey: samlTestKeyPEM(key)}}},
	}
	svc := Service{
		Clock:         fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator: platformuuid.V7Generator{},
		UOW:           uow,
	}

	encoded := encodeSAMLAuthnRequest("req-123", "https://sp.example.com", "https://sp.example.com/acs")
	result, err := svc.GenerateSAMLResponse(context.Background(), SAMLResponseCommand{
		OrganizationSlug: "saml-org",
		Username:         "samluser",
		ApplicationID:    samlTestAppID,
		SAMLRequest:      encoded,
		RelayState:       "relay-123",
	})
	if err != nil {
		t.Fatalf("expected signed response, got %v", err)
	}
	if result.RedirectURL != "https://sp.example.com/acs" || result.Method != "POST" {
		t.Fatalf("unexpected delivery target: %+v", result)
	}

	// Exactly one base64 layer: decoding once must yield the XML document.
	xmlBytes, err := base64.StdEncoding.DecodeString(result.SAMLResponse)
	if err != nil {
		t.Fatalf("SAMLResponse is not valid base64: %v", err)
	}
	xmlStr := string(xmlBytes)
	if !strings.HasPrefix(strings.TrimSpace(xmlStr), "<") {
		t.Fatal("decoded SAMLResponse is not XML — double encoding suspected")
	}
	if !strings.Contains(xmlStr, `InResponseTo="req-123"`) {
		t.Error("expected InResponseTo bound to the AuthnRequest ID")
	}
	if !strings.Contains(xmlStr, `Destination="https://sp.example.com/acs"`) {
		t.Error("expected Destination bound to the validated ACS URL")
	}

	// The signature must actually verify against the signing certificate.
	doc := etree.NewDocument()
	if err := doc.ReadFromString(xmlStr); err != nil {
		t.Fatalf("parse signed response: %v", err)
	}
	if _, err := verifySamlLogoutRequestSignature(doc.Root(), cert); err != nil {
		t.Fatalf("signed response failed verification: %v", err)
	}
}

func TestSAML_GenerateSAMLResponse_FailsClosedWithoutSigningKey(t *testing.T) {
	org := newSamlTestOrg()
	user := newSamlTestUser()
	app := newSamlTestApp()
	svc := newTestService(newTestUow(org, user, app)) // Certs() == nil

	encoded := encodeSAMLAuthnRequest("req-123", "https://sp.example.com", "https://sp.example.com/acs")
	_, err := svc.GenerateSAMLResponse(context.Background(), SAMLResponseCommand{
		OrganizationSlug: "saml-org",
		Username:         "samluser",
		ApplicationID:    samlTestAppID,
		SAMLRequest:      encoded,
	})
	if err == nil {
		t.Fatal("expected fail-close when no signing key is configured")
	}
	if apperrors.FromError(err).Code != apperrors.CodeInternal {
		t.Fatalf("expected internal, got %s", apperrors.FromError(err).Code)
	}
}

func TestSAML_HandleLoggedInSAML_RejectsUnregisteredACS(t *testing.T) {
	org := newSamlTestOrg()
	user := newSamlTestUser()
	app := newSamlTestApp()
	svc := newTestService(newTestUow(org, user, app))

	encoded := encodeSAMLAuthnRequest("req-123", "https://sp.example.com", "https://evil.example.com/acs")
	_, err := svc.handleLoggedInSAML(context.Background(), LoginCommand{
		SAMLRequest: encoded,
		RelayState:  "relay-123",
	}, org, app, user)
	if apperrors.FromError(err).Code != apperrors.CodeForbidden {
		t.Fatalf("expected forbidden for unregistered ACS URL on the login path, got %v", err)
	}
}

func TestSAML_ParseSamlRequest_DeflateBinding(t *testing.T) {
	raw := `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ID="req-deflate" Version="2.0" AssertionConsumerServiceURL="https://sp.example.com/acs"><Issuer>https://sp.example.com</Issuer></samlp:AuthnRequest>`
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		t.Fatalf("flate writer: %v", err)
	}
	if _, err := w.Write([]byte(raw)); err != nil {
		t.Fatalf("flate write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("flate close: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	svc := Service{}
	info, err := svc.ParseSamlRequest(context.Background(), encoded)
	if err != nil {
		t.Fatalf("expected deflate-compressed request to parse, got %v", err)
	}
	if info.ID != "req-deflate" || info.ACSURL != "https://sp.example.com/acs" {
		t.Fatalf("unexpected parsed request: %+v", info)
	}
}
