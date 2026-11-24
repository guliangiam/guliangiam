import { App } from "antd";
import { t } from "i18next";
import { useCallback, useState } from "react";
import { useDebounce } from "./useDebounce";
import { serverErrorMessage } from "@/shared/lib/server-error";
import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationOptions,
  type UseQueryResult,
} from "@tanstack/react-query";

// ─────────────────────────────────────────────────────────────────────
// useCrudList — CRUD 列表页 boilerplate 消除 hook
//
// 封装的职责：
//   · pagination state + search state + filter state
//   · useQuery + queryKey 构造
//   · deleteMutation + bulkDeleteMutation
//   · onSearch / onFilterChange / onPaginationChange / onSortChange handlers
//
// 留给页面 config 的部分：
//   · queryKey 构造器（依赖页面的具体业务 key）
//   · paginated API 调用（不同页面函数签名不同）
//   · delete API（不同页面 delete 的参数不同：id / name / key）
//   · 初始 filters / enabled 条件
//   · 成功消息、invalidate 键
//
// 设计原则：hook 提供默认实现，页面通过 config 覆盖。
//          hook 返回的 query/pagination/search/handlers 直接喂给 ResourceListCard。
// ─────────────────────────────────────────────────────────────────────

// Page 传入的分页查询函数签名（通用）
export type PaginatedFn<TItem, TParams> = (
  params: TParams,
) => Promise<{ items: TItem[]; total: number }>;

/**
 * 批量删除结果。默认实现逐条调用 deleteFn，用 allSettled 汇总成功/失败，
 * 单条失败不会中断其余行，也不会把成功的部分回滚成"全部失败"。
 */
export type BulkDeleteSummary = {
  total: number;
  succeeded: number;
  failed: number;
  /** 删除失败的行 key —— 保留在选中集合里，便于用户重试。 */
  failedKeys: string[];
};

/** 批量删除的并发批次大小：限制同时在途的 DELETE 请求数。 */
const BULK_DELETE_BATCH_SIZE = 10;

// 基础分页参数（多数页面通用）
export type BasicListParams = {
  page: number;
  pageSize: number;
  search?: string;
  filters?: Record<string, string>;
};

export type CrudListConfig<TItem, TParams, TDeleteKey> = {
  // ── 数据源（二选一） ──
  // queryKey 构造器，接收当前 search/pagination/filters，返回 query key 数组
  buildQueryKey: (p: TParams) => readonly unknown[];
  // 分页查询 API（返回 { items, total }）—— server mode
  paginatedFn?: PaginatedFn<TItem, TParams>;
  // 非分页查询 API（返回 TItem[] 全量数组）—— client mode
  // ResourceListCard 不传 serverPagination 时自动用客户端搜索+分页+Table sort/filter
  listFn?: (params: TParams) => Promise<TItem[]>;
  // 自动刷新间隔（毫秒）。只读列表常用（如 AI-gateway health 30s 刷新）
  refetchInterval?: number;
  // 从 query 结果中取 total 的回调（有的 API 直接返回，有的要从 headers）
  extractTotal?: (result: { items: TItem[]; total: number }) => number;
  // query enabled 条件（依赖 session 是否就绪等）
  enabled?: boolean;

  // ── 参数构造 ──
  // 把 hook 维护的 state 转换成 API 参数
  buildParams: (p: {
    page: number;
    pageSize: number;
    search: string;
    filters: Record<string, string>;
    sortField?: string;
    sortOrder?: "ascend" | "descend";
  }) => TParams;
  // 初始参数（例如 organizationId 来自 session）
  extraParams?: () => Partial<TParams>;

  // ── 删除 ──
  // 单个删除 API
  deleteFn?: (key: TDeleteKey) => Promise<unknown>;
  // 删除成功后 invalidate 的 query key（数组）。
  // 内层为 readonly：TanStack Query 的 queryKey 本身就是 readonly 元组
  // （`queryKeys.*.all` 由 `as const` 产出），可变类型会把它们全部拒之门外。
  invalidateKeysOnDelete?: readonly (readonly unknown[])[];
  // 单个删除 mutation options（可覆盖 onError 等）
  deleteMutationOptions?: UseMutationOptions<unknown, Error, TDeleteKey>;
  // 批量删除是否用 Promise.all（默认 true），false 则调 deleteFn 逐个
  bulkDeleteSerial?: boolean;
  // 完全自定义批量删除实现（覆盖 bulkDeleteSerial / deleteFn 默认行为）
  // 用于需要 BATCH_SIZE + Promise.allSettled + 进度回调的场景
  bulkDeleteFn?: (keys: TDeleteKey[]) => Promise<unknown>;
  // 批量删除进度回调（配合 bulkDeleteFn 自定义实现使用）
  // called with (done, total) after each batch
  onBulkDeleteProgress?: (done: number, total: number) => void;

  // ── UI 反馈 ──
  // 删除成功 toast（通常 message.success(t("deleteSuccess"))）
  onDeleteSuccess?: () => void;
  onDeleteError?: (err: Error) => void;

  // ── 初始 state ──
  initialPageSize?: number;
  /** 初始筛选值（例如从 URL ?category= 深链预置）。 */
  initialFilters?: Record<string, string>;
  defaultSortField?: string;
  defaultSortOrder?: "ascend" | "descend";
};

export type CrudListResult<TItem, TDeleteKey> = {
  query: UseQueryResult<TItem[], Error>;
  pagination: { page: number; pageSize: number; total: number };
  search: string;
  filters: Record<string, string>;
  sortField?: string;
  sortOrder?: "ascend" | "descend";
  selectedKeys: string[];
  setSelectedKeys: (keys: string[]) => void;
  setSearch: (v: string) => void;
  setFilters: (v: Record<string, string>) => void;
  setSortField: (v?: string) => void;
  setSortOrder: (v?: "ascend" | "descend") => void;
  deleteMutation: ReturnType<typeof useMutation<unknown, Error, TDeleteKey>>;
  bulkDeleteMutation: ReturnType<typeof useMutation<unknown, Error, TDeleteKey[]>>;
  /** true 表示列表具备批量删除能力（deleteFn 或 bulkDeleteFn 至少一个）。 */
  hasBulkDelete: boolean;
  /** true 表示非分页 listFn 模式——ResourceListCard 应传 client mode（无 serverPagination） */
  isClientMode: boolean;
  // serverPagination 只在分页模式下有效
  serverPagination: {
    page: number;
    pageSize: number;
    total: number;
    onChange: (page: number, pageSize: number) => void;
  };
  onSearch: (value: string) => void;
  onFilterChange: (filters: Record<string, string>) => void;
  onSortChange: (field: string | undefined, order: "ascend" | "descend" | null) => void;
  refetch: () => void;
};

export function useCrudList<TItem, TParams, TDeleteKey>(
  config: CrudListConfig<TItem, TParams, TDeleteKey>,
): CrudListResult<TItem, TDeleteKey> {
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const initialPageSize = config.initialPageSize ?? 50;
  const defaultSortField = config.defaultSortField;
  const defaultSortOrder = config.defaultSortOrder ?? "descend";

  const [pagination, setPagination] = useState({ page: 1, pageSize: initialPageSize, total: 0 });
  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<Record<string, string>>(config.initialFilters ?? {});
  const [selectedKeys, setSelectedKeys] = useState<string[]>([]);
  const [sortField, setSortField] = useState<string | undefined>(defaultSortField);
  const [sortOrder, setSortOrder] = useState<"ascend" | "descend" | undefined>(
    defaultSortField ? defaultSortOrder : undefined,
  );

  // Debounce only the request-side search value: the input stays instant while
  // server mode no longer fires one request per keystroke (local mode filters
  // its loaded array in ResourceListCard and is unaffected).
  const debouncedSearch = useDebounce(search, 300);

  // ── Query ──
  const params = config.buildParams({
    page: pagination.page,
    pageSize: pagination.pageSize,
    search: debouncedSearch,
    filters,
    sortField,
    sortOrder,
  });

  // client mode = 非分页 listFn；server mode = paginatedFn
  const isClientMode = !!config.listFn;

  const query = useQuery({
    queryKey: config.buildQueryKey(params),
    queryFn: async () => {
      if (isClientMode) {
        // listFn 返回全量 TItem[]，total = 数组长度
        const items = await config.listFn!(params);
        return { items, total: items.length };
      }
      if (!config.paginatedFn) throw new Error("paginatedFn or listFn must be provided");
      const result = await config.paginatedFn(params);
      const total = config.extractTotal ? config.extractTotal(result) : result.total;
      return { items: result.items, total };
    },
    enabled: config.enabled !== false,
    refetchInterval: config.refetchInterval,
  });

  // total is derived from the query result instead of mirrored into local state:
  // a setState-in-effect for pagination caused an extra render on every fetch.
  const total = query.data?.total ?? 0;

  // Shape query for ResourceListCard consumption — expects data = TItem[] not { items, total }
  const listQuery = query.data
    ? ({ ...query, data: query.data.items } as unknown as UseQueryResult<TItem[], Error>)
    : (query as unknown as UseQueryResult<TItem[], Error>);

  // ── Delete mutations ──
  const invalidateOnDelete = useCallback(() => {
    if (config.invalidateKeysOnDelete) {
      for (const key of config.invalidateKeysOnDelete) {
        queryClient.invalidateQueries({ queryKey: key });
      }
    }
  }, [config.invalidateKeysOnDelete, queryClient]);

  const defaultOnDeleteError = (err: unknown) =>
    message.error(serverErrorMessage(err, t, "common:deleteFailed"));

  const deleteMutation = useMutation({
    mutationFn: async (key: TDeleteKey) => {
      if (!config.deleteFn) throw new Error("deleteFn not provided");
      return config.deleteFn(key);
    },
    onSuccess: () => {
      invalidateOnDelete();
      config.onDeleteSuccess?.();
    },
    onError: (err) => {
      // 优先用页面传入的 onDeleteError，否则用默认兜底 toast
      if (config.onDeleteError) {
        config.onDeleteError(err);
      } else if (!config.deleteMutationOptions?.onError) {
        defaultOnDeleteError(err);
      }
    },
    ...config.deleteMutationOptions,
  });

  const bulkDeleteMutation = useMutation({
    mutationFn: async (keys: TDeleteKey[]) => {
      // 1) 完全自定义实现（覆盖一切）
      if (config.bulkDeleteFn) {
        return config.bulkDeleteFn(keys);
      }
      if (!config.deleteFn) throw new Error("deleteFn or bulkDeleteFn not provided");
      // 2) 默认：分批并发（限制在途请求数），allSettled 汇总，单条失败不阻断其余行
      let succeeded = 0;
      const failedKeys: string[] = [];
      for (let i = 0; i < keys.length; i += BULK_DELETE_BATCH_SIZE) {
        const batch = keys.slice(i, i + BULK_DELETE_BATCH_SIZE);
        const results = await Promise.allSettled(batch.map((key) => config.deleteFn!(key)));
        results.forEach((result, index) => {
          if (result.status === "fulfilled") {
            succeeded += 1;
          } else {
            // failedKeys 回填 selectedKeys（行 id 字符串）；复合删除 key 须取 id，
            // 直接 String(对象) 会得到 "[object Object]" 导致失败行无法保留选中。
            const key = batch[index];
            failedKeys.push(
              typeof key === "object" && key !== null && "id" in key
                ? String((key as { id: unknown }).id)
                : String(key),
            );
          }
        });
        if (config.onBulkDeleteProgress) {
          config.onBulkDeleteProgress(
            Math.min(i + BULK_DELETE_BATCH_SIZE, keys.length),
            keys.length,
          );
        }
      }
      const summary: BulkDeleteSummary = {
        total: keys.length,
        succeeded,
        failed: failedKeys.length,
        failedKeys,
      };
      return summary;
    },
    onSuccess: (data) => {
      invalidateOnDelete();
      const summary = data as BulkDeleteSummary | undefined;
      const failedKeys = summary?.failedKeys ?? [];
      // 失败行保留选中，便于用户直接重试；全部成功才清空。
      setSelectedKeys(failedKeys);
    },
    onError: (err) => {
      if (config.onDeleteError) {
        config.onDeleteError(err);
      } else {
        defaultOnDeleteError(err);
      }
    },
  });

  // ── Handlers ──
  // 选择是"当前视图内"的：换页/搜索/筛选/排序后旧 key 可能已不在结果集里，
  // 继续携带会让批量动作作用到用户看不见的行，因此统一清空。
  const handleSearch = useCallback((value: string) => {
    setSearch(value);
    setSelectedKeys([]);
    setPagination((prev) => ({ ...prev, page: 1 }));
  }, []);

  const handleFilterChange = useCallback((next: Record<string, string>) => {
    setFilters(next);
    setSelectedKeys([]);
    setPagination((prev) => ({ ...prev, page: 1 }));
  }, []);

  const handlePaginationChange = useCallback((page: number, pageSize: number) => {
    setSelectedKeys([]);
    setPagination((prev) => ({ ...prev, page, pageSize }));
  }, []);

  const handleSortChange = useCallback(
    (field: string | undefined, order: "ascend" | "descend" | null) => {
      setSortField(field);
      setSortOrder(order ?? undefined);
      setSelectedKeys([]);
      setPagination((prev) => ({ ...prev, page: 1 }));
    },
    [],
  );

  return {
    query: listQuery,
    pagination: { ...pagination, total },
    search,
    filters,
    sortField,
    sortOrder,
    selectedKeys,
    setSelectedKeys,
    setSearch,
    setFilters,
    setSortField,
    setSortOrder,
    deleteMutation,
    bulkDeleteMutation,
    hasBulkDelete: Boolean(config.deleteFn || config.bulkDeleteFn),
    isClientMode,
    serverPagination: {
      page: pagination.page,
      pageSize: pagination.pageSize,
      total,
      onChange: handlePaginationChange,
    },
    onSearch: handleSearch,
    onFilterChange: handleFilterChange,
    onSortChange: handleSortChange,
    // invalidateQueries 触发所有订阅同 queryKey 的 observer 重新 fetch，
    // 而 query.refetch() 只刷新当前 observer（会漏掉 ProviderDetailPage 等跨页面订阅）
    refetch: () => queryClient.invalidateQueries({ queryKey: config.buildQueryKey(params) }),
  };
}
