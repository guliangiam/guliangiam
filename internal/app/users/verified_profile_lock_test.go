package users

import (
	"context"
	"testing"
	"time"

	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// 规范 9.3/9.5：已验证用户的实名字段后端锁定；证件号原文永不回传，PUT 空值
// 必须保持存储值而不是清除。

func verifiedLockTestUser() *userdomain.User {
	return &userdomain.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		IsVerified:     true,
		Profile: userdomain.UserProfile{
			UserID:     "user-1",
			RealName:   "Zhang San",
			IDCardType: "id_card",
			IDCard:     "110101199001011234",
			PassportNo: "E12345678",
			Properties: map[string]any{
				"idCardFront": "https://cdn.example.com/front.png",
			},
		},
	}
}

func newVerifiedLockPutHarness(user *userdomain.User) (*Service, **userdomain.UserProfile) {
	var savedProfile *userdomain.UserProfile
	userRepo := &svcMockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			return user, nil
		},
		updateProfileFn: func(ctx context.Context, profile *userdomain.UserProfile) error {
			savedProfile = profile
			return nil
		},
	}
	svc := &Service{
		Clock: svcFakeClock{fixedTime: time.Now()},
		UOW:   svcTestUnitOfWork{users: userRepo},
	}
	return svc, &savedProfile
}

// PUT 空证件号 = 保持存储值（客户端拿不到原文，无法回传）。
func TestUpdateUserProfile_BlankDocumentNumbersPreserveStored(t *testing.T) {
	svc, saved := newVerifiedLockPutHarness(verifiedLockTestUser())

	err := svc.UpdateUserProfile(context.Background(), UpdateUserProfileCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		RealName:       "Zhang San",
		IDCardType:     "id_card",
		Properties:     map[string]any{"idCardFront": "https://cdn.example.com/front.png"},
	})
	if err != nil {
		t.Fatalf("UpdateUserProfile() error = %v", err)
	}
	if *saved == nil {
		t.Fatal("UpdateProfile was not called")
	}
	if (*saved).IDCard != "110101199001011234" || (*saved).PassportNo != "E12345678" {
		t.Fatalf("blank document numbers must preserve stored values: %+v", *saved)
	}
}

// 已验证用户改 realName：后端必须拒绝（前端 disabled 不是边界）。
func TestUpdateUserProfile_VerifiedLockRejectsRealNameChange(t *testing.T) {
	svc, saved := newVerifiedLockPutHarness(verifiedLockTestUser())

	err := svc.UpdateUserProfile(context.Background(), UpdateUserProfileCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		RealName:       "Li Si",
		IDCardType:     "id_card",
		Properties:     map[string]any{"idCardFront": "https://cdn.example.com/front.png"},
	})
	if selfProfileErrorCode(err) != apperrors.CodeInvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
	if *saved != nil {
		t.Fatal("locked update must not write")
	}
}

// 已验证用户显式改证件号（非空新值）同样拒绝。
func TestUpdateUserProfile_VerifiedLockRejectsIDCardChange(t *testing.T) {
	svc, saved := newVerifiedLockPutHarness(verifiedLockTestUser())

	err := svc.UpdateUserProfile(context.Background(), UpdateUserProfileCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		RealName:       "Zhang San",
		IDCardType:     "id_card",
		IDCard:         "110101199001015678",
		Properties:     map[string]any{"idCardFront": "https://cdn.example.com/front.png"},
	})
	if selfProfileErrorCode(err) != apperrors.CodeInvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
	if *saved != nil {
		t.Fatal("locked update must not write")
	}
}

// 已验证用户删除 idCardFront 影像（properties 缺键 = 删除）也属修改锁定字段。
func TestUpdateUserProfile_VerifiedLockRejectsImageRemoval(t *testing.T) {
	svc, saved := newVerifiedLockPutHarness(verifiedLockTestUser())

	err := svc.UpdateUserProfile(context.Background(), UpdateUserProfileCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		RealName:       "Zhang San",
		IDCardType:     "id_card",
		Properties:     map[string]any{},
	})
	if selfProfileErrorCode(err) != apperrors.CodeInvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
	if *saved != nil {
		t.Fatal("locked update must not write")
	}
}

// 未验证用户不受锁影响：realName 可改。
func TestUpdateUserProfile_UnverifiedUserNotLocked(t *testing.T) {
	user := verifiedLockTestUser()
	user.IsVerified = false
	svc, saved := newVerifiedLockPutHarness(user)

	err := svc.UpdateUserProfile(context.Background(), UpdateUserProfileCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		RealName:       "Li Si",
		IDCardType:     "id_card",
	})
	if err != nil {
		t.Fatalf("UpdateUserProfile() error = %v", err)
	}
	if *saved == nil || (*saved).RealName != "Li Si" {
		t.Fatalf("unverified realName change must persist: %+v", *saved)
	}
}

// PATCH 自助面：已验证用户显式改 idCard 拒绝；nil（不改）放行。
func TestUpdateSelfProfile_VerifiedLock(t *testing.T) {
	h := newSelfProfileHarness(verifiedLockTestUser(), nil)

	err := h.svc.UpdateSelfProfile(context.Background(), UpdateSelfProfileCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		IDCard:         strPtr("110101199001015678"),
	})
	if selfProfileErrorCode(err) != apperrors.CodeInvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
	if h.savedProfile != nil {
		t.Fatal("locked update must not write")
	}

	h2 := newSelfProfileHarness(verifiedLockTestUser(), nil)
	err = h2.svc.UpdateSelfProfile(context.Background(), UpdateSelfProfileCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		DisplayName:    strPtr("New Display"),
	})
	if err != nil {
		t.Fatalf("untouched locked fields must not block the update: %v", err)
	}
	if h2.savedProfile == nil || h2.savedProfile.IDCard != "110101199001011234" {
		t.Fatalf("locked field must survive untouched: %+v", h2.savedProfile)
	}
}

// PATCH 自助面：已验证用户用空串删除 idCardFront 影像同样拒绝。
func TestUpdateSelfProfile_VerifiedLockRejectsImageDeletion(t *testing.T) {
	h := newSelfProfileHarness(verifiedLockTestUser(), nil)

	err := h.svc.UpdateSelfProfile(context.Background(), UpdateSelfProfileCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		IDCardFront:    strPtr(""),
	})
	if selfProfileErrorCode(err) != apperrors.CodeInvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
	if h.savedProfile != nil {
		t.Fatal("locked update must not write")
	}
}
