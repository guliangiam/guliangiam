package auth

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"strings"
	"time"

	"github.com/beevik/etree"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	dsig "github.com/russellhaering/goxmldsig"
)

// HandleIdPInitiatedSLO processes a SAML LogoutRequest sent by a registered SP.
// The request must carry a valid XML signature against the certificate pinned
// in the application's SP metadata; the subject is read from the verified
// element, all of the subject's sessions are revoked in one transaction with
// the audit record, and a signed LogoutResponse is returned. Every failure is
// returned to the caller — an unverifiable request never yields Success.
func (s *Service) HandleIdPInitiatedSLO(ctx context.Context, samlRequest string) (string, error) {
	raw, err := decodeSAMLMessage(samlRequest)
	if err != nil {
		return "", err
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromString(string(raw)); err != nil {
		return "", apperrors.NewInvalidArgument("invalid SAML LogoutRequest XML")
	}
	root := doc.Root()
	if root == nil || root.Tag != "LogoutRequest" {
		return "", apperrors.NewInvalidArgument("SAML document is not a LogoutRequest")
	}
	requestID := strings.TrimSpace(root.SelectAttrValue("ID", ""))
	issuerEl := samlFirstChild(root, "Issuer")
	if requestID == "" || issuerEl == nil || strings.TrimSpace(issuerEl.Text()) == "" {
		return "", apperrors.NewInvalidArgument("SAML LogoutRequest requires ID and Issuer")
	}

	app, err := s.findApplicationByEntityID(ctx, strings.TrimSpace(issuerEl.Text()))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(app.SamlMetadata) == "" {
		return "", apperrors.NewUnauthorized("application has no SAML SP metadata for logout verification")
	}
	spCert, err := samlSigningCertificate(app.SamlMetadata)
	if err != nil {
		return "", apperrors.WrapUnauthorized(err, "extract SP signing certificate")
	}
	signed, err := verifySamlLogoutRequestSignature(root, spCert)
	if err != nil {
		return "", err
	}

	// The subject must come from the element the signature actually covered,
	// never from an independently parsed copy of the document.
	nameIDEl := samlFirstDescendant(signed, "NameID")
	if nameIDEl == nil || strings.TrimSpace(nameIDEl.Text()) == "" {
		return "", apperrors.NewInvalidArgument("SAML LogoutRequest has no NameID")
	}
	user, err := findUserByNameID(ctx, s.UOW, app.OrganizationID, strings.TrimSpace(nameIDEl.Text()))
	if err != nil {
		return "", apperrors.WrapUnauthorized(err, "resolve SAML logout subject")
	}

	now := s.Clock.Now().UTC()
	var revoked int64
	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		count, revokeErr := tx.Sessions().RevokeByUserExcept(ctx, user.ID, "", now)
		if revokeErr != nil {
			return apperrors.WrapInternalf(revokeErr, "revoke sessions for SAML SLO user %s", user.ID)
		}
		revoked = count
		return s.embedAuditEvent(tx, ctx,
			"", "", string(domainrecords.SurfaceProtocol), "", "",
			app.OrganizationID, user.ID, "user.saml_slo", "user", user.ID,
			"user", audit.EventAuthLogout,
			map[string]any{"revokedSessions": revoked, "applicationId": app.ID})
	}); err != nil {
		return "", err
	}

	responseID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return "", apperrors.WrapInternal(err, "generate SAML logout response id")
	}
	response := buildSamlLogoutResponse("_"+responseID, requestID, app.Slug, now)
	key := s.resolveApplicationPrivateKey(ctx, app)
	cert := s.resolveApplicationCert(ctx, app)
	signedXML, err := signSamlResponseWithKey(response, key, cert)
	if err != nil {
		return "", apperrors.WrapInternal(err, "sign SAML logout response")
	}
	return base64.StdEncoding.EncodeToString([]byte(signedXML)), nil
}

// findApplicationByEntityID resolves the SP application whose entityID (Slug)
// equals the LogoutRequest Issuer. The scan runs inside the caller's tenant
// scope; an entityID outside that scope is indistinguishable from an unknown
// one and fails closed.
func (s *Service) findApplicationByEntityID(ctx context.Context, entityID string) (*applicationdomain.Application, error) {
	apps, err := platformpagination.CollectAll(func(page, pageSize int) ([]*applicationdomain.Application, int64, error) {
		return s.UOW.Applications().List(ctx, applicationdomain.ListQuery{Page: page, PageSize: pageSize})
	})
	if err != nil {
		return nil, apperrors.WrapInternal(err, "list applications for SAML logout issuer resolution")
	}
	for _, app := range apps {
		if app != nil && app.Slug == entityID {
			return app, nil
		}
	}
	return nil, apperrors.NewUnauthorized("no application registered for SAML logout issuer")
}

// samlLogoutCertStore pins the single SP certificate used to verify a
// LogoutRequest signature.
type samlLogoutCertStore struct {
	cert *x509.Certificate
}

func (s samlLogoutCertStore) Certificates() ([]*x509.Certificate, error) {
	return []*x509.Certificate{s.cert}, nil
}

// verifySamlLogoutRequestSignature mirrors the infra/saml verification
// discipline: reject SHA-1 signature/digest algorithms, then validate the
// enveloped signature against the pinned SP certificate.
func verifySamlLogoutRequestSignature(root *etree.Element, cert *x509.Certificate) (*etree.Element, error) {
	sig := samlSignatureElement(root)
	if sig == nil {
		return nil, apperrors.NewUnauthorized("SAML LogoutRequest is not signed")
	}
	if err := rejectSamlSHA1Algorithms(sig); err != nil {
		return nil, err
	}
	validationContext := dsig.NewDefaultValidationContext(samlLogoutCertStore{cert: cert})
	signed, err := validationContext.Validate(root)
	if err != nil {
		return nil, apperrors.WrapUnauthorized(err, "SAML logout signature verification failed")
	}
	if signed == nil {
		return nil, apperrors.NewUnauthorized("SAML logout signature verification returned no signed element")
	}
	return signed, nil
}

func rejectSamlSHA1Algorithms(sig *etree.Element) error {
	signedInfo := samlFirstChild(sig, "SignedInfo")
	if signedInfo == nil {
		return apperrors.NewUnauthorized("SAML signature has no SignedInfo")
	}
	if method := samlFirstChild(signedInfo, "SignatureMethod"); method != nil {
		if isSamlSHA1Algorithm(method.SelectAttrValue("Algorithm", "")) {
			return apperrors.NewUnauthorized("SAML signature uses the disabled SHA-1 algorithm")
		}
	}
	for _, ref := range signedInfo.ChildElements() {
		if ref.Tag != "Reference" {
			continue
		}
		if digest := samlFirstChild(ref, "DigestMethod"); digest != nil {
			if isSamlSHA1Algorithm(digest.SelectAttrValue("Algorithm", "")) {
				return apperrors.NewUnauthorized("SAML signature digest uses the disabled SHA-1 algorithm")
			}
		}
	}
	return nil
}

func isSamlSHA1Algorithm(algorithm string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(algorithm)), "sha1")
}

// samlSignatureElement returns the first direct <ds:Signature> child of el.
func samlSignatureElement(el *etree.Element) *etree.Element {
	if el == nil {
		return nil
	}
	for _, child := range el.ChildElements() {
		if child.Tag != "Signature" {
			continue
		}
		if child.Space == "ds" || child.SelectAttrValue("xmlns", "") == dsig.Namespace {
			return child
		}
	}
	return nil
}

func samlFirstChild(el *etree.Element, localName string) *etree.Element {
	if el == nil {
		return nil
	}
	for _, child := range el.ChildElements() {
		if child.Tag == localName {
			return child
		}
	}
	return nil
}

func samlFirstDescendant(el *etree.Element, localName string) *etree.Element {
	if el == nil {
		return nil
	}
	for _, child := range el.ChildElements() {
		if child.Tag == localName {
			return child
		}
		if found := samlFirstDescendant(child, localName); found != nil {
			return found
		}
	}
	return nil
}

// buildSamlLogoutResponse constructs the LogoutResponse element; ID is an
// NCName ("_" prefix) and InResponseTo echoes the LogoutRequest ID.
func buildSamlLogoutResponse(id, inResponseTo, issuer string, now time.Time) *etree.Element {
	el := &etree.Element{Space: "samlp", Tag: "LogoutResponse"}
	el.CreateAttr("xmlns:samlp", "urn:oasis:names:tc:SAML:2.0:protocol")
	el.CreateAttr("xmlns:saml", "urn:oasis:names:tc:SAML:2.0:assertion")
	el.CreateAttr("ID", id)
	el.CreateAttr("Version", "2.0")
	el.CreateAttr("IssueInstant", now.Format(time.RFC3339))
	el.CreateAttr("InResponseTo", inResponseTo)
	el.CreateElement("saml:Issuer").SetText(issuer)
	status := el.CreateElement("samlp:Status")
	status.CreateElement("samlp:StatusCode").CreateAttr("Value", "urn:oasis:names:tc:SAML:2.0:status:Success")
	return el
}
