package auth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	trusteddevicesdomain "github.com/guliangiam/guliangiam/internal/domain/trusteddevices"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	webauthnmock "github.com/guliangiam/guliangiam/internal/domain/webauthn/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTrustedDeviceStatus(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	revoked := now.Add(-time.Hour)

	assert.Equal(t, "active", trustedDeviceStatus(now, nil, now.Add(time.Hour)))
	assert.Equal(t, "expired", trustedDeviceStatus(now, nil, now))
	assert.Equal(t, "expired", trustedDeviceStatus(now, nil, now.Add(-time.Minute)))
	assert.Equal(t, "revoked", trustedDeviceStatus(now, &revoked, now.Add(time.Hour)))
	assert.Equal(t, "revoked", trustedDeviceStatus(now, &revoked, now.Add(-time.Hour)))
}

func TestTrustedDeviceUAHash(t *testing.T) {
	hash := trustedDeviceUAHash("Mozilla/5.0 (Macintosh)")
	assert.Len(t, hash, 64)
	assert.Equal(t, hash, trustedDeviceUAHash("  Mozilla/5.0 (Macintosh) "))
	assert.NotEqual(t, hash, trustedDeviceUAHash("Mozilla/5.0 (Windows)"))
}

func TestTrustedDeviceSignatureMessage(t *testing.T) {
	assert.Equal(
		t,
		"trusted-device:v1:challenge-1:user-1:nonce-1",
		trustedDeviceSignatureMessage("challenge-1", "user-1", "nonce-1"),
	)
}

func TestTrustedDeviceDecodeBase64(t *testing.T) {
	raw := []byte("hello-der")
	for _, encoded := range []string{
		base64.StdEncoding.EncodeToString(raw),
		base64.URLEncoding.EncodeToString(raw),
		base64.RawURLEncoding.EncodeToString(raw),
	} {
		decoded, err := trustedDeviceDecodeBase64(encoded)
		require.NoError(t, err)
		assert.Equal(t, raw, decoded)
	}

	_, err := trustedDeviceDecodeBase64("not base64 !!!")
	require.Error(t, err)
	assertErrorCode(t, err, apperrors.CodeInvalidArgument)
}

func TestVerifyTrustedDeviceSignatureECDSA(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)

	message := trustedDeviceSignatureMessage("challenge-1", "user-1", "nonce-1")
	digest := sha256.Sum256([]byte(message))
	signature, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	require.NoError(t, err)

	require.NoError(t, verifyTrustedDeviceSignature(der, message, signature))

	err = verifyTrustedDeviceSignature(der, message+"tampered", signature)
	require.Error(t, err)
	assertErrorCode(t, err, apperrors.CodeUnauthorized)

	badSignature := append([]byte(nil), signature...)
	badSignature[len(badSignature)-1] ^= 0xff
	err = verifyTrustedDeviceSignature(der, message, badSignature)
	require.Error(t, err)
	assertErrorCode(t, err, apperrors.CodeUnauthorized)
}

func TestVerifyTrustedDeviceSignatureRSA(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)

	message := trustedDeviceSignatureMessage("challenge-rsa", "user-1", "nonce-1")
	digest := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)

	require.NoError(t, verifyTrustedDeviceSignature(der, message, signature))

	err = verifyTrustedDeviceSignature(der, message+"tampered", signature)
	require.Error(t, err)
	assertErrorCode(t, err, apperrors.CodeUnauthorized)
}

func TestVerifyTrustedDeviceSignatureEd25519(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	require.NoError(t, err)

	message := trustedDeviceSignatureMessage("challenge-ed", "user-1", "nonce-1")
	signature := ed25519.Sign(privateKey, []byte(message))

	require.NoError(t, verifyTrustedDeviceSignature(der, message, signature))

	err = verifyTrustedDeviceSignature(der, message+"tampered", signature)
	require.Error(t, err)
	assertErrorCode(t, err, apperrors.CodeUnauthorized)
}

func TestVerifyTrustedDeviceSignatureRejectsUnsupportedType(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)

	err = verifyTrustedDeviceSignature(der, "message", []byte("signature"))
	require.Error(t, err)
	assertErrorCode(t, err, apperrors.CodeUnprocessableEntity)

	err = verifyTrustedDeviceSignature([]byte("not-der"), "message", []byte("signature"))
	require.Error(t, err)
	assertErrorCode(t, err, apperrors.CodeUnprocessableEntity)
}

func TestTrustedDeviceViewNeverExposesKeyMaterial(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	device := &trusteddevicesdomain.TrustedDevice{
		ID:                 "device-1",
		OrganizationID:     "org-1",
		UserID:             "user-1",
		DeviceCredentialID: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		DevicePublicKey:    []byte("public-key"),
		UAHash:             "ua-hash",
		CreatedAt:          now,
		LastSeenAt:         now,
		ExpiresAt:          now.Add(30 * 24 * time.Hour),
		Revision:           1,
		UpdatedAt:          now,
	}
	view := toTrustedDeviceView(device, now)

	assert.Equal(t, "active", view.Status)

	payload, err := json.Marshal(view)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "devicePublicKey")
	assert.NotContains(t, string(payload), "uaHash")
	assert.NotContains(t, string(payload), "device_public_key")
	assert.NotContains(t, string(payload), "ua_hash")
}

func assertErrorCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, code, appErr.Code)
}

// capturingTrustedDeviceRepository records what the app layer sends to storage.
// FindActiveByCredential matches on the digest exactly (as the SQL WHERE does),
// so a plaintext row can never satisfy a digest lookup.
type capturingTrustedDeviceRepository struct {
	rows               []*trusteddevicesdomain.TrustedDevice
	created            []*trusteddevicesdomain.TrustedDevice
	revokedCredentials []string
	findCredentialIDs  []string
	touchedLastSeen    int
}

func (r *capturingTrustedDeviceRepository) Create(_ context.Context, device *trusteddevicesdomain.TrustedDevice) error {
	r.created = append(r.created, device)
	return nil
}

func (r *capturingTrustedDeviceRepository) GetByID(_ context.Context, id string) (*trusteddevicesdomain.TrustedDevice, error) {
	for _, row := range r.rows {
		if row.ID == id {
			return row, nil
		}
	}
	return nil, apperrors.NewNotFound("trusted device not found")
}

func (r *capturingTrustedDeviceRepository) ListByUser(_ context.Context, _, _ string) ([]*trusteddevicesdomain.TrustedDevice, error) {
	return r.rows, nil
}

func (r *capturingTrustedDeviceRepository) FindActiveByCredential(_ context.Context, organizationID, userID, credentialID, _ string, _ time.Time) (*trusteddevicesdomain.TrustedDevice, error) {
	r.findCredentialIDs = append(r.findCredentialIDs, credentialID)
	for _, row := range r.rows {
		if row.OrganizationID == organizationID && row.UserID == userID && row.DeviceCredentialID == credentialID {
			return row, nil
		}
	}
	return nil, apperrors.NewNotFound("no active trusted device for credential")
}

func (r *capturingTrustedDeviceRepository) Revoke(_ context.Context, _, _ string, _ time.Time) error {
	return nil
}

func (r *capturingTrustedDeviceRepository) TouchLastSeen(_ context.Context, _ string, _ time.Time) error {
	r.touchedLastSeen++
	return nil
}

func (r *capturingTrustedDeviceRepository) RevokeByCredential(_ context.Context, _, _, credentialID string, _ time.Time) error {
	r.revokedCredentials = append(r.revokedCredentials, credentialID)
	return nil
}

func (r *capturingTrustedDeviceRepository) RevokeByUser(_ context.Context, organizationID, userID string, _ time.Time) (int64, error) {
	var count int64
	for _, row := range r.rows {
		if row.OrganizationID == organizationID && row.UserID == userID {
			count++
		}
	}
	return count, nil
}

func TestTrustedDeviceCredentialDigestAndMask(t *testing.T) {
	plaintext := "  client-generated-credential-0123456789abcdef  "
	digest, err := trustedDeviceCredentialDigest(plaintext)
	require.NoError(t, err)

	expected := sha256.Sum256([]byte("client-generated-credential-0123456789abcdef"))
	assert.Equal(t, hex.EncodeToString(expected[:]), digest, "SHA-256 over the trimmed credential, lowercase hex")
	assert.Len(t, digest, 64)
	again, err := trustedDeviceCredentialDigest("client-generated-credential-0123456789abcdef")
	require.NoError(t, err)
	assert.Equal(t, digest, again)
	assert.NotContains(t, digest, "client-generated")

	mask := trustedDeviceCredentialMask(digest)
	assert.Equal(t, digest[:8]+"…", mask)
	assert.NotEqual(t, digest, mask)
	assert.Empty(t, trustedDeviceCredentialMask(""))
}

func TestTrustedDeviceCredentialStrength(t *testing.T) {
	valid := map[string]string{
		"lowercase hex 64":     "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"uppercase hex 64":     "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF",
		"base64url 32":         "abcdefghijklmnopqrstuvwxyz012345",
		"base64url with -_":    "abcdefghijklmnopqrstuvwxyz01234-_",
		"base64url 32 bytes":   "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8",
		"base64url 64 non-hex": "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
	}
	for name, credential := range valid {
		t.Run("accept "+name, func(t *testing.T) {
			err := trusteddevicesdomain.ValidateCredentialInput(credential)
			require.NoError(t, err)
			digest, err := trustedDeviceCredentialDigest(credential)
			require.NoError(t, err)
			assert.Len(t, digest, 64)
			assert.Equal(t, digest, strings.ToLower(digest), "落库摘要必须是小写 hex")
		})
	}

	missing := map[string]string{
		"empty":           "",
		"whitespace only": "   ",
	}
	for name, credential := range missing {
		t.Run("reject "+name, func(t *testing.T) {
			err := trusteddevicesdomain.ValidateCredentialInput(credential)
			require.Error(t, err)
			assertErrorCode(t, err, apperrors.CodeInvalidArgument)
			_, err = trustedDeviceCredentialDigest(credential)
			assertErrorCode(t, err, apperrors.CodeInvalidArgument)
		})
	}

	invalid := map[string]string{
		"31 base64url":   "abcdefghijklmnopqrstuvwxyz01234",
		"short hex":      "0123456789abcdef",
		"plus sign":      "abcdefghijklmnopqrstuvwxyz0123+4",
		"slash":          "abcdefghijklmnopqrstuvwxyz0123/4",
		"padding":        "abcdefghijklmnopqrstuvwxyz0123=",
		"internal space": "abcdefghijklmnopqrstuvwxyz 1234",
		"dot":            "abcdefghijklmnopqrstuvwxyz0123.4",
		"non-ascii":      "abcdefghijklmnopqrstuvwxyz0123é",
	}
	for name, credential := range invalid {
		t.Run("reject "+name, func(t *testing.T) {
			err := trusteddevicesdomain.ValidateCredentialInput(credential)
			require.Error(t, err)
			assertErrorCode(t, err, apperrors.CodeUnprocessableEntity)
			_, err = trustedDeviceCredentialDigest(credential)
			assertErrorCode(t, err, apperrors.CodeUnprocessableEntity)
		})
	}
}

func mustTrustedDeviceDigest(t *testing.T, credential string) string {
	t.Helper()
	digest, err := trustedDeviceCredentialDigest(credential)
	require.NoError(t, err)
	return digest
}

func TestTrustedDeviceViewMasksCredential(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	digest := mustTrustedDeviceDigest(t, "server-side-credential-never-returned")
	device := &trusteddevicesdomain.TrustedDevice{
		ID:                 "device-mask",
		OrganizationID:     "org-1",
		UserID:             "user-1",
		DeviceCredentialID: digest,
		CreatedAt:          now,
		LastSeenAt:         now,
		ExpiresAt:          now.Add(30 * 24 * time.Hour),
		Revision:           1,
		UpdatedAt:          now,
	}

	view := toTrustedDeviceView(device, now)
	assert.Equal(t, digest[:8]+"…", view.DeviceCredentialID)

	payload, err := json.Marshal(view)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), digest, "视图不得回显完整摘要")
	assert.NotContains(t, string(payload), "server-side-credential-never-returned")
}

func TestTrustedDeviceSatisfiesMfaUsesDigest(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	plaintext := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	digest := mustTrustedDeviceDigest(t, plaintext)

	repo := &capturingTrustedDeviceRepository{
		rows: []*trusteddevicesdomain.TrustedDevice{{
			ID:                 "device-1",
			OrganizationID:     "org-1",
			UserID:             "user-1",
			DeviceCredentialID: digest,
			UAHash:             trustedDeviceUAHash("test-agent"),
			CreatedAt:          now,
			LastSeenAt:         now,
			ExpiresAt:          now.Add(time.Hour),
			Revision:           1,
			UpdatedAt:          now,
		}},
	}
	svc := &Service{UOW: loginTestUnitOfWork{trustedDeviceRepo: repo}}

	assert.True(t, svc.trustedDeviceSatisfiesMfa(context.Background(), loginTestUnitOfWork{trustedDeviceRepo: repo}, "org-1", "user-1", plaintext, "test-agent", now))
	require.Len(t, repo.findCredentialIDs, 1)
	assert.Equal(t, digest, repo.findCredentialIDs[0], "登录判定只能把摘要送进仓储")
	assert.NotEqual(t, plaintext, repo.findCredentialIDs[0])
	assert.Equal(t, 1, repo.touchedLastSeen)
}

func TestTrustedDeviceSatisfiesMfaRejectsPlaintextExpiredRevoked(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	plaintext := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	digest := mustTrustedDeviceDigest(t, plaintext)
	revokedAt := now.Add(-time.Minute)

	cases := []struct {
		name string
		row  *trusteddevicesdomain.TrustedDevice
	}{
		{
			name: "legacy plaintext row",
			row: &trusteddevicesdomain.TrustedDevice{
				DeviceCredentialID: plaintext,
				ExpiresAt:          now.Add(time.Hour),
			},
		},
		{
			name: "expired row",
			row: &trusteddevicesdomain.TrustedDevice{
				DeviceCredentialID: digest,
				ExpiresAt:          now.Add(-time.Minute),
			},
		},
		{
			name: "revoked row",
			row: &trusteddevicesdomain.TrustedDevice{
				DeviceCredentialID: digest,
				ExpiresAt:          now.Add(time.Hour),
				RevokedAt:          &revokedAt,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.row.ID = "device-rejected"
			tc.row.OrganizationID = "org-1"
			tc.row.UserID = "user-1"
			tc.row.UAHash = trustedDeviceUAHash("test-agent")
			repo := &capturingTrustedDeviceRepository{rows: []*trusteddevicesdomain.TrustedDevice{tc.row}}
			svc := &Service{UOW: loginTestUnitOfWork{trustedDeviceRepo: repo}}

			assert.False(t, svc.trustedDeviceSatisfiesMfa(context.Background(), loginTestUnitOfWork{trustedDeviceRepo: repo}, "org-1", "user-1", plaintext, "test-agent", now))
			assert.Equal(t, 0, repo.touchedLastSeen, "未命中时不得更新 last_seen")
		})
	}
}

func TestRegisterTrustedDeviceStoresDigestAndMasksResponse(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	const (
		challengeID    = "challenge-register-1"
		userID         = "user-register-1"
		organizationID = "org-register-1"
		nonce          = "nonce-register-1"
		plaintext      = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	)
	digest := mustTrustedDeviceDigest(t, plaintext)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	message := trustedDeviceSignatureMessage(challengeID, userID, nonce)
	messageDigest := sha256.Sum256([]byte(message))
	signature, err := ecdsa.SignASN1(rand.Reader, key, messageDigest[:])
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	webRepo := webauthnmock.NewMockRepository(ctrl)
	webRepo.EXPECT().ConsumeChallenge(gomock.Any(), challengeID, gomock.Any()).Return(&webauthndomain.Challenge{
		ID:             challengeID,
		ChallengeHash:  trustedDeviceChallengeHash(nonce),
		CeremonyType:   webauthndomain.CeremonyTypeAuthentication,
		UserID:         userID,
		OrganizationID: organizationID,
		RelyingPartyID: trustedDeviceRelyingPartyID,
		ExpiresAt:      now.Add(time.Minute),
	}, nil)

	repo := &capturingTrustedDeviceRepository{}
	svc := &Service{
		Clock:         fakeClock{fixedTime: now},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "device-register-1"},
		UOW: loginTestUnitOfWork{
			trustedDeviceRepo: repo,
			webAuthnRepo:      webRepo,
		},
	}

	view, err := svc.RegisterTrustedDevice(context.Background(), TrustedDeviceRegisterCommand{
		OrganizationID:     organizationID,
		UserID:             userID,
		ChallengeID:        challengeID,
		Nonce:              nonce,
		DeviceCredentialID: plaintext,
		DevicePublicKey:    base64.StdEncoding.EncodeToString(der),
		Signature:          base64.StdEncoding.EncodeToString(signature),
		UserAgent:          "test-agent",
	})
	require.NoError(t, err)

	require.Len(t, repo.created, 1)
	assert.Equal(t, digest, repo.created[0].DeviceCredentialID, "落库的只能是摘要")
	assert.NotEqual(t, plaintext, repo.created[0].DeviceCredentialID)
	require.Len(t, repo.revokedCredentials, 1)
	assert.Equal(t, digest, repo.revokedCredentials[0], "吊销同样按摘要")

	assert.Equal(t, digest[:8]+"…", view.DeviceCredentialID, "注册响应只回掩码")
	assert.NotEqual(t, plaintext, view.DeviceCredentialID)
	assert.NotEqual(t, digest, view.DeviceCredentialID)
}

func TestRegisterTrustedDeviceRejectsWeakCredentialBeforePersistence(t *testing.T) {
	repo := &capturingTrustedDeviceRepository{}
	svc := &Service{
		Clock:         fakeClock{fixedTime: time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "device-weak-1"},
		UOW:           loginTestUnitOfWork{trustedDeviceRepo: repo},
	}

	t.Run("empty", func(t *testing.T) {
		_, err := svc.RegisterTrustedDevice(context.Background(), TrustedDeviceRegisterCommand{
			OrganizationID:     "org-1",
			UserID:             "user-1",
			ChallengeID:        "challenge-1",
			Nonce:              "nonce-1",
			DeviceCredentialID: "",
			DevicePublicKey:    "not-decoded",
			Signature:          "not-decoded",
		})
		assertErrorCode(t, err, apperrors.CodeInvalidArgument)
		assert.Empty(t, repo.created, "弱凭据不得进入持久化")
		assert.Empty(t, repo.revokedCredentials, "弱凭据不得触发吊销")
		assert.Empty(t, repo.findCredentialIDs, "弱凭据不得进入判定查询")
	})

	for name, credential := range map[string]string{
		"short":        "credential-1",
		"31 base64url": "abcdefghijklmnopqrstuvwxyz01234",
		"illegal char": "abcdefghijklmnopqrstuvwxyz0123+4",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := svc.RegisterTrustedDevice(context.Background(), TrustedDeviceRegisterCommand{
				OrganizationID:     "org-1",
				UserID:             "user-1",
				ChallengeID:        "challenge-1",
				Nonce:              "nonce-1",
				DeviceCredentialID: credential,
				DevicePublicKey:    "not-decoded",
				Signature:          "not-decoded",
			})
			assertErrorCode(t, err, apperrors.CodeUnprocessableEntity)
			assert.Empty(t, repo.created, "弱凭据不得进入持久化")
			assert.Empty(t, repo.revokedCredentials, "弱凭据不得触发吊销")
			assert.Empty(t, repo.findCredentialIDs, "弱凭据不得进入判定查询")
		})
	}
}

func TestTrustedDeviceSatisfiesMfaRejectsInvalidCredentialFormat(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	repo := &capturingTrustedDeviceRepository{}
	svc := &Service{UOW: loginTestUnitOfWork{trustedDeviceRepo: repo}}

	for _, credential := range []string{"", "credential-1", "abcdefghijklmnopqrstuvwxyz0123+4"} {
		assert.False(t, svc.trustedDeviceSatisfiesMfa(context.Background(), loginTestUnitOfWork{trustedDeviceRepo: repo}, "org-1", "user-1", credential, "test-agent", now))
	}
	assert.Empty(t, repo.findCredentialIDs, "非法凭据不得触达仓储查询")
	assert.Zero(t, repo.touchedLastSeen)
}
