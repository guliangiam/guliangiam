package providers

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainpayment "github.com/guliangiam/guliangiam/internal/domain/payments"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	cryptotest.Configure()
	os.Exit(m.Run())
}

type uuidSequence struct {
	ids   []string
	index int
}

func (g *uuidSequence) NewString() (string, error) {
	if g.index >= len(g.ids) {
		return "", errors.New("test UUID sequence exhausted")
	}
	id := g.ids[g.index]
	g.index++
	return id, nil
}

type providerRecordRepo struct {
	items []*recordsdomain.Record
}

func (r *providerRecordRepo) Create(ctx context.Context, record *recordsdomain.Record) error {
	r.items = append(r.items, record)
	return nil
}

func (r *providerRecordRepo) List(ctx context.Context, query recordsdomain.ListQuery) ([]*recordsdomain.Record, int64, error) {
	return r.items, int64(len(r.items)), nil
}

func (r *providerRecordRepo) Search(ctx context.Context, criteria recordsdomain.SearchCriteria) ([]*recordsdomain.Record, int64, error) {
	return r.items, int64(len(r.items)), nil
}

func (r *providerRecordRepo) LoadLastChainEntry(ctx context.Context, chainID string) (seq int64, hash []byte, err error) {
	return 0, nil, nil
}

func (r *providerRecordRepo) LoadAnchorSeq(ctx context.Context, organizationID, chainID string) (int64, error) {
	return 0, nil
}

func (r *providerRecordRepo) InsertAnchor(ctx context.Context, anchor *recordsdomain.ChainAnchor) error {
	return nil
}

func (r *providerRecordRepo) ListChainEntries(ctx context.Context, chainID string) ([]recordsdomain.ChainEntry, error) {
	return nil, nil
}

func (r *providerRecordRepo) snapshot() func() {
	items := append([]*recordsdomain.Record(nil), r.items...)
	return func() { r.items = items }
}

type providerOutboxRepo struct {
	items []*domainoutbox.OutboxEvent
}

func (r *providerOutboxRepo) Enqueue(ctx context.Context, item *domainoutbox.OutboxEvent) error {
	r.items = append(r.items, item)
	return nil
}

func (r *providerOutboxRepo) EnqueueBatch(ctx context.Context, items []*domainoutbox.OutboxEvent) error {
	r.items = append(r.items, items...)
	return nil
}

func (r *providerOutboxRepo) snapshot() func() {
	items := append([]*domainoutbox.OutboxEvent(nil), r.items...)
	return func() { r.items = items }
}

type mockPaymentChannelConfigRepo struct {
	created []*domainpayment.PayChannelConfig
	updated []*domainpayment.PayChannelConfig
}

func (r *mockPaymentChannelConfigRepo) GetByID(ctx context.Context, id string) (*domainpayment.PayChannelConfig, error) {
	return nil, nil
}

func (r *mockPaymentChannelConfigRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domainpayment.PayChannelConfig, error) {
	return nil, nil
}

func (r *mockPaymentChannelConfigRepo) GetActive(ctx context.Context, organizationID, channel, environment string) (*domainpayment.PayChannelConfig, error) {
	return nil, nil
}

func (r *mockPaymentChannelConfigRepo) List(ctx context.Context, organizationID string) ([]*domainpayment.PayChannelConfig, error) {
	return nil, nil
}

func (r *mockPaymentChannelConfigRepo) Create(ctx context.Context, config *domainpayment.PayChannelConfig) error {
	copied := *config
	r.created = append(r.created, &copied)
	return nil
}

func (r *mockPaymentChannelConfigRepo) Update(ctx context.Context, config *domainpayment.PayChannelConfig) error {
	copied := *config
	r.updated = append(r.updated, &copied)
	return nil
}

type paymentRotationUnitOfWork struct {
	testUnitOfWork
	channelConfigs *mockPaymentChannelConfigRepo
}

func (u paymentRotationUnitOfWork) PaymentChannelConfigs() domainpayment.ChannelConfigsRepo {
	return u.channelConfigs
}

func newUnifiedTestService(now time.Time) (Service, *mockProviderRepo, *providerRecordRepo, *providerOutboxRepo) {
	repo := &mockProviderRepo{}
	records := &providerRecordRepo{}
	outbox := &providerOutboxRepo{}
	uow := testUnitOfWork{providerRepo: repo, recordRepo: records, outboxRepo: outbox}
	svc := NewService(
		fakeClock{fixedTime: now},
		&uuidSequence{ids: []string{"provider-1", "secret-1", "secret-2"}},
		uow,
	)
	return svc, repo, records, outbox
}

func createIdentityCommand(secret string) CreateProviderCommand {
	return CreateProviderCommand{
		OrganizationID: "org-1",
		Name:           "github",
		DisplayName:    "GitHub",
		Category:       "OAuth",
		ProviderType:   "github",
		ClientID:       "client-id",
		ClientSecret:   secret,
	}
}

func TestUnifiedServiceCreateWithSecretPersistsRegistryAndAudit(t *testing.T) {
	now := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	svc, repo, records, outbox := newUnifiedTestService(now)

	const actorID = "019operator-0000-7000-8000-000000000005"
	command := createIdentityCommand("initial-secret")
	command.ActorID = actorID
	_, err := svc.CreateProvider(context.Background(), command)
	require.NoError(t, err)
	require.Len(t, repo.providers, 1)
	require.Len(t, repo.configs, 1)
	require.Len(t, repo.secretStatus, 1)
	assert.Equal(t, domain.FamilyIdentity, repo.providers[0].Family)
	assert.Equal(t, domain.Kind("github"), repo.providers[0].Kind)
	assert.Empty(t, repo.providers[0].Variant)
	assert.Equal(t, registryConfigType, repo.configs[0].ConfigType)
	assert.Equal(t, "active", repo.secretStatus[0].Status)
	assert.Equal(t, int64(1), repo.secretStatus[0].Version)

	metadata, err := getRegistryMetadata(context.Background(), repo, "provider-1")
	require.NoError(t, err)
	assert.Equal(t, string(domain.FamilyIdentity), metadata.Family)
	assert.Equal(t, "github", metadata.Kind)
	assert.Equal(t, string(domain.StatusDraft), metadata.Status)
	secret := metadata.Secrets["client_secret"]
	assert.Equal(t, fingerprintSecret("initial-secret"), secret.Fingerprint)
	assert.Equal(t, int64(1), secret.Version)
	assert.Equal(t, 1, secret.KeyVersion)

	require.Len(t, records.items, 1)
	require.Len(t, outbox.items, 1)
	assert.Equal(t, "provider.identity.created", records.items[0].Action)
	assert.Equal(t, actorID, records.items[0].ActorID, "actor 应透传到审计记录")
	assert.Equal(t, "provider.identity.created", outbox.items[0].EventType)
	assert.NotContains(t, string(outbox.items[0].Payload), "initial-secret")
	metadataJSON, err := json.Marshal(repo.configs[0].Config)
	require.NoError(t, err)
	assert.NotContains(t, string(metadataJSON), "initial-secret")

	detail, err := svc.GetProvider(context.Background(), "provider-1", "org-1")
	require.NoError(t, err)
	assert.Equal(t, string(domain.FamilyIdentity), detail.Family)
	assert.Equal(t, string(domain.StatusDraft), detail.Status)
	assert.True(t, detail.ClientSecretSet)
	assert.NotEqual(t, "initial-secret", detail.ClientSecretMasked)
}

func TestUnifiedServiceTestConnectionPassesIDVConfig(t *testing.T) {
	svc, repo, _, _ := newUnifiedTestService(time.Now())
	repo.providers = []*domain.Provider{{
		ID: "idv-1", OrganizationID: "org-1", Name: "aliyun-idv",
		Category: domain.CategoryIDVerification, Kind: domain.Kind("aliyun_idv"),
		ProviderType: domain.ProviderType("aliyun_idv"),
	}}
	repo.configs = []*domain.ProviderConfig{{
		ProviderID: "idv-1", ConfigType: "default",
		Config: map[string]any{"idvSceneId": "scene-1"},
	}}

	var received map[string]any
	svc.Unified.IDVProbe = func(_ context.Context, _ *domain.Provider, config map[string]any) error {
		received = config
		return nil
	}
	result, err := svc.TestConnection(context.Background(), "idv-1", "org-1", "")
	require.NoError(t, err)
	require.True(t, result.Connected)
	assert.Equal(t, "scene-1", received["idvSceneId"])
}

func TestNewRegistrationRowPersistsClassification(t *testing.T) {
	row := newRegistrationRow(&Provider{Registration: domain.Registration{
		Family:  domain.FamilyPayment,
		Kind:    domain.Kind("alipay"),
		Variant: domain.VariantSandbox,
	}})

	assert.Equal(t, domain.FamilyPayment, row.Family)
	assert.Equal(t, domain.Kind("alipay"), row.Kind)
	assert.Equal(t, domain.VariantSandbox, row.Variant)
}

func TestUnifiedServiceCreateWithSecretRollsBackWhenRotationFails(t *testing.T) {
	now := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	svc, repo, records, outbox := newUnifiedTestService(now)
	rotationErr := errors.New("secret status insert failed")
	repo.rotationErr = rotationErr

	_, err := svc.CreateProvider(context.Background(), createIdentityCommand("initial-secret"))
	require.ErrorIs(t, err, rotationErr)
	assert.Empty(t, repo.providers)
	assert.Empty(t, repo.configs)
	assert.Empty(t, repo.secretStatus)
	assert.Empty(t, records.items)
	assert.Empty(t, outbox.items)
}

func TestUnifiedServiceRotateSecretAdvancesVersionWithoutPlaintextResult(t *testing.T) {
	now := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	svc, repo, records, outbox := newUnifiedTestService(now)
	_, err := svc.CreateProvider(context.Background(), createIdentityCommand("initial-secret"))
	require.NoError(t, err)

	result, err := svc.Unified.RotateSecret(context.Background(), RotateProviderSecretCommand{
		ID:          "provider-1",
		Family:      "identity",
		Plaintext:   "rotated-secret",
		Revision:    1,
		GracePeriod: time.Hour,
	})
	require.NoError(t, err)
	assert.Equal(t, "provider-1", result.ProviderID)
	assert.Equal(t, "client_secret", result.SecretType)
	assert.True(t, result.SecretSet)
	assert.Equal(t, "******", result.SecretMasked)
	assert.Equal(t, fingerprintSecret("rotated-secret"), result.Fingerprint)
	assert.Equal(t, int64(2), result.Version)
	resultJSON, err := json.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(resultJSON), "rotated-secret")

	require.Len(t, repo.secretStatus, 2)
	assert.Equal(t, "grace", repo.secretStatus[0].Status)
	assert.True(t, repo.secretStatus[0].ExpiresAt.Equal(now.Add(time.Hour)))
	assert.Equal(t, "active", repo.secretStatus[1].Status)
	assert.Equal(t, int64(2), repo.secretStatus[1].Version)
	metadata, err := getRegistryMetadata(context.Background(), repo, "provider-1")
	require.NoError(t, err)
	assert.Equal(t, int64(2), metadata.Secrets["client_secret"].Version)
	assert.Equal(t, fingerprintSecret("rotated-secret"), metadata.Secrets["client_secret"].Fingerprint)
	metadataJSON, err := json.Marshal(repo.configs[0].Config)
	require.NoError(t, err)
	assert.NotContains(t, string(metadataJSON), "rotated-secret")

	require.Len(t, records.items, 2)
	require.Len(t, outbox.items, 2)
	assert.Equal(t, "provider.identity.secret.rotated", records.items[1].Action)
	assert.Equal(t, "provider.identity.secret.rotated", outbox.items[1].EventType)
	assert.NotContains(t, string(outbox.items[1].Payload), "rotated-secret")
}

func TestUnifiedServicePaymentRotationProviderAnchor(t *testing.T) {
	now := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name           string
		providerID     string
		wantProviderID string
		wantInternal   bool
	}{
		{name: "preserves existing anchor", providerID: "provider-1", wantProviderID: "provider-1"},
		{name: "backfills legacy empty anchor", wantProviderID: "provider-1"},
		{name: "rejects mismatched anchor", providerID: "provider-2", wantInternal: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockPaymentChannelConfigRepo{}
			tx := paymentRotationUnitOfWork{channelConfigs: repo}
			svc := &UnifiedService{
				Clock: fakeClock{fixedTime: now},
				UUID:  &uuidSequence{ids: []string{"detail-2"}},
			}
			detail := &domainpayment.PayChannelConfig{
				ID: "detail-1", OrganizationID: "org-1", ProviderID: tt.providerID,
				Channel: "alipay", Environment: "production", VersionNo: 4,
				Status: domainpayment.ChannelConfigStatusActive, CreatedAt: now, UpdatedAt: now, Revision: 3,
			}
			provider := &Provider{
				Registration: domain.Registration{
					ID: "provider-1", OrganizationID: "org-1", Family: domain.FamilyPayment, Status: domain.StatusActive,
				},
				Detail:   detail,
				metadata: registryMetadata{DetailID: detail.ID},
			}

			err := svc.rotateNativeSecret(context.Background(), tx, provider, RotateProviderSecretCommand{Plaintext: "rotated-config"})
			if tt.wantInternal {
				require.Error(t, err)
				var platformErr *apperrors.Error
				require.ErrorAs(t, err, &platformErr)
				assert.Equal(t, apperrors.CodeInternal, platformErr.Code)
				assert.Empty(t, repo.updated)
				assert.Empty(t, repo.created)
				return
			}
			require.NoError(t, err)
			require.Len(t, repo.updated, 1)
			require.Len(t, repo.created, 1)
			assert.Equal(t, tt.providerID, repo.updated[0].ProviderID)
			next := repo.created[0]
			assert.Equal(t, "detail-2", next.ID)
			assert.Equal(t, 5, next.VersionNo)
			assert.Equal(t, tt.wantProviderID, next.ProviderID)
			assert.Equal(t, tt.wantProviderID, provider.Detail.(*domainpayment.PayChannelConfig).ProviderID)
			assert.Equal(t, next.ID, provider.metadata.DetailID)
		})
	}
}

func TestServiceListProvidersUsesRegistryFiltersAndDTO(t *testing.T) {
	now := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	repo := &mockProviderRepo{
		providers: []*domain.Provider{
			{ID: "ai-1", OrganizationID: "org-1", Name: "openai", DisplayName: "OpenAI", Category: "AI", ProviderType: "openai", CreatedAt: now, UpdatedAt: now, Revision: 1},
			{ID: "identity-1", OrganizationID: "org-1", Name: "github", DisplayName: "GitHub", Category: "OAuth", ProviderType: "github", Enabled: true, CreatedAt: now, UpdatedAt: now, Revision: 1},
		},
	}
	aiRegistration := domain.Registration{ID: "ai-1", OrganizationID: "org-1", Name: "openai", DisplayName: "OpenAI", Family: domain.FamilyAI, Kind: "openai", Status: domain.StatusDraft, CreatedAt: now, UpdatedAt: now, Revision: 1}
	identityRegistration := domain.Registration{ID: "identity-1", OrganizationID: "org-1", Name: "github", DisplayName: "GitHub", Family: domain.FamilyIdentity, Kind: "github", Status: domain.StatusActive, CreatedAt: now, UpdatedAt: now, Revision: 1}
	aiConfig, err := metadataFromRegistration(aiRegistration, aiRegistration.ID).config(aiRegistration.ID, now)
	require.NoError(t, err)
	identityConfig, err := metadataFromRegistration(identityRegistration, identityRegistration.ID).config(identityRegistration.ID, now)
	require.NoError(t, err)
	repo.configs = []*domain.ProviderConfig{aiConfig, identityConfig}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{providerRepo: repo}}

	items, total, err := svc.ListProviders(context.Background(), ListProvidersQuery{Family: "AI", Kind: "OPENAI", Status: "DRAFT", Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "ai-1", items[0].ID)
	assert.Equal(t, "ai", items[0].Family)
	assert.Equal(t, "openai", items[0].Kind)
	assert.Equal(t, "draft", items[0].Status)
	assert.False(t, items[0].Enabled)
}

func TestUnifiedServiceDeleteRejectsActiveProvider(t *testing.T) {
	now := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	repo := &mockProviderRepo{providers: []*domain.Provider{{ID: "provider-1", OrganizationID: "org-1", Name: "github", DisplayName: "GitHub", Category: "OAuth", ProviderType: "github", Enabled: true, CreatedAt: now, UpdatedAt: now, Revision: 1}}}
	registration := domain.Registration{ID: "provider-1", OrganizationID: "org-1", Name: "github", DisplayName: "GitHub", Family: domain.FamilyIdentity, Kind: "github", Status: domain.StatusActive, CreatedAt: now, UpdatedAt: now, Revision: 1}
	config, err := metadataFromRegistration(registration, registration.ID).config(registration.ID, now)
	require.NoError(t, err)
	repo.configs = []*domain.ProviderConfig{config}
	records := &providerRecordRepo{}
	outbox := &providerOutboxRepo{}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{providerRepo: repo, recordRepo: records, outboxRepo: outbox}}

	err = svc.DeleteProvider(context.Background(), "provider-1", "org-1", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "active provider cannot be deleted")
	assert.Len(t, repo.providers, 1)
	assert.Empty(t, records.items)
	assert.Empty(t, outbox.items)
}

func TestEnableProvider_ReenablesDisabledProvider(t *testing.T) {
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	svc, repo, _, _ := newUnifiedTestService(now)
	command := createIdentityCommand("initial-secret")
	command.AuthUrl = "https://github.com/login/oauth/authorize"
	command.TokenUrl = "https://github.com/login/oauth/access_token"
	_, err := svc.CreateProvider(context.Background(), command)
	require.NoError(t, err)

	_, err = svc.EnableProvider(context.Background(), "provider-1", "org-1", "actor-1")
	require.NoError(t, err)
	_, err = svc.Unified.ChangeStatus(context.Background(), ChangeProviderStatusCommand{
		ID: "provider-1", Family: "identity", Status: string(domain.StatusDisabled), OrganizationID: "org-1",
	})
	require.NoError(t, err)

	_, err = svc.EnableProvider(context.Background(), "provider-1", "org-1", "actor-1")
	require.NoError(t, err)
	metadata, err := getRegistryMetadata(context.Background(), repo, "provider-1")
	require.NoError(t, err)
	assert.Equal(t, string(domain.StatusActive), metadata.Status)
}
