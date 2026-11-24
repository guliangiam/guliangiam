import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useSessionStore } from "@/features/session/session-store";
import { createTestWrapper } from "@/test-utils/render";
import WorkloadPage from "./WorkloadPage";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, options?: { defaultValue?: string }) => options?.defaultValue ?? key,
    i18n: { language: "en" },
  }),
}));

vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    isLoading: false,
    hasAction: () => true,
    hasResource: () => true,
  }),
}));

vi.mock("@/shared/hooks/use-identity-pools", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/shared/hooks/use-identity-pools")>();
  return {
    ...actual,
    useFederationTrusts: () => ({ data: [] }),
    useSetFederationTrustStatus: () => ({
      isPending: false,
      mutateAsync: vi.fn<(...args: unknown[]) => unknown>(),
    }),
    useDeleteFederationTrust: () => ({
      isPending: false,
      mutateAsync: vi.fn<(...args: unknown[]) => unknown>(),
    }),
  };
});

vi.mock("./IdentityPoolsTab", () => ({
  IdentityPoolsTab: ({
    activePoolId,
    onSelectPool,
  }: {
    activePoolId?: string;
    onSelectPool: (id: string) => void;
  }) => (
    <div>
      <span>Active pool: {activePoolId ?? "none"}</span>
      <button onClick={() => onSelectPool("pool-2")}>Select pool-2</button>
    </div>
  ),
}));

vi.mock("./ServiceAccountsTab", () => ({
  ServiceAccountsTab: ({ poolId }: { poolId?: string }) => (
    <div>Service accounts: {poolId ?? "none"}</div>
  ),
}));

function LocationProbe() {
  const location = useLocation();
  const params = new URLSearchParams(location.search);
  return (
    <>
      <output data-testid="pool-param">{params.get("poolId") ?? ""}</output>
      <output data-testid="view-param">{params.get("view") ?? ""}</output>
    </>
  );
}

function setOrganization(id: string) {
  useSessionStore.setState({
    account: null,
    organization: { id, name: id, displayName: id, navItems: [], userNavItems: [] },
    impersonatedUser: null,
  });
}

function renderPage(entry: string) {
  const Providers = createTestWrapper();
  return render(
    <MemoryRouter initialEntries={[entry]}>
      <Providers>
        <WorkloadPage />
        <LocationProbe />
      </Providers>
    </MemoryRouter>,
  );
}

describe("WorkloadPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setOrganization("org-1");
  });

  it("restores poolId on a fresh mount and persists a new selection", async () => {
    const user = userEvent.setup();
    renderPage("/identity-pools?poolId=pool-from-url&view=full");

    expect(screen.getByText("Active pool: pool-from-url")).toBeInTheDocument();
    expect(screen.getByTestId("pool-param")).toHaveTextContent("pool-from-url");

    await user.click(screen.getByRole("button", { name: "Select pool-2" }));

    await waitFor(() => expect(screen.getByTestId("pool-param")).toHaveTextContent("pool-2"));
    expect(screen.getByTestId("view-param")).toHaveTextContent("full");
    expect(screen.getByText("Active pool: pool-2")).toBeInTheDocument();
  });

  it("shows pool selection guidance in trusts when no pool is selected", async () => {
    const user = userEvent.setup();
    renderPage("/identity-pools");

    await user.click(screen.getByRole("tab", { name: "Federation Trusts" }));

    expect(await screen.findByText("Select an Identity Pool first")).toBeInTheDocument();
  });

  it("clears a stale poolId when the session organization changes", async () => {
    renderPage("/identity-pools?poolId=pool-from-org-1&view=full");
    expect(screen.getByTestId("pool-param")).toHaveTextContent("pool-from-org-1");

    act(() => setOrganization("org-2"));

    await waitFor(() => expect(screen.getByTestId("pool-param").textContent).toBe(""));
    expect(screen.getByTestId("view-param")).toHaveTextContent("full");
    expect(screen.getByText("Active pool: none")).toBeInTheDocument();
  });
});
