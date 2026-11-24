import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useSessionStore } from "@/features/session/session-store";
import type { ClaimDomainInput, ListDomainsParams } from "@/shared/api/iam/b2b";
import { createTestWrapper } from "@/test-utils/render";
import OrgDomainsPage from "./OrgDomainsPage";

const mockListDomains = vi.fn<(...args: unknown[]) => unknown>();
const mockClaimDomain = vi.fn<(...args: unknown[]) => unknown>();
const mockDeleteDomain = vi.fn<(...args: unknown[]) => unknown>();
const mockVerifyDomain = vi.fn<(...args: unknown[]) => unknown>();
const mockHasAction = vi.fn<(action: string) => boolean>();
const mockGetOrganizationIdentityBySlug =
  vi.fn<(identifier: string) => Promise<{ id: string; slug: string } | null>>();

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, options?: { defaultValue?: string }) => options?.defaultValue ?? key,
    i18n: { language: "en" },
  }),
}));

vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    isLoading: false,
    hasAction: mockHasAction,
    hasResource: () => true,
  }),
}));

vi.mock("@/shared/api/iam/b2b", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/shared/api/iam/b2b")>();
  return {
    ...actual,
    listDomains: (...args: unknown[]) => mockListDomains(...args),
    claimDomain: (...args: unknown[]) => mockClaimDomain(...args),
    deleteDomain: (...args: unknown[]) => mockDeleteDomain(...args),
    verifyDomain: (...args: unknown[]) => mockVerifyDomain(...args),
  };
});

vi.mock("@/shared/api/iam/organization", () => ({
  getOrganizationIdentityBySlug: (identifier: string) =>
    mockGetOrganizationIdentityBySlug(identifier),
}));

vi.mock("@/shared/ui/OrganizationSelect", () => ({
  OrganizationSelect: ({
    value,
    onChange,
    ariaLabel,
  }: {
    value?: string;
    onChange: (value: string) => void;
    ariaLabel?: string;
  }) => (
    <select
      aria-label={ariaLabel ?? "Organization"}
      value={value ?? ""}
      onChange={(event) => onChange(event.target.value)}
    >
      <option value="">Select organization</option>
      <option value="org-1">Organization One</option>
      <option value="org-2">Organization Two</option>
    </select>
  ),
}));

function LocationProbe() {
  const location = useLocation();
  const params = new URLSearchParams(location.search);
  return (
    <>
      <output data-testid="org-param">{params.get("orgId") ?? ""}</output>
      <output data-testid="page-param">{params.get("page") ?? ""}</output>
      <output data-testid="status-param">{params.get("status") ?? ""}</output>
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

function domainFor(organizationId: string) {
  return {
    id: `domain-${organizationId}`,
    organizationId,
    domain: `${organizationId}.example.com`,
    verificationToken: `token-${organizationId}`,
    status: "pending",
    createdAt: "2026-10-27T08:00:00Z",
    updatedAt: "2026-10-27T08:00:00Z",
  };
}

function renderPage(entry: string) {
  const Providers = createTestWrapper();
  return render(
    <MemoryRouter initialEntries={[entry]}>
      <Providers>
        <OrgDomainsPage />
        <LocationProbe />
      </Providers>
    </MemoryRouter>,
  );
}

function renderRoutePage(entry: string) {
  const Providers = createTestWrapper();
  return render(
    <MemoryRouter initialEntries={[entry]}>
      <Providers>
        <Routes>
          <Route path="/organizations/:organizationName/domains" element={<OrgDomainsPage />} />
        </Routes>
        <LocationProbe />
      </Providers>
    </MemoryRouter>,
  );
}

describe("OrgDomainsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockHasAction.mockReturnValue(true);
    mockGetOrganizationIdentityBySlug.mockResolvedValue(null);
    setOrganization("org-1");
    mockListDomains.mockImplementation(async (...args: unknown[]) => {
      const params = args[0] as ListDomainsParams;
      return {
        items: [domainFor(params.organizationId)],
        page: { page: params.page ?? 1, pageSize: params.pageSize ?? 20, total: 1 },
      };
    });
    mockClaimDomain.mockImplementation(async (...args: unknown[]) => {
      const input = args[0] as ClaimDomainInput;
      return { ...domainFor(input.organizationId), domain: input.domain };
    });
  });

  it("restores organization, page and status from the URL", async () => {
    renderPage("/organizations/acme/domains?orgId=org-1&page=2&status=pending&view=full");

    expect(await screen.findByText("org-1.example.com")).toBeInTheDocument();
    await waitFor(() =>
      expect(mockListDomains).toHaveBeenCalledWith({
        organizationId: "org-1",
        page: 2,
        pageSize: 20,
        status: "pending",
      }),
    );
    expect(screen.getByTestId("view-param")).toHaveTextContent("full");
  });

  it("resolves the organization from the route slug when ?orgId is absent", async () => {
    mockGetOrganizationIdentityBySlug.mockResolvedValue({ id: "org-9", slug: "acme" });
    renderRoutePage("/organizations/acme/domains");

    expect(await screen.findByText("org-9.example.com")).toBeInTheDocument();
    expect(mockGetOrganizationIdentityBySlug).toHaveBeenCalledWith("acme");
    await waitFor(() =>
      expect(mockListDomains).toHaveBeenCalledWith({
        organizationId: "org-9",
        page: 1,
        pageSize: 20,
        status: undefined,
      }),
    );
  });

  it("prefers an explicit ?orgId over the route slug resolution", async () => {
    renderRoutePage("/organizations/acme/domains?orgId=org-1");

    expect(await screen.findByText("org-1.example.com")).toBeInTheDocument();
    expect(mockGetOrganizationIdentityBySlug).not.toHaveBeenCalled();
  });

  it("persists organization selection in the URL", async () => {
    const user = userEvent.setup();
    renderPage("/organizations/acme/domains?view=full");

    await user.selectOptions(screen.getByLabelText("Organization"), "org-2");

    await waitFor(() => expect(screen.getByTestId("org-param")).toHaveTextContent("org-2"));
    expect(screen.getByTestId("view-param")).toHaveTextContent("full");
    expect(await screen.findByText("org-2.example.com")).toBeInTheDocument();
  });

  it("clears stale organization filters when the session organization changes", async () => {
    renderPage("/organizations/acme/domains?orgId=org-1&page=2&status=pending&view=full");
    expect(await screen.findByText("org-1.example.com")).toBeInTheDocument();

    act(() => setOrganization("org-2"));

    await waitFor(() => expect(screen.getByTestId("org-param").textContent).toBe(""));
    expect(screen.getByTestId("page-param").textContent).toBe("");
    expect(screen.getByTestId("status-param").textContent).toBe("");
    expect(screen.getByTestId("view-param")).toHaveTextContent("full");
    expect(screen.queryByText("org-1.example.com")).not.toBeInTheDocument();
  });

  it("normalizes a domain before claiming it", async () => {
    const user = userEvent.setup();
    renderPage("/organizations/acme/domains?orgId=org-1");
    expect(await screen.findByText("org-1.example.com")).toBeInTheDocument();

    await user.type(screen.getByLabelText("Domain"), "  CORP.Example.COM.  ");
    await user.click(screen.getByRole("button", { name: "Claim Domain" }));

    await waitFor(() => expect(mockClaimDomain).toHaveBeenCalledOnce());
    expect(mockClaimDomain.mock.calls[0][0]).toEqual({
      organizationId: "org-1",
      domain: "corp.example.com",
    });
  });

  it("hides write actions from read-only users", async () => {
    mockHasAction.mockImplementation((action) => action === "b2b:read");
    renderPage("/organizations/acme/domains?orgId=org-1");

    expect(await screen.findByText("org-1.example.com")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Claim Domain" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Verify" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Delete" })).not.toBeInTheDocument();
  });
});
