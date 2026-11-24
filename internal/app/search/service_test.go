package search

import (
	"context"
	"errors"
	"testing"

	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domainapplications "github.com/guliangiam/guliangiam/internal/domain/applications"
	applicationsmock "github.com/guliangiam/guliangiam/internal/domain/applications/mock"
	domaincerts "github.com/guliangiam/guliangiam/internal/domain/certs"
	certsmock "github.com/guliangiam/guliangiam/internal/domain/certs/mock"
	domaingroups "github.com/guliangiam/guliangiam/internal/domain/groups"
	groupsmock "github.com/guliangiam/guliangiam/internal/domain/groups/mock"
	domaininvitations "github.com/guliangiam/guliangiam/internal/domain/invitations"
	invitationsmock "github.com/guliangiam/guliangiam/internal/domain/invitations/mock"
	domainkeys "github.com/guliangiam/guliangiam/internal/domain/keys"
	keysmock "github.com/guliangiam/guliangiam/internal/domain/keys/mock"
	domainorganizations "github.com/guliangiam/guliangiam/internal/domain/organizations"
	organizationsmock "github.com/guliangiam/guliangiam/internal/domain/organizations/mock"
	domainpermissions "github.com/guliangiam/guliangiam/internal/domain/permissions"
	permissionsmock "github.com/guliangiam/guliangiam/internal/domain/permissions/mock"
	domainroles "github.com/guliangiam/guliangiam/internal/domain/roles"
	rolesmock "github.com/guliangiam/guliangiam/internal/domain/roles/mock"
	domainsessions "github.com/guliangiam/guliangiam/internal/domain/sessions"
	sessionsmock "github.com/guliangiam/guliangiam/internal/domain/sessions/mock"
	domaintickets "github.com/guliangiam/guliangiam/internal/domain/tickets"
	ticketsmock "github.com/guliangiam/guliangiam/internal/domain/tickets/mock"
	domaintokens "github.com/guliangiam/guliangiam/internal/domain/tokens"
	tokensmock "github.com/guliangiam/guliangiam/internal/domain/tokens/mock"
	domainusers "github.com/guliangiam/guliangiam/internal/domain/users"
	usersmock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// fixture 把 12 个实体仓储接好；每个仓储的 List 走可替换回调，默认返回空。
// 用例只设置自己关心的回调，避免 gomock 多期望匹配顺序造成的桩遮蔽。
type fixture struct {
	uow *appuowmock.MockUnitOfWork

	organizationsList func(domainorganizations.ListQuery) ([]*domainorganizations.Organization, int64, error)
	organizationsByID func(string) (*domainorganizations.Organization, error)
	usersList         func(domainusers.ListQuery) ([]*domainusers.User, int64, error)
	applicationsList  func(domainapplications.ListQuery) ([]*domainapplications.Application, int64, error)
	rolesList         func(domainroles.ListQuery) ([]*domainroles.Role, int64, error)
	permissionsList   func(domainpermissions.ListQuery) ([]*domainpermissions.Permission, int64, error)
	groupsList        func(domaingroups.ListQuery) ([]*domaingroups.Group, int64, error)
	certsList         func(domaincerts.ListQuery) ([]*domaincerts.Cert, int64, error)
	keysList          func(domainkeys.ListQuery) ([]*domainkeys.Key, int64, error)
	tokensList        func(domaintokens.ListQuery) ([]*domaintokens.Token, int64, error)
	sessionsList      func(domainsessions.ListQuery) ([]*domainsessions.Session, int64, error)
	ticketsList       func(domaintickets.ListQuery) ([]*domaintickets.Ticket, int64, error)
	invitationsList   func(domaininvitations.ListQuery) ([]*domaininvitations.Invitation, int64, error)
}

func newFixture(t *testing.T) (Service, *fixture) {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	f := &fixture{uow: appuowmock.NewMockUnitOfWork(ctrl)}
	organizations := organizationsmock.NewMockRepository(ctrl)
	users := usersmock.NewMockRepository(ctrl)
	applications := applicationsmock.NewMockRepository(ctrl)
	roles := rolesmock.NewMockRepository(ctrl)
	permissions := permissionsmock.NewMockRepository(ctrl)
	groups := groupsmock.NewMockRepository(ctrl)
	certs := certsmock.NewMockRepository(ctrl)
	keys := keysmock.NewMockRepository(ctrl)
	tokens := tokensmock.NewMockRepository(ctrl)
	sessions := sessionsmock.NewMockRepository(ctrl)
	tickets := ticketsmock.NewMockRepository(ctrl)
	invitations := invitationsmock.NewMockRepository(ctrl)

	f.uow.EXPECT().Organizations().Return(organizations).AnyTimes()
	f.uow.EXPECT().Users().Return(users).AnyTimes()
	f.uow.EXPECT().Applications().Return(applications).AnyTimes()
	f.uow.EXPECT().Roles().Return(roles).AnyTimes()
	f.uow.EXPECT().Permissions().Return(permissions).AnyTimes()
	f.uow.EXPECT().Groups().Return(groups).AnyTimes()
	f.uow.EXPECT().Certs().Return(certs).AnyTimes()
	f.uow.EXPECT().Keys().Return(keys).AnyTimes()
	f.uow.EXPECT().Tokens().Return(tokens).AnyTimes()
	f.uow.EXPECT().Sessions().Return(sessions).AnyTimes()
	f.uow.EXPECT().Tickets().Return(tickets).AnyTimes()
	f.uow.EXPECT().Invitations().Return(invitations).AnyTimes()

	organizations.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, query domainorganizations.ListQuery) ([]*domainorganizations.Organization, int64, error) {
			if f.organizationsList == nil {
				return nil, 0, nil
			}
			return f.organizationsList(query)
		}).AnyTimes()
	organizations.EXPECT().GetByID(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, id string) (*domainorganizations.Organization, error) {
			if f.organizationsByID == nil {
				return nil, errors.New("unexpected GetByID")
			}
			return f.organizationsByID(id)
		}).AnyTimes()
	users.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, query domainusers.ListQuery) ([]*domainusers.User, int64, error) {
			if f.usersList == nil {
				return nil, 0, nil
			}
			return f.usersList(query)
		}).AnyTimes()
	applications.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, query domainapplications.ListQuery) ([]*domainapplications.Application, int64, error) {
			if f.applicationsList == nil {
				return nil, 0, nil
			}
			return f.applicationsList(query)
		}).AnyTimes()
	roles.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, query domainroles.ListQuery) ([]*domainroles.Role, int64, error) {
			if f.rolesList == nil {
				return nil, 0, nil
			}
			return f.rolesList(query)
		}).AnyTimes()
	permissions.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, query domainpermissions.ListQuery) ([]*domainpermissions.Permission, int64, error) {
			if f.permissionsList == nil {
				return nil, 0, nil
			}
			return f.permissionsList(query)
		}).AnyTimes()
	groups.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, query domaingroups.ListQuery) ([]*domaingroups.Group, int64, error) {
			if f.groupsList == nil {
				return nil, 0, nil
			}
			return f.groupsList(query)
		}).AnyTimes()
	certs.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, query domaincerts.ListQuery) ([]*domaincerts.Cert, int64, error) {
			if f.certsList == nil {
				return nil, 0, nil
			}
			return f.certsList(query)
		}).AnyTimes()
	keys.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, query domainkeys.ListQuery) ([]*domainkeys.Key, int64, error) {
			if f.keysList == nil {
				return nil, 0, nil
			}
			return f.keysList(query)
		}).AnyTimes()
	tokens.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, query domaintokens.ListQuery) ([]*domaintokens.Token, int64, error) {
			if f.tokensList == nil {
				return nil, 0, nil
			}
			return f.tokensList(query)
		}).AnyTimes()
	sessions.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, query domainsessions.ListQuery) ([]*domainsessions.Session, int64, error) {
			if f.sessionsList == nil {
				return nil, 0, nil
			}
			return f.sessionsList(query)
		}).AnyTimes()
	tickets.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, query domaintickets.ListQuery) ([]*domaintickets.Ticket, int64, error) {
			if f.ticketsList == nil {
				return nil, 0, nil
			}
			return f.ticketsList(query)
		}).AnyTimes()
	invitations.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, query domaininvitations.ListQuery) ([]*domaininvitations.Invitation, int64, error) {
			if f.invitationsList == nil {
				return nil, 0, nil
			}
			return f.invitationsList(query)
		}).AnyTimes()

	return NewService(f.uow), f
}

// TestSearch_GroupsResultsByEntity 固定聚合输出：按实体分组、组内条目携带
// 标题/副标题与详情路由字段，users 组补齐 organizationSlug。
func TestSearch_GroupsResultsByEntity(t *testing.T) {
	service, f := newFixture(t)

	f.organizationsList = func(query domainorganizations.ListQuery) ([]*domainorganizations.Organization, int64, error) {
		return []*domainorganizations.Organization{
			{ID: "org-1", Slug: "acme", DisplayName: "Acme Inc"},
		}, 1, nil
	}
	f.usersList = func(query domainusers.ListQuery) ([]*domainusers.User, int64, error) {
		return []*domainusers.User{
			{ID: "user-1", OrganizationID: "org-1", Username: "alice", Profile: domainusers.UserProfile{
				DisplayName: "Alice Zhang", Email: "alice@acme.example",
			}},
		}, 1, nil
	}
	f.applicationsList = func(query domainapplications.ListQuery) ([]*domainapplications.Application, int64, error) {
		return []*domainapplications.Application{
			{ID: "app-1", OrganizationID: "org-1", Slug: "portal", DisplayName: "Portal"},
		}, 1, nil
	}
	f.organizationsByID = func(id string) (*domainorganizations.Organization, error) {
		require.Equal(t, "org-1", id)
		return &domainorganizations.Organization{ID: "org-1", Slug: "acme"}, nil
	}

	result, err := service.Search(context.Background(), Query{Keyword: "al", OrganizationID: "org-1"})
	require.NoError(t, err)
	require.Equal(t, 3, result.Total)
	require.Len(t, result.Groups, 3)
	assert.Equal(t, "organizations", result.Groups[0].Entity)
	assert.Equal(t, "users", result.Groups[1].Entity)
	assert.Equal(t, "applications", result.Groups[2].Entity)

	orgItem := result.Groups[0].Items[0]
	assert.Equal(t, "Acme Inc", orgItem.Title)
	assert.Equal(t, "acme", orgItem.Name)

	userItem := result.Groups[1].Items[0]
	assert.Equal(t, "Alice Zhang", userItem.Title)
	assert.Equal(t, "alice@acme.example", userItem.Subtitle)
	assert.Equal(t, "alice", userItem.Username)
	assert.Equal(t, "acme", userItem.OrganizationSlug, "users 组必须补查 organizationSlug 供前端详情路由")

	appItem := result.Groups[2].Items[0]
	assert.Equal(t, "Portal", appItem.Title)
	assert.Equal(t, "app-1", appItem.ID)
}

// TestSearch_DegradesFailedEntity 单实体查询失败只降级该组为空，整体不报错。
func TestSearch_DegradesFailedEntity(t *testing.T) {
	service, f := newFixture(t)

	f.usersList = func(query domainusers.ListQuery) ([]*domainusers.User, int64, error) {
		return nil, 0, errors.New("boom")
	}
	f.applicationsList = func(query domainapplications.ListQuery) ([]*domainapplications.Application, int64, error) {
		return []*domainapplications.Application{
			{ID: "app-1", Slug: "portal", DisplayName: "Portal"},
		}, 1, nil
	}

	result, err := service.Search(context.Background(), Query{Keyword: "portal", OrganizationID: "org-1"})
	require.NoError(t, err, "单实体失败不得升级为整体失败")
	require.Len(t, result.Groups, 1)
	assert.Equal(t, "applications", result.Groups[0].Entity)
}

// TestSearch_PassesTrustedOrganizationToRepositories 固定可信租户边界透传：
// 除 organizations 目录外，每个仓储都必须收到显式 organization_id 过滤
// （MySQL 无 RLS，租户隔离只能靠查询条件）。
func TestSearch_PassesTrustedOrganizationToRepositories(t *testing.T) {
	service, f := newFixture(t)

	f.usersList = func(query domainusers.ListQuery) ([]*domainusers.User, int64, error) {
		assert.Equal(t, "org-a", query.OrganizationID, "users 必须显式租户过滤")
		assert.Equal(t, "bob", query.Keyword)
		assert.Equal(t, perGroupLimit, query.PageSize)
		return nil, 0, nil
	}
	f.rolesList = func(query domainroles.ListQuery) ([]*domainroles.Role, int64, error) {
		assert.Equal(t, "org-a", query.OrganizationID, "roles 必须显式租户过滤")
		return nil, 0, nil
	}
	f.invitationsList = func(query domaininvitations.ListQuery) ([]*domaininvitations.Invitation, int64, error) {
		assert.Equal(t, "org-a", query.OrganizationID, "invitations 必须显式租户过滤")
		return nil, 0, nil
	}
	f.ticketsList = func(query domaintickets.ListQuery) ([]*domaintickets.Ticket, int64, error) {
		assert.Equal(t, "org-a", query.OrganizationID, "tickets 必须显式租户过滤")
		return nil, 0, nil
	}

	_, err := service.Search(context.Background(), Query{Keyword: "bob", OrganizationID: "org-a"})
	require.NoError(t, err)
}

// TestSearch_TrimsToTotalLimit 固定总上限裁剪：按实体顺序累计到 limit 即停。
func TestSearch_TrimsToTotalLimit(t *testing.T) {
	service, f := newFixture(t)

	users := make([]*domainusers.User, 0, perGroupLimit)
	for i := 0; i < perGroupLimit; i++ {
		users = append(users, &domainusers.User{ID: "user-" + string(rune('a'+i)), Username: "u"})
	}
	apps := make([]*domainapplications.Application, 0, perGroupLimit)
	for i := 0; i < perGroupLimit; i++ {
		apps = append(apps, &domainapplications.Application{ID: "app-" + string(rune('a'+i)), Slug: "s"})
	}
	f.usersList = func(query domainusers.ListQuery) ([]*domainusers.User, int64, error) {
		return users, int64(len(users)), nil
	}
	f.applicationsList = func(query domainapplications.ListQuery) ([]*domainapplications.Application, int64, error) {
		return apps, int64(len(apps)), nil
	}
	f.organizationsByID = func(id string) (*domainorganizations.Organization, error) {
		return nil, errors.New("not found")
	}

	result, err := service.Search(context.Background(), Query{Keyword: "x", OrganizationID: "org-1", Limit: 7})
	require.NoError(t, err)
	assert.Equal(t, 7, result.Total)
	require.Len(t, result.Groups, 2)
	assert.Len(t, result.Groups[0].Items, perGroupLimit)
	assert.Len(t, result.Groups[1].Items, 7-perGroupLimit, "末组必须裁剪到总上限")
}

// TestSearch_KeywordValidation 固定输入校验：空词是 InvalidArgument（缺字段），
// 超长词是 UnprocessableEntity（语义范围校验，手册 4.14.1）。
func TestSearch_KeywordValidation(t *testing.T) {
	service, _ := newFixture(t)

	_, err := service.Search(context.Background(), Query{Keyword: "   ", OrganizationID: "org-1"})
	require.Error(t, err)
	var invalid *apperrors.Error
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, apperrors.CodeInvalidArgument, invalid.Code)

	long := make([]rune, MaxKeywordRunes+1)
	for i := range long {
		long[i] = 'a'
	}
	_, err = service.Search(context.Background(), Query{Keyword: string(long), OrganizationID: "org-1"})
	require.Error(t, err)
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, invalid.Code)
}
