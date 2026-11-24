import { describe, it, expect } from "vitest";
import {
  queryKeys,
  resolveCacheStrategy,
  cacheStrategyByPrefix,
  CACHE_STATIC,
  CACHE_SEMI_STATIC,
  CACHE_DYNAMIC,
  CACHE_REALTIME,
  CACHE_EPHEMERAL,
  // Legacy aliases
  CACHE_AUTH,
  CACHE_LIST,
  CACHE_DETAIL,
  CACHE_CONFIG,
} from "./query-keys";

// ─── Cache Duration Presets (4-Tier) ─────────────────────────────────

describe("Cache Duration Presets (4-Tier)", () => {
  it("CACHE_STATIC has correct values (staleTime 30min, gcTime 60min)", () => {
    expect(CACHE_STATIC.staleTime).toBe(30 * 60_000);
    expect(CACHE_STATIC.gcTime).toBe(60 * 60_000);
    expect(CACHE_STATIC.retry).toBe(2);
  });

  it("CACHE_SEMI_STATIC has correct values (staleTime 5min, gcTime 30min)", () => {
    expect(CACHE_SEMI_STATIC.staleTime).toBe(5 * 60_000);
    expect(CACHE_SEMI_STATIC.gcTime).toBe(30 * 60_000);
    expect(CACHE_SEMI_STATIC.retry).toBe(2);
  });

  it("CACHE_DYNAMIC has correct values (staleTime 30s, gcTime 5min)", () => {
    expect(CACHE_DYNAMIC.staleTime).toBe(30_000);
    expect(CACHE_DYNAMIC.gcTime).toBe(5 * 60_000);
    expect(CACHE_DYNAMIC.retry).toBe(2);
  });

  it("CACHE_REALTIME has correct values (staleTime 0, gcTime 2min)", () => {
    expect(CACHE_REALTIME.staleTime).toBe(0);
    expect(CACHE_REALTIME.gcTime).toBe(2 * 60_000);
    expect(CACHE_REALTIME.retry).toBe(1);
  });

  it("Legacy aliases map to correct tiers", () => {
    expect(CACHE_AUTH).toEqual(CACHE_SEMI_STATIC);
    expect(CACHE_LIST).toEqual(CACHE_DYNAMIC);
    expect(CACHE_DETAIL).toEqual(CACHE_SEMI_STATIC);
    expect(CACHE_CONFIG).toEqual(CACHE_STATIC);
  });

  // The transcript read face must leave nothing behind: a gcTime above zero
  // keeps a response that may contain credentials in memory after the operator
  // has navigated away, which is the state 42.3.13 条 5 forbids.
  it("CACHE_EPHEMERAL retains nothing and does not retry", () => {
    expect(CACHE_EPHEMERAL.staleTime).toBe(0);
    expect(CACHE_EPHEMERAL.gcTime).toBe(0);
    expect(CACHE_EPHEMERAL.retry).toBe(0);
  });
});

// ─── Query Key Factory: Session ─────────────────────────────────

describe("queryKeys.session", () => {
  it("all returns correct key", () => {
    expect(queryKeys.session.all).toEqual(["session"]);
  });
});

// ─── Query Key Factory: Auth ────────────────────────────────────

describe("queryKeys.auth", () => {
  it("all returns correct key", () => {
    expect(queryKeys.auth.all).toEqual(["auth"]);
  });

  it("permissions returns correct key", () => {
    expect(queryKeys.auth.permissions()).toEqual(["auth", "permissions"]);
  });
});

// ─── Query Key Factory: Users ───────────────────────────────────

describe("queryKeys.users", () => {
  it("all returns correct key", () => {
    expect(queryKeys.users.all).toEqual(["users"]);
  });

  it("lists returns correct key", () => {
    expect(queryKeys.users.lists()).toEqual(["users", "list"]);
  });

  it("list returns key with params", () => {
    const params = { page: 1, search: "test" };
    expect(queryKeys.users.list(params)).toEqual(["users", "list", params]);
  });

  it("paginated returns correct key", () => {
    expect(queryKeys.users.paginated()).toEqual(["users", "paginated"]);
  });

  it("paginatedWithParams returns key with params", () => {
    const params = { page: 2 };
    expect(queryKeys.users.paginatedWithParams(params)).toEqual(["users", "paginated", params]);
  });

  it("detail returns key with id", () => {
    expect(queryKeys.users.detail("user-1")).toEqual(["users", "detail", "user-1"]);
  });

  it("auth returns correct key", () => {
    expect(queryKeys.users.auth()).toEqual(["users", "auth"]);
  });

  it("maskedContact returns key with username", () => {
    expect(queryKeys.users.maskedContact("admin")).toEqual(["users", "masked-contact", "admin"]);
  });
});

// ─── Query Key Factory: Organizations ───────────────────────────

describe("queryKeys.organizations", () => {
  it("all returns correct key", () => {
    expect(queryKeys.organizations.all).toEqual(["organizations"]);
  });

  it("lists returns correct key", () => {
    expect(queryKeys.organizations.lists()).toEqual(["organizations", "list"]);
  });

  it("list returns key with params", () => {
    expect(queryKeys.organizations.list({ search: "org" })).toEqual([
      "organizations",
      "list",
      { search: "org" },
    ]);
  });

  it("detail returns key with id", () => {
    expect(queryKeys.organizations.detail("org-1")).toEqual(["organizations", "detail", "org-1"]);
  });

  it("identity returns key with slug", () => {
    expect(queryKeys.organizations.identity("my-org")).toEqual([
      "organizations",
      "auth",
      "identity",
      "my-org",
    ]);
  });
});

// ─── Query Key Factory: Roles ───────────────────────────────────

describe("queryKeys.roles", () => {
  it("all returns correct key", () => {
    expect(queryKeys.roles.all).toEqual(["roles"]);
  });

  it("options returns key with orgId", () => {
    expect(queryKeys.roles.options("org-1")).toEqual(["roles", "options", "org-1"]);
  });

  it("options returns key without orgId", () => {
    expect(queryKeys.roles.options()).toEqual(["roles", "options", undefined]);
  });
});

// ─── Query Key Factory: Applications ────────────────────────────

describe("queryKeys.applications", () => {
  it("all returns correct key", () => {
    expect(queryKeys.applications.all).toEqual(["applications"]);
  });

  it("deleteSummary returns key with id", () => {
    expect(queryKeys.applications.deleteSummary("app-1")).toEqual([
      "applications",
      "delete-summary",
      "app-1",
    ]);
  });

  it("options returns key with orgId", () => {
    expect(queryKeys.applications.options("org-1")).toEqual([
      "applications",
      "auth",
      "options",
      "org-1",
    ]);
  });
});

// ─── Query Key Factory: Providers ───────────────────────────────

describe("queryKeys.providers", () => {
  it("all returns correct key", () => {
    expect(queryKeys.providers.all).toEqual(["providers"]);
  });

  it("lists returns correct key", () => {
    expect(queryKeys.providers.lists()).toEqual(["providers", "list"]);
  });

  it("paginated returns correct key", () => {
    expect(queryKeys.providers.paginated()).toEqual(["providers", "paginated"]);
  });

  it("paginatedWithParams returns key with params", () => {
    const params = { category: "OAuth" };
    expect(queryKeys.providers.paginatedWithParams(params)).toEqual([
      "providers",
      "paginated",
      params,
    ]);
  });

  it("detail returns key with id", () => {
    expect(queryKeys.providers.detail("prov-1")).toEqual(["providers", "detail", "prov-1"]);
  });

  it("payment returns correct key", () => {
    expect(queryKeys.providers.payment()).toEqual(["providers", "payment"]);
  });
});

// ─── Query Key Factory: Tickets ─────────────────────────────────

describe("queryKeys.tickets", () => {
  it("messages returns key with id", () => {
    expect(queryKeys.tickets.messages("ticket-1")).toEqual(["tickets", "messages", "ticket-1"]);
  });
});

// ─── Query Key Factory: Payments ────────────────────────────────

describe("queryKeys.payments", () => {
  it("result returns key with id", () => {
    expect(queryKeys.payments.result("pay-1")).toEqual(["payments", "iam", "result", "pay-1"]);
  });
});

// ─── Query Key Factory: Webhook Events ──────────────────────────

describe("queryKeys.webhookEvents", () => {
  it("all returns correct key", () => {
    expect(queryKeys.webhookEvents.all).toEqual(["webhook-events"]);
  });

  it("iamWithWebhook returns key with webhookId and params", () => {
    expect(queryKeys.webhookEvents.iamWithWebhook("wh-1", { page: 1 })).toEqual([
      "webhook-events",
      "iam",
      "wh-1",
      { page: 1 },
    ]);
  });

  it("iamWithWebhook returns key without params", () => {
    expect(queryKeys.webhookEvents.iamWithWebhook("wh-1")).toEqual([
      "webhook-events",
      "iam",
      "wh-1",
      undefined,
    ]);
  });
});

// ─── Query Key Factory: Transactions ────────────────────────────

describe("queryKeys.transactions", () => {
  it("org returns key with orgId", () => {
    expect(queryKeys.transactions.org("org-1")).toEqual(["transactions", "org", "org-1"]);
  });
});

// ─── Query Key Factory: Records ─────────────────────────────────

describe("queryKeys.records", () => {
  it("auth returns key with params", () => {
    expect(queryKeys.records.auth({ userId: "u1" })).toEqual(["records", "auth", { userId: "u1" }]);
  });

  it("auth returns key without params", () => {
    expect(queryKeys.records.auth()).toEqual(["records", "auth", undefined]);
  });
});

// ─── Query Key Hierarchy (Invalidation) ─────────────────────────

describe("Query Key Hierarchy", () => {
  it("users.lists() is a prefix of users.list(params)", () => {
    const listsKey = queryKeys.users.lists();
    const listKey = queryKeys.users.list({ page: 1 });
    // listKey should start with all elements of listsKey
    expect(listKey.slice(0, listsKey.length)).toEqual(listsKey);
  });

  it("users.all is a prefix of users.lists()", () => {
    const allKey = queryKeys.users.all;
    const listsKey = queryKeys.users.lists();
    expect(listsKey.slice(0, allKey.length)).toEqual(allKey);
  });

  it("users.all is a prefix of users.detail(id)", () => {
    const allKey = queryKeys.users.all;
    const detailKey = queryKeys.users.detail("user-1");
    expect(detailKey.slice(0, allKey.length)).toEqual(allKey);
  });

  it("providers.all is a prefix of providers.paginated()", () => {
    const allKey = queryKeys.providers.all;
    const paginatedKey = queryKeys.providers.paginated();
    expect(paginatedKey.slice(0, allKey.length)).toEqual(allKey);
  });

  it("organizations.all is a prefix of organizations.detail(id)", () => {
    const allKey = queryKeys.organizations.all;
    const detailKey = queryKeys.organizations.detail("org-1");
    expect(detailKey.slice(0, allKey.length)).toEqual(allKey);
  });

  // Write paths invalidate `X.all` after create/delete; the list must be a
  // descendant of that root or the invalidation silently misses it (this is how
  // `["users", "auth"]` used to leave the user list stale).
  const LIST_KEY_PAIRS = [
    {
      name: "users",
      all: queryKeys.users.all,
      list: queryKeys.users.paginatedWithParams({ page: 1 }),
    },
    {
      name: "organizations",
      all: queryKeys.organizations.all,
      list: queryKeys.organizations.paginatedWithParams({ page: 1 }),
    },
    { name: "roles", all: queryKeys.roles.all, list: queryKeys.roles.lists() },
    { name: "groups", all: queryKeys.groups.all, list: queryKeys.groups.lists() },
    { name: "permissions", all: queryKeys.permissions.all, list: queryKeys.permissions.lists() },
    {
      name: "plans",
      all: queryKeys.plans.all,
      list: queryKeys.plans.paginatedWithParams({ page: 1 }),
    },
    {
      name: "pricings",
      all: queryKeys.pricings.all,
      list: queryKeys.pricings.paginatedWithParams({ page: 1 }),
    },
    {
      name: "ldaps",
      all: queryKeys.ldaps.all,
      list: queryKeys.ldaps.paginatedWithParams({ page: 1 }),
    },
  ];

  it.each(LIST_KEY_PAIRS)("$name.all is a prefix of its list key", ({ all, list }) => {
    expect(list.slice(0, all.length)).toEqual(all);
  });
});

// ─── resolveCacheStrategy ───────────────────────────────────────

describe("resolveCacheStrategy", () => {
  // Tier 1: Static
  it("returns CACHE_STATIC for organizations keys", () => {
    expect(resolveCacheStrategy(["organizations"])).toEqual(CACHE_STATIC);
  });

  it("returns CACHE_STATIC for permissions keys", () => {
    expect(resolveCacheStrategy(["permissions"])).toEqual(CACHE_STATIC);
  });

  it("returns CACHE_STATIC for roles keys", () => {
    expect(resolveCacheStrategy(["roles"])).toEqual(CACHE_STATIC);
  });

  it("returns CACHE_STATIC for i18n keys", () => {
    expect(resolveCacheStrategy(["i18n"])).toEqual(CACHE_STATIC);
  });

  it("returns CACHE_STATIC for system-info keys", () => {
    expect(resolveCacheStrategy(["system-info"])).toEqual(CACHE_STATIC);
  });

  // Tier 2: Semi-static
  it("returns CACHE_SEMI_STATIC for session keys", () => {
    expect(resolveCacheStrategy(["session"])).toEqual(CACHE_SEMI_STATIC);
  });

  it("returns CACHE_SEMI_STATIC for auth keys", () => {
    expect(resolveCacheStrategy(["auth"])).toEqual(CACHE_SEMI_STATIC);
  });

  it("returns CACHE_SEMI_STATIC for providers keys", () => {
    expect(resolveCacheStrategy(["providers"])).toEqual(CACHE_SEMI_STATIC);
  });

  it("returns CACHE_SEMI_STATIC for applications keys", () => {
    expect(resolveCacheStrategy(["applications"])).toEqual(CACHE_SEMI_STATIC);
  });

  it("returns CACHE_SEMI_STATIC for settings keys", () => {
    expect(resolveCacheStrategy(["settings"])).toEqual(CACHE_SEMI_STATIC);
  });

  it("returns CACHE_SEMI_STATIC for enforcers keys", () => {
    expect(resolveCacheStrategy(["enforcers"])).toEqual(CACHE_SEMI_STATIC);
  });

  it("returns CACHE_SEMI_STATIC for adapters keys", () => {
    expect(resolveCacheStrategy(["adapters"])).toEqual(CACHE_SEMI_STATIC);
  });

  // Tier 3: Dynamic
  it("returns CACHE_DYNAMIC for users keys", () => {
    expect(resolveCacheStrategy(["users"])).toEqual(CACHE_DYNAMIC);
  });

  it("returns CACHE_DYNAMIC for tickets keys", () => {
    expect(resolveCacheStrategy(["tickets"])).toEqual(CACHE_DYNAMIC);
  });

  it("returns CACHE_DYNAMIC for payments keys", () => {
    expect(resolveCacheStrategy(["payments"])).toEqual(CACHE_DYNAMIC);
  });

  it("returns CACHE_DYNAMIC for orders keys", () => {
    expect(resolveCacheStrategy(["orders"])).toEqual(CACHE_DYNAMIC);
  });

  it("returns CACHE_DYNAMIC for subscriptions keys", () => {
    expect(resolveCacheStrategy(["subscriptions"])).toEqual(CACHE_DYNAMIC);
  });

  // Tier 4: Real-time
  it("returns CACHE_REALTIME for dashboard keys", () => {
    expect(resolveCacheStrategy(["dashboard"])).toEqual(CACHE_REALTIME);
  });

  it("returns CACHE_REALTIME for records keys", () => {
    expect(resolveCacheStrategy(["records"])).toEqual(CACHE_REALTIME);
  });

  it("returns CACHE_REALTIME for webhookEvents keys", () => {
    expect(resolveCacheStrategy(["webhook-events"])).toEqual(CACHE_REALTIME);
  });

  // Fallback
  it("returns CACHE_DYNAMIC as fallback for unknown keys", () => {
    expect(resolveCacheStrategy(["unknown-resource"])).toEqual(CACHE_DYNAMIC);
  });

  it("returns CACHE_DYNAMIC as fallback for empty key", () => {
    expect(resolveCacheStrategy([])).toEqual(CACHE_DYNAMIC);
  });

  it("returns CACHE_STATIC for plans keys", () => {
    expect(resolveCacheStrategy(["plans"])).toEqual(CACHE_STATIC);
  });
});

// ─── Cache strategy coverage (both directions) ──────────────────────
//
// `resolveCacheStrategy` falls back to CACHE_DYNAMIC for an unknown prefix, so a
// group whose wire prefix is missing from the table is indistinguishable from a
// group that deliberately chose CACHE_DYNAMIC — it just silently gets the wrong
// tier. The table also used camelCase aliases for kebab-case prefixes, which can
// never match and hid the gap. These two tests close both directions.
describe("cache strategy coverage", () => {
  const wirePrefix = (group: Record<string, unknown>): string | undefined => {
    const all = group.all;
    return Array.isArray(all) && typeof all[0] === "string" ? all[0] : undefined;
  };

  const groups = Object.entries(queryKeys)
    .map(([name, group]) => [name, wirePrefix(group as Record<string, unknown>)] as const)
    .filter((entry): entry is readonly [string, string] => entry[1] !== undefined);

  it("declares a strategy for every query key group", () => {
    const uncovered = groups
      .filter(([, prefix]) => !(prefix in cacheStrategyByPrefix))
      .map(([name, prefix]) => `${name} (${prefix})`);

    expect(uncovered).toEqual([]);
  });

  it("has no strategy key that no group produces", () => {
    const produced = new Set(groups.map(([, prefix]) => prefix));
    const unused = Object.keys(cacheStrategyByPrefix).filter((key) => !produced.has(key));

    expect(unused).toEqual([]);
  });

  it("resolves each group to its declared preset", () => {
    for (const [, prefix] of groups) {
      expect(resolveCacheStrategy([prefix]), `prefix ${prefix}`).toEqual(
        cacheStrategyByPrefix[prefix],
      );
    }
  });
});
