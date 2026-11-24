package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/domain/applications"
	domainauth "github.com/guliangiam/guliangiam/internal/domain/auth"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

// ──────────────────────────────────────────────────────────────
// Chain B helpers: signed RelayState binding + recording validator
// ──────────────────────────────────────────────────────────────

type recordingSAMLValidator struct {
	err   error
	input *domainauth.SAMLAssertionInput
	cfg   domainauth.SAMLAssertionConfig
}

func (v *recordingSAMLValidator) ValidateSPAuthnRequest(*domainauth.SAMLSPAuthnRequestInput, domainauth.SAMLSPAuthnRequestConfig) error {
	return nil
}

func (v *recordingSAMLValidator) ValidateSAMLAssertion(in *domainauth.SAMLAssertionInput, cfg domainauth.SAMLAssertionConfig) error {
	v.input = in
	v.cfg = cfg
	return v.err
}

type samlChainBAppRepository struct {
	applications.Repository
	app *applications.Application
}

func (r samlChainBAppRepository) GetByOrganizationAndSlug(_ context.Context, organizationID, slug string) (*applications.Application, error) {
	if r.app == nil || r.app.OrganizationID != organizationID || r.app.Slug != slug {
		return nil, apperrors.NewNotFound("application not found")
	}
	return r.app, nil
}

type samlChainBUserRepository struct {
	userdomain.Repository
	user *userdomain.User
}

func (r samlChainBUserRepository) List(_ context.Context, query userdomain.ListQuery) ([]*userdomain.User, int64, error) {
	if r.user == nil {
		return nil, 0, nil
	}
	if query.OrganizationID != "" && query.OrganizationID != r.user.OrganizationID {
		return nil, 0, nil
	}
	return []*userdomain.User{r.user}, 1, nil
}

type samlChainBUoW struct {
	loginTestUnitOfWork
	appRepo  applications.Repository
	userRepo userdomain.Repository
}

func (u samlChainBUoW) Applications() applications.Repository { return u.appRepo }
func (u samlChainBUoW) Users() userdomain.Repository          { return u.userRepo }

// samlMetadataWithCert builds IdP metadata carrying a throwaway signing cert.
func samlMetadataWithCert(t *testing.T) string {
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
	return fmt.Sprintf(`<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://idp.example.com">
  <md:IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <md:KeyDescriptor use="signing">
      <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
        <ds:X509Data><ds:X509Certificate>%s</ds:X509Certificate></ds:X509Data>
      </ds:KeyInfo>
    </md:KeyDescriptor>
  </md:IDPSSODescriptor>
</md:EntityDescriptor>`, base64.StdEncoding.EncodeToString(der))
}

func samlChainBResponseXML(nameID, inResponseTo, audience, acsURL string) string {
	now := time.Now().UTC()
	notOnOrAfter := now.Add(5 * time.Minute).Format(time.RFC3339)
	return fmt.Sprintf(`<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" ID="_resp-1" Version="2.0" IssueInstant="%s" Destination="%s" InResponseTo="%s">
  <saml:Issuer>https://idp.example.com</saml:Issuer>
  <samlp:Status><samlp:StatusCode Value="urn:oasis:names:tc:SAML:2.0:status:Success"/></samlp:Status>
  <saml:Assertion ID="_assert-1" Version="2.0" IssueInstant="%s">
    <saml:Issuer>https://idp.example.com</saml:Issuer>
    <ds:Signature><ds:SignedInfo/></ds:Signature>
    <saml:Subject>
      <saml:NameID Format="urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress">%s</saml:NameID>
      <saml:SubjectConfirmation Method="urn:oasis:names:tc:SAML:2.0:cm:bearer">
        <saml:SubjectConfirmationData NotOnOrAfter="%s" Recipient="%s" InResponseTo="%s"/>
      </saml:SubjectConfirmation>
    </saml:Subject>
    <saml:Conditions NotBefore="%s" NotOnOrAfter="%s">
      <saml:AudienceRestriction><saml:Audience>%s</saml:Audience></saml:AudienceRestriction>
    </saml:Conditions>
    <saml:AuthnStatement AuthnInstant="%s" SessionIndex="_session-1"/>
    <saml:AttributeStatement>
      <saml:Attribute Name="email"><saml:AttributeValue>%s</saml:AttributeValue></saml:Attribute>
    </saml:AttributeStatement>
  </saml:Assertion>
</samlp:Response>`,
		now.Format(time.RFC3339), acsURL, inResponseTo, now.Format(time.RFC3339),
		nameID, notOnOrAfter, acsURL, inResponseTo,
		now.Format(time.RFC3339), notOnOrAfter, audience,
		now.Format(time.RFC3339), nameID,
	)
}

func TestSAML_HandleSPInitiatedACS_BindsRequestContextFromRelayState(t *testing.T) {
	org := newSamlTestOrg()
	user := newSamlTestUser()
	app := newSamlTestApp()
	app.SamlMetadata = samlMetadataWithCert(t)

	uow := samlChainBUoW{
		loginTestUnitOfWork: newTestUow(org, user, app),
		appRepo:             samlChainBAppRepository{app: app},
		userRepo:            samlChainBUserRepository{user: user},
	}
	validator := &recordingSAMLValidator{}
	svc := Service{
		Clock:            fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator:    fakeUUIDGenerator{fixedID: "test-id"},
		UOW:              uow,
		RelayStateSigner: fakeRelayStateSigner{},
		SAMLValidator:    validator,
	}

	payload := encodeSPRelayState("_test-id", "https://sp.example.com", "https://sp.example.com/acs", "relay-123")
	response := samlChainBResponseXML("samluser@example.com", "_test-id", "https://sp.example.com", "https://sp.example.com/acs")

	result, err := svc.HandleSPInitiatedACS(context.Background(), HandleSPInitiatedACSCommand{
		OrganizationSlug: "saml-org",
		ApplicationSlug:  "saml-app",
		SAMLResponse:     base64.StdEncoding.EncodeToString([]byte(response)),
		RelayState:       "signed:saml-org:saml-app:" + payload,
	})
	if err != nil {
		t.Fatalf("HandleSPInitiatedACS() error = %v, want nil", err)
	}
	if result == nil || result.NameID != "samluser@example.com" {
		t.Fatalf("unexpected result: %+v", result)
	}

	if validator.cfg.ExpectedInResponseTo != "_test-id" {
		t.Fatalf("ExpectedInResponseTo = %q, want %q", validator.cfg.ExpectedInResponseTo, "_test-id")
	}
	if validator.cfg.ExpectedAudience != "https://sp.example.com" {
		t.Fatalf("ExpectedAudience = %q", validator.cfg.ExpectedAudience)
	}
	if validator.cfg.ExpectedRecipient != "https://sp.example.com/acs" ||
		validator.cfg.ExpectedDestination != "https://sp.example.com/acs" {
		t.Fatalf("recipient/destination not bound: %+v", validator.cfg)
	}
	if validator.cfg.ExpectedIssuer != "https://idp.example.com" {
		t.Fatalf("ExpectedIssuer = %q", validator.cfg.ExpectedIssuer)
	}
	if validator.cfg.IdPSigningCert == nil || len(validator.cfg.RawAssertion) == 0 {
		t.Fatal("signature verification inputs were not provided")
	}
	if !validator.input.SignaturePresent {
		t.Fatal("SignaturePresent = false, want true for a signed response")
	}
}

func TestSAML_HandleSPInitiatedACS_RequiresSignedRelayState(t *testing.T) {
	svc := Service{RelayStateSigner: fakeRelayStateSigner{}}
	_, err := svc.HandleSPInitiatedACS(context.Background(), HandleSPInitiatedACSCommand{
		OrganizationSlug: "saml-org",
		ApplicationSlug:  "saml-app",
		SAMLResponse:     base64.StdEncoding.EncodeToString([]byte("<Response/>")),
	})
	if err == nil {
		t.Fatal("missing RelayState was accepted")
	}
}

func TestSAML_HandleSPInitiatedACS_RejectsRelayStateWithoutRequestBinding(t *testing.T) {
	svc := Service{RelayStateSigner: fakeRelayStateSigner{}}
	_, err := svc.HandleSPInitiatedACS(context.Background(), HandleSPInitiatedACSCommand{
		OrganizationSlug: "saml-org",
		ApplicationSlug:  "saml-app",
		SAMLResponse:     base64.StdEncoding.EncodeToString([]byte("<Response/>")),
		RelayState:       "signed:saml-org:saml-app:not-a-request-binding",
	})
	if err == nil {
		t.Fatal("RelayState without the request binding was accepted")
	}
}

func TestSAML_HandleSPInitiatedACS_FailsClosedWithoutApplicationMetadata(t *testing.T) {
	org := newSamlTestOrg()
	user := newSamlTestUser()
	app := newSamlTestApp() // SamlMetadata intentionally empty

	uow := samlChainBUoW{
		loginTestUnitOfWork: newTestUow(org, user, app),
		appRepo:             samlChainBAppRepository{app: app},
		userRepo:            samlChainBUserRepository{user: user},
	}
	svc := Service{
		Clock:            fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator:    fakeUUIDGenerator{fixedID: "test-id"},
		UOW:              uow,
		RelayStateSigner: fakeRelayStateSigner{},
		SAMLValidator:    &recordingSAMLValidator{},
	}

	payload := encodeSPRelayState("_test-id", "https://sp.example.com", "https://sp.example.com/acs", "relay-123")
	_, err := svc.HandleSPInitiatedACS(context.Background(), HandleSPInitiatedACSCommand{
		OrganizationSlug: "saml-org",
		ApplicationSlug:  "saml-app",
		SAMLResponse:     base64.StdEncoding.EncodeToString([]byte(samlChainBResponseXML("samluser@example.com", "_test-id", "https://sp.example.com", "https://sp.example.com/acs"))),
		RelayState:       "signed:saml-org:saml-app:" + payload,
	})
	if err == nil {
		t.Fatal("response was accepted without an IdP signing certificate")
	}
}

// ──────────────────────────────────────────────────────────────
// Chain A audit: LoginWithProvider SAML rejection
// ──────────────────────────────────────────────────────────────

type samlAuditProviderRepository struct {
	providerdomain.Repository
	provider *providerdomain.Provider
}

func (r samlAuditProviderRepository) GetByID(context.Context, string) (*providerdomain.Provider, error) {
	return r.provider, nil
}

type samlAuditRecordRepository struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *samlAuditRecordRepository) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type samlAuditOutboxRepository struct {
	domainoutbox.Repository
	events []*domainoutbox.OutboxEvent
}

func (r *samlAuditOutboxRepository) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	r.events = append(r.events, event)
	return nil
}

type samlAuditUoW struct {
	loginTestUnitOfWork
	providers providerdomain.Repository
	records   recordsdomain.Repository
	outbox    domainoutbox.Repository
}

func (u samlAuditUoW) Providers() providerdomain.Repository { return u.providers }
func (u samlAuditUoW) Records() recordsdomain.Repository    { return u.records }
func (u samlAuditUoW) Outbox() domainoutbox.Repository      { return u.outbox }
func (u samlAuditUoW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func TestLoginWithProvider_SAMLFailureWritesAuditWithoutAssertion(t *testing.T) {
	original := exchangeSocialCode
	exchangeSocialCode = func(context.Context, SocialProviderConfig, string) (*SocialUserInfo, error) {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnauthorized, "saml rejected", map[string]any{
			"samlReason":    "audience_mismatch",
			"samlIssuer":    "https://idp.example.com",
			"samlRequestId": "_req-1",
			"rawAssertion":  "<saml:Assertion>secret</saml:Assertion>",
		})
	}
	t.Cleanup(func() { exchangeSocialCode = original })

	provider := &providerdomain.Provider{
		ID:             "provider-1",
		OrganizationID: testOrgID,
		Name:           "corp-saml",
		Category:       "saml",
		ProviderType:   "saml",
		Enabled:        true,
		CallbackUrl:    "https://sp.example.com/acs",
	}
	recordRepo := &samlAuditRecordRepository{}
	outboxRepo := &samlAuditOutboxRepository{}
	uow := samlAuditUoW{
		loginTestUnitOfWork: newTestUow(newTestOrg(), newTestUser(), newTestApp()),
		providers:           samlAuditProviderRepository{provider: provider},
		records:             recordRepo,
		outbox:              outboxRepo,
	}
	signer := samlACSTestSigner{issuedAt: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)}
	svc := Service{
		Clock:            fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator:    fakeUUIDGenerator{fixedID: "test-id"},
		UOW:              uow,
		RelayStateSigner: signer,
		SAMLReplayStore:  newSAMLAcstestReplayStore(),
	}

	_, err := svc.LoginWithProvider(context.Background(), SocialLoginCommand{
		ProviderID:       "provider-1",
		Code:             base64.StdEncoding.EncodeToString([]byte("<Response/>")),
		State:            samlACSTestState(t, signer, testOrgSlug, provider.ID, "nonce-audit"),
		OrganizationSlug: testOrgSlug,
		IPAddress:        "203.0.113.10",
		UserAgent:        "test-agent",
	})
	if err == nil {
		t.Fatal("SAML failure did not produce an error")
	}
	if apperrors.FromError(err).Code != apperrors.CodeUnauthorized {
		t.Fatalf("error code = %s, want unauthorized", apperrors.FromError(err).Code)
	}

	if len(recordRepo.created) != 1 {
		t.Fatalf("audit records = %d, want 1", len(recordRepo.created))
	}
	record := recordRepo.created[0]
	if record.Action != audit.EventAuthLoginFailed {
		t.Fatalf("audit action = %q, want %q", record.Action, audit.EventAuthLoginFailed)
	}
	if record.ResourceID != provider.ID {
		t.Fatalf("audit resource = %q, want provider id", record.ResourceID)
	}

	if len(outboxRepo.events) != 1 {
		t.Fatalf("outbox events = %d, want 1", len(outboxRepo.events))
	}
	var payload map[string]any
	if err := json.Unmarshal(outboxRepo.events[0].Payload, &payload); err != nil {
		t.Fatalf("decode audit payload: %v", err)
	}
	if payload["reason"] != "audience_mismatch" {
		t.Fatalf("payload reason = %v", payload["reason"])
	}
	if payload["issuer"] != "https://idp.example.com" {
		t.Fatalf("payload issuer = %v", payload["issuer"])
	}
	if payload["requestId"] != "_req-1" {
		t.Fatalf("payload requestId = %v", payload["requestId"])
	}
	if _, leaked := payload["rawAssertion"]; leaked {
		t.Fatal("audit payload leaked assertion content")
	}
	for _, value := range payload {
		if text, ok := value.(string); ok && text == "<saml:Assertion>secret</saml:Assertion>" {
			t.Fatal("audit payload leaked assertion content")
		}
	}
}
