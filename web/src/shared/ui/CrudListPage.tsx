import { useMemo, type ReactElement, type ReactNode } from "react";
import { App } from "antd";
import { useTranslation } from "react-i18next";
import { ResourceListCard, type FilterConfig } from "./ResourceListCard";
import { PopconfirmModal } from "./PopconfirmModal";
import { PermissionGuard } from "./PermissionGuard";
import {
  useCrudList,
  type BulkDeleteSummary,
  type CrudListConfig,
  type CrudListResult,
} from "../hooks/useCrudList";
import type { ColumnsType } from "antd/es/table";

// ─────────────────────────────────────────────────────────────────────
// renderCrudListPage — config-driven 列表页
//
// 把"一个标准 CRUD 列表页"收敛成一份 config 对象。
// 页面只负责：
//   1. 定义 columns（Ant Design Table columns）
//   2. 提供 paginatedFn / deleteFn / buildQueryKey / buildParams
//   3. 传入 title / filters / bulkActions / toolbar 等 UI 配置
//
// hook 负责：pagination / search / filters / selectedKeys / mutations / handlers
// ─────────────────────────────────────────────────────────────────────

export type CrudListPageConfig<TItem, TParams, TDeleteKey> = {
  // ── UI ──
  title: ReactNode;
  description?: string;
  searchPlaceholder?: string;
  rowKey: keyof TItem | ((record: TItem) => string);
  /**
   * 静态列，或接收 CrudListResult 的工厂函数。行删等行内 mutation 必须走
   * list.deleteMutation（统一缓存失效与错误提示），用函数形态拿到 list。
   */
  columns: ColumnsType<TItem> | ((list: CrudListResult<TItem, TDeleteKey>) => ColumnsType<TItem>);
  filters?: FilterConfig[];
  searchableColumns?: Array<{ dataIndex: string; label?: string }>;
  defaultSortField?: string;
  defaultSortOrder?: "ascend" | "descend";
  showRefresh?: boolean;
  badge?: number;
  hideSearch?: boolean;
  expandable?: Record<string, unknown>;
  emptyContent?: ReactNode;
  /**
   * Unique identifier for column preference persistence.
   * When provided, ResourceListCard saves/restores column visibility per table.
   * Recommended: route path or feature name (e.g. "users-list", "roles-list").
   */
  tableId?: string;
  /**
   * Data indexes hidden on first visit. They remain toggleable in the column
   * menu; a previously persisted choice wins over this default.
   */
  defaultHiddenColumns?: readonly string[];

  // ── Hooks (必填) — paginatedFn 或 listFn 二选一 ──
  buildQueryKey: CrudListConfig<TItem, TParams, TDeleteKey>["buildQueryKey"];
  buildParams: CrudListConfig<TItem, TParams, TDeleteKey>["buildParams"];
  paginatedFn?: CrudListConfig<TItem, TParams, TDeleteKey>["paginatedFn"];
  /** 非分页列表 API — ResourceListCard client mode，全量拉取+客户端分页+Table sort/filter */
  listFn?: CrudListConfig<TItem, TParams, TDeleteKey>["listFn"];
  /** 自动刷新间隔（毫秒），只读列表常用 */
  refetchInterval?: CrudListConfig<TItem, TParams, TDeleteKey>["refetchInterval"];

  // ── Hooks (可选) ──
  deleteFn?: CrudListConfig<TItem, TParams, TDeleteKey>["deleteFn"];
  invalidateKeysOnDelete?: CrudListConfig<TItem, TParams, TDeleteKey>["invalidateKeysOnDelete"];
  extractTotal?: CrudListConfig<TItem, TParams, TDeleteKey>["extractTotal"];
  enabled?: CrudListConfig<TItem, TParams, TDeleteKey>["enabled"];
  onDeleteSuccess?: CrudListConfig<TItem, TParams, TDeleteKey>["onDeleteSuccess"];
  onDeleteError?: CrudListConfig<TItem, TParams, TDeleteKey>["onDeleteError"];
  initialPageSize?: CrudListConfig<TItem, TParams, TDeleteKey>["initialPageSize"];

  // ── Bulk / Toolbar (可选) ──
  // 传 true 即开启复选框列 + 默认「批量删除」动作（依赖 deleteFn / bulkDeleteFn）
  enableBulkDelete?: boolean;
  /** 行是否可被选中；返回 false 的行复选框置灰（自我保护、内置对象等）。 */
  rowSelectable?: (record: TItem) => boolean;
  /** 批量删除确认弹窗的补充说明（默认只显示通用不可逆提示）。 */
  bulkDeleteDescription?: ReactNode;
  /** 默认批量动作的按钮文案；行删除实际是"标记已读/吊销"等语义时用它改名。 */
  bulkDeleteLabel?: string;
  /** 默认批量删除确认标题；默认「确定要删除选中的 N 项吗？」。 */
  bulkDeleteConfirmTitle?: (count: number) => string;
  /**
   * 批量删除动作入口的权限声明。传入时用 PermissionGuard 包裹默认批量删除
   * 按钮（含二次确认触发点）；未传时保持既有渲染，不做权限判断。
   * 复选框列不受影响：其他批量动作仍可选择行。
   */
  bulkDeletePermission?: { permission: string; resource: string };
  /** 额外的批量管理动作（启用/停用/导出等），与默认批量删除并排展示。 */
  bulkActions?: (ctx: { selectedKeys: string[]; records: TItem[] }) => ReactNode;
  toolbar?: ReactNode;

  // ── Row key 转 delete key (默认 rowKey 即 delete key) ──
  rowKeyToDeleteKey?: (record: TItem) => TDeleteKey;
};

/**
 * renderCrudListPage 内部交给 useCrudList 的字段；renderCrudListView 不消费它们。
 *
 * 页面如果需要 resetAll / optimistic update / 自定义 bulkDeleteFn，就必须自己
 * 调 useCrudList 拿到 CrudListResult，再用 renderCrudListView 把它传进来——否则
 * renderCrudListPage 会再建一个同 key 的 useQuery observer，页面壳操作的 state 也
 * 不是真正驱动渲染的那一份（reset 按钮会静默失效）。
 */
type HookOwnedKeys =
  | "buildQueryKey"
  | "buildParams"
  | "paginatedFn"
  | "deleteFn"
  | "invalidateKeysOnDelete"
  | "extractTotal"
  | "enabled"
  | "onDeleteSuccess"
  | "onDeleteError"
  | "initialPageSize";

export type CrudListViewConfig<TItem, TParams, TDeleteKey> = Omit<
  CrudListPageConfig<TItem, TParams, TDeleteKey>,
  HookOwnedKeys
>;

/**
 * renderCrudListView — 纯渲染层：吃一份外部 CrudListResult + UI config。
 * 用法见 UsersPage.tsx。
 */
export function renderCrudListView<TItem extends object, TParams, TDeleteKey>(
  config: CrudListViewConfig<TItem, TParams, TDeleteKey>,
  list: CrudListResult<TItem, TDeleteKey>,
): ReactElement {
  const rowKeyForCard = useMemo(() => {
    const rk = config.rowKey;
    return typeof rk === "function" ? (record: TItem) => String(rk(record)) : rk;
  }, [config.rowKey]);

  // client mode 不渲染 serverPagination — ResourceListCard 自动用客户端搜索+分页
  const serverPagination = list.isClientMode
    ? undefined
    : {
        page: list.pagination.page,
        pageSize: list.pagination.pageSize,
        total: list.pagination.total,
        onChange: list.serverPagination.onChange,
      };

  // client mode 的 search 是 ResourceListCard 内部做的，不需要 onSearch/onSortChange 回调
  const onSearch = list.isClientMode ? undefined : list.onSearch;
  const onSortChange = list.isClientMode ? undefined : list.onSortChange;

  // 选中集合存的是 rowKey 字符串；删除 API 可能需要别的 key（如 name/slug），
  // 由 rowKeyToDeleteKey 完成映射。未配置时按字符串 key 直接删除（既有行为）。
  const resolveDeleteKeys = (keys: string[]): TDeleteKey[] => {
    if (!config.rowKeyToDeleteKey) return keys as unknown as TDeleteKey[];
    const records = list.query.data ?? [];
    const toRowKey = (record: TItem) =>
      typeof rowKeyForCard === "function"
        ? rowKeyForCard(record)
        : String(record[rowKeyForCard as keyof TItem]);
    return keys.flatMap((key) => {
      const record = records.find((item) => toRowKey(item) === key);
      return record ? [config.rowKeyToDeleteKey!(record)] : [];
    });
  };

  const rowSelection =
    config.enableBulkDelete && (list.hasBulkDelete || Boolean(config.bulkActions))
      ? {
          selectedRowKeys: list.selectedKeys,
          onChange: list.setSelectedKeys,
          getCheckboxProps: config.rowSelectable
            ? (record: TItem) => ({ disabled: !config.rowSelectable!(record) })
            : undefined,
        }
      : undefined;

  const bulkDeleteAction = list.hasBulkDelete ? (
    <CrudListBulkDeleteAction
      selectedCount={list.selectedKeys.length}
      pending={list.bulkDeleteMutation.isPending}
      description={config.bulkDeleteDescription}
      label={config.bulkDeleteLabel}
      confirmTitle={config.bulkDeleteConfirmTitle}
      onDelete={() => list.bulkDeleteMutation.mutateAsync(resolveDeleteKeys(list.selectedKeys))}
    />
  ) : null;

  // 未配置权限时原样输出，保持既有渲染结构；配置后 denial 直接不渲染入口，
  // 而 rowSelection 仍按 enableBulkDelete 生成，复选框与其它批量动作不受影响。
  const guardedBulkDelete =
    config.bulkDeletePermission && list.hasBulkDelete ? (
      <PermissionGuard
        action={config.bulkDeletePermission.permission}
        resource={config.bulkDeletePermission.resource}
        fallback={null}
      >
        {bulkDeleteAction}
      </PermissionGuard>
    ) : (
      bulkDeleteAction
    );

  const bulkActions = rowSelection ? (
    <>
      {guardedBulkDelete}
      {config.bulkActions?.({ selectedKeys: list.selectedKeys, records: list.query.data ?? [] })}
    </>
  ) : undefined;

  const columns = typeof config.columns === "function" ? config.columns(list) : config.columns;

  return (
    <ResourceListCard<TItem>
      title={config.title}
      description={config.description}
      searchPlaceholder={config.searchPlaceholder}
      query={list.query}
      columns={columns}
      rowKey={rowKeyForCard}
      filters={config.filters}
      filterValues={list.filters}
      onFilterChange={list.onFilterChange}
      searchableColumns={config.searchableColumns}
      onSearch={onSearch}
      onSortChange={onSortChange}
      serverPagination={serverPagination}
      rowSelection={rowSelection}
      bulkActions={bulkActions}
      toolbar={config.toolbar}
      showRefresh={config.showRefresh}
      badge={config.badge}
      hideSearch={config.hideSearch}
      emptyContent={config.emptyContent}
      expandable={config.expandable}
      defaultSortField={config.defaultSortField}
      defaultSortOrder={config.defaultSortOrder}
      tableId={config.tableId}
      defaultHiddenColumns={config.defaultHiddenColumns}
    />
  );
}

/**
 * CrudListBulkDeleteAction — 共享层的默认「批量删除」按钮。
 *
 * 确认 → 调用批量删除 mutation → 按成功/失败汇总提示。失败行保留选中，
 * 由 useCrudList 的 onSuccess 处理。选择计数与"取消选择"由 BatchActionBar 负责。
 */
type CrudListBulkDeleteActionProps = {
  selectedCount: number;
  pending: boolean;
  description?: ReactNode;
  label?: string;
  confirmTitle?: (count: number) => string;
  onDelete: () => Promise<unknown>;
};

// oxlint-disable-next-line react/only-export-components -- 纯渲染助手模块，不经 Fast Refresh 热更新
function CrudListBulkDeleteAction({
  selectedCount,
  pending,
  description,
  label,
  confirmTitle,
  onDelete,
}: CrudListBulkDeleteActionProps) {
  const { t } = useTranslation("common");
  const { message } = App.useApp();

  if (selectedCount === 0) return null;

  return (
    <PopconfirmModal
      title={
        confirmTitle
          ? confirmTitle(selectedCount)
          : t("confirmBulkDelete", { count: selectedCount })
      }
      description={description}
      text={label ?? t("batchDelete")}
      size="small"
      disabled={pending}
      onConfirm={async () => {
        let result: unknown;
        try {
          result = await onDelete();
        } catch {
          // mutation 的 onError 已给出具体错误提示，这里只负责不发重复 toast
          return;
        }
        const summary = result as BulkDeleteSummary | undefined;
        if (!summary) {
          message.success(t("deleteSuccess"));
          return;
        }
        if (summary.failed > 0) {
          message.warning(
            t("bulkDeletePartial", {
              succeeded: summary.succeeded,
              failed: summary.failed,
              total: summary.total,
            }),
          );
        } else {
          message.success(t("bulkDeleteSuccess", { count: summary.succeeded }));
        }
      }}
    />
  );
}

/**
 * renderCrudListPage — 组件级封装。
 * 页面传入 config，内部调 useCrudList + renderCrudListView。
 *
 * 用法见 OrganizationsPage.tsx（config 直套）。
 */
export function renderCrudListPage<TItem extends object, TParams, TDeleteKey>(
  config: CrudListPageConfig<TItem, TParams, TDeleteKey>,
): ReactElement {
  const list = useCrudList<TItem, TParams, TDeleteKey>({
    buildQueryKey: config.buildQueryKey,
    buildParams: config.buildParams,
    paginatedFn: config.paginatedFn,
    listFn: config.listFn,
    refetchInterval: config.refetchInterval,
    deleteFn: config.deleteFn,
    invalidateKeysOnDelete: config.invalidateKeysOnDelete,
    extractTotal: config.extractTotal,
    enabled: config.enabled,
    onDeleteSuccess: config.onDeleteSuccess,
    onDeleteError: config.onDeleteError,
    initialPageSize: config.initialPageSize,
    defaultSortField: config.defaultSortField,
    defaultSortOrder: config.defaultSortOrder,
  });

  return renderCrudListView<TItem, TParams, TDeleteKey>(config, list);
}
