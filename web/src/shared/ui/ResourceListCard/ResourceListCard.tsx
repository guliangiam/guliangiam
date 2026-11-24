import { useCallback, useDeferredValue, useMemo, useState, memo } from "react";
import {
  Alert,
  Badge,
  Button,
  Card,
  Empty,
  Input,
  Pagination,
  Select,
  Skeleton,
  Space,
  Table,
  Typography,
} from "antd";
import ReloadOutlined from "@ant-design/icons/ReloadOutlined";
import type { ColumnsType, TablePaginationConfig } from "antd/es/table";
import type { UseQueryResult } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { VirtualTable } from "@/shared/ui/VirtualTable";
import { BatchActionBar } from "@/shared/ui/BatchActionBar";
import { useColumnPreferences } from "@/shared/hooks/useColumnPreferences";
import { serverErrorMessage } from "@/shared/lib/server-error";
import { ColumnMenu } from "./ColumnMenu";
import { createColumnSearchDropdown, type SearchableColumn } from "./ColumnSearchDropdown";

/**
 * Row count above which a client-side list is rendered with virtual scrolling.
 *
 * Only client mode can ever reach it: the server caps pageSize at
 * platformpagination.MaxPageSize (100), so a server-paginated page never holds
 * this many rows.
 */
const VIRTUAL_ROW_THRESHOLD = 200;

/** Rows per page in client mode, matching the Table pagination below. */
const CLIENT_PAGE_SIZE = 10;

/** Simple highlight: wraps matching text in a <mark> tag. */
function HighlightText({ text, search }: { text: string; search: string }) {
  if (!search) return <>{text}</>;
  const lower = text.toLowerCase();
  const idx = lower.indexOf(search.toLowerCase());
  if (idx === -1) return <>{text}</>;
  return (
    <>
      {text.slice(0, idx)}
      <mark style={{ backgroundColor: "#ffc069", padding: 0 }}>
        {text.slice(idx, idx + search.length)}
      </mark>
      {text.slice(idx + search.length)}
    </>
  );
}

type FilterOption = {
  label: string;
  value: string;
};

export type FilterConfig = {
  key: string;
  label: string;
  options: FilterOption[];
};

export type ServerPagination = {
  page: number;
  pageSize: number;
  total: number;
  onChange: (page: number, pageSize: number) => void;
};

type ResourceListCardProps<T extends object> = {
  title?: ReactNode;
  description?: string;
  searchPlaceholder?: string;
  query: UseQueryResult<T[], Error>;
  columns: ColumnsType<T>;
  rowKey: keyof T | ((record: T) => string);
  toolbar?: ReactNode;
  /** Define which columns are searchable. Enables column-specific search dropdowns. */
  searchableColumns?: SearchableColumn[];
  /** Filter dropdowns for structured filtering (e.g. status, type). */
  filters?: FilterConfig[];
  /** Current filter values. */
  filterValues?: Record<string, string>;
  /** Callback when filter values change. */
  onFilterChange?: (filters: Record<string, string>) => void;
  /** Enable row selection for bulk operations. */
  rowSelection?: {
    selectedRowKeys: string[];
    onChange: (selectedRowKeys: string[]) => void;
    /** Rows returning `disabled: true` cannot be selected (protected objects). */
    getCheckboxProps?: (record: T) => { disabled?: boolean };
  };
  /** Bulk action bar rendered when rows are selected. */
  bulkActions?: ReactNode;
  /** Server-side pagination. When provided, disables client-side search/pagination. */
  serverPagination?: ServerPagination;
  /** Server-side search callback. When provided with serverPagination, shows search input. */
  onSearch?: (value: string) => void;
  /** Loading state label for accessibility */
  loadingLabel?: string;
  /** Show refresh button in toolbar */
  showRefresh?: boolean;
  /** Badge count shown next to title */
  badge?: number;
  /** Hide search input */
  hideSearch?: boolean;
  /** Default sort field for the table */
  defaultSortField?: string;
  /** Default sort order */
  defaultSortOrder?: "ascend" | "descend";
  /** Callback when server-side sort changes (field, order) */
  onSortChange?: (field: string | undefined, order: "ascend" | "descend" | null) => void;
  /** Callback when column-specific search is triggered in server mode. */
  onColumnSearch?: (field: string, value: string) => void;
  /** Custom empty state content. When provided, replaces the default Empty component. */
  emptyContent?: ReactNode;
  /** Expandable row configuration, passed through to Ant Design Table's expandable prop. */
  expandable?: Record<string, unknown>;
  /** Row className callback, passed through to Ant Design Table's rowClassName prop. */
  rowClassName?: (record: T) => string;
  /** Row props callback, passed through to Ant Design Table's onRow prop. */
  onRow?: (record: T) => Record<string, unknown>;
  /**
   * Horizontal scroll width for the underlying Ant Design Table.
   * Defaults to "max-content", so a horizontal scrollbar only appears when the
   * columns cannot fit the viewport. Pass a number to pin a fixed width.
   */
  tableScrollX?: number | string;
  /**
   * Unique table identifier for column preference persistence.
   * When provided, columns' visibility is saved to localStorage as
   *   gl:col-pref:{tableId} and restored on mount.
   * If omitted, column visibility is not persisted.
   */
  tableId?: string;
  /**
   * Data indexes hidden on first visit. They stay in the column menu, so the
   * operator can reveal them; persisted choices always win over this default.
   */
  defaultHiddenColumns?: readonly string[];
  /** When true, skip the outer Card shell — renders header/table directly. */
  embedded?: boolean;
};

export const ResourceListCard = memo(function ResourceListCard<T extends object>({
  title,
  description,
  searchPlaceholder,
  query,
  columns,
  rowKey,
  toolbar,
  searchableColumns,
  filters,
  filterValues,
  onFilterChange,
  rowSelection,
  bulkActions,
  serverPagination,
  onSearch,
  showRefresh = true,
  badge,
  hideSearch,
  defaultSortField,
  defaultSortOrder,
  onSortChange,
  onColumnSearch,
  emptyContent,
  expandable,
  rowClassName,
  onRow,
  tableScrollX,
  tableId,
  defaultHiddenColumns,
  embedded,
}: ResourceListCardProps<T>) {
  const { t } = useTranslation("common");
  const [search, setSearch] = useState("");
  const [searchedColumn, setSearchedColumn] = useState("");
  const deferredSearch = useDeferredValue(search.trim().toLowerCase());

  const isServerMode = !!serverPagination;

  // 选中集合属于"当前视图"：client mode 的搜索/翻页在本组件内部完成，不经过
  // 壳层 useCrudList 的清空 handlers，必须在这里同步清空选中，否则批量动作
  // 会作用到搜索/翻页后已不可见的行。onChange 即 useState setter，引用稳定。
  const clearSelection = rowSelection?.onChange;
  const applyClientSearch = useCallback(
    (value: string) => {
      setSearch(value);
      clearSelection?.([]);
    },
    [clearSelection],
  );

  // --- Column visibility preferences (persisted per tableId) ---------------
  // 从 columns 中提取可持久化的 dataIndex 列表：必须有 dataIndex 且非 rowSelection 专用列
  const persistableCols = useMemo(
    () =>
      columns
        .filter(
          (c) =>
            "dataIndex" in c &&
            c.dataIndex !== undefined &&
            (c as { key?: unknown }).key !== "__row_selection__",
        )
        .map((c) => ({ dataIndex: String((c as { dataIndex?: unknown }).dataIndex) })),
    [columns],
  );
  const colPrefs = useColumnPreferences(tableId, persistableCols, !!tableId, defaultHiddenColumns);

  const items = useMemo(() => {
    if (isServerMode) return query.data ?? [];
    if (!deferredSearch) return query.data ?? [];
    return (query.data ?? []).filter((item) => {
      return Object.values(item).some((value) =>
        String(value ?? "")
          .toLowerCase()
          .includes(deferredSearch),
      );
    });
  }, [query.data, deferredSearch, isServerMode]);

  const highlightRender = useCallback(
    (text: unknown, dataIndex: string) => {
      if (searchedColumn === dataIndex && deferredSearch) {
        return <HighlightText text={text ? String(text) : ""} search={deferredSearch} />;
      }
      return text as ReactNode;
    },
    [searchedColumn, deferredSearch],
  );

  const enhancedColumns = useMemo(() => {
    let cols = columns;

    // Apply defaultSortOrder to matching column
    if (defaultSortField) {
      cols = cols.map((col) => {
        const colDataIndex = (col as Record<string, unknown>).dataIndex as string | undefined;
        if (colDataIndex === defaultSortField && (col as Record<string, unknown>).sorter) {
          return { ...col, defaultSortOrder: defaultSortOrder ?? "ascend" };
        }
        return col;
      });
    }

    // Apply searchable column dropdowns (auto-match control by columnType)
    if (searchableColumns && searchableColumns.length > 0) {
      cols = cols.map((col) => {
        const searchable = searchableColumns.find(
          (sc) => sc.dataIndex === (col as Record<string, unknown>).dataIndex,
        );
        if (!searchable) return col;

        return {
          ...col,
          filterDropdown: createColumnSearchDropdown({
            searchable,
            isServerMode,
            t,
            onColumnSearch,
            setSearch: applyClientSearch,
            setSearchedColumn,
          }),
          onFilter: undefined,
          render: col.render
            ? col.render
            : (text: unknown) => highlightRender(text, searchable.dataIndex),
        };
      });
    }

    // Apply column visibility filter (persisted per tableId)
    if (tableId) {
      const visibleSet = colPrefs.visible;
      cols = cols.filter((col) => {
        const di = String((col as Record<string, unknown>).dataIndex ?? "");
        // 无 dataIndex 的列（如 selection 列、expandIcon 列）始终保留
        if (!di) return true;
        return visibleSet.has(di);
      });
    }

    return cols;
  }, [
    columns,
    searchableColumns,
    highlightRender,
    t,
    isServerMode,
    onColumnSearch,
    applyClientSearch,
    defaultSortField,
    defaultSortOrder,
    tableId,
    colPrefs.visible,
  ]);

  const [clientPage, setClientPage] = useState(1);

  const paginationConfig: false | TablePaginationConfig = isServerMode
    ? {
        current: serverPagination.page,
        pageSize: serverPagination.pageSize,
        total: serverPagination.total,
        showSizeChanger: true,
        showTotal: (total) => t("total", { count: total }),
        onChange: serverPagination.onChange,
      }
    : {
        pageSize: CLIENT_PAGE_SIZE,
        showSizeChanger: false,
        showTotal: (total) => t("total", { count: total }),
        onChange: (page) => {
          setClientPage(page);
          clearSelection?.([]);
        },
      };

  // VirtualTable renders one Ant Design <Table> per row, so it has no pagination
  // control, no selection column, no expandable row and no working sort. Taking
  // that path while any of those are requested used to drop them silently: a
  // list past the threshold lost its page controls and its bulk actions with no
  // error anywhere. Only virtualize when there is genuinely nothing to drop,
  // and paginate the virtual slice here so client mode keeps its paging.
  //
  // Server mode never reaches the threshold: the middleware caps pageSize at
  // platformpagination.MaxPageSize (100).
  const hasSortableColumn = useMemo(
    () => columns.some((column) => Boolean((column as { sorter?: unknown }).sorter)),
    [columns],
  );
  const virtualize =
    !isServerMode &&
    !rowSelection &&
    !expandable &&
    !hasSortableColumn &&
    !onSortChange &&
    items.length > VIRTUAL_ROW_THRESHOLD;

  const clientPageCount = Math.max(1, Math.ceil(items.length / CLIENT_PAGE_SIZE));
  // A search that shrinks the list must not leave the user on an empty page.
  const effectiveClientPage = Math.min(clientPage, clientPageCount);
  const virtualItems = useMemo(
    () =>
      items.slice(
        (effectiveClientPage - 1) * CLIENT_PAGE_SIZE,
        effectiveClientPage * CLIENT_PAGE_SIZE,
      ),
    [items, effectiveClientPage],
  );

  const selectedCount = rowSelection?.selectedRowKeys?.length ?? 0;

  const inner = (
    <>
      {!embedded && (
        <div className="gl-resource-card__header">
          <div>
            <Typography.Title level={2}>
              {title}
              {badge !== undefined && (
                <Badge
                  count={badge}
                  size="small"
                  style={{ backgroundColor: "var(--gl-color-primary)", marginLeft: 8 }}
                />
              )}
            </Typography.Title>
            {description && <Typography.Paragraph>{description}</Typography.Paragraph>}
          </div>
          <div className="gl-resource-card__toolbar">
            {!isServerMode && !hideSearch && (
              <Input.Search
                className="gl-resource-card__search"
                placeholder={searchPlaceholder || t("placeholder.searchByName")}
                aria-label={searchPlaceholder || t("placeholder.searchByName")}
                allowClear
                value={search}
                onChange={(event) => {
                  applyClientSearch(event.target.value);
                  setSearchedColumn("");
                }}
              />
            )}
            {isServerMode && onSearch && (
              <Input.Search
                className="gl-resource-card__search"
                placeholder={searchPlaceholder || t("placeholder.searchByName")}
                aria-label={searchPlaceholder || t("placeholder.searchByName")}
                allowClear
                onSearch={onSearch}
                onChange={(event) => {
                  if (!event.target.value) onSearch("");
                }}
              />
            )}
            {filters && filters.length > 0 && (
              <Space>
                {filters.map((filter) => (
                  <Select
                    key={filter.key}
                    placeholder={filter.label}
                    aria-label={filter.label}
                    allowClear
                    style={{ minWidth: 128 }}
                    value={filterValues?.[filter.key] || undefined}
                    onChange={(value) => {
                      const next = { ...filterValues, [filter.key]: value };
                      if (!value) delete next[filter.key];
                      onFilterChange?.(next);
                    }}
                    options={filter.options}
                  />
                ))}
              </Space>
            )}
            {showRefresh && (
              <Button
                icon={<ReloadOutlined />}
                onClick={() => query.refetch()}
                loading={query.isFetching}
                aria-label={t("action.refresh")}
              />
            )}

            {/* ColumnMenu — 列显隐切换（持久化到 localStorage） */}
            {tableId && (
              <ColumnMenu columns={columns} visible={colPrefs.visible} toggle={colPrefs.toggle} />
            )}

            {toolbar}
          </div>
        </div>
      )}

      {selectedCount > 0 && bulkActions && (
        <BatchActionBar
          selectedCount={selectedCount}
          onClearSelection={() => rowSelection?.onChange([])}
        >
          {bulkActions}
        </BatchActionBar>
      )}

      {query.isLoading ? (
        <Skeleton active paragraph={{ rows: 10 }} title={false} />
      ) : query.isError ? (
        <Alert
          type="error"
          title={t("loadFailed")}
          description={serverErrorMessage(query.error, t, "loadFailed")}
          showIcon
          action={
            <Button size="small" onClick={() => query.refetch()}>
              {t("action.refresh")}
            </Button>
          }
        />
      ) : items.length === 0 ? (
        (emptyContent ?? <Empty description={t("noData")} />)
      ) : virtualize ? (
        <>
          <VirtualTable<T>
            dataSource={virtualItems}
            columns={enhancedColumns}
            rowKey={rowKey}
            rowHeight={54}
            overscan={5}
          />
          <Pagination
            style={{ marginTop: 12, textAlign: "right" }}
            current={effectiveClientPage}
            pageSize={CLIENT_PAGE_SIZE}
            total={items.length}
            showSizeChanger={false}
            onChange={setClientPage}
            showTotal={(total) => t("total", { count: total })}
          />
        </>
      ) : (
        <Table<T>
          rowKey={rowKey as never}
          dataSource={items}
          columns={enhancedColumns}
          pagination={paginationConfig}
          rowSelection={
            rowSelection
              ? {
                  selectedRowKeys: rowSelection.selectedRowKeys,
                  onChange: (keys) => rowSelection.onChange(keys as string[]),
                  getCheckboxProps: rowSelection.getCheckboxProps
                    ? (record: T) => rowSelection.getCheckboxProps!(record)
                    : undefined,
                }
              : undefined
          }
          scroll={{ x: tableScrollX ?? "max-content" }}
          expandable={expandable}
          rowClassName={rowClassName}
          onRow={onRow}
          onChange={(_pagination, _filters, sorter, extra) => {
            // Pagination also fires Table onChange with an empty sorter. Calling
            // onSortChange there reset the page to 1 immediately after a page
            // click, so the list never left the first page.
            if (isServerMode && onSortChange && extra.action === "sort") {
              const sort = Array.isArray(sorter) ? sorter[0] : sorter;
              const field = sort.field as string | undefined;
              const order = sort.order as "ascend" | "descend" | null;
              onSortChange(field, order);
            }
          }}
        />
      )}
    </>
  );

  if (embedded) return inner;

  return (
    <Card className="gl-panel gl-resource-card" variant="borderless">
      {inner}
    </Card>
  );
}) as <T extends object>(props: ResourceListCardProps<T>) => React.JSX.Element;
