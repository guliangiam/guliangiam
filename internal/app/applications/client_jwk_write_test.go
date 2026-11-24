package applications

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// 本文件覆盖 private_key_jwt 的写入闭环：内嵌 JWKS 与 application/审计同事务
// 落库；payload 只记录 kid 数量与 kid 列表，绝不含 JWK 公钥内容或私钥参数。
// 同时覆盖 DCR 对 jwks_uri / 非法 JWKS 的 400 契约。

var (
	jwksTestRSAOnce sync.Once
	jwksTestRSA     *rsa.PrivateKey
	jwksTestRSAErr  error
)

func jwksTestRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	jwksTestRSAOnce.Do(func() {
		jwksTestRSA, jwksTestRSAErr = rsa.GenerateKey(rand.Reader, 2048)
	})
	require.NoError(t, jwksTestRSAErr)
	return jwksTestRSA
}

func publicRSAJWK(t *testing.T, key *rsa.PrivateKey, kid string) map[string]any {
	t.Helper()
	return map[string]any{
		"kty": "RSA",
		"kid": kid,
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}
}

func jwksDocument(t *testing.T, keys ...map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"keys": keys})
	require.NoError(t, err)
	return raw
}

// jwkTx/jwkUoW 只在事务成功时提交审计与 Outbox，用来证明回滚语义。
type jwkTx struct {
	appuow.UnitOfWork
	apps    domain.Repository
	orgs    orgdomain.Repository
	records *redlineRecords
	outbox  *redlineOutbox
}

func (u *jwkTx) Applications() domain.Repository     { return u.apps }
func (u *jwkTx) Organizations() orgdomain.Repository { return u.orgs }
func (u *jwkTx) Records() recordsdomain.Repository   { return u.records }
func (u *jwkTx) Outbox() domainoutbox.Repository     { return u.outbox }

type jwkUoW struct {
	appuow.UnitOfWork
	apps    domain.Repository
	orgs    orgdomain.Repository
	records *redlineRecords
	outbox  *redlineOutbox
}

func (u *jwkUoW) Applications() domain.Repository     { return u.apps }
func (u *jwkUoW) Organizations() orgdomain.Repository { return u.orgs }
func (u *jwkUoW) Records() recordsdomain.Repository   { return u.records }
func (u *jwkUoW) Outbox() domainoutbox.Repository     { return u.outbox }

func (u *jwkUoW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	tx := &jwkTx{apps: u.apps, orgs: u.orgs, records: &redlineRecords{}, outbox: &redlineOutbox{}}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	u.records.created = append(u.records.created, tx.records.created...)
	u.outbox.events = append(u.outbox.events, tx.outbox.events...)
	return nil
}

func resetFakeJWKStores() {
	fakeClientJWKStore = map[string][]*domain.ClientJWK{}
	fakeClientSecretStore = map[string][]*domain.ClientSecret{}
}

func newJWKService(t *testing.T) (Service, *mockApplicationRepo, *redlineRecords, *redlineOutbox) {
	t.Helper()
	repo := &mockApplicationRepo{}
	records := &redlineRecords{}
	outbox := &redlineOutbox{}
	uow := &jwkUoW{apps: repo, orgs: &appTestOrgRepo{orgs: []*orgdomain.Organization{{ID: "org-1", Slug: "built-in"}}}, records: records, outbox: outbox}
	svc := NewService(
		fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		&fakeUUIDGenerator{fixedID: "jwk-app-id", altID: "jwk-row-id"},
		uow,
	)
	return svc, repo, records, outbox
}

func activeJWKs(organizationID, applicationID string) []*domain.ClientJWK {
	all := fakeClientJWKStore[fakeClientSecretKey(organizationID, applicationID)]
	active := make([]*domain.ClientJWK, 0, len(all))
	for _, key := range all {
		if key.Status == domain.ClientJWKStatusActive {
			active = append(active, key)
		}
	}
	return active
}

func assertPayloadHasNoKeyMaterial(t *testing.T, event *domainoutbox.OutboxEvent) map[string]any {
	t.Helper()
	payload := decodeRedlinePayload(t, event)
	for _, forbidden := range []string{"jwks", "keys", "jwk", "kidValues", "n", "e", "d"} {
		_, exists := payload[forbidden]
		assert.Falsef(t, exists, "payload 不得包含字段 %q", forbidden)
	}
	assert.NotContains(t, string(event.Payload), "\"n\":", "payload 不得回显 JWK 模数")
	return payload
}

func TestCreateApplicationWithPrivateKeyJWTStoresJWKS(t *testing.T) {
	resetFakeJWKStores()
	svc, _, records, outbox := newJWKService(t)
	key := jwksTestRSAKey(t)

	result, err := svc.CreateApplication(context.Background(), CreateApplicationCommand{
		OrganizationID:          "org-1",
		ActorID:                 "actor-1",
		Slug:                    "jwt-app",
		DisplayName:             "JWT Application",
		AppType:                 "machine",
		Status:                  "active",
		TokenEndpointAuthMethod: "private_key_jwt",
		GrantTypes:              []string{"client_credentials"},
		JWKS:                    jwksDocument(t, publicRSAJWK(t, key, "kid-1")),
	})
	require.NoError(t, err)
	assert.Empty(t, result.ClientSecretPlain, "private_key_jwt 不应生成 client secret")

	require.Len(t, activeJWKs("org-1", result.Application.ID), 1, "公钥必须与查询键同租户落库")
	assert.Equal(t, "kid-1", activeJWKs("org-1", result.Application.ID)[0].Kid)

	require.Len(t, records.created, 1)
	require.Len(t, outbox.events, 1)
	payload := assertPayloadHasNoKeyMaterial(t, outbox.events[0])
	assert.Equal(t, float64(1), payload["jwksKidCount"])
	assert.Equal(t, "private_key_jwt", payload["tokenEndpointAuthMethod"])
}

func TestCreateApplicationWithPrivateKeyJWTRequiresJWKS(t *testing.T) {
	resetFakeJWKStores()
	svc, _, records, outbox := newJWKService(t)

	_, err := svc.CreateApplication(context.Background(), CreateApplicationCommand{
		OrganizationID:          "org-1",
		ActorID:                 "actor-1",
		Slug:                    "jwt-no-jwks",
		DisplayName:             "JWT No JWKS",
		AppType:                 "machine",
		Status:                  "active",
		TokenEndpointAuthMethod: "private_key_jwt",
		GrantTypes:              []string{"client_credentials"},
	})
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "jwks", appErr.Meta["field"])
	assert.Equal(t, "non_empty_jwks", appErr.Meta["expected"])

	assert.Empty(t, records.created, "422 发生在事务之前，不得产生审计")
	assert.Empty(t, outbox.events, "422 发生在事务之前，不得产生 Outbox")
}

func TestPatchApplicationReplacesJWKSAndAuditsKidList(t *testing.T) {
	resetFakeJWKStores()
	svc, repo, records, outbox := newJWKService(t)
	now := time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC)

	repo.apps = append(repo.apps, &domain.Application{
		ID:                      "jwk-app-id",
		OrganizationID:          "org-1",
		ClientID:                "gl_cid_jwk",
		TokenEndpointAuthMethod: domain.TokenEndpointAuthMethodPrivateKeyJWT,
		Revision:                1,
	})
	require.NoError(t, repo.ReplaceClientJWKs(context.Background(), "org-1", "jwk-app-id",
		[]*domain.ClientJWK{{ID: "old", ApplicationID: "jwk-app-id", OrganizationID: "org-1",
			Kid: "old-kid", Alg: "RS256", JWK: `{"kty":"RSA"}`, Status: domain.ClientJWKStatusActive}},
		now))

	newKeys := jwksDocument(t,
		publicRSAJWK(t, jwksTestRSAKey(t), "new-kid-a"),
		publicRSAJWK(t, jwksTestRSAKey(t), "new-kid-b"),
	)
	err := svc.PatchApplication(context.Background(), PatchApplicationCommand{
		ID:             "jwk-app-id",
		OrganizationID: "org-1",
		ActorID:        "actor-1",
		JWKS:           newKeys,
	})
	require.NoError(t, err)

	active := activeJWKs("org-1", "jwk-app-id")
	require.Len(t, active, 2)
	assert.ElementsMatch(t, []string{"new-kid-a", "new-kid-b"}, []string{active[0].Kid, active[1].Kid})

	require.Len(t, outbox.events, 2, "PATCH 同时写 jwks 变更事件与 application.updated 事件")
	var jwksEvent *domainoutbox.OutboxEvent
	for _, event := range outbox.events {
		if event.EventType == "application.client_jwks.updated" {
			jwksEvent = event
		}
	}
	require.NotNil(t, jwksEvent, "必须存在 application.client_jwks.updated 事件")
	require.NotEmpty(t, records.created)
	var jwksRecord *recordsdomain.Record
	for _, record := range records.created {
		if record.Action == "application.client_jwks.updated" {
			jwksRecord = record
		}
	}
	require.NotNil(t, jwksRecord, "必须存在 application.client_jwks.updated 审计")

	payload := assertPayloadHasNoKeyMaterial(t, jwksEvent)
	assert.Equal(t, float64(2), payload["kidCount"])
	assert.ElementsMatch(t, []any{"new-kid-a", "new-kid-b"}, payload["kids"])
}

func TestDCRJWKSURIShapeAndPrivateKeys(t *testing.T) {
	key := jwksTestRSAKey(t)

	t.Run("https_jwks_uri_accepted", func(t *testing.T) {
		require.NoError(t, validateDCRRequest(DCRRequest{
			RedirectURIs:            []string{"https://example.com/cb"},
			TokenEndpointAuthMethod: "private_key_jwt",
			JWKSURI:                 "https://client.example.com/jwks.json",
		}))
	})

	t.Run("non_https_jwks_uri_rejected", func(t *testing.T) {
		err := validateDCRRequest(DCRRequest{
			RedirectURIs:            []string{"https://example.com/cb"},
			TokenEndpointAuthMethod: "private_key_jwt",
			JWKSURI:                 "http://client.example.com/jwks.json",
		})
		assertInvalidClientMetadata(t, err)
	})

	t.Run("private_key_member_rejected", func(t *testing.T) {
		privateJWK := publicRSAJWK(t, key, "leaked")
		privateJWK["d"] = base64.RawURLEncoding.EncodeToString(key.D.Bytes())
		err := validateDCRRequest(DCRRequest{
			RedirectURIs:            []string{"https://example.com/cb"},
			TokenEndpointAuthMethod: "private_key_jwt",
			JWKS:                    jwksDocument(t, privateJWK),
		})
		assertInvalidClientMetadata(t, err)
	})

	t.Run("private_key_jwt_without_material", func(t *testing.T) {
		err := validateDCRRequest(DCRRequest{
			RedirectURIs:            []string{"https://example.com/cb"},
			TokenEndpointAuthMethod: "private_key_jwt",
		})
		assertInvalidClientMetadata(t, err)
	})

	t.Run("private_key_jwt_with_jwks_ok", func(t *testing.T) {
		require.NoError(t, validateDCRRequest(DCRRequest{
			RedirectURIs:            []string{"https://example.com/cb"},
			TokenEndpointAuthMethod: "private_key_jwt",
			JWKS:                    jwksDocument(t, publicRSAJWK(t, key, "kid-1")),
		}))
	})
}

func assertInvalidClientMetadata(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var metadataErr interface {
		HTTPStatus() int
		ProtocolCode() string
	}
	require.True(t, errors.As(err, &metadataErr))
	assert.Equal(t, 400, metadataErr.HTTPStatus())
	assert.Equal(t, "invalid_client_metadata", metadataErr.ProtocolCode())
}

// TestCrossTenantReadAndRotateReturnNotFound 锁定“跨租户不泄露存在性”：
// 读取与轮换都按可信 organization_id 过滤，越权访问统一返回 404，而不是 403。
func TestCrossTenantReadAndRotateReturnNotFound(t *testing.T) {
	resetFakeJWKStores()
	svc, repo, _, _ := newJWKService(t)
	repo.apps = append(repo.apps, &domain.Application{
		ID:                      "jwk-app-id",
		OrganizationID:          "org-1",
		ClientID:                "gl_cid_jwk",
		TokenEndpointAuthMethod: domain.TokenEndpointAuthMethodClientSecretBasic,
	})

	t.Run("read_by_id", func(t *testing.T) {
		_, err := svc.GetApplication(context.Background(), "org-2", "jwk-app-id")
		assertNotFound(t, err)
	})

	t.Run("rotate_secret", func(t *testing.T) {
		_, err := svc.RotateClientSecret(context.Background(), RotateClientSecretCommand{
			ID:             "jwk-app-id",
			OrganizationID: "org-2",
			ActorID:        "actor-2",
		})
		assertNotFound(t, err)
	})

	t.Run("patch_application", func(t *testing.T) {
		err := svc.PatchApplication(context.Background(), PatchApplicationCommand{
			ID:             "jwk-app-id",
			OrganizationID: "org-2",
			ActorID:        "actor-2",
		})
		assertNotFound(t, err)
	})
}

func assertNotFound(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.Equal(t, 404, appErr.HTTPStatus())
}

func (r *mockApplicationRepo) SetClientJWKSURI(_ context.Context, organizationID, applicationID, jwksURI string, _ time.Time) error {
	if r.jwksURIs == nil {
		r.jwksURIs = map[string]string{}
	}
	r.jwksURIs[organizationID+"/"+applicationID] = jwksURI
	return nil
}

func (r *mockApplicationRepo) GetClientJWKSURI(_ context.Context, organizationID, applicationID string) (*domain.ClientJWKSURI, error) {
	if uri, ok := r.jwksURIs[organizationID+"/"+applicationID]; ok {
		return &domain.ClientJWKSURI{ApplicationID: applicationID, OrganizationID: organizationID, JWKSURI: uri}, nil
	}
	return nil, apperrors.NewNotFound("client jwks_uri not found")
}

func (r *mockApplicationRepo) DeleteClientJWKSURI(_ context.Context, organizationID, applicationID string) error {
	delete(r.jwksURIs, organizationID+"/"+applicationID)
	return nil
}

func TestDCRRegistration_StoresRemoteJWKSURI(t *testing.T) {
	repo := &mockApplicationRepo{}
	service := dcrRegistrationService(repo)
	uri := "https://client.example.com/jwks.json"

	_, err := service.RegisterDynamicClient(context.Background(), DCRRequest{
		ClientName:              "Remote JWKS Client",
		RedirectURIs:            []string{"https://app.example/callback"},
		TokenEndpointAuthMethod: "private_key_jwt",
		JWKSURI:                 uri,
		OrganizationSlug:        "built-in",
	})
	require.NoError(t, err)
	require.Len(t, repo.apps, 1)
	require.Equal(t, uri, repo.jwksURIs[repo.apps[0].OrganizationID+"/"+repo.apps[0].ID])
	require.Empty(t, repo.apps[0].ClientSecretHash, "private_key_jwt must not mint a client secret")
}

func TestCreateApplicationWithPrivateKeyJWTStoresRemoteJWKSURI(t *testing.T) {
	resetFakeJWKStores()
	svc, repo, _, _ := newJWKService(t)
	uri := "https://client.example.com/jwks.json"

	result, err := svc.CreateApplication(context.Background(), CreateApplicationCommand{
		OrganizationID:          "org-1",
		ActorID:                 "actor-1",
		Slug:                    "jwt-uri-app",
		DisplayName:             "JWT URI Application",
		AppType:                 "machine",
		Status:                  "active",
		TokenEndpointAuthMethod: "private_key_jwt",
		GrantTypes:              []string{"client_credentials"},
		JWKSURI:                 uri,
	})
	require.NoError(t, err)
	require.Equal(t, uri, repo.jwksURIs["org-1/"+result.Application.ID])
	require.Empty(t, activeJWKs("org-1", result.Application.ID))
}

func TestCreateApplicationRejectsNonHTTPSJWKSURI(t *testing.T) {
	resetFakeJWKStores()
	svc, _, _, _ := newJWKService(t)
	_, err := svc.CreateApplication(context.Background(), CreateApplicationCommand{
		OrganizationID:          "org-1",
		ActorID:                 "actor-1",
		Slug:                    "jwt-uri-bad",
		DisplayName:             "JWT URI Bad",
		AppType:                 "machine",
		Status:                  "active",
		TokenEndpointAuthMethod: "private_key_jwt",
		GrantTypes:              []string{"client_credentials"},
		JWKSURI:                 "http://client.example.com/jwks.json",
	})
	require.Error(t, err)
}
