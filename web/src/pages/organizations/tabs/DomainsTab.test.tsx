import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createTestWrapper } from "@/test-utils/render";
import type { DomainResponse } from "@/shared/api/iam/b2b";
import { DomainsTab } from "./DomainsTab";

const { mockListDomains, mockClaimDomain, mockVerifyDomain, mockDeleteDomain, mockMessage } =
  vi.hoisted(() => ({
    mockListDomains: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
    mockClaimDomain: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
    mockVerifyDomain: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
    mockDeleteDomain: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
    mockMessage: {
      success: vi.fn<(content: unknown) => void>(),
      error: vi.fn<(content: unknown) => void>(),
      warning: vi.fn<(content: unknown) => void>(),
      info: vi.fn<(content: unknown) => void>(),
    },
  }));

vi.mock("antd", async (importOriginal) => {
  const actual = await importOriginal<typeof import("antd")>();
  const AppWithMock = Object.assign(actual.App, {
    useApp: () => ({ message: mockMessage, notification: {}, modal: {} }),
  });
  return { ...actual, App: AppWithMock };
});

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
}));

// 真实 PermissionGuard 渲染，只替换权限来源：缺少 b2b:write 时写入口必须消失。
const permissionState = vi.hoisted(() => ({ actions: [] as string[] }));

vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    permissions: { menus: [], actions: permissionState.actions, resources: [] },
    isLoading: false,
    hasMenu: () => false,
    hasAction: (action: string) => permissionState.actions.includes(action),
    hasResource: () => false,
    refetch: vi.fn<(...args: unknown[]) => unknown>(),
  }),
}));

vi.mock("@/shared/api/iam/b2b", () => ({
  listDomains: (...args: unknown[]) => mockListDomains(...args),
  claimDomain: (...args: unknown[]) => mockClaimDomain(...args),
  verifyDomain: (...args: unknown[]) => mockVerifyDomain(...args),
  deleteDomain: (...args: unknown[]) => mockDeleteDomain(...args),
}));

function domain(overrides: Partial<DomainResponse> = {}): DomainResponse {
  return {
    id: "d1",
    organizationId: "org-1",
    domain: "example.com",
    verificationToken: "token-abc",
    status: "pending",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

function renderTab() {
  return render(<DomainsTab organizationId="org-1" />, { wrapper: createTestWrapper() });
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  permissionState.actions = ["b2b:write"];
  mockListDomains.mockResolvedValue({
    items: [domain()],
    page: { page: 1, pageSize: 100, total: 1 },
  });
  mockClaimDomain.mockResolvedValue(
    domain({ domain: "new.example.com", verificationToken: "txt-value-1" }),
  );
  mockVerifyDomain.mockResolvedValue({ verified: true, domain: domain() });
  mockDeleteDomain.mockResolvedValue(undefined);
});

describe("DomainsTab", () => {
  it("shows the claimed domain together with its DNS TXT verification token", async () => {
    renderTab();

    expect(await screen.findByText("example.com")).toBeInTheDocument();
    expect(screen.getByText("token-abc")).toBeInTheDocument();
    expect(mockListDomains).toHaveBeenCalledWith({
      organizationId: "org-1",
      page: 1,
      pageSize: 100,
    });
  });

  it("claims a domain and surfaces the DNS TXT value to publish", async () => {
    const user = userEvent.setup();
    renderTab();
    await screen.findByText("example.com");

    await user.type(screen.getByLabelText("b2b.domain"), "new.example.com");
    await user.click(screen.getByRole("button", { name: "b2b.claim" }));

    expect(await screen.findByText(/txt-value-1/)).toBeInTheDocument();
    expect(mockClaimDomain.mock.calls[0]![0]).toMatchObject({
      organizationId: "org-1",
      domain: "new.example.com",
    });
  });

  it("reports a verified domain after the verify action succeeds", async () => {
    const user = userEvent.setup();
    renderTab();
    await screen.findByText("example.com");

    await user.click(screen.getByRole("button", { name: "b2b.verify" }));

    await waitFor(() => expect(mockVerifyDomain).toHaveBeenCalledWith("org-1", "d1"));
    expect(mockMessage.success).toHaveBeenCalledWith("b2b.verifySuccess");
  });

  it("reports the not-verified branch when DNS verification does not pass", async () => {
    mockVerifyDomain.mockResolvedValue({ verified: false, domain: domain() });
    const user = userEvent.setup();
    renderTab();
    await screen.findByText("example.com");

    await user.click(screen.getByRole("button", { name: "b2b.verify" }));

    await waitFor(() => expect(mockMessage.warning).toHaveBeenCalledWith("b2b.verifyNotVerified"));
  });

  it("deletes a domain only after the confirmation popup is accepted", async () => {
    const user = userEvent.setup();
    renderTab();
    await screen.findByText("example.com");

    await user.click(screen.getByRole("button", { name: "delete" }));
    expect(mockDeleteDomain).not.toHaveBeenCalled();

    const deleteButtons = await screen.findAllByRole("button", { name: "delete" });
    await user.click(deleteButtons[deleteButtons.length - 1]!);

    await waitFor(() => expect(mockDeleteDomain).toHaveBeenCalledWith("org-1", "d1"));
  });

  it("hides every write entry point when b2b:write is missing", async () => {
    permissionState.actions = [];
    renderTab();

    expect(await screen.findByText("example.com")).toBeInTheDocument();
    expect(screen.queryByLabelText("b2b.domain")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "b2b.claim" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "b2b.verify" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "delete" })).not.toBeInTheDocument();
  });
});
