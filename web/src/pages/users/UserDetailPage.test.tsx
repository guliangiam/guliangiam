import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { App } from "antd";
import { QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createTestQueryClient } from "@/test-utils/render";
import type { UserDetail } from "@/shared/api/iam/user";
import UserDetailPage from "./UserDetailPage";
import { USER_DETAIL_TABS } from "./user-detail-tabs";

/**
 * Page-level save closure for UserDetailPage.
 *
 * The page owns one shared form and routes the header Save button by active section:
 * basic/account → PATCH /users then PUT /profile (and PUT /credentials when
 * `needUpdatePassword` changed), realname → PUT /profile, credentials →
 * PUT /credentials. Everything else saves through its own guard.
 */
const api = vi.hoisted(() => ({
  getUserByOrganizationSlugAndUsername: vi.fn<() => Promise<unknown>>(),
  updateUser: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  updateUserProfile: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  updateUserCredentials: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  verifyIdentification: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  replaceUserIdentities: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  replaceUserRoleBindings: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  replaceUserGroupMemberships: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  replaceUserMfaMethods: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  updateUserPreferredMfa: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  deleteUser: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  listUserTrustedDevices: vi.fn<() => Promise<unknown[]>>(),
  revokeUserTrustedDevice: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  listUserConsents: vi.fn<() => Promise<unknown[]>>(),
  revokeConsent: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
  listUserTransactions: vi.fn<() => Promise<unknown>>(),
  listUserOrders: vi.fn<() => Promise<unknown>>(),
  listUserPayments: vi.fn<() => Promise<unknown>>(),
}));

const queries = vi.hoisted(() => ({
  getRoleOptions: vi.fn<() => Promise<unknown[]>>(),
  getGroups: vi.fn<() => Promise<unknown[]>>(),
  getOrganization: vi.fn<() => Promise<unknown>>(),
  getApplicationOptions: vi.fn<() => Promise<unknown[]>>(),
  prefetchUserRelated: vi.fn<() => void>(),
}));

// CloudAuth SDK 在 jsdom 里注入真实脚本没有意义（且会等待 8s 超时）：桩化采集层，
// 页面测试只验「拿到 metaInfo 才发请求、失败就不发」这一契约。
const sdk = vi.hoisted(() => {
  class CloudAuthMetaError extends Error {
    reason: string;
    constructor(reason: string) {
      super(`cloudauth: ${reason}`);
      this.name = "CloudAuthMetaError";
      this.reason = reason;
    }
  }
  return {
    collectMetaInfo: vi.fn<() => Promise<string>>(),
    buildIdvReturnUrl: (path: string) => `https://console.test${path}`,
    CloudAuthMetaError,
  };
});

const idv = vi.hoisted(() => ({
  getIDVVerification: vi.fn<() => Promise<unknown>>(),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key, i18n: { language: "en" } }),
}));

vi.mock("@/shared/api/iam/user", () => api);
vi.mock("@/shared/api/iam/session", () => ({
  listUserSessions: vi.fn<() => Promise<unknown[]>>(async () => []),
  revokeUserSession: vi.fn<(...args: unknown[]) => Promise<unknown>>(async () => undefined),
  forceLogoutUserSessions: vi.fn<(...args: unknown[]) => Promise<unknown>>(async () => undefined),
}));
vi.mock("@/shared/api/iam/auth", () => ({
  socialAuthorize: vi.fn<() => Promise<unknown>>(async () => ({ redirectUrl: "" })),
}));
vi.mock("@/shared/api/iam/role", () => ({ getRoleOptions: queries.getRoleOptions }));
vi.mock("@/shared/api/iam/group", () => ({ getGroups: queries.getGroups }));
vi.mock("@/shared/api/iam/organization", () => ({ getOrganization: queries.getOrganization }));
vi.mock("@/shared/api/iam/application", () => ({
  getApplicationOptions: queries.getApplicationOptions,
}));
vi.mock("@/shared/lib/cloudauth-sdk", () => sdk);
vi.mock("@/shared/api/iam/idv", () => idv);

vi.mock("@/shared/hooks/usePrefetch", () => ({
  usePrefetch: () => ({ prefetchUserRelated: queries.prefetchUserRelated }),
}));
vi.mock("@/shared/hooks/useProviderOptions", () => ({
  useProviderOptions: () => ({ data: [], isLoading: false }),
}));
vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    hasAction: (action: string) => action !== "users:password:reset",
    hasResource: () => true,
    isLoading: false,
  }),
}));

vi.mock("@/shared/ui/ImageCropUpload", () => ({ ImageCropUpload: () => null }));
vi.mock("@/shared/ui/CountryCodeSelect", () => ({ CountryCodeSelect: () => <input /> }));
vi.mock("@/shared/ui/RegionSelect", () => ({ RegionSelect: () => <input /> }));
vi.mock("@/shared/ui/AddressTable", () => ({ default: () => null }));
vi.mock("@/shared/ui/ResetContactModal", () => ({ ResetContactModal: () => null }));
vi.mock("@/shared/ui/OAuthWidget", () => ({ OAuthWidget: () => null }));
vi.mock("@/shared/ui/SamlWidget", () => ({ SamlWidget: () => null }));
vi.mock("@/shared/ui/ConsentTable", () => ({ default: () => null }));
vi.mock("@/shared/ui/FaceIdTable", () => ({ default: () => null }));
vi.mock("@/pages/users/tabs/WebAuthnCredentialsTab", () => ({ default: () => null }));

function buildUser(): UserDetail {
  return {
    id: "u-1",
    organizationId: "built-in",
    username: "alice",
    displayName: "Alice",
    avatarUrl: "",
    email: "alice@example.com",
    phone: "+8613800000000",
    userType: "normal",
    status: "active",
    isAdmin: false,
    isForbidden: false,
    isDeleted: false,
    isVerified: false,
    tag: "",
    score: 0,
    karma: 12,
    ranking: 5,
    balance: 0,
    lastSigninAt: null,
    createdAt: "2026-01-01T00:00:00Z",
    externalId: "ext-1",
    signupApplicationId: "app-1",
    registerType: "signup",
    registerSource: "web",
    preferredMfaType: "",
    recoveryCodes: [],
    mfaSetupSummary: [],
    profile: {
      displayName: "Alice",
      email: "alice@example.com",
      phone: "+8613800000000",
      homepage: "https://alice.example.com",
      language: "en",
      realName: "Alice Chen",
      idCardType: "id_card",
      // 规范 9.5：响应不含证件号原文，只有 Set/Masked 对。
      idCardSet: true,
      idCardMasked: "******",
      properties: {
        department: "Engineering",
        karma: 12,
        ranking: 5,
        homepage: "https://alice.example.com",
      },
    },
    credentialSummary: { needUpdatePassword: false, hasPassword: true },
    identities: [
      {
        id: "identity-1",
        providerId: "github",
        identityType: "oauth2",
        subject: "alice-gh",
        providerAccount: "alice",
      },
    ],
    mfaMethods: [{ methodType: "totp", enabled: true, verified: true, displayName: "TOTP" }],
    webauthnCredentialCount: 0,
    roleBindings: [{ roleId: "role-1", roleName: "Administrator" }],
    groupMemberships: [{ groupId: "group-1", groupName: "Operators" }],
    consentSummary: {},
    balanceCredit: 0,
    balanceCurrency: "CNY",
    ipWhitelist: "",
    addresses: [],
    managedAccounts: [],
    faceIds: [],
    cart: [],
    mfaItems: [],
    mfaAccounts: [],
    applicationScopes: [],
    custom1: "",
    custom2: "",
    custom3: "",
    custom4: "",
    custom5: "",
    custom6: "",
    custom7: "",
    custom8: "",
    custom9: "",
    custom10: "",
    revision: 3,
  } as unknown as UserDetail;
}

function renderPage(initialEntry = "/users/built-in/alice") {
  const queryClient = createTestQueryClient();
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
  const router = createMemoryRouter(
    [
      { path: "/users/:organizationName/:userName", element: <UserDetailPage /> },
      { path: "/users", element: <div data-testid="users-list" /> },
    ],
    { initialEntries: [initialEntry] },
  );

  const utils = render(
    <QueryClientProvider client={queryClient}>
      <App component={false}>
        <RouterProvider router={router} />
      </App>
    </QueryClientProvider>,
  );

  return { ...utils, queryClient, invalidateSpy };
}

/**
 * 分区导航改为锚点滚动后，所有分区内容同时挂载；但页头保存动作仍按「当前分区」
 * （`?tab=`）路由，因此测试需点一次分区导航条目把 activeTab 落到位。
 */
async function openTab(user: ReturnType<typeof userEvent.setup>, labelKey: string) {
  await user.click(await screen.findByRole("link", { name: labelKey }));
}

async function confirmPopover(name: string) {
  const user = userEvent.setup();
  const button = await waitFor(() => {
    const candidate = screen
      .queryAllByRole("button", { name })
      .find((element) => element.closest(".ant-popover"));
    if (!candidate) throw new Error(`popconfirm button ${name} not found`);
    return candidate;
  });
  await user.click(button);
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  api.getUserByOrganizationSlugAndUsername.mockResolvedValue(buildUser());
  api.updateUser.mockResolvedValue(undefined);
  api.updateUserProfile.mockResolvedValue(undefined);
  api.updateUserCredentials.mockResolvedValue(undefined);
  api.verifyIdentification.mockResolvedValue(undefined);
  sdk.collectMetaInfo.mockResolvedValue(JSON.stringify({ deviceType: "web" }));
  // 轮询在挂载时就会跑一次（localStorage 里可能残留上一次的待收尾标记）：给个
  // 非终态返回值，测试只关心请求是否发生，不关心结果。
  idv.getIDVVerification.mockResolvedValue({ status: "pending" });
  api.replaceUserIdentities.mockResolvedValue(undefined);
  api.replaceUserRoleBindings.mockResolvedValue(undefined);
  api.replaceUserGroupMemberships.mockResolvedValue(undefined);
  api.replaceUserMfaMethods.mockResolvedValue(undefined);
  api.updateUserPreferredMfa.mockResolvedValue(undefined);
  api.deleteUser.mockResolvedValue(undefined);
  api.listUserTrustedDevices.mockResolvedValue([]);
  api.listUserConsents.mockResolvedValue([]);
  api.listUserTransactions.mockResolvedValue({ items: [], total: 0 });
  api.listUserOrders.mockResolvedValue({ items: [], total: 0 });
  api.listUserPayments.mockResolvedValue({ items: [], total: 0 });
  queries.getRoleOptions.mockResolvedValue([
    { id: "role-2", name: "auditor", displayName: "Auditor" },
  ]);
  queries.getGroups.mockResolvedValue([
    {
      id: "group-2",
      owner: "built-in",
      name: "engineering",
      displayName: "Engineering",
      type: "virtual",
      parentId: "",
      isTopGroup: true,
      isEnabled: true,
    },
  ]);
  queries.getOrganization.mockResolvedValue({ id: "built-in", settings: { accountItems: [] } });
  queries.getApplicationOptions.mockResolvedValue([]);
});

describe("UserDetailPage basic/account save", () => {
  it("saves the basic tab through PATCH then PUT and invalidates user caches", async () => {
    const user = userEvent.setup();
    const { invalidateSpy } = renderPage();

    const externalId = await screen.findByLabelText("resource:user.externalId");
    await user.clear(externalId);
    await user.type(externalId, "ext-2");
    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => expect(api.updateUser).toHaveBeenCalledTimes(1));
    expect(api.updateUser.mock.calls[0][0]).toBe("u-1");
    expect(api.updateUser.mock.calls[0][1]).toMatchObject({
      externalId: "ext-2",
      revision: 3,
    });

    expect(api.updateUserProfile.mock.calls[0][0]).toBe("u-1");
    expect(api.updateUserProfile.mock.calls[0][1]).toMatchObject({
      displayName: "Alice",
      email: "alice@example.com",
      homepage: "https://alice.example.com",
      language: "en",
      properties: { department: "Engineering", karma: 12, ranking: 5 },
    });
    expect(api.updateUserCredentials).not.toHaveBeenCalled();
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["user"] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["users"] });
  });

  it("runs the three serial saves when needUpdatePassword changes before saving", async () => {
    const user = userEvent.setup();
    const order: string[] = [];
    api.updateUser.mockImplementation(async () => {
      order.push("users");
    });
    api.updateUserProfile.mockImplementation(async () => {
      order.push("profile");
    });
    api.updateUserCredentials.mockImplementation(async () => {
      order.push("credentials");
    });
    renderPage();

    await openTab(user, "resource:user.tabs.credentials");
    const needUpdatePassword = await screen.findByLabelText("resource:user.needUpdatePassword");
    await user.click(needUpdatePassword);

    await openTab(user, "resource:user.tabs.basic");
    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => expect(api.updateUserCredentials).toHaveBeenCalledTimes(1));
    expect(order).toEqual(["users", "profile", "credentials"]);
    expect(api.updateUserCredentials.mock.calls[0][0]).toBe("u-1");
    // 凭据阶段现在统一用 buildCredentialsPayload，显式带上两个开关（false = 不轮换）。
    expect(api.updateUserCredentials.mock.calls[0][1]).toEqual({
      rotateAccessSecret: false,
      needUpdatePassword: true,
    });
  });

  it("saves the account tab through PATCH then PUT without a credentials request", async () => {
    const user = userEvent.setup();
    renderPage();

    await openTab(user, "resource:user.tabs.account");
    const isAdmin = await screen.findByLabelText("isAdmin");
    await user.click(isAdmin);
    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => expect(api.updateUser).toHaveBeenCalledTimes(1));
    expect(api.updateUser.mock.calls[0][1]).toMatchObject({ isAdmin: true, revision: 3 });
    expect(api.updateUserProfile).toHaveBeenCalledTimes(1);
    expect(api.updateUserCredentials).not.toHaveBeenCalled();
  });

  it("names the failed stage when a later request in the serial save fails", async () => {
    const user = userEvent.setup();
    const { invalidateSpy } = renderPage();
    api.updateUserProfile.mockRejectedValueOnce(new Error("profile unavailable"));

    await openTab(user, "resource:user.tabs.basic");
    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => expect(api.updateUser).toHaveBeenCalledTimes(1));
    expect(
      await screen.findByText("resource:user.saveUserFailed: resource:user.tabs.profile"),
    ).toBeTruthy();
    expect(api.updateUserCredentials).not.toHaveBeenCalled();
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["user"] });
  });
});

describe("UserDetailPage profile-style saves", () => {
  it("saves the profile fields from the basic section through PATCH then PUT", async () => {
    const user = userEvent.setup();
    const { invalidateSpy } = renderPage();

    await openTab(user, "resource:user.tabs.basic");
    const homepage = await screen.findByLabelText("resource:user.homepage");
    await user.clear(homepage);
    await user.type(homepage, "https://new.example.com");
    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => expect(api.updateUserProfile).toHaveBeenCalledTimes(1));
    expect(api.updateUser).toHaveBeenCalledTimes(1);
    expect(api.updateUserProfile.mock.calls[0][1]).toMatchObject({
      homepage: "https://new.example.com",
      properties: { department: "Engineering", karma: 12, ranking: 5 },
    });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["user"] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["users"] });
  });

  it("saves edited custom properties through PUT /profile", async () => {
    const user = userEvent.setup();
    renderPage();

    await openTab(user, "resource:user.tabs.realname");
    const value = await screen.findByDisplayValue("Engineering");
    await user.clear(value);
    await user.type(value, "Platform");
    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => expect(api.updateUserProfile).toHaveBeenCalledTimes(1));
    expect(api.updateUserProfile.mock.calls[0][1]).toMatchObject({
      properties: { department: "Platform", karma: 12, ranking: 5 },
    });
  });

  it("keeps property and profile edits when the two sections are saved in sequence", async () => {
    const user = userEvent.setup();
    let current = buildUser();
    api.getUserByOrganizationSlugAndUsername.mockImplementation(async () => current);
    api.updateUserProfile.mockImplementation(async (_id, payload) => {
      const next = payload as Record<string, unknown>;
      current = {
        ...current,
        profile: { ...current.profile, ...next, properties: next.properties },
      } as unknown as UserDetail;
    });
    renderPage();

    // 1) Real-name section: department Engineering → Platform
    await openTab(user, "resource:user.tabs.realname");
    const department = await screen.findByDisplayValue("Engineering");
    await user.clear(department);
    await user.type(department, "Platform");
    await user.click(screen.getByRole("button", { name: "save" }));
    await waitFor(() => expect(api.updateUserProfile).toHaveBeenCalledTimes(1));

    // 2) Basic section: homepage change, then save again
    await openTab(user, "resource:user.tabs.basic");
    const homepage = await screen.findByLabelText("resource:user.homepage");
    await user.clear(homepage);
    await user.type(homepage, "https://new.example.com");
    await user.click(screen.getByRole("button", { name: "save" }));
    await waitFor(() => expect(api.updateUserProfile).toHaveBeenCalledTimes(2));

    // 第二次 PUT 是整份 properties 替换，必须仍然带上第一次改过的 department。
    expect(api.updateUserProfile.mock.calls[1][1]).toMatchObject({
      homepage: "https://new.example.com",
      properties: { department: "Platform", karma: 12, ranking: 5 },
    });
    expect(current.profile.properties).toMatchObject({ department: "Platform" });
    expect(current.profile.homepage).toBe("https://new.example.com");
  });

  it("maps the single account status control to consistent derived payload fields", async () => {
    const user = userEvent.setup();
    renderPage();

    await openTab(user, "resource:user.tabs.basic");
    const status = await screen.findByLabelText("resource:user.status");
    await user.click(status);
    await user.click(await screen.findByText("resource:user.statusDisabled"));
    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => expect(api.updateUser).toHaveBeenCalledTimes(1));
    expect(api.updateUser.mock.calls[0][1]).toMatchObject({
      status: "disabled",
      isForbidden: true,
      revision: 3,
    });
    expect(api.updateUser.mock.calls[0][1]).not.toHaveProperty("isDeleted");
  });

  it("edits balance on the account tab and renders it read-only on the finance tab", async () => {
    const user = userEvent.setup();
    let current = buildUser();
    api.getUserByOrganizationSlugAndUsername.mockImplementation(async () => current);
    api.updateUser.mockImplementation(async (_id, payload) => {
      current = { ...current, ...(payload as Record<string, unknown>) } as unknown as UserDetail;
    });
    api.listUserOrders.mockResolvedValue({ items: [], total: 0 });
    api.listUserPayments.mockResolvedValue({ items: [], total: 0 });
    api.listUserTransactions.mockResolvedValue({ items: [], total: 0 });
    renderPage();

    await openTab(user, "resource:user.tabs.account");
    const balance = await screen.findByLabelText("resource:user.balance");
    await user.clear(balance);
    await user.type(balance, "12.34");
    await user.click(screen.getByRole("button", { name: "save" }));
    await waitFor(() => expect(api.updateUser).toHaveBeenCalledTimes(1));
    expect(api.updateUser.mock.calls[0][1]).toMatchObject({ balance: 12.34, revision: 3 });

    await openTab(user, "resource:user.tabs.assets");
    expect(await screen.findByText(/12\.34/)).toBeTruthy();
  });

  it("verifies identification from the real-name tab", async () => {
    const user = userEvent.setup();
    const { invalidateSpy } = renderPage();

    await openTab(user, "resource:user.tabs.realname");
    // 按钮带图标，svg 的 aria-label 会拼进可访问名。
    await user.click(await screen.findByRole("button", { name: /verifyIdentification/ }));

    await waitFor(() => expect(api.verifyIdentification).toHaveBeenCalledTimes(1));
    expect(api.verifyIdentification.mock.calls[0][0]).toBe("u-1");
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["user"] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["users"] });
  });

  it("sends the collected CloudAuth metaInfo and a pure return path", async () => {
    const user = userEvent.setup();
    renderPage();

    await openTab(user, "resource:user.tabs.realname");
    await user.click(await screen.findByRole("button", { name: /verifyIdentification/ }));

    await waitFor(() => expect(api.verifyIdentification).toHaveBeenCalledTimes(1));
    // 后端对 metaInfo / returnUrl 双 fail-closed：两者缺一即 400，returnUrl 还不能带
    // query/hash，所以断言的是「详情页纯路径」而不是当前地址。
    expect(api.verifyIdentification.mock.calls[0][2]).toEqual({
      metaInfo: JSON.stringify({ deviceType: "web" }),
      returnUrl: "https://console.test/users/built-in/alice",
    });
  });

  it("opens the scan dialog instead of navigating the operator away", async () => {
    const user = userEvent.setup();
    // jsdom 不实现 Clipboard API：不桩化的话组件按 fail-open 规则隐藏复制入口。
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText: vi.fn<() => Promise<void>>().mockResolvedValue(undefined) },
    });
    api.verifyIdentification.mockResolvedValue({
      status: "pending",
      data: { verificationId: "v-9", redirectUrl: "https://verify.test/certify/v-9" },
    });
    renderPage();

    await openTab(user, "resource:user.tabs.realname");
    await user.click(await screen.findByRole("button", { name: /verifyIdentification/ }));

    // 刷新页面后仍能接上轮询：待收尾标记必须落盘。
    await waitFor(() => expect(localStorage.getItem("idv:pending:u-1")).toBe("v-9"));
    // jsdom 不跑 rc-motion 的 transitionend，弹窗节点已挂载但仍在入场态：
    // 页面层只断言「弹窗出现且内容是扫码引导」，可访问名由组件测试覆盖。
    const dialog = await screen.findByText("verifyIdQrTitle");
    expect(dialog.textContent).toBe("verifyIdQrTitle");
    expect(document.body.textContent).toContain("verifyIdQrHint");
    expect(
      within(dialog.closest("[role=dialog]") as HTMLElement).getByRole("button", {
        name: "verifyIdCopyLink",
        hidden: true,
      }),
    ).toBeTruthy();
  });

  it("does not call the backend when CloudAuth metaInfo cannot be collected", async () => {
    const user = userEvent.setup();
    sdk.collectMetaInfo.mockRejectedValue(new sdk.CloudAuthMetaError("timeout"));
    renderPage();

    await openTab(user, "resource:user.tabs.realname");
    await user.click(await screen.findByRole("button", { name: /verifyIdentification/ }));

    expect(await screen.findByText("verifyIdMetaFailed")).toBeTruthy();
    expect(api.verifyIdentification).not.toHaveBeenCalled();
  });
});

describe("UserDetailPage cross-tab saves", () => {
  it("submits changes made in another tab when saving from the profile tab", async () => {
    const user = userEvent.setup();
    renderPage();

    const externalId = await screen.findByLabelText("resource:user.externalId");
    await user.clear(externalId);
    await user.type(externalId, "ext-2");

    await openTab(user, "resource:user.tabs.basic");
    const homepage = await screen.findByLabelText("resource:user.homepage");
    await user.clear(homepage);
    await user.type(homepage, "https://new.example.com");
    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => expect(api.updateUser).toHaveBeenCalledTimes(1));
    expect(api.updateUser.mock.calls[0][1]).toMatchObject({ externalId: "ext-2", revision: 3 });
    expect(api.updateUserProfile.mock.calls[0][1]).toMatchObject({
      homepage: "https://new.example.com",
    });
  });

  it("keeps untouched fields from other tabs out of the payload", async () => {
    const user = userEvent.setup();
    renderPage();

    await openTab(user, "resource:user.tabs.basic");
    const homepage = await screen.findByLabelText("resource:user.homepage");
    await user.clear(homepage);
    await user.type(homepage, "https://new.example.com");
    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => expect(api.updateUserProfile).toHaveBeenCalledTimes(1));
    const payload = api.updateUserProfile.mock.calls[0][1] as Record<string, unknown>;
    expect(payload).not.toHaveProperty("tag");
    expect(payload).not.toHaveProperty("status");
    expect(payload).not.toHaveProperty("isAdmin");
    expect(payload).not.toHaveProperty("externalId");
  });

  it("sends an explicit empty string for an optional field cleared in another tab", async () => {
    const user = userEvent.setup();
    renderPage();

    const externalId = await screen.findByLabelText("resource:user.externalId");
    await user.clear(externalId);

    await openTab(user, "resource:user.tabs.basic");
    const homepage = await screen.findByLabelText("resource:user.homepage");
    await user.clear(homepage);
    await user.type(homepage, "https://new.example.com");
    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => expect(api.updateUser).toHaveBeenCalledTimes(1));
    expect(api.updateUser.mock.calls[0][1]).toMatchObject({ externalId: "", revision: 3 });
    expect(api.updateUserProfile.mock.calls[0][1]).toMatchObject({
      homepage: "https://new.example.com",
    });
  });

  it("still persists another tab's change when saving from the credentials tab", async () => {
    const user = userEvent.setup();
    renderPage();

    const externalId = await screen.findByLabelText("resource:user.externalId");
    await user.clear(externalId);
    await user.type(externalId, "ext-2");

    await openTab(user, "resource:user.tabs.credentials");
    await user.click(
      await screen.findByRole("button", { name: "resource:user.rotateAccessSecret" }),
    );
    await user.click(
      await screen.findByRole("button", { name: "resource:user.rotateAccessSecretConfirmOk" }),
    );
    await waitFor(() => expect(api.updateUserCredentials).toHaveBeenCalled());

    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => expect(api.updateUser).toHaveBeenCalledTimes(1));
    expect(api.updateUser.mock.calls[0][1]).toMatchObject({ externalId: "ext-2", revision: 3 });
    expect(api.updateUserCredentials.mock.calls[0][1]).toMatchObject({ rotateAccessSecret: true });
  });
});

describe("UserDetailPage credentials save", () => {
  it("rotates the access secret through PUT /credentials immediately", async () => {
    const user = userEvent.setup();
    const { invalidateSpy } = renderPage();

    await openTab(user, "resource:user.tabs.credentials");
    await user.click(
      await screen.findByRole("button", { name: "resource:user.rotateAccessSecret" }),
    );
    await user.click(
      await screen.findByRole("button", { name: "resource:user.rotateAccessSecretConfirmOk" }),
    );

    await waitFor(() => expect(api.updateUserCredentials).toHaveBeenCalledTimes(1));
    // 轮换是即时动作而非草稿：确认后立即 PUT，且第三个参数是可信租户。
    expect(api.updateUserCredentials.mock.calls[0]).toEqual([
      "u-1",
      { rotateAccessSecret: true },
      "built-in",
    ]);
    expect(api.updateUser).not.toHaveBeenCalled();
    expect(api.updateUserProfile).not.toHaveBeenCalled();
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["user"] });
  });

  it("renders the masked access key from the user detail response", async () => {
    api.getUserByOrganizationSlugAndUsername.mockResolvedValue({
      ...buildUser(),
      credentialSummary: {
        hasPassword: true,
        hasAccessKey: true,
        accessKeySet: true,
        accessKeyMasked: "******",
      },
    });

    const user = userEvent.setup();
    renderPage();
    await openTab(user, "resource:user.tabs.credentials");

    expect(await screen.findByText("******")).toBeTruthy();
    expect(screen.queryByText("user-access-key")).toBeNull();
  });
});

describe("UserDetailPage managed accounts", () => {
  it("displays only the masked password returned by the API", async () => {
    api.getUserByOrganizationSlugAndUsername.mockResolvedValue({
      ...buildUser(),
      managedAccounts: [
        {
          application: "Example",
          username: "alice",
          passwordSet: true,
          passwordMasked: "******",
          signinUrl: "https://example.test",
        },
      ],
    });

    const user = userEvent.setup();
    renderPage();
    await openTab(user, "resource:user.tabs.assets");

    expect(await screen.findByText("******")).toBeTruthy();
    expect(screen.queryByText("managed-password")).toBeNull();
  });
});

describe("UserDetailPage identity and role/group side effects", () => {
  it("replaces identities when removing an existing identity", async () => {
    const user = userEvent.setup();
    const { invalidateSpy } = renderPage();

    await openTab(user, "resource:user.tabs.identity");
    const row = (await screen.findByText("github")).closest("tr");
    expect(row).not.toBeNull();
    await user.click(within(row as HTMLElement).getByRole("button"));
    await confirmPopover("OK");

    await waitFor(() => expect(api.replaceUserIdentities).toHaveBeenCalledTimes(1));
    expect(api.replaceUserIdentities.mock.calls[0]).toEqual(["u-1", []]);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["user"] });
  });

  it("replaces role bindings when removing a role", async () => {
    const user = userEvent.setup();
    renderPage();

    await openTab(user, "resource:user.tabs.account");
    await user.click(await screen.findByText("Administrator"));
    await confirmPopover("common:delete");

    await waitFor(() => expect(api.replaceUserRoleBindings).toHaveBeenCalledTimes(1));
    expect(api.replaceUserRoleBindings.mock.calls[0]).toEqual(["u-1", [], "built-in"]);
  });

  it("replaces group memberships when removing a group", async () => {
    const user = userEvent.setup();
    renderPage();

    await openTab(user, "resource:user.tabs.account");
    await user.click(await screen.findByText("Operators"));
    await confirmPopover("common:delete");

    await waitFor(() => expect(api.replaceUserGroupMemberships).toHaveBeenCalledTimes(1));
    expect(api.replaceUserGroupMemberships.mock.calls[0]).toEqual(["u-1", [], "built-in"]);
  });
});

describe("UserDetailPage MFA side effects", () => {
  it("persists preferred MFA and method toggles", async () => {
    const user = userEvent.setup();
    const { invalidateSpy } = renderPage();

    await openTab(user, "resource:user.tabs.mfa");
    await user.click(await screen.findByRole("button", { name: "resource:user.setPreferred" }));

    await waitFor(() => expect(api.updateUserPreferredMfa).toHaveBeenCalledTimes(1));
    expect(api.updateUserPreferredMfa.mock.calls[0]).toEqual([
      "u-1",
      { mfaType: "totp" },
      "built-in",
    ]);

    const row = screen.getByText("totp").closest("tr");
    expect(row).not.toBeNull();
    await user.click(within(row as HTMLElement).getByRole("switch"));

    await waitFor(() => expect(api.replaceUserMfaMethods).toHaveBeenCalledTimes(1));
    expect(api.replaceUserMfaMethods.mock.calls[0]).toEqual([
      "u-1",
      3,
      [{ methodType: "totp", enabled: false, displayName: "TOTP" }],
      "built-in",
    ]);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["user"] });
  });
});

describe("UserDetailPage section navigation", () => {
  it("renders each business section as one anchor with a matching nav entry", async () => {
    expect(USER_DETAIL_TABS).toHaveLength(8);
    renderPage();

    for (const tab of USER_DETAIL_TABS) {
      expect(await screen.findByRole("link", { name: tab.labelKey })).toBeTruthy();
      expect(document.getElementById(`user-section-${tab.key}`)).not.toBeNull();
    }
  });

  it("highlights the deep-linked section on first paint", async () => {
    renderPage("/users/built-in/alice?tab=managed-accounts");

    const link = await screen.findByRole("link", { name: "resource:user.tabs.assets" });
    expect(link.getAttribute("aria-current")).toBe("location");
  });

  it("maps legacy and group aliases to a concrete section", async () => {
    renderPage("/users/built-in/alice?tab=real-name");

    const link = await screen.findByRole("link", { name: "resource:user.tabs.realname" });
    expect(link.getAttribute("aria-current")).toBe("location");
  });
});
