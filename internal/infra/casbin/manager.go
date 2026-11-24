package casbin

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	domainpermissions "github.com/guliangiam/guliangiam/internal/domain/permissions"
	domainroles "github.com/guliangiam/guliangiam/internal/domain/roles"
)

// Manager manages Casbin enforcer instances keyed by enforcer ID.
// Each enforcer corresponds to a domain enforcer record and holds
// an in-memory Casbin enforcer backed by the dialect-neutral PolicyStore
// (PostgreSQL: pgx Adapter; MySQL: internal/infra/mysql/casbin.Store).
type Manager struct {
	mu           sync.RWMutex
	enforcers    map[string]*casbin.Enforcer
	orgEnforcers map[string]*casbin.Enforcer
	adapter      PolicyStore
	cache        *EnforceCache
	jitEnforce   func(context.Context, string, string, ...interface{}) (bool, error)
}

func NewManager(adapter PolicyStore) *Manager {
	mgr := &Manager{
		enforcers:    make(map[string]*casbin.Enforcer),
		orgEnforcers: make(map[string]*casbin.Enforcer),
		adapter:      adapter,
		cache:        NewEnforceCache(0), // 0 = use default maxSize (1M)
	}
	mgr.jitEnforce = adapter.EnforceJIT
	return mgr
}

func (mgr *Manager) SetCache(c *EnforceCache) {
	mgr.cache = c
}

func (mgr *Manager) EnforceCache() *EnforceCache {
	return mgr.cache
}

// enforcerCacheKey scopes a cached enforcer to both the enforcer record and the
// Casbin model text.
//
// One enforcer backs several principals, and each principal tier evaluates a
// different model (platform_admin uses an org-wildcard matcher). Keying by the
// enforcer UUID alone would hand the first caller's model to every later
// caller, so a user request reusing an enforcer cached by a platform admin
// would be evaluated against the wildcard model and its organization-scoped
// `g` rules would never match.
func enforcerCacheKey(enforcerID, modelText string) string {
	return enforcerID + "\x00" + modelText
}

// GetOrCreateEnforcer returns the cached enforcer for the given ID and model,
// or creates a new one by loading the model text and adapter policies from the
// database.
func (mgr *Manager) GetOrCreateEnforcer(enforcerID, modelText string) (*casbin.Enforcer, error) {
	key := enforcerCacheKey(enforcerID, modelText)

	mgr.mu.RLock()
	e, ok := mgr.enforcers[key]
	mgr.mu.RUnlock()

	if ok {
		return e, nil
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	// Double-check after acquiring write lock.
	if e, ok = mgr.enforcers[key]; ok {
		return e, nil
	}

	m, err := model.NewModelFromString(modelText)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse casbin model for enforcer %s", enforcerID)
	}

	adapter := mgr.adapter.Scoped(enforcerID)
	e, err = casbin.NewEnforcer(m, adapter)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "create casbin enforcer %s", enforcerID)
	}

	mgr.enforcers[key] = e
	return e, nil
}

// Enforce performs a single policy check against the specified enforcer.
// Enforce performs a single policy check with OTel span tracing.
// The ctx parameter is used for trace parent inheritance — Casbin's Enforce()
// itself does not take a context (v3 signature is variadic-only), so we wrap
// it with an "authz.casbin.enforce" span and extract subject/resource/action
// from rvals[0..2] for attributes.
func (mgr *Manager) Enforce(ctx context.Context, enforcerID, modelText string, rvals ...interface{}) (bool, error) {
	return mgr.enforce(ctx, enforcerID, modelText, "", rvals...)
}

// EnforcePrivileged evaluates permanent policies first, then limits any JIT
// fallback to the lifecycle bound to the repository-backed privileged session.
func (mgr *Manager) EnforcePrivileged(ctx context.Context, enforcerID, modelText, lifecycleID string, rvals ...interface{}) (bool, error) {
	return mgr.enforce(ctx, enforcerID, modelText, lifecycleID, rvals...)
}

func (mgr *Manager) enforce(ctx context.Context, enforcerID, modelText, lifecycleID string, rvals ...interface{}) (bool, error) {
	start := time.Now()
	tracer := otel.GetTracerProvider().Tracer("casbin")
	ctx, span := tracer.Start(ctx, "authz.casbin.enforce")
	defer span.End()

	// Extract common Casbin rvals[subject, object, action, ...] for attributes.
	if len(rvals) >= 1 {
		if s, ok := rvals[0].(string); ok {
			span.SetAttributes(attribute.String("casbin.subject", s))
		}
	}
	if len(rvals) >= 2 {
		if o, ok := rvals[1].(string); ok {
			span.SetAttributes(attribute.String("casbin.resource", o))
		}
	}
	if len(rvals) >= 3 {
		if a, ok := rvals[2].(string); ok {
			span.SetAttributes(attribute.String("casbin.action", a))
		}
	}
	span.SetAttributes(attribute.String("casbin.enforcer_id", enforcerID))

	e, err := mgr.GetOrCreateEnforcer(enforcerID, modelText)
	if err != nil {
		span.SetStatus(codes.Error, "get_or_create_enforcer_failed: "+err.Error())
		enforceDuration.WithLabelValues(enforcerID, modelMetricLabel(modelText), "error", "").Observe(time.Since(start).Seconds())
		enforceTotal.WithLabelValues(enforcerID, "error").Inc()
		return false, err
	}
	allowed, err := e.Enforce(rvals...)
	if err == nil && !allowed && strings.TrimSpace(lifecycleID) != "" {
		allowed, err = mgr.jitEnforce(ctx, enforcerID, lifecycleID, rvals...)
	}
	if err != nil {
		span.SetStatus(codes.Error, "casbin_enforce_failed: "+err.Error())
		enforceDuration.WithLabelValues(enforcerID, modelMetricLabel(modelText), "error", "").Observe(time.Since(start).Seconds())
		enforceTotal.WithLabelValues(enforcerID, "error").Inc()
		return false, err
	}
	// OTel best practice: always set status to Ok on success, so downstream
	// consumers can distinguish "unset" (did't run) from "ok" (ran fine).
	span.SetStatus(codes.Ok, "")
	result := "deny"
	if allowed {
		result = "allow"
	}
	enforceDuration.WithLabelValues(enforcerID, modelMetricLabel(modelText), result, "").Observe(time.Since(start).Seconds())
	enforceTotal.WithLabelValues(enforcerID, result).Inc()
	return allowed, nil
}

// BatchEnforce performs multiple policy checks against the specified enforcer.
// BatchEnforce performs multiple policy checks with OTel span tracing.
func (mgr *Manager) BatchEnforce(ctx context.Context, enforcerID, modelText string, rvals [][]interface{}) ([]bool, error) {
	tracer := otel.GetTracerProvider().Tracer("casbin")
	_, span := tracer.Start(ctx, "authz.casbin.batch_enforce")
	defer span.End()
	span.SetAttributes(
		attribute.String("casbin.enforcer_id", enforcerID),
		attribute.Int("casbin.batch_size", len(rvals)),
	)

	e, err := mgr.GetOrCreateEnforcer(enforcerID, modelText)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	results, err := e.BatchEnforce(rvals)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetStatus(codes.Ok, "")
	return results, nil
}

func (mgr *Manager) AddPolicy(enforcerID, modelText string, params []string) error {
	e, err := mgr.GetOrCreateEnforcer(enforcerID, modelText)
	if err != nil {
		return err
	}
	_, err = e.AddPolicy(params)
	if err == nil && mgr.cache != nil {
		mgr.cache.Invalidate()
	}
	return err
}

func (mgr *Manager) RemovePolicy(enforcerID, modelText string, params []string) error {
	e, err := mgr.GetOrCreateEnforcer(enforcerID, modelText)
	if err != nil {
		return err
	}
	_, err = e.RemovePolicy(params)
	if err == nil && mgr.cache != nil {
		mgr.cache.Invalidate()
	}
	return err
}

// GetPolicies returns all policies in the enforcer.
func (mgr *Manager) GetPolicies(enforcerID, modelText string) ([][]string, error) {
	e, err := mgr.GetOrCreateEnforcer(enforcerID, modelText)
	if err != nil {
		return nil, err
	}
	return e.GetPolicy()
}

// GetFilteredPolicies returns filtered policies in the enforcer.
func (mgr *Manager) GetFilteredPolicies(enforcerID, modelText string, fieldIndex int, fieldValues ...string) ([][]string, error) {
	e, err := mgr.GetOrCreateEnforcer(enforcerID, modelText)
	if err != nil {
		return nil, err
	}
	return e.GetFilteredPolicy(fieldIndex, fieldValues...)
}

func (mgr *Manager) UpdatePolicy(enforcerID, modelText string, oldPolicy, newPolicy []string) error {
	e, err := mgr.GetOrCreateEnforcer(enforcerID, modelText)
	if err != nil {
		return err
	}
	_, err = e.UpdatePolicy(oldPolicy, newPolicy)
	if err == nil && mgr.cache != nil {
		mgr.cache.Invalidate()
	}
	return err
}

func (mgr *Manager) ReloadPolicy(enforcerID, modelText string) error {
	e, err := mgr.GetOrCreateEnforcer(enforcerID, modelText)
	if err != nil {
		return err
	}
	err = e.LoadPolicy()
	if err == nil && mgr.cache != nil {
		mgr.cache.Invalidate()
	}
	return err
}

// GetAllSubjects returns all subjects in the policy.
func (mgr *Manager) GetAllSubjects(enforcerID, modelText string) ([]string, error) {
	e, err := mgr.GetOrCreateEnforcer(enforcerID, modelText)
	if err != nil {
		return nil, err
	}
	return e.GetAllSubjects()
}

// GetAllObjects returns all objects in the policy.
func (mgr *Manager) GetAllObjects(enforcerID, modelText string) ([]string, error) {
	e, err := mgr.GetOrCreateEnforcer(enforcerID, modelText)
	if err != nil {
		return nil, err
	}
	return e.GetAllObjects()
}

// GetAllActions returns all actions in the policy.
func (mgr *Manager) GetAllActions(enforcerID, modelText string) ([]string, error) {
	e, err := mgr.GetOrCreateEnforcer(enforcerID, modelText)
	if err != nil {
		return nil, err
	}
	return e.GetAllActions()
}

// GetAllRoles returns all roles in the policy.
func (mgr *Manager) GetAllRoles(enforcerID, modelText string) ([]string, error) {
	e, err := mgr.GetOrCreateEnforcer(enforcerID, modelText)
	if err != nil {
		return nil, err
	}
	return e.GetAllRoles()
}

// AddRoleForUser adds a role inheritance rule (grouping policy) to the specified enforcer.
// For example, AddRoleForUser("admin", "editor") means admin inherits editor's permissions.
func (mgr *Manager) AddRoleForUser(enforcerID, modelText, user, role string) error {
	e, err := mgr.GetOrCreateEnforcer(enforcerID, modelText)
	if err != nil {
		return err
	}
	added, err := e.AddRoleForUser(user, role)
	if err != nil {
		return err
	}
	// 与 Add/Remove/Update/Reload 对称：角色继承（g 规则）变更后必须失效
	// EnforceCache，否则缓存窗口内仍按旧继承关系裁决。
	if added && mgr.cache != nil {
		mgr.cache.Invalidate()
	}
	return nil
}

// GetEnforcer returns the org-scoped enforcer for the given organization ID.
// It uses a cache keyed by orgID. If not cached, it creates a new enforcer
// with a filtered adapter that only loads policies for that orgID.
// The orgID is used as the enforcer ID in the adapter, which scopes the
// casbin_rules query to that organization's policies.
func (mgr *Manager) GetEnforcer(orgID string) (*casbin.Enforcer, error) {
	mgr.mu.RLock()
	e, ok := mgr.orgEnforcers[orgID]
	mgr.mu.RUnlock()

	if ok {
		return e, nil
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	// Double-check after acquiring write lock.
	if e, ok = mgr.orgEnforcers[orgID]; ok {
		return e, nil
	}

	// Use the default RBAC model with domains for org-level enforcement.
	// This model supports: p = sub, dom, obj, act and g = _, _.
	modelText := `[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = (r.sub == p.sub || g(r.sub, p.sub)) && r.dom == p.dom && (p.obj == "*" || r.obj == p.obj) && (p.act == "*" || r.act == p.act)
`
	m, err := model.NewModelFromString(modelText)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "parse casbin model for org %s", orgID)
	}

	// 策略表的外键要求 enforcer_id 是 enforcers.id，因此先把组织解析成该组织
	// 启用的 enforcer 记录 ID，再让 adapter 只加载/写入这组策略。
	enforcerID, err := mgr.adapter.ResolveEnforcerID(context.Background(), orgID)
	if err != nil {
		return nil, err
	}
	adapter := mgr.adapter.Scoped(enforcerID)
	e, err = casbin.NewEnforcer(m, adapter)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "create casbin enforcer for org %s", orgID)
	}

	mgr.orgEnforcers[orgID] = e
	return e, nil
}

// InvalidateOrg removes the cached org-scoped enforcer, forcing recreation on next access.
func (mgr *Manager) InvalidateOrg(orgID string) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	delete(mgr.orgEnforcers, orgID)
}

// Invalidate removes every cached model variant of the given enforcer, forcing
// recreation on next access.
func (mgr *Manager) Invalidate(enforcerID string) {
	prefix := enforcerID + "\x00"
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	for key := range mgr.enforcers {
		if strings.HasPrefix(key, prefix) {
			delete(mgr.enforcers, key)
		}
	}
}

// InvalidateAll removes all cached enforcers, forcing recreation on next access.
func (mgr *Manager) InvalidateAll() {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	for id := range mgr.enforcers {
		delete(mgr.enforcers, id)
	}
}

var _ domainroles.PolicyManager = (*Manager)(nil)

func (mgr *Manager) SyncRole(ctx context.Context, orgID string, role *domainroles.Role) error {
	if mgr == nil || role == nil || !role.IsEnabled {
		return nil
	}
	e, err := mgr.GetEnforcer(orgID)
	if err != nil {
		return err
	}
	// 写入侧守卫与 ReplaceUserGroupings 一致（INC-BE-04 残留）：空 subject/
	// role 不得落库，否则加载侧会看到空列 g 行，补位语义只能兜底不能授权。
	for _, userID := range role.Users {
		if strings.TrimSpace(userID) == "" {
			continue
		}
		if _, err := e.AddRoleForUser(userID, role.ID); err != nil {
			return err
		}
	}
	for _, parentRoleID := range role.RoleIDs {
		if strings.TrimSpace(parentRoleID) == "" {
			continue
		}
		if _, err := e.AddRoleForUser(role.ID, parentRoleID); err != nil {
			return err
		}
	}

	// g 行已写入 casbin_rules；请求时 enforcer 与判定缓存可能仍持旧策略，
	// 与 SyncUserRoles 一致地失效，下一次 Enforce 重新加载。
	enforcerID, err := mgr.ResolveEnforcerUUID(ctx, orgID)
	if err != nil {
		return err
	}
	mgr.Invalidate(enforcerID)
	mgr.InvalidateOrg(orgID)
	if mgr.cache != nil {
		mgr.cache.Invalidate()
	}
	return nil
}

// SyncUserRoles rewrites a user's Casbin role-inheritance rules so that the
// request-time model (`g(user, role, org)`) reflects the database bindings.
//
// Role bindings live in `user_role_bindings`; authorization does not consult
// that table, so the grouping rows must be written explicitly. The method
// replaces all rows for the user (grant + revoke in one transaction) and drops
// the cached enforcers so the next decision reloads the policy table.
func (mgr *Manager) SyncUserRoles(ctx context.Context, orgID, orgSlug, username string, roleNames []string) error {
	if mgr == nil || strings.TrimSpace(orgID) == "" || strings.TrimSpace(orgSlug) == "" || strings.TrimSpace(username) == "" {
		return nil
	}

	enforcerID, err := mgr.ResolveEnforcerUUID(ctx, orgID)
	if err != nil {
		return err
	}
	if err := mgr.adapter.Scoped(enforcerID).ReplaceUserGroupings(ctx, username, orgSlug, roleNames); err != nil {
		return err
	}

	// Grouping rows changed underneath the in-memory enforcers; drop them so the
	// next Enforce call reloads casbin_rules.
	mgr.Invalidate(enforcerID)
	mgr.InvalidateOrg(orgID)
	if mgr.cache != nil {
		mgr.cache.Invalidate()
	}
	return nil
}

func (mgr *Manager) RemoveRole(ctx context.Context, orgID, roleID string) error {
	if mgr == nil {
		return nil
	}
	e, err := mgr.GetEnforcer(orgID)
	if err != nil {
		return err
	}
	if _, err := e.RemoveFilteredGroupingPolicy(0, roleID); err != nil {
		return err
	}
	if _, err := e.RemoveFilteredGroupingPolicy(1, roleID); err != nil {
		return err
	}
	return nil
}

var _ domainpermissions.CasbinPolicyManager = (*Manager)(nil)

func (mgr *Manager) RemoveFilteredPolicy(enforcerID, modelText string, fieldIndex int, fieldValues ...string) error {
	e, err := mgr.GetOrCreateEnforcer(enforcerID, modelText)
	if err != nil {
		return err
	}
	_, err = e.RemoveFilteredPolicy(fieldIndex, fieldValues...)
	if err == nil && mgr.cache != nil {
		mgr.cache.Invalidate()
	}
	return err
}

func (mgr *Manager) GetOrgEnforcerPolicies(orgID string) ([][]string, error) {
	e, err := mgr.GetEnforcer(orgID)
	if err != nil {
		return nil, err
	}
	return e.GetPolicy()
}

func (mgr *Manager) AddOrgPolicy(orgID string, params []string) error {
	e, err := mgr.GetEnforcer(orgID)
	if err != nil {
		return err
	}
	_, err = e.AddPolicy(params)
	if err == nil && mgr.cache != nil {
		mgr.cache.Invalidate()
	}
	return err
}

func (mgr *Manager) RemoveOrgFilteredPolicy(orgID string, fieldIndex int, fieldValues ...string) error {
	e, err := mgr.GetEnforcer(orgID)
	if err != nil {
		return err
	}
	_, err = e.RemoveFilteredPolicy(fieldIndex, fieldValues...)
	if err == nil && mgr.cache != nil {
		mgr.cache.Invalidate()
	}
	return err
}

func (mgr *Manager) AddOrgRoleForUser(orgID, user, role string) error {
	e, err := mgr.GetEnforcer(orgID)
	if err != nil {
		return err
	}
	_, err = e.AddRoleForUser(user, role)
	if err == nil && mgr.cache != nil {
		mgr.cache.Invalidate()
	}
	return err
}

func (mgr *Manager) RemoveOrgFilteredGroupingPolicy(orgID string, fieldIndex int, fieldValues ...string) error {
	e, err := mgr.GetEnforcer(orgID)
	if err != nil {
		return err
	}
	_, err = e.RemoveFilteredGroupingPolicy(fieldIndex, fieldValues...)
	if err == nil && mgr.cache != nil {
		mgr.cache.Invalidate()
	}
	return err
}

// ResolveEnforcerUUID returns the UUID of the first enabled enforcer for the given
// organization UUID. casbin_rules.enforcer_id 有 FK → enforcers(id)，调用方必须传真实
// enforcer UUID（"default" 之类会触发 UUID 解析失败）。
//
// 解析细节下沉到 PolicyStore 实现：PostgreSQL 需要在同一连接上注入 super_admin GUC
// 绕过 RLS，MySQL 无 RLS 直接查询。
func (mgr *Manager) ResolveEnforcerUUID(ctx context.Context, orgUUID string) (string, error) {
	if mgr == nil || mgr.adapter == nil {
		return "", apperrors.NewUnavailable("casbin policy store is not configured")
	}
	return mgr.adapter.ResolveEnforcerUUID(ctx, orgUUID)
}

// GetOrgSubjectPolicies returns every p-policy row that applies to the given
// subject after expanding role inheritance through g-policies.
//
// The Casbin model used by GetEnforcer is:
//
//	p = sub, dom, obj, act    — authorization rules
//	g = child, parent, dom    — role inheritance
//
// BFS from `subject` follows all g-edges up the hierarchy (cycle-safe, max
// depth 20), then returns the union of p-rules whose sub equals `subject` or
// any inherited role. The returned rows are [sub, dom, obj, act].
func (mgr *Manager) GetOrgSubjectPolicies(orgID, subject string) ([][]string, error) {
	e, err := mgr.GetEnforcer(orgID)
	if err != nil {
		return nil, err
	}

	policies, err := e.GetPolicy()
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "load p-policies for org %s", orgID)
	}

	groupings, err := e.GetGroupingPolicy()
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "load g-policies for org %s", orgID)
	}

	// Build adjacency: child → [parent, ...]  per domain.
	parentsOf := buildParentMap(groupings)

	// BFS collect all subjects (self + inherited roles).
	visited := map[string]bool{subject: true}
	queue := []string{subject}
	for depth := 0; depth < 20 && len(queue) > 0; depth++ {
		cur := queue[0]
		queue = queue[1:]
		for _, parent := range parentsOf[cur] {
			if !visited[parent] {
				visited[parent] = true
				queue = append(queue, parent)
			}
		}
	}

	// Collect p-policies whose sub is in the visited set.
	// Dedup on (sub, dom, obj, act).
	seen := make(map[string]struct{})
	var result [][]string
	for _, p := range policies {
		if len(p) < 4 {
			continue
		}
		sub := p[0]
		if !visited[sub] {
			continue
		}
		key := p[0] + "|" + p[1] + "|" + p[2] + "|" + p[3]
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		// Keep only the 4 canonical fields [sub, dom, obj, act].
		row := make([]string, 4)
		copy(row, p[:4])
		result = append(result, row)
	}

	return result, nil
}

// buildParentMap converts g-policies [child, parent, ...] into child→parents.
func buildParentMap(groupings [][]string) map[string][]string {
	m := make(map[string][]string)
	for _, g := range groupings {
		if len(g) < 2 {
			continue
		}
		child := g[0]
		parent := g[1]
		m[child] = append(m[child], parent)
	}
	return m
}
