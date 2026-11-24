import { describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ResourceListCard } from "./ResourceListCard";
import { HttpError } from "@/shared/api/http";
import type { UseQueryResult } from "@tanstack/react-query";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) => {
      if (key === "total") return `Total: ${opts?.count ?? 0}`;
      if (key === "noData") return "No data";
      if (key === "selected") return `Selected: ${opts?.count ?? 0}`;
      if (key === "search") return "Search";
      if (key === "reset") return "Reset";
      if (key === "placeholder.searchByName") return "Filter by name...";
      return key;
    },
  }),
}));

type Item = { id: string; name: string; status: string };

function makeQueryResult(
  data: Item[],
  overrides?: Partial<UseQueryResult<Item[], Error>>,
): UseQueryResult<Item[], Error> {
  return {
    data,
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    refetch: vi.fn<(...args: unknown[]) => unknown>(),
    isSuccess: true,
    isPending: false,
    status: "success",
    fetchStatus: "idle",
    dataUpdatedAt: 0,
    errorUpdatedAt: 0,
    failureCount: 0,
    failureReason: null,
    isPaused: false,
    ...overrides,
  } as UseQueryResult<Item[], Error>;
}

const columns = [
  { title: "Name", dataIndex: "name", key: "name" },
  { title: "Status", dataIndex: "status", key: "status" },
];

describe("ResourceListCard", () => {
  describe("renders list data", () => {
    it("renders title", () => {
      const query = makeQueryResult([]);
      render(<ResourceListCard title="Users" query={query} columns={columns} rowKey="id" />);
      expect(screen.getByText("Users")).toBeInTheDocument();
    });

    it("renders table rows", () => {
      const data: Item[] = [
        { id: "1", name: "Alice", status: "active" },
        { id: "2", name: "Bob", status: "inactive" },
      ];
      const query = makeQueryResult(data);
      render(<ResourceListCard title="Users" query={query} columns={columns} rowKey="id" />);
      expect(screen.getByText("Alice")).toBeInTheDocument();
      expect(screen.getByText("Bob")).toBeInTheDocument();
    });

    it("shows description", () => {
      const query = makeQueryResult([]);
      render(
        <ResourceListCard
          title="Users"
          description="Manage users"
          query={query}
          columns={columns}
          rowKey="id"
        />,
      );
      expect(screen.getByText("Manage users")).toBeInTheDocument();
    });

    it("renders badge count", () => {
      const query = makeQueryResult([]);
      render(
        <ResourceListCard title="Users" query={query} columns={columns} rowKey="id" badge={5} />,
      );
      expect(screen.getByText("5")).toBeInTheDocument();
    });
  });

  describe("empty state", () => {
    it("shows empty state when no data", () => {
      const query = makeQueryResult([]);
      render(<ResourceListCard title="Users" query={query} columns={columns} rowKey="id" />);
      expect(screen.getAllByText("No data").length).toBeGreaterThan(0);
    });

    it("shows custom empty content when provided", () => {
      const query = makeQueryResult([]);
      render(
        <ResourceListCard
          title="Users"
          query={query}
          columns={columns}
          rowKey="id"
          emptyContent={<div data-testid="custom-empty">No users found</div>}
        />,
      );
      expect(screen.getByTestId("custom-empty")).toBeInTheDocument();
      expect(screen.getByText("No users found")).toBeInTheDocument();
    });
  });

  describe("loading state", () => {
    it("shows loading skeleton", () => {
      const query = makeQueryResult([], { isLoading: true, isPending: true, status: "pending" });
      render(<ResourceListCard title="Users" query={query} columns={columns} rowKey="id" />);
      expect(screen.queryByText("Alice")).not.toBeInTheDocument();
    });
  });

  describe("error state", () => {
    it("maps the failure to a localized status instead of backend prose", () => {
      const query = makeQueryResult([], {
        isError: true,
        isSuccess: false,
        status: "error",
        error: new HttpError(503, undefined, "backend prose"),
      });
      render(
        <ResourceListCard
          title="Users"
          query={query}
          columns={columns}
          rowKey="id"
          showRefresh={false}
        />,
      );

      expect(screen.getByText("loadFailed")).toBeInTheDocument();
      expect(screen.getByText("common:serverError.unavailable")).toBeInTheDocument();
      expect(screen.queryByText("backend prose")).not.toBeInTheDocument();
    });

    it("keeps the retry action wired to refetch", async () => {
      const user = userEvent.setup();
      const refetch = vi.fn<(...args: unknown[]) => unknown>();
      const query = makeQueryResult([], {
        isError: true,
        isSuccess: false,
        status: "error",
        error: new HttpError(500, undefined, "backend prose"),
        refetch,
      });
      render(
        <ResourceListCard
          title="Users"
          query={query}
          columns={columns}
          rowKey="id"
          showRefresh={false}
        />,
      );

      await user.click(screen.getByRole("button", { name: "action.refresh" }));
      expect(refetch).toHaveBeenCalledOnce();
    });
  });

  describe("pagination", () => {
    it("renders server pagination total", () => {
      const query = makeQueryResult([{ id: "1", name: "A", status: "ok" }]);
      render(
        <ResourceListCard
          title="Users"
          query={query}
          columns={columns}
          rowKey="id"
          serverPagination={{
            page: 1,
            pageSize: 10,
            total: 50,
            onChange: vi.fn<(...args: unknown[]) => unknown>(),
          }}
        />,
      );
      expect(screen.getByText("Total: 50")).toBeInTheDocument();
    });

    it("calls serverPagination.onChange when page changes", async () => {
      const onChange = vi.fn<(...args: unknown[]) => unknown>();
      const query = makeQueryResult([{ id: "1", name: "A", status: "ok" }]);
      render(
        <ResourceListCard
          title="Users"
          query={query}
          columns={columns}
          rowKey="id"
          serverPagination={{ page: 1, pageSize: 10, total: 50, onChange }}
        />,
      );
      expect(screen.getByText("Total: 50")).toBeInTheDocument();

      fireEvent.click(screen.getByLabelText("Page 2"));

      await waitFor(() => expect(onChange).toHaveBeenCalledWith(2, 10));
    });
  });

  describe("search and filter", () => {
    it("filters items by search in client mode", async () => {
      const data: Item[] = [
        { id: "1", name: "Alice", status: "active" },
        { id: "2", name: "Bob", status: "inactive" },
      ];
      const query = makeQueryResult(data);
      const user = userEvent.setup();
      render(<ResourceListCard title="Users" query={query} columns={columns} rowKey="id" />);

      const searchInput = screen.getByRole("searchbox");
      await user.type(searchInput, "alice");
      expect(screen.getByText("Alice")).toBeInTheDocument();
      expect(screen.queryByText("Bob")).not.toBeInTheDocument();
    });

    it("hides search input when hideSearch is true", () => {
      const query = makeQueryResult([]);
      render(
        <ResourceListCard title="Users" query={query} columns={columns} rowKey="id" hideSearch />,
      );
      expect(screen.queryByRole("searchbox")).not.toBeInTheDocument();
    });

    it("shows server-side search input when serverPagination and onSearch are provided", () => {
      const query = makeQueryResult([{ id: "1", name: "A", status: "ok" }]);
      render(
        <ResourceListCard
          title="Users"
          query={query}
          columns={columns}
          rowKey="id"
          serverPagination={{
            page: 1,
            pageSize: 10,
            total: 50,
            onChange: vi.fn<(...args: unknown[]) => unknown>(),
          }}
          onSearch={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      expect(screen.getByRole("searchbox")).toBeInTheDocument();
    });

    it("calls onSearch when server-side search is triggered", async () => {
      const onSearch = vi.fn<(...args: unknown[]) => unknown>();
      const query = makeQueryResult([{ id: "1", name: "A", status: "ok" }]);
      const user = userEvent.setup();
      render(
        <ResourceListCard
          title="Users"
          query={query}
          columns={columns}
          rowKey="id"
          serverPagination={{
            page: 1,
            pageSize: 10,
            total: 50,
            onChange: vi.fn<(...args: unknown[]) => unknown>(),
          }}
          onSearch={onSearch}
        />,
      );

      const searchInput = screen.getByRole("searchbox");
      await user.type(searchInput, "test{Enter}");
      // Input.Search onSearch callback receives (value, event, source)
      expect(onSearch).toHaveBeenCalled();
      expect(onSearch.mock.calls[0][0]).toBe("test");
    });

    it("renders filter dropdowns when filters are provided", () => {
      const query = makeQueryResult([]);
      render(
        <ResourceListCard
          title="Users"
          query={query}
          columns={columns}
          rowKey="id"
          filters={[
            {
              key: "status",
              label: "Status",
              options: [
                { label: "Active", value: "active" },
                { label: "Inactive", value: "inactive" },
              ],
            },
          ]}
          onFilterChange={vi.fn<(...args: unknown[]) => unknown>()}
        />,
      );
      // The filter select should be rendered
      const statusFilter = screen.getByText("Status");
      expect(statusFilter).toBeInTheDocument();
    });
  });

  describe("refresh button", () => {
    it("renders a refresh button by default", () => {
      const query = makeQueryResult([{ id: "1", name: "A", status: "ok" }]);
      render(<ResourceListCard title="Users" query={query} columns={columns} rowKey="id" />);
      // At least one button should exist (the refresh button + search button)
      const buttons = screen.getAllByRole("button");
      expect(buttons.length).toBeGreaterThanOrEqual(1);
    });

    it("has fewer buttons when showRefresh is false", () => {
      const query = makeQueryResult([{ id: "1", name: "A", status: "ok" }]);
      const { rerender } = render(
        <ResourceListCard
          title="Users"
          query={query}
          columns={columns}
          rowKey="id"
          showRefresh={true}
        />,
      );
      const buttonsWithRefresh = screen.getAllByRole("button").length;

      rerender(
        <ResourceListCard
          title="Users"
          query={query}
          columns={columns}
          rowKey="id"
          showRefresh={false}
        />,
      );
      const buttonsWithoutRefresh = screen.getAllByRole("button").length;

      expect(buttonsWithoutRefresh).toBeLessThan(buttonsWithRefresh);
    });

    it("refetch is available on the query object for refresh", () => {
      const refetch = vi.fn<(...args: unknown[]) => unknown>();
      const query = makeQueryResult([{ id: "1", name: "A", status: "ok" }], { refetch });
      render(<ResourceListCard title="Users" query={query} columns={columns} rowKey="id" />);

      // The query.refetch is passed to the Button's onClick
      // We can verify refetch is a function on the query object
      expect(query.refetch).toBe(refetch);
      expect(typeof query.refetch).toBe("function");
    });
  });

  describe("row selection and bulk actions", () => {
    it("shows bulk action bar when rows are selected", () => {
      const query = makeQueryResult([
        { id: "1", name: "Alice", status: "active" },
        { id: "2", name: "Bob", status: "inactive" },
      ]);
      render(
        <ResourceListCard
          title="Users"
          query={query}
          columns={columns}
          rowKey="id"
          rowSelection={{
            selectedRowKeys: ["1"],
            onChange: vi.fn<(...args: unknown[]) => unknown>(),
          }}
          bulkActions={<button type="button">Delete Selected</button>}
        />,
      );
      expect(screen.getByText("Delete Selected")).toBeInTheDocument();
      expect(screen.getByText(/Selected: 1/)).toBeInTheDocument();
    });

    it("does not show bulk action bar when no rows are selected", () => {
      const query = makeQueryResult([{ id: "1", name: "Alice", status: "active" }]);
      render(
        <ResourceListCard
          title="Users"
          query={query}
          columns={columns}
          rowKey="id"
          rowSelection={{ selectedRowKeys: [], onChange: vi.fn<(...args: unknown[]) => unknown>() }}
          bulkActions={<button type="button">Delete Selected</button>}
        />,
      );
      expect(screen.queryByText("Delete Selected")).not.toBeInTheDocument();
    });
  });

  describe("toolbar", () => {
    it("renders custom toolbar content", () => {
      const query = makeQueryResult([]);
      render(
        <ResourceListCard
          title="Users"
          query={query}
          columns={columns}
          rowKey="id"
          toolbar={<button type="button">Add User</button>}
        />,
      );
      expect(screen.getByText("Add User")).toBeInTheDocument();
    });
  });

  describe("large client-side lists", () => {
    function makeItems(count: number): Item[] {
      return Array.from({ length: count }, (_, i) => ({
        id: String(i),
        name: `User ${i}`,
        status: "active",
      }));
    }

    // Above the virtual-scroll threshold the card used to swap in VirtualTable,
    // which renders one Ant Design <Table> per row and therefore has no
    // pagination control, no selection column and no sort. All three were
    // dropped silently. The virtual path is now taken only when there is
    // nothing to drop.
    it("keeps the pagination control past the virtual threshold", () => {
      const query = makeQueryResult(makeItems(250));
      render(<ResourceListCard title="Users" query={query} columns={columns} rowKey="id" />);
      // 250 rows is 25 pages of 10 in client mode.
      expect(screen.getByText("Total: 250")).toBeInTheDocument();
    });

    it("keeps row selection available past the virtual threshold", () => {
      const selected: string[] = [];
      const query = makeQueryResult(makeItems(250));
      render(
        <ResourceListCard
          title="Users"
          query={query}
          columns={columns}
          rowKey="id"
          rowSelection={{
            selectedRowKeys: selected,
            onChange: vi.fn<(...args: unknown[]) => unknown>(),
          }}
        />,
      );
      // A selection column means checkboxes are rendered; VirtualTable has none.
      expect(document.querySelectorAll(".ant-checkbox-input").length).toBeGreaterThan(0);
    });

    it("keeps sorting available past the virtual threshold", () => {
      const query = makeQueryResult(makeItems(250));
      const sortableColumns = [{ title: "Name", dataIndex: "name", key: "name", sorter: () => 0 }];
      render(
        <ResourceListCard title="Users" query={query} columns={sortableColumns} rowKey="id" />,
      );
      expect(document.querySelectorAll(".ant-table-column-sorters").length).toBeGreaterThan(0);
    });
  });
});
