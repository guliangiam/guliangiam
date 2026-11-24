package permissions

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	permissionmock "github.com/guliangiam/guliangiam/internal/domain/permissions/mock"
	roledomain "github.com/guliangiam/guliangiam/internal/domain/roles"
	rolemock "github.com/guliangiam/guliangiam/internal/domain/roles/mock"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	usermock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestListUserEffectivePermissions_ExpandsAndDeduplicates(t *testing.T) {
	ctrl := gomock.NewController(t)
	users := usermock.NewMockRepository(ctrl)
	roles := rolemock.NewMockRepository(ctrl)
	casbin := permissionmock.NewMockCasbinPolicyManager(ctrl)

	users.EXPECT().
		GetByIDForOrganization(gomock.Any(), "user-1", "org-1").
		Return(&userdomain.User{
			ID:             "user-1",
			OrganizationID: "org-1",
			RoleBindings: []userdomain.UserRoleBinding{
				{UserID: "user-1", RoleID: "role-1"},
				{UserID: "user-1", RoleID: "role-2"},
			},
		}, nil)
	roles.EXPECT().
		GetByIDForOrganization(gomock.Any(), "role-1", "org-1").
		Return(&roledomain.Role{ID: "role-1", Name: "admin"}, nil)
	roles.EXPECT().
		GetByIDForOrganization(gomock.Any(), "role-2", "org-1").
		Return(&roledomain.Role{ID: "role-2", Name: "auditor"}, nil)
	casbin.EXPECT().
		GetOrgSubjectPolicies("org-1", "admin").
		Return([][]string{
			{"admin", "org-1", "users", "read"},
			{"admin", "org-1", "users", "read"},
			{"admin", "org-1", "users", "write"},
		}, nil)
	casbin.EXPECT().
		GetOrgSubjectPolicies("org-1", "auditor").
		Return([][]string{{"auditor", "org-1", "audit_logs", "read"}}, nil)

	svc := Service{UOW: testUnitOfWork{userRepo: users, roleRepo: roles}, CasbinManager: casbin}
	grants, err := svc.ListUserEffectivePermissions(context.Background(), "org-1", "user-1")
	if err != nil {
		t.Fatalf("ListUserEffectivePermissions() error = %v", err)
	}

	want := []EffectivePermission{
		{Resource: "audit_logs", Action: "read", Role: "auditor"},
		{Resource: "users", Action: "read", Role: "admin"},
		{Resource: "users", Action: "write", Role: "admin"},
	}
	if len(grants) != len(want) {
		t.Fatalf("len(grants) = %d, want %d (%+v)", len(grants), len(want), grants)
	}
	for i := range want {
		if grants[i] != want[i] {
			t.Errorf("grants[%d] = %+v, want %+v", i, grants[i], want[i])
		}
	}
}

func TestListUserEffectivePermissions_RequiresOrganization(t *testing.T) {
	svc := Service{UOW: testUnitOfWork{}}
	if _, err := svc.ListUserEffectivePermissions(context.Background(), " ", "user-1"); err == nil {
		t.Fatal("expected invalid argument error")
	}
}

func TestListUserEffectivePermissions_PropagatesTenantMiss(t *testing.T) {
	ctrl := gomock.NewController(t)
	users := usermock.NewMockRepository(ctrl)
	users.EXPECT().
		GetByIDForOrganization(gomock.Any(), "user-1", "org-2").
		Return(nil, apperrors.NewNotFound("user not found"))

	svc := Service{UOW: testUnitOfWork{userRepo: users}, CasbinManager: permissionmock.NewMockCasbinPolicyManager(ctrl)}
	if _, err := svc.ListUserEffectivePermissions(context.Background(), "org-2", "user-1"); err == nil {
		t.Fatal("expected not found error")
	}
}
