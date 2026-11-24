package payments

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── fake clock (suffix "cc" to avoid conflict with service_test.go) ─

type ccFakeClock struct {
	t time.Time
}

func (f ccFakeClock) Now() time.Time { return f.t }

// compile-time guard
var _ clock.Clock = ccFakeClock{}

// ── fake cipher ──────────────────────────────────────────────────────

type recordingCCipher struct {
	prefix    string
	plaintext string
	calls     int
	err       error
}

func (c *recordingCCipher) EncryptField(plaintext string) (string, error) {
	c.plaintext = plaintext
	c.calls++
	if c.err != nil {
		return "", c.err
	}
	return c.prefix + plaintext, nil
}

var _ ChannelConfigCipher = (*recordingCCipher)(nil)

// ── fake repo ───────────────────────────────────────────────────────

type fakeChannelConfigsRepo struct {
	list    []*domain.PayChannelConfig
	byID    map[string]*domain.PayChannelConfig
	created *domain.PayChannelConfig
	updated *domain.PayChannelConfig
}

func newFakeCCRepo() *fakeChannelConfigsRepo {
	return &fakeChannelConfigsRepo{byID: make(map[string]*domain.PayChannelConfig)}
}

func (f *fakeChannelConfigsRepo) GetByID(_ context.Context, id string) (*domain.PayChannelConfig, error) {
	c, ok := f.byID[id]
	if !ok {
		return nil, apperrors.NewNotFound("channel config not found")
	}
	return c, nil
}
func (f *fakeChannelConfigsRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.PayChannelConfig, error) {
	c, ok := f.byID[id]
	if !ok || c.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("channel config not found")
	}
	return c, nil
}
func (f *fakeChannelConfigsRepo) GetActive(_ context.Context, _, _, _ string) (*domain.PayChannelConfig, error) {
	return nil, apperrors.NewNotFound("not configured")
}
func (f *fakeChannelConfigsRepo) List(_ context.Context, _ string) ([]*domain.PayChannelConfig, error) {
	return f.list, nil
}
func (f *fakeChannelConfigsRepo) Create(_ context.Context, c *domain.PayChannelConfig) error {
	f.created = c
	f.byID[c.ID] = c
	return nil
}
func (f *fakeChannelConfigsRepo) Update(_ context.Context, c *domain.PayChannelConfig) error {
	f.updated = c
	return nil
}

// compile-time
var _ domain.ChannelConfigsRepo = (*fakeChannelConfigsRepo)(nil)

// ── tests ──────────────────────────────────────────────────────────

func TestChannelConfigService_Create_Initial(t *testing.T) {
	repo := newFakeCCRepo()
	now := time.Now().UTC()
	cipher := &recordingCCipher{prefix: "v1-test:"}
	svc := NewChannelConfigService(repo, ccFakeClock{t: now}, cipher, fakeUUIDGenerator{fixedID: "cc-generated"})

	cfg, err := svc.Create(context.Background(), CreateChannelConfigInput{
		ID:             "cc-1",
		OrganizationID: "org-1",
		Channel:        "alipay",
		Environment:    "production",
		Config:         `{"privateKey":"secret-value","appId":"app-1"}`,
	})
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "draft", cfg.Status)
	assert.Equal(t, 1, cfg.VersionNo)
	assert.NotEmpty(t, cfg.ConfigFingerprint)
	assert.Equal(t, now, cfg.CreatedAt)
	assert.NotNil(t, repo.created)

	// The service seals the canonical plaintext and derives configSet itself.
	assert.Equal(t, `{"appId":"app-1","privateKey":"secret-value"}`, cipher.plaintext)
	assert.Equal(t, "v1-test:"+cipher.plaintext, cfg.ConfigSealed)
	assert.NotEqual(t, cipher.plaintext, cfg.ConfigSealed)
	assert.Equal(t, []string{"appId", "privateKey"}, cfg.ConfigSet)

	sum := sha256.Sum256([]byte(cipher.plaintext))
	assert.Equal(t, hex.EncodeToString(sum[:8]), cfg.ConfigFingerprint)
}

func TestChannelConfigService_Create_GeneratesIDWhenOmitted(t *testing.T) {
	repo := newFakeCCRepo()
	svc := NewChannelConfigService(repo, ccFakeClock{t: time.Now()}, &recordingCCipher{prefix: "v1-test:"}, fakeUUIDGenerator{fixedID: "cc-generated"})

	cfg, err := svc.Create(context.Background(), CreateChannelConfigInput{
		OrganizationID: "org-1",
		Channel:        "square",
		Environment:    "sandbox",
		Config:         `{"accessToken":"t","locationId":"L"}`,
	})
	require.NoError(t, err)
	assert.Equal(t, "cc-generated", cfg.ID)
	assert.Equal(t, "cc-generated", repo.created.ID)
}

func TestChannelConfigService_Create_AutoVersionIncrement(t *testing.T) {
	repo := newFakeCCRepo()
	repo.list = []*domain.PayChannelConfig{
		{OrganizationID: "org-1", Channel: "alipay", Environment: "production", VersionNo: 2},
	}
	svc := NewChannelConfigService(repo, ccFakeClock{t: time.Now()}, &recordingCCipher{prefix: "v1-test:"}, fakeUUIDGenerator{fixedID: "cc-generated"})

	cfg, err := svc.Create(context.Background(), CreateChannelConfigInput{
		ID:             "cc-3",
		OrganizationID: "org-1",
		Channel:        "alipay",
		Environment:    "production",
		Config:         `{"appId":"app-3"}`,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, cfg.VersionNo, "should increment to 3")
}

func TestChannelConfigService_Create_DefaultEnvironment(t *testing.T) {
	repo := newFakeCCRepo()
	svc := NewChannelConfigService(repo, ccFakeClock{t: time.Now()}, &recordingCCipher{prefix: "v1-test:"}, fakeUUIDGenerator{fixedID: "cc-generated"})

	cfg, err := svc.Create(context.Background(), CreateChannelConfigInput{
		ID:             "cc-4",
		OrganizationID: "org-1",
		Channel:        "stripe",
		Config:         `{"appId":"app-4"}`,
	})
	require.NoError(t, err)
	assert.Equal(t, "production", cfg.Environment)
}

func TestChannelConfigService_Create_ValidationError(t *testing.T) {
	repo := newFakeCCRepo()
	svc := NewChannelConfigService(repo, ccFakeClock{t: time.Now()}, &recordingCCipher{prefix: "v1-test:"}, fakeUUIDGenerator{fixedID: "cc-generated"})

	_, err := svc.Create(context.Background(), CreateChannelConfigInput{
		ID:             "cc-5",
		OrganizationID: "", // missing
		Channel:        "alipay",
		Config:         `{"appId":"app-5"}`,
	})
	assert.Error(t, err)
}

func TestChannelConfigService_Transition_Valid(t *testing.T) {
	now := time.Now().UTC()
	repo := newFakeCCRepo()
	repo.byID["cc-1"] = &domain.PayChannelConfig{
		ID:             "cc-1",
		OrganizationID: "org-1",
		Status:         domain.ChannelConfigStatusDraft,
		VersionNo:      1,
		Environment:    "production",
		Channel:        "alipay",
		Revision:       1,
		CreatedAt:      now,
	}
	svc := NewChannelConfigService(repo, ccFakeClock{t: now}, &recordingCCipher{prefix: "v1-test:"}, fakeUUIDGenerator{fixedID: "cc-generated"})

	cfg, err := svc.Transition(context.Background(), "org-1", "cc-1", domain.ChannelConfigStatusTesting)
	require.NoError(t, err)
	assert.Equal(t, domain.ChannelConfigStatusTesting, cfg.Status)
	assert.NotNil(t, repo.updated)
}

func TestChannelConfigService_Transition_ActiveSetsActivatedAt(t *testing.T) {
	now := time.Now().UTC()
	repo := newFakeCCRepo()
	repo.byID["cc-1"] = &domain.PayChannelConfig{
		ID:             "cc-1",
		OrganizationID: "org-1",
		Status:         domain.ChannelConfigStatusTesting,
		VersionNo:      1,
		Channel:        "alipay",
		Revision:       1,
		CreatedAt:      now,
	}
	svc := NewChannelConfigService(repo, ccFakeClock{t: now}, &recordingCCipher{prefix: "v1-test:"}, fakeUUIDGenerator{fixedID: "cc-generated"})

	cfg, err := svc.Transition(context.Background(), "org-1", "cc-1", domain.ChannelConfigStatusActive)
	require.NoError(t, err)
	assert.Equal(t, domain.ChannelConfigStatusActive, cfg.Status)
	assert.NotNil(t, cfg.ActivatedAt)
}

func TestChannelConfigService_Transition_InvalidPath(t *testing.T) {
	now := time.Now().UTC()
	repo := newFakeCCRepo()
	repo.byID["cc-1"] = &domain.PayChannelConfig{
		ID:             "cc-1",
		OrganizationID: "org-1",
		Status:         domain.ChannelConfigStatusActive,
		VersionNo:      1,
		Channel:        "alipay",
		Revision:       1,
		CreatedAt:      now,
	}
	svc := NewChannelConfigService(repo, ccFakeClock{t: now}, &recordingCCipher{prefix: "v1-test:"}, fakeUUIDGenerator{fixedID: "cc-generated"})

	// active → testing 是非法路径
	_, err := svc.Transition(context.Background(), "org-1", "cc-1", domain.ChannelConfigStatusTesting)
	assert.Error(t, err)
}

func TestChannelConfigService_Disable(t *testing.T) {
	now := time.Now().UTC()
	repo := newFakeCCRepo()
	repo.byID["cc-1"] = &domain.PayChannelConfig{
		ID:             "cc-1",
		OrganizationID: "org-1",
		Status:         domain.ChannelConfigStatusActive,
		VersionNo:      1,
		Channel:        "alipay",
		Revision:       1,
		CreatedAt:      now,
	}
	svc := NewChannelConfigService(repo, ccFakeClock{t: now}, &recordingCCipher{prefix: "v1-test:"}, fakeUUIDGenerator{fixedID: "cc-generated"})

	cfg, err := svc.Disable(context.Background(), "org-1", "cc-1")
	require.NoError(t, err)
	assert.Equal(t, domain.ChannelConfigStatusDisabled, cfg.Status)
	assert.NotNil(t, cfg.DisabledAt)
}

func TestChannelConfigService_Create_FingerprintUsesCanonicalPlaintext(t *testing.T) {
	now := time.Now().UTC()

	firstCipher := &recordingCCipher{prefix: "v1-test:"}
	firstRepo := newFakeCCRepo()
	first, err := NewChannelConfigService(firstRepo, ccFakeClock{t: now}, firstCipher, fakeUUIDGenerator{fixedID: "cc-generated"}).Create(
		context.Background(), CreateChannelConfigInput{
			ID: "cc-a", OrganizationID: "org-1", Channel: "stripe",
			Config: `{"b":2,"a":1}`,
		})
	require.NoError(t, err)

	secondCipher := &recordingCCipher{prefix: "v1-test:"}
	secondRepo := newFakeCCRepo()
	second, err := NewChannelConfigService(secondRepo, ccFakeClock{t: now}, secondCipher, fakeUUIDGenerator{fixedID: "cc-generated"}).Create(
		context.Background(), CreateChannelConfigInput{
			ID: "cc-b", OrganizationID: "org-1", Channel: "stripe",
			Config: "{\n  \"a\": 1,\n  \"b\": 2\n}",
		})
	require.NoError(t, err)

	assert.Equal(t, firstCipher.plaintext, secondCipher.plaintext)
	assert.Equal(t, first.ConfigFingerprint, second.ConfigFingerprint,
		"key order and whitespace must not change the fingerprint")
	assert.Equal(t, []string{"a", "b"}, first.ConfigSet)
}

func TestChannelConfigService_Create_RejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config string
	}{
		{"missing", ""},
		{"whitespace", "   "},
		{"empty object", `{}`},
		{"null", `null`},
		{"array", `[]`},
		{"string", `"{\"a\":1}"`},
		{"number", `1`},
		{"malformed", `{"a":`},
		{"two objects", `{"a":1}{"b":2}`},
		{"oversize", `{"a":"` + strings.Repeat("x", maxChannelConfigBytes) + `"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeCCRepo()
			cipher := &recordingCCipher{prefix: "v1-test:"}
			svc := NewChannelConfigService(repo, ccFakeClock{t: time.Now()}, cipher, fakeUUIDGenerator{fixedID: "cc-generated"})

			_, err := svc.Create(context.Background(), CreateChannelConfigInput{
				ID: "cc-bad", OrganizationID: "org-1", Channel: "stripe", Config: tt.config,
			})
			requireAppErrorCode(t, err, apperrors.CodePaymentChannelConfigInvalid)
			assert.Zero(t, cipher.calls, "invalid config must not reach the cipher")
			assert.Nil(t, repo.created)
		})
	}
}

func TestChannelConfigService_Create_FailsClosedWithoutCipher(t *testing.T) {
	repo := newFakeCCRepo()
	svc := NewChannelConfigService(repo, ccFakeClock{t: time.Now()}, nil, fakeUUIDGenerator{fixedID: "cc-generated"})

	_, err := svc.Create(context.Background(), CreateChannelConfigInput{
		ID: "cc-1", OrganizationID: "org-1", Channel: "stripe", Config: `{"a":1}`,
	})
	requireAppErrorCode(t, err, apperrors.CodeInternal)
	assert.Nil(t, repo.created)
}

func TestChannelConfigService_Create_CipherFailureIsInternal(t *testing.T) {
	repo := newFakeCCRepo()
	cipher := &recordingCCipher{err: apperrors.NewInternal("kms unavailable")}
	svc := NewChannelConfigService(repo, ccFakeClock{t: time.Now()}, cipher, fakeUUIDGenerator{fixedID: "cc-generated"})

	_, err := svc.Create(context.Background(), CreateChannelConfigInput{
		ID: "cc-1", OrganizationID: "org-1", Channel: "stripe", Config: `{"a":1}`,
	})
	requireAppErrorCode(t, err, apperrors.CodeInternal)
	assert.Nil(t, repo.created)
	assert.NotContains(t, err.Error(), `{"a":1}`)
}

// guard errInvalidTransition is local type — reference it once
var _ error = (*transitionError)(nil)
