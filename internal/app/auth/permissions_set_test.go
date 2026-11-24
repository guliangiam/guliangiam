package auth

import (
	"strings"
	"testing"

	domainpermissions "github.com/guliangiam/guliangiam/internal/domain/permissions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubCasbinPolicies 只实现 GetOrgSubjectPolicies；其余方法经嵌入接口透传，
// 本组用例不会调用，避免为 25 个方法写样板。
type stubCasbinPolicies struct {
	domainpermissions.CasbinPolicyManager
	policies [][]string
}

func (s stubCasbinPolicies) GetOrgSubjectPolicies(string, string) ([][]string, error) {
	return s.policies, nil
}

func TestAdminPermissionSet_IncludesB2BDomains(t *testing.T) {
	set := adminPermissionSet()
	require.Contains(t, set.Actions, "b2b:read")
	require.Contains(t, set.Actions, "b2b:write")
	require.Contains(t, set.Resources, "domains")
}

// 三个安全扩展 provider 的路由 DELETE 均声明为 write（无独立 delete 动作），
// 管理员回退集不得下发无路由支撑的 :delete，否则前端多出永远无效的权限点。
func TestAdminPermissionSet_OmitsProviderDeleteKeys(t *testing.T) {
	set := adminPermissionSet()
	for _, key := range []string{
		"captcha-provider:delete",
		"notification-provider:delete",
		"face-id-provider:delete",
	} {
		require.NotContains(t, set.Actions, key, "无路由支撑的 key 不应下发: %s", key)
	}
	for _, key := range []string{
		"captcha-provider:read", "captcha-provider:write",
		"notification-provider:read", "notification-provider:write",
		"face-id-provider:read", "face-id-provider:write",
	} {
		require.Contains(t, set.Actions, key)
	}
}

// organizations 走 (a) 路线：后端输出由单数 `org:*` 改为复数
// `organizations:*`，与路由声明/permissions.yaml 一致。DELETE 有独立动作
// `organizations:delete`（routes.go:71 + permissions.yaml:103），故管理员回退集
// 必须下发它；旧单数拼写 `org:*`（含 `org:delete`）不得下发。
func TestAdminPermissionSet_UsesPluralOrganizationsKeys(t *testing.T) {
	set := adminPermissionSet()
	require.Contains(t, set.Actions, "organizations:read")
	require.Contains(t, set.Actions, "organizations:write")
	require.Contains(t, set.Actions, "organizations:delete")
	for _, legacy := range []string{"org:read", "org:write", "org:delete"} {
		require.NotContains(t, set.Actions, legacy, "不应下发旧拼写 key: %s", legacy)
	}
	require.Contains(t, set.Resources, resourceOrganizations)
}

// 批次9a：无角色用户 fallback 收敛为仅自助面（"/" 与 "/account"），
// 不再默认授予任何读 action——未授权菜单必须完全隐藏，且工单等列表接口
// 无属主过滤，默认授予读权限会泄露他人数据。
func TestViewerPermissionSet_SelfServiceOnly(t *testing.T) {
	set := viewerPermissionSet()
	require.Equal(t, []string{"/", "/account"}, set.Menus)
	require.Empty(t, set.Actions)
	require.Empty(t, set.Resources)
}

func TestResourceToActionKey_MapsOrganizationsToPlural(t *testing.T) {
	assert.Equal(t, "organizations", resourceToActionKey[resourceOrganizations])
}

// 通配 p-策略（obj="*"）同样按映射展开，必须输出复数 organizations:*。
func TestResolveFromRealCasbin_WildcardEmitsPluralOrganizationsKeys(t *testing.T) {
	svc := &Service{CasbinPolicyManager: stubCasbinPolicies{
		policies: [][]string{{"admin", "built-in", "*", "write"}},
	}}

	actions, resources := svc.resolveFromRealCasbin("org-1", []string{"admin"})

	require.Contains(t, actions, "organizations:write")
	require.NotContains(t, actions, "org:write")
	require.Contains(t, resources, resourceOrganizations)
}

// 管理员此前拿不到 Actions/ReBAC/Risk/Metering 的写权限：它们既不在
// adminPermissionSet，也未在 permissions.yaml 登记，前端 PermissionGuard
// 因此把写按钮全部隐藏。本用例钉住管理员回退集与注册表的同批补齐。
func TestAdminPermissionSet_IncludesGovernanceDomains(t *testing.T) {
	set := adminPermissionSet()
	for _, key := range []string{
		"actions:read", "actions:write",
		"rebac:read", "rebac:write", "rebac:check",
		"risk:read", "risk:write",
		"metering:read", "metering:write",
		"iga:read", "iga:write",
	} {
		require.Contains(t, set.Actions, key, "adminPermissionSet missing %s", key)
	}
	for _, resource := range []string{resourceActions, resourceReBAC, resourceRisk, resourceMetering, resourceIGA} {
		require.Contains(t, set.Resources, resource, "adminPermissionSet missing resource %s", resource)
	}
}

// 通配 p-策略（obj="*"）按 resourceToActionKey 展开。Actions/ReBAC/Risk/
// Metering 若不在映射里，管理员（以及任何持通配策略的角色）在
// GET /api/v1/auth/permissions 中拿不到这些 action/resource，前端
// PermissionGuard 会把对应页面的写按钮全部隐藏。
func TestResolveFromRealCasbin_WildcardExpandsGovernanceSurfaces(t *testing.T) {
	svc := &Service{CasbinPolicyManager: stubCasbinPolicies{
		policies: [][]string{
			{"admin", "built-in", "*", "*"},
			{"admin", "built-in", "*", "write"},
		},
	}}

	actions, resources := svc.resolveFromRealCasbin("org-1", []string{"admin"})

	// act="*" 展开为 CRUD 三动作；前端只做精确匹配，"actions:*" 不再是有效授权键。
	for _, key := range []string{"actions:read", "actions:write", "actions:delete", "rebac:read", "rebac:write", "risk:write", "metering:write"} {
		assert.Contains(t, actions, key)
	}
	assert.NotContains(t, actions, "actions:*")
	for _, resource := range []string{resourceActions, resourceReBAC, resourceRisk, resourceMetering} {
		assert.Contains(t, resources, resource)
	}
}

// 治理面 Casbin obj 与前端 action 前缀同名；四条映射缺失即为本缺口的代码侧根因。
func TestResourceToActionKey_MapsGovernanceSurfaces(t *testing.T) {
	for _, resource := range []string{resourceActions, resourceReBAC, resourceRisk, resourceMetering} {
		assert.Equal(t, resource, resourceToActionKey[resource])
	}
}

// 通配 p-策略（obj="*"）按 resourceToActionKey 展开；缺少 domains→b2b 映射时
// 前端永远拿不到 b2b:read，/organizations/:org/domains 会被 PermissionGuard 判 403。
func TestResourceToActionKey_MapsDomainsToB2B(t *testing.T) {
	assert.Equal(t, "b2b", resourceToActionKey["domains"])
}

func TestResolveFromRealCasbin_WildcardExpandsB2B(t *testing.T) {
	svc := &Service{CasbinPolicyManager: stubCasbinPolicies{
		policies: [][]string{{"admin", "built-in", "*", "read"}},
	}}

	actions, resources := svc.resolveFromRealCasbin("org-1", []string{"admin"})

	assert.Contains(t, actions, "b2b:read")
	assert.Contains(t, resources, "domains")
}

// 前端 guard 的 action key 必须与 admin/viewer 动作集、Casbin 资源派生结果一致，
// 否则 Workload/Forms/Entries/Syncers 页面按钮会对管理员或非管理员单侧消失。
func TestAdminPermissionSet_CoversWorkloadFormsEntriesSyncers(t *testing.T) {
	set := adminPermissionSet()
	for _, action := range []string{
		"identity-pools:read", "identity-pools:write",
		"form:read", "form:write",
		"entry:read", "entry:write",
		"syncer:read", "syncer:write", "syncer:delete",
	} {
		require.Contains(t, set.Actions, action)
	}
	require.Contains(t, set.Resources, "identity-pools")
	// The legacy "sync:*" alias remains until the SyncersPage guard (F3.2b)
	// moves to the canonical "syncer:write"; only the canonical key is locked.
}

func TestResourceToActionKey_MapsSyncerAndIdentityPools(t *testing.T) {
	assert.Equal(t, "syncer", resourceToActionKey["syncers"])
	assert.Equal(t, "identity-pools", resourceToActionKey["identity-pools"])
}

// 路由声明与 permissions.yaml 以复数 `users:*` 为唯一规范。历史聚合输出单数
// `user:*`，前端只能用别名兼容；本用例钉住代码侧不再产出单数形式。
func TestAdminPermissionSet_UsesPluralUsersKeys(t *testing.T) {
	set := adminPermissionSet()
	require.Contains(t, set.Actions, "users:read")
	require.Contains(t, set.Actions, "users:write")
	for _, legacy := range []string{"user:read", "user:write", "user:delete"} {
		require.NotContains(t, set.Actions, legacy, "legacy singular key leaked: %s", legacy)
	}
	require.Contains(t, set.Resources, "users")
}

// 通配 p-策略（obj="*"）同样按映射展开，必须输出复数 users:*。
// （批次9a 后 viewer fallback 不再下发任何 action，复数 users:* 拼写由
// adminPermissionSet 与本用例钉住。）
func TestResolveFromRealCasbin_WildcardEmitsPluralUsersKeys(t *testing.T) {
	svc := &Service{CasbinPolicyManager: stubCasbinPolicies{
		policies: [][]string{{"admin", "built-in", "*", "write"}},
	}}

	actions, resources := svc.resolveFromRealCasbin("org-1", []string{"admin"})

	require.Contains(t, actions, "users:write")
	require.NotContains(t, actions, "user:write")
	require.Contains(t, resources, "users")
}

func TestResolveFromRealCasbin_DerivesWorkloadFormsEntriesSyncersActions(t *testing.T) {
	svc := &Service{CasbinPolicyManager: stubCasbinPolicies{
		policies: [][]string{
			{"editor", "built-in", "identity-pools", "read"},
			{"editor", "built-in", "forms", "write"},
			{"editor", "built-in", "entries", "write"},
			{"editor", "built-in", "syncers", "write"},
		},
	}}

	actions, resources := svc.resolveFromRealCasbin("org-1", []string{"editor"})

	assert.Contains(t, actions, "identity-pools:read")
	assert.Contains(t, actions, "form:write")
	assert.Contains(t, actions, "entry:write")
	assert.Contains(t, actions, "syncer:write")
	assert.Contains(t, resources, "identity-pools")
	assert.Contains(t, resources, "forms")
	assert.Contains(t, resources, "entries")
	assert.Contains(t, resources, "syncers")
}

// 支付对账/渠道配置、Adapters、I18n 的 guard key 必须同时存在于 admin 兜底
// 与 viewer 只读兜底，否则 B6 接线后管理员或被隐藏/单侧消失。
func TestAdminPermissionSet_CoversReconChannelsAdaptersI18n(t *testing.T) {
	set := adminPermissionSet()
	for _, action := range []string{
		"adapter:read", "adapter:write", "adapter:delete",
		"pay-recon:write",
		"pay-recon-items:write",
		"pay-channel-configs:read", "pay-channel-configs:write",
		"i18n:read", "i18n:write",
	} {
		require.Contains(t, set.Actions, action)
	}
	for _, resource := range []string{
		"adapters", "pay-recon", "pay-recon-items", "pay-channel-configs", "i18n",
	} {
		require.Contains(t, set.Resources, resource)
	}
}

// 路径首段资源必须进入 resourceToActionKey，否则通配角色（editor/operator）
// 拿不到 pay-recon / pay-channel-configs / i18n 的前端 key。
func TestResourceToActionKey_MapsReconChannelsAdaptersI18n(t *testing.T) {
	assert.Equal(t, "pay-recon", resourceToActionKey["pay-recon"])
	assert.Equal(t, "pay-recon-items", resourceToActionKey["pay-recon-items"])
	assert.Equal(t, "pay-channel-configs", resourceToActionKey["pay-channel-configs"])
	assert.Equal(t, "adapter", resourceToActionKey["adapters"])
	assert.Equal(t, "i18n", resourceToActionKey["i18n"])
}

func TestResolveFromRealCasbin_ExpandsReconChannelsAdaptersI18n(t *testing.T) {
	// 通配读策略按 map 展开；显式写策略同时保留 HTTP 动词 key 与 CRUD alias。
	svc := &Service{CasbinPolicyManager: stubCasbinPolicies{
		policies: [][]string{
			{"editor", "built-in", "*", "read"},
			{"editor", "built-in", "pay-recon", "post"},
			{"editor", "built-in", "pay-recon-items", "post"},
			{"editor", "built-in", "pay-channel-configs", "write"},
			{"editor", "built-in", "adapters", "write"},
			{"editor", "built-in", "i18n", "put"},
		},
	}}

	actions, resources := svc.resolveFromRealCasbin("org-1", []string{"editor"})

	for _, key := range []string{
		"adapter:read",
		"pay-recon:read",
		"pay-recon-items:read",
		"pay-channel-configs:read",
		"i18n:read",
	} {
		require.Contains(t, actions, key)
	}
	// 旧 key 只增不减：显式策略的 verb key 仍原样输出。
	for _, legacy := range []string{
		"pay-recon:post", "pay-recon-items:post", "i18n:put",
	} {
		require.Contains(t, actions, legacy)
	}
	for _, key := range []string{
		"pay-recon:write", "pay-recon-items:write",
		"pay-channel-configs:write", "adapter:write", "i18n:write",
	} {
		require.Contains(t, actions, key)
	}
	for _, resource := range []string{
		"adapters", "pay-recon", "pay-recon-items", "pay-channel-configs", "i18n",
	} {
		require.Contains(t, resources, resource)
	}
}

// 批次9a：deriveMenus 必须只保留 actions 授权的菜单键，公开键 "/"、"/account"
// 恒可见，"/system-info" 仅 admin 保留，其余按 menuRequiredAction 过滤；
// 返回顺序与 fullMenus 一致且非 nil。
func TestDeriveMenus_FiltersByActionsPreservesOrder(t *testing.T) {
	menus := deriveMenus([]string{"users:read", "record:read", "ldaps:read"})
	require.NotNil(t, menus)

	// 公开键恒在
	require.Contains(t, menus, "/")
	require.Contains(t, menus, "/account")
	// 授权键保留
	require.Contains(t, menus, "/users")
	require.Contains(t, menus, "/records")
	require.Contains(t, menus, "/ldaps")
	// 未授权键隐藏
	require.NotContains(t, menus, "/organizations")
	require.NotContains(t, menus, "/sessions")
	// admin-only 键永不派生
	require.NotContains(t, menus, "/system-info")
	require.NotContains(t, menus, "/shortcuts")

	// 顺序与 fullMenus 相对顺序一致
	idx := func(key string) int {
		for i, m := range fullMenus {
			if m == key {
				return i
			}
		}
		return -1
	}
	for i := 1; i < len(menus); i++ {
		require.Less(t, idx(menus[i-1]), idx(menus[i]), "menus 必须保持 fullMenus 顺序")
	}
}

// menuRequiredAction 的每个值都必须与 resolvePermissionsFromCasbin 实际产出的
// action 拼写一致：值要么命中 resourceToActionKey 的前缀（复数/单数以映射为准），
// 要么是 audit/idv/verifications/radius-accountings 这类路径派生特例。
// 拼写漂移会让有角色的用户菜单被误隐藏。
func TestMenuRequiredAction_SpellsKnownActionKeys(t *testing.T) {
	prefixes := make(map[string]struct{}, len(resourceToActionKey))
	for _, prefix := range resourceToActionKey {
		prefixes[prefix] = struct{}{}
	}
	// 路径首段资源不在 resourceToActionKey 通配展开里，但显式 p-策略按 obj
	// 原样派生前缀；这些菜单键依赖显式授权，前缀即 obj 本身。
	pathDerived := map[string]struct{}{
		"audit":              {},
		"migration":          {},
		"idv":                {},
		"verifications":      {},
		"radius-accountings": {},
		"notifications":      {},
		"aigateway-usage":    {},
		"aigateway-health":   {},
	}

	for menuKey, action := range menuRequiredAction {
		require.True(t, strings.HasSuffix(action, ":read"), "%s 必须是 :read 动作", menuKey)
		prefix := strings.TrimSuffix(action, ":read")
		_, known := prefixes[prefix]
		_, derived := pathDerived[prefix]
		require.True(t, known || derived, "%s → %s 的前缀 %q 不在已知 action 派生集合内", menuKey, action, prefix)
	}

	// 钉住易错的复数/单数拼写，防止回归。
	require.Equal(t, "users:read", menuRequiredAction["/users"])
	require.Equal(t, "ldaps:read", menuRequiredAction["/ldaps"])
	require.Equal(t, "rules:read", menuRequiredAction["/rules"])
	require.Equal(t, "actions:read", menuRequiredAction["/actions"])
	require.Equal(t, "rebac:read", menuRequiredAction["/rebac"])
	require.Equal(t, "record:read", menuRequiredAction["/records"])
	require.Equal(t, "app:read", menuRequiredAction["/applications"])
	require.Equal(t, "server:read", menuRequiredAction["/servers"])
	require.Equal(t, "aigateway-provider:read", menuRequiredAction["/ai/providers"])
	require.Equal(t, "verifications:read", menuRequiredAction["/verifications"])
	require.Equal(t, "radius-accountings:read", menuRequiredAction["/radius-accountings"])
	require.Equal(t, "audit:read", menuRequiredAction["/audit/login-logs"])
	require.Equal(t, "idv:read", menuRequiredAction["/idv"])
}

// fullMenus 中除公开键与 admin-only 键外，每个菜单键都必须有 required-action 映射，
// 否则 deriveMenus 会把它当成公开键恒放行。
func TestMenuRequiredAction_CoversAllGuardedMenus(t *testing.T) {
	public := map[string]struct{}{"/": {}, "/account": {}}
	for _, key := range fullMenus {
		if _, ok := public[key]; ok {
			continue
		}
		if _, adminOnly := adminOnlyMenus[key]; adminOnly {
			require.NotContains(t, menuRequiredAction, key, "%s 不应同时有 required-action 映射", key)
			continue
		}
		_, mapped := menuRequiredAction[key]
		require.True(t, mapped, "%s 缺少 menuRequiredAction 映射", key)
	}
}
