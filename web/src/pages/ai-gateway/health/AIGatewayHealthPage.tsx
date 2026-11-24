import { Tooltip, Tag } from "antd";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import {
  AI_GATEWAY_DEFAULT_PAGE_SIZE,
  listProviderHealth,
  type ListPageParams,
  type ProviderHealthResponse,
} from "@/shared/api/aigateway";
import { useCrudList } from "@/shared/hooks/useCrudList";
import { renderCrudListView } from "@/shared/ui/CrudListPage";

const STATE_COLORS: Record<string, string> = {
  healthy: "green",
  degraded: "orange",
  open: "red",
  half_open: "gold",
  closed: "default",
};

const STATE_LABELS: Record<string, string> = {
  healthy: "Healthy",
  degraded: "Degraded",
  open: "Open (circuit)",
  half_open: "Half-open",
  closed: "Closed",
};

export default function AIGatewayHealthPage() {
  const { t: tc } = useTranslation("common");
  const account = useSessionStore((state) => state.account);

  // ── server 分页 + 30s 自动刷新 ──
  // 只读列表：熔断器状态由 GatewayService.ChatStream + HealthProbeWorker 自动写入
  const list = useCrudList<ProviderHealthResponse, ListPageParams, string>({
    buildQueryKey: (params) => ["aigateway", "health", params.page, params.pageSize] as const,
    buildParams: ({ page, pageSize }) => ({ page, pageSize }),
    paginatedFn: async (params) => {
      const response = await listProviderHealth(params);
      return { items: response?.items ?? [], total: response?.page.total ?? 0 };
    },
    initialPageSize: AI_GATEWAY_DEFAULT_PAGE_SIZE,
    refetchInterval: 30_000,
    enabled: account !== null,
  });

  return renderCrudListView(
    {
      title: tc("nav.aiGateway.health", "Provider Health"),
      description: tc(
        "aigateway.healthDescription",
        "Read-only circuit-breaker state per provider. Auto-refreshes every 30s. Healthy = green, Open = red (blocked).",
      ),
      rowKey: "id",
      columns: [
        {
          title: tc("aigateway.provider", "Provider"),
          dataIndex: "providerId",
          key: "providerId",
          width: 220,
          ellipsis: true,
          render: (id: string) => (
            <Tooltip title={id}>
              <code style={{ fontSize: 12 }}>{id.slice(0, 16)}…</code>
            </Tooltip>
          ),
        },
        {
          title: tc("aigateway.state", "Circuit State"),
          dataIndex: "state",
          key: "state",
          width: 140,
          render: (s: string) => (
            <Tag color={STATE_COLORS[s] ?? "default"}>{STATE_LABELS[s] ?? s}</Tag>
          ),
        },
        {
          title: tc("aigateway.consecutiveFailures", "Failures"),
          dataIndex: "consecutiveFailures",
          key: "consecutiveFailures",
          width: 100,
          align: "right",
          sorter: (a, b) => a.consecutiveFailures - b.consecutiveFailures,
          render: (v: number) =>
            v > 0 ? (
              <span style={{ color: v >= 5 ? "var(--gl-color-error-text)" : undefined }}>{v}</span>
            ) : (
              "0"
            ),
        },
        {
          title: tc("aigateway.openUntil", "Open Until"),
          dataIndex: "openUntil",
          key: "openUntil",
          width: 180,
          render: (v: string | null | undefined) => (v ? new Date(v).toLocaleString() : "—"),
        },
        {
          title: tc("aigateway.lastLatency", "Last Latency"),
          dataIndex: "lastLatencyMs",
          key: "lastLatencyMs",
          width: 140,
          align: "right",
          sorter: (a, b) => (a.lastLatencyMs ?? 0) - (b.lastLatencyMs ?? 0),
          render: (v: number | null | undefined) =>
            v !== null && v !== undefined ? `${v}ms` : "—",
        },
        {
          title: tc("aigateway.lastCheckedAt", "Last Check"),
          dataIndex: "lastCheckedAt",
          key: "lastCheckedAt",
          width: 180,
          render: (v: string | null | undefined) => (v ? new Date(v).toLocaleString() : "—"),
        },
        {
          title: tc("aigateway.updatedAt", "Updated"),
          dataIndex: "updatedAt",
          key: "updatedAt",
          width: 180,
          render: (v: string) => new Date(v).toLocaleString(),
        },
      ],
    },
    list,
  );
}
