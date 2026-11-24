package users

import (
	"context"
	"errors"
	"testing"
	"time"

	groupsdomain "github.com/guliangiam/guliangiam/internal/domain/groups"
	roledomain "github.com/guliangiam/guliangiam/internal/domain/roles"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// stubRoleRepo / stubGroupRepo model tenant-scoped lookups: only ids present
// in own resolve; everything else is a foreign or missing tenant resource.
type stubRoleRepo struct {
	roledomain.Repository
	own map[string]bool
}

func (r *stubRoleRepo) GetByIDForOrganization(_ context.Context, id, _ string) (*roledomain.Role, error) {
	if r.own[id] {
		return &roledomain.Role{ID: id}, nil
	}
	return nil, apperrors.NewNotFound("role not found")
}

type stubGroupRepo struct {
	groupsdomain.Repository
	own map[string]bool
}

func (r *stubGroupRepo) GetByIDForOrganization(_ context.Context, id, _ string) (*groupsdomain.Group, error) {
	if r.own[id] {
		return &groupsdomain.Group{ID: id}, nil
	}
	return nil, apperrors.NewNotFound("group not found")
}

type createUserTenantObservations struct {
	created          *userdomain.User
	roleBindings     []userdomain.UserRoleBinding
	groupMemberships []userdomain.UserGroupMembership
}

func newCreateUserTenantStack(roles *stubRoleRepo, groups *stubGroupRepo) (*Service, *createUserTenantObservations) {
	observed := &createUserTenantObservations{}
	userRepo := &mockUserRepo{
		createFn: func(_ context.Context, user *userdomain.User) error {
			observed.created = user
			return nil
		},
		replaceRoleBindingsFn: func(_ context.Context, _ string, bindings []userdomain.UserRoleBinding) error {
			observed.roleBindings = bindings
			return nil
		},
		replaceGroupMembershipsFn: func(_ context.Context, _ string, memberships []userdomain.UserGroupMembership) error {
			observed.groupMemberships = memberships
			return nil
		},
	}
	uow := testUnitOfWork{users: userRepo, records: &fakeRecordsRepo{}, roles: roles, groups: groups}
	svc := &Service{
		Clock: fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUID:  fakeUUIDGenerator{fixedID: "019newid-f000-7000-8000-000000000099"},
		UOW:   uow,
	}
	return svc, observed
}

func createTenantUserCommand(roleIDs, groupIDs []string) CreateUserWithProfileCommand {
	return CreateUserWithProfileCommand{
		CreateUserCommand: CreateUserCommand{
			OrganizationID: testOrganizationID,
			ActorID:        "019operator-0000-7000-8000-000000000001",
			Username:       "newuser",
			UserType:       "normal",
			Status:         "active",
		},
		RoleBindingIDs: roleIDs,
		GroupIDs:       groupIDs,
		Profile: CreateUserProfileCommand{
			DisplayName: "New User",
			Email:       "newuser@example.com",
		},
	}
}

// R2-C06: a foreign or missing role/group id must reject the whole creation
// before any write or audit (mirrors ReplaceUserRoleBindings /
// ReplaceUserGroupMemberships).
func TestCreateUserRejectsForeignRoleBinding(t *testing.T) {
	roles := &stubRoleRepo{own: map[string]bool{"role-own": true}}
	groups := &stubGroupRepo{own: map[string]bool{"group-own": true}}
	svc, observed := newCreateUserTenantStack(roles, groups)

	_, err := svc.CreateUser(context.Background(), createTenantUserCommand([]string{"role-foreign"}, nil))
	assertNotFound(t, err)
	if observed.created != nil {
		t.Fatal("user row was created despite the foreign role id")
	}
}

func TestCreateUserRejectsForeignGroupMembership(t *testing.T) {
	roles := &stubRoleRepo{own: map[string]bool{"role-own": true}}
	groups := &stubGroupRepo{own: map[string]bool{"group-own": true}}
	svc, observed := newCreateUserTenantStack(roles, groups)

	_, err := svc.CreateUser(context.Background(), createTenantUserCommand(nil, []string{"group-foreign"}))
	assertNotFound(t, err)
	if observed.created != nil {
		t.Fatal("user row was created despite the foreign group id")
	}
}

// Control case: tenant-owned ids pass the in-transaction validation and the
// creation proceeds with bindings replaced.
func TestCreateUserAcceptsOwnRoleAndGroupBindings(t *testing.T) {
	roles := &stubRoleRepo{own: map[string]bool{"role-own": true}}
	groups := &stubGroupRepo{own: map[string]bool{"group-own": true}}
	svc, observed := newCreateUserTenantStack(roles, groups)

	if _, err := svc.CreateUser(context.Background(), createTenantUserCommand([]string{"role-own"}, []string{"group-own"})); err != nil {
		t.Fatalf("CreateUser() with own role/group ids: %v", err)
	}
	if observed.created == nil {
		t.Fatal("user row was not created")
	}
	if len(observed.roleBindings) != 1 || observed.roleBindings[0].RoleID != "role-own" {
		t.Fatalf("role bindings = %#v, want one binding for role-own", observed.roleBindings)
	}
	if len(observed.groupMemberships) != 1 || observed.groupMemberships[0].GroupID != "group-own" {
		t.Fatalf("group memberships = %#v, want one membership for group-own", observed.groupMemberships)
	}
}

func assertNotFound(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("foreign tenant resource must be rejected, got nil error")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error type = %T, want *apperrors.Error", err)
	}
	if appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("error code = %q, want %q", appErr.Code, apperrors.CodeNotFound)
	}
}
