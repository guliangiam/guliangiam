package records

import (
	"context"
	"testing"
	"time"

	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/records"
	repoMock "github.com/guliangiam/guliangiam/internal/domain/records/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"go.uber.org/mock/gomock"
)

// --- Fake platform implementations ---

type fakeClock struct{ fixedTime time.Time }

func (c fakeClock) Now() time.Time { return c.fixedTime }

type fakeUUIDGenerator struct{ fixedID string }

func (g fakeUUIDGenerator) NewString() (string, error) { return g.fixedID, nil }

// --- Test cases ---

func TestNewService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	svc := NewService(fakeClock{fixedTime: time.Now()}, fakeUUIDGenerator{fixedID: "test"}, mockUOW)
	if svc.Clock == nil {
		t.Error("Clock should not be nil")
	}
}

func TestCreateRecord_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fixedNow := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	fixedID := "rec-001"

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockRecordsRepo := repoMock.NewMockRepository(ctrl)

	// Set up expectations
	mockUOW.EXPECT().Records().Return(mockRecordsRepo).AnyTimes()
	mockRecordsRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, record *domain.Record) error {
			if record.ID != fixedID {
				t.Errorf("record.ID = %q, want %q", record.ID, fixedID)
			}
			if record.OrganizationID != "org-001" {
				t.Errorf("record.OrganizationID = %q, want %q", record.OrganizationID, "org-001")
			}
			if record.Action != "user.login" {
				t.Errorf("record.Action = %q, want %q", record.Action, "user.login")
			}
			return nil
		},
	)

	svc := Service{
		Clock: fakeClock{fixedTime: fixedNow},
		UUID:  fakeUUIDGenerator{fixedID: fixedID},
		UOW:   mockUOW,
	}

	dto, err := svc.CreateRecord(context.Background(), CreateRecordCommand{
		OrganizationID: "org-001",
		ActorID:        "user-001",
		Action:         "user.login",
		ResourceType:   "session",
		ResourceID:     "sess-001",
		IP:             "192.168.1.1",
		UserAgent:      "Mozilla/5.0",
	})
	if err != nil {
		t.Fatalf("CreateRecord() returned unexpected error: %v", err)
	}
	if dto == nil {
		t.Fatal("expected non-nil dto")
	}
	if dto.ID != fixedID {
		t.Errorf("dto.ID = %q, want %q", dto.ID, fixedID)
	}
	if dto.Action != "user.login" {
		t.Errorf("dto.Action = %q, want %q", dto.Action, "user.login")
	}
}

func TestCreateRecord_RepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fixedNow := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	fixedID := "rec-002"

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockRecordsRepo := repoMock.NewMockRepository(ctrl)

	mockUOW.EXPECT().Records().Return(mockRecordsRepo).AnyTimes()
	mockRecordsRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(
		apperrors.NewInternal("database error"),
	)

	svc := Service{
		Clock: fakeClock{fixedTime: fixedNow},
		UUID:  fakeUUIDGenerator{fixedID: fixedID},
		UOW:   mockUOW,
	}

	dto, err := svc.CreateRecord(context.Background(), CreateRecordCommand{
		OrganizationID: "org-001",
		Action:         "user.login",
		ResourceType:   "session",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if dto != nil {
		t.Errorf("expected nil dto, got %+v", dto)
	}
}

func TestListRecords_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockRecordsRepo := repoMock.NewMockRepository(ctrl)

	now := time.Now()
	records := []*domain.Record{
		{ID: "rec-a", Action: "user.login", CreatedAt: now},
		{ID: "rec-b", Action: "user.logout", CreatedAt: now},
	}

	mockUOW.EXPECT().Records().Return(mockRecordsRepo).AnyTimes()
	mockRecordsRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(records, int64(2), nil)

	svc := Service{UOW: mockUOW}

	dtos, total, err := svc.ListRecords(context.Background(), ListRecordsQuery{
		Page:           1,
		PageSize:       50,
		OrganizationID: "org-001",
	})
	if err != nil {
		t.Fatalf("ListRecords() returned unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want %d", total, 2)
	}
	if len(dtos) != 2 {
		t.Fatalf("len(dtos) = %d, want %d", len(dtos), 2)
	}
	if dtos[0].ID != "rec-a" {
		t.Errorf("dtos[0].ID = %q, want %q", dtos[0].ID, "rec-a")
	}
}

func TestListRecords_EmptyResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockRecordsRepo := repoMock.NewMockRepository(ctrl)

	mockUOW.EXPECT().Records().Return(mockRecordsRepo).AnyTimes()
	mockRecordsRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*domain.Record{}, int64(0), nil)

	svc := Service{UOW: mockUOW}

	dtos, total, err := svc.ListRecords(context.Background(), ListRecordsQuery{
		Page:           1,
		PageSize:       50,
		OrganizationID: "org-001",
	})
	if err != nil {
		t.Fatalf("ListRecords() returned unexpected error: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want %d", total, 0)
	}
	if len(dtos) != 0 {
		t.Errorf("len(dtos) = %d, want %d", len(dtos), 0)
	}
}
