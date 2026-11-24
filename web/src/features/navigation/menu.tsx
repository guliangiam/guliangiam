import {
  AlertOutlined,
  ApiOutlined,
  AppstoreOutlined,
  AuditOutlined,
  BarChartOutlined,
  BellOutlined,
  BgColorsOutlined,
  CloudServerOutlined,
  ClusterOutlined,
  CodeOutlined,
  ControlOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  DollarOutlined,
  ExperimentOutlined,
  FileOutlined,
  FileSearchOutlined,
  FileTextOutlined,
  FormOutlined,
  FundOutlined,
  GlobalOutlined,
  HddOutlined,
  HeartOutlined,
  IdcardOutlined,
  ImportOutlined,
  InsuranceOutlined,
  KeyOutlined,
  LaptopOutlined,
  LineChartOutlined,
  LinkOutlined,
  LockOutlined,
  LoginOutlined,
  MoneyCollectOutlined,
  PartitionOutlined,
  ProfileOutlined,
  ProjectOutlined,
  PropertySafetyOutlined,
  RobotOutlined,
  SafetyCertificateOutlined,
  ScanOutlined,
  ScheduleOutlined,
  ShopOutlined,
  WarningOutlined,
  SecurityScanOutlined,
  SendOutlined,
  SettingOutlined,
  ShoppingCartOutlined,
  SolutionOutlined,
  SwapOutlined,
  SyncOutlined,
  TagsOutlined,
  TeamOutlined,
  ThunderboltOutlined,
  ToolOutlined,
  UnorderedListOutlined,
  UserOutlined,
} from "@ant-design/icons";
import type { ReactNode } from "react";
import type { TFunction } from "i18next";
import type { SessionAccount, SessionOrganization } from "@/shared/types/domain";

import type { ItemType } from "antd/es/menu/interface";

export type MenuChild = {
  key: string;
  label: string;
  icon: ReactNode;
  matchPaths?: string[];
};

export type MenuGroup = {
  key: string;
  label: string;
  icon: ReactNode;
  routeKeys: string[];
  children: MenuChild[];
};

export const menuGroups: MenuGroup[] = [
  {
    key: "home",
    label: "nav.home",
    icon: <DashboardOutlined />,
    routeKeys: ["/", "/shortcuts", "/apps", "/oidc-discovery"],
    children: [
      { key: "/", label: "nav.dashboard", icon: <BarChartOutlined /> },
      { key: "/shortcuts", label: "nav.shortcuts", icon: <AppstoreOutlined /> },
      { key: "/apps", label: "nav.apps", icon: <GlobalOutlined /> },
      { key: "/oidc-discovery", label: "nav.oidcDiscovery", icon: <FileSearchOutlined /> },
    ],
  },
  {
    key: "identity",
    label: "nav.identity",
    icon: <SafetyCertificateOutlined />,
    routeKeys: [
      "/organizations",
      "/settings",
      "/users",
      "/users/import",
      "/groups",
      "/invitations",
      "/applications",
    ],
    children: [
      { key: "/organizations", label: "nav.orgAndSettings", icon: <TeamOutlined /> },
      { key: "/users", label: "nav.usersAndImport", icon: <UserOutlined /> },
      { key: "/groups", label: "nav.groups", icon: <IdcardOutlined /> },
      { key: "/invitations", label: "nav.invitations", icon: <SendOutlined /> },
      { key: "/applications", label: "nav.applications", icon: <ProjectOutlined /> },
    ],
  },
  {
    key: "authz",
    label: "nav.authorization",
    icon: <LockOutlined />,
    routeKeys: [
      "/roles",
      "/permissions",
      "/models",
      "/adapters",
      "/enforcers",
      "/rebac",
      "/actions",
    ],
    children: [
      { key: "/roles", label: "nav.roles", icon: <LockOutlined /> },
      { key: "/permissions", label: "nav.permissions", icon: <ControlOutlined /> },
      { key: "/models", label: "nav.models", icon: <PartitionOutlined /> },
      { key: "/adapters", label: "nav.adapters", icon: <DatabaseOutlined /> },
      { key: "/enforcers", label: "nav.enforcers", icon: <AuditOutlined /> },
      { key: "/rebac", label: "nav.rebac", icon: <PropertySafetyOutlined /> },
      { key: "/actions", label: "nav.actions.list", icon: <ThunderboltOutlined /> },
    ],
  },
  {
    key: "integration",
    label: "nav.integration",
    icon: <ToolOutlined />,
    routeKeys: [
      "/providers",
      "/syncers",
      "/migration",
      "/webhooks",
      "/webhooks/:webhookId/events",
      "/ldaps",
      "/servers",
      "/sites",
    ],
    children: [
      { key: "/providers", label: "nav.providers", icon: <ApiOutlined /> },
      { key: "/syncers", label: "nav.syncers", icon: <SyncOutlined /> },
      { key: "/migration", label: "nav.migration", icon: <ImportOutlined /> },
      { key: "/webhooks", label: "nav.webhooks", icon: <LinkOutlined /> },
      { key: "/ldaps", label: "nav.ldap", icon: <SolutionOutlined /> },
      { key: "/servers", label: "nav.servers", icon: <CloudServerOutlined /> },
      { key: "/sites", label: "nav.sites", icon: <HddOutlined /> },
    ],
  },
  {
    key: "audit",
    label: "nav.audit",
    icon: <ScheduleOutlined />,
    routeKeys: [
      "/audit/login-logs",
      "/audit/operation-logs",
      "/audit/error-logs",
      "/sessions",
      "/records",
      "/tokens",
      "/verifications",
      "/entries",
    ],
    children: [
      { key: "/audit/login-logs", label: "nav.loginLogs", icon: <LoginOutlined /> },
      { key: "/audit/operation-logs", label: "nav.operationLogs", icon: <UnorderedListOutlined /> },
      { key: "/audit/error-logs", label: "nav.errorLogs", icon: <WarningOutlined /> },
      { key: "/sessions", label: "nav.sessions", icon: <AlertOutlined /> },
      { key: "/records", label: "nav.records", icon: <FileTextOutlined /> },
      { key: "/tokens", label: "nav.tokens", icon: <KeyOutlined /> },
      { key: "/verifications", label: "nav.verifications", icon: <ScanOutlined /> },
      { key: "/entries", label: "nav.entries", icon: <FileSearchOutlined /> },
    ],
  },
  {
    key: "security",
    label: "nav.security",
    icon: <SecurityScanOutlined />,
    routeKeys: [
      "/certs",
      "/keys",
      "/radius-configs",
      "/radius-accountings",
      "/rules",
      "/risk",
      "/iga",
      "/idv",
      "/identity-pools",
    ],
    children: [
      { key: "/certs", label: "nav.certs", icon: <SafetyCertificateOutlined /> },
      { key: "/keys", label: "nav.keys", icon: <PropertySafetyOutlined /> },
      { key: "/radius-configs", label: "nav.radiusConfigs", icon: <ApiOutlined /> },
      { key: "/radius-accountings", label: "nav.radiusAccountings", icon: <BarChartOutlined /> },
      { key: "/rules", label: "nav.rules", icon: <ThunderboltOutlined /> },
      { key: "/risk", label: "nav.risk", icon: <ScanOutlined /> },
      { key: "/iga", label: "nav.iga.title", icon: <SolutionOutlined /> },
      { key: "/idv", label: "nav.idv", icon: <IdcardOutlined /> },
      { key: "/identity-pools", label: "nav.workloadTitle", icon: <ClusterOutlined /> },
    ],
  },
  {
    key: "business",
    label: "nav.business",
    icon: <ShoppingCartOutlined />,
    routeKeys: [
      "/store",
      "/products",
      "/plans",
      "/coupons",
      "/cart",
      "/orders",
      "/payments",
      "/pricings",
      "/subscriptions",
      "/transactions",
      "/metering",
    ],
    children: [
      { key: "/store", label: "nav.store", icon: <ShopOutlined /> },
      { key: "/products", label: "nav.productsAndPlans", icon: <TagsOutlined /> },
      { key: "/coupons", label: "nav.coupons", icon: <MoneyCollectOutlined /> },
      { key: "/cart", label: "nav.cart", icon: <ShoppingCartOutlined /> },
      { key: "/orders", label: "nav.ordersAndPayments", icon: <UnorderedListOutlined /> },
      { key: "/pricings", label: "nav.pricings", icon: <FundOutlined /> },
      { key: "/subscriptions", label: "nav.subscriptions", icon: <IdcardOutlined /> },
      { key: "/transactions", label: "nav.transactions", icon: <SwapOutlined /> },
      { key: "/metering", label: "nav.metering", icon: <FundOutlined /> },
    ],
  },
  {
    key: "llm",
    label: "nav.llmAi",
    icon: <ExperimentOutlined />,
    routeKeys: [
      "/ai/providers",
      "/ai/providers/new",
      "/ai/providers/:id",
      "/ai/providers/:id/test",
      "/ai/usage/stats",
      "/ai/agents/new",
      "/ai/agents/:id",
      "/ai/models",
      "/ai/aliases",
      "/ai/routes",
      "/ai/api-keys",
      "/ai/quotas",
      "/ai/prices",
      "/ai/usage",
      "/ai/health",
      "/ai/agents",
      "/ai/mcp-servers",
      "/ai/mcp-servers/new",
      "/ai/mcp-servers/:id",
      "/ai/mcp-servers/store",
    ],
    children: [
      { key: "/ai/providers", label: "nav.aiGateway.providers", icon: <DatabaseOutlined /> },
      { key: "/ai/models", label: "nav.aiGateway.models", icon: <ClusterOutlined /> },
      { key: "/ai/aliases", label: "nav.aiGateway.aliases", icon: <TagsOutlined /> },
      { key: "/ai/routes", label: "nav.aiGateway.routes", icon: <SwapOutlined /> },
      { key: "/ai/api-keys", label: "nav.aiGateway.apiKeys", icon: <KeyOutlined /> },
      { key: "/ai/quotas", label: "nav.aiGateway.quotas", icon: <ControlOutlined /> },
      { key: "/ai/prices", label: "nav.aiGateway.prices", icon: <DollarOutlined /> },
      { key: "/ai/usage", label: "nav.aiGateway.usage", icon: <BarChartOutlined /> },
      { key: "/ai/usage/stats", label: "nav.aiGateway.usageStats", icon: <LineChartOutlined /> },
      { key: "/ai/health", label: "nav.aiGateway.health", icon: <HeartOutlined /> },
      { key: "/ai/agents", label: "nav.aiGateway.agents", icon: <RobotOutlined /> },
      { key: "/ai/mcp-servers", label: "nav.aiGateway.mcpServers", icon: <LaptopOutlined /> },
      { key: "/ai/mcp-servers/store", label: "nav.aiGateway.store", icon: <ShopOutlined /> },
    ],
  },
  {
    key: "admin",
    label: "nav.admin",
    icon: <SettingOutlined />,
    routeKeys: [
      "/system-info",
      "/resources",
      "/account",
      "/theme-editor",
      "/forms",
      "/tickets",
      "/swagger",
      "/i18n/languages",
      "/i18n/translations",
      "/notifications",
      "/notification-deliveries",
    ],
    children: [
      { key: "/system-info", label: "nav.systemInfo", icon: <FundOutlined /> },
      { key: "/resources", label: "nav.resources", icon: <FileOutlined /> },
      { key: "/account", label: "nav.account", icon: <ProfileOutlined /> },
      { key: "/theme-editor", label: "nav.themeEditor", icon: <BgColorsOutlined /> },
      { key: "/forms", label: "nav.forms", icon: <FormOutlined /> },
      { key: "/tickets", label: "nav.tickets", icon: <InsuranceOutlined /> },
      { key: "/swagger", label: "nav.swagger", icon: <CodeOutlined /> },
      { key: "/i18n/languages", label: "nav.i18n", icon: <GlobalOutlined /> },
      { key: "/notifications", label: "nav.notifications", icon: <BellOutlined /> },
      {
        key: "/notification-deliveries",
        label: "nav.notificationDeliveries",
        icon: <SendOutlined />,
      },
    ],
  },
];

/**
 * 返回组织配置的导航键数组；null 表示「未配置限制」（空数组或含 "all"），
 * 此时菜单不做过滤、保持静态顺序。
 */
function getConfiguredNavKeys(
  account: SessionAccount | null,
  organization: SessionOrganization | null,
): string[] | null {
  const configured = account?.isAdmin ? organization?.navItems : organization?.userNavItems;
  if (!configured || configured.length === 0 || configured.includes("all")) {
    return null;
  }
  return configured;
}

export function getVisibleRouteKeys(
  account: SessionAccount | null,
  organization: SessionOrganization | null,
): Set<string> | null {
  const configured = getConfiguredNavKeys(account, organization);
  if (!configured) return null;
  return new Set(configured);
}

/** 兼容历史数据：navItems 可能存带斜杠或不带斜杠两种形态。 */
function isVisibleKey(visible: Set<string>, key: string): boolean {
  return visible.has(key) || visible.has(key.replace(/^\//, ""));
}

/** 按配置数组顺序给叶子排序；未列出的叶子保持静态顺序排在最后。 */
function sortChildrenByOrder(children: MenuChild[], order: string[] | null): MenuChild[] {
  if (!order) return children;
  const rank = new Map(order.map((key, index) => [key.replace(/^\//, ""), index]));
  return [...children].sort(
    (a, b) =>
      (rank.get(a.key.replace(/^\//, "")) ?? Number.MAX_SAFE_INTEGER) -
      (rank.get(b.key.replace(/^\//, "")) ?? Number.MAX_SAFE_INTEGER),
  );
}

/** 按 navGroupOrder 给分组排序；未列出的分组保持静态顺序排在最后。 */
function orderedGroups(organization: SessionOrganization | null): MenuGroup[] {
  const groupOrder = organization?.navGroupOrder;
  if (!groupOrder || groupOrder.length === 0) return menuGroups;
  const rank = new Map(groupOrder.map((key, index) => [key, index]));
  return [...menuGroups].sort(
    (a, b) =>
      (rank.get(a.key) ?? Number.MAX_SAFE_INTEGER) - (rank.get(b.key) ?? Number.MAX_SAFE_INTEGER),
  );
}

/**
 * 直达路由守卫：判断当前 pathname 是否被组织导航配置允许。
 *
 * 与侧边栏过滤同源（getVisibleRouteKeys）：先找最长匹配的菜单叶子，
 * 命中且不可见返回 false；未配置限制或路径不属于任何菜单叶子时返回 true
 * （交由路由本身处理）。
 */
export function isRouteKeyVisible(
  account: SessionAccount | null,
  organization: SessionOrganization | null,
  pathname: string,
): boolean {
  const visibleRouteKeys = getVisibleRouteKeys(account, organization);
  if (!visibleRouteKeys) return true;

  // /account 是自助页与强制改密（needUpdatePassword）唯一落点：即使被导航配置
  // 隐藏也必须可达，否则被要求改密的用户会被 403 守卫锁死在门外。
  if (pathname === "/account" || pathname.startsWith("/account/")) return true;

  let bestKey: string | null = null;
  let bestLen = 0;
  for (const group of menuGroups) {
    for (const child of group.children) {
      if (pathname === child.key || pathname.startsWith(`${child.key}/`)) {
        if (child.key.length > bestLen) {
          bestLen = child.key.length;
          bestKey = child.key;
        }
      }
    }
  }
  if (!bestKey) return true;
  return isVisibleKey(visibleRouteKeys, bestKey);
}

const FLAT_MENU_THRESHOLD = 7;

/**
 * Search all menu items for a matching query string.
 * Returns matching items with their group context for quick navigation.
 * `allowedMenus`（批次9a）为服务端 /auth/permissions 派生的授权菜单键集合：
 * 传入数组时未授权叶子完全不参与搜索；null/undefined 表示不过滤（加载中或
 * 向后兼容调用）。
 */
export function searchMenuItems(
  account: SessionAccount | null,
  organization: SessionOrganization | null,
  query: string,
  t: TFunction,
  allowedMenus?: string[] | null,
): Array<{ key: string; label: string; groupLabel: string; icon: ReactNode }> {
  const lowerQuery = query.toLowerCase();
  const results: Array<{ key: string; label: string; groupLabel: string; icon: ReactNode }> = [];

  const visibleRouteKeys = getVisibleRouteKeys(account, organization);
  const leafOrder = getConfiguredNavKeys(account, organization);
  // oxlint-disable-next-line eqeqeq — intentional null+undefined coalesce
  const allowed = allowedMenus == null ? null : new Set(allowedMenus);

  for (const group of orderedGroups(organization)) {
    const groupLabel = t(group.label);
    for (const child of sortChildrenByOrder(group.children, leafOrder)) {
      if (visibleRouteKeys && !isVisibleKey(visibleRouteKeys, child.key)) {
        continue;
      }
      if (allowed && !isVisibleKey(allowed, child.key)) {
        continue;
      }
      const label = t(child.label);
      if (
        label.toLowerCase().includes(lowerQuery) ||
        child.key.toLowerCase().includes(lowerQuery) ||
        groupLabel.toLowerCase().includes(lowerQuery)
      ) {
        results.push({
          key: child.key,
          label,
          groupLabel,
          icon: child.icon,
        });
      }
    }
  }

  return results;
}

/**
 * Build sidebar menu items. `allowedMenus`（批次9a）为服务端
 * /auth/permissions 派生的授权菜单键集合：传入数组时未授权叶子完全隐藏
 * （整组无可见叶子则隐藏分组）；null/undefined 表示不过滤（加载中或
 * 向后兼容调用）。
 */
export function buildMenuItems(
  account: SessionAccount | null,
  organization: SessionOrganization | null,
  t: TFunction,
  allowedMenus?: string[] | null,
): ItemType[] {
  const visibleRouteKeys = getVisibleRouteKeys(account, organization);
  const leafOrder = getConfiguredNavKeys(account, organization);
  // oxlint-disable-next-line eqeqeq — intentional null+undefined coalesce
  const allowed = allowedMenus == null ? null : new Set(allowedMenus);

  // 逐叶过滤 + 按配置顺序排序：分组内只保留可见且已授权的叶子，
  // 整组无可见叶子则隐藏该组。
  const filteredGroups = orderedGroups(organization)
    .map((group) => ({
      group,
      children: sortChildrenByOrder(group.children, leafOrder).filter((child) => {
        if (visibleRouteKeys && !isVisibleKey(visibleRouteKeys, child.key)) return false;
        if (allowed && !isVisibleKey(allowed, child.key)) return false;
        return true;
      }),
    }))
    .filter(({ children }) => {
      if (!visibleRouteKeys && !allowed) return true;
      return children.length > 0;
    });

  // Count total visible child items
  const totalItems = filteredGroups.reduce((sum, g) => sum + g.children.length, 0);

  // Flat mode: when total items <= threshold, render as a flat list without groups
  if (totalItems <= FLAT_MENU_THRESHOLD) {
    const flatItems: ItemType[] = [];
    for (const { children } of filteredGroups) {
      for (const child of children) {
        flatItems.push({
          key: child.key,
          icon: child.icon,
          label: t(child.label),
        });
      }
    }
    return flatItems;
  }

  // Grouped mode: use SubMenu for accordion-style collapsible groups
  return filteredGroups.map(({ group, children }): ItemType => ({
    key: group.key,
    icon: group.icon,
    label: t(group.label),
    children: children.map((child) => ({
      key: child.key,
      icon: child.icon,
      label: t(child.label),
    })),
  }));
}

/**
 * Find the selected menu key from the current pathname.
 * Returns the child item key whose path best matches the pathname.
 */
export function findSelectedKey(items: ItemType[], pathname: string): string {
  let bestKey = "/";
  let bestLen = 0;

  for (const item of items) {
    if (!item) continue;
    // Flat item (no children)
    if ("key" in item && item.key && typeof item.key === "string") {
      const paths = [item.key as string];
      for (const p of paths) {
        if (pathname === p || pathname.startsWith(`${p}/`)) {
          if (p.length > bestLen) {
            bestLen = p.length;
            bestKey = item.key as string;
          }
        }
      }
    }
    // Group or SubMenu with children
    if ("children" in item && item.children) {
      for (const child of item.children) {
        if (!child || !("key" in child)) continue;
        const p = child.key as string;
        if (pathname === p || pathname.startsWith(`${p}/`)) {
          if (p.length > bestLen) {
            bestLen = p.length;
            bestKey = p;
          }
        }
      }
    }
  }

  return bestKey;
}

/**
 * Build breadcrumb items from current pathname.
 * Returns an array of { path, label } for rendering.
 * Supports three-level breadcrumbs: Home > Group > Page
 * The first two levels are clickable for navigation.
 */
export function buildBreadcrumbs(
  account: SessionAccount | null,
  organization: SessionOrganization | null,
  pathname: string,
  t: TFunction,
): Array<{ path: string; label: string }> {
  // Find the LONGEST-matching child across all groups (not first-match-wins)
  let bestGroup: (typeof menuGroups)[number] | null = null;
  let bestChild: { key: string; label: string } | null = null;
  let bestLen = 0;

  for (const group of menuGroups) {
    for (const child of group.children) {
      if (pathname === child.key || pathname.startsWith(`${child.key}/`)) {
        if (child.key.length > bestLen) {
          bestLen = child.key.length;
          bestGroup = group;
          bestChild = child;
        }
      }
    }
  }

  if (!bestGroup || !bestChild) return [];

  return [
    { path: "/", label: t("nav.home") },
    { path: "/", label: t(bestGroup.label) },
    { path: bestChild.key, label: t(bestChild.label) },
  ];
}

/**
 * Find the menu group key that contains the given route path.
 * Used for auto-expanding the correct group in the sidebar.
 */
export function findGroupKeyForPath(pathname: string): string | null {
  // Find the group whose child has the LONGEST matching key
  let bestGroup: string | null = null;
  let bestLen = 0;
  for (const group of menuGroups) {
    for (const child of group.children) {
      if (pathname === child.key || pathname.startsWith(`${child.key}/`)) {
        if (child.key.length > bestLen) {
          bestLen = child.key.length;
          bestGroup = group.key;
        }
      }
    }
  }
  return bestGroup;
}
