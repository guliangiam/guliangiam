package users

import (
	"context"
	"errors"
	"testing"
	"time"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type recordedRoleSync struct {
	orgID    string
	orgSlug  string
	username string
	roles    []string
}

type recordingRolePolicyManager struct {
	calls []recordedRoleSync
	err   error
}

func (m *recordingRolePolicyManager) SyncUserRoles(_ context.Context, orgID, orgSlug, username string, roleNames []string) error {
	m.calls = append(m.calls, recordedRoleSync{orgID: orgID, orgSlug: orgSlug, username: username, roles: append([]string(nil), roleNames...)})
	return m.err
}

func roleSyncFixture(t *testing.T, manager RolePolicyManager) Service {
	t.Helper()
	userRepo := &svcMockUserRepo{
		getByIDFn: func(context.Context, string) (*userdomain.User, error) {
			return &userdomain.User{
				ID: "user-1", OrganizationID: "org-1", Username: "approver",
				RoleBindings: []userdomain.UserRoleBinding{{UserID: "user-1", RoleID: "role-1"}},
			}, nil
		},
		listRoleNamesFn: func(context.Context, string) ([]string, error) {
			return []string{"built-in-editor"}, nil
		},
	}
	orgRepo := &svcMockOrgRepo{
		getByIDFn: func(context.Context, string) (*orgdomain.Organization, error) {
			return &orgdomain.Organization{ID: "org-1", Slug: "built-in"}, nil
		},
	}
	return Service{
		Clock:        svcFakeClock{fixedTime: time.Now()},
		UOW:          svcTestUnitOfWork{users: userRepo, organizations: orgRepo, roles: svcAnyRoleRepo{}},
		RolePolicies: manager,
	}
}

func TestReplaceUserRoleBindings_ProjectsRolesIntoCasbin(t *testing.T) {
	manager := &recordingRolePolicyManager{}
	svc := roleSyncFixture(t, manager)

	if err := svc.ReplaceUserRoleBindings(context.Background(), "org-1", "user-1", []ReplaceUserRoleBindingCommand{{RoleID: "role-1"}}); err != nil {
		t.Fatalf("ReplaceUserRoleBindings() error = %v", err)
	}
	if len(manager.calls) != 1 {
		t.Fatalf("SyncUserRoles calls = %d, want 1", len(manager.calls))
	}
	call := manager.calls[0]
	if call.orgID != "org-1" || call.orgSlug != "built-in" || call.username != "approver" {
		t.Fatalf("unexpected sync scope: %+v", call)
	}
	if len(call.roles) != 1 || call.roles[0] != "built-in-editor" {
		t.Fatalf("roles = %v, want [built-in-editor]", call.roles)
	}
}

func TestDeleteUserRoleBinding_ProjectsRolesIntoCasbin(t *testing.T) {
	manager := &recordingRolePolicyManager{}
	svc := roleSyncFixture(t, manager)

	if err := svc.DeleteUserRoleBinding(context.Background(), "org-1", "user-1", "role-1"); err != nil {
		t.Fatalf("DeleteUserRoleBinding() error = %v", err)
	}
	if len(manager.calls) != 1 {
		t.Fatalf("SyncUserRoles calls = %d, want 1", len(manager.calls))
	}
}

func TestReplaceUserRoleBindings_SurfacesPolicySyncFailure(t *testing.T) {
	manager := &recordingRolePolicyManager{err: apperrors.NewInternal("policy store unavailable")}
	svc := roleSyncFixture(t, manager)

	err := svc.ReplaceUserRoleBindings(context.Background(), "org-1", "user-1", []ReplaceUserRoleBindingCommand{{RoleID: "role-1"}})
	if err == nil {
		t.Fatal("expected error when casbin sync fails")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeInternal {
		t.Fatalf("error = %v, want internal app error", err)
	}
}
