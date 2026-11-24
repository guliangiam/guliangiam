import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import { createTestWrapper } from "@/test-utils/render";
import type { OrganizationDetail } from "@/shared/api/iam/organization";
import OrganizationDetailPage from "./OrganizationDetailPage";

const {
  mockIdentity,
  mockDetail,
  mockOrganizationOptions,
  mockRegions,
  mockApplicationOptions,
  mockDeleteOrganization,
  mockSaveOrganization,
  mockHasAction,
  mockHasResource,
} = vi.hoisted(() => ({
  mockIdentity: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockDetail: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockOrganizationOptions: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockRegions: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockApplicationOptions: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockDeleteOrganization: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockSaveOrganization: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  mockHasAction: vi.fn<(action: string) => boolean>(),
  mockHasResource: vi.fn<(resource: string) => boolean>(),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
}));

// 默认只授予组织读写删权限；B2B 域名等其它分区保持无权限状态，避免额外请求。
vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    permissions: { menus: [], actions: [], resources: [] },
    isLoading: false,
    hasMenu: () => false,
    hasAction: (action: string) => mockHasAction(action),
    hasResource: (resource: string) => mockHasResource(resource),
    refetch: vi.fn<(...args: unknown[]) => unknown>(),
  }),
}));

vi.mock("@/shared/api/iam/organization", () => ({
  getOrganizationIdentityBySlug: (...args: unknown[]) => mockIdentity(...args),
  getOrganization: (...args: unknown[]) => mockDetail(...args),
  getOrganizationOptions: (...args: unknown[]) => mockOrganizationOptions(...args),
  getDeploymentRegions: (...args: unknown[]) => mockRegions(...args),
  deleteOrganization: (...args: unknown[]) => mockDeleteOrganization(...args),
  saveOrganizationConfiguration: (...args: unknown[]) => mockSaveOrganization(...args),
}));

vi.mock("@/shared/api/iam/application", () => ({
  getApplicationOptions: (...args: unknown[]) => mockApplicationOptions(...args),
}));

vi.mock("@/shared/api/iam/ldap", () => ({
  getIamLdaps: vi.fn<() => Promise<unknown[]>>(async () => []),
}));

vi.mock("@/shared/api/iam/transaction", () => ({
  getIamTransactions: vi.fn<() => Promise<unknown[]>>(async () => []),
}));

function organizationDetail(overrides: Partial<OrganizationDetail> = {}): OrganizationDetail {
  return {
    id: "org-1",
    parentId: "",
    regionCode: "",
    slug: "my-org",
    displayName: "My Org",
    websiteUrl: "",
    logoUrl: "",
    logoDarkUrl: "",
    faviconUrl: "",
    status: "active",
    passwordType: "argon2id",
    passwordSaltSet: false,
    passwordSaltMasked: "",
    passwordObfuscatorType: "Plain",
    passwordObfuscatorKeySet: false,
    passwordObfuscatorKeyMasked: "",
    passwordOptions: [],
    defaultPasswordSet: false,
    defaultPasswordMasked: "",
    enableSoftDeletion: false,
    ipWhitelist: "",
    initiateLoginUrl: "",
    balance: 0,
    currency: "USD",
    disableSignin: false,
    enableDeposit: false,
    balanceCredit: 0,
    mu: 0,
    defaultQuota: 0,
    defaultCurrency: "USD",
    initScore: 0,
    defaultAvatar: "",
    enableDarkLogo: false,
    hasPrivilegeConsent: false,
    ldapAttributes: [],
    settings: {},
    theme: {},
    securityPolicySummary: {},
    masterPasswordSet: false,
    masterPasswordMasked: "",
    masterVerificationCodeSet: false,
    masterVerificationCodeMasked: "",
    isProfilePublic: false,
    useEmailAsUsername: false,
    usePermanentAvatar: false,
    enableTour: false,
    mfaRememberInHours: 12,
    kerberosRealm: "",
    kerberosKdcHost: "",
    kerberosKeytabSet: false,
    kerberosKeytabMasked: "",
    kerberosServiceName: "",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    revision: 1,
    ...overrides,
  };
}

function renderPage() {
  const router = createMemoryRouter(
    [
      { path: "/organizations/:organizationName", element: <OrganizationDetailPage /> },
      { path: "/organizations", element: <div>organizations-list</div> },
    ],
    { initialEntries: ["/organizations/my-org"] },
  );
  return render(<RouterProvider router={router} />, { wrapper: createTestWrapper() });
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  mockIdentity.mockResolvedValue({ id: "org-1", slug: "my-org" });
  mockDetail.mockResolvedValue(organizationDetail());
  mockOrganizationOptions.mockResolvedValue([]);
  mockRegions.mockResolvedValue([]);
  mockApplicationOptions.mockResolvedValue([]);
  mockDeleteOrganization.mockResolvedValue(undefined);
  mockHasAction.mockImplementation(
    (action) => action === "organizations:write" || action === "organizations:delete",
  );
  mockHasResource.mockImplementation((resource) => resource === "organizations");
});

describe("OrganizationDetailPage delete flow", () => {
  it("deletes the organization after the confirmation popup is accepted", async () => {
    const user = userEvent.setup();
    renderPage();

    await screen.findByText("My Org");
    await user.click(screen.getByTestId("organization-delete"));
    expect(mockDeleteOrganization).not.toHaveBeenCalled();

    // Popconfirm 的确认按钮在 portal 中，用容器选择器避免与页面上其它 delete 文案冲突。
    const confirmButton = await waitFor(() => {
      const button = document.querySelector<HTMLButtonElement>(
        ".ant-popconfirm-buttons .ant-btn-primary",
      );
      expect(button).not.toBeNull();
      return button as HTMLButtonElement;
    });
    await user.click(confirmButton);

    await waitFor(() => expect(mockDeleteOrganization).toHaveBeenCalledWith("org-1"));
  });

  it("does not delete the organization when the confirmation is cancelled", async () => {
    const user = userEvent.setup();
    renderPage();

    await screen.findByText("My Org");
    await user.click(screen.getByTestId("organization-delete"));
    await user.click(await screen.findByRole("button", { name: "cancel" }));

    expect(mockDeleteOrganization).not.toHaveBeenCalled();
  });

  it("hides the delete action without the organizations:delete permission", async () => {
    mockHasAction.mockImplementation((action) => action === "organizations:write");
    renderPage();

    await screen.findByText("My Org");
    expect(screen.queryByTestId("organization-delete")).not.toBeInTheDocument();
  });

  it("protects the built-in organization from deletion", async () => {
    const user = userEvent.setup();
    mockDetail.mockResolvedValue(organizationDetail({ slug: "built-in" }));
    renderPage();

    await screen.findByText("My Org");
    const deleteButton = screen.getByTestId("organization-delete");
    expect(deleteButton).toBeDisabled();
    await user.click(deleteButton);
    expect(mockDeleteOrganization).not.toHaveBeenCalled();
  });
});
