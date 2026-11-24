package verifications

import (
	"context"
	"testing"
	"time"

	organizationdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	verificationsdomain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	rotationCurrentRoot  = "verification-rotation-current-root"
	rotationPreviousRoot = "verification-rotation-previous-root"
	rotationLegacyRoot   = "verification-rotation-legacy-root"
	rotationRetiredRoot  = "verification-rotation-retired-root"
	rotationRecipient    = "+8613800000001"
	rotationCode         = "654321"
)

func setVerificationRootKeys(t *testing.T, current, previous, legacy string) {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY_CURRENT", current)
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", previous)
	t.Setenv("ENCRYPTION_KEY", legacy)
}

func pepperForRoot(t *testing.T, root string) []byte {
	t.Helper()
	pepper, err := verificationPepperFromRoot(root)
	require.NoError(t, err)
	return pepper
}

func digestFromRoot(t *testing.T, algorithm, root string) string {
	t.Helper()
	digest, err := verificationCodeDigest(algorithm, pepperForRoot(t, root),
		testOrganizationID, VerificationTypePhone, rotationRecipient, rotationCode)
	require.NoError(t, err)
	return digest
}

func rotationRepo(t *testing.T, storedDigest string) *fakeVerificationRepo {
	t.Helper()
	return &fakeVerificationRepo{active: &verificationsdomain.Verification{
		ID:             "verification-rotation",
		OrganizationID: testOrganizationID,
		Type:           VerificationTypePhone,
		Recipient:      rotationRecipient,
		Code:           storedDigest,
		Status:         "pending",
		ExpiresAt:      fakeClock{}.Now().Add(5 * time.Minute),
	}}
}

func rotationHitCount(t *testing.T, generation string) float64 {
	t.Helper()
	metric := &dto.Metric{}
	require.NoError(t, verificationPepperRotationHitTotal.
		WithLabelValues(generation, VerificationTypePhone).Write(metric))
	return metric.GetCounter().GetValue()
}

func TestVerificationPepperCandidates_OrderSkipsEmptyAndDeduplicates(t *testing.T) {
	setVerificationRootKeys(t, rotationCurrentRoot, rotationPreviousRoot, rotationLegacyRoot)
	candidates, err := verificationCodePepperCandidates()
	require.NoError(t, err)
	require.Len(t, candidates, 3)
	assert.Equal(t, verificationPepperCurrent, candidates[0].generation)
	assert.Equal(t, verificationPepperPrevious, candidates[1].generation)
	assert.Equal(t, verificationPepperLegacy, candidates[2].generation)
	for _, candidate := range candidates {
		assert.NotEmpty(t, candidate.pepper)
	}

	// PREVIOUS equal to CURRENT must not double-count: the earlier generation wins.
	setVerificationRootKeys(t, rotationCurrentRoot, rotationCurrentRoot, "")
	candidates, err = verificationCodePepperCandidates()
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, verificationPepperCurrent, candidates[0].generation)
}

func TestVerificationPepperCandidates_EmptyKeysReturnUnavailable(t *testing.T) {
	setVerificationRootKeys(t, "", "", "")
	_, err := verificationCodePepperCandidates()
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnavailable, apperrors.FromError(err).Code)
}

func TestVerificationPepper_IssuanceIgnoresPreviousKey(t *testing.T) {
	setVerificationRootKeys(t, "", rotationPreviousRoot, rotationLegacyRoot)
	pepper, err := verificationCodePepper()
	require.NoError(t, err)
	assert.Equal(t, pepperForRoot(t, rotationLegacyRoot), pepper,
		"new codes must not bind to the decryption-only PREVIOUS key")
}

func TestMatchActiveVerificationCode_CurrentPepperHit(t *testing.T) {
	setVerificationRootKeys(t, rotationCurrentRoot, rotationPreviousRoot, rotationLegacyRoot)
	repo := rotationRepo(t, digestFromRoot(t, organizationdomain.PasswordDeriveAlgorithmHMACSHA256, rotationCurrentRoot))

	before := rotationHitCount(t, "previous")
	matched, err := MatchActiveVerificationCode(context.Background(), repo, rotationRecipient, VerificationTypePhone, rotationCode)
	require.NoError(t, err)
	assert.Equal(t, "verification-rotation", matched.ID)
	assert.Equal(t, before, rotationHitCount(t, "previous"), "current-key hits must not be counted as rotation")
}

func TestMatchActiveVerificationCode_PreviousPepperHitIsCounted(t *testing.T) {
	setVerificationRootKeys(t, rotationCurrentRoot, rotationPreviousRoot, "")
	repo := rotationRepo(t, digestFromRoot(t, organizationdomain.PasswordDeriveAlgorithmHMACSHA256, rotationPreviousRoot))

	before := rotationHitCount(t, "previous")
	matched, err := MatchActiveVerificationCode(context.Background(), repo, rotationRecipient, VerificationTypePhone, rotationCode)
	require.NoError(t, err)
	assert.Equal(t, "verification-rotation", matched.ID)
	assert.Equal(t, before+1, rotationHitCount(t, "previous"))
}

func TestMatchActiveVerificationCode_LegacyPepperHitIsCounted(t *testing.T) {
	setVerificationRootKeys(t, rotationCurrentRoot, rotationPreviousRoot, rotationLegacyRoot)
	repo := rotationRepo(t, digestFromRoot(t, organizationdomain.PasswordDeriveAlgorithmHMACSM3, rotationLegacyRoot))

	before := rotationHitCount(t, "legacy")
	matched, err := MatchActiveVerificationCode(context.Background(), repo, rotationRecipient, VerificationTypePhone, rotationCode)
	require.NoError(t, err)
	assert.Equal(t, "verification-rotation", matched.ID)
	assert.Equal(t, before+1, rotationHitCount(t, "legacy"))
}

func TestMatchActiveVerificationCode_UnknownRootReturnsNotFound(t *testing.T) {
	setVerificationRootKeys(t, rotationCurrentRoot, rotationPreviousRoot, rotationLegacyRoot)
	repo := rotationRepo(t, digestFromRoot(t, organizationdomain.PasswordDeriveAlgorithmHMACSHA256, rotationRetiredRoot))

	_, err := MatchActiveVerificationCode(context.Background(), repo, rotationRecipient, VerificationTypePhone, rotationCode)
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

func TestMatchActiveVerificationCode_UnknownDigestAlgorithmIsRejected(t *testing.T) {
	setVerificationRootKeys(t, rotationCurrentRoot, "", "")
	repo := rotationRepo(t, "hmac_sha1$deadbeef")

	_, err := MatchActiveVerificationCode(context.Background(), repo, rotationRecipient, VerificationTypePhone, rotationCode)
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

func TestMatchActiveVerificationCode_EmptyPepperCandidatesReturnUnavailable(t *testing.T) {
	setVerificationRootKeys(t, "", "", "")
	digest, err := verificationCodeDigest(organizationdomain.PasswordDeriveAlgorithmHMACSHA256,
		[]byte("standalone-test-pepper"), testOrganizationID, VerificationTypePhone, rotationRecipient, rotationCode)
	require.NoError(t, err)

	_, err = MatchActiveVerificationCode(context.Background(), rotationRepo(t, digest),
		rotationRecipient, VerificationTypePhone, rotationCode)
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnavailable, apperrors.FromError(err).Code)
}

func TestVerifyCode_AcceptsDigestIssuedWithPreviousRoot(t *testing.T) {
	setVerificationRootKeys(t, rotationPreviousRoot, "", "")
	issuedDigest := digestFromRoot(t, organizationdomain.PasswordDeriveAlgorithmHMACSHA256, rotationPreviousRoot)

	setVerificationRootKeys(t, rotationCurrentRoot, rotationPreviousRoot, "")
	service, repo := newTestService(nil)
	service.pepper = nil
	repo.active = rotationRepo(t, issuedDigest).active

	before := rotationHitCount(t, "previous")
	require.NoError(t, service.VerifyCode(context.Background(), VerifyCodeCommand{
		Type:      VerificationTypePhone,
		Recipient: rotationRecipient,
		Code:      rotationCode,
	}))
	assert.Equal(t, "verified", repo.active.Status)
	assert.Equal(t, before+1, rotationHitCount(t, "previous"))
}

func TestVerifyCode_RejectsWrongCodeWhenOnlyPreviousRootMatches(t *testing.T) {
	setVerificationRootKeys(t, rotationPreviousRoot, "", "")
	issuedDigest := digestFromRoot(t, organizationdomain.PasswordDeriveAlgorithmHMACSHA256, rotationPreviousRoot)

	setVerificationRootKeys(t, rotationCurrentRoot, rotationPreviousRoot, "")
	service, repo := newTestService(nil)
	service.pepper = nil
	repo.active = rotationRepo(t, issuedDigest).active

	require.Error(t, service.VerifyCode(context.Background(), VerifyCodeCommand{
		Type:      VerificationTypePhone,
		Recipient: rotationRecipient,
		Code:      "000000",
	}))
	assert.Equal(t, "pending", repo.active.Status)
}
