import { t } from "i18next";
import type { ApplicationItem } from "@/shared/types/domain";

const DEFAULT_ORGANIZATION_NAME = "built-in";
const DEFAULT_APPLICATION_NAME = "app-built-in";

export type LoginMethodKey =
  | "password"
  | "verificationCode"
  | "verificationCodeEmail"
  | "verificationCodePhone"
  | "ldap"
  | "webauthn"
  | "faceId"
  | "qrcode";

export type SigninItemConfig = {
  name?: string | null;
  label?: string | null;
  visible?: boolean | null;
  rule?: string | null;
  customCss?: string | null;
  isCustom?: boolean | null;
};

export type SigninMethodTab = {
  key: LoginMethodKey;
  label: string;
  disabled?: boolean;
  /**
   * login.md「浏览器能力检测 Hook」：能力检测失败禁用的 Tab 必须悬浮说明原因
   * （i18n 键 `auth:signin.tab.disabledReason.*`）。应用配置未开启的 Tab 不带原因。
   */
  disabledReason?: string;
};

/**
 * 登录方式 Tab 启用/禁用的判定上下文。
 *
 * - hasFaceIdProvider:       后端是否配置了 Face ID Provider（鉴权入口存在性）
 * - supportsWebAuthn:        浏览器 navigator.credentials.get 可用（WebAuthn/Passkey API 存在）
 * - isSecureContext:         HTTPS / localhost 环境（WebAuthn 强制要求）
 * - hasMediaDevices:         camera 设备访问（Face ID 采集前置）
 * - supportsPlatformAuth:    platform authenticator 可用（TouchID / Windows Hello / Android 指纹）
 */
export type SigninCapabilities = {
  hasFaceIdProvider: boolean;
  supportsWebAuthn?: boolean;
  isSecureContext?: boolean;
  hasMediaDevices?: boolean;
  supportsPlatformAuth?: boolean;
};

export function getForgetPasswordUrl(applicationName: string, configuredUrl?: string | null) {
  if (configuredUrl && configuredUrl.trim()) {
    return configuredUrl;
  }

  return `/forgot-password?application=${encodeURIComponent(applicationName)}`;
}

export function getSignupUrl(application: ApplicationItem, pathname: string) {
  const configuredSignupUrl =
    typeof (application as Record<string, unknown>).signupUrl === "string"
      ? ((application as Record<string, unknown>).signupUrl as string)
      : "";

  if (configuredSignupUrl.trim()) {
    return configuredSignupUrl;
  }

  if (pathname.includes("/login/oauth/authorize")) {
    return pathname.replace("/login/oauth/authorize", "/signup/oauth/authorize");
  }

  if (application.name === DEFAULT_APPLICATION_NAME) {
    return "/signup";
  }

  // 注册页路径段是组织 slug（/signup/:organizationName）。此前误把应用名当路径段，
  // 注册页将其当作组织 slug 解析（GetBySlug），应用名≠组织 slug 时即 404。
  // 应用名由调用方拼接的 login search（?application=）携带，这里只产出组织 slug 路径，
  // 避免与 SignupLinkSigninItem 的 `${signupUrl}${search}` 拼接产生重复查询串。
  const organizationSlug =
    organizationSlugFromLoginPath(pathname) ||
    application.organization?.trim() ||
    DEFAULT_ORGANIZATION_NAME;
  return `/signup/${encodeURIComponent(organizationSlug)}`;
}

/** 从登录路径 /login/{orgSlug} 取组织 slug；协议子路径（oauth/saml）不是组织段。 */
function organizationSlugFromLoginPath(pathname: string): string | null {
  const match = /^\/login\/([^/]+)/.exec(pathname);
  if (!match) return null;
  const segment = decodeURIComponent(match[1]);
  if (segment === "oauth" || segment === "saml") return null;
  return segment || null;
}

export function getFormPositionStyles(formOffset: ApplicationItem["formOffset"] | undefined) {
  switch (`${formOffset ?? 2}`) {
    case "1":
      return { justifyContent: "flex-start", alignItems: "flex-start" };
    case "3":
      return { justifyContent: "center", alignItems: "flex-end" };
    case "4":
      return { justifyContent: "center", alignItems: "stretch" };
    default:
      return { justifyContent: "center", alignItems: "center" };
  }
}

/** 应用级 CSS 的作用域：登录/注册壳与预览帧共用同一类名，租户 CSS 只落在壳内，
 * 不波及 antd portal（弹窗/下拉/消息）与文档其余部分。 */
export const LOGIN_SHELL_CSS_SCOPE = ".login-content";

/** 登录/注册壳共用的外背景：应用背景图叠加半透明渐变，无图时用品牌 token 渐变。
 * 遮罩色消费 --gl-color-bg（浅色/深色主题各自取值），不写死浅色值。 */
export function getOuterBackground(formBackgroundUrl?: string | null): string {
  return formBackgroundUrl
    ? `linear-gradient(color-mix(in srgb, var(--gl-color-bg) 78%, transparent), color-mix(in srgb, var(--gl-color-bg) 90%, transparent)), url(${formBackgroundUrl}) center / cover no-repeat`
    : "radial-gradient(circle at top left, color-mix(in srgb, var(--gl-color-primary) 15%, transparent), transparent 32%), linear-gradient(180deg, var(--gl-color-bg) 0%, var(--gl-color-bg-elevated) 100%)";
}

/** formOffset=4 为左右分栏布局（侧栏 HTML + 表单），登录/注册壳同一判定。 */
export function getIsSplitLayout(formOffset?: string | number | null): boolean {
  return `${formOffset ?? 2}` === "4";
}

/**
 * 默认登录项。当应用未配置或配置不完整时，这些项保证登录页始终可用
 * （至少包含 Username / Password / Login button 等必需项）。
 */
export const DEFAULT_SIGNIN_ITEMS: SigninItemConfig[] = [
  { name: "Logo", visible: true },
  { name: "Languages", visible: true },
  { name: "Signin methods", visible: true },
  { name: "Username", visible: true },
  { name: "Password", visible: true },
  { name: "Verification code", visible: true },
  { name: "Agreement", visible: true },
  { name: "Captcha", visible: true },
  { name: "Login button", visible: true },
  { name: "Forgot password?", visible: true },
  { name: "Signup link", visible: true },
  { name: "Providers", visible: true },
];

/**
 * 解析登录项列表。
 *
 * 合并策略：自定义项按 name 覆盖默认项；默认项中未被自定义覆盖的保留；
 * 自定义项中不在默认项里的追加到末尾（支持自定义 Text 项）。
 * 这样即使应用只配置了部分 signinItems，Login button / Username 等
 * 必需项也始终存在，登录页不会因配置不完整而不可用。
 */
export function getSigninItems(application: { signinItems?: SigninItemConfig[] | null } | null) {
  const custom = application?.signinItems ?? [];
  if (custom.length === 0) {
    return DEFAULT_SIGNIN_ITEMS;
  }

  const customByName = new Map<string, SigninItemConfig>();
  for (const item of custom) {
    if (item?.name) {
      customByName.set(item.name, item);
    }
  }

  // 1. 默认项：被同名自定义项覆盖则用自定义，否则用默认
  const merged: SigninItemConfig[] = DEFAULT_SIGNIN_ITEMS.map((defaultItem) => {
    const override = defaultItem.name ? customByName.get(defaultItem.name) : undefined;
    return override ?? defaultItem;
  });

  // 2. 自定义独有项（不在默认项中）追加到末尾
  const defaultNames = new Set(DEFAULT_SIGNIN_ITEMS.map((i) => i.name));
  for (const item of custom) {
    if (item?.name && !defaultNames.has(item.name)) {
      merged.push(item);
    }
  }

  return merged;
}

export function isSigninItemShown(item: SigninItemConfig | null) {
  if (!item) {
    return false;
  }

  if (item.rule === "None") {
    return false;
  }

  return item.visible !== false;
}

export function canUseQrLogin(application: ApplicationItem | null) {
  // 扫码登录需要发起端把自己所属的应用/组织交给服务端去建二维码会话；
  // 两者都是 resolve 响应里的 UUID，缺一个就不该显示该 tab。
  return Boolean(application?.id && application?.organizationId);
}

export function getSigninMethodTabs(
  application: ApplicationItem | null,
  capabilities: SigninCapabilities,
): SigninMethodTab[] {
  const signinItems = getSigninItems(application);
  const showPassword = signinItems.some(
    (item) => item.name === "Password" && isSigninItemShown(item),
  );
  const showVerificationCode = signinItems.some(
    (item) => item.name === "Verification code" && isSigninItemShown(item),
  );

  const webAuthnDisabled =
    !showPassword ||
    capabilities.supportsWebAuthn === false ||
    capabilities.isSecureContext === false;

  // getUserMedia 与 WebAuthn 一样要求安全上下文（HTTPS/localhost）。
  const faceIdDisabled =
    !capabilities.hasFaceIdProvider ||
    capabilities.hasMediaDevices === false ||
    capabilities.isSecureContext === false;

  // 检测失败不降级功能（fail-close），只禁用 Tab 并悬浮说明原因（login.md §浏览器能力检测）。
  const insecureContextReason =
    capabilities.isSecureContext === false
      ? t("auth:signin.tab.disabledReason.insecureContext")
      : undefined;
  const webAuthnCapabilityReason =
    insecureContextReason ??
    (capabilities.supportsWebAuthn === false
      ? t("auth:signin.tab.disabledReason.unsupportedBrowser")
      : undefined);
  const faceIdCapabilityReason =
    insecureContextReason ??
    (capabilities.hasMediaDevices === false
      ? t("auth:signin.tab.disabledReason.unsupportedBrowser")
      : undefined);

  return [
    {
      key: "password",
      label: t("auth:signin.methodTabs.password"),
      disabled: !showPassword,
    },
    { key: "ldap", label: "LDAP", disabled: !showPassword },
    {
      key: "verificationCode",
      label: t("auth:signin.methodTabs.verificationCode"),
      disabled: !showVerificationCode,
    },
    {
      key: "webauthn",
      label: "WebAuthn",
      disabled: webAuthnDisabled,
      disabledReason: showPassword ? webAuthnCapabilityReason : undefined,
    },
    {
      key: "faceId",
      label: "Face ID",
      disabled: faceIdDisabled,
      disabledReason: capabilities.hasFaceIdProvider ? faceIdCapabilityReason : undefined,
    },
    {
      key: "qrcode",
      label: t("auth:login.qrcode.title"),
      disabled: !canUseQrLogin(application),
    },
  ];
}

export function getDefaultLoginMethod(
  application: ApplicationItem | null,
  capabilities: SigninCapabilities,
): LoginMethodKey {
  return (
    getSigninMethodTabs(application, capabilities).find((item) => !item.disabled)?.key ?? "password"
  );
}

export function isPasswordMethod(loginMethod: LoginMethodKey) {
  return loginMethod === "password" || loginMethod === "ldap" || loginMethod === "webauthn";
}

export function isVerificationCodeMethod(loginMethod: LoginMethodKey) {
  return loginMethod.startsWith("verificationCode");
}

/**
 * 一次性码登录的请求层通道值：后端 validateLoginCommand 只接受 "email"/"sms"
 * （存储侧的 "phone" 由服务端映射，不出现在请求里）。判定规则与发码端
 * useVerificationCodeFlow.getVerificationCodeTarget 一致：显式 tab 优先，
 * 混合 tab 按用户名形态（含 @ 即邮箱）。
 */
export function getLoginVerificationChannel(
  username: string,
  loginMethod: LoginMethodKey,
): "email" | "sms" {
  if (loginMethod === "verificationCodeEmail") {
    return "email";
  }
  if (loginMethod === "verificationCodePhone") {
    return "sms";
  }
  return username.includes("@") ? "email" : "sms";
}

export function getSigninMethodName(loginMethod: LoginMethodKey) {
  if (loginMethod === "ldap") {
    return "LDAP";
  }
  if (isVerificationCodeMethod(loginMethod)) {
    return "Verification code";
  }
  if (loginMethod === "webauthn") {
    return "WebAuthn";
  }
  if (loginMethod === "faceId") {
    return "Face ID";
  }
  if (loginMethod === "qrcode") {
    return "QR Code";
  }
  return "Password";
}

export function getUsernamePlaceholder(loginMethod: LoginMethodKey) {
  switch (loginMethod) {
    case "verificationCode":
      return t("auth:signin.placeholders.emailOrPhone");
    case "verificationCodeEmail":
      return t("auth:signin.placeholders.email");
    case "verificationCodePhone":
      return t("auth:signin.placeholders.phone");
    case "ldap":
      return t("auth:signin.placeholders.ldapUsername");
    default:
      return t("auth:signin.placeholders.defaultUsername");
  }
}

export function isCustomSigninItem(item: SigninItemConfig) {
  if (item.isCustom) {
    return true;
  }

  return Boolean(item.name?.startsWith("Text "));
}

export { DEFAULT_APPLICATION_NAME, DEFAULT_ORGANIZATION_NAME };
