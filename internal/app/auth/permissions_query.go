package auth

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/guliangiam/guliangiam/internal/app/uow"
)

// UserPermissionSet represents the permission set for the current user
// that the frontend consumes to drive menu + button visibility.
type UserPermissionSet struct {
	Menus     []string `json:"menus"`
	Actions   []string `json:"actions"`
	Resources []string `json:"resources"`
}

// fullMenus — every sidebar route key, kept in sync with the leaf keys of
// web/src/features/navigation/menu.tsx (menuGroups). Menu visibility is UX
// (not security); the backend still enforces access via Casbin policies. For
// non-admin users the delivered menus are derived from the resolved action
// set (deriveMenus); only adminPermissionSet keeps the full list.
var fullMenus = []string{
	"/", "/shortcuts", "/apps", "/oidc-discovery",
	"/organizations", "/users", "/groups", "/invitations", "/applications",
	"/roles", "/permissions", "/models", "/adapters", "/enforcers", "/rebac", "/actions",
	"/providers", "/syncers", "/migration", "/webhooks", "/ldaps", "/servers", "/sites",
	"/audit/login-logs", "/audit/operation-logs", "/audit/error-logs",
	"/sessions", "/records", "/tokens", "/verifications", "/entries",
	"/certs", "/keys", "/radius-configs", "/radius-accountings", "/rules", "/risk", "/iga", "/idv", "/identity-pools",
	"/store", "/products", "/coupons", "/cart", "/orders", "/pricings", "/subscriptions", "/transactions", "/metering",
	"/ai/providers", "/ai/models", "/ai/aliases", "/ai/routes", "/ai/api-keys", "/ai/quotas", "/ai/prices",
	"/ai/usage", "/ai/usage/stats", "/ai/health", "/ai/agents", "/ai/mcp-servers", "/ai/mcp-servers/store",
	"/system-info", "/resources", "/account", "/theme-editor", "/forms", "/tickets", "/swagger", "/i18n/languages", "/notifications",
}

// menuRequiredAction maps each sidebar route key to the read action that must
// be present in the user's resolved action set for the menu to be visible.
// Values are the exact spellings resolvePermissionsFromCasbin emits: prefixes
// from resourceToActionKey (users:/ldaps:/rules:/actions:/rebac: stay plural;
// app:/record:/server:/ticket:/agent: etc. are the singular exceptions), plus
// the raw path-derived obj for resources absent from that map (audit:, idv:,
// verifications:, radius-accountings:, notifications:, migration:,
// aigateway-usage:, aigateway-health: — those only ever appear through
// explicit p-policies, never through wildcard expansion).
// Keys intentionally missing from the map ("/", "/account") are public
// self-service surfaces and always visible.
var menuRequiredAction = map[string]string{
	"/apps":                 "app:read",
	"/organizations":        "organizations:read",
	"/users":                "users:read",
	"/groups":               "group:read",
	"/invitations":          "invitation:read",
	"/applications":         "app:read",
	"/roles":                "role:read",
	"/permissions":          "permission:read",
	"/models":               "model:read",
	"/adapters":             "adapter:read",
	"/enforcers":            "enforcer:read",
	"/rebac":                "rebac:read",
	"/actions":              "actions:read",
	"/providers":            "provider:read",
	"/syncers":              "syncer:read",
	"/migration":            "migration:read",
	"/webhooks":             "webhook:read",
	"/ldaps":                "ldaps:read",
	"/servers":              "server:read",
	"/sites":                "site:read",
	"/audit/login-logs":     "audit:read",
	"/audit/operation-logs": "audit:read",
	"/audit/error-logs":     "audit:read",
	"/sessions":             "session:read",
	"/records":              "record:read",
	"/tokens":               "token:read",
	"/verifications":        "verifications:read",
	"/entries":              "entry:read",
	"/certs":                "cert:read",
	"/keys":                 "key:read",
	"/radius-configs":       "radius-config:read",
	"/radius-accountings":   "radius-accountings:read",
	"/rules":                "rules:read",
	"/risk":                 "risk:read",
	"/iga":                  "iga:read",
	"/idv":                  "idv:read",
	"/identity-pools":       "identity-pools:read",
	"/store":                "product:read",
	"/products":             "product:read",
	"/coupons":              "coupon:read",
	"/cart":                 "order:read",
	"/orders":               "order:read",
	"/pricings":             "pricing:read",
	"/subscriptions":        "subscription:read",
	"/transactions":         "transaction:read",
	"/metering":             "metering:read",
	"/ai/providers":         "aigateway-provider:read",
	"/ai/models":            "aigateway-model:read",
	"/ai/aliases":           "aigateway-alias:read",
	"/ai/routes":            "aigateway-route:read",
	"/ai/api-keys":          "aigateway-api-key:read",
	"/ai/quotas":            "aigateway-quota:read",
	"/ai/prices":            "aigateway-price:read",
	"/ai/usage":             "aigateway-usage:read",
	"/ai/usage/stats":       "aigateway-usage:read",
	"/ai/health":            "aigateway-health:read",
	"/ai/agents":            "agent:read",
	"/ai/mcp-servers":       "mcp-server:read",
	"/ai/mcp-servers/store": "mcp-server:read",
	"/resources":            "resource:read",
	"/forms":                "form:read",
	"/tickets":              "ticket:read",
	"/i18n/languages":       "i18n:read",
	"/notifications":        "notifications:read",
}

// adminOnlyMenus are sidebar keys with no authorization surface at all
// ("/system-info" is platform-admin data; shortcuts/theme-editor/swagger/
// oidc-discovery are console utilities without any Casbin-guarded read
// action). They are delivered only through adminPermissionSet and are never
// derived for role users — an un-authorized menu must stay hidden.
var adminOnlyMenus = map[string]struct{}{
	"/system-info":    {},
	"/shortcuts":      {},
	"/theme-editor":   {},
	"/swagger":        {},
	"/oidc-discovery": {},
}

// deriveMenus returns the sidebar keys the given action set authorizes, in
// fullMenus order. Unmapped keys ("/", "/account") are always kept,
// adminOnlyMenus are never derived. The result is non-nil.
func deriveMenus(actions []string) []string {
	granted := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		granted[action] = struct{}{}
	}
	menus := make([]string, 0, len(fullMenus))
	for _, key := range fullMenus {
		if _, restricted := adminOnlyMenus[key]; restricted {
			continue
		}
		required, mapped := menuRequiredAction[key]
		if !mapped {
			menus = append(menus, key)
			continue
		}
		if _, ok := granted[required]; ok {
			menus = append(menus, key)
		}
	}
	return menus
}

// resourceToActionKey maps Casbin resource slugs → frontend action prefixes.
// Casbin policy stores obj = "users", "applications", or "*"; we convert
// each to the "prefix" side of the frontend "prefix:verb" convention.
//
// Colon-scoped Casbin objects (AI Gateway: "aigateway:providers") are not
// listed here on purpose. frontendResourceAlias derives their hyphenated
// frontend slug at query time, so the raw colon key stays addressable and the
// wildcard expansion below keeps iterating only canonical resource slugs.
var resourceToActionKey = map[string]string{
	// ── IAM core ──
	// 前缀必须与路由声明 / permissions.yaml 的规范键一致（复数 `users:*`）。
	// 历史版本输出单数 `user:*`，前端曾用别名兼容；此处以路由声明为唯一规范。
	resourceUsers:         "users",
	resourceOrganizations: "organizations",
	resourceDomains:       "b2b",
	resourceApplications:  "app",
	resourceProviders:     "provider",
	resourceGroups:        "group",
	resourceRoles:         "role",
	resourcePermissions:   "permission",
	resourceSessions:      "session",
	resourceTokens:        "token",
	resourceRecords:       "record",
	resourceWebhooks:      "webhook",
	resourceCerts:         "cert",
	resourceKeys:          "key",
	resourceModels:        "model",
	resourceAdapters:      "adapter",
	resourceEnforcers:     "enforcer",
	resourceSyncers:       "syncer",
	resourceResources:     "resource",
	resourceInvitations:   "invitation",
	resourceForms:         "form",
	resourceEntries:       "entry",
	resourceMCPServers:    "mcp-server",
	resourcePolicies:      "policy",
	resourceGateway:       resourceGateway,
	// ── Commerce ──
	resourcePlans:         "plan",
	resourcePricings:      "pricing",
	resourceProducts:      "product",
	resourceOrders:        "order",
	resourcePayments:      "payment",
	resourceCoupons:       "coupon",
	resourceSubscriptions: "subscription",
	resourceTransactions:  "transaction",
	// ── Payments reconciliation ──
	// Path-derived resources keep their own slug as the frontend prefix.
	resourcePayRecon:          resourcePayRecon,
	resourcePayReconItems:     resourcePayReconItems,
	resourcePayChannelConfigs: resourcePayChannelConfigs,
	// ── AI Gateway ──
	resourceAIGatewayProviders: "aigateway-provider",
	resourceAIGatewayModels:    "aigateway-model",
	resourceAIGatewayAliases:   "aigateway-alias",
	resourceAIGatewayRoutes:    "aigateway-route",
	resourceAIGatewayAPIKeys:   "aigateway-api-key",
	resourceAIGatewayQuotas:    "aigateway-quota",
	resourceAIGatewayPrices:    "aigateway-price",
	// ── Security extensions ──
	resourceCaptchaProviders:      "captcha-provider",
	resourceNotificationProviders: "notification-provider",
	resourceFaceIDProviders:       "face-id-provider",
	resourceRadiusConfigs:         "radius-config",
	// ── Gateway servers/sites ──
	resourceServers: "server",
	resourceSites:   "site",
	// ── AI agents ──
	resourceAgents: "agent",
	// ── Support ──
	resourceTickets: "ticket",
	// ── Sync/Directory/Rule engines ──
	resourceLDAPs: resourceLDAPs,
	resourceRules: resourceRules,
	resourceIGA:   resourceIGA,
	// ── Governance surfaces ──
	// Casbin obj 与前端 action 前缀同名；缺少这四条时通配 p-策略（obj="*"）
	// 展开不会下发 actions/rebac/risk/metering，前端 PermissionGuard 会把
	// Actions/ReBAC/Risk/Metering 页面的写按钮全部隐藏。
	resourceActions:  resourceActions,
	resourceReBAC:    resourceReBAC,
	resourceRisk:     resourceRisk,
	resourceMetering: resourceMetering,
	// ── Workload identity ──
	resourceIdentityPools: resourceIdentityPools,
	// ── Localization ──
	resourceI18n: resourceI18n,
}

// frontendResourceAlias derives the frontend-facing resource slug and action
// prefix for a colon-scoped Casbin object. AI Gateway policies store
// "aigateway:providers"/"aigateway:api_keys" while the UI consumes the
// hyphenated plural resource ("aigateway-providers"/"aigateway-api-keys") and
// the singular action prefix ("aigateway-provider"/"aigateway-api-key").
//
// The alias is additive: callers keep emitting the raw object/action pair, so
// policies created under the old naming convention remain visible without
// re-granting. Objects without a colon are already canonical and return ok=false.
func frontendResourceAlias(obj string) (prefix, resource string, ok bool) {
	if !strings.Contains(obj, ":") {
		return "", "", false
	}
	resource = strings.ReplaceAll(obj, ":", "-")
	resource = strings.ReplaceAll(resource, "_", "-")
	prefix = resource
	if mapped, known := resourceToActionKey[resource]; known {
		prefix = mapped
	}
	return prefix, resource, true
}

// crudActionAlias normalizes an HTTP-method policy action to the CRUD
// vocabulary the frontend consumes. Runtime policies may store either form
// (the PEP tries the method first and the CRUD alias second); querying the
// Casbin policies must expose the CRUD key or explicit-policy roles lose
// buttons that the backend would in fact authorize.
//
// changed=false means the action is already CRUD or is not a recognized HTTP
// verb; callers then keep the raw key only.
func crudActionAlias(action string) (alias string, changed bool) {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "get", "head":
		return "read", true
	case "post", "put", "patch":
		return "write", true
	case "delete":
		return "delete", true
	default:
		return action, false
	}
}

// frontendPolicyKeys returns the frontend action keys and resource slugs for
// one Casbin p-policy row. The first pair is the legacy derivation and is
// emitted verbatim; the optional second pair is the normalized alias.
// Callers deduplicate because aliases can coincide with the raw pair.
func frontendPolicyKeys(obj, action string) (actions, resources []string) {
	prefix, known := resourceToActionKey[obj]
	if !known {
		prefix = obj
	}
	actions = append(actions, prefix+":"+action)
	resources = append(resources, obj)

	aliasPrefix, aliasResource, hasResourceAlias := frontendResourceAlias(obj)
	aliasAction, hasActionAlias := crudActionAlias(action)
	if !hasResourceAlias && !hasActionAlias {
		return actions, resources
	}
	if !hasResourceAlias {
		aliasPrefix, aliasResource = prefix, obj
	}
	if !hasActionAlias {
		aliasAction = action
	}
	actions = append(actions, aliasPrefix+":"+aliasAction)
	resources = append(resources, aliasResource)
	return actions, resources
}

// permissionCacheTTL is how long GetCurrentUserPermissions results are memoized
// per (orgSlug, username). Chosen to match the frontend usePermissions hook's
// 5-minute staleTime so the backend never recomputes a result the frontend
// would have re-fetched anyway.
const permissionCacheTTL = 5 * time.Minute

// cachedPermission wraps a UserPermissionSet with its expiry timestamp.
type cachedPermission struct {
	expiresAt time.Time
	set       *UserPermissionSet
}

// GetCurrentUserPermissions derives the frontend permission set from the user's
// Casbin role. Results are memoized per (orgSlug, username) for 5 minutes so
// repeated hits from the same browser session skip UOW + Casbin BFS entirely.
//
// Cache misses / DB errors degrade gracefully to an empty set — the UI hides
// create/edit/delete buttons rather than showing them to an unverified user.
func (s *Service) GetCurrentUserPermissions(ctx context.Context, username, orgSlug string) (*UserPermissionSet, error) {
	empty := &UserPermissionSet{}
	if username == "" || orgSlug == "" {
		return empty, nil
	}

	cacheKey := orgSlug + "|" + username

	// Fast path: valid cache hit.
	if v, ok := s.permissionCache.Load(cacheKey); ok {
		cached := v.(cachedPermission)
		if time.Now().Before(cached.expiresAt) {
			return cached.set, nil
		}
		// expired — fall through to recompute
		s.permissionCache.Delete(cacheKey)
	}

	var (
		isAdmin   bool
		orgID     string
		roleSlugs []string
	)

	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx uow.UnitOfWork) error {
		org, err := tx.Organizations().GetBySlug(ctx, orgSlug)
		if err != nil {
			return err
		}
		orgID = org.ID

		user, err := tx.Users().GetByUsername(ctx, orgID, username)
		if err != nil {
			return err
		}
		isAdmin = user.IsAdmin

		if !isAdmin {
			roleSlugs, err = tx.Users().ListRoleNames(ctx, user.ID)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		// Fail-closed: an unresolvable permission set must not grant anything.
		slog.WarnContext(ctx, "permission resolution transaction failed; returning empty set", "error", err)
		return empty, nil
	}

	var result *UserPermissionSet
	if isAdmin {
		result = adminPermissionSet()
	} else {
		actions, resources := s.resolvePermissionsFromCasbin(orgID, roleSlugs)
		if len(actions) == 0 {
			result = viewerPermissionSet()
		} else {
			result = &UserPermissionSet{
				Menus:     deriveMenus(actions),
				Actions:   actions,
				Resources: resources,
			}
		}
	}

	// Populate cache.
	s.permissionCache.Store(cacheKey, cachedPermission{
		expiresAt: time.Now().Add(permissionCacheTTL),
		set:       result,
	})

	return result, nil
}

// InvalidatePermissionCache removes a single (orgSlug, username) entry. Useful
// after role binding or Casbin policy changes for a specific user.
func (s *Service) InvalidatePermissionCache(orgSlug, username string) {
	if orgSlug == "" || username == "" {
		return
	}
	s.permissionCache.Delete(orgSlug + "|" + username)
}

// InvalidateAllPermissionCaches wipes every memoized entry. Use after bulk
// Casbin policy reloads or org-wide permission changes.
func (s *Service) InvalidateAllPermissionCaches() {
	s.permissionCache.Range(func(key, _ any) bool {
		s.permissionCache.Delete(key)
		return true
	})
}

func adminPermissionSet() *UserPermissionSet {
	actions := []string{
		// IAM core
		"users:read", "users:write", "users:delete", "users:password:reset",
		"organizations:read", "organizations:write", "organizations:delete",
		"b2b:read", "b2b:write",
		"app:read", "app:write", "app:delete",
		"role:read", "role:write", "role:delete",
		"permission:read", "permission:write", "permission:delete",
		"provider:read", "provider:write", "provider:delete",
		"cert:read", "cert:write", "cert:delete",
		"webhook:read", "webhook:write", "webhook:delete",
		"record:read", "record:write",
		"session:read", "session:write",
		"token:read", "token:write", "token:delete",
		"enforcer:read", "enforcer:write", "enforcer:delete",
		"policy:read", "policy:write", "policy:delete",
		"key:read", "key:write", "key:delete",
		// resourceSyncers maps to the "syncer" prefix. "sync:*" is kept as a
		// compatibility alias until the SyncersPage guard migrates (F3.2b).
		"syncer:read", "syncer:write", "syncer:delete",
		"sync:read", "sync:write", "sync:delete",
		"resource:read", "resource:write", "resource:delete",
		"group:read", "group:write", "group:delete",
		"invitation:read", "invitation:write", "invitation:delete",
		"mcp-server:read", "mcp-server:write", "mcp-server:delete",
		"form:read", "form:write",
		"entry:read", "entry:write",
		"adapter:read", "adapter:write", "adapter:delete",
		// Commerce
		"plan:read", "plan:write", "plan:delete",
		"pricing:read", "pricing:write", "pricing:delete",
		"product:read", "product:write", "product:delete",
		"order:read", "order:write", "order:delete",
		"payment:read", "payment:write", "payment:delete",
		"coupon:read", "coupon:write", "coupon:delete",
		"subscription:read", "subscription:write", "subscription:delete",
		"transaction:read", "transaction:write", "transaction:delete",
		// Payments reconciliation (POST-only run/resolve endpoints)
		"pay-recon:write",
		"pay-recon-items:write",
		"pay-channel-configs:read", "pay-channel-configs:write",
		// AI Gateway
		"aigateway-provider:read", "aigateway-provider:write", "aigateway-provider:delete",
		"aigateway-model:read", "aigateway-model:write", "aigateway-model:delete",
		"aigateway-alias:read", "aigateway-alias:write", "aigateway-alias:delete",
		"aigateway-route:read", "aigateway-route:write", "aigateway-route:delete",
		"aigateway-api-key:read", "aigateway-api-key:write", "aigateway-api-key:delete",
		"aigateway-quota:read", "aigateway-quota:write", "aigateway-quota:delete",
		"aigateway-price:read", "aigateway-price:write", "aigateway-price:delete",
		// Security extensions — DELETE 在每个 routes.go 中都声明为 write，
		// 因此这里不下发无路由支撑的 :delete。
		"captcha-provider:read", "captcha-provider:write",
		"notification-provider:read", "notification-provider:write",
		"face-id-provider:read", "face-id-provider:write",
		"radius-config:read", "radius-config:write", "radius-config:delete",
		// Gateway servers/sites
		"server:read", "server:write", "server:delete",
		"site:read", "site:write", "site:delete",
		// AI agents
		"agent:read", "agent:write", "agent:delete",
		// Support
		"ticket:read", "ticket:write", "ticket:delete",
		// Sync/Directory/Rule engines
		"ldaps:read", "ldaps:write", "ldaps:delete",
		"rules:read", "rules:write", "rules:delete",
		"iga:read", "iga:write", "iga:delete",
		// Governance surfaces (IGA/ReBAC/Actions/Risk/Metering). Without these the
		// built-in administrator sees no write controls on those pages even though
		// Casbin grants the wildcard policy.
		"actions:read", "actions:write",
		"rebac:read", "rebac:write", "rebac:check",
		"risk:read", "risk:write",
		"metering:read", "metering:write",
		// Workload identity
		"identity-pools:read", "identity-pools:write",
		// Localization
		"i18n:read", "i18n:write",
		// System
		"dashboard:read",
	}
	resources := []string{
		// IAM core
		resourceUsers, resourceOrganizations, casbinResourceDomains, resourceApplications, resourceProviders,
		resourceRoles, resourcePermissions, resourceModels, resourceAdapters, resourceEnforcers,
		resourceCerts, resourceWebhooks, resourceSyncers, resourceResources,
		resourceRecords, resourceSessions, resourceTokens, resourceGroups, resourceInvitations,
		resourceKeys, resourcePolicies, resourceMCPServers, resourceGateway, resourceForms, resourceEntries,
		resourceIdentityPools,
		// Commerce
		resourcePlans, resourcePricings, resourceProducts, resourceOrders,
		resourcePayments, resourceCoupons, resourceSubscriptions, resourceTransactions,
		// Payments reconciliation
		resourcePayRecon, resourcePayReconItems, resourcePayChannelConfigs,
		// AI Gateway
		resourceAIGatewayProviders, resourceAIGatewayModels, resourceAIGatewayAliases,
		resourceAIGatewayRoutes, resourceAIGatewayAPIKeys, resourceAIGatewayQuotas, resourceAIGatewayPrices,
		// Security extensions
		resourceCaptchaProviders, resourceNotificationProviders, resourceFaceIDProviders, resourceRadiusConfigs,
		// Gateway servers/sites
		resourceServers, resourceSites,
		// AI agents
		resourceAgents,
		// Support
		resourceTickets,
		// Sync/Directory/Rule engines
		resourceLDAPs,
		resourceRules,
		resourceIGA,
		// Governance surfaces
		resourceActions, resourceReBAC, resourceRisk, resourceMetering,
		// Localization
		resourceI18n,
	}
	return &UserPermissionSet{Menus: fullMenus, Actions: actions, Resources: resources}
}

// viewerPermissionSet is the converged fallback for users bound to no role.
// Requirement (batch 9a): un-authorized menus must be fully hidden, so a
// role-less user gets only the self-service surfaces ("/" home and
// "/account"). The historical full-read fallback is removed on purpose:
// list endpoints such as tickets filter by organization, not creator, so
// granting read actions by default would leak other members' records.
func viewerPermissionSet() *UserPermissionSet {
	return &UserPermissionSet{
		Menus:     []string{"/", "/account"},
		Actions:   []string{},
		Resources: []string{},
	}
}

// resolvePermissionsFromCasbin resolves the frontend-friendly action/resource
// set for every role the user is bound to.
//
// When CasbinPolicyManager is wired (bootstrap injects it), it queries real
// p-policies from the DB-backed Casbin enforcer (role inheritance expanded),
// converts each Casbin (resource, action) row through resourceToActionKey,
// and merges across roles.
//
// When CasbinPolicyManager is nil, falls back to hard-coded built-in role
// semantics (editor=read+write, operator=read+write+delete, viewer=read) so
// unit tests that only construct Service with UOW keep working.
func (s *Service) resolvePermissionsFromCasbin(orgID string, roleSlugs []string) (actions, resources []string) {
	if len(roleSlugs) == 0 {
		return nil, nil
	}

	if s.CasbinPolicyManager != nil {
		return s.resolveFromRealCasbin(orgID, roleSlugs)
	}
	return s.resolveFromHardcodedSemantics(orgID, roleSlugs)
}

// resolveFromRealCasbin queries p-policies from the DB-backed Casbin enforcer
// and converts them to frontend action/resource sets.
//
// Casbin p-policy row: [sub, dom, obj, act]
//
//	obj = Casbin resource ("users", "aigateway:providers", ...) or "*" meaning all.
//	act = Casbin action ("read", "write", "delete") or an HTTP method verb.
//
// Each row keeps its legacy key and also contributes a normalized frontend
// alias (see frontendPolicyKeys), so old grants never stop mapping to a key.
func (s *Service) resolveFromRealCasbin(orgID string, roleSlugs []string) (actionsList, resourcesList []string) {
	actions := make(map[string]struct{})
	resources := make(map[string]struct{})

	for _, roleSlug := range roleSlugs {
		// Try the raw role name first (that's what casbin_rules.v0 stores).
		policies, err := s.CasbinPolicyManager.GetOrgSubjectPolicies(orgID, roleSlug)
		if err != nil || len(policies) == 0 {
			// Fall back to normalized form in case a deployment stored it that way.
			actor := normalizeActor(roleSlug, orgID)
			if actor != roleSlug {
				policies, err = s.CasbinPolicyManager.GetOrgSubjectPolicies(orgID, actor)
			}
			if err != nil || len(policies) == 0 {
				continue
			}
		}

		for _, p := range policies {
			if len(p) < 4 {
				continue
			}
			obj := p[2]
			act := p[3]

			if obj == "*" {
				// Wildcard → every known resource gets this action verb, plus
				// the CRUD alias when the policy stores an HTTP method.
				// Colon-scoped aliases are omitted here because the canonical
				// map keys already produce the frontend-facing prefixes.
				//
				// act=="*" 必须展开为 read/write/delete：前端 hasAction /
				// PermissionGuard 只做精确匹配（含别名），不会把 "prefix:*"
				// 理解为通配，否则持通配策略的租户管理员会拿到几乎空白的菜单。
				verbs := []string{act}
				if act == "*" {
					verbs = []string{"read", "write", "delete"}
				}
				for resource, key := range resourceToActionKey {
					for _, verb := range verbs {
						actions[key+":"+verb] = struct{}{}
						if alias, changed := crudActionAlias(verb); changed {
							actions[key+":"+alias] = struct{}{}
						}
					}
					resources[resource] = struct{}{}
				}
				continue
			}

			// 单个资源上的通配动作（obj 指定、act="*"）同样展开为 CRUD 三动作。
			if act == "*" {
				for _, verb := range []string{"read", "write", "delete"} {
					actionKeys, resourceKeys := frontendPolicyKeys(obj, verb)
					for _, key := range actionKeys {
						actions[key] = struct{}{}
					}
					for _, key := range resourceKeys {
						resources[key] = struct{}{}
					}
				}
				continue
			}

			// Legacy raw key first, then the additive normalized alias
			// (colon→hyphen resource / HTTP verb→CRUD action). Unknown custom
			// resources fall back to their raw obj as the action prefix.
			actionKeys, resourceKeys := frontendPolicyKeys(obj, act)
			for _, key := range actionKeys {
				actions[key] = struct{}{}
			}
			for _, key := range resourceKeys {
				resources[key] = struct{}{}
			}
		}
	}

	return mapToSortedSlices(actions, resources)
}

// resolveFromHardcodedSemantics is the legacy fallback used when CasbinManager
// is nil (unit tests without bootstrap wiring).
func (s *Service) resolveFromHardcodedSemantics(orgID string, roleSlugs []string) (actionList, resourceList []string) {
	actions := make(map[string]struct{})
	resources := make(map[string]struct{})

	for _, slug := range roleSlugs {
		actor := normalizeActor(slug, orgID)
		switch actor {
		case "editor":
			for resource, key := range resourceToActionKey {
				actions[key+":read"] = struct{}{}
				actions[key+":write"] = struct{}{}
				resources[resource] = struct{}{}
			}
		case "operator":
			for resource, key := range resourceToActionKey {
				actions[key+":read"] = struct{}{}
				actions[key+":write"] = struct{}{}
				switch resource {
				case resourceRecords, resourceSessions:
					// audit/log → read only
				default:
					actions[key+":delete"] = struct{}{}
				}
				resources[resource] = struct{}{}
			}
		case "viewer":
			for resource, key := range resourceToActionKey {
				actions[key+":read"] = struct{}{}
				resources[resource] = struct{}{}
			}
		default:
			for resource, key := range resourceToActionKey {
				actions[key+":read"] = struct{}{}
				resources[resource] = struct{}{}
			}
		}
	}

	return mapToSortedSlices(actions, resources)
}

// mapToSortedSlices converts deduped maps to sorted slices for deterministic output.
func mapToSortedSlices(actions, resources map[string]struct{}) (actionsOut, resourcesOut []string) {
	actionList := make([]string, 0, len(actions))
	for k := range actions {
		actionList = append(actionList, k)
	}
	resourceList := make([]string, 0, len(resources))
	for k := range resources {
		resourceList = append(resourceList, k)
	}
	sort.Strings(actionList)
	sort.Strings(resourceList)
	return actionList, resourceList
}

// normalizeActor strips org prefixes so that e.g.
// "built-in-editor", "<orgID>-editor", "editor" all collapse to "editor".
func normalizeActor(slug, orgID string) string {
	actor := strings.TrimSpace(slug)
	actor = strings.TrimPrefix(actor, orgID+"-")
	actor = strings.TrimPrefix(actor, "built-in-")
	return strings.ToLower(actor)
}

// casbinResourceDomains 是 B2B 域名认领在 Casbin p-策略里的资源名。
const casbinResourceDomains = resourceDomains
