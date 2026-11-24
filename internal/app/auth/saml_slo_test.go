package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	certdomain "github.com/guliangiam/guliangiam/internal/domain/certs"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	dsig "github.com/russellhaering/goxmldsig"
)

// samlSLOSessionRepo records the SLO fan-out call. Embedding the domain
// interface keeps the unused methods out of the test.
type samlSLOSessionRepo struct {
	sessiondomain.Repository

	calls     int
	userID    string
	exclude   string
	revokedAt time.Time
	revoked   int64
}

func (r *samlSLOSessionRepo) RevokeByUserExcept(_ context.Context, userID, excludeSessionID string, revokedAt time.Time) (int64, error) {
	r.calls++
	r.userID = userID
	r.exclude = excludeSessionID
	r.revokedAt = revokedAt
	return r.revoked, nil
}

type samlSLOAppRepo struct {
	applicationdomain.Repository
	apps []*applicationdomain.Application
}

func (r *samlSLOAppRepo) List(_ context.Context, _ applicationdomain.ListQuery) ([]*applicationdomain.Application, int64, error) {
	return r.apps, int64(len(r.apps)), nil
}

type samlSLOUserRepo struct {
	userdomain.Repository
	users []*userdomain.User
}

func (r *samlSLOUserRepo) List(_ context.Context, _ userdomain.ListQuery) ([]*userdomain.User, int64, error) {
	return r.users, int64(len(r.users)), nil
}

// samlSLOTestUoW overrides the repositories the SLO path touches; Records and
// Outbox stay nil (EmbedAuditEvent tolerates absent sinks) and the embedded
// WithinTransaction runs the callback inline.
type samlSLOTestUoW struct {
	loginTestUnitOfWork
	apps     *samlSLOAppRepo
	users    *samlSLOUserRepo
	sessions *samlSLOSessionRepo
	certs    certdomain.Repository
}

func (u samlSLOTestUoW) Applications() applicationdomain.Repository { return u.apps }
func (u samlSLOTestUoW) Users() userdomain.Repository               { return u.users }
func (u samlSLOTestUoW) Sessions() sessiondomain.Repository         { return u.sessions }
func (u samlSLOTestUoW) Certs() certdomain.Repository               { return u.certs }

// The embedded WithinTransaction would hand the callback the inner
// loginTestUnitOfWork, bypassing these overrides — pass the outer UoW instead.
func (u samlSLOTestUoW) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

// samlSPMetadata renders an SP EntityDescriptor pinning the given certificate,
// the shape samlSigningCertificate consumes.
func samlSPMetadata(cert *x509.Certificate) string {
	return fmt.Sprintf(`<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="test-idp"><IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"><KeyDescriptor use="signing"><KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#"><X509Data><X509Certificate>%s</X509Certificate></X509Data></KeyInfo></KeyDescriptor></IDPSSODescriptor></EntityDescriptor>`,
		base64.StdEncoding.EncodeToString(cert.Raw))
}

func signedSAMLLogoutRequest(t *testing.T, key *rsa.PrivateKey, cert *x509.Certificate, nameID string, hash crypto.Hash) string {
	t.Helper()
	root := &etree.Element{Space: "samlp", Tag: "LogoutRequest"}
	root.CreateAttr("xmlns:samlp", "urn:oasis:names:tc:SAML:2.0:protocol")
	root.CreateAttr("xmlns:saml", "urn:oasis:names:tc:SAML:2.0:assertion")
	root.CreateAttr("ID", "logout-1")
	root.CreateAttr("Version", "2.0")
	root.CreateAttr("IssueInstant", time.Now().UTC().Format(time.RFC3339))
	root.CreateElement("saml:Issuer").SetText("test-idp")
	root.CreateElement("saml:NameID").SetText(nameID)

	signingContext, err := dsig.NewSigningContext(key, [][]byte{cert.Raw})
	require.NoError(t, err)
	signingContext.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")
	signingContext.Hash = hash
	signed, err := signingContext.SignEnveloped(root)
	require.NoError(t, err)
	doc := etree.NewDocument()
	doc.SetRoot(signed)
	xmlStr, err := doc.WriteToString()
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString([]byte(xmlStr))
}

func unsignedSAMLLogoutRequest(nameID string) string {
	request := fmt.Sprintf(`<samlp:LogoutRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="logout-1" Version="2.0">
<saml:Issuer>test-idp</saml:Issuer>
<saml:NameID Format="urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified">%s</saml:NameID>
</samlp:LogoutRequest>`, nameID)
	return base64.StdEncoding.EncodeToString([]byte(request))
}

func newSamlSLOFixture(t *testing.T) (Service, *samlSLOSessionRepo, *rsa.PrivateKey, *x509.Certificate) {
	t.Helper()
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)

	spKey, spCert := newSamlTestKeyPair(t)
	idpKey, idpCert := newSamlTestKeyPair(t)

	app := newSamlTestApp()
	app.Slug = "test-idp"
	app.SamlMetadata = samlSPMetadata(spCert)

	user := newSamlTestUser()

	sessions := &samlSLOSessionRepo{revoked: 2}
	uow := samlSLOTestUoW{
		apps:     &samlSLOAppRepo{apps: []*applicationdomain.Application{app}},
		users:    &samlSLOUserRepo{users: []*userdomain.User{user}},
		sessions: sessions,
		certs:    &samlTestCertRepo{certs: []*certdomain.Cert{{Certificate: samlTestCertPEM(idpCert), PrivateKey: samlTestKeyPEM(idpKey)}}},
	}
	svc := Service{
		Clock:         fakeClock{fixedTime: now},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "logout-response-1"},
		UOW:           uow,
	}
	return svc, sessions, spKey, spCert
}

// SAML SLO must revoke every session of the verified subject and must not
// exclude any session: it passes an empty excludeSessionID down to the
// repository, which is exactly the path that used to send "" into a uuid cast.
func TestSAML_HandleIdPInitiatedSLO_RevokesAllSessionsWithoutExclusion(t *testing.T) {
	svc, repo, spKey, spCert := newSamlSLOFixture(t)
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)

	response, err := svc.HandleIdPInitiatedSLO(context.Background(),
		signedSAMLLogoutRequest(t, spKey, spCert, "samluser@example.com", crypto.SHA256))
	require.NoError(t, err)
	require.NotEmpty(t, response)

	require.Equal(t, 1, repo.calls, "SLO 必须调用一次会话撤销")
	assert.Equal(t, samlTestUserID, repo.userID, "必须撤销验签主体的会话，而不是 NameID 原文")
	assert.Empty(t, repo.exclude, "SAML SLO 撤销全部会话，不得排除任何会话")
	assert.Equal(t, now, repo.revokedAt)

	decoded, err := base64.StdEncoding.DecodeString(response)
	require.NoError(t, err)
	xmlStr := string(decoded)
	assert.Contains(t, xmlStr, "urn:oasis:names:tc:SAML:2.0:status:Success")
	assert.Contains(t, xmlStr, `InResponseTo="logout-1"`, "InResponseTo 必须回显 LogoutRequest 的 ID")
	assert.Contains(t, xmlStr, `ID="_logout-response-1"`, "Response ID 必须是 NCName")
	assert.Contains(t, xmlStr, "Signature", "LogoutResponse 必须签名")
}

func TestSAML_HandleIdPInitiatedSLO_RejectsUnsignedRequest(t *testing.T) {
	svc, repo, _, _ := newSamlSLOFixture(t)

	_, err := svc.HandleIdPInitiatedSLO(context.Background(), unsignedSAMLLogoutRequest("samluser@example.com"))
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnauthorized, apperrors.FromError(err).Code)
	assert.Zero(t, repo.calls, "未验签请求不得触发任何会话撤销")
}

func TestSAML_HandleIdPInitiatedSLO_RejectsUnknownIssuer(t *testing.T) {
	svc, repo, spKey, spCert := newSamlSLOFixture(t)
	// The fixture application registers entityID "test-idp"; re-sign with a
	// different Issuer to prove the entityID binding.
	root := &etree.Element{Space: "samlp", Tag: "LogoutRequest"}
	root.CreateAttr("xmlns:samlp", "urn:oasis:names:tc:SAML:2.0:protocol")
	root.CreateAttr("xmlns:saml", "urn:oasis:names:tc:SAML:2.0:assertion")
	root.CreateAttr("ID", "logout-1")
	root.CreateAttr("Version", "2.0")
	root.CreateElement("saml:Issuer").SetText("unknown-idp")
	root.CreateElement("saml:NameID").SetText("samluser@example.com")
	signingContext, err := dsig.NewSigningContext(spKey, [][]byte{spCert.Raw})
	require.NoError(t, err)
	signingContext.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")
	signed, err := signingContext.SignEnveloped(root)
	require.NoError(t, err)
	doc := etree.NewDocument()
	doc.SetRoot(signed)
	xmlStr, err := doc.WriteToString()
	require.NoError(t, err)

	_, err = svc.HandleIdPInitiatedSLO(context.Background(), base64.StdEncoding.EncodeToString([]byte(xmlStr)))
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnauthorized, apperrors.FromError(err).Code)
	assert.Zero(t, repo.calls)
}

func TestSAML_HandleIdPInitiatedSLO_RejectsTamperedNameID(t *testing.T) {
	svc, repo, spKey, spCert := newSamlSLOFixture(t)

	encoded := signedSAMLLogoutRequest(t, spKey, spCert, "samluser@example.com", crypto.SHA256)
	raw, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	// Swap the signed subject for another account; the signature must fail
	// before any user lookup or revocation happens.
	tampered := strings.ReplaceAll(string(raw), "samluser@example.com", "victim@example.com")

	_, err = svc.HandleIdPInitiatedSLO(context.Background(), base64.StdEncoding.EncodeToString([]byte(tampered)))
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnauthorized, apperrors.FromError(err).Code)
	assert.Zero(t, repo.calls)
}

func TestSAML_HandleIdPInitiatedSLO_RejectsSHA1Signature(t *testing.T) {
	svc, repo, spKey, spCert := newSamlSLOFixture(t)

	_, err := svc.HandleIdPInitiatedSLO(context.Background(),
		signedSAMLLogoutRequest(t, spKey, spCert, "samluser@example.com", crypto.SHA1))
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnauthorized, apperrors.FromError(err).Code)
	assert.Zero(t, repo.calls)
}

func TestSAML_HandleIdPInitiatedSLO_RejectsUnknownSubject(t *testing.T) {
	svc, repo, spKey, spCert := newSamlSLOFixture(t)

	_, err := svc.HandleIdPInitiatedSLO(context.Background(),
		signedSAMLLogoutRequest(t, spKey, spCert, "nobody@example.com", crypto.SHA256))
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnauthorized, apperrors.FromError(err).Code)
	assert.Zero(t, repo.calls, "主体解析失败不得撤销任何会话")
}
