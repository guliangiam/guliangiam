import { Space, Tag, Tooltip } from "antd";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import {
  AI_GATEWAY_DEFAULT_PAGE_SIZE,
  listUsageLogs,
  type ListPageParams,
  type UsageLogResponse,
} from "@/shared/api/aigateway";
import { useCrudList } from "@/shared/hooks/useCrudList";
import { renderCrudListView } from "@/shared/ui/CrudListPage";

const STATUS_COLORS: Record<string, string> = {
  succeeded: "green",
  failed: "red",
  degraded: "orange",
  throttled: "gold",
};

const PROTOCOL_LABELS: Record<string, string> = {
  openai_compatible: "OpenAI",
  anthropic_messages: "Anthropic",
  gemini_generate_content: "Gemini",
};

export default function AIGatewayUsagePage() {
  const { t: tc } = useTranslation("common");
  const account = useSessionStore((state) => state.account);

  // ── server 分页 + 30s 自动刷新 ──
  const list = useCrudList<UsageLogResponse, ListPageParams, string>({
    buildQueryKey: (params) => ["aigateway", "usage", params.page, params.pageSize] as const,
    buildParams: ({ page, pageSize }) => ({ page, pageSize }),
    paginatedFn: async (params) => {
      const response = await listUsageLogs(params);
      return { items: response?.items ?? [], total: response?.page.total ?? 0 };
    },
    initialPageSize: AI_GATEWAY_DEFAULT_PAGE_SIZE,
    refetchInterval: 30_000,
    enabled: account !== null,
  });

  return renderCrudListView(
    {
      title: tc("nav.aiGateway.usage", "Usage Logs"),
      description: tc("aigateway.usageDescription"),
      rowKey: "id",
      columns: [
        {
          title: tc("aigateway.alias"),
          dataIndex: "aliasId",
          key: "aliasId",
          width: 180,
          fixed: "left",
          ellipsis: true,
          render: (id: string, record) => (
            <Tooltip
              title={`alias: ${id}\nmodel: ${record.modelId}\nprovider: ${record.providerId}`}
            >
              <code style={{ fontSize: 12 }}>{id.slice(0, 12)}…</code>
            </Tooltip>
          ),
        },
        {
          title: tc("aigateway.protocolFace"),
          dataIndex: "protocolFace",
          key: "protocolFace",
          width: 140,
          render: (p: string) => <Tag color="blue">{PROTOCOL_LABELS[p] ?? p}</Tag>,
        },
        {
          title: tc("aigateway.tokens"),
          key: "tokens",
          width: 260,
          render: (_, record) => (
            <Space size={12} style={{ fontSize: 12 }}>
              <span>
                <span style={{ color: "var(--gl-color-text-tertiary)" }}>in </span>
                <strong>{record.inputTokens.toLocaleString()}</strong>
              </span>
              <span>
                <span style={{ color: "var(--gl-color-text-tertiary)" }}>out </span>
                <strong>{record.outputTokens.toLocaleString()}</strong>
              </span>
              <span>
                <span style={{ color: "var(--gl-color-text-tertiary)" }}>total </span>
                <strong>{record.totalTokens.toLocaleString()}</strong>
              </span>
            </Space>
          ),
        },
        {
          title: tc("aigateway.cost"),
          dataIndex: "costAmount",
          key: "costAmount",
          width: 120,
          align: "right",
          render: (v: number, record) =>
            v > 0 ? `${record.currency ?? "USD"} ${v.toFixed(4)}` : "—",
        },
        {
          title: tc("aigateway.latency"),
          key: "latency",
          width: 160,
          align: "right",
          render: (_, record) => (
            <Space size={12} style={{ fontSize: 12 }}>
              {record.ttftMs !== null && record.ttftMs !== undefined && (
                <span>TTFT {record.ttftMs}ms</span>
              )}
              {record.totalLatencyMs !== null && record.totalLatencyMs !== undefined && (
                <span>Total {record.totalLatencyMs}ms</span>
              )}
            </Space>
          ),
        },
        {
          title: tc("aigateway.status"),
          dataIndex: "status",
          key: "status",
          width: 100,
          render: (s: string) => <Tag color={STATUS_COLORS[s] ?? "default"}>{s}</Tag>,
        },
        {
          title: tc("aigateway.attempt"),
          dataIndex: "attemptNo",
          key: "attemptNo",
          width: 80,
          align: "right",
        },
        {
          title: tc("aigateway.createdAt"),
          dataIndex: "createdAt",
          key: "createdAt",
          width: 180,
          render: (v: string) => new Date(v).toLocaleString(),
        },
      ],
    },
    list,
  );
}
