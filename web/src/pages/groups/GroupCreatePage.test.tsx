import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { App } from "antd";
import { QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createTestQueryClient } from "@/test-utils/render";
import GroupCreatePage from "./GroupCreatePage";

const api = vi.hoisted(() => ({
  createGroup: vi.fn<(payload: unknown) => Promise<unknown>>(),
  getGroups: vi.fn<() => Promise<unknown[]>>(),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock("@/shared/api/iam/group", () => api);

// 叶子选择器只负责把值交给 Form.Item，页面级测试聚焦 payload 装配与副作用。
vi.mock("@/shared/ui/OrganizationSelect", () => ({
  OrganizationSelect: ({
    value,
    onChange,
  }: {
    value?: string;
    onChange?: (value: string) => void;
  }) => (
    <button type="button" onClick={() => onChange?.("org-1")}>
      {`org:${value ?? ""}`}
    </button>
  ),
}));

vi.mock("@/shared/ui/GroupTreeSelect", () => ({
  GroupTreeSelect: ({
    value,
    onChange,
    groups,
  }: {
    value?: string;
    onChange?: (value: string) => void;
    groups: Array<{ id: string; displayName: string }>;
  }) => (
    <select
      aria-label="parent-group"
      value={value ?? ""}
      onChange={(event) => onChange?.(event.target.value)}
    >
      <option value="">none</option>
      {groups.map((group) => (
        <option key={group.id} value={group.id}>
          {group.displayName}
        </option>
      ))}
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
    <button type="button" onClick={() => onChange?.(["alice"])}>
      {`users:${(value ?? []).join(",")}`}
    </button>
  ),
}));

function renderPage() {
  const queryClient = createTestQueryClient();
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
  const router = createMemoryRouter(
    [
      { path: "/groups/new", element: <GroupCreatePage /> },
      { path: "/groups", element: <div data-testid="groups-list" /> },
    ],
    { initialEntries: ["/groups/new"] },
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

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  api.getGroups.mockResolvedValue([
    {
      id: "group-parent",
      owner: "org-1",
      name: "engineering",
      displayName: "Engineering",
      type: "virtual",
      parentId: "",
      isTopGroup: true,
      isEnabled: true,
    },
    {
      id: "group-foreign",
      owner: "org-2",
      name: "sales",
      displayName: "Sales",
      type: "virtual",
      parentId: "",
      isTopGroup: true,
      isEnabled: true,
    },
  ]);
  api.createGroup.mockResolvedValue(undefined);
});

describe("GroupCreatePage save closure", () => {
  it("submits the selected organization, parent, users and flags, then navigates to the list", async () => {
    const user = userEvent.setup();
    // 保存成功后的跳转不能触发「未保存修改」确认：确认框一律拒绝，
    // 若守卫仍然拦截，页面就不会切到列表，下面的断言会失败。
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);
    const { invalidateSpy } = renderPage();

    await user.click(await screen.findByRole("button", { name: "org:" }));

    const name = screen.getByLabelText("name");
    await user.clear(name);
    await user.type(name, "engineering");

    const displayName = screen.getByLabelText("displayName");
    await user.clear(displayName);
    await user.type(displayName, "Engineering");

    const parent = screen.getByLabelText("parent-group");
    await waitFor(() =>
      expect(screen.getByRole("option", { name: "Engineering" })).toBeInTheDocument(),
    );
    await user.selectOptions(parent, "group-parent");

    await user.click(screen.getByRole("button", { name: "users:" }));
    await user.click(screen.getByLabelText("group.isTopGroup"));
    await user.click(screen.getByRole("button", { name: "save" }));

    await waitFor(() => expect(api.createGroup).toHaveBeenCalledTimes(1));
    expect(api.createGroup.mock.calls[0][0]).toMatchObject({
      organizationId: "org-1",
      name: "engineering",
      displayName: "Engineering",
      parentId: "group-parent",
      users: ["alice"],
      isTopGroup: true,
      isEnabled: true,
    });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["groups"] });
    expect(await screen.findByTestId("groups-list")).toBeInTheDocument();
    // 跳转后创建页已卸载，不存在「再点一次保存重复建组」的入口。
    expect(confirmSpy).not.toHaveBeenCalled();
    expect(api.createGroup).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("button", { name: "save" })).not.toBeInTheDocument();
    confirmSpy.mockRestore();
  });
});
