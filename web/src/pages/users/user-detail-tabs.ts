/**
 * 用户详情页的分区导航模型。
 *
 * 按业务维度收敛为 8 个平铺分区（不再渲染二级分组）：资料、实名认证、身份、
 * 权限、凭证、MFA、会话与授权、资产与托管。多个原 Tab 合并为同一分区时，
 * 分区内的卡片自带视觉区隔，导航只保留业务维度。
 *
 * URL 只保存 `?tab=<分区 key>`；旧 Tab key 通过别名折叠，保证老链接仍能定位。
 */

export type UserDetailTabKey =
  | "basic"
  | "realname"
  | "identity"
  | "account"
  | "credentials"
  | "mfa"
  | "sessions"
  | "assets";

export type UserDetailTabEntry = {
  key: UserDetailTabKey;
  labelKey: string;
};

/** 全部业务分区，顺序即默认导航顺序。 */
export const USER_DETAIL_TABS: readonly UserDetailTabEntry[] = [
  { key: "basic", labelKey: "resource:user.tabs.basic" },
  { key: "realname", labelKey: "resource:user.tabs.realname" },
  { key: "identity", labelKey: "resource:user.tabs.identity" },
  { key: "account", labelKey: "resource:user.tabs.account" },
  { key: "credentials", labelKey: "resource:user.tabs.credentials" },
  { key: "mfa", labelKey: "resource:user.tabs.mfa" },
  { key: "sessions", labelKey: "resource:user.tabs.sessions" },
  { key: "assets", labelKey: "resource:user.tabs.assets" },
];

export const USER_DETAIL_TAB_KEYS: readonly UserDetailTabKey[] = USER_DETAIL_TABS.map(
  (tab) => tab.key,
);

const USER_DETAIL_TAB_KEY_SET = new Set<string>(USER_DETAIL_TAB_KEYS);

/**
 * 旧参数/自然写法别名。原 12 个 Tab 与更早的分组名都折叠到新分区：
 * profile/properties/custom 归入资料（自定义属性已合并到资料卡片），
 * consents 归入会话，finance/managed 归入资产。
 */
const TAB_ALIASES: Record<string, UserDetailTabKey> = {
  profile: "basic",
  properties: "basic",
  custom: "basic",
  attributes: "basic",
  "real-name": "realname",
  consents: "sessions",
  session: "sessions",
  finance: "assets",
  "managed-accounts": "assets",
  managedaccounts: "assets",
  payment: "assets",
  payments: "assets",
  access: "identity",
};

/** URL `?tab=` → 可渲染的分区 key；未知值回落到默认 Basic。 */
export function resolveUserDetailTab(raw: string | null | undefined): UserDetailTabKey {
  const value = (raw ?? "").trim();
  if (!value) return "basic";
  if (USER_DETAIL_TAB_KEY_SET.has(value)) return value as UserDetailTabKey;
  return TAB_ALIASES[value] ?? TAB_ALIASES[value.toLowerCase()] ?? "basic";
}
