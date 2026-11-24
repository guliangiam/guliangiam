package verifications

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	organizationdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	verificationsdomain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testOrganizationID = "00000000-0000-0000-0000-000000000001"

type fakeClock struct{}

func (fakeClock) Now() time.Time { return time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC) }

type fakeUUID struct{ id string }

func (f fakeUUID) NewString() (string, error) { return f.id, nil }

type fakeVerificationRepo struct {
	verificationsdomain.Repository
	created []*verificationsdomain.Verification
	active  *verificationsdomain.Verification
}

func (f *fakeVerificationRepo) Create(_ context.Context, verification *verificationsdomain.Verification) error {
	f.created = append(f.created, verification)
	return nil
}

func (f *fakeVerificationRepo) GetActiveByRecipientAndCode(_ context.Context, recipient, code, verificationType string) (*verificationsdomain.Verification, error) {
	if f.active != nil && f.active.Recipient == recipient && f.active.Type == verificationType && f.active.Code == code {
		return f.active, nil
	}
	return nil, apperrors.NewNotFound("verification not found")
}

func (f *fakeVerificationRepo) GetActiveByRecipient(_ context.Context, recipient, verificationType string) (*verificationsdomain.Verification, error) {
	// Mirrors the SQL `status = 'pending'` guard; empty status stands for a
	// freshly issued row in fixtures that omit the field.
	if f.active != nil && (f.active.Status == "" || f.active.Status == "pending") &&
		f.active.Recipient == recipient && f.active.Type == verificationType {
		return f.active, nil
	}
	return nil, apperrors.NewNotFound("verification not found")
}

func (f *fakeVerificationRepo) UpdateCode(_ context.Context, _ string, code string) error {
	if f.active != nil {
		f.active.Code = code
	}
	return nil
}

func (f *fakeVerificationRepo) UpdateStatus(_ context.Context, _ string, status string) error {
	if f.active != nil {
		f.active.Status = status
	}
	return nil
}

type fakeOrganizationRepo struct {
	organizationdomain.Repository
	org *organizationdomain.Organization
}

func (f *fakeOrganizationRepo) GetByID(context.Context, string) (*organizationdomain.Organization, error) {
	if f.org == nil {
		return nil, apperrors.NewNotFound("organization not found")
	}
	return f.org, nil
}

func (f *fakeOrganizationRepo) GetBySlug(context.Context, string) (*organizationdomain.Organization, error) {
	if f.org == nil {
		return nil, apperrors.NewNotFound("organization not found")
	}
	return f.org, nil
}

type fakeAuthUoW struct {
	appuow.AuthUoW
	repo *fakeVerificationRepo
	orgs organizationdomain.Repository
}

func (f fakeAuthUoW) Verifications() verificationsdomain.Repository { return f.repo }
func (f fakeAuthUoW) Organizations() organizationdomain.Repository  { return f.orgs }

type captureEmailSender struct {
	calls   int
	to      string
	subject string
	body    string
	err     error
}

func (s *captureEmailSender) Send(_ context.Context, to, subject, body string) error {
	s.calls++
	s.to, s.subject, s.body = to, subject, body
	return s.err
}

type captureSmsSender struct {
	calls int
	phone string
	code  string
	err   error
}

func (s *captureSmsSender) Send(_ context.Context, phone, code string) error {
	s.calls++
	s.phone, s.code = phone, code
	return s.err
}

// verifierSmsSender mimics a provider that generates and checks the code
// itself (Twilio Verify): Send ignores the local code and CheckCode decides.
type verifierSmsSender struct {
	calls    int
	checks   int
	approved bool
}

func (s *verifierSmsSender) Send(context.Context, string, string) error {
	s.calls++
	return nil
}

func (s *verifierSmsSender) CheckCode(context.Context, string, string) (bool, error) {
	s.checks++
	return s.approved, nil
}

type fakeSenderResolver struct {
	email      verificationsdomain.EmailSender
	sms        verificationsdomain.SmsSender
	err        error
	emailCalls int
	smsCalls   int
}

func (f *fakeSenderResolver) ResolveEmailSender(context.Context, string) (verificationsdomain.EmailSender, error) {
	f.emailCalls++
	return f.email, f.err
}

func (f *fakeSenderResolver) ResolveSmsSender(context.Context, string) (verificationsdomain.SmsSender, error) {
	f.smsCalls++
	return f.sms, f.err
}

func newTestService(resolver verificationsdomain.SenderResolver) (Service, *fakeVerificationRepo) {
	return newTestServiceWithAlgorithm(resolver, organizationdomain.PasswordDeriveAlgorithmHMACSHA256)
}

func newTestServiceWithAlgorithm(resolver verificationsdomain.SenderResolver, algorithm string) (Service, *fakeVerificationRepo) {
	repo := &fakeVerificationRepo{}
	orgs := &fakeOrganizationRepo{org: &organizationdomain.Organization{
		ID: testOrganizationID,
		Security: organizationdomain.OrganizationSecurityPolicy{
			PasswordDeriveAlgorithm: algorithm,
		},
	}}
	return Service{
		Clock:          fakeClock{},
		UUID:           fakeUUID{id: "verification-1"},
		UOW:            fakeAuthUoW{repo: repo, orgs: orgs},
		SenderResolver: resolver,
		pepper:         func() ([]byte, error) { return []byte("test-verification-pepper"), nil },
	}, repo
}

func TestSendVerification_EmailUsesResolvedOrganizationSender(t *testing.T) {
	sender := &captureEmailSender{}
	resolver := &fakeSenderResolver{email: sender}
	service, repo := newTestService(resolver)

	result, err := service.SendVerification(context.Background(), SendVerificationCommand{
		OrganizationID: testOrganizationID,
		Type:           VerificationTypeEmail,
		Recipient:      "user@example.com",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "verification-1", result.ID)
	assert.Equal(t, 1, resolver.emailCalls)
	assert.Zero(t, resolver.smsCalls)
	require.Len(t, repo.created, 1)
	assert.Equal(t, testOrganizationID, repo.created[0].OrganizationID)
	assert.Equal(t, "user@example.com", repo.created[0].Recipient)
	assert.True(t, strings.HasPrefix(repo.created[0].Code, "hmac_sha256$"),
		"stored code must be a digest: %q", repo.created[0].Code)
	assert.NotEqual(t, emailCode(t, sender.body), repo.created[0].Code)

	assert.Equal(t, 1, sender.calls)
	assert.Equal(t, "user@example.com", sender.to)
	assert.Contains(t, sender.body, "Your verification code is: ")
}

func TestSendVerification_PhoneUsesResolvedSmsSender(t *testing.T) {
	sender := &captureSmsSender{}
	resolver := &fakeSenderResolver{sms: sender}
	service, repo := newTestService(resolver)

	result, err := service.SendVerification(context.Background(), SendVerificationCommand{
		OrganizationID: testOrganizationID,
		Type:           VerificationTypePhone,
		Recipient:      "+8613800000000",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, resolver.smsCalls)
	assert.Zero(t, resolver.emailCalls)
	require.Len(t, repo.created, 1)

	assert.Equal(t, 1, sender.calls)
	assert.Equal(t, "+8613800000000", sender.phone)
	assert.NotEqual(t, repo.created[0].Code, sender.code)
	assert.True(t, strings.HasPrefix(repo.created[0].Code, "hmac_sha256$"))
}

func TestSendVerification_UnavailableProviderStoresNothing(t *testing.T) {
	resolver := &fakeSenderResolver{err: apperrors.NewUnavailable("no email provider")}
	service, repo := newTestService(resolver)

	result, err := service.SendVerification(context.Background(), SendVerificationCommand{
		OrganizationID: testOrganizationID,
		Type:           VerificationTypeEmail,
		Recipient:      "user@example.com",
	})
	require.Error(t, err)
	assert.Nil(t, result)
	appErr, ok := err.(*apperrors.Error)
	require.True(t, ok)
	assert.Equal(t, 503, appErr.HTTPStatus())
	assert.Empty(t, repo.created, "no pending code should be stored when delivery is unavailable")
}

func TestSendVerification_WithoutResolverKeepsLegacyBehaviour(t *testing.T) {
	service, repo := newTestService(nil)

	result, err := service.SendVerification(context.Background(), SendVerificationCommand{
		OrganizationID: testOrganizationID,
		Type:           VerificationTypeEmail,
		Recipient:      "user@example.com",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, repo.created, 1)
}

func TestSendVerification_ProviderManagedSmsStoresEmptyCode(t *testing.T) {
	sender := &verifierSmsSender{approved: true}
	service, repo := newTestService(&fakeSenderResolver{sms: sender})

	result, err := service.SendVerification(context.Background(), SendVerificationCommand{
		OrganizationID: testOrganizationID,
		Type:           VerificationTypePhone,
		Recipient:      "+8613800000000",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, repo.created, 1)
	assert.Empty(t, repo.created[0].Code, "provider-managed senders keep the stored code empty")
	assert.Equal(t, 1, sender.calls)
}

func TestVerifyCode_DelegatesToProviderVerifier(t *testing.T) {
	sender := &verifierSmsSender{approved: true}
	service, repo := newTestService(&fakeSenderResolver{sms: sender})
	repo.active = &verificationsdomain.Verification{
		ID:             "verification-1",
		OrganizationID: testOrganizationID,
		Type:           VerificationTypePhone,
		Recipient:      "+8613800000000",
		Code:           "",
		Status:         "pending",
		ExpiresAt:      fakeClock{}.Now().Add(time.Hour),
	}

	err := service.VerifyCode(context.Background(), VerifyCodeCommand{
		Type:      VerificationTypePhone,
		Recipient: "+8613800000000",
		Code:      "123456",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, sender.checks)
	assert.Equal(t, "verified", repo.active.Status)
	assert.True(t, strings.HasPrefix(repo.active.Code, "hmac_sha256$"),
		"approved code must be persisted as a digest: %q", repo.active.Code)
}

func TestVerifyCode_ProviderRejectsCode(t *testing.T) {
	sender := &verifierSmsSender{approved: false}
	service, repo := newTestService(&fakeSenderResolver{sms: sender})
	repo.active = &verificationsdomain.Verification{
		ID:             "verification-1",
		OrganizationID: testOrganizationID,
		Type:           VerificationTypePhone,
		Recipient:      "+8613800000000",
		Code:           "",
		Status:         "pending",
		ExpiresAt:      fakeClock{}.Now().Add(time.Hour),
	}

	err := service.VerifyCode(context.Background(), VerifyCodeCommand{
		Type:      VerificationTypePhone,
		Recipient: "+8613800000000",
		Code:      "000000",
	})
	require.Error(t, err)
	assert.Equal(t, 1, sender.checks)
	assert.Equal(t, "pending", repo.active.Status)
}

func TestSendVerification_DeliveryFailureStoresNothing(t *testing.T) {
	sender := &captureEmailSender{err: errors.New("smtp unavailable")}
	service, repo := newTestService(&fakeSenderResolver{email: sender})

	_, err := service.SendVerification(context.Background(), SendVerificationCommand{
		OrganizationID: testOrganizationID,
		Type:           VerificationTypeEmail,
		Recipient:      "user@example.com",
	})
	require.Error(t, err)
	appErr, ok := err.(*apperrors.Error)
	require.True(t, ok)
	assert.Equal(t, apperrors.CodeUnavailable, appErr.Code)
	assert.Equal(t, 503, appErr.HTTPStatus())
	assert.Empty(t, repo.created, "a failed delivery must not leave a pending code")
}

func TestSendAndVerifyCode_DigestAlgorithms(t *testing.T) {
	for _, algorithm := range []string{
		organizationdomain.PasswordDeriveAlgorithmHMACSHA256,
		organizationdomain.PasswordDeriveAlgorithmHMACSM3,
	} {
		t.Run(algorithm, func(t *testing.T) {
			sender := &captureSmsSender{}
			service, repo := newTestServiceWithAlgorithm(&fakeSenderResolver{sms: sender}, algorithm)

			_, err := service.SendVerification(context.Background(), SendVerificationCommand{
				OrganizationID: testOrganizationID,
				Type:           VerificationTypePhone,
				Recipient:      "+8613800000000",
			})
			require.NoError(t, err)
			require.Len(t, repo.created, 1)
			assert.True(t, strings.HasPrefix(repo.created[0].Code, algorithm+"$"),
				"stored code = %q, want %s$ digest", repo.created[0].Code, algorithm)
			assert.NotEqual(t, sender.code, repo.created[0].Code)

			repo.active = repo.created[0]
			require.NoError(t, service.VerifyCode(context.Background(), VerifyCodeCommand{
				Type:      VerificationTypePhone,
				Recipient: "+8613800000000",
				Code:      sender.code,
			}))
			assert.Equal(t, "verified", repo.active.Status)
		})
	}
}

func TestVerifyCode_RejectsWrongDigestCode(t *testing.T) {
	sender := &captureSmsSender{}
	service, repo := newTestService(&fakeSenderResolver{sms: sender})

	_, err := service.SendVerification(context.Background(), SendVerificationCommand{
		OrganizationID: testOrganizationID,
		Type:           VerificationTypePhone,
		Recipient:      "+8613800000000",
	})
	require.NoError(t, err)
	repo.active = repo.created[0]

	wrong := "000000"
	if sender.code == wrong {
		wrong = "111111"
	}
	require.Error(t, service.VerifyCode(context.Background(), VerifyCodeCommand{
		Type:      VerificationTypePhone,
		Recipient: "+8613800000000",
		Code:      wrong,
	}))
	assert.Equal(t, "pending", repo.active.Status)
}

func TestVerifyCode_AcceptsLegacyPlaintextRow(t *testing.T) {
	service, repo := newTestService(nil)
	repo.active = &verificationsdomain.Verification{
		ID:             "verification-legacy",
		OrganizationID: testOrganizationID,
		Type:           VerificationTypeEmail,
		Recipient:      "legacy@example.com",
		Code:           "123456",
		Status:         "pending",
		ExpiresAt:      fakeClock{}.Now().Add(time.Minute),
	}

	require.NoError(t, service.VerifyCode(context.Background(), VerifyCodeCommand{
		Type:      VerificationTypeEmail,
		Recipient: "legacy@example.com",
		Code:      "123456",
	}))
	assert.Equal(t, "verified", repo.active.Status)
}

func TestMatchActiveVerificationCode_DigestAndLegacy(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", "test-root-key")
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
	t.Setenv("ENCRYPTION_KEY", "")
	pepper, err := verificationCodePepper()
	require.NoError(t, err)

	digest, err := verificationCodeDigest(
		organizationdomain.PasswordDeriveAlgorithmHMACSM3, pepper,
		testOrganizationID, VerificationTypePhone, "+8613800000000", "654321",
	)
	require.NoError(t, err)

	_, repo := newTestService(nil)
	repo.active = &verificationsdomain.Verification{
		ID:             "verification-digest",
		OrganizationID: testOrganizationID,
		Type:           VerificationTypePhone,
		Recipient:      "+8613800000000",
		Code:           digest,
		Status:         "pending",
		ExpiresAt:      fakeClock{}.Now().Add(time.Minute),
	}
	matched, err := MatchActiveVerificationCode(context.Background(), repo, "+8613800000000", VerificationTypePhone, "654321")
	require.NoError(t, err)
	assert.Equal(t, "verification-digest", matched.ID)

	repo.active.Code = "123456"
	matched, err = MatchActiveVerificationCode(context.Background(), repo, "+8613800000000", VerificationTypePhone, "123456")
	require.NoError(t, err)
	assert.Equal(t, "verification-digest", matched.ID)
}

func emailCode(t *testing.T, body string) string {
	t.Helper()
	const marker = "Your verification code is: "
	index := strings.Index(body, marker)
	require.GreaterOrEqual(t, index, 0, "email body is missing the code marker")
	return strings.TrimSpace(body[index+len(marker):])
}
