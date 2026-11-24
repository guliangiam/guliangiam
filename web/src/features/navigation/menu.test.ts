import { describe, it, expect } from "vitest";
import {
  buildBreadcrumbs,
  buildMenuItems,
  findSelectedKey,
  findGroupKeyForPath,
  isRouteKeyVisible,
  menuGroups,
  searchMenuItems,
} from "./menu";

const mockT = ((key: string) => key) as never;

describe("menuGroups structure", () => {
  it("has 9 groups", () => {
    expect(menuGroups).toHaveLength(9);
  });

  it("each group has 3-15 children (balanced distribution)", () => {
    for (const group of menuGroups) {
      expect(group.children.length).toBeGreaterThanOrEqual(3);
      expect(group.children.length).toBeLessThanOrEqual(15);
    }
  });

  it("home group has 4 items (dashboard, shortcuts, apps, oidc-discovery)", () => {
    const home = menuGroups.find((g) => g.key === "home");
    expect(home).toBeDefined();
    expect(home!.children.length).toBe(4);
  });

  it("identity group has 5 items after slimming", () => {
    const identity = menuGroups.find((g) => g.key === "identity");
    expect(identity).toBeDefined();
    expect(identity!.children.length).toBe(5);
  });

  it("authz group has 7 items", () => {
    const authz = menuGroups.find((g) => g.key === "authz");
    expect(authz).toBeDefined();
    expect(authz!.children.length).toBe(7);
  });

  it("audit group has 8 items", () => {
    const audit = menuGroups.find((g) => g.key === "audit");
    expect(audit).toBeDefined();
    expect(audit!.children.length).toBe(8);
    expect(audit!.children.map((child) => child.key)).toEqual(
      expect.arrayContaining(["/audit/login-logs", "/audit/operation-logs", "/audit/error-logs"]),
    );
  });

  it("business group has 9 items", () => {
    const business = menuGroups.find((g) => g.key === "business");
    expect(business).toBeDefined();
    expect(business!.children.length).toBe(9);
  });

  it("integration group has 7 items (providers moved in)", () => {
    const integration = menuGroups.find((g) => g.key === "integration");
    expect(integration).toBeDefined();
    expect(integration!.children.length).toBe(7);
  });

  it("llm group has 13 items (AI gateway + agents + mcp-servers)", () => {
    const llm = menuGroups.find((g) => g.key === "llm");
    expect(llm).toBeDefined();
    expect(llm!.children.length).toBe(13);
  });

  it("security group has 9 items (resources moved to admin)", () => {
    const security = menuGroups.find((g) => g.key === "security");
    expect(security).toBeDefined();
    expect(security!.children.length).toBe(9);
    expect(security!.children.map((c) => c.key)).not.toContain("/resources");
  });

  it("admin group has 10 items (resources + notification deliveries)", () => {
    const admin = menuGroups.find((g) => g.key === "admin");
    expect(admin).toBeDefined();
    expect(admin!.children.length).toBe(10);
    expect(admin!.children.map((c) => c.key)).toContain("/resources");
    expect(admin!.routeKeys).toContain("/resources");
    expect(admin!.children.map((c) => c.key)).toContain("/notification-deliveries");
    expect(admin!.routeKeys).toContain("/notification-deliveries");
  });

  it("no icon is used more than twice within a group", () => {
    for (const group of menuGroups) {
      const iconCounts = new Map<string, number>();
      for (const child of group.children) {
        const iconType =
          (child.icon as { type?: { displayName?: string } })?.type?.displayName ?? "";
        if (!iconType) continue;
        iconCounts.set(iconType, (iconCounts.get(iconType) ?? 0) + 1);
      }
      // A single icon component may appear up to twice (e.g. FundOutlined for
      // pricings + metering, ScanOutlined for face-id + risk); anything above
      // two is a likely mistake worth flagging.
      for (const [icon, count] of iconCounts) {
        expect(count, `${group.key} uses ${icon} ${count} times`).toBeLessThanOrEqual(2);
      }
    }
  });
});

describe("buildBreadcrumbs", () => {
  it("returns three-level breadcrumbs for /users", () => {
    const crumbs = buildBreadcrumbs(null, null, "/users", mockT);
    expect(crumbs).toHaveLength(3);
    expect(crumbs[0]).toEqual({ path: "/", label: "nav.home" });
    expect(crumbs[1].label).toBe("nav.identity");
    expect(crumbs[2].label).toBe("nav.usersAndImport");
  });

  it("returns breadcrumbs for /users/import (nested path)", () => {
    const crumbs = buildBreadcrumbs(null, null, "/users/import", mockT);
    expect(crumbs).toHaveLength(3);
    expect(crumbs[1].label).toBe("nav.identity");
  });

  it("returns breadcrumbs for /organizations", () => {
    const crumbs = buildBreadcrumbs(null, null, "/organizations", mockT);
    expect(crumbs).toHaveLength(3);
    expect(crumbs[2].label).toBe("nav.orgAndSettings");
  });

  it("returns empty for unmatched path", () => {
    const crumbs = buildBreadcrumbs(null, null, "/nonexistent", mockT);
    expect(crumbs).toHaveLength(0);
  });

  it("returns breadcrumbs for /sessions (audit group)", () => {
    const crumbs = buildBreadcrumbs(null, null, "/sessions", mockT);
    expect(crumbs).toHaveLength(3);
    expect(crumbs[1].label).toBe("nav.audit");
  });

  it("returns breadcrumbs for /webhooks (integration group)", () => {
    const crumbs = buildBreadcrumbs(null, null, "/webhooks", mockT);
    expect(crumbs).toHaveLength(3);
    expect(crumbs[1].label).toBe("nav.integration");
  });

  it("returns breadcrumbs for /providers (integration group)", () => {
    const crumbs = buildBreadcrumbs(null, null, "/providers", mockT);
    expect(crumbs).toHaveLength(3);
    expect(crumbs[1].label).toBe("nav.integration");
  });

  it("returns breadcrumbs for /resources (moved to admin group)", () => {
    const crumbs = buildBreadcrumbs(null, null, "/resources", mockT);
    expect(crumbs).toHaveLength(3);
    expect(crumbs[1].label).toBe("nav.admin");
    expect(crumbs[2].label).toBe("nav.resources");
  });

  it("returns breadcrumbs for /certs (now in security group)", () => {
    const crumbs = buildBreadcrumbs(null, null, "/certs", mockT);
    expect(crumbs).toHaveLength(3);
    expect(crumbs[1].label).toBe("nav.security");
  });

  it("returns breadcrumbs for /keys (now in security group)", () => {
    const crumbs = buildBreadcrumbs(null, null, "/keys", mockT);
    expect(crumbs).toHaveLength(3);
    expect(crumbs[1].label).toBe("nav.security");
  });

  it("returns breadcrumbs for /entries (now in audit group)", () => {
    const crumbs = buildBreadcrumbs(null, null, "/entries", mockT);
    expect(crumbs).toHaveLength(3);
    expect(crumbs[1].label).toBe("nav.audit");
  });

  it("returns breadcrumbs for /oidc-discovery (home group)", () => {
    const crumbs = buildBreadcrumbs(null, null, "/oidc-discovery", mockT);
    expect(crumbs).toHaveLength(3);
    expect(crumbs[1].label).toBe("nav.home");
  });
});

describe("findGroupKeyForPath", () => {
  it("finds home group for /", () => {
    expect(findGroupKeyForPath("/")).toBe("home");
  });

  it("finds identity group for /users", () => {
    expect(findGroupKeyForPath("/users")).toBe("identity");
  });

  it("finds authz group for /roles", () => {
    expect(findGroupKeyForPath("/roles")).toBe("authz");
  });

  it("finds audit group for /sessions", () => {
    expect(findGroupKeyForPath("/sessions")).toBe("audit");
  });

  it("finds audit group for /entries (moved from integration)", () => {
    expect(findGroupKeyForPath("/entries")).toBe("audit");
  });

  it("finds integration group for /webhooks", () => {
    expect(findGroupKeyForPath("/webhooks")).toBe("integration");
  });

  it("finds integration group for /syncers", () => {
    expect(findGroupKeyForPath("/syncers")).toBe("integration");
  });

  it("finds integration group for /providers", () => {
    expect(findGroupKeyForPath("/providers")).toBe("integration");
  });

  it("finds security group for /rules", () => {
    expect(findGroupKeyForPath("/rules")).toBe("security");
  });

  it("finds admin group for /resources (moved from security)", () => {
    expect(findGroupKeyForPath("/resources")).toBe("admin");
  });

  it("finds security group for /certs (moved from identity)", () => {
    expect(findGroupKeyForPath("/certs")).toBe("security");
  });

  it("finds security group for /keys (moved from identity)", () => {
    expect(findGroupKeyForPath("/keys")).toBe("security");
  });

  it("finds admin group for /tickets", () => {
    expect(findGroupKeyForPath("/tickets")).toBe("admin");
  });

  it("finds llm group for /ai/agents", () => {
    expect(findGroupKeyForPath("/ai/agents")).toBe("llm");
  });

  it("finds llm group for /ai/mcp-servers", () => {
    expect(findGroupKeyForPath("/ai/mcp-servers")).toBe("llm");
  });

  it("finds home group for /oidc-discovery", () => {
    expect(findGroupKeyForPath("/oidc-discovery")).toBe("home");
  });

  it("returns null for unmatched path", () => {
    expect(findGroupKeyForPath("/nonexistent")).toBeNull();
  });

  it("matches nested paths", () => {
    expect(findGroupKeyForPath("/users/import")).toBe("identity");
  });
});

describe("resources menu placement", () => {
  const account = { isAdmin: true } as never;
  const organization = { navItems: ["/resources"], userNavItems: [] } as never;

  it("buildMenuItems renders only the configured leaf /resources (flat mode)", () => {
    const items = buildMenuItems(account, organization, mockT);
    expect(items).toHaveLength(1);
    expect(items[0]).toMatchObject({ key: "/resources" });
  });

  it("searchMenuItems reports the admin group label for /resources", () => {
    const results = searchMenuItems(account, organization, "nav.resources", mockT);
    const resource = results.find((result) => result.key === "/resources");
    expect(resource).toBeDefined();
    expect(resource!.groupLabel).toBe("nav.admin");
  });
});

describe("nav ordering (navItems array order + navGroupOrder)", () => {
  const account = { isAdmin: true } as never;

  it("leaves follow the configured navItems order", () => {
    const staticAdminLeaves = menuGroups
      .find((group) => group.key === "admin")!
      .children.map((child) => child.key);
    // 反转管理员组的叶子顺序，验证渲染顺序跟随配置数组而非静态定义。
    const reversed = [...staticAdminLeaves].reverse();
    const organization = {
      navItems: reversed,
      userNavItems: [],
      navGroupOrder: [],
    } as never;
    const items = buildMenuItems(account, organization, mockT) as Array<{
      key: string;
      children?: Array<{ key: string }>;
    }>;
    const admin = items.find((item) => item.key === "admin");
    expect(admin?.children?.map((child) => child.key)).toEqual(reversed);
  });

  it("groups follow navGroupOrder; unlisted groups keep static order at the end", () => {
    const allAdminLeaves = menuGroups
      .find((group) => group.key === "admin")!
      .children.map((child) => child.key);
    const organization = {
      navItems: allAdminLeaves,
      userNavItems: [],
      navGroupOrder: ["admin"],
    } as never;
    const items = buildMenuItems(account, organization, mockT) as Array<{ key: string }>;
    expect(items[0].key).toBe("admin");
  });

  it("unconfigured organization keeps the exact static layout", () => {
    const organization = { navItems: [], userNavItems: [], navGroupOrder: [] } as never;
    const items = buildMenuItems(account, organization, mockT) as Array<{
      key: string;
      children?: Array<{ key: string }>;
    }>;
    expect(items.map((item) => item.key)).toEqual(menuGroups.map((group) => group.key));
    expect(items[0].children?.map((child) => child.key)).toEqual(
      menuGroups[0].children.map((child) => child.key),
    );
  });
});

describe("isRouteKeyVisible", () => {
  const account = { isAdmin: true } as never;

  it("allows everything when no restriction is configured", () => {
    expect(isRouteKeyVisible(account, { navItems: [], userNavItems: [] } as never, "/users")).toBe(
      true,
    );
    expect(
      isRouteKeyVisible(account, { navItems: ["all"], userNavItems: [] } as never, "/users"),
    ).toBe(true);
  });

  it("denies direct access to a hidden leaf and its sub-paths", () => {
    const organization = { navItems: ["/roles"], userNavItems: [] } as never;
    expect(isRouteKeyVisible(account, organization, "/users")).toBe(false);
    expect(isRouteKeyVisible(account, organization, "/users/123")).toBe(false);
    expect(isRouteKeyVisible(account, organization, "/roles")).toBe(true);
  });

  it("always allows /account so forced password change is never locked out", () => {
    const organization = { navItems: ["/roles"], userNavItems: ["/roles"] } as never;
    expect(isRouteKeyVisible(account, organization, "/account")).toBe(true);
    expect(isRouteKeyVisible({ isAdmin: false } as never, organization, "/account")).toBe(true);
  });

  it("does not govern paths outside the menu tree", () => {
    const organization = { navItems: ["/roles"], userNavItems: [] } as never;
    expect(isRouteKeyVisible(account, organization, "/login")).toBe(true);
    expect(isRouteKeyVisible(account, organization, "/nonexistent")).toBe(true);
  });

  it("uses userNavItems for non-admin viewers", () => {
    const viewer = { isAdmin: false } as never;
    const organization = { navItems: ["/users"], userNavItems: ["/roles"] } as never;
    expect(isRouteKeyVisible(viewer, organization, "/users")).toBe(false);
    expect(isRouteKeyVisible(viewer, organization, "/roles")).toBe(true);
  });
});

describe("findSelectedKey", () => {
  it("finds correct key from flat items", () => {
    const items = [
      { key: "/", label: "Home" },
      { key: "/users", label: "Users" },
      { key: "/settings", label: "Settings" },
    ];
    expect(findSelectedKey(items, "/users")).toBe("/users");
  });

  it("finds correct key from grouped items", () => {
    const items = [
      {
        key: "identity",
        label: "Identity",
        children: [
          { key: "/users", label: "Users" },
          { key: "/organizations", label: "Orgs" },
        ],
      },
    ];
    expect(findSelectedKey(items, "/users")).toBe("/users");
  });

  it("matches nested path to parent key", () => {
    const items = [
      { key: "/users", label: "Users" },
      { key: "/settings", label: "Settings" },
    ];
    expect(findSelectedKey(items, "/users/detail")).toBe("/users");
  });

  it("defaults to / for root path", () => {
    const items = [
      { key: "/", label: "Home" },
      { key: "/users", label: "Users" },
    ];
    expect(findSelectedKey(items, "/")).toBe("/");
  });
});

// 批次9a：侧栏叠加服务端授权菜单过滤（/auth/permissions 派生的 menus）。
describe("buildMenuItems allowedMenus filter", () => {
  const account = { isAdmin: false } as never;
  const unconfiguredOrg = { navItems: [], userNavItems: [], navGroupOrder: [] } as never;

  it("null / omitted allowedMenus keeps the full static layout (backward compatible)", () => {
    const omitted = buildMenuItems(account, unconfiguredOrg, mockT) as Array<{ key: string }>;
    const explicitNull = buildMenuItems(account, unconfiguredOrg, mockT, null) as Array<{
      key: string;
    }>;
    expect(explicitNull.map((item) => item.key)).toEqual(omitted.map((item) => item.key));
    expect(omitted).toHaveLength(menuGroups.length);
  });

  it("hides unauthorized leaves (flat mode keeps only authorized keys)", () => {
    const items = buildMenuItems(account, unconfiguredOrg, mockT, [
      "/",
      "/account",
      "/users",
    ]) as Array<{ key: string }>;
    expect(items.map((item) => item.key)).toEqual(["/", "/users", "/account"]);
  });

  it("drops groups without any authorized leaf (grouped mode)", () => {
    // 8 个可见叶子 > FLAT_MENU_THRESHOLD(7)，进入分组渲染模式。
    const items = buildMenuItems(account, unconfiguredOrg, mockT, [
      "/",
      "/shortcuts",
      "/apps",
      "/oidc-discovery",
      "/users",
      "/organizations",
      "/groups",
      "/account",
    ]) as Array<{ key: string; children?: Array<{ key: string }> }>;
    const keys = items.map((item) => item.key);
    expect(keys).toEqual(["home", "identity", "admin"]);
    const home = items.find((item) => item.key === "home");
    expect(home?.children?.map((child) => child.key)).toEqual([
      "/",
      "/shortcuts",
      "/apps",
      "/oidc-discovery",
    ]);
    const identity = items.find((item) => item.key === "identity");
    expect(identity?.children?.map((child) => child.key)).toEqual([
      "/organizations",
      "/users",
      "/groups",
    ]);
    const admin = items.find((item) => item.key === "admin");
    expect(admin?.children?.map((child) => child.key)).toEqual(["/account"]);
  });

  it("empty allowedMenus hides every leaf", () => {
    const items = buildMenuItems(account, unconfiguredOrg, mockT, []);
    expect(items).toHaveLength(0);
  });

  it("combines with organization nav config (both filters must pass)", () => {
    const organization = {
      navItems: ["/users", "/roles"],
      userNavItems: ["/users", "/roles"],
      navGroupOrder: [],
    } as never;
    const items = buildMenuItems({ isAdmin: false } as never, organization, mockT, [
      "/users",
      "/account",
    ]) as Array<{ key: string; children?: Array<{ key: string }> }>;
    const flatKeys = items.flatMap((item) =>
      item.children ? item.children.map((child) => child.key) : [item.key],
    );
    expect(flatKeys).toContain("/users");
    expect(flatKeys).not.toContain("/roles");
  });
});

describe("searchMenuItems allowedMenus filter", () => {
  const account = { isAdmin: false } as never;
  const unconfiguredOrg = { navItems: [], userNavItems: [], navGroupOrder: [] } as never;

  it("excludes unauthorized leaves from search results", () => {
    const filtered = searchMenuItems(account, unconfiguredOrg, "nav.users", mockT, [
      "/account",
      "/users",
    ]);
    expect(filtered.map((item) => item.key)).toContain("/users");
    const denied = searchMenuItems(account, unconfiguredOrg, "nav.users", mockT, ["/account"]);
    expect(denied.map((item) => item.key)).not.toContain("/users");
  });

  it("null allowedMenus keeps legacy unfiltered behavior", () => {
    const results = searchMenuItems(account, unconfiguredOrg, "nav.users", mockT, null);
    expect(results.map((item) => item.key)).toContain("/users");
  });
});
