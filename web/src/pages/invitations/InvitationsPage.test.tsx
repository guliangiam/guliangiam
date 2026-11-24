import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { createTestWrapper, createTestQueryClient } from "@/test-utils/render";
import InvitationsPage from "./InvitationsPage";

type ListParams = {
  page?: number;
  pageSize?: number;
  search?: string;
  status?: string;
  organizationId?: string;
};

const mockGetInvitationsPaginated = vi.fn<(params?: ListParams) => unknown>();
const mockRevokeInvitation = vi.fn<(...args: unknown[]) => unknown>();
const mockResendInvitation = vi.fn<(...args: unknown[]) => unknown>();
const mockSendInvitation = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("@/shared/api/iam/invitation", () => ({
  getInvitationsPaginated: (params?: ListParams) => mockGetInvitationsPaginated(params),
  revokeInvitation: (id: string) => mockRevokeInvitation(id),
  resendInvitation: (id: string) => mockResendInvitation(id),
  sendInvitation: (id: string) => mockSendInvitation(id),
}));

vi.mock("@/shared/api/iam/role", () => ({
  getRoleOptions: vi.fn<() => Promise<unknown[]>>().mockResolvedValue([]),
}));

vi.mock("@/shared/api/iam/group", () => ({
  getGroupOptions: vi.fn<() => Promise<unknown[]>>().mockResolvedValue([]),
}));

vi.mock("@/shared/api/iam/organization", () => ({
  getOrganizations: vi.fn<() => Promise<unknown[]>>().mockResolvedValue([]),
}));

vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    isLoading: false,
    hasAction: () => true,
    hasResource: () => true,
  }),
}));

vi.mock("@/shared/hooks/useResourceNames", () => ({
  useUserNames: () => ({ nameMap: new Map() }),
  useOrganizationNames: () => ({ nameMap: new Map() }),
}));

vi.mock("@/features/session/session-store", () => ({
  useSessionStore: (selector: (state: unknown) => unknown) =>
    selector({ account: { username: "admin" } }),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
}));

function invitation(id: string, email: string, status: string) {
  return {
    id,
    organizationId: "org-1",
    email,
    code: `code-${id}`,
    roleIds: [],
    groupIds: [],
    status,
    expiresAt: "2026-12-01T00:00:00Z",
    invitedBy: "admin",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    revision: 1,
  };
}

const ROWS = [
  invitation("inv-1", "alice@example.com", "pending"),
  invitation("inv-2", "bob@example.com", "pending"),
  invitation("inv-3", "carol@example.com", "accepted"),
];

function renderPage(initialEntry = "/invitations") {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <InvitationsPage />
    </MemoryRouter>,
    { wrapper: createTestWrapper() },
  );
}

// 模拟服务端分页/筛选：keyword 按 email 包含匹配，status 精确匹配，LIMIT/OFFSET 切片。
function serveRows(rows: ReturnType<typeof invitation>[]) {
  mockGetInvitationsPaginated.mockImplementation((params?: ListParams) => {
    const page = params?.page ?? 1;
    const pageSize = params?.pageSize ?? 50;
    let matched = rows;
    if (params?.search) matched = matched.filter((r) => r.email.includes(params.search!));
    if (params?.status) matched = matched.filter((r) => r.status === params.status);
    return Promise.resolve({
      items: matched.slice((page - 1) * pageSize, page * pageSize),
      page,
      pageSize,
      total: matched.length,
    });
  });
}

function lastListParams(): ListParams | undefined {
  const calls = mockGetInvitationsPaginated.mock.calls;
  return calls[calls.length - 1]?.[0];
}

async function selectPendingRows(user: ReturnType<typeof userEvent.setup>) {
  await screen.findByText("alice@example.com");
  const checkboxes = screen.getAllByRole("checkbox");
  // [全选, inv-1, inv-2, inv-3]
  await user.click(checkboxes[1]);
  await user.click(checkboxes[2]);
}

describe("InvitationsPage bulk actions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    serveRows(ROWS);
    mockRevokeInvitation.mockResolvedValue(undefined);
    mockResendInvitation.mockResolvedValue(undefined);
    mockSendInvitation.mockResolvedValue(undefined);
  });

  it("only pending rows are selectable", async () => {
    renderPage();
    await screen.findByText("alice@example.com");
    const checkboxes = screen.getAllByRole("checkbox");
    expect(checkboxes[1]).not.toBeDisabled();
    expect(checkboxes[2]).not.toBeDisabled();
    expect(checkboxes[3]).toBeDisabled();
  });

  it("bulk revoke calls revoke per selected id and clears selection on success", async () => {
    const user = userEvent.setup();
    renderPage();
    await selectPendingRows(user);

    await user.click(screen.getByRole("button", { name: "invitation.bulkRevoke" }));
    await user.click(await screen.findByRole("button", { name: "ok" }));

    await waitFor(() => expect(mockRevokeInvitation).toHaveBeenCalledTimes(2));
    expect(mockRevokeInvitation).toHaveBeenCalledWith("inv-1");
    expect(mockRevokeInvitation).toHaveBeenCalledWith("inv-2");
    expect(await screen.findByText("invitation.bulkRevokeSuccess")).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "invitation.bulkRevoke" })).toBeNull(),
    );
  });

  it("keeps failed rows selected and reports partial failure", async () => {
    mockRevokeInvitation.mockImplementation((id: string) =>
      id === "inv-2" ? Promise.reject(new Error("boom")) : Promise.resolve(undefined),
    );
    const user = userEvent.setup();
    renderPage();
    await selectPendingRows(user);

    await user.click(screen.getByRole("button", { name: "invitation.bulkRevoke" }));
    await user.click(await screen.findByRole("button", { name: "ok" }));

    expect(await screen.findByText("invitation.batchPartial")).toBeInTheDocument();
    // 失败行保留选中：批量栏仍在，inv-2 复选框仍勾选
    expect(screen.getByRole("button", { name: "invitation.bulkRevoke" })).toBeInTheDocument();
    const checkboxes = screen.getAllByRole("checkbox");
    expect(checkboxes[2]).toBeChecked();
    expect(checkboxes[1]).not.toBeChecked();
  });

  it("bulk send dispatches send for each selected pending invitation", async () => {
    const user = userEvent.setup();
    renderPage();
    await selectPendingRows(user);

    await user.click(screen.getByRole("button", { name: "invitation.bulkSend" }));
    await user.click(await screen.findByRole("button", { name: "ok" }));

    await waitFor(() => expect(mockSendInvitation).toHaveBeenCalledTimes(2));
    expect(mockSendInvitation).toHaveBeenCalledWith("inv-1");
    expect(mockSendInvitation).toHaveBeenCalledWith("inv-2");
    expect(await screen.findByText("invitation.bulkSendSuccess")).toBeInTheDocument();
  });

  // INC-FE-01 回归：搜索/翻页必须清空选中，批量动作不得作用不可见行。
  it("clears selection when server search hides the selected rows", async () => {
    const user = userEvent.setup();
    renderPage();
    await selectPendingRows(user);
    expect(screen.getByRole("button", { name: "invitation.bulkRevoke" })).toBeInTheDocument();

    // 服务端模式 Input.Search 仅在回车/点击搜索时提交（清空例外）。
    const input = screen.getByPlaceholderText("invitation.searchPlaceholder");
    await user.type(input, "carol{Enter}");

    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "invitation.bulkRevoke" })).toBeNull(),
    );
    await waitFor(() => expect(screen.queryByText("alice@example.com")).toBeNull());
    expect(lastListParams()?.search).toBe("carol");
  });

  it("clears selection when server pagination changes the visible page", async () => {
    const rows = Array.from({ length: 60 }, (_, i) =>
      invitation(`inv-${i}`, `user${i}@example.com`, "pending"),
    );
    serveRows(rows);
    const user = userEvent.setup();
    const { container } = renderPage();
    await screen.findByText("user0@example.com");
    const checkboxes = screen.getAllByRole("checkbox");
    await user.click(checkboxes[1]);
    expect(screen.getByRole("button", { name: "invitation.bulkRevoke" })).toBeInTheDocument();

    const page2 = container.querySelector(".ant-pagination-item-2 a");
    expect(page2).not.toBeNull();
    await user.click(page2!);

    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "invitation.bulkRevoke" })).toBeNull(),
    );
    expect(lastListParams()?.page).toBe(2);
  });

  // INC-FE-03 回归：列表动作必须失效详情页订阅的两个查询键（复数全量 + 单数详情前缀）。
  it("invalidates detail-page query keys after bulk revoke", async () => {
    const queryClient = createTestQueryClient();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <InvitationsPage />
      </MemoryRouter>,
      { wrapper: createTestWrapper({ queryClient }) },
    );
    await selectPendingRows(user);

    await user.click(screen.getByRole("button", { name: "invitation.bulkRevoke" }));
    await user.click(await screen.findByRole("button", { name: "ok" }));

    await waitFor(() => expect(mockRevokeInvitation).toHaveBeenCalledTimes(2));
    expect(spy).toHaveBeenCalledWith({ queryKey: ["invitations", "auth"] });
    expect(spy).toHaveBeenCalledWith({ queryKey: ["invitation"] });
  });

  // INC-FE-02 回归：批量并发不得超过批次上限 10（与壳层 bulkDelete 一致）。
  it("caps bulk concurrency at the batch size", async () => {
    const rows = Array.from({ length: 12 }, (_, i) =>
      invitation(`inv-${i}`, `user${i}@example.com`, "pending"),
    );
    serveRows(rows);
    let active = 0;
    let maxActive = 0;
    mockRevokeInvitation.mockImplementation(async () => {
      active += 1;
      maxActive = Math.max(maxActive, active);
      await new Promise((resolve) => setTimeout(resolve, 5));
      active -= 1;
    });
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("user0@example.com");
    // 全选当前页（12 行全部 pending）
    await user.click(screen.getAllByRole("checkbox")[0]);

    await user.click(screen.getByRole("button", { name: "invitation.bulkRevoke" }));
    await user.click(await screen.findByRole("button", { name: "ok" }));

    await waitFor(() => expect(mockRevokeInvitation).toHaveBeenCalledTimes(12));
    expect(maxActive).toBeLessThanOrEqual(10);
    expect(maxActive).toBeGreaterThan(1);
  });

  it("passes the URL organization context to the server list request", async () => {
    renderPage("/invitations?organizationId=org-9");
    await screen.findByText("alice@example.com");
    expect(lastListParams()?.organizationId).toBe("org-9");
  });

  it("sends the status filter as a server-side status param", async () => {
    renderPage();
    await waitFor(() => expect(mockGetInvitationsPaginated).toHaveBeenCalled());
    const combobox = screen.getByRole("combobox", { name: "statusLabel" });
    fireEvent.mouseDown(combobox);
    // rc-select 依据 keyCode 处理键盘导航：下移到第四项 revoked 后回车选中。
    // 虚拟列表在 jsdom 只渲染前两项，不能按 option 名等待，直接断言请求参数。
    for (let i = 0; i < 3; i += 1) {
      fireEvent.keyDown(combobox, { key: "ArrowDown", keyCode: 40 });
    }
    fireEvent.keyDown(combobox, { key: "Enter", keyCode: 13 });
    await waitFor(() => expect(lastListParams()?.status).toBe("revoked"));
  });
});
