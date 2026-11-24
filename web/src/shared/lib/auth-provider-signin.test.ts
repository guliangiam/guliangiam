import { describe, it, expect, vi } from "vitest";
import { expectKind } from "@/test-utils/assertions";
import {
  buildProviderSigninOptions,
  resolveProviderLaunchPlan,
  type ProviderSigninOption,
  type ProviderLaunchPlan,
} from "./auth-provider-signin";

// Mock i18next
// i18n is consumed through the named `t` export; keep default.t for legacy call sites.
vi.mock("i18next", () => {
  const t = (key: string) => key;
  return { default: { t }, t };
});

// Mock auth-route-context
vi.mock("@/shared/lib/auth-route-context", () => ({
  resolveProviderHintUrl: (values: { providerName: string }) =>
    `/login/provider/${values.providerName}`,
}));

// Mock provider-auth
vi.mock("@/shared/lib/provider-auth", () => ({
  canStartDirectOAuthProvider: (provider: { type?: string | null }) =>
    provider.type === "GitHub" || provider.type === "Google",
  getDirectOAuthProviderAuthUrl: vi
    .fn<(...args: unknown[]) => Promise<unknown>>()
    .mockResolvedValue("https://auth.example.com"),
}));

// Mock provider-web3
vi.mock("@/shared/lib/provider-web3", () => ({
  canStartDirectWeb3Provider: (provider: { type?: string | null }) => provider.type === "MetaMask",
  startDirectWeb3ProviderSignin: vi.fn<(...args: unknown[]) => unknown>(),
}));

describe("resolveProviderLaunchPlan", () => {
  it("returns saml plan for SAML provider", () => {
    const provider: ProviderSigninOption = {
      name: "saml-idp",
      category: "SAML",
      canStartDirectly: true,
    };
    const plan = resolveProviderLaunchPlan(provider);
    expect(plan.kind).toBe("saml");
  });

  it("returns oauth plan for OAuth provider without url and canStartDirectly", () => {
    const provider: ProviderSigninOption = {
      name: "github",
      category: "OAuth",
      canStartDirectly: true,
    };
    const plan = resolveProviderLaunchPlan(provider);
    expect(plan.kind).toBe("oauth");
  });

  it("returns url plan for OAuth provider with url", () => {
    const provider: ProviderSigninOption = {
      name: "github",
      category: "OAuth",
      canStartDirectly: true,
      url: "https://github.com/login",
    };
    const plan = resolveProviderLaunchPlan(provider);
    expectKind(plan, "url");
    expect(plan.url).toBe("https://github.com/login");
  });

  it("returns unsupported for Web3 provider (Phase 2: wallet-sign removed)", () => {
    // Phase 2: wallet-sign Web3 login deprecated → Web3 category returns unsupported.
    // Web3 identity binding now goes through Passkey (WebAuthn) registration.
    const provider: ProviderSigninOption = {
      name: "metamask",
      category: "Web3",
      canStartDirectly: true,
    };
    const plan = resolveProviderLaunchPlan(provider);
    expect(plan.kind).toBe("unsupported");
  });

  it("returns url plan when provider has url regardless of category", () => {
    const provider: ProviderSigninOption = {
      name: "custom",
      category: "OAuth",
      canStartDirectly: false,
      url: "https://custom.example.com/login",
    };
    const plan = resolveProviderLaunchPlan(provider);
    expect(plan.kind).toBe("url");
  });

  it("returns unsupported for unknown provider without url", () => {
    const provider: ProviderSigninOption = {
      name: "unknown",
      category: "Unknown",
      canStartDirectly: false,
    };
    const plan = resolveProviderLaunchPlan(provider);
    expect(plan.kind).toBe("unsupported");
  });

  it("returns oauth plan for OAuth that cannot start directly but has no url", () => {
    const provider: ProviderSigninOption = {
      name: "custom-oauth",
      category: "OAuth",
      canStartDirectly: false,
    };
    const plan = resolveProviderLaunchPlan(provider);
    // No url, cannot start directly → unsupported
    expect(plan.kind).toBe("unsupported");
  });
});

describe("buildProviderSigninOptions", () => {
  it("builds options from provider items", () => {
    const providerItems = [
      {
        provider: {
          name: "github",
          type: "GitHub",
          category: "OAuth",
          clientId: "gh-client-id",
          domain: "github.com",
        },
      },
    ];

    const options = buildProviderSigninOptions({
      providerItems,
      pathname: "/login",
      currentHref: "https://example.com/login",
      origin: "https://example.com",
      oauthContext: null,
    });

    expect(options).toHaveLength(1);
    expect(options[0].name).toBe("github");
    expect(options[0].category).toBe("OAuth");
    expect(options[0].clientId).toBe("gh-client-id");
  });

  it("skips provider items without name", () => {
    const providerItems = [{ provider: { type: "GitHub", category: "OAuth" } }];

    const options = buildProviderSigninOptions({
      providerItems,
      pathname: "/login",
      currentHref: "https://example.com/login",
      origin: "https://example.com",
      oauthContext: null,
    });

    expect(options).toHaveLength(0);
  });

  it("sets canStartDirectly for SAML provider", () => {
    const providerItems = [
      {
        provider: {
          name: "saml-idp",
          category: "SAML",
        },
      },
    ];

    const options = buildProviderSigninOptions({
      providerItems,
      pathname: "/login",
      currentHref: "https://example.com/login",
      origin: "https://example.com",
      oauthContext: null,
    });

    expect(options).toHaveLength(1);
    expect(options[0].canStartDirectly).toBe(true);
  });

  it("handles multiple provider items", () => {
    const providerItems = [
      { provider: { name: "github", type: "GitHub", category: "OAuth" } },
      { provider: { name: "google", type: "Google", category: "OAuth" } },
      { provider: { name: "saml", category: "SAML" } },
    ];

    const options = buildProviderSigninOptions({
      providerItems,
      pathname: "/login",
      currentHref: "https://example.com/login",
      origin: "https://example.com",
      oauthContext: null,
    });

    expect(options).toHaveLength(3);
    expect(options[0].name).toBe("github");
    expect(options[1].name).toBe("google");
    expect(options[2].name).toBe("saml");
  });

  it("resolves url from provider hint", () => {
    const providerItems = [{ provider: { name: "github", type: "GitHub", category: "OAuth" } }];

    const options = buildProviderSigninOptions({
      providerItems,
      pathname: "/login",
      currentHref: "https://example.com/login",
      origin: "https://example.com",
      oauthContext: null,
    });

    expect(options[0].url).toContain("github");
  });
});

describe("ProviderLaunchPlan type coverage", () => {
  it("all plan kinds are distinct", () => {
    const plans: ProviderLaunchPlan[] = [
      { kind: "saml" },
      { kind: "oauth" },
      { kind: "web3" },
      { kind: "url", url: "https://example.com" },
      { kind: "unsupported" },
    ];

    const kinds = plans.map((p) => p.kind);
    expect(new Set(kinds).size).toBe(5);
  });
});
