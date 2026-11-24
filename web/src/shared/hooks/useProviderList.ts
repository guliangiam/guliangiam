import { useState, useCallback } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import type { UseQueryResult } from "@tanstack/react-query";
import { App } from "antd";
import { useTranslation } from "react-i18next";
import { getProvidersPaginated, deleteProvider } from "@/shared/api/iam/provider-extended";
import { useSessionStore } from "@/features/session/session-store";
import { localizedError } from "@/shared/lib/server-error";

// ─────────────────────────────────────────────
// Hook: useProviderList
// ─────────────────────────────────────────────

export type UseProviderListOptions = {
  /** Initial organization filter — defaults to session org */
  initialOrgId?: string;
};

export type UseProviderListReturn = {
  /** Paginated provider list query */
  query: ReturnType<
    typeof useQuery<Awaited<ReturnType<typeof getProvidersPaginated>>["items"], Error>
  >;
  /** Current search text */
  search: string;
  /** Set search text (resets page to 1) */
  setSearch: (value: string) => void;
  /** Current organization filter */
  orgFilter: string;
  /** Set organization filter (resets page to 1) */
  setOrgFilter: (value: string) => void;
  /** Current active filters (category, type, etc.) */
  activeFilters: Record<string, string>;
  /** Set active filters (resets page to 1) */
  setActiveFilters: (filters: Record<string, string>) => void;
  /** Pagination state */
  pagination: { page: number; pageSize: number; total: number };
  /** Set pagination (page + pageSize) */
  setPagination: (
    updater: (prev: { page: number; pageSize: number; total: number }) => {
      page: number;
      pageSize: number;
      total: number;
    },
  ) => void;
  /** Delete single provider mutation */
  deleteMutation: ReturnType<typeof useMutation<void, Error, string>>;
  /** Bulk delete providers mutation */
  bulkDeleteMutation: ReturnType<typeof useMutation<void, Error, string[]>>;
  /** Selected row keys */
  selectedRowKeys: string[];
  /** Set selected row keys */
  setSelectedRowKeys: (keys: string[]) => void;
};

/**
 * Hook that manages provider list page state: paginated fetching,
 * search, filtering, organization switching, and delete mutations.
 *
 * Encapsulates all React Query logic and state that was previously
 * inline in ProvidersPage.
 */
export function useProviderList(options?: UseProviderListOptions): UseProviderListReturn {
  const { t } = useTranslation("common");
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const organization = useSessionStore((state) => state.organization);

  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);
  const [orgFilter, setOrgFilterInternal] = useState(
    options?.initialOrgId ?? organization?.id ?? "",
  );
  const [pagination, setPagination] = useState({
    page: 1,
    pageSize: 50,
    total: 0,
  });
  const [search, setSearchInternal] = useState("");
  const [activeFilters, setActiveFiltersInternal] = useState<Record<string, string>>({});

  // ── Filter setters that reset page to 1 ──
  const setSearch = useCallback((value: string) => {
    setSearchInternal(value);
    setPagination((prev) => ({ ...prev, page: 1 }));
  }, []);

  const setOrgFilter = useCallback((value: string) => {
    setOrgFilterInternal(value);
    setPagination((prev) => ({ ...prev, page: 1 }));
  }, []);

  const setActiveFilters = useCallback((filters: Record<string, string>) => {
    setActiveFiltersInternal(filters);
    setPagination((prev) => ({ ...prev, page: 1 }));
  }, []);

  // ── Paginated query ──
  const query = useQuery({
    queryKey: [
      "providers",
      "paginated",
      orgFilter,
      pagination.page,
      pagination.pageSize,
      search,
      activeFilters,
    ],
    queryFn: async () => {
      const result = await getProvidersPaginated({
        page: pagination.page,
        pageSize: pagination.pageSize,
        search: search || undefined,
        organizationId: orgFilter || undefined,
        category: activeFilters.category || undefined,
        providerType: activeFilters.type || undefined,
      });
      return { items: result.items, total: result.total };
    },
    enabled: organization !== null,
  });

  // total is derived from the query result in the return value; no mirrored state.

  // Shape query.data from { items, total } → items[] for consumers
  const listQuery = { ...query, data: query.data?.items } as UseQueryResult<
    Awaited<ReturnType<typeof getProvidersPaginated>>["items"],
    Error
  >;

  // ── Delete mutation ──
  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteProvider(id),
    onSuccess: () => {
      message.success(t("deleted"));
      setSelectedRowKeys((prev) => prev.filter((k) => k !== deleteMutation.variables));
      queryClient.invalidateQueries({ queryKey: ["providers"] });
    },
    onError: (error) => {
      message.error(localizedError(error, t, "deleteFailed"));
    },
  });

  // ── Bulk delete mutation ──
  const bulkDeleteMutation = useMutation({
    mutationFn: async (ids: string[]) => {
      await Promise.all(ids.map((id) => deleteProvider(id)));
    },
    onSuccess: () => {
      message.success(t("bulkDeleted"));
      setSelectedRowKeys([]);
      queryClient.invalidateQueries({ queryKey: ["providers"] });
    },
    onError: (error) => {
      message.error(localizedError(error, t, "deleteFailed"));
      queryClient.invalidateQueries({ queryKey: ["providers"] });
    },
  });

  return {
    query: listQuery,
    search,
    setSearch,
    orgFilter,
    setOrgFilter,
    activeFilters,
    setActiveFilters,
    pagination: { ...pagination, total: query.data?.total ?? 0 },
    setPagination,
    deleteMutation,
    bulkDeleteMutation,
    selectedRowKeys,
    setSelectedRowKeys,
  };
}
