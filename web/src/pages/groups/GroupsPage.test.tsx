import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { App } from "antd";
import { QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useSessionStore } from "@/features/session/session-store";
import {
  deleteGroup,
  getGroupsPaginated,
  importGroups,
  importGroupsFromExcel,
} from "@/shared/api/iam/group";
import { createTestQueryClient } from "@/test-utils/render";
import GroupsPage from "./GroupsPage";

vi.mock("@/shared/ui/OrganizationSelect", () => ({
  OrganizationSelect: ({
    value,
    onChange,
  }: {
    value: string;
    onChange: (value: string) => void;
  }) => (
    <button type="button" data-testid="org-select" onClick={() => onChange("")}>
      {`org:${value}`}
    </button>
  ),
}));

vi.mock("@/shared/ui/PermissionGuard", () => ({
  PermissionGuard: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock("@/shared/api/iam/group", () => ({
  getGroupsPaginated: vi.fn<() => Promise<unknown>>(),
  deleteGroup: vi.fn<() => Promise<void>>(),
  importGroups: vi.fn<() => Promise<unknown>>(),
  importGroupsFromExcel: vi.fn<() => Promise<unknown>>(),
}));

vi.mock("@/shared/hooks/useResourceNames", () => ({
  useOrganizationNames: () => ({ nameMap: new Map([["org-2", "Org Two"]]) }),
}));

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location-search">{location.search}</div>;
}

function BackButton() {
  const navigate = useNavigate();
  return (
    <button type="button" data-testid="back" onClick={() => navigate(-1)}>
      back
    </button>
  );
}

function renderPage(entry: string | string[], initialIndex = 0) {
  const queryClient = createTestQueryClient();
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
  const entries = Array.isArray(entry) ? entry : [entry];
  const view = render(
    <QueryClientProvider client={queryClient}>
      <App component={false}>
        <MemoryRouter initialEntries={entries} initialIndex={initialIndex}>
          <Routes>
            <Route path="/groups" element={<GroupsPage />} />
          </Routes>
          <LocationProbe />
          <BackButton />
        </MemoryRouter>
      </App>
    </QueryClientProvider>,
  );
  return { ...view, queryClient, invalidateSpy };
}

function lastListParams() {
  const calls = vi.mocked(getGroupsPaginated).mock.calls;
  return calls.at(-1)?.[0] as
    | { organizationId?: string; status?: string; sortBy?: string; sortOrder?: string }
    | undefined;
}

function sessionAsAdmin() {
  useSessionStore.setState({
    account: { owner: "built-in", name: "admin", isAdmin: true },
    organization: {
      id: "org-1",
      name: "built-in",
      displayName: "Built-in",
      navItems: [],
      userNavItems: [],
    },
    impersonatedUser: null,
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  sessionAsAdmin();
  vi.mocked(getGroupsPaginated).mockResolvedValue({
    items: [],
    page: 1,
    pageSize: 50,
    total: 0,
  });
});

describe("GroupsPage organization context", () => {
  it("consumes ?organizationId into the list params and clears it from the URL", async () => {
    const user = userEvent.setup();
    renderPage("/groups?organizationId=org-2");

    await waitFor(() => expect(lastListParams()?.organizationId).toBe("org-2"));
    expect(screen.getByTestId("org-select")).toHaveTextContent("org:org-2");
    expect(screen.getByTestId("location-search")).toHaveTextContent("organizationId=org-2");

    await user.click(screen.getByTestId("org-select"));

    await waitFor(() => expect(screen.getByTestId("location-search").textContent).toBe(""));
    await waitFor(() => expect(lastListParams()?.organizationId).toBeUndefined());
  });

  it("follows the URL when browser history changes the organizationId", async () => {
    const user = userEvent.setup();
    renderPage(["/groups?organizationId=org-2", "/groups?organizationId=org-3"], 1);

    await waitFor(() => expect(lastListParams()?.organizationId).toBe("org-3"));

    // 显式清除会留下选择状态；后退必须让 URL 重新成为唯一来源。
    await user.click(screen.getByTestId("org-select"));
    await waitFor(() => expect(lastListParams()?.organizationId).toBeUndefined());

    await user.click(screen.getByTestId("back"));
    await waitFor(() => expect(lastListParams()?.organizationId).toBe("org-2"));
  });

  it("shows the URL organization context to non-admin users and lets them clear it", async () => {
    useSessionStore.setState({
      account: { owner: "org-1", name: "operator", isAdmin: false },
    });
    const { container } = renderPage("/groups?organizationId=org-2");

    expect(screen.getByText(/Org Two/)).toBeInTheDocument();
    await waitFor(() => expect(lastListParams()?.organizationId).toBe("org-2"));

    const closeIcon = container.querySelector(".ant-tag-close-icon");
    expect(closeIcon).not.toBeNull();
    fireEvent.click(closeIcon!);

    await waitFor(() => expect(screen.getByTestId("location-search").textContent).toBe(""));
    await waitFor(() => expect(lastListParams()?.organizationId).toBeUndefined());
  });
});

describe("GroupsPage server-side list contract", () => {
  it("maps the enabled/disabled filter onto the backend active/inactive status", async () => {
    renderPage("/groups");
    await waitFor(() => expect(getGroupsPaginated).toHaveBeenCalled());

    const combobox = screen.getByRole("combobox", { name: "statusLabel" });
    fireEvent.mouseDown(combobox);
    await screen.findByRole("option", { name: "disabled" });
    // rc-select 依据 keyCode 处理键盘导航：下移到第二项 disabled 后回车选中。
    fireEvent.keyDown(combobox, { key: "ArrowDown", keyCode: 40 });
    fireEvent.keyDown(combobox, { key: "Enter", keyCode: 13 });

    await waitFor(() => expect(lastListParams()?.status).toBe("inactive"));
  });

  it("requests server sorting when a sortable column header is clicked", async () => {
    const user = userEvent.setup();
    // 空列表渲染 Empty 占位而非表格；表头排序断言需要至少一行数据。
    vi.mocked(getGroupsPaginated).mockResolvedValue({
      items: [{ id: "g1", owner: "org-1", name: "eng", displayName: "Engineering" }],
      page: 1,
      pageSize: 50,
      total: 1,
    });
    renderPage("/groups");
    await screen.findByText("eng");

    // 默认排序 name ascend；点击表头切换到 descend 必须走服务端排序参数。
    const header = await screen.findByRole("columnheader", { name: /^name/ });
    await user.click(header);

    await waitFor(() => expect(lastListParams()?.sortBy).toBe("name"));
    expect(lastListParams()?.sortOrder).toBe("desc");
  });
});

describe("GroupsPage single import entry", () => {
  async function selectImportFile(
    user: ReturnType<typeof userEvent.setup>,
    container: HTMLElement,
    file: File,
  ) {
    const input = container.querySelector<HTMLInputElement>('input[type="file"]');
    if (!input) throw new Error("import file input not found");
    await user.upload(input, file);
  }

  async function uploadExcel(user: ReturnType<typeof userEvent.setup>, container: HTMLElement) {
    await selectImportFile(user, container, new File(["name,displayName"], "groups.xlsx"));
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "import" }));
    return dialog;
  }

  it("routes a .json file to the JSON import chain and never opens the Excel dialog", async () => {
    const user = userEvent.setup();
    vi.mocked(importGroups).mockResolvedValue({ total: 1, succeeded: 1, failed: 0, errors: [] });
    const { invalidateSpy, container } = renderPage("/groups?organizationId=org-2");

    await selectImportFile(
      user,
      container,
      new File([JSON.stringify([{ name: "eng", displayName: "Eng" }])], "groups.json", {
        type: "application/json",
      }),
    );

    await waitFor(() =>
      expect(importGroups).toHaveBeenCalledWith("org-2", [{ name: "eng", displayName: "Eng" }]),
    );
    expect(importGroupsFromExcel).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    await waitFor(() => expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["groups"] }));
  });

  it("passes the URL organization and invalidates the list after a successful import", async () => {
    const user = userEvent.setup();
    vi.mocked(importGroupsFromExcel).mockResolvedValue({
      total: 2,
      succeeded: 2,
      failed: 0,
      errors: [],
    });
    const { invalidateSpy, container } = renderPage("/groups?organizationId=org-2");

    const dialog = await uploadExcel(user, container);

    await waitFor(() => expect(importGroupsFromExcel).toHaveBeenCalledTimes(1));
    const [uploadedFile, organizationId] = vi.mocked(importGroupsFromExcel).mock.calls[0];
    expect((uploadedFile as File).name).toBe("groups.xlsx");
    expect(organizationId).toBe("org-2");
    expect(importGroups).not.toHaveBeenCalled();
    await waitFor(() => expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["groups"] }));
    expect(within(dialog).getByText("importSuccess")).toBeInTheDocument();
  });

  it("keeps the failure result visible and skips the list invalidation when import fails", async () => {
    const user = userEvent.setup();
    vi.mocked(importGroupsFromExcel).mockRejectedValue(new Error("backend down"));
    const { invalidateSpy, container } = renderPage("/groups");

    const dialog = await uploadExcel(user, container);

    await waitFor(() => expect(within(dialog).getByText("importPartial")).toBeInTheDocument());
    expect(within(dialog).getByText("Upload failed")).toBeInTheDocument();
    expect(invalidateSpy).not.toHaveBeenCalled();
  });
});

describe("GroupsPage single delete path", () => {
  const group = {
    id: "group-1",
    owner: "org-1",
    name: "eng",
    displayName: "Engineering",
    type: "physical",
    parentId: "",
    isTopGroup: true,
    isEnabled: true,
  };

  it("exposes one delete trigger and sends exactly one DELETE through the shared mutation", async () => {
    const user = userEvent.setup();
    vi.mocked(getGroupsPaginated).mockResolvedValue({
      items: [group],
      page: 1,
      pageSize: 50,
      total: 1,
    });
    vi.mocked(deleteGroup).mockResolvedValue(undefined);
    const { invalidateSpy } = renderPage("/groups");

    await screen.findByText("eng");

    // 行操作只有一个删除入口；未选中行时批量删除按钮不渲染。
    const deleteTriggers = screen.getAllByRole("button", { name: "delete" });
    expect(deleteTriggers).toHaveLength(1);

    await user.click(deleteTriggers[0]);
    const confirm = await waitFor(() => {
      const button = document.querySelector<HTMLElement>(".ant-popconfirm .ant-btn-primary");
      if (!button) throw new Error("delete confirmation did not open");
      return button;
    });
    await user.click(confirm);

    await waitFor(() => expect(deleteGroup).toHaveBeenCalledTimes(1));
    expect(deleteGroup).toHaveBeenCalledWith("group-1", "org-1");
    await waitFor(() => expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["groups"] }));
  });

  it("maps bulk-selected row keys back to record ids before deleting", async () => {
    const user = userEvent.setup();
    vi.mocked(getGroupsPaginated).mockResolvedValue({
      items: [group],
      page: 1,
      pageSize: 50,
      total: 1,
    });
    vi.mocked(deleteGroup).mockResolvedValue(undefined);
    renderPage("/groups");

    await screen.findByText("eng");

    // 选中集合存的是 rowKey（`owner/name`），批量删除必须映射回 record.id。
    const checkboxes = screen.getAllByRole("checkbox");
    await user.click(checkboxes[checkboxes.length - 1]);

    await user.click(await screen.findByRole("button", { name: "batchDelete" }));
    const confirm = await waitFor(() => {
      const button = document.querySelector<HTMLElement>(".ant-popconfirm .ant-btn-primary");
      if (!button) throw new Error("bulk delete confirmation did not open");
      return button;
    });
    await user.click(confirm);

    await waitFor(() => expect(deleteGroup).toHaveBeenCalledWith("group-1", "org-1"));
    expect(deleteGroup).not.toHaveBeenCalledWith("org-1/eng");
  });
});
