package users

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

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// WebAuthn admin test harness
// ---------------------------------------------------------------------------

type waMockRepo struct {
	createFn            func(ctx context.Context, credential *webauthndomain.Credential) error
	getByCredentialIDFn func(ctx context.Context, credentialID []byte) (*webauthndomain.Credential, error)
	listByUserIDFn      func(ctx context.Context, userID string) ([]*webauthndomain.Credential, error)
	createChallengeFn   func(ctx context.Context, challenge *webauthndomain.Challenge) error
	consumeChallengeFn  func(ctx context.Context, id string, now time.Time) (*webauthndomain.Challenge, error)
	updateFn            func(ctx context.Context, credential *webauthndomain.Credential) error
	updateStatusFn      func(ctx context.Context, id string, status webauthndomain.CredentialStatus) error
	deleteFn            func(ctx context.Context, id string) error
}

func (r *waMockRepo) Create(ctx context.Context, credential *webauthndomain.Credential) error {
	if r.createFn != nil {
		return r.createFn(ctx, credential)
	}
	return nil
}

func (r *waMockRepo) GetByCredentialID(ctx context.Context, credentialID []byte) (*webauthndomain.Credential, error) {
	if r.getByCredentialIDFn != nil {
		return r.getByCredentialIDFn(ctx, credentialID)
	}
	return nil, apperrors.NewNotFound("credential not found")
}

func (r *waMockRepo) ListByUserID(ctx context.Context, userID string) ([]*webauthndomain.Credential, error) {
	if r.listByUserIDFn != nil {
		return r.listByUserIDFn(ctx, userID)
	}
	return nil, nil
}

func (r *waMockRepo) UpdateSignCount(context.Context, string, int64) error { return nil }

func (r *waMockRepo) Update(ctx context.Context, credential *webauthndomain.Credential) error {
	if r.updateFn != nil {
		return r.updateFn(ctx, credential)
	}
	return nil
}

func (r *waMockRepo) UpdateStatus(ctx context.Context, id string, status webauthndomain.CredentialStatus) error {
	if r.updateStatusFn != nil {
		return r.updateStatusFn(ctx, id, status)
	}
	return nil
}

func (r *waMockRepo) Delete(ctx context.Context, id string) error {
	if r.deleteFn != nil {
		return r.deleteFn(ctx, id)
	}
	return nil
}

func (r *waMockRepo) CreateChallenge(ctx context.Context, challenge *webauthndomain.Challenge) error {
	if r.createChallengeFn != nil {
		return r.createChallengeFn(ctx, challenge)
	}
	return nil
}

func (r *waMockRepo) ConsumeChallenge(ctx context.Context, id string, now time.Time) (*webauthndomain.Challenge, error) {
	if r.consumeChallengeFn != nil {
		return r.consumeChallengeFn(ctx, id, now)
	}
	return nil, apperrors.NewNotFound("challenge not found")
}

func (r *waMockRepo) DeleteExpiredChallenges(context.Context, time.Time) (int64, error) {
	return 0, nil
}

// waTestUoW overrides the WebAuthn repository while reusing the users test UoW.
type waTestUoW struct {
	svcTestUnitOfWork
	webauthn webauthndomain.Repository
}

func (u waTestUoW) WebAuthnCredentials() webauthndomain.Repository { return u.webauthn }

func (u waTestUoW) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

var waFixedNow = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

func newAdminWebAuthnService(t *testing.T, repo *waMockRepo, org repoOrgFixture, uuidValue string) *Service {
	t.Helper()
	usersRepo := &svcMockUserRepo{
		getByIDFn: func(context.Context, string) (*userdomain.User, error) {
			return newTestUser(), nil
		},
		getByIDForOrganizationFn: func(_ context.Context, id, organizationID string) (*userdomain.User, error) {
			if id != testUserID || organizationID != testOrganizationID {
				return nil, apperrors.NewNotFound("user not found")
			}
			return newTestUser(), nil
		},
	}
	organizations := &svcMockOrgRepo{getByIDFn: org.getByID}
	uow := waTestUoW{
		svcTestUnitOfWork: svcTestUnitOfWork{users: usersRepo, organizations: organizations},
		webauthn:          repo,
	}
	return &Service{
		UOW:   uow,
		Clock: svcFakeClock{fixedTime: waFixedNow},
		UUID:  svcFakeUUID{fixedID: uuidValue},
	}
}

// ---------------------------------------------------------------------------
// Fixtures for the org repository.
// ---------------------------------------------------------------------------

type repoOrgFixture struct {
	organization *orgdomain.Organization
	getByID      func(ctx context.Context, id string) (*orgdomain.Organization, error)
}

func newOrgFixture() repoOrgFixture {
	org := newTestOrganization()
	return repoOrgFixture{
		organization: org,
		getByID: func(_ context.Context, id string) (*orgdomain.Organization, error) {
			if id != testOrganizationID {
				return nil, apperrors.NewNotFound("organization not found")
			}
			return org, nil
		},
	}
}

// ---------------------------------------------------------------------------
// CBOR / authenticatorData builders (mirror internal/platform/webauthn tests)
// ---------------------------------------------------------------------------

func waEncodeHead(major byte, value uint64) []byte {
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

func waEncodeInt(v int) []byte {
	if v >= 0 {
		return waEncodeHead(0, uint64(v))
	}
	return waEncodeHead(1, uint64(-1-v))
}

func waByteString(b []byte) []byte {
	return append(waEncodeHead(2, uint64(len(b))), b...)
}

func waTextString(s string) []byte {
	return append(waEncodeHead(3, uint64(len(s))), []byte(s)...)
}

func waCOSEKey(t *testing.T) ([]byte, *ecdsa.PublicKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	x := key.PublicKey.X.FillBytes(make([]byte, 32))
	y := key.PublicKey.Y.FillBytes(make([]byte, 32))
	out := []byte{0xa5}
	out = append(out, waEncodeInt(1)...)
	out = append(out, waEncodeInt(2)...) // kty: EC2
	out = append(out, waEncodeInt(3)...)
	out = append(out, waEncodeInt(-7)...) // alg: ES256
	out = append(out, waEncodeInt(-1)...)
	out = append(out, waEncodeInt(1)...) // crv: P-256
	out = append(out, waEncodeInt(-2)...)
	out = append(out, waByteString(x)...)
	out = append(out, waEncodeInt(-3)...)
	out = append(out, waByteString(y)...)
	return out, &key.PublicKey
}

func waAuthenticatorData(rpID string, credentialID, coseKey []byte, signCount uint32) []byte {
	// rpIdHash | flags=UP+AT | signCount | aaguid(16) | credLen | credentialId | COSE key
	hash := sha256.Sum256([]byte(rpID))
	out := make([]byte, 0, 37+16+2+len(credentialID)+len(coseKey))
	out = append(out, hash[:]...)
	out = append(out, 0x41)
	out = append(out, byte(signCount>>24), byte(signCount>>16), byte(signCount>>8), byte(signCount))
	out = append(out, make([]byte, 16)...)
	out = append(out, byte(len(credentialID)>>8), byte(len(credentialID)))
	out = append(out, credentialID...)
	out = append(out, coseKey...)
	return out
}

func waAttestationObject(authData []byte) []byte {
	out := []byte{0xa3}
	out = append(out, waTextString("fmt")...)
	out = append(out, waTextString("none")...)
	out = append(out, waTextString("attStmt")...)
	out = append(out, 0xa0)
	out = append(out, waTextString("authData")...)
	out = append(out, waByteString(authData)...)
	return out
}

func waClientDataJSON(t *testing.T, challenge, origin string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"type":      "webauthn.create",
		"challenge": base64.RawURLEncoding.EncodeToString([]byte(challenge)),
		"origin":    origin,
	})
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(payload)
}

// ---------------------------------------------------------------------------
// Fixtures for the happy path
// ---------------------------------------------------------------------------

const (
	waRPID   = "app.example.com"
	waOrigin = "https://app.example.com"
)

func newValidFinishFixture(t *testing.T, repo *waMockRepo, challenge string) (AdminWebAuthnRegistrationFinishCommand, []byte, []byte) {
	t.Helper()
	t.Setenv("CORS_ALLOWED_ORIGINS", waOrigin)
	t.Setenv("ISSUER_URL", "")

	credentialID := []byte("credential-id-21-byt")
	coseKey, _ := waCOSEKey(t)
	authData := waAuthenticatorData(waRPID, credentialID, coseKey, 0)
	attestationObject := waAttestationObject(authData)

	command := AdminWebAuthnRegistrationFinishCommand{
		OrganizationID:    testOrganizationID,
		UserID:            testUserID,
		ChallengeID:       "challenge-id",
		Challenge:         challenge,
		ClientDataJSON:    waClientDataJSON(t, challenge, waOrigin),
		AttestationObject: base64.RawURLEncoding.EncodeToString(attestationObject),
	}
	return command, credentialID, coseKey
}

func withValidChallenge(challenge string) func(context.Context, string, time.Time) (*webauthndomain.Challenge, error) {
	return func(context.Context, string, time.Time) (*webauthndomain.Challenge, error) {
		return &webauthndomain.Challenge{
			ID:             "challenge-id",
			ChallengeHash:  webauthndomain.HashChallenge(challenge),
			CeremonyType:   webauthndomain.CeremonyTypeRegistration,
			UserID:         testUserID,
			OrganizationID: testOrganizationID,
			RelyingPartyID: waRPID,
			ExpiresAt:      waFixedNow.Add(5 * time.Minute),
		}, nil
	}
}

func requireAppCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %T", err)
	assert.Equal(t, code, appErr.Code)
}

// ---------------------------------------------------------------------------
// GET detail
// ---------------------------------------------------------------------------

func TestGetUserWebAuthnCredential_WrongUserOrTenantIsNotFound(t *testing.T) {
	repo := &waMockRepo{
		getByCredentialIDFn: func(context.Context, []byte) (*webauthndomain.Credential, error) {
			return &webauthndomain.Credential{ID: "cred", UserID: "someone-else", Status: webauthndomain.CredentialStatusActive}, nil
		},
	}
	svc := newAdminWebAuthnService(t, repo, newOrgFixture(), "unused")

	_, err := svc.GetUserWebAuthnCredential(context.Background(), testOrganizationID, testUserID, base64.RawURLEncoding.EncodeToString([]byte("cred-id")))
	requireAppCode(t, err, apperrors.CodeNotFound)

	_, err = svc.GetUserWebAuthnCredential(context.Background(), "foreign-org", testUserID, base64.RawURLEncoding.EncodeToString([]byte("cred-id")))
	requireAppCode(t, err, apperrors.CodeNotFound)
}

func TestGetUserWebAuthnCredential_InvalidCredentialID(t *testing.T) {
	svc := newAdminWebAuthnService(t, &waMockRepo{}, newOrgFixture(), "unused")
	_, err := svc.GetUserWebAuthnCredential(context.Background(), testOrganizationID, testUserID, "!!!")
	requireAppCode(t, err, apperrors.CodeUnprocessableEntity)
}

// ---------------------------------------------------------------------------
// Rename
// ---------------------------------------------------------------------------

func TestRenameUserWebAuthnCredential_TrimsWhitespace(t *testing.T) {
	var updated *webauthndomain.Credential
	repo := &waMockRepo{
		getByCredentialIDFn: func(context.Context, []byte) (*webauthndomain.Credential, error) {
			return &webauthndomain.Credential{
				ID:           "cred",
				UserID:       testUserID,
				CredentialID: []byte("cred-id"),
				Name:         "old",
				Status:       webauthndomain.CredentialStatusActive,
				UpdatedAt:    waFixedNow,
			}, nil
		},
		updateFn: func(_ context.Context, credential *webauthndomain.Credential) error {
			updated = credential
			return nil
		},
	}
	svc := newAdminWebAuthnService(t, repo, newOrgFixture(), "unused")

	item, err := svc.RenameUserWebAuthnCredential(context.Background(), RenameUserWebAuthnCredentialCommand{
		OrganizationID: testOrganizationID,
		UserID:         testUserID,
		CredentialID:   base64.RawURLEncoding.EncodeToString([]byte("cred-id")),
		Name:           "  My Passkey  ",
	})
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "My Passkey", updated.Name)
	assert.Equal(t, waFixedNow, updated.UpdatedAt)
	assert.Equal(t, "My Passkey", item.Name)
	assert.Equal(t, webauthndomain.CredentialStatusActive, webauthndomain.CredentialStatus(item.Status))
	assert.Equal(t, base64.RawURLEncoding.EncodeToString([]byte("cred-id")), item.CredentialID)
	assert.NotEmpty(t, item.UpdatedAt)
}

func TestRenameUserWebAuthnCredential_RejectsBlankAndOversizeName(t *testing.T) {
	validID := base64.RawURLEncoding.EncodeToString([]byte("cred-id"))
	for _, tc := range []struct {
		name string
		val  string
		want apperrors.Code
	}{
		{"blank", "   ", apperrors.CodeInvalidArgument},
		{"oversize", string(make([]rune, 256)), apperrors.CodeUnprocessableEntity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &waMockRepo{}
			svc := newAdminWebAuthnService(t, repo, newOrgFixture(), "unused")
			_, err := svc.RenameUserWebAuthnCredential(context.Background(), RenameUserWebAuthnCredentialCommand{
				OrganizationID: testOrganizationID,
				UserID:         testUserID,
				CredentialID:   validID,
				Name:           tc.val,
			})
			requireAppCode(t, err, tc.want)
		})
	}
}

// ---------------------------------------------------------------------------
// Begin
// ---------------------------------------------------------------------------

func TestBeginAdminWebAuthnRegistration_PersistsOnlyHash(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", waOrigin)
	t.Setenv("ISSUER_URL", "")

	var stored *webauthndomain.Challenge
	repo := &waMockRepo{
		createChallengeFn: func(_ context.Context, challenge *webauthndomain.Challenge) error {
			stored = challenge
			return nil
		},
	}
	svc := newAdminWebAuthnService(t, repo, newOrgFixture(), "challenge-id")

	result, err := svc.BeginAdminWebAuthnRegistration(context.Background(), AdminWebAuthnRegistrationBeginCommand{
		OrganizationID: testOrganizationID,
		UserID:         testUserID,
	})
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.NotEmpty(t, result.Challenge)
	assert.NotEqual(t, result.Challenge, stored.ChallengeHash)
	assert.Equal(t, webauthndomain.HashChallenge(result.Challenge), stored.ChallengeHash)
	assert.Equal(t, "challenge-id", result.ChallengeID)
	assert.Equal(t, waRPID, result.RelyingPartyID)
	assert.Equal(t, webauthndomain.CeremonyTypeRegistration, stored.CeremonyType)
	assert.Equal(t, testUserID, stored.UserID)
	assert.Equal(t, testOrganizationID, stored.OrganizationID)
	assert.Equal(t, waFixedNow.Add(5*time.Minute), stored.ExpiresAt)
	assert.Len(t, stored.ChallengeHash, 64)

	// userId is a stable opaque handle, never the raw UUID.
	assert.NotEqual(t, testUserID, result.UserID)
	sum := sha256.Sum256([]byte(testUserID))
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(sum[:32]), result.UserID)
}

func TestBeginAdminWebAuthnRegistration_ExcludesUsersActiveCredentials(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", waOrigin)
	t.Setenv("ISSUER_URL", "")

	active := []byte("active-credential-01")
	revoked := []byte("revoked-credential-1")
	repo := &waMockRepo{
		listByUserIDFn: func(_ context.Context, userID string) ([]*webauthndomain.Credential, error) {
			require.Equal(t, testUserID, userID)
			return []*webauthndomain.Credential{
				{ID: "active", UserID: testUserID, CredentialID: active, Status: webauthndomain.CredentialStatusActive},
				{ID: "revoked", UserID: testUserID, CredentialID: revoked, Status: webauthndomain.CredentialStatusRevoked},
			}, nil
		},
	}
	svc := newAdminWebAuthnService(t, repo, newOrgFixture(), "challenge-id")

	result, err := svc.BeginAdminWebAuthnRegistration(context.Background(), AdminWebAuthnRegistrationBeginCommand{
		OrganizationID: testOrganizationID,
		UserID:         testUserID,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{base64.RawURLEncoding.EncodeToString(active)}, result.ExcludeCredentials)
}

func TestBeginAdminWebAuthnRegistration_AdvertisesUserVerificationFromPolicy(t *testing.T) {
	t.Run("default policy is preferred", func(t *testing.T) {
		t.Setenv("CORS_ALLOWED_ORIGINS", waOrigin)
		t.Setenv("ISSUER_URL", "")
		svc := newAdminWebAuthnService(t, &waMockRepo{}, newOrgFixture(), "challenge-id")

		result, err := svc.BeginAdminWebAuthnRegistration(context.Background(), AdminWebAuthnRegistrationBeginCommand{
			OrganizationID: testOrganizationID,
			UserID:         testUserID,
		})
		require.NoError(t, err)
		assert.Equal(t, "preferred", result.UserVerification)
	})

	t.Run("organization policy requires UV", func(t *testing.T) {
		t.Setenv("CORS_ALLOWED_ORIGINS", waOrigin)
		t.Setenv("ISSUER_URL", "")
		orgFixture := newOrgFixture()
		orgFixture.organization.Security.MfaPolicy = map[string]any{"requireUV": true}
		svc := newAdminWebAuthnService(t, &waMockRepo{}, orgFixture, "challenge-id")

		result, err := svc.BeginAdminWebAuthnRegistration(context.Background(), AdminWebAuthnRegistrationBeginCommand{
			OrganizationID: testOrganizationID,
			UserID:         testUserID,
		})
		require.NoError(t, err)
		assert.Equal(t, "required", result.UserVerification)
	})
}

func TestBeginAdminWebAuthnRegistration_UserOutsideTenantIsNotFound(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", waOrigin)
	t.Setenv("ISSUER_URL", "")
	svc := newAdminWebAuthnService(t, &waMockRepo{}, newOrgFixture(), "challenge-id")

	_, err := svc.BeginAdminWebAuthnRegistration(context.Background(), AdminWebAuthnRegistrationBeginCommand{
		OrganizationID: "foreign-org",
		UserID:         testUserID,
	})
	requireAppCode(t, err, apperrors.CodeNotFound)
}

func TestBeginAdminWebAuthnRegistration_PasskeyDisabledIsForbidden(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", waOrigin)
	t.Setenv("ISSUER_URL", "")

	orgFixture := newOrgFixture()
	orgFixture.organization.Security.MfaPolicy = map[string]any{"passkeyEnabled": false}
	svc := newAdminWebAuthnService(t, &waMockRepo{}, orgFixture, "challenge-id")

	_, err := svc.BeginAdminWebAuthnRegistration(context.Background(), AdminWebAuthnRegistrationBeginCommand{
		OrganizationID: testOrganizationID,
		UserID:         testUserID,
	})
	requireAppCode(t, err, apperrors.CodeForbidden)
}

// ---------------------------------------------------------------------------
// Finish
// ---------------------------------------------------------------------------

func TestFinishAdminWebAuthnRegistration_ExpiredOrConsumedChallengeIsGone(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", waOrigin)
	t.Setenv("ISSUER_URL", "")
	repo := &waMockRepo{
		consumeChallengeFn: func(context.Context, string, time.Time) (*webauthndomain.Challenge, error) {
			return nil, apperrors.NewNotFound("challenge not found")
		},
	}
	svc := newAdminWebAuthnService(t, repo, newOrgFixture(), "cred-row-id")

	_, err := svc.FinishAdminWebAuthnRegistration(context.Background(), AdminWebAuthnRegistrationFinishCommand{
		OrganizationID:    testOrganizationID,
		UserID:            testUserID,
		ChallengeID:       "challenge-id",
		Challenge:         "challenge",
		ClientDataJSON:    "eyJ0eXBlIjoid2ViYXV0aG4uY3JlYXRlIn0", // arbitrary non-empty
		AttestationObject: "Y2Jvcg",                              // arbitrary non-empty
	})
	requireAppCode(t, err, apperrors.CodeGone)
}

func TestFinishAdminWebAuthnRegistration_MismatchedUserOrTenantIsForbidden(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", waOrigin)
	t.Setenv("ISSUER_URL", "")

	cases := []struct {
		name   string
		userID string
		orgID  string
	}{
		{"user", "another-user", testOrganizationID},
		{"tenant", testUserID, "another-org"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &waMockRepo{
				consumeChallengeFn: func(context.Context, string, time.Time) (*webauthndomain.Challenge, error) {
					return &webauthndomain.Challenge{
						ID:             "challenge-id",
						ChallengeHash:  webauthndomain.HashChallenge("challenge"),
						CeremonyType:   webauthndomain.CeremonyTypeRegistration,
						UserID:         tc.userID,
						OrganizationID: tc.orgID,
						RelyingPartyID: waRPID,
					}, nil
				},
			}
			svc := newAdminWebAuthnService(t, repo, newOrgFixture(), "cred-row-id")
			_, err := svc.FinishAdminWebAuthnRegistration(context.Background(), AdminWebAuthnRegistrationFinishCommand{
				OrganizationID:    testOrganizationID,
				UserID:            testUserID,
				ChallengeID:       "challenge-id",
				Challenge:         "challenge",
				ClientDataJSON:    "eA",
				AttestationObject: "eA",
			})
			requireAppCode(t, err, apperrors.CodeForbidden)
		})
	}
}

func TestFinishAdminWebAuthnRegistration_DuplicateCredentialIsAlreadyExists(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", waOrigin)
	t.Setenv("ISSUER_URL", "")
	challenge := "duplicate-challenge"
	repo := &waMockRepo{
		consumeChallengeFn: withValidChallenge(challenge),
		getByCredentialIDFn: func(context.Context, []byte) (*webauthndomain.Credential, error) {
			return &webauthndomain.Credential{ID: "existing", UserID: testUserID}, nil
		},
	}
	svc := newAdminWebAuthnService(t, repo, newOrgFixture(), "cred-row-id")
	command, _, _ := newValidFinishFixture(t, repo, challenge)

	_, err := svc.FinishAdminWebAuthnRegistration(context.Background(), command)
	requireAppCode(t, err, apperrors.CodeAlreadyExists)
}

func TestFinishAdminWebAuthnRegistration_DerivesMaterialFromAuthenticatorData(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", waOrigin)
	t.Setenv("ISSUER_URL", "")
	challenge := "derive-challenge"

	var created *webauthndomain.Credential
	repo := &waMockRepo{
		consumeChallengeFn: withValidChallenge(challenge),
		createFn: func(_ context.Context, credential *webauthndomain.Credential) error {
			created = credential
			return nil
		},
	}
	svc := newAdminWebAuthnService(t, repo, newOrgFixture(), "cred-row-id")
	command, credentialID, coseKey := newValidFinishFixture(t, repo, challenge)
	// Deliberately omit credentialId: the server must derive it from attestationObject.
	require.Empty(t, command.CredentialID)

	item, err := svc.FinishAdminWebAuthnRegistration(context.Background(), command)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, credentialID, created.CredentialID)
	assert.Equal(t, coseKey, created.PublicKey)
	assert.Equal(t, webauthndomain.CredentialStatusActive, created.Status)
	assert.Equal(t, int64(0), created.SignCount)
	assert.Equal(t, "Passkey", created.Name)
	assert.Equal(t, "none", created.AttestationType)
	assert.Equal(t, "cred-row-id", created.ID)

	assert.Equal(t, base64.RawURLEncoding.EncodeToString(credentialID), item.CredentialID)
	assert.Equal(t, "active", item.Status)
	assert.Equal(t, "none", item.AttestationType)
	assert.NotEmpty(t, item.UpdatedAt)
}

// 注册阶段的 signCount 不是「必须为 0」：CTAP2 虚拟认证器与部分平台 Passkey
// 首次注册即上报 1。这里覆盖非 0 起始值的合法注册，并要求把实际值持久化，
// 供后续认证的克隆检测基线使用。
func TestFinishAdminWebAuthnRegistration_AcceptsNonZeroRegistrationSignCount(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", waOrigin)
	t.Setenv("ISSUER_URL", "")
	challenge := "nonzero-sign-count"

	var created *webauthndomain.Credential
	repo := &waMockRepo{
		consumeChallengeFn: withValidChallenge(challenge),
		createFn: func(_ context.Context, credential *webauthndomain.Credential) error {
			created = credential
			return nil
		},
	}
	svc := newAdminWebAuthnService(t, repo, newOrgFixture(), "cred-row-id")

	credentialID := []byte("credential-id-21-byt")
	coseKey, _ := waCOSEKey(t)
	authData := waAuthenticatorData(waRPID, credentialID, coseKey, 1)
	attestationObject := waAttestationObject(authData)
	command := AdminWebAuthnRegistrationFinishCommand{
		OrganizationID:    testOrganizationID,
		UserID:            testUserID,
		ChallengeID:       "challenge-id",
		Challenge:         challenge,
		ClientDataJSON:    waClientDataJSON(t, challenge, waOrigin),
		AttestationObject: base64.RawURLEncoding.EncodeToString(attestationObject),
	}

	item, err := svc.FinishAdminWebAuthnRegistration(context.Background(), command)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, credentialID, created.CredentialID)
	assert.Equal(t, int64(1), created.SignCount)
	assert.Equal(t, int64(1), item.SignCount)
}

func TestFinishAdminWebAuthnRegistration_PasskeyDisabledIsForbidden(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", waOrigin)
	t.Setenv("ISSUER_URL", "")

	orgFixture := newOrgFixture()
	orgFixture.organization.Security.MfaPolicy = map[string]any{"passkeyEnabled": false}
	repo := &waMockRepo{consumeChallengeFn: withValidChallenge("challenge")}
	svc := newAdminWebAuthnService(t, repo, orgFixture, "cred-row-id")

	_, err := svc.FinishAdminWebAuthnRegistration(context.Background(), AdminWebAuthnRegistrationFinishCommand{
		OrganizationID:    testOrganizationID,
		UserID:            testUserID,
		ChallengeID:       "challenge-id",
		Challenge:         "challenge",
		ClientDataJSON:    "eA",
		AttestationObject: "eA",
	})
	requireAppCode(t, err, apperrors.CodeForbidden)
}
