import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";

import SocialCallbackPage from "./SocialCallbackPage";
import { useSessionStore } from "@/features/session/session-store";
import { createTestWrapper } from "../../test-utils/render";

const mockSocialLogin = vi.fn<(...args: unknown[]) => unknown>();
const mockGetAuthAccountContext = vi.fn<(...args: unknown[]) => unknown>();
const mockGetProviders = vi.fn<(...args: unknown[]) => unknown>();
const mockNavigate = vi.fn<(...args: unknown[]) => unknown>();

let mockParams: { providerType?: string } = {};
let mockSearchParams = new URLSearchParams();

vi.mock("@/shared/api/iam/auth", () => ({
  socialLogin: (...args: unknown[]) => mockSocialLogin(...args),
  getAuthAccountContext: (...args: unknown[]) => mockGetAuthAccountContext(...args),
}));

vi.mock("@/shared/api/iam/provider-extended", () => ({
  getProviders: (...args: unknown[]) => mockGetProviders(...args),
}));

vi.mock("react-router-dom", () => ({
  useNavigate: () => mockNavigate,
  useParams: () => mockParams,
  useSearchParams: () => [mockSearchParams],
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, options?: { provider?: string }) =>
      options?.provider ? `${key}:${options.provider}` : key,
    i18n: { language: "en" },
  }),
  Trans: ({ children }: { children: React.ReactNode }) => children,
}));

const account = {
  id: "user-1",
  owner: "built-in",
  name: "alice",
  displayName: "Alice",
  avatar: null,
  isAdmin: false,
};

const organization = {
  id: "org-1",
  name: "built-in",
  displayName: "Built-in",
  navItems: [],
  userNavItems: [],
};

const githubProvider = { id: "provider-1", type: "OIDC", name: "github" };

function renderPage() {
  return render(<SocialCallbackPage />, { wrapper: createTestWrapper() });
}

describe("SocialCallbackPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockParams = {};
    mockSearchParams = new URLSearchParams();
    mockSocialLogin.mockResolvedValue({
      status: "success",
      account,
      organization,
    });
    mockGetAuthAccountContext.mockResolvedValue({ account, organization });
    mockGetProviders.mockResolvedValue([githubProvider]);
    useSessionStore.setState({ account: null, organization: null, impersonatedUser: null });
  });

  it("exchanges code + state and navigates to the dashboard on success", async () => {
    mockParams = { providerType: "github" };
    mockSearchParams = new URLSearchParams("code=code-1&state=state-1");

    renderPage();

    await waitFor(() => {
      expect(mockSocialLogin).toHaveBeenCalledWith({
        providerId: "provider-1",
        code: "code-1",
        state: "state-1",
        applicationId: "app-built-in",
      });
    });
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/", { replace: true });
    });
    expect(useSessionStore.getState().account).toEqual(account);
  });

  it("does not call the login API and shows an actionable error when code is missing", async () => {
    mockParams = { providerType: "github" };
    mockSearchParams = new URLSearchParams("state=state-1");

    renderPage();

    expect(await screen.findByText("common:socialCallback.failed")).toBeInTheDocument();
    expect(screen.getByText("common:socialCallback.missingParams")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "common:socialCallback.tryAgain" }),
    ).toBeInTheDocument();
    expect(mockGetProviders).not.toHaveBeenCalled();
    expect(mockSocialLogin).not.toHaveBeenCalled();
  });

  it("does not call the login API and shows an actionable error when state is missing", async () => {
    mockParams = { providerType: "github" };
    mockSearchParams = new URLSearchParams("code=code-1");

    renderPage();

    expect(await screen.findByText("common:socialCallback.failed")).toBeInTheDocument();
    expect(screen.getByText("common:socialCallback.missingParams")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "common:socialCallback.goHome" }),
    ).toBeInTheDocument();
    expect(mockGetProviders).not.toHaveBeenCalled();
    expect(mockSocialLogin).not.toHaveBeenCalled();
  });

  it("shows a provider-not-found error instead of crashing for an unknown providerType", async () => {
    mockParams = { providerType: "unknown-provider" };
    mockSearchParams = new URLSearchParams("code=code-1&state=state-1");

    renderPage();

    expect(
      await screen.findByText("common:socialCallback.providerNotFound:unknown-provider"),
    ).toBeInTheDocument();
    expect(mockSocialLogin).not.toHaveBeenCalled();
  });

  it("shows the provider load failure state and does not silently retry", async () => {
    mockParams = { providerType: "github" };
    mockSearchParams = new URLSearchParams("code=code-1&state=state-1");
    mockGetProviders.mockRejectedValue(new Error("provider api down"));

    renderPage();

    expect(await screen.findByText("common:socialCallback.providerLoadFailed")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "common:socialCallback.tryAgain" }),
    ).toBeInTheDocument();
    expect(mockSocialLogin).not.toHaveBeenCalled();

    // Give a would-be automatic retry a chance to fire, then assert it did not.
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(mockGetProviders).toHaveBeenCalledTimes(1);
  });
});
