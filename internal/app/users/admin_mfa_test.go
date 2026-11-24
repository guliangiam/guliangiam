package users

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// adminRebindRecordsRepo is a minimal audit repository: Search answers the
// cooldown probe (and records its criteria), Create captures the audit event
// written by EmbedAuditEvent, and the hash-chain methods are unused here.
type adminRebindRecordsRepo struct {
	searchTotal  int64
	searchErr    error
	searchCalls  []recordsdomain.SearchCriteria
	created      []*recordsdomain.Record
	createCalled bool
}

func (r *adminRebindRecordsRepo) Create(_ context.Context, record *recordsdomain.Record) error {
	r.createCalled = true
	r.created = append(r.created, record)
	return nil
}

func (r *adminRebindRecordsRepo) List(context.Context, recordsdomain.ListQuery) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *adminRebindRecordsRepo) Search(_ context.Context, criteria recordsdomain.SearchCriteria) ([]*recordsdomain.Record, int64, error) {
	r.searchCalls = append(r.searchCalls, criteria)
	return nil, r.searchTotal, r.searchErr
}

func (r *adminRebindRecordsRepo) LoadLastChainEntry(context.Context, string) (int64, []byte, error) {
	return 0, nil, nil
}

func (r *adminRebindRecordsRepo) LoadAnchorSeq(context.Context, string, string) (int64, error) {
	return 0, nil
}

func (r *adminRebindRecordsRepo) InsertAnchor(context.Context, *recordsdomain.ChainAnchor) error {
	return nil
}

func (r *adminRebindRecordsRepo) ListChainEntries(context.Context, string) ([]recordsdomain.ChainEntry, error) {
	return nil, nil
}

func rebindErrorCode(err error) apperrors.Code {
	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

func TestValidateAdminRebindTOTPCommand(t *testing.T) {
	tests := []struct {
		name    string
		command AdminRebindTOTPCommand
		want    apperrors.Code
	}{
		{
			name:    "tenant administrator denied",
			command: AdminRebindTOTPCommand{ActorPrincipalType: "user", Confirm: true, Reason: "compromised device"},
			want:    apperrors.CodeForbidden,
		},
		{
			name:    "non admin principal denied",
			command: AdminRebindTOTPCommand{ActorPrincipalType: "service_account", Confirm: true, Reason: "compromised device"},
			want:    apperrors.CodeForbidden,
		},
		{
			name:    "confirmation missing",
			command: AdminRebindTOTPCommand{ActorPrincipalType: "platform_admin", Confirm: false, Reason: "compromised device"},
			want:    apperrors.CodeUnprocessableEntity,
		},
		{
			name:    "reason blank",
			command: AdminRebindTOTPCommand{ActorPrincipalType: "platform_admin", Confirm: true, Reason: "   "},
			want:    apperrors.CodeInvalidArgument,
		},
		{
			name:    "reason empty",
			command: AdminRebindTOTPCommand{ActorPrincipalType: "platform_admin", Confirm: true, Reason: ""},
			want:    apperrors.CodeInvalidArgument,
		},
		{
			name:    "reason too long",
			command: AdminRebindTOTPCommand{ActorPrincipalType: "platform_admin", Confirm: true, Reason: strings.Repeat("a", 501)},
			want:    apperrors.CodeUnprocessableEntity,
		},
		{
			name:    "platform administrator accepted",
			command: AdminRebindTOTPCommand{ActorPrincipalType: "platform_admin", Confirm: true, Reason: "compromised device"},
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAdminRebindTOTPCommand(tt.command)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("validateAdminRebindTOTPCommand() error = %v, want nil", err)
				}
				return
			}
			if got := rebindErrorCode(err); got != tt.want {
				t.Errorf("error code = %q, want %q (err = %v)", got, tt.want, err)
			}
		})
	}
}

func TestAdminRebindCoolingDown(t *testing.T) {
	since := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

	if err := adminRebindCoolingDown(0, since); err != nil {
		t.Errorf("adminRebindCoolingDown(0) error = %v, want nil", err)
	}
	err := adminRebindCoolingDown(1, since)
	if got := rebindErrorCode(err); got != apperrors.CodeTooManyRequests {
		t.Errorf("error code = %q, want %q", got, apperrors.CodeTooManyRequests)
	}
}

func TestService_AdminRebindTOTP_Success(t *testing.T) {
	now := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	target := &userdomain.User{
		ID:                 "target-user-1",
		OrganizationID:     "org-1",
		Username:           "alice",
		TotpSecret:         "JBSWY3DPEHPK3PXP",
		RecoveryCodeHashes: []string{"recovery-1", "recovery-2"},
		MfaMethods: []userdomain.UserMfaMethod{
			{ID: "mfa-totp", UserID: "target-user-1", MethodType: "totp", Enabled: true, Verified: true},
			{ID: "mfa-email", UserID: "target-user-1", MethodType: "email", Enabled: true, Verified: true},
		},
		Revision: 4,
	}

	var updated *userdomain.User
	var replacedMethods []userdomain.UserMfaMethod
	userRepo := &svcMockUserRepo{
		getByIDFn: func(context.Context, string) (*userdomain.User, error) {
			return target, nil
		},
		updateFn: func(_ context.Context, user *userdomain.User) error {
			updated = user
			return nil
		},
		replaceMfaMethodsFn: func(_ context.Context, userID string, methods []userdomain.UserMfaMethod) error {
			if userID != "target-user-1" {
				t.Errorf("ReplaceMfaMethods userID = %q, want %q", userID, "target-user-1")
			}
			replacedMethods = methods
			return nil
		},
	}

	recordsRepo := &adminRebindRecordsRepo{searchTotal: 0}
	svc := Service{
		Clock: svcFakeClock{fixedTime: now},
		UOW:   svcTestUnitOfWork{users: userRepo, records: recordsRepo},
	}

	err := svc.AdminRebindTOTP(context.Background(), AdminRebindTOTPCommand{
		ActorID:            "platform-admin-1",
		ActorPrincipalType: "platform_admin",
		TargetUserID:       "target-user-1",
		Reason:             "device compromised",
		Confirm:            true,
	})
	if err != nil {
		t.Fatalf("AdminRebindTOTP() error = %v", err)
	}

	if updated == nil {
		t.Fatal("Update() was not called")
	}
	if updated.TotpSecret != "" {
		t.Errorf("TotpSecret = %q, want empty", updated.TotpSecret)
	}
	if updated.RecoveryCodeHashes != nil {
		t.Errorf("RecoveryCodes = %v, want nil", updated.RecoveryCodeHashes)
	}
	if updated.Revision != 5 {
		t.Errorf("Revision = %d, want 5", updated.Revision)
	}
	if len(updated.MfaMethods) != 1 || updated.MfaMethods[0].MethodType != "email" {
		t.Errorf("MfaMethods = %+v, want the email method only", updated.MfaMethods)
	}
	if len(replacedMethods) != 1 || replacedMethods[0].MethodType != "email" {
		t.Errorf("ReplaceMfaMethods = %+v, want the email method only", replacedMethods)
	}

	if len(recordsRepo.searchCalls) != 1 {
		t.Fatalf("Search() calls = %d, want 1", len(recordsRepo.searchCalls))
	}
	criteria := recordsRepo.searchCalls[0]
	if criteria.Action != "mfa.totp.admin_rebind" ||
		criteria.ResourceType != "User" ||
		criteria.ResourceID != "target-user-1" ||
		criteria.OrganizationID != "org-1" {
		t.Errorf("cooldown criteria = %+v, want the target user's rebind records", criteria)
	}
	if criteria.StartTime == nil || !criteria.StartTime.Equal(now.Add(-24*time.Hour)) {
		t.Errorf("StartTime = %v, want %v", criteria.StartTime, now.Add(-24*time.Hour))
	}

	if !recordsRepo.createCalled {
		t.Fatal("audit event was not written")
	}
	if len(recordsRepo.created) != 1 {
		t.Fatalf("audit records = %d, want 1", len(recordsRepo.created))
	}
	auditRecord := recordsRepo.created[0]
	if auditRecord.Action != "mfa.totp.admin_rebind" ||
		auditRecord.ResourceType != "User" ||
		auditRecord.ResourceID != "target-user-1" ||
		auditRecord.OrganizationID != "org-1" {
		t.Errorf("audit record = %+v, want the rebind event for the target user", auditRecord)
	}
}

func TestService_AdminRebindTOTP_CoolingDown(t *testing.T) {
	now := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	target := &userdomain.User{ID: "target-user-1", OrganizationID: "org-1", Username: "alice"}

	userRepo := &svcMockUserRepo{
		getByIDFn: func(context.Context, string) (*userdomain.User, error) { return target, nil },
		updateFn: func(context.Context, *userdomain.User) error {
			t.Error("Update() must not be called while cooling down")
			return nil
		},
	}
	recordsRepo := &adminRebindRecordsRepo{searchTotal: 1}

	svc := Service{
		Clock: svcFakeClock{fixedTime: now},
		UOW:   svcTestUnitOfWork{users: userRepo, records: recordsRepo},
	}

	err := svc.AdminRebindTOTP(context.Background(), AdminRebindTOTPCommand{
		ActorID:            "platform-admin-1",
		ActorPrincipalType: "platform_admin",
		TargetUserID:       "target-user-1",
		Reason:             "device compromised",
		Confirm:            true,
	})
	if got := rebindErrorCode(err); got != apperrors.CodeTooManyRequests {
		t.Errorf("error code = %q, want %q (err = %v)", got, apperrors.CodeTooManyRequests, err)
	}
	if recordsRepo.createCalled {
		t.Error("audit event must not be written when cooling down")
	}
}
