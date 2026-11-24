package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	webauthnmock "github.com/guliangiam/guliangiam/internal/domain/webauthn/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformwebauthn "github.com/guliangiam/guliangiam/internal/platform/webauthn"
)

const (
	hardeningRPID   = "login.example.com"
	hardeningOrigin = "https://login.example.com"
)

// hardeningCredentialRepo keeps the WebAuthn credential rows in memory so the
// tests can observe what the transaction actually commits.
type hardeningCredentialRepo struct {
	*webauthnmock.MockRepository
	rows    map[string]*webauthndomain.Credential
	creates int
}

func newHardeningCredentialRepo(t *testing.T) *hardeningCredentialRepo {
	t.Helper()
	return &hardeningCredentialRepo{
		MockRepository: webauthnmock.NewMockRepository(gomock.NewController(t)),
		rows:           map[string]*webauthndomain.Credential{},
	}
}

func (r *hardeningCredentialRepo) Create(_ context.Context, credential *webauthndomain.Credential) error {
	r.creates++
	r.rows[string(credential.CredentialID)] = credential
	return nil
}

func (r *hardeningCredentialRepo) GetByCredentialID(_ context.Context, credentialID []byte) (*webauthndomain.Credential, error) {
	if credential, ok := r.rows[string(credentialID)]; ok {
		return credential, nil
	}
	return nil, apperrors.NewNotFound("webauthn credential not found")
}

func (r *hardeningCredentialRepo) ListByUserID(_ context.Context, userID string) ([]*webauthndomain.Credential, error) {
	out := make([]*webauthndomain.Credential, 0, len(r.rows))
	for _, credential := range r.rows {
		if credential.UserID == userID {
			out = append(out, credential)
		}
	}
	return out, nil
}

// hardeningUOW applies credential writes to a staged map and only merges them
// into the committed map when the transaction callback returns nil.
type hardeningUOW struct {
	loginTestUnitOfWork
	creds *hardeningCredentialRepo
	sess  sessiondomain.Repository
}

func (u hardeningUOW) WebAuthnCredentials() webauthndomain.Repository { return u.creds }
func (u hardeningUOW) Sessions() sessiondomain.Repository             { return u.sess }

func (u hardeningUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	committed := u.creds.rows
	staged := make(map[string]*webauthndomain.Credential, len(committed))
	for key, credential := range committed {
		staged[key] = credential
	}
	u.creds.rows = staged
	if err := fn(ctx, u); err != nil {
		u.creds.rows = committed // rollback
		return err
	}
	u.creds.rows = committed
	for key, credential := range staged {
		committed[key] = credential
	}
	return nil
}

type rotateSessionRepo struct {
	*loginTestSessionRepository
	old       *sessiondomain.Session
	createErr error
}

func (r *rotateSessionRepo) GetByID(context.Context, string) (*sessiondomain.Session, error) {
	return r.old, nil
}

func (r *rotateSessionRepo) Revoke(context.Context, string, time.Time) error { return nil }

func (r *rotateSessionRepo) Create(context.Context, *sessiondomain.Session) error {
	return r.createErr
}

func newHardeningService(t *testing.T, repo *hardeningCredentialRepo, sess sessiondomain.Repository) Service {
	t.Helper()
	t.Setenv("APP_ENV", "test")
	t.Setenv("CORS_ALLOWED_ORIGINS", hardeningOrigin)
	t.Setenv("ISSUER_URL", "")

	base := newTestUow(newTestOrg(), newTestUser(), newTestApp())
	uow := hardeningUOW{loginTestUnitOfWork: base, creds: repo, sess: sess}
	return Service{
		Clock:                  fakeClock{fixedTime: time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator:          fakeUUIDGenerator{fixedID: "019wer-0000-0000-0000-000000000001"},
		UOW:                    uow,
		WebAuthnAllowedOrigins: []string{hardeningOrigin},
	}
}

func hardeningAttestation(t *testing.T, credentialID []byte) (publicKey, authenticatorData, attestationObject []byte) {
	t.Helper()
	coseKey, _ := hardeningCOSEKey(t)
	authData := hardeningAuthenticatorData(hardeningRPID, credentialID, coseKey, 0)
	return coseKey, authData, hardeningAttestationObject(authData)
}

func hardeningClientData(t *testing.T, challenge, origin string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"type":      "webauthn.create",
		"challenge": base64.RawURLEncoding.EncodeToString([]byte(challenge)),
		"origin":    origin,
	})
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(payload)
}

func hardeningChallenge(challenge string) *webauthndomain.Challenge {
	return &webauthndomain.Challenge{
		ID:             "challenge-id",
		ChallengeHash:  webauthndomain.HashChallenge(challenge),
		CeremonyType:   webauthndomain.CeremonyTypeRegistration,
		UserID:         testUserID,
		OrganizationID: testOrgID,
		RelyingPartyID: hardeningRPID,
		ExpiresAt:      time.Date(2026, 11, 9, 10, 5, 0, 0, time.UTC),
	}
}

func requireHardeningCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %T", err)
	assert.Equal(t, code, appErr.Code)
}

func hardeningFinishCommand(t *testing.T, repo *hardeningCredentialRepo, challenge string) (WebAuthnRegistrationFinishCommand, []byte) {
	t.Helper()
	repo.MockRepository.EXPECT().
		ConsumeChallenge(gomock.Any(), "challenge-id", gomock.Any()).
		Return(hardeningChallenge(challenge), nil)

	credentialID := []byte("credential-id-21-byt")
	publicKey, authData, attestationObject := hardeningAttestation(t, credentialID)
	return WebAuthnRegistrationFinishCommand{
		OrganizationSlug:  testOrgSlug,
		Username:          testUsername,
		ChallengeID:       "challenge-id",
		Challenge:         challenge,
		CredentialID:      base64.RawURLEncoding.EncodeToString(credentialID),
		PublicKey:         base64.RawURLEncoding.EncodeToString(publicKey),
		ClientDataJSON:    hardeningClientData(t, challenge, hardeningOrigin),
		AuthenticatorData: base64.RawURLEncoding.EncodeToString(authData),
		AttestationObject: base64.RawURLEncoding.EncodeToString(attestationObject),
		AttestationType:   "none",
	}, credentialID
}

// ---------------------------------------------------------------------------
// Defect 2: excludeCredentials on begin
// ---------------------------------------------------------------------------

func TestWebAuthnRegistrationBeginIncludesOnlyUsersActiveCredentials(t *testing.T) {
	repo := newHardeningCredentialRepo(t)
	active := []byte("active-credential-01")
	repo.rows[string(active)] = &webauthndomain.Credential{
		ID: "cred-active", UserID: testUserID, CredentialID: active,
		Status: webauthndomain.CredentialStatusActive,
	}
	revoked := []byte("revoked-credential-1")
	repo.rows[string(revoked)] = &webauthndomain.Credential{
		ID: "cred-revoked", UserID: testUserID, CredentialID: revoked,
		Status: webauthndomain.CredentialStatusRevoked,
	}
	foreign := []byte("foreign-credential-1")
	repo.rows[string(foreign)] = &webauthndomain.Credential{
		ID: "cred-foreign", UserID: "another-user", CredentialID: foreign,
		Status: webauthndomain.CredentialStatusActive,
	}
	repo.MockRepository.EXPECT().CreateChallenge(gomock.Any(), gomock.Any()).Return(nil)

	svc := newHardeningService(t, repo, nil)
	result, err := svc.WebAuthnRegistrationBegin(context.Background(), WebAuthnRegistrationBeginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{base64.RawURLEncoding.EncodeToString(active)}, result.ExcludeCredentials)
}

func TestWebAuthnRegistrationBeginOmitsExcludeCredentialsWhenUserHasNone(t *testing.T) {
	repo := newHardeningCredentialRepo(t)
	repo.MockRepository.EXPECT().CreateChallenge(gomock.Any(), gomock.Any()).Return(nil)

	svc := newHardeningService(t, repo, nil)
	result, err := svc.WebAuthnRegistrationBegin(context.Background(), WebAuthnRegistrationBeginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
	})
	require.NoError(t, err)
	assert.Empty(t, result.ExcludeCredentials)
}

// begin 必须把 UV 策略下发给浏览器，且取值与服务端配置一致；只下发默认值
// 或只断言字段存在都无法证明「策略真的被读取」。
func TestWebAuthnRegistrationBeginAdvertisesUserVerificationFromPolicy(t *testing.T) {
	cases := []struct {
		name      string
		requireUV bool
		want      string
	}{
		{name: "default policy is preferred", requireUV: false, want: platformwebauthn.UserVerificationPreferred},
		{name: "deployment switch requires UV", requireUV: true, want: platformwebauthn.UserVerificationRequired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newHardeningCredentialRepo(t)
			repo.MockRepository.EXPECT().CreateChallenge(gomock.Any(), gomock.Any()).Return(nil)

			svc := newHardeningService(t, repo, nil)
			svc.WebAuthnRequireUV = tc.requireUV
			result, err := svc.WebAuthnRegistrationBegin(context.Background(), WebAuthnRegistrationBeginCommand{
				OrganizationSlug: testOrgSlug,
				Username:         testUsername,
			})
			require.NoError(t, err)
			assert.Equal(t, tc.want, result.UserVerification)
		})
	}
}

// 注册 finish 与 begin 同源：preferred 接受 UP-only 注册，required 必须拒绝
// flags.UV=false。两个子用例一起才证明校验读的是策略，而不是「反正客户端会带」。
func TestWebAuthnRegistrationFinishUserVerificationFollowsPolicy(t *testing.T) {
	cases := []struct {
		name      string
		requireUV bool
		wantErr   bool
	}{
		{name: "preferred accepts UP-only registration", requireUV: false},
		{name: "required rejects UP-only registration", requireUV: true, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newHardeningCredentialRepo(t)
			command, _ := hardeningFinishCommand(t, repo, "challenge-value")

			svc := newHardeningService(t, repo, nil)
			svc.WebAuthnRequireUV = tc.requireUV

			_, _, err := svc.WebAuthnRegistrationFinish(context.Background(), command)
			if tc.wantErr {
				requireHardeningCode(t, err, apperrors.CodeUnprocessableEntity)
				assert.Contains(t, err.Error(), "user verification")
				assert.Zero(t, repo.creates, "policy rejection must happen before any write")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, 1, repo.creates)
		})
	}
}

// ---------------------------------------------------------------------------
// Defect 1: duplicate credential pre-check
// ---------------------------------------------------------------------------

func TestWebAuthnRegistrationFinishRejectsDuplicateCredentialID(t *testing.T) {
	repo := newHardeningCredentialRepo(t)
	command, credentialID := hardeningFinishCommand(t, repo, "challenge-value")
	repo.rows[string(credentialID)] = &webauthndomain.Credential{
		ID: "existing-credential", UserID: "another-user", CredentialID: credentialID,
		Status: webauthndomain.CredentialStatusActive,
	}

	svc := newHardeningService(t, repo, nil)
	_, _, err := svc.WebAuthnRegistrationFinish(context.Background(), command)
	requireHardeningCode(t, err, apperrors.CodeAlreadyExists)
	assert.Zero(t, repo.creates, "duplicate pre-check must not reach Create")
}

// ---------------------------------------------------------------------------
// Defect 2: server-side attestation derivation / no forged public key
// ---------------------------------------------------------------------------

func TestWebAuthnRegistrationFinishPersistsDerivedMaterial(t *testing.T) {
	repo := newHardeningCredentialRepo(t)
	command, credentialID := hardeningFinishCommand(t, repo, "challenge-value")

	svc := newHardeningService(t, repo, nil)
	result, _, err := svc.WebAuthnRegistrationFinish(context.Background(), command)
	require.NoError(t, err)
	require.Equal(t, 1, repo.creates)

	credential := repo.rows[string(credentialID)]
	require.NotNil(t, credential)
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(credentialID), result.CredentialID)
	assert.Equal(t, "none", credential.AttestationType)
	assert.Equal(t, testUserID, credential.UserID)
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(credential.PublicKey), command.PublicKey)

	// 与管理员面共用同一派生入口：同一 attestationObject 得到同样的输出。
	derived, err := platformwebauthn.DeriveRegistrationMaterial(command.AttestationObject)
	require.NoError(t, err)
	assert.Equal(t, derived.CredentialIDB64, result.CredentialID)
	assert.Equal(t, derived.PublicKey, credential.PublicKey)
	assert.Equal(t, derived.AttestationType, credential.AttestationType)
}

func TestWebAuthnRegistrationFinishRejectsTamperedPublicKey(t *testing.T) {
	repo := newHardeningCredentialRepo(t)
	command, _ := hardeningFinishCommand(t, repo, "challenge-value")

	tampered, _ := hardeningCOSEKey(t)
	command.PublicKey = base64.RawURLEncoding.EncodeToString(tampered)

	svc := newHardeningService(t, repo, nil)
	_, _, err := svc.WebAuthnRegistrationFinish(context.Background(), command)
	requireHardeningCode(t, err, apperrors.CodeUnprocessableEntity)
	assert.Zero(t, repo.creates, "public key that does not match the attestation must be rejected before Create")
}

// ---------------------------------------------------------------------------
// Defect 3: no half-written credential when a later step fails
// ---------------------------------------------------------------------------

func TestWebAuthnRegistrationFinishRollsBackCredentialWhenSessionRotationFails(t *testing.T) {
	repo := newHardeningCredentialRepo(t)
	command, _ := hardeningFinishCommand(t, repo, "challenge-value")
	rotationErr := errors.New("injected session rotation failure")
	sess := &rotateSessionRepo{
		loginTestSessionRepository: &loginTestSessionRepository{},
		old: &sessiondomain.Session{
			ID:               "old-session-id",
			UserID:           testUserID,
			Username:         testUsername,
			OrganizationID:   testOrgID,
			OrganizationSlug: testOrgSlug,
		},
		createErr: rotationErr,
	}
	command.PreviousSessionID = "old-session-id"

	svc := newHardeningService(t, repo, sess)
	_, _, err := svc.WebAuthnRegistrationFinish(context.Background(), command)
	require.ErrorIs(t, err, rotationErr)
	assert.Equal(t, 1, repo.creates, "credential Create happens before the injected failure")
	assert.Empty(t, repo.rows, "failed transaction must not leave a credential row behind")
}

// ---------------------------------------------------------------------------
// CBOR / authenticatorData builders (same fixture shape as the admin path)
// ---------------------------------------------------------------------------

func hardeningEncodeHead(major byte, value uint64) []byte {
	switch {
	case value < 24:
		return []byte{major<<5 | byte(value)}
	case value <= 0xff:
		return []byte{major<<5 | 24, byte(value)}
	case value <= 0xffff:
		return []byte{major<<5 | 25, byte(value >> 8), byte(value)}
	default:
		return []byte{major<<5 | 26, byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value)}
	}
}

func hardeningEncodeInt(v int) []byte {
	if v >= 0 {
		return hardeningEncodeHead(0, uint64(v))
	}
	return hardeningEncodeHead(1, uint64(-1-v))
}

func hardeningByteString(b []byte) []byte {
	return append(hardeningEncodeHead(2, uint64(len(b))), b...)
}

func hardeningTextString(s string) []byte {
	return append(hardeningEncodeHead(3, uint64(len(s))), []byte(s)...)
}

func hardeningCOSEKey(t *testing.T) ([]byte, *ecdsa.PublicKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	x := key.PublicKey.X.FillBytes(make([]byte, 32))
	y := key.PublicKey.Y.FillBytes(make([]byte, 32))
	out := []byte{0xa5}
	out = append(out, hardeningEncodeInt(1)...)
	out = append(out, hardeningEncodeInt(2)...) // kty: EC2
	out = append(out, hardeningEncodeInt(3)...)
	out = append(out, hardeningEncodeInt(-7)...) // alg: ES256
	out = append(out, hardeningEncodeInt(-1)...)
	out = append(out, hardeningEncodeInt(1)...) // crv: P-256
	out = append(out, hardeningEncodeInt(-2)...)
	out = append(out, hardeningByteString(x)...)
	out = append(out, hardeningEncodeInt(-3)...)
	out = append(out, hardeningByteString(y)...)
	return out, &key.PublicKey
}

func hardeningAuthenticatorData(rpID string, credentialID, coseKey []byte, signCount uint32) []byte {
	hash := sha256.Sum256([]byte(rpID))
	out := make([]byte, 0, 37+16+2+len(credentialID)+len(coseKey))
	out = append(out, hash[:]...)
	out = append(out, 0x41) // UP + AT
	out = append(out, byte(signCount>>24), byte(signCount>>16), byte(signCount>>8), byte(signCount))
	out = append(out, make([]byte, 16)...)
	out = append(out, byte(len(credentialID)>>8), byte(len(credentialID)))
	out = append(out, credentialID...)
	out = append(out, coseKey...)
	return out
}

func hardeningAttestationObject(authData []byte) []byte {
	out := []byte{0xa3}
	out = append(out, hardeningTextString("fmt")...)
	out = append(out, hardeningTextString("none")...)
	out = append(out, hardeningTextString("attStmt")...)
	out = append(out, 0xa0)
	out = append(out, hardeningTextString("authData")...)
	out = append(out, hardeningByteString(authData)...)
	return out
}
