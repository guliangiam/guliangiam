import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { App } from "antd";
import { QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createTestQueryClient } from "@/test-utils/render";
import GroupDetailPage from "./GroupDetailPage";

const api = vi.hoisted(() => ({
  getGroupsPaginated: vi.fn<
    (params?: {
      page?: number;
      pageSize?: number;
      search?: string;
      organizationId?: string;
    }) => Promise<{
      items: unknown[];
      page: number;
      pageSize: number;
      total: number;
    }>
  >(),
  getGroup: vi.fn<(id: string) => Promise<unknown>>(),
  updateGroup: vi.fn<(id: string, payload: unknown) => Promise<unknown>>(),
  deleteGroup: vi.fn<(id: string) => Promise<unknown>>(),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock("@/shared/api/iam/group", () => api);
// 详情页先解析路由里的组织 slug → id；测试里直接给固定映射。
vi.mock("@/shared/api/iam/organization", () => ({
  getOrganizationIdentityBySlug: vi
    .fn<(identifier: string) => Promise<{ id: string; slug: string } | null>>()
    .mockImplementation(async (identifier: string) =>
      identifier ? { id: identifier, slug: identifier } : null,
    ),
  getOrganizationOptions: vi
    .fn<() => Promise<Array<{ id: string; displayName: string; slug: string }>>>()
    .mockResolvedValue([
      { id: "built-in", displayName: "Built-in Organization", slug: "built-in" },
    ]),
}));
vi.mock("@/shared/ui/PermissionGuard", () => ({
  PermissionGuard: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

// 叶子选择器不做异步加载，避免测试依赖下拉的远程候选。
vi.mock("@/shared/ui/GroupTreeSelect", () => ({
  GroupTreeSelect: ({
    value,
    onChange,
  }: {
    value?: string;
    onChange?: (value: string) => void;
  }) => (
    <select
      aria-label="parent-group"
      value={value ?? ""}
      onChange={(event) => onChange?.(event.target.value)}
    >
      <option value="">none</option>
    </select>
  ),
}));

vi.mock("@/shared/ui/UserPaginateSelect", () => ({
  UserPaginateSelect: ({
    value,
    onChange,
  }: {
    value?: string[];
    onChange?: (value: string[]) => void;
  }) => (
    <span>
      {`users:${(value ?? []).join(",")}`}
      <button type="button" onClick={() => onChange?.(["carol"])}>
        select-carol
      </button>
    </span>
  ),
}));

// G3：组↔角色关联为只读展示，数据源是角色列表的 groupIds。
vi.mock("@/shared/api/iam/role", () => ({
  getRoles: vi.fn<() => Promise<unknown[]>>().mockResolvedValue([
    {
      id: "role-1",
      organizationId: "built-in",
      name: "eng-admin",
      displayName: "Eng Admin",
      description: "",
      groupIds: ["group-1"],
      isEnabled: true,
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-01T00:00:00Z",
    },
    {
      id: "role-2",
      organizationId: "built-in",
      name: "dev",
      displayName: "Dev",
      description: "",
      groupIds: [],
      isEnabled: true,
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-01T00:00:00Z",
    },
  ]),
}));

function renderPage() {
  const queryClient = createTestQueryClient();
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
  const router = createMemoryRouter(
    [
      { path: "/groups/:organizationName/:groupName", element: <GroupDetailPage /> },
      { path: "/groups", element: <div data-testid="groups-list" /> },
    ],
    { initialEntries: ["/groups/built-in/engineering"] },
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
  const groupItems = [
    {
      id: "group-1",
      owner: "built-in",
      name: "engineering",
      displayName: "Engineering",
      type: "virtual",
      parentId: "",
      isTopGroup: false,
      isEnabled: true,
    },
    {
      id: "group-2",
      owner: "built-in",
      name: "platform",
      displayName: "Platform",
      type: "virtual",
      parentId: "",
      isTopGroup: true,
      isEnabled: true,
    },
  ];
  api.getGroupsPaginated.mockResolvedValue({
    items: groupItems,
    page: 1,
    pageSize: 50,
    total: groupItems.length,
  });
  api.getGroup.mockResolvedValue({
    id: "group-1",
    organizationId: "built-in",
    name: "engineering",
    displayName: "Engineering",
    type: "virtual",
    parentId: "",
    isTopGroup: false,
    isEnabled: true,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-02T00:00:00Z",
    manager: "alice",
    contactEmail: "old@example.com",
    homepage: "https://old.example.com",
    users: ["alice", "bob"],
    revision: 3,
  });
  api.updateGroup.mockResolvedValue(undefined);
  api.deleteGroup.mockResolvedValue(undefined);
});

describe("GroupDetailPage save closure", () => {
  it("echoes the detail and submits edited fields with the loaded revision", async () => {
    const user = userEvent.setup();
    const { invalidateSpy } = renderPage();

    await screen.findByDisplayValue("Engineering");
    expect(screen.getByLabelText("group.homepage")).toHaveValue("https://old.example.com");
    expect(screen.getByLabelText("group.contactEmail")).toHaveValue("old@example.com");
    expect(screen.getByText("users:alice,bob")).toBeInTheDocument();

    const displayName = screen.getByLabelText("displayName");
    await user.clear(displayName);
    await user.type(displayName, "Engineering 2");

    const homepage = screen.getByLabelText("group.homepage");
    await user.clear(homepage);
    await user.type(homepage, "https://new.example.com");

    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => expect(api.updateGroup).toHaveBeenCalledTimes(1));
    expect(api.updateGroup.mock.calls[0][0]).toBe("group-1");
    expect(api.updateGroup.mock.calls[0][1]).toMatchObject({
      displayName: "Engineering 2",
      homepage: "https://new.example.com",
      type: "virtual",
      parentId: "",
      manager: "alice",
      contactEmail: "old@example.com",
      users: ["alice", "bob"],
      isTopGroup: false,
      isEnabled: true,
      revision: 3,
    });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["groups"] });
  });

  it("keeps the isEnabled switch in the basic info section and submits its toggled value", async () => {
    const user = userEvent.setup();
    renderPage();

    await screen.findByDisplayValue("Engineering");
    const basicSection = screen
      .getByRole("heading", { name: "group.basicInfo", level: 3 })
      .closest(".gl-form-section");
    expect(basicSection).not.toBeNull();
    const toggle = within(basicSection as HTMLElement).getByLabelText("enabled");
    expect(toggle).toBeChecked();

    await user.click(toggle);
    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => expect(api.updateGroup).toHaveBeenCalledTimes(1));
    expect(api.updateGroup.mock.calls[0][1]).toMatchObject({ isEnabled: false });
  });

  it("deletes the group after confirmation and returns to the list", async () => {
    const user = userEvent.setup();
    const { invalidateSpy } = renderPage();

    await screen.findByDisplayValue("Engineering");
    await user.click(screen.getByRole("button", { name: "delete" }));
    await confirmPopover("delete");

    await waitFor(() => expect(api.deleteGroup).toHaveBeenCalledTimes(1));
    expect(api.deleteGroup.mock.calls[0][0]).toBe("group-1");
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["groups"] });
    expect(await screen.findByTestId("groups-list")).toBeInTheDocument();
  });

  it("shows related roles read-only from role groupIds (G3)", async () => {
    renderPage();

    await screen.findByDisplayValue("Engineering");
    await screen.findByText("Eng Admin");
    expect(screen.queryByText("Dev")).not.toBeInTheDocument();
  });
});

describe("GroupMembersDrawer round-trip", () => {
  it("adds and removes members then saves users + revision via updateGroup", async () => {
    const user = userEvent.setup();
    const { invalidateSpy } = renderPage();

    await screen.findByDisplayValue("Engineering");
    await user.click(screen.getByRole("button", { name: "group.manageMembers" }));

    const drawer = await waitFor(() => {
      const element = document.querySelector(".ant-drawer-section");
      if (!element) throw new Error("drawer not open");
      return element as HTMLElement;
    });

    // 当前成员 alice/bob 渲染在表格中。
    expect(within(drawer).getByText("alice")).toBeInTheDocument();
    expect(within(drawer).getByText("bob")).toBeInTheDocument();

    // 添加 carol。
    await user.click(within(drawer).getByRole("button", { name: "select-carol" }));
    await user.click(within(drawer).getByRole("button", { name: "group.addMembers" }));
    expect(await within(drawer).findByText("carol")).toBeInTheDocument();

    // 勾选 bob 并批量移除（Popconfirm 默认 OK 按钮确认）。
    const bobRow = within(drawer).getByText("bob").closest("tr");
    expect(bobRow).not.toBeNull();
    await user.click(within(bobRow as HTMLTableRowElement).getByRole("checkbox"));
    await user.click(within(drawer).getByRole("button", { name: "group.removeSelected" }));
    const okButton = await waitFor(() => {
      const candidate = Array.from(document.querySelectorAll(".ant-popover button")).find(
        (element) => element.textContent?.includes("ok"),
      );
      if (!candidate) throw new Error("popconfirm ok not found");
      return candidate as HTMLElement;
    });
    await user.click(okButton);
    await waitFor(() => expect(within(drawer).queryByText("bob")).not.toBeInTheDocument());

    // 保存：round-trip 断言 users + revision。
    await user.click(within(drawer).getByRole("button", { name: "save" }));

    await waitFor(() => expect(api.updateGroup).toHaveBeenCalledTimes(1));
    expect(api.updateGroup.mock.calls[0][0]).toBe("group-1");
    expect(api.updateGroup.mock.calls[0][1]).toEqual({
      users: ["alice", "carol"],
      revision: 3,
    });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["groups"] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["group"] });
  });
});
