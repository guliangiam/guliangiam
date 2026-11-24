package keys

import (
	"context"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/keys"
	repoMock "github.com/guliangiam/guliangiam/internal/domain/keys/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"go.uber.org/mock/gomock"
)

type fakeClock struct{ fixedTime time.Time }

func (c fakeClock) Now() time.Time { return c.fixedTime }

type fakeUUIDGenerator struct{ fixedID string }

func (g fakeUUIDGenerator) NewString() (string, error) { return g.fixedID, nil }

func TestNewService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	svc := NewService(fakeClock{fixedTime: time.Now()}, fakeUUIDGenerator{fixedID: "test"}, mockUOW)
	if svc.Clock == nil {
		t.Error("Clock should not be nil")
	}
}

func TestCreateKey_Validation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "k-001"},
		UOW:   mockUOW,
	}

	t.Run("missing_orgId", func(t *testing.T) {
		err := svc.CreateKey(context.Background(), CreateKeyCommand{Name: "x", KeyType: "RSA"})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("missing_name", func(t *testing.T) {
		err := svc.CreateKey(context.Background(), CreateKeyCommand{OrganizationID: "org-1", KeyType: "RSA"})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("missing_keyType", func(t *testing.T) {
		err := svc.CreateKey(context.Background(), CreateKeyCommand{OrganizationID: "org-1", Name: "test-key"})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestService_ByIDRejectsEmptyOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// No repository/transaction expectation: every call must stop at the
	// tenant fail-fast before it can touch storage.
	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	svc := Service{UOW: mockUOW}

	assertInvalidArgument := func(t *testing.T, err error) {
		t.Helper()
		appErr, ok := err.(*apperrors.Error)
		if !ok {
			t.Fatalf("err = %v (%T), want *apperrors.Error", err, err)
		}
		if appErr.Code != apperrors.CodeInvalidArgument {
			t.Fatalf("code = %q, want %q", appErr.Code, apperrors.CodeInvalidArgument)
		}
	}

	_, err := svc.GetKey(context.Background(), "  ", "key-1")
	assertInvalidArgument(t, err)
	assertInvalidArgument(t, svc.PatchKey(context.Background(), "", PatchKeyCommand{ID: "key-1"}))
	assertInvalidArgument(t, svc.DeleteKey(context.Background(), "", "key-1"))
}

func TestCreateKey_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockKeysRepo := repoMock.NewMockRepository(ctrl)

	mockUOW.EXPECT().Keys().Return(mockKeysRepo).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	)
	mockKeysRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, k *domain.Key) error {
			if k.ID != "k-001" {
				t.Errorf("key.ID = %q, want %q", k.ID, "k-001")
			}
			if k.OrganizationID != "org-1" {
				t.Errorf("key.OrganizationID = %q, want %q", k.OrganizationID, "org-1")
			}
			if k.Algorithm != "RS256" {
				t.Errorf("key.Algorithm = %q, want %q (default)", k.Algorithm, "RS256")
			}
			return nil
		},
	)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "k-001"},
		UOW:   mockUOW,
	}

	err := svc.CreateKey(context.Background(), CreateKeyCommand{
		OrganizationID: "org-1",
		Name:           "test-key",
		KeyType:        "RSA",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListKeys_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockKeysRepo := repoMock.NewMockRepository(ctrl)

	now := time.Now()
	keys := []*domain.Key{
		{ID: "k-1", Name: "key-1", KeyType: "RSA", CreatedAt: now, UpdatedAt: now},
		{ID: "k-2", Name: "key-2", KeyType: "EC", CreatedAt: now, UpdatedAt: now},
	}

	mockUOW.EXPECT().Keys().Return(mockKeysRepo).AnyTimes()
	mockKeysRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(keys, int64(2), nil)

	svc := Service{UOW: mockUOW}

	items, total, err := svc.ListKeys(context.Background(), ListKeysQuery{
		Page:           1,
		PageSize:       50,
		OrganizationID: "org-001",
	})
	if err != nil {
		t.Fatalf("ListKeys() returned unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want %d", total, 2)
	}
	if len(items) != 2 {
		t.Errorf("len(items) = %d, want %d", len(items), 2)
	}
}

func TestGetKey_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockKeysRepo := repoMock.NewMockRepository(ctrl)

	now := time.Now()
	key := &domain.Key{ID: "k-1", Name: "key-1", KeyType: "RSA", CreatedAt: now, UpdatedAt: now}

	mockUOW.EXPECT().Keys().Return(mockKeysRepo).AnyTimes()
	mockKeysRepo.EXPECT().GetByID(gomock.Any(), "k-1", "org-1").Return(key, nil)

	svc := Service{UOW: mockUOW}

	detail, err := svc.GetKey(context.Background(), "org-1", "k-1")
	if err != nil {
		t.Fatalf("GetKey() returned unexpected error: %v", err)
	}
	if detail.ID != "k-1" {
		t.Errorf("detail.ID = %q, want %q", detail.ID, "k-1")
	}
}

func TestGetKey_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockKeysRepo := repoMock.NewMockRepository(ctrl)

	mockUOW.EXPECT().Keys().Return(mockKeysRepo).AnyTimes()
	mockKeysRepo.EXPECT().GetByID(gomock.Any(), "nonexistent", "org-1").Return(nil, apperrors.NewNotFound("key not found"))

	svc := Service{UOW: mockUOW}

	detail, err := svc.GetKey(context.Background(), "org-1", "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if detail != nil {
		t.Errorf("expected nil detail, got %+v", detail)
	}
}

// TestByIDKeyMethods_EmptyOrganizationRejected pins that tenant-scoped by-ID
// methods never reach a repository with an empty organization: on dialects
// without RLS an empty predicate degrades to "all tenants".
func TestByIDKeyMethods_EmptyOrganizationRejected(t *testing.T) {
	svc := Service{}
	ctx := context.Background()

	calls := map[string]func() error{
		"get": func() error {
			_, err := svc.GetKey(ctx, " ", "k-1")
			return err
		},
		"patch": func() error {
			return svc.PatchKey(ctx, "", PatchKeyCommand{ID: "k-1"})
		},
		"delete": func() error {
			return svc.DeleteKey(ctx, "\t", "k-1")
		},
		"disable": func() error {
			return svc.DisableKey(ctx, " ", "k-1", 1)
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("expected organizationId validation error, got nil")
			}
		})
	}
}
