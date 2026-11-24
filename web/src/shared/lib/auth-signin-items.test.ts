import { describe, it, expect, vi } from "vitest";
import {
  getForgetPasswordUrl,
  getSignupUrl,
  getFormPositionStyles,
  getSigninItems,
  isSigninItemShown,
  getSigninMethodTabs,
  getDefaultLoginMethod,
  canUseQrLogin,
  isPasswordMethod,
  isVerificationCodeMethod,
  getSigninMethodName,
  getUsernamePlaceholder,
  isCustomSigninItem,
  DEFAULT_APPLICATION_NAME,
  DEFAULT_ORGANIZATION_NAME,
  type SigninItemConfig,
} from "./auth-signin-items";
import type { ApplicationItem } from "@/shared/types/domain";

// Mock i18next
// i18n is consumed through the named `t` export; keep default.t for legacy call sites.
vi.mock("i18next", () => {
  const t = (key: string) => key;
  return { default: { t }, t };
});

describe("getForgetPasswordUrl", () => {
  it("returns configured URL when provided", () => {
    expect(getForgetPasswordUrl("app-1", "https://custom.example.com/reset")).toBe(
      "https://custom.example.com/reset",
    );
  });

  it("returns default URL when configured URL is empty", () => {
    expect(getForgetPasswordUrl("app-1", "")).toBe("/forgot-password?application=app-1");
  });

  it("returns default URL when configured URL is null", () => {
    expect(getForgetPasswordUrl("app-1", null)).toBe("/forgot-password?application=app-1");
  });

  it("encodes application name in URL", () => {
    expect(getForgetPasswordUrl("my app")).toBe("/forgot-password?application=my%20app");
  });

  it("trims configured URL and returns default if only whitespace", () => {
    expect(getForgetPasswordUrl("app-1", "   ")).toBe("/forgot-password?application=app-1");
  });
});

describe("getSignupUrl", () => {
  it("returns configured signup URL when set", () => {
    const app = { name: "app-1", signupUrl: "https://custom.signup" } as unknown as ApplicationItem;
    expect(getSignupUrl(app, "/login")).toBe("https://custom.signup");
  });

  it("replaces login with signup in OAuth authorize path", () => {
    const app = { name: "app-1" } as unknown as ApplicationItem;
    expect(getSignupUrl(app, "/login/oauth/authorize")).toBe("/signup/oauth/authorize");
  });

  it("returns /signup for default application", () => {
    const app = { name: DEFAULT_APPLICATION_NAME } as unknown as ApplicationItem;
    expect(getSignupUrl(app, "/login")).toBe("/signup");
  });

  it("uses the organization slug from the login path", () => {
    const app = { name: "my-app" } as unknown as ApplicationItem;
    expect(getSignupUrl(app, "/login/guliangtech")).toBe("/signup/guliangtech");
  });

  it("does not use the application name as the organization slug (signup 404 regression)", () => {
    const app = { name: "guliang", organization: "guliangtech" } as unknown as ApplicationItem;
    expect(getSignupUrl(app, "/login/guliangtech")).toBe("/signup/guliangtech");
  });

  it("falls back to application.organization when login path has no org segment", () => {
    const app = {
      name: "my-app",
      organization: "org-1",
      isShared: true,
    } as unknown as ApplicationItem;
    expect(getSignupUrl(app, "/login")).toBe("/signup/org-1");
  });

  it("defaults organization to built-in when no org slug is available", () => {
    const app = { name: "my-app" } as unknown as ApplicationItem;
    expect(getSignupUrl(app, "/login")).toBe("/signup/built-in");
  });
});

describe("getFormPositionStyles", () => {
  it("returns flex-start for offset 1", () => {
    const styles = getFormPositionStyles(1);
    expect(styles.justifyContent).toBe("flex-start");
    expect(styles.alignItems).toBe("flex-start");
  });

  it("returns center for offset 2 (default)", () => {
    const styles = getFormPositionStyles(2);
    expect(styles.justifyContent).toBe("center");
    expect(styles.alignItems).toBe("center");
  });

  it("returns flex-end for offset 3", () => {
    const styles = getFormPositionStyles(3);
    expect(styles.justifyContent).toBe("center");
    expect(styles.alignItems).toBe("flex-end");
  });

  it("returns stretch for offset 4", () => {
    const styles = getFormPositionStyles(4);
    expect(styles.justifyContent).toBe("center");
    expect(styles.alignItems).toBe("stretch");
  });

  it("defaults to center for undefined offset", () => {
    const styles = getFormPositionStyles(undefined);
    expect(styles.justifyContent).toBe("center");
    expect(styles.alignItems).toBe("center");
  });
});

describe("getSigninItems", () => {
  it("merges custom items with defaults by name", () => {
    const items: SigninItemConfig[] = [
      { name: "Username", visible: true },
      { name: "Password", visible: false },
    ];
    const result = getSigninItems({ signinItems: items });
    // 自定义项保留
    expect(result.some((i) => i.name === "Username" && i.visible === true)).toBe(true);
    expect(result.some((i) => i.name === "Password" && i.visible === false)).toBe(true);
    // 默认补充的必需项存在
    expect(result.some((i) => i.name === "Login button")).toBe(true);
    expect(result.some((i) => i.name === "Logo")).toBe(true);
    expect(result.some((i) => i.name === "Providers")).toBe(true);
  });

  it("always includes Login button even when custom items omit it", () => {
    const items: SigninItemConfig[] = [
      { name: "user", visible: true },
      { name: "Password", visible: true },
    ];
    const result = getSigninItems({ signinItems: items });
    expect(result.some((i) => i.name === "Login button")).toBe(true);
    expect(result.some((i) => i.name === "Username")).toBe(true);
    // 自定义独有项追加到末尾
    expect(result.some((i) => i.name === "user")).toBe(true);
  });

  it("custom Password visible:false overrides default Password visible:true", () => {
    const items: SigninItemConfig[] = [{ name: "Password", visible: false }];
    const result = getSigninItems({ signinItems: items });
    const password = result.find((i) => i.name === "Password");
    expect(password?.visible).toBe(false);
  });

  it("returns default items when application is null", () => {
    const result = getSigninItems(null);
    expect(result.length).toBeGreaterThan(0);
    expect(result.some((item) => item.name === "Username")).toBe(true);
    expect(result.some((item) => item.name === "Password")).toBe(true);
  });

  it("returns default items when signinItems is empty", () => {
    const result = getSigninItems({ signinItems: [] });
    expect(result.length).toBeGreaterThan(0);
  });

  it("default items include all essential items", () => {
    const result = getSigninItems(null);
    const names = result.map((item) => item.name);
    expect(names).toContain("Logo");
    expect(names).toContain("Username");
    expect(names).toContain("Password");
    expect(names).toContain("Login button");
    expect(names).toContain("Providers");
  });

  it("appends custom-only items (e.g. Text items) after defaults", () => {
    const items: SigninItemConfig[] = [{ name: "Text welcome", visible: true, isCustom: true }];
    const result = getSigninItems({ signinItems: items });
    const lastItem = result[result.length - 1];
    expect(lastItem.name).toBe("Text welcome");
    expect(result.some((i) => i.name === "Login button")).toBe(true);
  });
});

describe("isSigninItemShown", () => {
  it("returns false for null item", () => {
    expect(isSigninItemShown(null)).toBe(false);
  });

  it("returns false when rule is None", () => {
    expect(isSigninItemShown({ name: "test", rule: "None" })).toBe(false);
  });

  it("returns true when visible is true", () => {
    expect(isSigninItemShown({ name: "test", visible: true })).toBe(true);
  });

  it("returns true when visible is null (default visible)", () => {
    expect(isSigninItemShown({ name: "test", visible: null })).toBe(true);
  });

  it("returns false when visible is explicitly false", () => {
    expect(isSigninItemShown({ name: "test", visible: false })).toBe(false);
  });

  it("rule None overrides visible true", () => {
    expect(isSigninItemShown({ name: "test", visible: true, rule: "None" })).toBe(false);
  });
});

describe("getSigninMethodTabs", () => {
  it("returns all tabs with correct keys", () => {
    const tabs = getSigninMethodTabs(null, { hasFaceIdProvider: true });
    const keys = tabs.map((tab) => tab.key);
    expect(keys).toContain("password");
    expect(keys).toContain("ldap");
    expect(keys).toContain("verificationCode");
    expect(keys).toContain("webauthn");
    expect(keys).toContain("faceId");
    // 微信扫码 tab 已随 legacy 端点移除；扫码登录换用自建 /qrcode 契约。
    expect(keys).toContain("qrcode");
    expect(keys).not.toContain("wechat");
  });

  it("disables faceId when no faceId provider", () => {
    const tabs = getSigninMethodTabs(null, { hasFaceIdProvider: false });
    const faceIdTab = tabs.find((tab) => tab.key === "faceId");
    expect(faceIdTab?.disabled).toBe(true);
  });

  it("disables faceId outside a secure context even with provider and mediaDevices", () => {
    const tabs = getSigninMethodTabs(null, {
      hasFaceIdProvider: true,
      hasMediaDevices: true,
      isSecureContext: false,
    });
    expect(tabs.find((tab) => tab.key === "faceId")?.disabled).toBe(true);
  });

  it("keeps faceId enabled in a secure context with provider and mediaDevices", () => {
    const tabs = getSigninMethodTabs(null, {
      hasFaceIdProvider: true,
      hasMediaDevices: true,
      isSecureContext: true,
    });
    expect(tabs.find((tab) => tab.key === "faceId")?.disabled).toBe(false);
  });

  it("explains capability-disabled tabs with a tooltip reason (login.md §浏览器能力检测)", () => {
    const insecure = getSigninMethodTabs(null, {
      hasFaceIdProvider: true,
      hasMediaDevices: true,
      isSecureContext: false,
    });
    expect(insecure.find((tab) => tab.key === "webauthn")?.disabledReason).toContain(
      "disabledReason.insecureContext",
    );
    expect(insecure.find((tab) => tab.key === "faceId")?.disabledReason).toContain(
      "disabledReason.insecureContext",
    );

    const noWebAuthn = getSigninMethodTabs(null, {
      hasFaceIdProvider: true,
      hasMediaDevices: true,
      isSecureContext: true,
      supportsWebAuthn: false,
    });
    expect(noWebAuthn.find((tab) => tab.key === "webauthn")?.disabledReason).toContain(
      "disabledReason.unsupportedBrowser",
    );
    expect(noWebAuthn.find((tab) => tab.key === "faceId")?.disabledReason).toBeUndefined();
  });

  it("attaches no browser-capability reason to configuration-disabled tabs", () => {
    // faceId 提供方未配置属于应用配置禁用，不是浏览器能力检测失败（login.md 范围）。
    const tabs = getSigninMethodTabs(null, { hasFaceIdProvider: false });
    expect(tabs.find((tab) => tab.key === "faceId")?.disabled).toBe(true);
    expect(tabs.find((tab) => tab.key === "faceId")?.disabledReason).toBeUndefined();
  });

  it("disables qrcode until the application carries id + organizationId", () => {
    const withoutIds = getSigninMethodTabs({ name: "app-1" } as unknown as ApplicationItem, {
      hasFaceIdProvider: false,
    });
    expect(withoutIds.find((tab) => tab.key === "qrcode")?.disabled).toBe(true);

    const resolved = getSigninMethodTabs(
      {
        name: "app-1",
        id: "019app",
        organizationId: "019org",
      } as unknown as ApplicationItem,
      { hasFaceIdProvider: false },
    );
    expect(resolved.find((tab) => tab.key === "qrcode")?.disabled).toBe(false);
  });
});

describe("canUseQrLogin", () => {
  it("accepts an application resolved down to id and organizationId", () => {
    expect(
      canUseQrLogin({ id: "app-1", organizationId: "org-1" } as unknown as ApplicationItem),
    ).toBe(true);
  });

  it("rejects an unresolved application", () => {
    expect(canUseQrLogin(null)).toBe(false);
  });

  it("rejects an application without organizationId", () => {
    expect(canUseQrLogin({ id: "app-1" } as unknown as ApplicationItem)).toBe(false);
  });
});

describe("getDefaultLoginMethod", () => {
  it("returns password when password is enabled", () => {
    const method = getDefaultLoginMethod(null, { hasFaceIdProvider: false });
    expect(method).toBe("password");
  });

  it("returns verificationCode when password is disabled", () => {
    const app = {
      signinItems: [
        { name: "Password", visible: false },
        { name: "Verification code", visible: true },
      ],
    } as unknown as ApplicationItem;
    const method = getDefaultLoginMethod(app, { hasFaceIdProvider: false });
    expect(method).toBe("verificationCode");
  });
});

describe("isPasswordMethod", () => {
  it("returns true for password", () => {
    expect(isPasswordMethod("password")).toBe(true);
  });

  it("returns true for ldap", () => {
    expect(isPasswordMethod("ldap")).toBe(true);
  });

  it("returns true for webauthn", () => {
    expect(isPasswordMethod("webauthn")).toBe(true);
  });

  it("returns false for verificationCode", () => {
    expect(isPasswordMethod("verificationCode")).toBe(false);
  });

  it("returns false for faceId", () => {
    expect(isPasswordMethod("faceId")).toBe(false);
  });

  it("returns false for qrcode", () => {
    expect(isPasswordMethod("qrcode")).toBe(false);
  });
});

describe("isVerificationCodeMethod", () => {
  it("returns true for verificationCode", () => {
    expect(isVerificationCodeMethod("verificationCode")).toBe(true);
  });

  it("returns true for verificationCodeEmail", () => {
    expect(isVerificationCodeMethod("verificationCodeEmail")).toBe(true);
  });

  it("returns true for verificationCodePhone", () => {
    expect(isVerificationCodeMethod("verificationCodePhone")).toBe(true);
  });

  it("returns false for password", () => {
    expect(isVerificationCodeMethod("password")).toBe(false);
  });
});

describe("getSigninMethodName", () => {
  it("returns LDAP for ldap", () => {
    expect(getSigninMethodName("ldap")).toBe("LDAP");
  });

  it("returns Verification code for verificationCode", () => {
    expect(getSigninMethodName("verificationCode")).toBe("Verification code");
  });

  it("returns WebAuthn for webauthn", () => {
    expect(getSigninMethodName("webauthn")).toBe("WebAuthn");
  });

  it("returns Face ID for faceId", () => {
    expect(getSigninMethodName("faceId")).toBe("Face ID");
  });

  it("returns QR Code for qrcode", () => {
    expect(getSigninMethodName("qrcode")).toBe("QR Code");
  });

  it("returns Password for password", () => {
    expect(getSigninMethodName("password")).toBe("Password");
  });
});

describe("getUsernamePlaceholder", () => {
  it("returns email or phone placeholder for verificationCode", () => {
    const result = getUsernamePlaceholder("verificationCode");
    expect(result).toContain("emailOrPhone");
  });

  it("returns email placeholder for verificationCodeEmail", () => {
    const result = getUsernamePlaceholder("verificationCodeEmail");
    expect(result).toContain("email");
  });

  it("returns phone placeholder for verificationCodePhone", () => {
    const result = getUsernamePlaceholder("verificationCodePhone");
    expect(result).toContain("phone");
  });

  it("returns LDAP placeholder for ldap", () => {
    const result = getUsernamePlaceholder("ldap");
    expect(result).toContain("ldapUsername");
  });

  it("returns default placeholder for password", () => {
    const result = getUsernamePlaceholder("password");
    expect(result).toContain("defaultUsername");
  });
});

describe("isCustomSigninItem", () => {
  it("returns true when isCustom is true", () => {
    expect(isCustomSigninItem({ name: "test", isCustom: true })).toBe(true);
  });

  it("returns true when name starts with Text", () => {
    expect(isCustomSigninItem({ name: "Text content" })).toBe(true);
  });

  it("returns false for standard items", () => {
    expect(isCustomSigninItem({ name: "Password" })).toBe(false);
  });

  it("returns false when isCustom is false and name does not start with Text", () => {
    expect(isCustomSigninItem({ name: "Username", isCustom: false })).toBe(false);
  });
});

describe("Constants", () => {
  it("exports correct default application name", () => {
    expect(DEFAULT_APPLICATION_NAME).toBe("app-built-in");
  });

  it("exports correct default organization name", () => {
    expect(DEFAULT_ORGANIZATION_NAME).toBe("built-in");
  });
});
