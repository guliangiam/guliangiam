package applications

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func resetFakeClientCertificateStore() {
	fakeClientCertificateStore = map[string][]*domain.ClientCertificate{}
	fakeClientSecretStore = map[string][]*domain.ClientSecret{}
}

type testClientCertificate struct {
	certificate *x509.Certificate
	pemText     string
	fingerprint string
}

func newTestClientCertificate(t *testing.T) testClientCertificate {
	t.Helper()
	key := jwksTestRSAKey(t)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "client.example.test"},
		Issuer:                pkix.Name{CommonName: "client.example.test"},
		NotBefore:             time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:              time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	certificate, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	digest := sha256.Sum256(der)
	return testClientCertificate{
		certificate: certificate,
		pemText:     string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		fingerprint: base64.RawURLEncoding.EncodeToString(digest[:]),
	}
}

func newClientCertificateService(t *testing.T, method string) (Service, *mockApplicationRepo, *redlineRecords, *redlineOutbox) {
	t.Helper()
	resetFakeClientCertificateStore()
	svc, repo, records, outbox := newJWKService(t)
	repo.apps = append(repo.apps, &domain.Application{
		ID:                      "mtls-app-id",
		OrganizationID:          "org-1",
		ClientID:                "gl_cid_mtls",
		TokenEndpointAuthMethod: method,
		Revision:                1,
	})
	return svc, repo, records, outbox
}

func assertUnprocessable(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
}

func TestReplaceClientCertificatesStoresFingerprintAndAuditsPrefixOnly(t *testing.T) {
	svc, _, records, outbox := newClientCertificateService(t, domain.TokenEndpointAuthMethodSelfSignedTLSClientAuth)
	certificate := newTestClientCertificate(t)

	err := svc.ReplaceClientCertificates(context.Background(), ReplaceClientCertificatesCommand{
		ID:             "mtls-app-id",
		OrganizationID: "org-1",
		ActorID:        "actor-1",
		Certificates: []ClientCertificateInput{{
			FingerprintSHA256: certificate.fingerprint,
			Certificate:       certificate.pemText,
			SelfSigned:        true,
		}},
	})
	require.NoError(t, err)

	active := fakeClientCertificateStore[fakeClientSecretKey("org-1", "mtls-app-id")]
	require.Len(t, active, 1)
	assert.Equal(t, certificate.fingerprint, active[0].FingerprintSHA256)
	assert.Equal(t, domain.TokenEndpointAuthMethodSelfSignedTLSClientAuth, active[0].AuthMethod)

	require.Len(t, records.created, 1)
	require.Len(t, outbox.events, 1)
	assert.Equal(t, "application.client_certificate.replaced", outbox.events[0].EventType)

	payload := decodeRedlinePayload(t, outbox.events[0])
	assert.Equal(t, float64(1), payload["count"])
	assert.Equal(t, float64(1), payload["selfSignedCount"])
	prefixes, ok := payload["fingerprintPrefixes"].([]any)
	require.True(t, ok)
	require.Len(t, prefixes, 1)
	assert.Equal(t, certificate.fingerprint[:12], prefixes[0])

	raw := string(outbox.events[0].Payload)
	assert.NotContains(t, raw, certificate.fingerprint, "审计不得包含完整指纹")
	assert.NotContains(t, raw, "BEGIN CERTIFICATE", "审计不得包含证书原文")
}

func TestReplaceClientCertificatesRejectsFingerprintMismatch(t *testing.T) {
	svc, _, records, outbox := newClientCertificateService(t, domain.TokenEndpointAuthMethodSelfSignedTLSClientAuth)
	certificate := newTestClientCertificate(t)

	err := svc.ReplaceClientCertificates(context.Background(), ReplaceClientCertificatesCommand{
		ID:             "mtls-app-id",
		OrganizationID: "org-1",
		ActorID:        "actor-1",
		Certificates: []ClientCertificateInput{{
			FingerprintSHA256: base64.RawURLEncoding.EncodeToString(make([]byte, 32)),
			Certificate:       certificate.pemText,
			SelfSigned:        true,
		}},
	})
	assertUnprocessable(t, err)
	assert.Empty(t, fakeClientCertificateStore[fakeClientSecretKey("org-1", "mtls-app-id")])
	assert.Empty(t, records.created)
	assert.Empty(t, outbox.events)
}

func TestReplaceClientCertificatesRejectsFingerprintOnly(t *testing.T) {
	svc, _, _, _ := newClientCertificateService(t, domain.TokenEndpointAuthMethodSelfSignedTLSClientAuth)
	certificate := newTestClientCertificate(t)

	err := svc.ReplaceClientCertificates(context.Background(), ReplaceClientCertificatesCommand{
		ID:             "mtls-app-id",
		OrganizationID: "org-1",
		ActorID:        "actor-1",
		Certificates: []ClientCertificateInput{{
			FingerprintSHA256: certificate.fingerprint,
			SelfSigned:        true,
		}},
	})
	assertUnprocessable(t, err)
}

func TestReplaceClientCertificatesRejectsPrivateKeyMaterial(t *testing.T) {
	svc, _, _, _ := newClientCertificateService(t, domain.TokenEndpointAuthMethodSelfSignedTLSClientAuth)
	certificate := newTestClientCertificate(t)
	leaked := certificate.pemText + "-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n"

	err := svc.ReplaceClientCertificates(context.Background(), ReplaceClientCertificatesCommand{
		ID:             "mtls-app-id",
		OrganizationID: "org-1",
		ActorID:        "actor-1",
		Certificates: []ClientCertificateInput{{
			FingerprintSHA256: certificate.fingerprint,
			Certificate:       leaked,
			SelfSigned:        true,
		}},
	})
	assertUnprocessable(t, err)
}

func TestReplaceClientCertificatesRejectsSelfSignedFlagMismatch(t *testing.T) {
	svc, _, _, _ := newClientCertificateService(t, domain.TokenEndpointAuthMethodTLSClientAuth)
	certificate := newTestClientCertificate(t)

	err := svc.ReplaceClientCertificates(context.Background(), ReplaceClientCertificatesCommand{
		ID:             "mtls-app-id",
		OrganizationID: "org-1",
		ActorID:        "actor-1",
		Certificates: []ClientCertificateInput{{
			FingerprintSHA256: certificate.fingerprint,
			Certificate:       certificate.pemText,
			SelfSigned:        true,
		}},
	})
	assertUnprocessable(t, err)
}

func TestReplaceClientCertificatesRejectsNonMTLSMethod(t *testing.T) {
	svc, _, _, _ := newClientCertificateService(t, domain.TokenEndpointAuthMethodClientSecretBasic)
	certificate := newTestClientCertificate(t)

	err := svc.ReplaceClientCertificates(context.Background(), ReplaceClientCertificatesCommand{
		ID:             "mtls-app-id",
		OrganizationID: "org-1",
		ActorID:        "actor-1",
		Certificates: []ClientCertificateInput{{
			FingerprintSHA256: certificate.fingerprint,
			Certificate:       certificate.pemText,
			SelfSigned:        false,
		}},
	})
	assertUnprocessable(t, err)
}

func TestReplaceClientCertificatesIsTenantScoped(t *testing.T) {
	svc, _, _, _ := newClientCertificateService(t, domain.TokenEndpointAuthMethodSelfSignedTLSClientAuth)
	certificate := newTestClientCertificate(t)

	err := svc.ReplaceClientCertificates(context.Background(), ReplaceClientCertificatesCommand{
		ID:             "mtls-app-id",
		OrganizationID: "org-2",
		ActorID:        "actor-1",
		Certificates: []ClientCertificateInput{{
			FingerprintSHA256: certificate.fingerprint,
			Certificate:       certificate.pemText,
			SelfSigned:        true,
		}},
	})
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.Empty(t, fakeClientCertificateStore[fakeClientSecretKey("org-1", "mtls-app-id")])
}

func TestListClientCertificatesOmitsCertificateBody(t *testing.T) {
	svc, _, _, _ := newClientCertificateService(t, domain.TokenEndpointAuthMethodSelfSignedTLSClientAuth)
	certificate := newTestClientCertificate(t)
	require.NoError(t, svc.ReplaceClientCertificates(context.Background(), ReplaceClientCertificatesCommand{
		ID:             "mtls-app-id",
		OrganizationID: "org-1",
		ActorID:        "actor-1",
		Certificates: []ClientCertificateInput{{
			FingerprintSHA256: certificate.fingerprint,
			Certificate:       certificate.pemText,
			SelfSigned:        true,
		}},
	}))

	views, err := svc.ListClientCertificates(context.Background(), "org-1", "mtls-app-id")
	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.Equal(t, certificate.fingerprint, views[0].FingerprintSHA256)
	assert.True(t, views[0].SelfSigned)
	require.NotNil(t, views[0].NotAfter)

	raw, err := json.Marshal(views)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "BEGIN CERTIFICATE")
	assert.NotContains(t, string(raw), "certificatePem")
}

// ── create / PATCH 的证书闭环 ────────────────────────────────────────────────

func newMTLSPatchService(t *testing.T, method, secretHash string, grants []string) (Service, *mockApplicationRepo, *redlineOutbox) {
	t.Helper()
	resetFakeClientCertificateStore()
	svc, repo, _, outbox := newJWKService(t)
	repo.apps = append(repo.apps, &domain.Application{
		ID:                      "mtls-app-id",
		OrganizationID:          "org-1",
		ClientID:                "gl_cid_mtls",
		TokenEndpointAuthMethod: method,
		ClientSecretHash:        secretHash,
		GrantTypes:              grants,
		Revision:                1,
	})
	return svc, repo, outbox
}

func findOutboxEventByType(t *testing.T, outbox *redlineOutbox, eventType string) *domainoutbox.OutboxEvent {
	t.Helper()
	for _, event := range outbox.events {
		if event.EventType == eventType {
			return event
		}
	}
	t.Fatalf("outbox event %q not found (events = %d)", eventType, len(outbox.events))
	return nil
}

func TestCreateApplicationRequiresClientCertificatesForMTLS(t *testing.T) {
	resetFakeClientCertificateStore()
	svc, repo, records, outbox := newJWKService(t)

	_, err := svc.CreateApplication(context.Background(), CreateApplicationCommand{
		OrganizationID:          "org-1",
		ActorID:                 "actor-1",
		Slug:                    "mtls-no-cert",
		DisplayName:             "mTLS No Cert",
		Status:                  "active",
		TokenEndpointAuthMethod: domain.TokenEndpointAuthMethodTLSClientAuth,
		GrantTypes:              []string{grantTypeClientCredentials},
	})
	appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
	assert.Equal(t, "clientCertificates", appErr.Meta["field"])
	assert.Equal(t, "non_empty_client_certificates", appErr.Meta["expected"])
	assert.Empty(t, repo.apps, "422 发生在事务之前，不得创建应用")
	assert.Empty(t, records.created)
	assert.Empty(t, outbox.events)
}

func TestCreateApplicationWritesClientCertificatesInTransaction(t *testing.T) {
	resetFakeClientCertificateStore()
	svc, repo, records, outbox := newJWKService(t)
	certificate := newTestClientCertificate(t)

	result, err := svc.CreateApplication(context.Background(), CreateApplicationCommand{
		OrganizationID:          "org-1",
		ActorID:                 "actor-1",
		Slug:                    "mtls-app",
		DisplayName:             "mTLS App",
		Status:                  "active",
		TokenEndpointAuthMethod: domain.TokenEndpointAuthMethodSelfSignedTLSClientAuth,
		GrantTypes:              []string{grantTypeClientCredentials},
		ClientCertificates: []ClientCertificateInput{{
			FingerprintSHA256: certificate.fingerprint,
			Certificate:       certificate.pemText,
			SelfSigned:        true,
		}},
	})
	require.NoError(t, err)
	require.Len(t, repo.apps, 1)
	assert.Equal(t, domain.TokenEndpointAuthMethodSelfSignedTLSClientAuth, result.Application.TokenEndpointAuthMethod)
	assert.Empty(t, result.ClientSecretPlain, "mTLS 不应生成 client secret")

	active := fakeClientCertificateStore[fakeClientSecretKey("org-1", result.Application.ID)]
	require.Len(t, active, 1, "证书必须与 application 同事务落库")
	assert.Equal(t, certificate.fingerprint, active[0].FingerprintSHA256)
	assert.Equal(t, domain.TokenEndpointAuthMethodSelfSignedTLSClientAuth, active[0].AuthMethod)
	assert.Equal(t, domain.ClientCertificateStatusActive, active[0].Status)

	assert.Len(t, records.created, 2, "证书替换 + 应用创建各一条审计")
	event := findOutboxEventByType(t, outbox, "application.client_certificate.replaced")
	payload := decodeRedlinePayload(t, event)
	assert.Equal(t, float64(1), payload["count"])
	assert.Equal(t, float64(1), payload["selfSignedCount"])
	assert.NotContains(t, string(event.Payload), certificate.fingerprint, "审计不得包含完整指纹")
	assert.NotContains(t, string(event.Payload), "BEGIN CERTIFICATE", "审计不得包含证书原文")
}

func TestCreateApplicationRejectsCertificatesForNonMTLSMethod(t *testing.T) {
	resetFakeClientCertificateStore()
	svc, repo, _, _ := newJWKService(t)
	certificate := newTestClientCertificate(t)

	_, err := svc.CreateApplication(context.Background(), CreateApplicationCommand{
		OrganizationID:          "org-1",
		ActorID:                 "actor-1",
		Slug:                    "secret-app",
		DisplayName:             "Secret App",
		Status:                  "active",
		TokenEndpointAuthMethod: domain.TokenEndpointAuthMethodClientSecretBasic,
		ClientSecret:            "secret-0123456789",
		GrantTypes:              []string{grantTypeClientCredentials},
		ClientCertificates: []ClientCertificateInput{{
			FingerprintSHA256: certificate.fingerprint,
			Certificate:       certificate.pemText,
		}},
	})
	assertUnprocessable(t, err)
	assert.Empty(t, repo.apps)
	assert.Empty(t, fakeClientCertificateStore)
}

func TestPatchApplicationRequiresClientCertificatesWhenSwitchingToMTLS(t *testing.T) {
	svc, repo, outbox := newMTLSPatchService(t, domain.TokenEndpointAuthMethodClientSecretBasic, "legacy-hash", []string{grantTypeClientCredentials})
	method := domain.TokenEndpointAuthMethodTLSClientAuth

	err := svc.PatchApplication(context.Background(), PatchApplicationCommand{
		ID:                      "mtls-app-id",
		OrganizationID:          "org-1",
		ActorID:                 "actor-1",
		TokenEndpointAuthMethod: &method,
		Revision:                1,
	})
	appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
	assert.Equal(t, "clientCertificates", appErr.Meta["field"])
	assert.Equal(t, "non_empty_client_certificates", appErr.Meta["expected"])
	assert.Equal(t, domain.TokenEndpointAuthMethodClientSecretBasic, repo.apps[0].TokenEndpointAuthMethod, "方法切换必须整体回滚")
	assert.Empty(t, outbox.events)
	assert.Empty(t, fakeClientCertificateStore)
}

func TestPatchApplicationWritesClientCertificatesWithMethod(t *testing.T) {
	svc, repo, outbox := newMTLSPatchService(t, domain.TokenEndpointAuthMethodClientSecretBasic, "legacy-hash", []string{grantTypeClientCredentials})
	certificate := newTestClientCertificate(t)
	method := domain.TokenEndpointAuthMethodTLSClientAuth
	inputs := []ClientCertificateInput{{
		FingerprintSHA256: certificate.fingerprint,
		Certificate:       certificate.pemText,
	}}

	err := svc.PatchApplication(context.Background(), PatchApplicationCommand{
		ID:                      "mtls-app-id",
		OrganizationID:          "org-1",
		ActorID:                 "actor-1",
		TokenEndpointAuthMethod: &method,
		ClientCertificates:      &inputs,
		Revision:                1,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.TokenEndpointAuthMethodTLSClientAuth, repo.apps[0].TokenEndpointAuthMethod)

	active := fakeClientCertificateStore[fakeClientSecretKey("org-1", "mtls-app-id")]
	require.Len(t, active, 1)
	assert.Equal(t, domain.TokenEndpointAuthMethodTLSClientAuth, active[0].AuthMethod)
	assert.Equal(t, certificate.fingerprint, active[0].FingerprintSHA256)

	event := findOutboxEventByType(t, outbox, "application.client_certificate.replaced")
	assert.NotContains(t, string(event.Payload), certificate.fingerprint)
	findOutboxEventByType(t, outbox, "application.updated")
}

func TestPatchApplicationRejectsCertificatesForNonMTLSMethod(t *testing.T) {
	svc, repo, outbox := newMTLSPatchService(t, domain.TokenEndpointAuthMethodClientSecretPost, "legacy-hash", []string{grantTypeClientCredentials})
	certificate := newTestClientCertificate(t)
	inputs := []ClientCertificateInput{{
		FingerprintSHA256: certificate.fingerprint,
		Certificate:       certificate.pemText,
	}}

	err := svc.PatchApplication(context.Background(), PatchApplicationCommand{
		ID:                 "mtls-app-id",
		OrganizationID:     "org-1",
		ActorID:            "actor-1",
		ClientCertificates: &inputs,
		Revision:           1,
	})
	assertUnprocessable(t, err)
	assert.Equal(t, domain.TokenEndpointAuthMethodClientSecretPost, repo.apps[0].TokenEndpointAuthMethod)
	assert.Empty(t, fakeClientCertificateStore)
	assert.Empty(t, outbox.events)
}

func TestPatchApplicationRejectsPublicClientCredentialGrant(t *testing.T) {
	t.Run("client_credentials_forbidden", func(t *testing.T) {
		svc, repo, _ := newMTLSPatchService(t, domain.TokenEndpointAuthMethodNone, "", []string{grantTypeAuthorizationCode})
		grants := []string{grantTypeAuthorizationCode, grantTypeClientCredentials}

		err := svc.PatchApplication(context.Background(), PatchApplicationCommand{
			ID:             "mtls-app-id",
			OrganizationID: "org-1",
			ActorID:        "actor-1",
			GrantTypes:     &grants,
			Revision:       1,
		})
		appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
		assert.Equal(t, "grantTypes", appErr.Meta[metaKeyField])
		assert.Equal(t, grantTypeClientCredentials, appErr.Meta[metaKeyValue])
		assert.Equal(t, []string{grantTypeAuthorizationCode}, repo.apps[0].GrantTypes)
	})

	t.Run("authorization_code_required", func(t *testing.T) {
		svc, repo, _ := newMTLSPatchService(t, domain.TokenEndpointAuthMethodNone, "", []string{grantTypeAuthorizationCode})
		grants := []string{"urn:ietf:params:oauth:grant-type:token-exchange"}

		err := svc.PatchApplication(context.Background(), PatchApplicationCommand{
			ID:             "mtls-app-id",
			OrganizationID: "org-1",
			ActorID:        "actor-1",
			GrantTypes:     &grants,
			Revision:       1,
		})
		appErr := assertAppError(t, err, apperrors.CodeUnprocessableEntity)
		assert.Equal(t, "grantTypes", appErr.Meta[metaKeyField])
		assert.Equal(t, grantTypeAuthorizationCode, appErr.Meta["expected"])
		assert.Equal(t, []string{grantTypeAuthorizationCode}, repo.apps[0].GrantTypes)
	})
}

func TestPatchApplicationAllowsLegacyPublicClientDisplayNameEdit(t *testing.T) {
	// 历史遗留的 none + client_credentials 组合不可用，但编辑无关字段不应被
	// 新规则阻断：只有触碰方法/授权类型时才强制闭环。
	svc, repo, outbox := newMTLSPatchService(t, domain.TokenEndpointAuthMethodNone, "", []string{grantTypeClientCredentials})
	displayName := "Renamed Legacy"

	err := svc.PatchApplication(context.Background(), PatchApplicationCommand{
		ID:             "mtls-app-id",
		OrganizationID: "org-1",
		ActorID:        "actor-1",
		DisplayName:    &displayName,
		Revision:       1,
	})
	require.NoError(t, err)
	assert.Equal(t, displayName, repo.apps[0].DisplayName)
	require.Len(t, outbox.events, 1)
	assert.Equal(t, "application.updated", outbox.events[0].EventType)
}
