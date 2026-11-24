/**
 * Closed value sets for enum-shaped API fields.
 *
 * These lists mirror the server's closed sets (手册 4.14.3): each one is declared
 * in Go under `internal/domain/<module>` and enforced on write, so an unknown
 * value comes back as 422. A list here that offers a value the server does not
 * accept produces a form the user cannot submit, and a list that omits a value
 * the server does accept hides a supported capability — both are defects, which
 * is why `scripts/check-enums.py` compares the two sides on every run.
 *
 * The `@enumset <name>` tag links a list to its Go set. The name must match the
 * identifier passed to `enumset.NewSet` in the server code; the gate resolves it
 * and fails on any difference in either direction.
 *
 * A list whose values are shown to a user also carries `captions=<ns>:<group>`.
 * That declares where each value's caption lives, so a value added to the server
 * set without a caption in one of the language packs is a gate failure rather
 * than an untranslated cell nobody notices until a customer reports it.
 *
 * The raw value stays the machine name the API speaks (手册 4.14.3 item 2: the
 * console reads any value the server sends and writes only a declared one).
 * Values are ordered as the server declares them, so an option list renders in
 * the server's order.
 */

/** CAPTCHA provider types (`domain/captchaproviders.ProviderTypes`). */
// @enumset captchaProviderType
export const CAPTCHA_PROVIDER_TYPES = [
  "default",
  "recaptcha",
  "recaptcha_v3",
  "recaptcha_enterprise",
  "hcaptcha",
  "turnstile",
  "geetest",
  "aliyun_captcha",
  "tencent_captcha",
  "netease_yidun",
  "baidu_captcha",
  "go_captcha",
] as const;

/** Notification provider types (`domain/notificationproviders.ProviderTypes`). */
// @enumset notificationProviderType
export const NOTIFICATION_PROVIDER_TYPES = [
  "email",
  "sms",
  "dingtalk",
  "lark",
  "wecom",
  "wecom_app",
  "wechat_official",
  "slack",
  "discord",
  "rocketchat",
  "teams",
  "bark",
  "telegram",
  "pushover",
  "line",
  "matrix",
  "pushbullet",
  "viber",
  "webpush",
  "twitter",
  "reddit",
  "customhttp",
  "googlechat",
  "cucloud",
  "ntfy",
  "serverchan",
  "pagerduty",
  "opsgenie",
  "gotify",
] as const;

/** Resource types (`app/resources.ResourceTypes`). */
// @enumset resourceType
export const RESOURCE_TYPES = ["image", "css", "js", "file", "video", "audio"] as const;

/**
 * Lifecycle value sets for the commerce domain.
 *
 * Every set below mirrors a server lifecycle declared in
 * `registries/state-machine-registry.md` and enforced by a DDL `CHECK` plus the
 * domain transition table. The console must never invent a value for these
 * fields, and it must never render a raw PascalCase caption of its own: the
 * registry is the single source of truth for what a state is called, and the
 * caption is a translation of the value, not a replacement for it.
 *
 * The write side is closed (an unknown value is a 422), the read side is open
 * (手册 4.14.3 item 2): `enumOptions()` drops values the server rejects, while
 * `unknownStateLabel()` renders any value the server sends that a deployed
 * console does not know yet.
 */

/** Order states (`domain/orders.States`, registry §1.1). */
// @enumset ordersState captions=common:enumValues
export const ORDER_STATES = [
  "created",
  "payment_pending",
  "paid",
  "completed",
  "failed",
  "cancelled",
  "refunded",
  "closed",
] as const;

/** Payment states (`domain/payments.States`, registry §1.2). */
// @enumset paymentsState captions=common:enumValues
export const PAYMENT_STATES = [
  "created",
  "processing",
  "succeeded",
  "failed",
  "cancelled",
  "refunded",
  "partially_refunded",
] as const;

/** Subscription states (`domain/subscriptions.States`, registry §1.3). */
// @enumset subscriptionsState captions=common:enumValues
export const SUBSCRIPTION_STATES = [
  "pending",
  "trialing",
  "active",
  "past_due",
  "expired",
  "cancelled",
] as const;

/** Transaction kinds (`domain/transactions.Types`, registry §1.4). */
// @enumset transactionsType captions=common:enumValues
export const TRANSACTION_TYPES = ["payment", "refund", "cancellation"] as const;

/** Transaction settlement statuses (`domain/transactions.Statuses`, registry §1.4). */
// @enumset transactionsStatus captions=common:enumValues
export const TRANSACTION_STATUSES = ["pending", "completed", "failed", "cancelled"] as const;

/** Coupon states (`domain/coupons.States`, registry §1.5). */
// @enumset couponsState captions=common:enumValues
export const COUPON_STATES = ["active", "disabled", "expired"] as const;

/** Product states (`domain/products.States`, registry §1.5). */
// @enumset productsState captions=common:enumValues
export const PRODUCT_STATES = ["draft", "active", "discontinued"] as const;

/** Plan states (`domain/plans.States`, registry §1.5). */
// @enumset plansState captions=common:enumValues
export const PLAN_STATES = ["draft", "active", "archived"] as const;

/** Plan billing periods (`domain/plans.Periods`; DDL `ck_plans_period`). */
// @enumset plansPeriod captions=common:enumValues
export const PLAN_PERIODS = [
  "daily",
  "weekly",
  "monthly",
  "quarterly",
  "yearly",
  "lifetime",
] as const;

/** Ticket statuses (`domain/tickets.Statuses`, registry §1.5). */
// @enumset ticketsStatus captions=common:enumValues
export const TICKET_STATUSES = ["open", "pending", "resolved", "closed"] as const;

/** Ticket submission targets (`domain/tickets.Types`, naming registry §ticket_type). */
// @enumset ticketsType
export const TICKET_TYPES = ["org_request", "vendor_feedback"] as const;

/**
 * Caption keys for ticket submission targets.
 *
 * Unlike lifecycle states, a ticket type is a routing concept owned by the
 * ticket domain, so its captions live in the resource namespace next to the
 * rest of the ticket copy rather than in the shared `enumValues` group.
 */
const TICKET_TYPE_CAPTION_KEYS: Record<string, string> = {
  org_request: "ticket.typeOrgRequest",
  vendor_feedback: "ticket.typeVendorFeedback",
};

/** Select options for ticket submission targets (resource-namespace captions). */
export function ticketTypeOptions(tr: (key: string) => string): EnumOption[] {
  return TICKET_TYPES.map((value) => ({
    label: tr(TICKET_TYPE_CAPTION_KEYS[value] ?? value),
    value,
  }));
}

/** Caption for a ticket type; an unknown legacy value renders as the raw server value. */
export function ticketTypeLabel(value: string, tr: (key: string) => string): string {
  const key = TICKET_TYPE_CAPTION_KEYS[value];
  return key ? tr(key) : value;
}

const TICKET_PRIORITY_CAPTION_KEYS: Record<string, string> = {
  Low: "ticket.priorityLow",
  Medium: "ticket.priorityMedium",
  High: "ticket.priorityHigh",
  Urgent: "ticket.priorityUrgent",
};

/** Caption for a ticket priority; list/overview and form selectors share one wording. */
export function ticketPriorityLabel(value: string, tr: (key: string) => string): string {
  const key = TICKET_PRIORITY_CAPTION_KEYS[value];
  return key ? tr(key) : value;
}

/** Risk decision actions (`domain/risk.DecisionActions`; 手册 4.14.3). */
// @enumset riskDecisionAction
export const RISK_DECISION_ACTIONS = ["allow", "challenge", "deny"] as const;

/** Session governance actions (`domain/risk.GovernanceActions`). */
// @enumset riskGovernanceAction
export const RISK_GOVERNANCE_ACTIONS = ["monitor", "step_up", "revoke"] as const;

/** Risk appeal resolutions (`domain/risk.AppealResolutions`). */
// @enumset riskAppealResolution
export const RISK_APPEAL_RESOLUTIONS = ["accepted", "rejected"] as const;

/**
 * Password derivation algorithms (`domain/organizations.PasswordDeriveAlgorithm*`;
 * accepted/rejected in `app/organizations`). The server declares these as plain
 * constants plus a validation slice, so this list is the shared frontend source
 * used by the security-policy selector.
 */
export const PASSWORD_DERIVE_ALGORITHMS = ["hmac_sha256", "hmac_sm3"] as const;
export type PasswordDeriveAlgorithm = (typeof PASSWORD_DERIVE_ALGORITHMS)[number];

/**
 * Token signing methods (`domain/applications.TokenSigningMethod*`;
 * accepted/rejected in `app/applications`). Order follows the server declaration.
 */
export const TOKEN_SIGNING_METHODS = ["RS256", "SM2-SM3"] as const;
export type TokenSigningMethod = (typeof TOKEN_SIGNING_METHODS)[number];

/**
 * Database adapter options for Permission resource adapter selection.
 * Mirrors server's `internal/domain/applications.Application.ResourceType` closed set.
 * Used in PermissionDetailPage + PermissionDetailSections (§6 前端组件复用).
 */
export const DATABASE_ADAPTER_OPTIONS: EnumOption[] = [
  { label: "PostgreSQL", value: "postgresql" },
  { label: "File", value: "file" },
];

/** Display captions for provider types whose raw value is not readable enough. */
const PROVIDER_TYPE_LABELS: Record<string, string> = {
  recaptcha: "reCAPTCHA",
  recaptcha_v3: "reCAPTCHA v3",
  recaptcha_enterprise: "reCAPTCHA Enterprise",
  hcaptcha: "hCaptcha",
  turnstile: "Cloudflare Turnstile",
  geetest: "GeeTest",
  aliyun_captcha: "Aliyun Captcha",
  tencent_captcha: "Tencent Captcha",
  netease_yidun: "NetEase Yidun",
  baidu_captcha: "Baidu Captcha",
  go_captcha: "Go Captcha",
  dingtalk: "DingTalk",
  lark: "Lark",
  wecom: "WeCom",
  rocketchat: "Rocket.Chat",
  webpush: "Web Push",
  customhttp: "Custom HTTP",
  googlechat: "Google Chat",
  cucloud: "CUCloud",
  serverchan: "ServerChan",
  pagerduty: "PagerDuty",
  opsgenie: "OpsGenie",
  gotify: "Gotify",
};

export type EnumOption = { label: string; value: string };

function option(value: string): EnumOption {
  return { label: PROVIDER_TYPE_LABELS[value] ?? value, value };
}

/** Select options for CAPTCHA provider types. */
export function captchaProviderTypeOptions(): EnumOption[] {
  return CAPTCHA_PROVIDER_TYPES.map(option);
}

/** Select options for notification provider types. */
export function notificationProviderTypeOptions(): EnumOption[] {
  return NOTIFICATION_PROVIDER_TYPES.map(option);
}

/** Select options for resource types. */
export function resourceTypeOptions(): EnumOption[] {
  return RESOURCE_TYPES.map((value) => ({ label: value.toUpperCase(), value }));
}

/**
 * Caption key, tag colour and option list for a registry value.
 *
 * The console renders a lifecycle value in exactly one of two ways, and both go
 * through this module so a new value cannot be handled in one page and forgotten
 * in another:
 *
 *   - the write side (a select) uses the closed list above plus
 *     {@link stateOptions}, so the option a form offers is the value the server
 *     accepts;
 *   - the read side (a table cell or a detail field) uses {@link stateColor} and
 *     {@link stateCaptionKey} through `StateTag`, which falls back to the raw
 *     server value for a state this build does not know yet (手册 4.14.3 item 2).
 */

/** i18n path of the caption for a registry value (`common.enumValues.<value>`). */
export function stateCaptionKey(value: string): string {
  return `enumValues.${value}`;
}

/**
 * Antd tag colour for a registry value, keyed by the value itself.
 *
 * Colour carries the registry's own vocabulary: green is a settled success
 * state, blue is a state that is still moving, orange needs an operator (an
 * unpaid order, a subscription past due), red is a failure or a lapse, and every
 * terminal-but-not-successful state (`cancelled`, `closed`, `refunded`,
 * `discontinued`, `archived`, `draft`, `disabled`) stays neutral. An unknown
 * value is neutral too, because the console must not imply a meaning the server
 * did not send.
 */
const STATE_COLORS: Record<string, string> = {
  created: "blue",
  payment_pending: "orange",
  paid: "green",
  completed: "green",
  failed: "red",
  cancelled: "default",
  refunded: "purple",
  closed: "default",
  processing: "blue",
  succeeded: "green",
  pending: "orange",
  trialing: "cyan",
  active: "green",
  past_due: "orange",
  expired: "red",
  disabled: "default",
  draft: "default",
  discontinued: "default",
  archived: "default",
  open: "blue",
  resolved: "green",
};

/** Tag colour for a registry value; an unknown value is neutral. */
export function stateColor(value: string): string {
  return STATE_COLORS[value] ?? "default";
}

/**
 * Select options for one registry set.
 *
 * The translator is passed in rather than imported so the caller's namespace is
 * used and a missing caption shows up as a raw key in review instead of silently
 * rendering English. `scripts/check-i18n-keys.py` keeps every caption present in
 * all language packs.
 */
export function stateOptions(
  values: readonly string[],
  translate: (key: string) => string,
): EnumOption[] {
  return values.map((value) => ({ label: translate(stateCaptionKey(value)), value }));
}

/**
 * Caption for a provider type that came from the server.
 *
 * The response side is open (手册 4.14.3 item 2): the server may add a value at
 * any time, and a client that does not know it must still render something. The
 * raw value is the fallback — never a blank cell or a thrown error.
 */
export function providerTypeLabel(value: string): string {
  return PROVIDER_TYPE_LABELS[value] ?? value;
}
