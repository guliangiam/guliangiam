import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { App } from "antd";
import type { UseQueryResult } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { renderCrudListView, type CrudListViewConfig } from "./CrudListPage";
import type { CrudListResult } from "../hooks/useCrudList";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) =>
      key === "selected" ? `Selected: ${opts?.count ?? 0}` : key,
  }),
}));

const permissionState = vi.hoisted(() => ({ allowed: false }));

vi.mock("@/shared/hooks/usePermissions", () => ({
  usePermissions: () => ({
    permissions: { menus: [], actions: [], resources: [] },
    isLoading: false,
    hasMenu: () => false,
    hasAction: () => permissionState.allowed,
    hasResource: () => permissionState.allowed,
    refetch: vi.fn<(...args: unknown[]) => unknown>(),
  }),
}));

type Item = { id: string; name: string };

const COLUMNS = [{ title: "Name", dataIndex: "name", key: "name" }];

function makeList(overrides: Partial<CrudListResult<Item, string>> = {}) {
  const query = {
    data: [
      { id: "1", name: "Alice" },
      { id: "2", name: "Bob" },
    ],
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    refetch: vi.fn<(...args: unknown[]) => unknown>(),
    isSuccess: true,
    isPending: false,
    status: "success",
    fetchStatus: "idle",
  } as unknown as UseQueryResult<Item[], Error>;

  const list = {
    query,
    pagination: { page: 1, pageSize: 50, total: 2 },
    search: "",
    filters: {},
    selectedKeys: [],
    setSelectedKeys: vi.fn<(...args: unknown[]) => unknown>(),
    setSearch: vi.fn<(...args: unknown[]) => unknown>(),
    setFilters: vi.fn<(...args: unknown[]) => unknown>(),
    setSortField: vi.fn<(...args: unknown[]) => unknown>(),
    setSortOrder: vi.fn<(...args: unknown[]) => unknown>(),
    deleteMutation: { isPending: false },
    bulkDeleteMutation: {
      isPending: false,
      mutateAsync: vi.fn<(...args: unknown[]) => Promise<unknown>>(async () => ({
        total: 1,
        succeeded: 1,
        failed: 0,
        failedKeys: [],
      })),
    },
    hasBulkDelete: true,
    isClientMode: false,
    serverPagination: {
      page: 1,
      pageSize: 50,
      total: 2,
      onChange: vi.fn<(...args: unknown[]) => unknown>(),
    },
    onSearch: vi.fn<(...args: unknown[]) => unknown>(),
    onFilterChange: vi.fn<(...args: unknown[]) => unknown>(),
    onSortChange: vi.fn<(...args: unknown[]) => unknown>(),
    refetch: vi.fn<(...args: unknown[]) => unknown>(),
  } as unknown as CrudListResult<Item, string>;

  return Object.assign(list, overrides);
}

/** renderCrudListView 是"在渲染期直接调用"的函数组件，必须由一个组件承载才能触发 hooks。 */
function Harness({
  config,
  list,
}: {
  config: CrudListViewConfig<Item, unknown, string>;
  list: CrudListResult<Item, string>;
}): ReactNode {
  return <App>{renderCrudListView<Item, unknown, string>(config, list)}</App>;
}

function renderList(config: CrudListViewConfig<Item, unknown, string>, list = makeList()) {
  return render(<Harness config={config} list={list} />);
}

describe("renderCrudListView bulk selection", () => {
  it("renders the selection column before anything is selected", () => {
    renderList({ title: "Items", rowKey: "id", columns: COLUMNS, enableBulkDelete: true });

    // header select-all + one checkbox per row
    expect(screen.getAllByRole("checkbox")).toHaveLength(3);
    expect(screen.queryByRole("toolbar")).not.toBeInTheDocument();
  });

  it("does not render a selection column when bulk actions are disabled", () => {
    renderList({ title: "Items", rowKey: "id", columns: COLUMNS });

    expect(screen.queryAllByRole("checkbox")).toHaveLength(0);
  });

  it("disables the checkbox of protected rows via rowSelectable", () => {
    renderList({
      title: "Items",
      rowKey: "id",
      columns: COLUMNS,
      enableBulkDelete: true,
      rowSelectable: (record) => record.name !== "Bob",
    });

    const bobRow = screen.getByText("Bob").closest("tr");
    expect(bobRow).not.toBeNull();
    expect(within(bobRow as HTMLElement).getByRole("checkbox")).toBeDisabled();

    const aliceRow = screen.getByText("Alice").closest("tr");
    expect(within(aliceRow as HTMLElement).getByRole("checkbox")).not.toBeDisabled();
  });

  it("shows the bulk bar and deletes the selected rows through the shared action", async () => {
    const list = makeList({ selectedKeys: ["1"] });
    renderList({ title: "Items", rowKey: "id", columns: COLUMNS, enableBulkDelete: true }, list);

    expect(screen.getByRole("toolbar")).toBeInTheDocument();
    expect(screen.getByText("Selected: 1")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "batchDelete" }));
    await userEvent.click(screen.getByRole("button", { name: "ok" }));

    expect(list.bulkDeleteMutation.mutateAsync).toHaveBeenCalledWith(["1"]);
  });

  it("renders extra management actions next to the default delete", () => {
    renderList(
      {
        title: "Items",
        rowKey: "id",
        columns: COLUMNS,
        enableBulkDelete: true,
        bulkActions: () => <button type="button">Bulk Enable</button>,
      },
      makeList({ selectedKeys: ["1"] }),
    );

    expect(screen.getByRole("button", { name: "batchDelete" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Bulk Enable" })).toBeInTheDocument();
  });
});

describe("renderCrudListView bulkDeletePermission", () => {
  it("hides the bulk delete action but keeps the selection column when denied", () => {
    permissionState.allowed = false;

    renderList(
      {
        title: "Items",
        rowKey: "id",
        columns: COLUMNS,
        enableBulkDelete: true,
        bulkDeletePermission: { permission: "item:delete", resource: "items" },
        bulkActions: () => <button type="button">Bulk Enable</button>,
      },
      makeList({ selectedKeys: ["1"] }),
    );

    expect(screen.queryByRole("button", { name: "batchDelete" })).not.toBeInTheDocument();
    // Selection column stays: header select-all + one checkbox per row.
    expect(screen.getAllByRole("checkbox")).toHaveLength(3);
    expect(screen.getByRole("button", { name: "Bulk Enable" })).toBeInTheDocument();
  });

  it("renders the bulk delete action when the prop is omitted", () => {
    permissionState.allowed = false;

    renderList(
      { title: "Items", rowKey: "id", columns: COLUMNS, enableBulkDelete: true },
      makeList({ selectedKeys: ["1"] }),
    );

    expect(screen.getByRole("button", { name: "batchDelete" })).toBeInTheDocument();
  });
});
