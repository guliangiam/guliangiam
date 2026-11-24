import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, waitFor } from "@testing-library/react";
import { createTestWrapper } from "@/test-utils/render";
import { OrganizationThemeProvider } from "./OrganizationThemeProvider";
import { useDiscoveredOrgStore } from "./discovered-org-store";

const {
  mockToken,
  mockGetOrganizationTheme,
  mockGetOrganizationPublicTheme,
  themeState,
  sessionState,
} = vi.hoisted(() => ({
  mockToken: { current: null as Record<string, unknown> | null },
  mockGetOrganizationTheme: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockGetOrganizationPublicTheme: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  themeState: {
    resolvedTheme: "dark" as "dark" | "light",
    config: { primaryColor: "#0b8073", darkPrimaryColor: "#2ec4b6", borderRadius: 10 },
  },
  sessionState: { organization: { id: "org-1" } as { id: string } | null },
}));

vi.mock("antd/es/config-provider", () => ({
  default: ({
    theme,
    children,
  }: {
    theme?: { token?: Record<string, unknown> };
    children?: ReactNode;
  }) => {
    mockToken.current = theme?.token ?? null;
    return <>{children}</>;
  },
}));

vi.mock("antd/es/app", () => ({
  default: ({ children }: { children?: ReactNode }) => <>{children}</>,
}));

vi.mock("@/shared/api/iam/organization", () => ({
  getOrganizationTheme: (...args: unknown[]) => mockGetOrganizationTheme(...args),
  getOrganizationPublicTheme: (...args: unknown[]) => mockGetOrganizationPublicTheme(...args),
}));

vi.mock("@/features/session/session-store", () => ({
  useSessionStore: (selector: (state: unknown) => unknown) => selector(sessionState),
}));

vi.mock("@/features/theme/theme-store", () => ({
  useThemeStore: (selector: (state: unknown) => unknown) => selector(themeState),
  resolvePrimaryColor: () => "#2ec4b6",
}));

vi.mock("@/shared/i18n/antd-locale", () => ({ useAntdLocale: () => undefined }));

function renderProvider() {
  return render(
    <OrganizationThemeProvider>
      <div>child</div>
    </OrganizationThemeProvider>,
    { wrapper: createTestWrapper({ withApp: false }) },
  );
}

describe("OrganizationThemeProvider seed tokens", () => {
  beforeEach(() => {
    mockToken.current = null;
    themeState.resolvedTheme = "dark";
    sessionState.organization = { id: "org-1" };
    mockGetOrganizationTheme.mockResolvedValue({ tokens: { wireframe: true }, customCss: "" });
  });

  // 回归：组织 token 缺省 fontSize 时曾把 undefined 传入 ConfigProvider，
  // 覆盖 antd seed 默认值，派生出 NaN 字号并使 Switch 尺寸塌缩 0×0 不可见。
  it("never emits undefined fontSize when org tokens omit it (dark)", async () => {
    renderProvider();
    await waitFor(() => expect(mockGetOrganizationTheme).toHaveBeenCalled());
    await waitFor(() => expect(mockToken.current).not.toBeNull());
    expect(typeof mockToken.current?.fontSize).toBe("number");
    expect(mockToken.current?.fontSize).toBe(14);
  });

  it("never emits undefined colorLink in light mode", async () => {
    themeState.resolvedTheme = "light";
    renderProvider();
    await waitFor(() => expect(mockToken.current).not.toBeNull());
    expect(typeof mockToken.current?.colorLink).toBe("string");
    expect(mockToken.current?.colorLink).toBeTruthy();
  });

  it("honors an org-provided fontSize", async () => {
    mockGetOrganizationTheme.mockResolvedValue({ tokens: { fontSize: 16 }, customCss: "" });
    renderProvider();
    await waitFor(() => expect(mockToken.current?.fontSize).toBe(16));
  });
});

describe("OrganizationThemeProvider anonymous theme discovery", () => {
  beforeEach(() => {
    mockToken.current = null;
    themeState.resolvedTheme = "light";
    sessionState.organization = null;
    mockGetOrganizationTheme.mockReset();
    mockGetOrganizationPublicTheme.mockReset();
    useDiscoveredOrgStore.getState().setOrgSlug(null);
    document.querySelector("style[data-organization-theme]")?.remove();
  });

  it("fetches the public theme by discovered slug when there is no session organization", async () => {
    useDiscoveredOrgStore.getState().setOrgSlug("acme");
    mockGetOrganizationPublicTheme.mockResolvedValue({ tokens: { fontSize: 18 }, customCss: "" });
    renderProvider();

    await waitFor(() => expect(mockGetOrganizationPublicTheme).toHaveBeenCalledWith("acme"));
    await waitFor(() => expect(mockToken.current?.fontSize).toBe(18));
    expect(mockGetOrganizationTheme).not.toHaveBeenCalled();
  });

  it("injects the public theme customCss tagged with the discovered slug", async () => {
    useDiscoveredOrgStore.getState().setOrgSlug("acme");
    mockGetOrganizationPublicTheme.mockResolvedValue({
      tokens: {},
      customCss: ".gl-panel { border-radius: 4px; }",
    });
    renderProvider();

    await waitFor(() => {
      const style = document.querySelector<HTMLStyleElement>("style[data-organization-theme]");
      expect(style?.textContent).toContain(".gl-panel");
      expect(style?.dataset.organizationTheme).toBe("acme");
    });
  });

  it("keeps the personal theme when no organization context is discovered", async () => {
    renderProvider();

    await waitFor(() => expect(mockToken.current).not.toBeNull());
    expect(mockGetOrganizationPublicTheme).not.toHaveBeenCalled();
    expect(mockGetOrganizationTheme).not.toHaveBeenCalled();
    expect(mockToken.current?.colorPrimary).toBe("#2ec4b6");
  });

  it("prefers the authenticated theme over the discovered slug when a session organization exists", async () => {
    sessionState.organization = { id: "org-1" };
    useDiscoveredOrgStore.getState().setOrgSlug("acme");
    mockGetOrganizationTheme.mockResolvedValue({ tokens: { fontSize: 20 }, customCss: "" });
    renderProvider();

    await waitFor(() => expect(mockToken.current?.fontSize).toBe(20));
    expect(mockGetOrganizationPublicTheme).not.toHaveBeenCalled();
  });

  it("falls back to the personal theme when the public discovery returns null", async () => {
    useDiscoveredOrgStore.getState().setOrgSlug("missing");
    mockGetOrganizationPublicTheme.mockResolvedValue(null);
    renderProvider();

    await waitFor(() => expect(mockGetOrganizationPublicTheme).toHaveBeenCalled());
    await waitFor(() => expect(mockToken.current).not.toBeNull());
    expect(mockToken.current?.colorPrimary).toBe("#2ec4b6");
    expect(document.querySelector("style[data-organization-theme]")).toBeNull();
  });
});
