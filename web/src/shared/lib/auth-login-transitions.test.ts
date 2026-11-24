import { describe, it, expect, vi } from "vitest";
import {
  resolveLoginTransition,
  executeLoginTransition,
  resolveSamlLoginTransition,
  executeSamlLoginTransition,
  toMfaProps,
  buildConsentRoute,
  type LoginTransition,
  type SamlLoginTransition,
} from "./auth-login-transitions";
import type { LoginResult, LoginMfaRequired } from "@/shared/api/iam/auth";
import type { ResponseEnvelope } from "@/shared/api/http";
import { expectKind } from "@/test-utils/assertions";

// Mock i18next
// i18n is consumed through the named `t` export; keep default.t for legacy call sites.
vi.mock("i18next", () => {
  const t = (key: string) => key;
  return { default: { t }, t };
});

// Mock auth-mfa-setup
vi.mock("@/shared/lib/auth-mfa-setup", () => ({
  buildMfaSetupUrl: (values: { owner: string; name: string; returnTo: string }) =>
    `/mfa/setup?owner=${values.owner}&name=${values.name}&returnTo=${encodeURIComponent(values.returnTo)}`,
}));

type TestPayload = {
  organization: string;
  username: string;
  application: string;
  password: string;
  challengeId: string;
};

const basePayload: TestPayload = {
  organization: "org-1",
  username: "admin",
  application: "app-1",
  password: "secret",
  challengeId: "challenge-1",
};

describe("resolveLoginTransition", () => {
  it("returns success transition for successful login", () => {
    const response: LoginResult = {
      status: "success",
      account: { owner: "org-1", name: "admin", displayName: null, avatar: null, isAdmin: true },
      organization: {
        id: "org-1",
        name: "default",
        displayName: null,
        logoUrl: null,
        faviconUrl: null,
        navItems: [],
        userNavItems: [],
      },
    };

    const transition = resolveLoginTransition({
      payload: basePayload,
      response,
      returnTo: "/dashboard",
    });

    expectKind(transition, "success");
    expect(transition.finishContext.organization).toBe("org-1");
    expect(transition.finishContext.application).toBe("app-1");
  });

  it("returns pending-mfa transition for MFA required response", () => {
    const response: LoginMfaRequired = {
      status: "mfa_required",
      challengeId: "ch-1",
      nextMfa: "totp",
      requiredMfa: ["totp"],
      availableMfaMethods: [{ id: "m1", methodType: "totp", displayName: "Authenticator" }],
    };

    const transition = resolveLoginTransition({
      payload: basePayload,
      response,
      returnTo: "/dashboard",
    });

    expectKind(transition, "pending-mfa");
    expect(transition.pendingMfa.source).toBe("auth");
    expect(transition.pendingMfa.mfaProps).toHaveLength(1);
    expect(transition.infoMessage).toBeDefined();
  });

  it("returns required-mfa-setup transition for MFA setup required response", () => {
    const response: LoginResult = {
      status: "required_mfa_setup",
      challengeId: "ch-setup-1",
      requiredMfa: ["totp"],
    };

    const transition = resolveLoginTransition({
      payload: basePayload,
      response,
      returnTo: "/dashboard",
    });

    expectKind(transition, "required-mfa-setup");
    expect(transition.targetUrl).toContain("/mfa/setup");
    expect(transition.pendingSetup).toBeDefined();
  });
});

describe("executeLoginTransition", () => {
  it("executes success transition by calling finishConsoleLogin", async () => {
    const finishConsoleLogin = vi
      .fn<(...args: unknown[]) => Promise<unknown>>()
      .mockResolvedValue(undefined);
    const transition: LoginTransition<TestPayload> = {
      kind: "success",
      finishContext: { organization: "org-1", application: "app-1" },
    };

    await executeLoginTransition({
      transition,
      setPendingMfa: vi.fn<(...args: unknown[]) => unknown>(),
      showInfo: vi.fn<(...args: unknown[]) => unknown>(),
      storePendingSetup: vi.fn<(...args: unknown[]) => unknown>(),
      navigate: vi.fn<(...args: unknown[]) => unknown>(),
      finishConsoleLogin,
    });

    expect(finishConsoleLogin).toHaveBeenCalledWith({
      organization: "org-1",
      application: "app-1",
    });
  });

  it("executes pending-mfa transition by setting pending MFA state", async () => {
    const setPendingMfa = vi.fn<(...args: unknown[]) => unknown>();
    const showInfo = vi.fn<(...args: unknown[]) => unknown>();
    const pendingMfa = {
      source: "auth" as const,
      loginValues: { ...basePayload, password: "", challengeId: "ch-1" },
      mfaProps: [],
    };
    const transition: LoginTransition<TestPayload> = {
      kind: "pending-mfa",
      pendingMfa,
      infoMessage: "MFA required",
    };

    await executeLoginTransition({
      transition,
      setPendingMfa,
      showInfo,
      storePendingSetup: vi.fn<(...args: unknown[]) => unknown>(),
      navigate: vi.fn<(...args: unknown[]) => unknown>(),
      finishConsoleLogin: vi.fn<(...args: unknown[]) => unknown>(),
    });

    expect(setPendingMfa).toHaveBeenCalledWith(pendingMfa);
    expect(showInfo).toHaveBeenCalledWith("MFA required");
  });

  it("executes pending-mfa transition with onPendingMfa callback", async () => {
    const onPendingMfa = vi.fn<(...args: unknown[]) => unknown>();
    const setPendingMfa = vi.fn<(...args: unknown[]) => unknown>();
    const pendingMfa = {
      source: "auth" as const,
      loginValues: { ...basePayload, password: "", challengeId: "ch-1" },
      mfaProps: [],
    };
    const transition: LoginTransition<TestPayload> = {
      kind: "pending-mfa",
      pendingMfa,
      infoMessage: "MFA required",
    };

    await executeLoginTransition({
      transition,
      setPendingMfa,
      showInfo: vi.fn<(...args: unknown[]) => unknown>(),
      storePendingSetup: vi.fn<(...args: unknown[]) => unknown>(),
      navigate: vi.fn<(...args: unknown[]) => unknown>(),
      finishConsoleLogin: vi.fn<(...args: unknown[]) => unknown>(),
      onPendingMfa,
    });

    expect(onPendingMfa).toHaveBeenCalledWith(pendingMfa);
    expect(setPendingMfa).not.toHaveBeenCalled();
  });

  it("executes required-mfa-setup transition by storing setup and navigating", async () => {
    const storePendingSetup = vi.fn<(...args: unknown[]) => unknown>();
    const navigate = vi.fn<(...args: unknown[]) => unknown>();
    const pendingSetup = {
      organization: "org-1",
      application: "app-1",
      username: "admin",
      returnTo: "/dashboard",
      challengeId: "ch-setup-1",
      requiredMfa: ["totp"],
    };
    const transition: LoginTransition<TestPayload> = {
      kind: "required-mfa-setup",
      pendingSetup,
      targetUrl: "/mfa/setup?owner=org-1",
    };

    await executeLoginTransition({
      transition,
      setPendingMfa: vi.fn<(...args: unknown[]) => unknown>(),
      showInfo: vi.fn<(...args: unknown[]) => unknown>(),
      storePendingSetup,
      navigate,
      finishConsoleLogin: vi.fn<(...args: unknown[]) => unknown>(),
    });

    expect(storePendingSetup).toHaveBeenCalledWith(pendingSetup);
    expect(navigate).toHaveBeenCalledWith("/mfa/setup?owner=org-1");
  });
});

describe("resolveSamlLoginTransition", () => {
  it("returns error transition for non-ok status", () => {
    const response: ResponseEnvelope = { status: "error", msg: "Login failed" };
    const transition = resolveSamlLoginTransition({
      payload: basePayload,
      response,
      returnTo: "/dashboard",
      isOauthAuthorize: false,
      locationSearch: "",
    });

    expectKind(transition, "error");
    expect(transition.message).toBe("Login failed");
    expect(transition.shouldReloadCaptcha).toBe(true);
  });

  it("returns pending-mfa transition for NextMfa response", () => {
    const response: ResponseEnvelope = { status: "ok", data: "NextMfa", data2: [] };
    const transition = resolveSamlLoginTransition({
      payload: basePayload,
      response,
      returnTo: "/dashboard",
      isOauthAuthorize: false,
      locationSearch: "",
    });

    expectKind(transition, "pending-mfa");
    expect(transition.pendingMfa.source).toBe("legacy");
  });

  it("returns required-mfa-setup transition for RequiredMfa response", () => {
    const response: ResponseEnvelope = { status: "ok", data: "RequiredMfa" };
    const transition = resolveSamlLoginTransition({
      payload: basePayload,
      response,
      returnTo: "/dashboard",
      isOauthAuthorize: false,
      locationSearch: "",
    });

    expectKind(transition, "required-mfa-setup");
    expect(transition.shouldInvalidateSession).toBe(true);
    expect(transition.targetUrl).toContain("/mfa/setup");
  });

  it("returns consent transition for consent-required response", () => {
    const response: ResponseEnvelope = {
      status: "ok",
      data: { required: true },
    };
    const transition = resolveSamlLoginTransition({
      payload: basePayload,
      response,
      returnTo: "/dashboard",
      isOauthAuthorize: true,
      consentApplicationName: "my-app",
      locationSearch: "?state=abc",
    });

    expectKind(transition, "consent");
    expect(transition.targetUrl).toContain("/consent/");
  });

  it("returns error when consent is required but consentApplicationName is missing", () => {
    const response: ResponseEnvelope = {
      status: "ok",
      data: { required: true },
    };
    const transition = resolveSamlLoginTransition({
      payload: basePayload,
      response,
      returnTo: "/dashboard",
      isOauthAuthorize: true,
      consentApplicationName: null,
      locationSearch: "?state=abc",
    });

    expectKind(transition, "error");
    expect(transition.shouldReloadCaptcha).toBe(false);
  });

  it("returns finish transition for normal ok response", () => {
    const response: ResponseEnvelope = { status: "ok", data: "some-data" };
    const transition = resolveSamlLoginTransition({
      payload: basePayload,
      response,
      returnTo: "/dashboard",
      isOauthAuthorize: false,
      locationSearch: "",
    });

    expect(transition.kind).toBe("finish");
  });
});

describe("executeSamlLoginTransition", () => {
  it("executes error transition by showing error and reloading captcha", async () => {
    const showError = vi.fn<(...args: unknown[]) => unknown>();
    const reloadCaptcha = vi.fn<(...args: unknown[]) => unknown>();
    const transition: SamlLoginTransition<TestPayload> = {
      kind: "error",
      message: "Login failed",
      shouldReloadCaptcha: true,
    };

    await executeSamlLoginTransition({
      transition,
      response: { status: "ok" },
      payload: basePayload,
      captchaRequired: true,
      reloadCaptcha,
      showError,
      showInfo: vi.fn<(...args: unknown[]) => unknown>(),
      setPendingMfa: vi.fn<(...args: unknown[]) => unknown>(),
      invalidateSession: vi.fn<(...args: unknown[]) => unknown>(),
      navigate: vi.fn<(...args: unknown[]) => unknown>(),
      finishLogin: vi.fn<(...args: unknown[]) => unknown>(),
    });

    expect(showError).toHaveBeenCalledWith("Login failed");
    expect(reloadCaptcha).toHaveBeenCalled();
  });

  it("does not reload captcha when captchaRequired is false", async () => {
    const showError = vi.fn<(...args: unknown[]) => unknown>();
    const reloadCaptcha = vi.fn<(...args: unknown[]) => unknown>();
    const transition: SamlLoginTransition<TestPayload> = {
      kind: "error",
      message: "Login failed",
      shouldReloadCaptcha: true,
    };

    await executeSamlLoginTransition({
      transition,
      response: { status: "ok" },
      payload: basePayload,
      captchaRequired: false,
      reloadCaptcha,
      showError,
      showInfo: vi.fn<(...args: unknown[]) => unknown>(),
      setPendingMfa: vi.fn<(...args: unknown[]) => unknown>(),
      invalidateSession: vi.fn<(...args: unknown[]) => unknown>(),
      navigate: vi.fn<(...args: unknown[]) => unknown>(),
      finishLogin: vi.fn<(...args: unknown[]) => unknown>(),
    });

    expect(showError).toHaveBeenCalled();
    expect(reloadCaptcha).not.toHaveBeenCalled();
  });

  it("executes finish transition by calling finishLogin", async () => {
    const finishLogin = vi
      .fn<(...args: unknown[]) => Promise<unknown>>()
      .mockResolvedValue(undefined);
    const response: ResponseEnvelope = { status: "ok" };
    const transition: SamlLoginTransition<TestPayload> = { kind: "finish" };

    await executeSamlLoginTransition({
      transition,
      response,
      payload: basePayload,
      showError: vi.fn<(...args: unknown[]) => unknown>(),
      showInfo: vi.fn<(...args: unknown[]) => unknown>(),
      setPendingMfa: vi.fn<(...args: unknown[]) => unknown>(),
      invalidateSession: vi.fn<(...args: unknown[]) => unknown>(),
      navigate: vi.fn<(...args: unknown[]) => unknown>(),
      finishLogin,
    });

    expect(finishLogin).toHaveBeenCalledWith(response, basePayload);
  });

  it("executes consent transition by navigating to consent URL", async () => {
    const navigate = vi.fn<(...args: unknown[]) => unknown>();
    const transition: SamlLoginTransition<TestPayload> = {
      kind: "consent",
      targetUrl: "/consent/my-app",
    };

    await executeSamlLoginTransition({
      transition,
      response: { status: "ok" },
      payload: basePayload,
      showError: vi.fn<(...args: unknown[]) => unknown>(),
      showInfo: vi.fn<(...args: unknown[]) => unknown>(),
      setPendingMfa: vi.fn<(...args: unknown[]) => unknown>(),
      invalidateSession: vi.fn<(...args: unknown[]) => unknown>(),
      navigate,
      finishLogin: vi.fn<(...args: unknown[]) => unknown>(),
    });

    expect(navigate).toHaveBeenCalledWith("/consent/my-app", { replace: true });
  });

  it("executes required-mfa-setup transition by invalidating session and navigating", async () => {
    const navigate = vi.fn<(...args: unknown[]) => unknown>();
    const invalidateSession = vi
      .fn<(...args: unknown[]) => Promise<unknown>>()
      .mockResolvedValue(undefined);
    const transition: SamlLoginTransition<TestPayload> = {
      kind: "required-mfa-setup",
      targetUrl: "/mfa/setup",
      shouldInvalidateSession: true,
    };

    await executeSamlLoginTransition({
      transition,
      response: { status: "ok" },
      payload: basePayload,
      showError: vi.fn<(...args: unknown[]) => unknown>(),
      showInfo: vi.fn<(...args: unknown[]) => unknown>(),
      setPendingMfa: vi.fn<(...args: unknown[]) => unknown>(),
      invalidateSession,
      navigate,
      finishLogin: vi.fn<(...args: unknown[]) => unknown>(),
    });

    expect(invalidateSession).toHaveBeenCalled();
    expect(navigate).toHaveBeenCalledWith("/mfa/setup");
  });
});

describe("toMfaProps", () => {
  it("maps available MFA methods to MfaPropsItem", () => {
    const result: LoginMfaRequired = {
      status: "mfa_required",
      challengeId: "ch-1",
      nextMfa: "totp",
      requiredMfa: ["totp"],
      availableMfaMethods: [
        { id: "m1", methodType: "totp", displayName: "Authenticator" },
        { id: "m2", methodType: "sms", target: "+1234567890", countryCode: "+1" },
      ],
    };

    const props = toMfaProps(result);
    expect(props).toHaveLength(2);
    expect(props[0].mfaType).toBe("totp");
    expect(props[0].isPreferred).toBe(true);
    expect(props[1].mfaType).toBe("sms");
    expect(props[1].isPreferred).toBe(false);
    expect(props[1].secret).toBe("+1234567890");
    expect(props[1].countryCode).toBe("+1");
  });

  it("handles empty available MFA methods", () => {
    const result: LoginMfaRequired = {
      status: "mfa_required",
      challengeId: "ch-1",
      requiredMfa: [],
      availableMfaMethods: [],
    };

    const props = toMfaProps(result);
    expect(props).toHaveLength(0);
  });
});

describe("buildConsentRoute", () => {
  it("builds consent route with encoded application name", () => {
    const route = buildConsentRoute({ applicationName: "my app", locationSearch: "?state=abc" });
    expect(route).toBe("/consent/my%20app?state=abc");
  });

  it("handles application name with special characters", () => {
    const route = buildConsentRoute({ applicationName: "app/name", locationSearch: "" });
    expect(route).toContain("/consent/");
    expect(route).toContain(encodeURIComponent("app/name"));
  });
});

describe("Login Transition State Machine", () => {
  type LoginState = "idle" | "loading" | "mfa_required" | "success" | "error";

  interface TransitionInput {
    from: LoginState;
    event: string;
    to: LoginState;
  }

  const transitions: TransitionInput[] = [
    { from: "idle", event: "SUBMIT", to: "loading" },
    { from: "loading", event: "SUCCESS", to: "success" },
    { from: "loading", event: "MFA_REQUIRED", to: "mfa_required" },
    { from: "loading", event: "ERROR", to: "error" },
    { from: "mfa_required", event: "MFA_SUCCESS", to: "success" },
    { from: "mfa_required", event: "MFA_ERROR", to: "error" },
    { from: "error", event: "RETRY", to: "idle" },
  ];

  function getNextState(current: LoginState, event: string): LoginState {
    const transition = transitions.find((t) => t.from === current && t.event === event);
    return transition ? transition.to : current;
  }

  it("transitions from idle to loading on SUBMIT", () => {
    expect(getNextState("idle", "SUBMIT")).toBe("loading");
  });

  it("transitions from loading to success on SUCCESS", () => {
    expect(getNextState("loading", "SUCCESS")).toBe("success");
  });

  it("transitions from loading to mfa_required on MFA_REQUIRED", () => {
    expect(getNextState("loading", "MFA_REQUIRED")).toBe("mfa_required");
  });

  it("transitions from loading to error on ERROR", () => {
    expect(getNextState("loading", "ERROR")).toBe("error");
  });

  it("transitions from mfa_required to success on MFA_SUCCESS", () => {
    expect(getNextState("mfa_required", "MFA_SUCCESS")).toBe("success");
  });

  it("transitions from mfa_required to error on MFA_ERROR", () => {
    expect(getNextState("mfa_required", "MFA_ERROR")).toBe("error");
  });

  it("transitions from error to idle on RETRY", () => {
    expect(getNextState("error", "RETRY")).toBe("idle");
  });

  it("stays in same state for unknown event", () => {
    expect(getNextState("idle", "UNKNOWN")).toBe("idle");
  });

  it("full login flow: idle → loading → success", () => {
    let state: LoginState = "idle";
    state = getNextState(state, "SUBMIT");
    expect(state).toBe("loading");
    state = getNextState(state, "SUCCESS");
    expect(state).toBe("success");
  });

  it("MFA flow: idle → loading → mfa_required → success", () => {
    let state: LoginState = "idle";
    state = getNextState(state, "SUBMIT");
    state = getNextState(state, "MFA_REQUIRED");
    expect(state).toBe("mfa_required");
    state = getNextState(state, "MFA_SUCCESS");
    expect(state).toBe("success");
  });

  it("error recovery flow: idle → loading → error → idle → loading → success", () => {
    let state: LoginState = "idle";
    state = getNextState(state, "SUBMIT");
    state = getNextState(state, "ERROR");
    expect(state).toBe("error");
    state = getNextState(state, "RETRY");
    expect(state).toBe("idle");
    state = getNextState(state, "SUBMIT");
    state = getNextState(state, "SUCCESS");
    expect(state).toBe("success");
  });

  it("cannot transition from success to any state", () => {
    expect(getNextState("success", "SUBMIT")).toBe("success");
    expect(getNextState("success", "ERROR")).toBe("success");
  });

  it("cannot transition from idle with non-SUBMIT event", () => {
    expect(getNextState("idle", "SUCCESS")).toBe("idle");
    expect(getNextState("idle", "ERROR")).toBe("idle");
  });
});
