package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

// loginFailureAuditUOW keeps the login-test repositories while exposing
// capturing audit/outbox repositories, so the failure path can be observed
// without a real database.
type loginFailureAuditUOW struct {
	loginTestUnitOfWork
	records domainrecords.Repository
	outbox  domainoutbox.Repository
}

func (u *loginFailureAuditUOW) Records() domainrecords.Repository { return u.records }
func (u *loginFailureAuditUOW) Outbox() domainoutbox.Repository   { return u.outbox }
func (u *loginFailureAuditUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

type failingAuditRecords struct {
	auditPlainRecords
	err error
}

func (r *failingAuditRecords) Create(context.Context, *domainrecords.Record) error {
	return r.err
}

func newLoginFailureAuditService(uow appuow.AuthUoW) Service {
	svc := newTestService(loginTestUnitOfWork{})
	svc.UOW = uow
	return svc
}

func failureAuditUOW() (*loginFailureAuditUOW, *auditPlainRecords, *auditPlainOutbox) {
	base := newTestUow(newTestOrg(), newTestUser(), newTestApp())
	records := &auditPlainRecords{}
	outbox := &auditPlainOutbox{}
	return &loginFailureAuditUOW{loginTestUnitOfWork: base, records: records, outbox: outbox}, records, outbox
}

func failureAuditRecords(records []*domainrecords.Record) []*domainrecords.Record {
	out := make([]*domainrecords.Record, 0, len(records))
	for _, record := range records {
		if record.Action == audit.EventAuthLoginFailed {
			out = append(out, record)
		}
	}
	return out
}

func failureAuditMetadata(t *testing.T, record *domainrecords.Record) map[string]any {
	t.Helper()
	require.NotNil(t, record.Metadata)
	return record.Metadata
}

func TestPasswordLogin_CredentialFailureWritesAudit(t *testing.T) {
	uow, records, _ := failureAuditUOW()
	svc := newLoginFailureAuditService(uow)

	_, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         "WrongPassword!",
		ApplicationID:    testAppID,
		IPAddress:        "203.0.113.7",
		UserAgent:        "go-test-agent",
	})
	require.Error(t, err)

	auditRecords := failureAuditRecords(records.created)
	require.Len(t, auditRecords, 1, "credential failure must write exactly one failure audit record")
	record := auditRecords[0]
	assert.Equal(t, testOrgID, record.OrganizationID)
	assert.Equal(t, testUserID, record.ActorID)
	assert.Equal(t, "203.0.113.7", record.IP)
	assert.Equal(t, "go-test-agent", record.UserAgent)

	metadata := failureAuditMetadata(t, record)
	assert.Equal(t, "invalid_credentials", metadata["reason"])
	assert.Equal(t, testUsername, metadata["username"])
	assert.Equal(t, false, metadata["anonymous"])
}

func TestPasswordLogin_UnknownUserAuditUsesAnonymousMarker(t *testing.T) {
	uow, records, _ := failureAuditUOW()
	svc := newLoginFailureAuditService(uow)

	_, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         "does-not-exist",
		Password:         "WrongPassword!",
		ApplicationID:    testAppID,
		IPAddress:        "203.0.113.9",
		UserAgent:        "go-test-agent",
	})
	require.Error(t, err)

	auditRecords := failureAuditRecords(records.created)
	require.Len(t, auditRecords, 1)
	record := auditRecords[0]
	assert.Equal(t, testOrgID, record.OrganizationID)
	assert.Equal(t, "", record.ActorID, "an unknown username has no local user UUID")
	assert.Equal(t, "203.0.113.9", record.IP)

	metadata := failureAuditMetadata(t, record)
	assert.Equal(t, "invalid_credentials", metadata["reason"])
	assert.Equal(t, "does-not-exist", metadata["username"])
	assert.Equal(t, true, metadata["anonymous"])
}

func TestPasswordLogin_SuccessWritesNoFailureAudit(t *testing.T) {
	uow, records, _ := failureAuditUOW()
	svc := newLoginFailureAuditService(uow)

	_, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	require.NoError(t, err)

	assert.Empty(t, failureAuditRecords(records.created), "successful login must not write a failure audit")
}

func TestPasswordLogin_LockedAndWrongPasswordResponsesMatch(t *testing.T) {
	lockedUOW, lockedRecords, _ := failureAuditUOW()
	lockedUntil := time.Date(2026, 5, 15, 10, 30, 0, 0, time.UTC)
	lockedUOW.lockoutAttempt.attempts = map[string]*lockoutattempts.Attempt{
		testUserID: {
			ID:          "lockout-locked",
			UserID:      testUserID,
			Attempts:    5,
			LockedUntil: &lockedUntil,
			CreatedAt:   time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC),
		},
	}
	lockedSvc := newLoginFailureAuditService(lockedUOW)
	_, lockErr := lockedSvc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
		IPAddress:        "203.0.113.10",
		UserAgent:        "go-test-agent",
	})
	require.Error(t, lockErr)

	uow, _, _ := failureAuditUOW()
	svc := newLoginFailureAuditService(uow)
	_, wrongErr := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         "WrongPassword!",
		ApplicationID:    testAppID,
		IPAddress:        "203.0.113.10",
		UserAgent:        "go-test-agent",
	})
	require.Error(t, wrongErr)

	lockTyped := lockErr.(*apperrors.Error)
	wrongTyped := wrongErr.(*apperrors.Error)
	assert.Equal(t, wrongTyped.Code, lockTyped.Code, "lockout must use the credential-failure error code")
	assert.Equal(t, wrongTyped.Message, lockTyped.Message, "lockout must use the generic credential message")
	assert.Equal(t, wrongTyped.MessageKey, lockTyped.MessageKey)
	assert.Equal(t, wrongTyped.Meta, lockTyped.Meta)
	assert.Equal(t, wrongTyped.HTTPStatus(), lockTyped.HTTPStatus())

	attempt, err := lockedUOW.lockoutAttempt.GetByUserID(context.Background(), testUserID)
	require.NoError(t, err)
	require.NotNil(t, attempt)
	assert.Equal(t, 5, attempt.Attempts, "a locked login must not increment the failure counter")

	auditRecords := failureAuditRecords(lockedRecords.created)
	require.Len(t, auditRecords, 1)
	metadata := failureAuditMetadata(t, auditRecords[0])
	assert.Equal(t, "account_locked", metadata["reason"])
	assert.NotZero(t, metadata["lockout_remaining_seconds"], "remaining lock time stays server-side")
	assert.NotContains(t, lockTyped.Message, "locked")
	assert.NotContains(t, lockTyped.Message, "try again")
}

func TestPasswordLogin_AuditFailureIsFailOpen(t *testing.T) {
	base := newTestUow(newTestOrg(), newTestUser(), newTestApp())
	uow := &loginFailureAuditUOW{
		loginTestUnitOfWork: base,
		records:             &failingAuditRecords{err: errors.New("audit store unavailable")},
		outbox:              &auditPlainOutbox{},
	}
	svc := newLoginFailureAuditService(uow)

	_, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         "WrongPassword!",
		ApplicationID:    testAppID,
	})
	require.Error(t, err)
	typed, ok := err.(*apperrors.Error)
	require.True(t, ok)
	assert.Equal(t, apperrors.CodeUnauthorized, typed.Code)
	assert.Equal(t, "invalid username or password", typed.Message)

	attempt, getErr := base.lockoutAttempt.GetByUserID(context.Background(), testUserID)
	require.NoError(t, getErr)
	require.NotNil(t, attempt, "audit failure must not roll back the independent lockout counter")
	assert.Equal(t, 1, attempt.Attempts)
}
