/**
 * Centralized Query Key Factory for TanStack Query.
 *
 * Follows the hierarchical pattern:
 *   resource.all → resource.lists → resource.list(params) → resource.detail(id)
 *
 * This ensures that invalidating a parent key (e.g. users.all)
 * correctly invalidates all child queries (lists + details).
 *
 * @see https://tanstack.com/query/latest/docs/react/guides/query-keys
 */

// ─── Cache Duration Presets ─────────────────────────────────────
// Tier 1: Static data (organization list, permission definitions) — rarely changes
export const CACHE_STATIC = { staleTime: 30 * 60_000, gcTime: 60 * 60_000, retry: 2 } as const;
// Tier 2: Semi-static data (application config, role definitions) — changes occasionally
export const CACHE_SEMI_STATIC = { staleTime: 5 * 60_000, gcTime: 30 * 60_000, retry: 2 } as const;
// Tier 3: Dynamic data (user list, sessions) — changes frequently
export const CACHE_DYNAMIC = { staleTime: 30_000, gcTime: 5 * 60_000, retry: 2 } as const;
// Tier 4: Real-time data (audit records, online status) — always fresh
export const CACHE_REALTIME = { staleTime: 0, gcTime: 2 * 60_000, retry: 1 } as const;
// Tier 5: Not cacheable (reads that may contain live credentials, one-time
// tokens). Nothing is retained after the observer unmounts, and a failed read is
// not retried behind the operator's back — a retry is an explicit action there.
export const CACHE_EPHEMERAL = { staleTime: 0, gcTime: 0, retry: 0 } as const;

// ─── Legacy aliases (backward compatibility) ────────────────────
// Auth/session data: maps to semi-static tier
export const CACHE_AUTH = CACHE_SEMI_STATIC;
// List data: maps to dynamic tier
export const CACHE_LIST = CACHE_DYNAMIC;
// Detail data: maps to semi-static tier
export const CACHE_DETAIL = CACHE_SEMI_STATIC;
// Config/reference data: maps to static tier
export const CACHE_CONFIG = CACHE_STATIC;

// ─── Query Key Factory ─────────────────────────────────────────

export const queryKeys = {
  // Session / Auth
  session: {
    all: ["session"] as const,
  },

  // Auth (permissions, etc.)
  auth: {
    all: ["auth"] as const,
    permissions: () => [...queryKeys.auth.all, "permissions"] as const,
  },

  // User detail page primary query (singular — distinct from the `users`
  // list group). Keyed by route params (organization slug + username), not
  // the user id: the detail route is /users/:organizationName/:userName.
  // Writers that mutate user data must invalidate the ["user"] prefix.
  user: {
    all: ["user"] as const,
    byPath: (organizationSlug?: string, username?: string) =>
      [...queryKeys.user.all, "path", organizationSlug, username] as const,
  },

  // Users
  users: {
    all: ["users"] as const,
    lists: () => [...queryKeys.users.all, "list"] as const,
    list: (params: unknown) => [...queryKeys.users.lists(), params] as const,
    paginated: () => [...queryKeys.users.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.users.paginated(), params] as const,
    detail: (id: string) => [...queryKeys.users.all, "detail", id] as const,
    auth: () => [...queryKeys.users.all, "auth"] as const,
    maskedContact: (username: string) =>
      [...queryKeys.users.all, "masked-contact", username] as const,
    webauthnCredentials: (userId: string, organizationId?: string) =>
      [...queryKeys.users.all, "webauthn-credentials", userId, organizationId ?? ""] as const,
    webauthnCredential: (userId: string, credentialId: string, organizationId?: string) =>
      [...queryKeys.users.webauthnCredentials(userId, organizationId), credentialId] as const,
  },

  // Organizations
  organizations: {
    all: ["organizations"] as const,
    lists: () => [...queryKeys.organizations.all, "list"] as const,
    list: (params: unknown) => [...queryKeys.organizations.lists(), params] as const,
    paginated: () => [...queryKeys.organizations.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) =>
      [...queryKeys.organizations.paginated(), params] as const,
    detail: (id: string) => [...queryKeys.organizations.all, "detail", id] as const,
    auth: () => [...queryKeys.organizations.all, "auth"] as const,
    identity: (slug: string) => [...queryKeys.organizations.all, "auth", "identity", slug] as const,
  },

  // Roles
  roles: {
    all: ["roles"] as const,
    lists: () => [...queryKeys.roles.all, "list"] as const,
    list: (params: unknown) => [...queryKeys.roles.lists(), params] as const,
    detail: (id: string) => [...queryKeys.roles.all, "detail", id] as const,
    auth: () => [...queryKeys.roles.all, "auth"] as const,
    options: (orgId?: string) => [...queryKeys.roles.all, "options", orgId] as const,
  },

  // Permissions
  permissions: {
    all: ["permissions"] as const,
    lists: () => [...queryKeys.permissions.all, "list"] as const,
    list: (params: unknown) => [...queryKeys.permissions.lists(), params] as const,
    detail: (id: string) => [...queryKeys.permissions.all, "detail", id] as const,
    auth: () => [...queryKeys.permissions.all, "auth"] as const,
  },

  // Groups
  groups: {
    all: ["groups"] as const,
    lists: () => [...queryKeys.groups.all, "list"] as const,
    list: (params: unknown) => [...queryKeys.groups.lists(), params] as const,
    detail: (id: string) => [...queryKeys.groups.all, "detail", id] as const,
    auth: () => [...queryKeys.groups.all, "auth"] as const,
  },

  // Applications
  applications: {
    all: ["applications"] as const,
    lists: () => [...queryKeys.applications.all, "list"] as const,
    list: (params: unknown) => [...queryKeys.applications.lists(), params] as const,
    paginated: () => [...queryKeys.applications.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) =>
      [...queryKeys.applications.paginated(), params] as const,
    detail: (id: string) => [...queryKeys.applications.all, "detail", id] as const,
    deleteSummary: (id: string) => [...queryKeys.applications.all, "delete-summary", id] as const,
    clientCertificates: (id: string) =>
      [...queryKeys.applications.all, "client-certificates", id] as const,
    auth: () => [...queryKeys.applications.all, "auth"] as const,
    options: (orgId?: string) => [...queryKeys.applications.all, "auth", "options", orgId] as const,
  },

  // B2B Domain governance (MODULE-B2B)
  b2bDomains: {
    all: ["b2b", "domains"] as const,
    list: (params: unknown) => [...queryKeys.b2bDomains.all, "list", params] as const,
  },

  // Providers
  providers: {
    all: ["providers"] as const,
    lists: () => [...queryKeys.providers.all, "list"] as const,
    paginated: () => [...queryKeys.providers.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.providers.paginated(), params] as const,
    detail: (id: string) => [...queryKeys.providers.all, "detail", id] as const,
    payment: () => [...queryKeys.providers.all, "payment"] as const,
  },

  // Dashboard
  dashboard: {
    all: ["dashboard"] as const,
  },

  // Settings
  settings: {
    all: ["settings"] as const,
  },

  // Tickets
  tickets: {
    all: ["tickets"] as const,
    paginated: () => [...queryKeys.tickets.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.tickets.paginated(), params] as const,
    detail: (id: string) => [...queryKeys.tickets.all, "detail", id] as const,
    iam: () => [...queryKeys.tickets.all, "iam"] as const,
    messages: (id: string) => [...queryKeys.tickets.all, "messages", id] as const,
  },

  // Payments
  payments: {
    all: ["payments"] as const,
    paginated: () => [...queryKeys.payments.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.payments.paginated(), params] as const,
    iam: () => [...queryKeys.payments.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.payments.all, "detail", id] as const,
    result: (id: string) => [...queryKeys.payments.all, "iam", "result", id] as const,
  },

  // Products
  products: {
    all: ["products"] as const,
    paginated: () => [...queryKeys.products.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.products.paginated(), params] as const,
    iam: () => [...queryKeys.products.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.products.all, "detail", id] as const,
    store: () => [...queryKeys.products.all, "store"] as const,
  },

  // Orders
  orders: {
    all: ["orders"] as const,
    paginated: () => [...queryKeys.orders.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.orders.paginated(), params] as const,
    iam: () => [...queryKeys.orders.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.orders.all, "detail", id] as const,
  },

  // Subscriptions
  subscriptions: {
    all: ["subscriptions"] as const,
    paginated: () => [...queryKeys.subscriptions.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) =>
      [...queryKeys.subscriptions.paginated(), params] as const,
    iam: () => [...queryKeys.subscriptions.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.subscriptions.all, "detail", id] as const,
  },

  // Plans
  plans: {
    all: ["plans"] as const,
    paginated: () => [...queryKeys.plans.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.plans.paginated(), params] as const,
    iam: () => [...queryKeys.plans.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.plans.all, "detail", id] as const,
  },

  // Pricings
  pricings: {
    all: ["pricings"] as const,
    paginated: () => [...queryKeys.pricings.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.pricings.paginated(), params] as const,
    iam: () => [...queryKeys.pricings.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.pricings.all, "detail", id] as const,
  },

  // Models
  models: {
    all: ["models"] as const,
    paginated: () => [...queryKeys.models.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.models.paginated(), params] as const,
    iam: () => [...queryKeys.models.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.models.all, "detail", id] as const,
  },

  // Adapters
  adapters: {
    all: ["adapters"] as const,
    paginated: () => [...queryKeys.adapters.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.adapters.paginated(), params] as const,
    iam: () => [...queryKeys.adapters.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.adapters.all, "detail", id] as const,
  },

  // Enforcers
  enforcers: {
    all: ["enforcers"] as const,
    paginated: () => [...queryKeys.enforcers.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.enforcers.paginated(), params] as const,
    iam: () => [...queryKeys.enforcers.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.enforcers.all, "detail", id] as const,
  },

  // Resources
  resources: {
    all: ["resources"] as const,
    paginated: () => [...queryKeys.resources.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.resources.paginated(), params] as const,
    iam: () => [...queryKeys.resources.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.resources.all, "detail", id] as const,
  },

  // Syncers
  syncers: {
    all: ["syncers"] as const,
    paginated: () => [...queryKeys.syncers.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.syncers.paginated(), params] as const,
    iam: () => [...queryKeys.syncers.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.syncers.all, "detail", id] as const,
  },

  // Forms
  forms: {
    all: ["forms"] as const,
    paginated: () => [...queryKeys.forms.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.forms.paginated(), params] as const,
    iam: () => [...queryKeys.forms.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.forms.all, "detail", id] as const,
  },

  // Agents
  agents: {
    all: ["agents"] as const,
    paginated: () => [...queryKeys.agents.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.agents.paginated(), params] as const,
    iam: () => [...queryKeys.agents.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.agents.all, "detail", id] as const,
  },

  // Webhooks
  webhooks: {
    all: ["webhooks"] as const,
    paginated: () => [...queryKeys.webhooks.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.webhooks.paginated(), params] as const,
    iam: () => [...queryKeys.webhooks.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.webhooks.all, "detail", id] as const,
  },

  // Webhook Events
  webhookEvents: {
    all: ["webhook-events"] as const,
    iam: () => [...queryKeys.webhookEvents.all, "iam"] as const,
    iamWithWebhook: (webhookId: string, params?: unknown) =>
      [...queryKeys.webhookEvents.iam(), webhookId, params] as const,
  },

  // Notification Providers
  notificationProviders: {
    all: ["notification-providers"] as const,
    paginated: () => [...queryKeys.notificationProviders.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) =>
      [...queryKeys.notificationProviders.paginated(), params] as const,
    iam: () => [...queryKeys.notificationProviders.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.notificationProviders.all, "detail", id] as const,
  },

  // Captcha Providers
  captchaProviders: {
    all: ["captcha-providers"] as const,
    paginated: () => [...queryKeys.captchaProviders.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) =>
      [...queryKeys.captchaProviders.paginated(), params] as const,
    iam: () => [...queryKeys.captchaProviders.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.captchaProviders.all, "detail", id] as const,
  },

  // Radius Configs
  radiusConfigs: {
    all: ["radius-configs"] as const,
    paginated: () => [...queryKeys.radiusConfigs.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) =>
      [...queryKeys.radiusConfigs.paginated(), params] as const,
    iam: () => [...queryKeys.radiusConfigs.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.radiusConfigs.all, "detail", id] as const,
  },

  // Radius Accountings
  radiusAccountings: {
    all: ["radius-accountings"] as const,
    paginated: () => [...queryKeys.radiusAccountings.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) =>
      [...queryKeys.radiusAccountings.paginated(), params] as const,
    iam: () => [...queryKeys.radiusAccountings.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.radiusAccountings.all, "detail", id] as const,
  },

  // Entries
  entries: {
    all: ["entries"] as const,
    paginated: () => [...queryKeys.entries.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.entries.paginated(), params] as const,
    iam: () => [...queryKeys.entries.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.entries.all, "detail", id] as const,
  },

  // Entry transcript (FE-015). It is its own group rather than a child of
  // `entries` because the cache tier has to differ: the transcript may contain
  // credentials, so the console must not keep a copy (42.3.13 条 5), while the
  // entry list itself is semi-static. A child key cannot resolve to a different
  // tier — `resolveCacheStrategy` only sees `queryKey[0]`.
  entryTranscript: {
    all: ["entry-transcript"] as const,
    page: (entryId: string) => [...queryKeys.entryTranscript.all, entryId] as const,
  },

  // Transactions
  transactions: {
    all: ["transactions"] as const,
    paginated: () => [...queryKeys.transactions.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) =>
      [...queryKeys.transactions.paginated(), params] as const,
    org: (orgId: string) => [...queryKeys.transactions.all, "org", orgId] as const,
    iam: () => [...queryKeys.transactions.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.transactions.all, "detail", id] as const,
  },

  // LDAPs
  ldaps: {
    all: ["ldaps"] as const,
    paginated: () => [...queryKeys.ldaps.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.ldaps.paginated(), params] as const,
    org: (orgId: string) => [...queryKeys.ldaps.all, "org", orgId] as const,
    iam: () => [...queryKeys.ldaps.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.ldaps.all, "detail", id] as const,
  },

  // Certs
  certs: {
    all: ["certs"] as const,
    paginated: () => [...queryKeys.certs.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.certs.paginated(), params] as const,
    iam: () => [...queryKeys.certs.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.certs.all, "detail", id] as const,
  },

  // Records
  records: {
    all: ["records"] as const,
    paginated: () => [...queryKeys.records.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.records.paginated(), params] as const,
    auth: (params?: unknown) => [...queryKeys.records.all, "auth", params] as const,
  },

  // Sessions (user sessions)
  userSessions: {
    all: ["sessions"] as const,
    paginated: () => [...queryKeys.userSessions.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) =>
      [...queryKeys.userSessions.paginated(), params] as const,
    iam: () => [...queryKeys.userSessions.all, "iam"] as const,
  },

  // Tokens
  tokens: {
    all: ["tokens"] as const,
    paginated: () => [...queryKeys.tokens.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.tokens.paginated(), params] as const,
    iam: () => [...queryKeys.tokens.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.tokens.all, "detail", id] as const,
  },

  // Invitations
  invitations: {
    all: ["invitations"] as const,
    paginated: () => [...queryKeys.invitations.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) =>
      [...queryKeys.invitations.paginated(), params] as const,
    iam: () => [...queryKeys.invitations.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.invitations.all, "detail", id] as const,
  },

  // Keys
  keys: {
    all: ["keys"] as const,
    paginated: () => [...queryKeys.keys.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.keys.paginated(), params] as const,
    iam: () => [...queryKeys.keys.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.keys.all, "detail", id] as const,
  },

  // Rules
  rules: {
    all: ["rules"] as const,
    paginated: () => [...queryKeys.rules.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.rules.paginated(), params] as const,
    iam: () => [...queryKeys.rules.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.rules.all, "detail", id] as const,
  },

  // Coupons
  coupons: {
    all: ["coupons"] as const,
    paginated: () => [...queryKeys.coupons.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.coupons.paginated(), params] as const,
    iam: () => [...queryKeys.coupons.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.coupons.all, "detail", id] as const,
  },

  // MCP Servers
  mcpServers: {
    all: ["mcp-servers"] as const,
    paginated: () => [...queryKeys.mcpServers.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) =>
      [...queryKeys.mcpServers.paginated(), params] as const,
    iam: () => [...queryKeys.mcpServers.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.mcpServers.all, "detail", id] as const,
    store: () => [...queryKeys.mcpServers.all, "store"] as const,
  },

  // Face ID Providers
  faceIdProviders: {
    all: ["face-id-providers"] as const,
    paginated: () => [...queryKeys.faceIdProviders.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) =>
      [...queryKeys.faceIdProviders.paginated(), params] as const,
    iam: () => [...queryKeys.faceIdProviders.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.faceIdProviders.all, "detail", id] as const,
  },

  // Verifications
  verifications: {
    all: ["verifications"] as const,
    paginated: () => [...queryKeys.verifications.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) =>
      [...queryKeys.verifications.paginated(), params] as const,
  },

  // Gateway / Servers
  servers: {
    all: ["servers"] as const,
    paginated: () => [...queryKeys.servers.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.servers.paginated(), params] as const,
    iam: () => [...queryKeys.servers.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.servers.all, "detail", id] as const,
  },

  // Gateway / Sites
  sites: {
    all: ["sites"] as const,
    paginated: () => [...queryKeys.sites.all, "paginated"] as const,
    paginatedWithParams: (params: unknown) => [...queryKeys.sites.paginated(), params] as const,
    iam: () => [...queryKeys.sites.all, "iam"] as const,
    detail: (id: string) => [...queryKeys.sites.all, "detail", id] as const,
  },

  // Notifications
  notifications: {
    all: ["notifications"] as const,
  },

  // i18n
  i18n: {
    all: ["i18n"] as const,
  },

  // System Info
  systemInfo: {
    all: ["system-info"] as const,
  },

  // ── Target domains (Step 4 schema) ────────────────────────────

  // ReBAC (relationship-based access control)
  rebac: {
    all: ["rebac"] as const,
    namespaces: () => [...queryKeys.rebac.all, "namespaces"] as const,
    tuples: (namespaceId: string) => [...queryKeys.rebac.all, "tuples", namespaceId] as const,
  },

  // Workload Identity
  "identity-pools": {
    all: ["identity-pools"] as const,
    organization: (organizationId: string) =>
      [...queryKeys["identity-pools"].all, "organization", organizationId] as const,
    lists: (organizationId: string) =>
      [...queryKeys["identity-pools"].organization(organizationId), "list"] as const,
    list: (organizationId: string, params: unknown) =>
      [...queryKeys["identity-pools"].lists(organizationId), params] as const,
    detail: (organizationId: string, id: string) =>
      [...queryKeys["identity-pools"].organization(organizationId), "detail", id] as const,
    "federation-trusts": (organizationId: string, poolId: string) =>
      [
        ...queryKeys["identity-pools"].organization(organizationId),
        "pool",
        poolId,
        "federation-trusts",
      ] as const,
    "federation-trust": (organizationId: string, poolId: string, id: string) =>
      [...queryKeys["identity-pools"]["federation-trusts"](organizationId, poolId), id] as const,
    "service-accounts": (organizationId: string, poolId: string) =>
      [
        ...queryKeys["identity-pools"].organization(organizationId),
        "pool",
        poolId,
        "service-accounts",
      ] as const,
    "service-account": (organizationId: string, poolId: string, id: string) =>
      [...queryKeys["identity-pools"]["service-accounts"](organizationId, poolId), id] as const,
    "service-account-delete-precheck": (organizationId: string, poolId: string, id: string) =>
      [
        ...queryKeys["identity-pools"]["service-account"](organizationId, poolId, id),
        "delete-precheck",
      ] as const,
  },

  // Metering
  metering: {
    all: ["metering"] as const,
    events: (params: unknown) => [...queryKeys.metering.all, "events", params] as const,
    aggregates: (params: unknown) => [...queryKeys.metering.all, "aggregates", params] as const,
  },

  // IGA (Identity Governance & Administration)
  iga: {
    all: ["iga"] as const,
    "access-campaigns": {
      lists: () => [...queryKeys.iga.all, "access-campaigns", "list"] as const,
      detail: (id: string) => [...queryKeys.iga.all, "access-campaigns", "detail", id] as const,
    },
    "sod-rules": {
      lists: () => [...queryKeys.iga.all, "sod-rules", "list"] as const,
      detail: (id: string) => [...queryKeys.iga.all, "sod-rules", "detail", id] as const,
    },
    "access-lifecycle": (params: unknown) =>
      [...queryKeys.iga.all, "access-lifecycle", params] as const,
  },

  // Risk
  risk: {
    all: ["risk"] as const,
    signals: (params: unknown) => [...queryKeys.risk.all, "signals", params] as const,
    decisions: (params: unknown) => [...queryKeys.risk.all, "decisions", params] as const,
    "session-governance": (params: unknown) =>
      [...queryKeys.risk.all, "session-governance", params] as const,
  },

  // Actions (identity automation)
  actions: {
    all: ["actions"] as const,
    lists: () => [...queryKeys.actions.all, "list"] as const,
    list: (params: unknown) => [...queryKeys.actions.lists(), params] as const,
    detail: (id: string) => [...queryKeys.actions.all, "detail", id] as const,
    versions: (actionId: string) => [...queryKeys.actions.all, actionId, "versions"] as const,
    "execution-logs": (actionId: string) =>
      [...queryKeys.actions.all, actionId, "execution-logs"] as const,
  },
} as const;

// ─── Cache Strategy by Query Key Prefix ────────────────────────
// Maps the first element of a queryKey to the appropriate cache preset.
//
// Keys MUST be the wire prefix — the `all: ["…"]` value of a queryKeys group,
// which is what `resolveCacheStrategy` sees as `queryKey[0]`. They are kebab-case
// (`webhook-events`, `system-info`), never the camelCase group name: the
// camelCase aliases that used to live here could never match, so the groups they
// were meant to cover silently fell back to CACHE_DYNAMIC. `queryKeys.test.ts`
// now asserts that every group resolves to its declared tier.
//
// Exported so the test can assert both directions (no uncovered group, no key
// that no group produces); it is static configuration, not runtime state.

type CachePreset = { staleTime: number; gcTime: number; retry: number };

export const cacheStrategyByPrefix: Record<string, CachePreset> = {
  // ── Tier 1: Static data (staleTime 30min, gcTime 60min) ──
  // Organization list & permission definitions rarely change
  organizations: CACHE_STATIC,
  permissions: CACHE_STATIC,
  roles: CACHE_STATIC,
  groups: CACHE_STATIC,
  i18n: CACHE_STATIC,
  "system-info": CACHE_STATIC,
  plans: CACHE_STATIC,
  pricings: CACHE_STATIC,

  // ── Tier 2: Semi-static data (staleTime 5min, gcTime 30min) ──
  // Application config, provider definitions, auth/session data
  session: CACHE_SEMI_STATIC,
  auth: CACHE_SEMI_STATIC,
  applications: CACHE_SEMI_STATIC,
  providers: CACHE_SEMI_STATIC,
  // B2B domain governance is configuration-level, edited rarely.
  b2b: CACHE_SEMI_STATIC,
  "notification-providers": CACHE_SEMI_STATIC,
  "captcha-providers": CACHE_SEMI_STATIC,
  adapters: CACHE_SEMI_STATIC,
  enforcers: CACHE_SEMI_STATIC,
  models: CACHE_SEMI_STATIC,
  syncers: CACHE_SEMI_STATIC,
  resources: CACHE_SEMI_STATIC,
  rules: CACHE_SEMI_STATIC,
  ldaps: CACHE_SEMI_STATIC,
  certs: CACHE_SEMI_STATIC,
  agents: CACHE_SEMI_STATIC,
  forms: CACHE_SEMI_STATIC,
  "radius-configs": CACHE_SEMI_STATIC,
  entries: CACHE_SEMI_STATIC,
  webhooks: CACHE_SEMI_STATIC,
  invitations: CACHE_SEMI_STATIC,
  keys: CACHE_SEMI_STATIC,
  "face-id-providers": CACHE_SEMI_STATIC,
  "mcp-servers": CACHE_SEMI_STATIC,
  settings: CACHE_SEMI_STATIC,
  // Gateway servers/sites are configuration: edited rarely, read often.
  servers: CACHE_SEMI_STATIC,
  sites: CACHE_SEMI_STATIC,
  products: CACHE_SEMI_STATIC,
  coupons: CACHE_SEMI_STATIC,
  // ReBAC namespaces and workload identity pools are configuration assets.
  rebac: CACHE_SEMI_STATIC,
  "identity-pools": CACHE_SEMI_STATIC,
  // Action definitions change rarely; execution logs are tiered separately.
  actions: CACHE_SEMI_STATIC,

  // ── Tier 3: Dynamic data (staleTime 30s, gcTime 5min) ──
  // User lists, sessions, transactions — changes frequently
  user: CACHE_DYNAMIC,
  users: CACHE_DYNAMIC,
  tickets: CACHE_DYNAMIC,
  payments: CACHE_DYNAMIC,
  transactions: CACHE_DYNAMIC,
  orders: CACHE_DYNAMIC,
  sessions: CACHE_DYNAMIC,
  tokens: CACHE_DYNAMIC,
  subscriptions: CACHE_DYNAMIC,
  // Verification codes are issued and consumed continuously.
  verifications: CACHE_DYNAMIC,
  // The notification log is append-only and polled by the console.
  notifications: CACHE_DYNAMIC,
  // RADIUS accounting rows accumulate per session.
  "radius-accountings": CACHE_DYNAMIC,
  // Metering aggregates are rolled up periodically; IGA lifecycle evolves.
  metering: CACHE_DYNAMIC,
  iga: CACHE_DYNAMIC,

  // ── Tier 4: Real-time data (staleTime 0, gcTime 2min) ──
  // Audit records, online status, dashboard — always refetch
  records: CACHE_REALTIME,
  dashboard: CACHE_REALTIME,
  "webhook-events": CACHE_REALTIME,
  // Risk signals and session governance are security-sensitive and volatile.
  risk: CACHE_REALTIME,

  // ── Tier 5: Not cacheable (staleTime 0, gcTime 0, no retry) ──
  // A read face that may contain live credentials must not outlive the
  // redaction that produced it (42.3.13 条 5), and neither may a one-time
  // secret surface (FE-015 / FE-016).
  "entry-transcript": CACHE_EPHEMERAL,
};

/**
 * Resolve cache strategy (staleTime, gcTime) for a given queryKey.
 * Falls back to CACHE_DYNAMIC (Tier 3) for unknown keys.
 */
export function resolveCacheStrategy(queryKey: readonly unknown[]): CachePreset {
  const prefix = String(queryKey[0] ?? "");
  return cacheStrategyByPrefix[prefix] ?? CACHE_DYNAMIC;
}
