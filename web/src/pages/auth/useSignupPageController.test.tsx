import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { createTestQueryClient } from "@/test-utils/render";
import { useSignupPageController } from "./useSignupPageController";
import type { ApplicationPublicDetail } from "@/shared/api/iam/application";

const resolveApplicationMock =
  vi.fn<(ownerSlug: string, appSlug: string) => Promise<ApplicationPublicDetail | null>>();

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key, i18n: { language: "en" } }),
}));

vi.mock("antd", async (importOriginal) => {
  const actual = await importOriginal<typeof import("antd")>();
  return {
    ...actual,
    App: {
      ...actual.App,
      useApp: () => ({
        message: { success: vi.fn<() => void>(), error: vi.fn<() => void>() },
      }),
    },
  };
});

vi.mock("@/shared/api/iam/application", () => ({
  resolveApplication: (ownerSlug: string, appSlug: string) =>
    resolveApplicationMock(ownerSlug, appSlug),
}));

vi.mock("@/shared/api/iam/auth-signup", () => ({
  signup: vi.fn<(...args: unknown[]) => unknown>(),
}));

vi.mock("@/shared/api/iam/invitation", () => ({
  verifyInvitation: vi.fn<(...args: unknown[]) => unknown>(),
}));

let captchaProviderRequired = false;

vi.mock("@/shared/hooks/useCaptchaChallenge", () => ({
  useCaptchaChallenge: () => ({
    required: captchaProviderRequired,
    state: "idle",
    seed: 0,
    token: "",
    captchaId: "",
    setToken: vi.fn<(token: string) => void>(),
    reload: vi.fn<() => void>(),
  }),
}));

function publicApplication(
  overrides: Partial<ApplicationPublicDetail> = {},
): ApplicationPublicDetail {
  return {
    id: "app-1",
    organizationId: "org-1",
    slug: "my-app",
    name: "my-app",
    displayName: "My App",
    description: "",
    homepageUrl: "",
    clientId: "client-1",
    logo: "",
    title: "",
    favicon: "",
    isShared: false,
    enableAutoSignin: false,
    enableSignup: true,
    forcedRedirectOrigin: "",
    signupUrl: "",
    forgetUrl: "",
    termsOfUse: "",
    privacyPolicy: "",
    orgChoiceMode: "",
    signinHtml: "",
    signupHtml: "",
    headerHtml: "",
    footerHtml: "",
    customCss: "",
    customHeader: "",
    formCss: "",
    formCssMobile: "",
    formSideHtml: "",
    formBackgroundUrl: "",
    formBackgroundUrlMobile: "",
    formOffset: "2",
    signinMethods: [],
    signinItems: [],
    signupItems: [],
    ...overrides,
  };
}

function renderController(entry: string) {
  const queryClient = createTestQueryClient();
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[entry]}>
        <Routes>
          <Route path="/signup/:organizationName" element={children} />
          <Route path="/signup" element={children} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
  return renderHook(() => useSignupPageController(), { wrapper });
}

describe("useSignupPageController application discovery", () => {
  beforeEach(() => {
    resolveApplicationMock.mockReset();
    captchaProviderRequired = false;
  });

  it("resolves the application through the anonymous public projection", async () => {
    resolveApplicationMock.mockResolvedValue(
      publicApplication({ signupItems: [{ name: "Email", visible: true, required: true }] }),
    );
    const { result } = renderController("/signup/my-org?application=my-app");

    await waitFor(() => {
      expect(result.current.application?.name).toBe("my-app");
    });
    expect(resolveApplicationMock).toHaveBeenCalledWith("my-org", "my-app");
    expect(result.current.signupItems.map((item) => item.name)).toEqual(["Email"]);
  });

  it("falls back to the organization default application without ?application=", async () => {
    resolveApplicationMock.mockResolvedValue(
      publicApplication({ slug: "app-built-in", name: "app-built-in" }),
    );
    renderController("/signup/my-org");

    await waitFor(() => {
      expect(resolveApplicationMock).toHaveBeenCalledWith("my-org", "app-built-in");
    });
  });

  it("keeps platform defaults when the application cannot be resolved", async () => {
    resolveApplicationMock.mockResolvedValue(null);
    const { result } = renderController("/signup/my-org?application=missing");

    await waitFor(() => {
      expect(result.current.application).toBeNull();
    });
    expect(result.current.signupDisabled).toBe(false);
    expect(result.current.signupItems.length).toBeGreaterThan(0);
    expect(result.current.outerBackground).toContain("var(--gl-color-bg)");
  });

  it("disables signup when the application turns enableSignup off", async () => {
    resolveApplicationMock.mockResolvedValue(publicApplication({ enableSignup: false }));
    const { result } = renderController("/signup/my-org?application=my-app");

    await waitFor(() => {
      expect(result.current.signupDisabled).toBe(true);
    });
  });

  it("derives shell presentation from the same rules as the login page", async () => {
    resolveApplicationMock.mockResolvedValue(
      publicApplication({
        formBackgroundUrl: "https://cdn.example.com/bg.png",
        formOffset: "4",
        signupHtml: "<div>full page</div>",
      }),
    );
    const { result } = renderController("/signup/my-org?application=my-app");

    await waitFor(() => {
      expect(result.current.outerBackground).toContain("url(https://cdn.example.com/bg.png)");
    });
    expect(result.current.isSplitLayout).toBe(true);
    expect(result.current.formPositionStyles).toEqual({
      justifyContent: "center",
      alignItems: "stretch",
    });
    expect(result.current.customSignupHtml).toBe("<div>full page</div>");
  });
});

describe("useSignupPageController captcha gating", () => {
  beforeEach(() => {
    resolveApplicationMock.mockReset();
    captchaProviderRequired = false;
  });

  it("follows the organization captcha provider when the app has no Captcha signup item", async () => {
    captchaProviderRequired = true;
    resolveApplicationMock.mockResolvedValue(
      publicApplication({ signupItems: [{ name: "Email", visible: true, required: false }] }),
    );
    const { result } = renderController("/signup/my-org?application=my-app");

    await waitFor(() => {
      expect(result.current.application?.name).toBe("my-app");
    });
    expect(result.current.captchaRequired).toBe(true);
  });

  it("turns the challenge off when the app hides its Captcha signup item", async () => {
    captchaProviderRequired = true;
    resolveApplicationMock.mockResolvedValue(
      publicApplication({
        signupItems: [{ name: "Captcha", visible: false, required: false }],
      }),
    );
    const { result } = renderController("/signup/my-org?application=my-app");

    await waitFor(() => {
      expect(result.current.application?.name).toBe("my-app");
    });
    expect(result.current.captchaRequired).toBe(false);
  });

  it("never demands a challenge when the organization has no captcha provider", async () => {
    captchaProviderRequired = false;
    resolveApplicationMock.mockResolvedValue(
      publicApplication({
        signupItems: [{ name: "Captcha", visible: true, required: true }],
      }),
    );
    const { result } = renderController("/signup/my-org?application=my-app");

    await waitFor(() => {
      expect(result.current.application?.name).toBe("my-app");
    });
    expect(result.current.captchaRequired).toBe(false);
  });
});
