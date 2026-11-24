// Package search 提供全局聚合搜索用例：按关键词在各核心实体的可搜索列
// （名称/显示名/邮箱/编号等）上做租户内检索，输出按实体分组的结果集，
// 供控制台全局搜索框（GET /api/v1/search）消费。
//
// 设计约束（批次 2 计划）：
//   - 复用各域仓储 List 已有的 Keyword 过滤能力，不新增 SQL；
//   - 单实体查询失败只降级为该组为空并记日志，不影响整体返回；
//   - 租户边界由 transport 层用可信 scope 解析后传入：普通成员始终携带
//     非空 OrganizationID（仓储显式 organization_id 过滤，MySQL 无 RLS）；
//     平台管理员可传空 OrganizationID 做跨组织聚合（可见范围由 super_admin
//     RLS 升级决定，与 /api/v1/global/* 同一口径）；
//   - records（审计流水）不进聚合：大表 keyword ILIKE 无索引支撑，全局搜索
//     框的高频触发会放大扫描成本，审计列表页已自带同词检索。
package search

import (
	"context"
	"log/slog"
	"strings"
	"unicode/utf8"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainapplications "github.com/guliangiam/guliangiam/internal/domain/applications"
	domaincerts "github.com/guliangiam/guliangiam/internal/domain/certs"
	domaingroups "github.com/guliangiam/guliangiam/internal/domain/groups"
	domaininvitations "github.com/guliangiam/guliangiam/internal/domain/invitations"
	domainkeys "github.com/guliangiam/guliangiam/internal/domain/keys"
	domainorganizations "github.com/guliangiam/guliangiam/internal/domain/organizations"
	domainpermissions "github.com/guliangiam/guliangiam/internal/domain/permissions"
	domainroles "github.com/guliangiam/guliangiam/internal/domain/roles"
	domainsessions "github.com/guliangiam/guliangiam/internal/domain/sessions"
	domaintickets "github.com/guliangiam/guliangiam/internal/domain/tickets"
	domaintokens "github.com/guliangiam/guliangiam/internal/domain/tokens"
	domainusers "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	// DefaultLimit 是未指定 limit 时的总结果上限。
	DefaultLimit = 50
	// MaxLimit 是调用方可请求的总结果上限。
	MaxLimit = 200
	// perGroupLimit 是每个实体组抓取的最大命中数。
	perGroupLimit = 5
	// MaxKeywordRunes 是搜索词长度上限（rune 计数）。
	MaxKeywordRunes = 64
)

// Query 是一次全局聚合搜索的输入。
type Query struct {
	// Keyword 是必填搜索词，由 transport 层传入前已 trim。
	Keyword string
	// OrganizationID 是可信租户边界；空值仅对平台管理员合法（跨组织聚合）。
	OrganizationID string
	// Limit 是总结果上限；<=0 取 DefaultLimit，>MaxLimit 收敛到 MaxLimit。
	Limit int
}

// Item 是一条命中实体。路由字段按实体类型取用：
//   - users：OrganizationSlug + Username → /users/{org}/{username}
//   - roles/permissions/groups/invitations：OrganizationID + Name → 名称型详情路由
//   - organizations：Name（= slug）→ /organizations/{slug}
//   - applications/certs/keys/tokens/tickets：ID → /{resource}/{id}
//   - sessions：无详情路由，前端跳列表页
type Item struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Subtitle         string `json:"subtitle"`
	Name             string `json:"name,omitempty"`
	Username         string `json:"username,omitempty"`
	OrganizationID   string `json:"organizationId,omitempty"`
	OrganizationSlug string `json:"organizationSlug,omitempty"`
}

// Group 是一个实体分组；Entity 是稳定机器名（users/applications/...），
// 前端据此渲染组标题与跳转路径。空组不会出现在结果里。
type Group struct {
	Entity string `json:"entity"`
	Items  []Item `json:"items"`
}

// Result 是聚合搜索输出。
type Result struct {
	Query  string  `json:"query"`
	Total  int     `json:"total"`
	Groups []Group `json:"groups"`
}

// Service 聚合各核心实体仓储的 Keyword 检索能力。
type Service struct {
	UOW appuow.UnitOfWork
}

// NewService 构造搜索用例服务。
func NewService(uow appuow.UnitOfWork) Service {
	return Service{UOW: uow}
}

// Search 执行聚合查询。输入非法返回 InvalidArgument；单实体查询失败仅
// 降级该组为空并记日志，整体仍返回 200 语义的结果。
func (s Service) Search(ctx context.Context, query Query) (*Result, error) {
	keyword := strings.TrimSpace(query.Keyword)
	if keyword == "" {
		return nil, apperrors.NewInvalidArgument("q is required")
	}
	if utf8.RuneCountInString(keyword) > MaxKeywordRunes {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "q exceeds the maximum length", map[string]any{
			"field": "q", "value": utf8.RuneCountInString(keyword), "max": MaxKeywordRunes,
		})
	}
	limit := query.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}

	result := &Result{Query: keyword, Groups: []Group{}}
	for _, searcher := range s.searchers() {
		items, err := searcher.search(ctx, keyword, query.OrganizationID)
		if err != nil {
			// 错误降级：单实体失败不影响整体（计划批次 2 明确允许）。
			slog.WarnContext(ctx, "global search: entity query failed, group degraded to empty",
				"entity", searcher.entity, "error", err)
			continue
		}
		if len(items) == 0 {
			continue
		}
		if remaining := limit - result.Total; remaining <= 0 {
			break
		} else if len(items) > remaining {
			items = items[:remaining]
		}
		result.Groups = append(result.Groups, Group{Entity: searcher.entity, Items: items})
		result.Total += len(items)
	}

	// users 详情路由需要 organizationSlug：UserListItem 不携带 slug，这里按命中
	// 用户所属组织去重后补查（普通成员至多 1 次；平台管理员至多 perGroupLimit 次）。
	s.fillUserOrganizationSlugs(ctx, result)
	return result, nil
}

type entitySearcher struct {
	entity string
	search func(ctx context.Context, keyword, organizationID string) ([]Item, error)
}

func (s Service) searchers() []entitySearcher {
	return []entitySearcher{
		{entity: "organizations", search: s.searchOrganizations},
		{entity: "users", search: s.searchUsers},
		{entity: "applications", search: s.searchApplications},
		{entity: "roles", search: s.searchRoles},
		{entity: "permissions", search: s.searchPermissions},
		{entity: "groups", search: s.searchGroups},
		{entity: "certs", search: s.searchCerts},
		{entity: "keys", search: s.searchKeys},
		{entity: "tokens", search: s.searchTokens},
		{entity: "sessions", search: s.searchSessions},
		{entity: "tickets", search: s.searchTickets},
		{entity: "invitations", search: s.searchInvitations},
	}
}

func (s Service) searchOrganizations(ctx context.Context, keyword, organizationID string) ([]Item, error) {
	rows, _, err := s.UOW.Organizations().List(ctx, domainorganizations.ListQuery{
		Page: 1, PageSize: perGroupLimit, Keyword: keyword,
	})
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(rows))
	for _, org := range rows {
		items = append(items, Item{
			ID: org.ID, Title: firstNonEmpty(org.DisplayName, org.Slug), Subtitle: org.Slug,
			Name: org.Slug, OrganizationSlug: org.Slug,
		})
	}
	return items, nil
}

func (s Service) searchUsers(ctx context.Context, keyword, organizationID string) ([]Item, error) {
	rows, _, err := s.UOW.Users().List(ctx, domainusers.ListQuery{
		Page: 1, PageSize: perGroupLimit, OrganizationID: organizationID, Keyword: keyword,
	})
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(rows))
	for _, user := range rows {
		items = append(items, Item{
			ID: user.ID, Title: firstNonEmpty(user.Profile.DisplayName, user.Username),
			Subtitle: firstNonEmpty(user.Profile.Email, user.Username),
			Username: user.Username, OrganizationID: user.OrganizationID,
		})
	}
	return items, nil
}

func (s Service) searchApplications(ctx context.Context, keyword, organizationID string) ([]Item, error) {
	rows, _, err := s.UOW.Applications().List(ctx, domainapplications.ListQuery{
		Page: 1, PageSize: perGroupLimit, OrganizationID: organizationID, Keyword: keyword,
	})
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(rows))
	for _, app := range rows {
		items = append(items, Item{
			ID: app.ID, Title: firstNonEmpty(app.DisplayName, app.Slug), Subtitle: app.Slug,
			Name: app.Slug, OrganizationID: app.OrganizationID,
		})
	}
	return items, nil
}

func (s Service) searchRoles(ctx context.Context, keyword, organizationID string) ([]Item, error) {
	rows, _, err := s.UOW.Roles().List(ctx, domainroles.ListQuery{
		Page: 1, PageSize: perGroupLimit, OrganizationID: organizationID, Keyword: keyword,
	})
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(rows))
	for _, role := range rows {
		items = append(items, Item{
			ID: role.ID, Title: firstNonEmpty(role.DisplayName, role.Name), Subtitle: role.Name,
			Name: role.Name, OrganizationID: role.OrganizationID,
		})
	}
	return items, nil
}

func (s Service) searchPermissions(ctx context.Context, keyword, organizationID string) ([]Item, error) {
	rows, _, err := s.UOW.Permissions().List(ctx, domainpermissions.ListQuery{
		Page: 1, PageSize: perGroupLimit, OrganizationID: organizationID, Keyword: keyword,
	})
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(rows))
	for _, permission := range rows {
		items = append(items, Item{
			ID: permission.ID, Title: firstNonEmpty(permission.DisplayName, permission.Name),
			Subtitle: firstNonEmpty(permission.ResourceType, permission.Name),
			Name:     permission.Name, OrganizationID: permission.OrganizationID,
		})
	}
	return items, nil
}

func (s Service) searchGroups(ctx context.Context, keyword, organizationID string) ([]Item, error) {
	rows, _, err := s.UOW.Groups().List(ctx, domaingroups.ListQuery{
		Page: 1, PageSize: perGroupLimit, OrganizationID: organizationID, Keyword: keyword,
	})
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(rows))
	for _, group := range rows {
		items = append(items, Item{
			ID: group.ID, Title: firstNonEmpty(group.DisplayName, group.Name), Subtitle: group.Name,
			Name: group.Name, OrganizationID: group.OrganizationID,
		})
	}
	return items, nil
}

func (s Service) searchCerts(ctx context.Context, keyword, organizationID string) ([]Item, error) {
	rows, _, err := s.UOW.Certs().List(ctx, domaincerts.ListQuery{
		Page: 1, PageSize: perGroupLimit, OrganizationID: organizationID, Keyword: keyword,
	})
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(rows))
	for _, cert := range rows {
		items = append(items, Item{
			ID: cert.ID, Title: firstNonEmpty(cert.DisplayName, cert.Name),
			Subtitle: firstNonEmpty(cert.Type, cert.Scope),
			Name:     cert.Name, OrganizationID: cert.OrganizationID,
		})
	}
	return items, nil
}

func (s Service) searchKeys(ctx context.Context, keyword, organizationID string) ([]Item, error) {
	rows, _, err := s.UOW.Keys().List(ctx, domainkeys.ListQuery{
		Page: 1, PageSize: perGroupLimit, OrganizationID: organizationID, Keyword: keyword,
	})
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(rows))
	for _, key := range rows {
		items = append(items, Item{
			ID: key.ID, Title: firstNonEmpty(key.DisplayName, key.Name), Subtitle: key.KeyType,
			Name: key.Name, OrganizationID: key.OrganizationID,
		})
	}
	return items, nil
}

func (s Service) searchTokens(ctx context.Context, keyword, organizationID string) ([]Item, error) {
	rows, _, err := s.UOW.Tokens().List(ctx, domaintokens.ListQuery{
		Page: 1, PageSize: perGroupLimit, OrganizationID: organizationID, Keyword: keyword,
	})
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(rows))
	for _, token := range rows {
		items = append(items, Item{
			ID: token.ID, Title: firstNonEmpty(token.TokenPrefix, token.ID),
			Subtitle:       firstNonEmpty(token.UserID, token.ApplicationID),
			OrganizationID: token.OrganizationID,
		})
	}
	return items, nil
}

func (s Service) searchSessions(ctx context.Context, keyword, organizationID string) ([]Item, error) {
	rows, _, err := s.UOW.Sessions().List(ctx, domainsessions.ListQuery{
		Page: 1, PageSize: perGroupLimit, OrganizationID: organizationID, Keyword: keyword,
	})
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(rows))
	for _, session := range rows {
		items = append(items, Item{
			ID: session.ID, Title: firstNonEmpty(session.Username, session.ID),
			Subtitle: session.OrganizationSlug,
			Username: session.Username, OrganizationID: session.OrganizationID,
			OrganizationSlug: session.OrganizationSlug,
		})
	}
	return items, nil
}

func (s Service) searchTickets(ctx context.Context, keyword, organizationID string) ([]Item, error) {
	rows, _, err := s.UOW.Tickets().List(ctx, domaintickets.ListQuery{
		Page: 1, PageSize: perGroupLimit, OrganizationID: organizationID, Keyword: keyword,
	})
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(rows))
	for _, ticket := range rows {
		items = append(items, Item{
			ID: ticket.ID, Title: firstNonEmpty(ticket.DisplayName, ticket.Name),
			Subtitle: ticket.Status, Name: ticket.Name, OrganizationID: ticket.OrganizationID,
		})
	}
	return items, nil
}

func (s Service) searchInvitations(ctx context.Context, keyword, organizationID string) ([]Item, error) {
	rows, _, err := s.UOW.Invitations().List(ctx, domaininvitations.ListQuery{
		Page: 1, PageSize: perGroupLimit, OrganizationID: organizationID, Keyword: keyword,
	})
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(rows))
	for _, invitation := range rows {
		// 详情路由为 /invitations/{organizationId}/{email}（与 InvitationsPage 跳转一致）。
		// Subtitle 只展示状态：邀请 Code 是凭据，全局搜索面向任意可搜索成员，
		// 把完整码放进响应等于让低权成员批量收集邀请码（BE-C-20）。
		items = append(items, Item{
			ID: invitation.ID, Title: invitation.Email,
			Subtitle: invitation.Status,
			Name:     invitation.Email, OrganizationID: invitation.OrganizationID,
		})
	}
	return items, nil
}

// fillUserOrganizationSlugs 为 users 组补 organizationSlug。补查失败只记日志，
// 对应条目保持空 slug（前端退化为跳用户列表页）。
func (s Service) fillUserOrganizationSlugs(ctx context.Context, result *Result) {
	var userItems []Item
	slugByOrg := map[string]string{}
	for _, group := range result.Groups {
		if group.Entity != "users" {
			continue
		}
		userItems = group.Items
		for _, item := range group.Items {
			if item.OrganizationID == "" {
				continue
			}
			if _, done := slugByOrg[item.OrganizationID]; done {
				continue
			}
			org, err := s.UOW.Organizations().GetByID(ctx, item.OrganizationID)
			if err != nil {
				slog.WarnContext(ctx, "global search: resolve organization slug failed",
					"organizationId", item.OrganizationID, "error", err)
				slugByOrg[item.OrganizationID] = ""
				continue
			}
			slugByOrg[item.OrganizationID] = org.Slug
		}
	}
	if len(userItems) == 0 {
		return
	}
	for i := range result.Groups {
		if result.Groups[i].Entity != "users" {
			continue
		}
		for j := range result.Groups[i].Items {
			result.Groups[i].Items[j].OrganizationSlug = slugByOrg[result.Groups[i].Items[j].OrganizationID]
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
