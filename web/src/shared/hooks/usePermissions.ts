import { useQuery } from "@tanstack/react-query";
import { iamGet } from "@/shared/api/iam/client";
import { queryKeys } from "@/shared/api/query-keys";
import { useSessionStore } from "@/features/session/session-store";

import { AUTH_PERMISSIONS } from "@/shared/api/paths";

type UserPermissionSet = {
  menus: string[];
  actions: string[];
  resources: string[];
};

/**
 * Full admin permission set — hard-coded frontend copy of the backend
 * `adminPermissionSet()` in internal/app/auth/permissions_query.go.
 * Used as a safe fallback when /api/v1/auth/permissions fails at runtime
 * (network blip, CORS, backend restart) but the local session still says
 * the user is an admin. Non-admin callers never see this.
 * Kept in sync with contracts/registries/permissions.yaml by
 * usePermissions.mirror.test.ts.
 */
export const ADMIN_FULL_PERMISSIONS: UserPermissionSet = {
  menus: [
    "/",
    "/shortcuts",
    "/apps",
    "/oidc-discovery",
    "/organizations",
    "/users",
    "/groups",
    "/invitations",
    "/applications",
    "/roles",
    "/permissions",
    "/models",
    "/adapters",
    "/enforcers",
    "/rebac",
    "/actions",
    "/providers",
    "/syncers",
    "/migration",
    "/webhooks",
    "/ldaps",
    "/servers",
    "/sites",
    "/audit/login-logs",
    "/audit/operation-logs",
    "/audit/error-logs",
    "/sessions",
    "/records",
    "/tokens",
    "/verifications",
    "/entries",
    "/certs",
    "/keys",
    "/radius-configs",
    "/radius-accountings",
    "/rules",
    "/risk",
    "/iga",
    "/idv",
    "/identity-pools",
    "/store",
    "/products",
    "/coupons",
    "/cart",
    "/orders",
    "/pricings",
    "/subscriptions",
    "/transactions",
    "/metering",
    "/ai/providers",
    "/ai/models",
    "/ai/aliases",
    "/ai/routes",
    "/ai/api-keys",
    "/ai/quotas",
    "/ai/prices",
    "/ai/usage",
    "/ai/usage/stats",
    "/ai/health",
    "/ai/agents",
    "/ai/mcp-servers",
    "/ai/mcp-servers/store",
    "/system-info",
    "/resources",
    "/account",
    "/theme-editor",
    "/forms",
    "/tickets",
    "/swagger",
    "/i18n/languages",
    "/notifications",
  ],
  actions: [
    "user:read",
    "user:write",
    "user:delete",
    "users:read",
    "users:write",
    "users:delete",
    "users:password:reset",
    "organizations:read",
    "organizations:write",
    "organizations:delete",
    "b2b:read",
    "b2b:write",
    "app:read",
    "app:write",
    "app:delete",
    "role:read",
    "role:write",
    "permission:read",
    "permission:write",
    "permission:enforce",
    "provider:read",
    "provider:write",
    "provider:delete",
    "cert:read",
    "cert:write",
    "cert:delete",
    "webhook:read",
    "webhook:write",
    "webhook:delete",
    "record:read",
    "record:write",
    "session:read",
    "session:write",
    "token:read",
    "token:write",
    "token:delete",
    "enforcer:read",
    "enforcer:write",
    "enforcer:delete",
    "policy:read",
    "policy:write",
    "policy:delete",
    "key:read",
    "key:write",
    "key:delete",
    "syncer:read",
    "syncer:write",
    "syncer:delete",
    // Legacy aliases kept until the SyncersPage guard migrates (F3.2b).
    "sync:read",
    "sync:write",
    "sync:delete",
    "resource:read",
    "resource:write",
    "resource:delete",
    "group:read",
    "group:write",
    "group:delete",
    "invitation:read",
    "invitation:write",
    "invitation:delete",
    "mcp-server:read",
    "mcp-server:write",
    "mcp-server:delete",
    "form:read",
    "form:write",
    "entry:read",
    "entry:write",
    "adapter:read",
    "adapter:write",
    "adapter:delete",
    // Commerce
    "plan:read",
    "plan:write",
    "plan:delete",
    "pricing:read",
    "pricing:write",
    "pricing:delete",
    "product:read",
    "product:write",
    "product:delete",
    "order:read",
    "order:write",
    "order:delete",
    "payment:read",
    "payment:write",
    "payment:delete",
    "coupon:read",
    "coupon:write",
    "coupon:delete",
    "subscription:read",
    "subscription:write",
    "subscription:delete",
    "transaction:read",
    "transaction:write",
    "transaction:delete",
    // Payments reconciliation (POST-only run/resolve endpoints)
    "pay-recon:write",
    "pay-recon-items:approve",
    "pay-recon-items:write",
    "pay-channel-configs:read",
    "pay-channel-configs:write",
    // AI Gateway
    "aigateway-provider:read",
    "aigateway-provider:write",
    "aigateway-provider:delete",
    "aigateway-model:read",
    "aigateway-model:write",
    "aigateway-model:delete",
    "aigateway-alias:read",
    "aigateway-alias:write",
    "aigateway-alias:delete",
    "aigateway-route:read",
    "aigateway-route:write",
    "aigateway-route:delete",
    "aigateway-api-key:read",
    "aigateway-api-key:write",
    "aigateway-api-key:delete",
    "aigateway-quota:read",
    "aigateway-quota:write",
    "aigateway-quota:delete",
    "aigateway-price:read",
    "aigateway-price:write",
    "aigateway-price:delete",
    // Security extensions
    "captcha-provider:read",
    "captcha-provider:write",
    "notification-provider:read",
    "notification-provider:write",
    "face-id-provider:read",
    "face-id-provider:write",
    "radius-config:read",
    "radius-config:write",
    "radius-config:delete",
    // Gateway servers/sites
    "server:read",
    "server:write",
    "server:delete",
    "site:read",
    "site:write",
    "site:delete",
    // AI agents
    "agent:read",
    "agent:write",
    "agent:delete",
    // Support
    "ticket:read",
    "ticket:write",
    "ticket:delete",
    // Sync/Directory/Rule engines
    "ldaps:read",
    "ldaps:write",
    "ldaps:delete",
    "rules:read",
    "rules:write",
    "rules:delete",
    "iga:read",
    "iga:write",
    // Governance surfaces: keep the admin fallback in lockstep with
    // internal/app/auth/adminPermissionSet and contracts/registries/permissions.yaml.
    "actions:read",
    "actions:write",
    "rebac:read",
    "rebac:write",
    "rebac:check",
    "risk:read",
    "risk:write",
    "metering:read",
    "metering:write",
    // Workload identity
    "identity-pools:read",
    "identity-pools:write",
    // Localization
    "i18n:read",
    "i18n:write",
    // System
    "dashboard:read",
  ],
  resources: [
    "users",
    "organizations",
    "domains",
    "applications",
    "providers",
    "roles",
    "permissions",
    "models",
    "adapters",
    "enforcers",
    "certs",
    "webhooks",
    "syncers",
    "resources",
    "records",
    "sessions",
    "tokens",
    "groups",
    "invitations",
    "keys",
    "policies",
    "mcp-servers",
    "gateway",
    "forms",
    "entries",
    "identity-pools",
    // Commerce
    "plans",
    "pricings",
    "products",
    "orders",
    "payments",
    "coupons",
    "subscriptions",
    "transactions",
    // Payments reconciliation
    "pay-recon",
    "pay-recon-items",
    "pay-channel-configs",
    // AI Gateway
    "aigateway-providers",
    "aigateway-models",
    "aigateway-aliases",
    "aigateway-routes",
    "aigateway-api-keys",
    "aigateway-quotas",
    "aigateway-prices",
    // Security extensions
    "captcha-providers",
    "notification-providers",
    "face-id-providers",
    "radius-configs",
    // Gateway servers/sites
    "servers",
    "sites",
    // AI agents
    "agents",
    // Support
    "tickets",
    // Sync/Directory/Rule engines
    "ldaps",
    "rules",
    "iga",
    "actions",
    "rebac",
    "risk",
    "metering",
    // Localization
    "i18n",
  ],
};

const EMPTY_PERMISSIONS: UserPermissionSet = {
  menus: [],
  actions: [],
  resources: [],
};

/**
 * TEMPORARY ALIAS (transitional) — do not inline aliases at call sites.
 *
 * The route registry declares the canonical keys `users:read|write|delete`
 * (plural resource), while older `/auth/permissions` responses and in-tree
 * guards still use the singular `user:*` spelling. Accept either spelling so
 * guards do not silently hide admin buttons. Delete this map one release
 * after every in-tree guard has migrated to the plural keys.
 * 删除用户有独立权限点 `users:delete`（DELETE /api/v1/users/{id} 的声明权限，
 * permissions.yaml users 域），树内守卫直接使用复数规范键；单数 `user:delete`
 * 没有生产守卫，因此不需要 delete 别名组。
 */
export const ACTION_ALIAS_GROUPS: readonly (readonly string[])[] = [
  ["users:read", "user:read"],
  ["users:write", "user:write"],
  // organizations 迁移过渡：后端与路由声明已改用复数 `organizations:*`，
  // 一版内保留旧 `org:*` 拼写以免树外守卫静默失效。删除组织有独立权限点
  // `organizations:delete`（DELETE /api/v1/organizations/{id} 的声明权限，
  // routes.go:71 + permissions.yaml:103），树内守卫直接使用复数规范键；单数
  // `org:delete` 没有生产守卫，因此不需要 delete 别名组。
  ["organizations:read", "org:read"],
  ["organizations:write", "org:write"],
];

function resolveActionAliases(action: string): readonly string[] {
  const group = ACTION_ALIAS_GROUPS.find((entry) => entry.includes(action));
  return group ?? [action];
}

function isPermissionSet(obj: unknown): obj is UserPermissionSet {
  return (
    typeof obj === "object" &&
    obj !== null &&
    "menus" in obj &&
    "actions" in obj &&
    "resources" in obj
  );
}

function unwrapDataEnvelope(raw: unknown): unknown {
  if (typeof raw === "object" && raw !== null && "data" in raw) {
    return (raw as { data: unknown }).data;
  }
  return raw;
}

export function usePermissions() {
  const account = useSessionStore((state) => state.account);
  const isAdmin = account?.isAdmin === true;

  const query = useQuery<UserPermissionSet>({
    queryKey: queryKeys.auth.permissions(),
    queryFn: async () => {
      const raw = await iamGet<unknown>(AUTH_PERMISSIONS);
      // Backend may wrap in { status, data } envelope — unwrap defensively.
      if (isPermissionSet(raw)) return raw;
      const unwrapped = unwrapDataEnvelope(raw);
      if (isPermissionSet(unwrapped)) return unwrapped;
      return EMPTY_PERMISSIONS;
    },
    staleTime: 5 * 60 * 1000, // 5 minutes cache
    // retry 由 QueryClient.defaultOptions 统一管理（生产 401 不重试，测试 retry: false）
  });

  // ── Degraded-mode fallback ──
  // If the permissions API is failing but we know from the local session
  // store that the current user is an admin, hand back the hard-coded
  // full set so PermissionGuard buttons don't silently disappear.
  // Non-admin users still get the empty set (safe default).
  const perms: UserPermissionSet =
    query.isError && isAdmin
      ? ADMIN_FULL_PERMISSIONS
      : (query.data ?? (isAdmin ? ADMIN_FULL_PERMISSIONS : EMPTY_PERMISSIONS));

  return {
    permissions: perms,
    isLoading: query.isLoading,
    isError: query.isError,
    refetch: query.refetch,
    hasMenu: (path: string) => perms.menus.includes(path),
    hasAction: (action: string) =>
      resolveActionAliases(action).some((key) => perms.actions.includes(key)),
    hasResource: (resource: string) => perms.resources.includes(resource),
  };
}
