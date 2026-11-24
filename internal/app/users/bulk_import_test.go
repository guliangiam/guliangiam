package users

import (
	"context"
	"testing"
	"time"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// defaultOrgRepo 返回一个 mockOrganizationRepo，让 slug 查询返回同名 org
func defaultOrgRepo() *mockOrgRepo {
	return &mockOrgRepo{
		getBySlugFn: func(_ context.Context, slug string) (*orgdomain.Organization, error) {
			return &orgdomain.Organization{ID: slug, Slug: slug}, nil
		},
	}
}

func TestBulkImportUsers_AllSuccess(t *testing.T) {
	var created []*userdomain.User
	userRepo := &mockUserRepo{
		createFn: func(_ context.Context, user *userdomain.User) error {
			created = append(created, user)
			return nil
		},
		updateProfileFn: func(_ context.Context, _ *userdomain.UserProfile) error { return nil },
	}
	uow := testUnitOfWork{users: userRepo, organizations: defaultOrgRepo()}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "bulk-id"},
		UOW:   uow,
	}

	users := []ImportUser{
		{Username: "u1", DisplayName: "User 1", Email: "u1@test.com", OrganizationID: "org-1"},
		{Username: "u2", DisplayName: "User 2", Email: "u2@test.com", OrganizationID: "org-1"},
	}
	result, err := svc.BulkImportUsers(context.Background(), "org-default", users)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("Total = %d, want 2", result.Total)
	}
	if result.Succeeded != 2 {
		t.Errorf("Succeeded = %d, want 2", result.Succeeded)
	}
	if result.Failed != 0 {
		t.Errorf("Failed = %d, want 0", result.Failed)
	}
	if len(created) != 2 {
		t.Errorf("created count = %d, want 2", len(created))
	}
}

func TestBulkImportUsers_PartialFailure(t *testing.T) {
	callCount := 0
	userRepo := &mockUserRepo{
		createFn: func(_ context.Context, user *userdomain.User) error {
			callCount++
			if callCount == 2 {
				return apperrors.NewConflict("duplicate")
			}
			return nil
		},
		updateProfileFn: func(_ context.Context, _ *userdomain.UserProfile) error { return nil },
	}
	uow := testUnitOfWork{users: userRepo, organizations: defaultOrgRepo()}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "bulk-id"},
		UOW:   uow,
	}

	users := []ImportUser{
		{Username: "u1", DisplayName: "User 1", OrganizationID: "org-1"},
		{Username: "u2", DisplayName: "User 2", OrganizationID: "org-1"},
		{Username: "u3", DisplayName: "User 3", OrganizationID: "org-1"},
	}
	result, err := svc.BulkImportUsers(context.Background(), "org-1", users)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 3 {
		t.Errorf("Total = %d, want 3", result.Total)
	}
	if result.Succeeded != 2 {
		t.Errorf("Succeeded = %d, want 2", result.Succeeded)
	}
	if result.Failed != 1 {
		t.Errorf("Failed = %d, want 1", result.Failed)
	}
	if len(result.Errors) != 1 {
		t.Errorf("Errors count = %d, want 1", len(result.Errors))
	}
}

func TestBulkImportUsers_RowOrganizationIDIgnored(t *testing.T) {
	var created []*userdomain.User
	userRepo := &mockUserRepo{
		createFn: func(_ context.Context, user *userdomain.User) error {
			created = append(created, user)
			return nil
		},
		updateProfileFn: func(_ context.Context, _ *userdomain.UserProfile) error { return nil },
	}
	uow := testUnitOfWork{users: userRepo, organizations: defaultOrgRepo()}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "bulk-id"},
		UOW:   uow,
	}

	users := []ImportUser{
		{Username: "u1", DisplayName: "User 1"},
		{Username: "u2", DisplayName: "User 2", OrganizationID: "org-explicit"},
	}
	_, err := svc.BulkImportUsers(context.Background(), "org-fallback", users)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created[0].OrganizationID != "org-fallback" {
		t.Errorf("user 0 OrganizationID = %q, want %q", created[0].OrganizationID, "org-fallback")
	}
	// A per-row organizationId is request input: the outer trusted tenant wins.
	if created[1].OrganizationID != "org-fallback" {
		t.Errorf("user 1 OrganizationID = %q, want %q", created[1].OrganizationID, "org-fallback")
	}
}

func TestBulkImportUsers_Empty(t *testing.T) {
	userRepo := &mockUserRepo{}
	uow := testUnitOfWork{users: userRepo}
	svc := Service{UOW: uow}

	result, err := svc.BulkImportUsers(context.Background(), "org-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("Total = %d, want 0", result.Total)
	}
	if result.Succeeded != 0 {
		t.Errorf("Succeeded = %d, want 0", result.Succeeded)
	}
}

// BE-A-21：JSON 导入行数上限边界——超过 maxCSVImportRows 必须在建号前整体拒绝（422），
// 且不得触发任何创建（每行含 argon2 哈希，超大请求会拖垮服务）。
func TestBulkImportUsers_ExceedsRowLimit(t *testing.T) {
	creates := 0
	userRepo := &mockUserRepo{
		createFn: func(_ context.Context, _ *userdomain.User) error { creates++; return nil },
	}
	uow := testUnitOfWork{users: userRepo, organizations: defaultOrgRepo()}
	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UUID: fakeUUIDGenerator{fixedID: "bulk-id"}, UOW: uow}

	users := make([]ImportUser, maxCSVImportRows+1)
	for i := range users {
		users[i] = ImportUser{Username: "u", DisplayName: "U", Email: "u@test.com", OrganizationID: "org-1"}
	}
	_, err := svc.BulkImportUsers(context.Background(), "org-default", users)
	if err == nil {
		t.Fatal("expected error for over-limit import")
	}
	if apperrors.FromError(err).Code != apperrors.CodeUnprocessableEntity {
		t.Errorf("code = %s, want unprocessable_entity", apperrors.FromError(err).Code)
	}
	if creates != 0 {
		t.Errorf("creates = %d, want 0 (must reject before any creation)", creates)
	}
}
