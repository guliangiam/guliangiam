import { Card, Col, Row, Select, Space, Table, Tag, Tooltip, Typography } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import {
  ArrowUpOutlined,
  ArrowDownOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  CloudServerOutlined,
  ThunderboltOutlined,
  DollarOutlined,
  ClockCircleOutlined,
} from "@ant-design/icons";
import { useTranslation } from "react-i18next";

import {
  fetchUsageStats,
  type UsageStats,
  type ProviderUsageBreakdown,
  type DailyUsageBreakdown,
  type ProtocolUsageBreakdown,
} from "@/shared/api/aigateway";

const { Title, Text } = Typography;

const FACE_LABELS: Record<string, string> = {
  openai_compatible: "OpenAI Compatible",
  anthropic_messages: "Anthropic Messages",
  gemini_generate_content: "Gemini generateContent",
  anthropic: "Anthropic",
  gemini: "Gemini",
  openai: "OpenAI",
};

function faceLabel(face: string): string {
  return FACE_LABELS[face] ?? face;
}

function formatUSD(v: number): string {
  if (v < 0.01) return "$" + v.toFixed(4);
  if (v < 1) return "$" + v.toFixed(3);
  return "$" + v.toFixed(2);
}

function formatInt(v: number): string {
  return v.toLocaleString();
}

// =============================================================================
// Main Page
// =============================================================================

export default function AIGatewayUsageStatsPage() {
  const { t } = useTranslation();
  const [days, setDays] = useState<number>(7);

  const { data, isLoading } = useQuery<UsageStats | undefined>({
    queryKey: ["aigateway", "usage-stats", days],
    queryFn: () => fetchUsageStats(days),
    staleTime: 30_000,
  });

  const stats = data;

  const daysOptions = [
    { label: t("aigateway.usageStats.today"), value: 1 },
    { label: t("aigateway.usageStats.last7Days"), value: 7 },
    { label: t("aigateway.usageStats.last30Days"), value: 30 },
    { label: t("aigateway.usageStats.allTime"), value: 0 },
  ];

  const successRate = useMemo(() => {
    if (!stats || stats.totalRequests === 0) return null;
    return (stats.succeededCount / stats.totalRequests) * 100;
  }, [stats]);

  return (
    <div style={{ padding: 24 }}>
      {/* Header + Window Selector */}
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          marginBottom: 24,
        }}
      >
        <Title level={3} style={{ margin: 0 }}>
          <ThunderboltOutlined style={{ marginRight: 8, color: "var(--gl-color-primary)" }} />
          {t("aigateway.usageStats.title")}
        </Title>
        <Space>
          <Text type="secondary">{t("aigateway.usageStats.window")}:</Text>
          <Select
            value={days}
            onChange={setDays}
            aria-label={t("aigateway.usageStats.window")}
            options={daysOptions}
            style={{ minWidth: 140 }}
            disabled={isLoading}
          />
        </Space>
      </div>

      {/* ── KPI Row ────────────────────────────────────────────────── */}
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <KpiCard
          title={t("aigateway.usageStats.totalRequests")}
          value={stats ? formatInt(stats.totalRequests) : "—"}
          icon={<CloudServerOutlined />}
          color="var(--gl-color-primary)"
          loading={isLoading}
        />
        <KpiCard
          title={t("aigateway.usageStats.successRate")}
          value={successRate !== null ? successRate.toFixed(1) + "%" : "—"}
          icon={
            successRate !== null && successRate >= 95 ? (
              <CheckCircleOutlined />
            ) : successRate !== null && successRate < 80 ? (
              <CloseCircleOutlined />
            ) : (
              <CheckCircleOutlined />
            )
          }
          color={
            successRate === null
              ? "var(--gl-color-text-tertiary)"
              : successRate >= 95
                ? "var(--gl-color-success)"
                : successRate < 80
                  ? "var(--gl-color-error)"
                  : "var(--gl-color-warning)"
          }
          subtitle={
            stats
              ? `${formatInt(stats.succeededCount)} / ${formatInt(stats.totalRequests)}`
              : undefined
          }
          loading={isLoading}
        />
        <KpiCard
          title={t("aigateway.usageStats.totalTokens")}
          value={stats ? formatInt(stats.totalTokens) : "—"}
          icon={<ThunderboltOutlined />}
          color="#722ed1"
          subtitle={
            stats
              ? t("aigateway.usageStats.inOut", {
                  input: formatInt(stats.totalInputTokens),
                  output: formatInt(stats.totalOutputTokens),
                })
              : undefined
          }
          loading={isLoading}
        />
        <KpiCard
          title={t("aigateway.usageStats.totalCost")}
          value={stats ? formatUSD(stats.totalCostUsd) : "—"}
          icon={<DollarOutlined />}
          color="#13c2c2"
          loading={isLoading}
        />
        <KpiCard
          title={t("aigateway.usageStats.avgTtft")}
          value={
            stats && stats.avgTtftMs !== null && stats.avgTtftMs !== undefined
              ? `${stats.avgTtftMs.toFixed(0)} ms`
              : "—"
          }
          icon={<ClockCircleOutlined />}
          color="#eb2f96"
          loading={isLoading}
        />
        <KpiCard
          title={t("aigateway.usageStats.avgLatency")}
          value={
            stats && stats.avgLatencyMs !== null && stats.avgLatencyMs !== undefined
              ? `${stats.avgLatencyMs.toFixed(0)} ms`
              : "—"
          }
          icon={<ClockCircleOutlined />}
          color="#fa541c"
          loading={isLoading}
        />
      </Row>

      {/* ── Breakdowns Row ────────────────────────────────────────── */}
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={12}>
          <BreakdownCard
            title={t("aigateway.usageStats.byProvider")}
            data={stats?.byProvider ?? []}
            rowKey={(record) => record.providerId}
            columns={[
              {
                title: t("aigateway.usageStats.provider"),
                dataIndex: "providerId",
                key: "providerId",
                ellipsis: true,
                render: (id: string) => (
                  <Tooltip title={id}>
                    <code style={{ fontSize: 12 }}>{id.slice(0, 8)}…</code>
                  </Tooltip>
                ),
              },
              {
                title: t("aigateway.usageStats.requests"),
                dataIndex: "requests",
                key: "requests",
                render: (v: number) => formatInt(v),
              },
              {
                title: t("aigateway.usageStats.tokens"),
                dataIndex: "tokens",
                key: "tokens",
                render: (v: number) => formatInt(v),
              },
              {
                title: t("aigateway.usageStats.cost"),
                dataIndex: "costUsd",
                key: "costUsd",
                render: (v: number) => formatUSD(v),
              },
            ]}
            loading={isLoading}
          />
        </Col>

        <Col xs={24} lg={12}>
          <BreakdownCard
            title={t("aigateway.usageStats.byProtocol")}
            data={stats?.byProtocol ?? []}
            rowKey={(record) => record.protocolFace}
            columns={[
              {
                title: t("aigateway.usageStats.protocol"),
                dataIndex: "protocolFace",
                key: "protocolFace",
                render: (face: string) => <Tag>{faceLabel(face)}</Tag>,
              },
              {
                title: t("aigateway.usageStats.requests"),
                dataIndex: "requests",
                key: "requests",
                render: (v: number) => formatInt(v),
              },
              {
                title: t("aigateway.usageStats.tokens"),
                dataIndex: "tokens",
                key: "tokens",
                render: (v: number) => formatInt(v),
              },
            ]}
            loading={isLoading}
          />
        </Col>

        <Col xs={24}>
          <BreakdownCard
            title={t("aigateway.usageStats.byDay")}
            data={stats?.byDay ?? []}
            rowKey={(record) => record.date}
            columns={[
              {
                title: t("aigateway.usageStats.date"),
                dataIndex: "date",
                key: "date",
              },
              {
                title: t("aigateway.usageStats.tokens"),
                dataIndex: "tokens",
                key: "tokens",
                render: (v: number) => formatInt(v),
              },
              {
                title: t("aigateway.usageStats.cost"),
                dataIndex: "costUsd",
                key: "costUsd",
                render: (v: number) => formatUSD(v),
              },
              {
                title: t("aigateway.usageStats.trend"),
                key: "trend",
                render: (
                  _: unknown,
                  __: DailyUsageBreakdown,
                  idx: number,
                  arr: DailyUsageBreakdown[],
                ) => {
                  // byDay is ordered DESC — 最新的在前面
                  if (idx >= arr.length - 1) return <Text type="secondary">—</Text>;
                  const curr = arr[idx].tokens;
                  const prev = arr[idx + 1].tokens;
                  if (prev === 0) return <Text type="secondary">—</Text>;
                  const pct = ((curr - prev) / prev) * 100;
                  if (pct > 0) {
                    return (
                      <Text type="success">
                        <ArrowUpOutlined /> +{pct.toFixed(1)}%
                      </Text>
                    );
                  }
                  return (
                    <Text type="danger">
                      <ArrowDownOutlined /> {pct.toFixed(1)}%
                    </Text>
                  );
                },
              },
            ]}
            loading={isLoading}
          />
        </Col>
      </Row>
    </div>
  );
}

// =============================================================================
// Sub-components
// =============================================================================

interface KpiCardProps {
  title: string;
  value: string;
  icon: React.ReactNode;
  color: string;
  subtitle?: string;
  loading?: boolean;
}

function KpiCard({ title, value, icon, color, subtitle, loading }: KpiCardProps) {
  return (
    <Col xs={24} sm={12} md={8} xl={4}>
      <Card
        loading={loading}
        variant="borderless"
        style={{
          borderRadius: 12,
          boxShadow: "0 1px 2px rgba(0,0,0,0.06)",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <div
            style={{
              width: 44,
              height: 44,
              borderRadius: 10,
              background: `color-mix(in srgb, ${color} 8%, transparent)`,
              color,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              fontSize: 20,
              flexShrink: 0,
            }}
          >
            {icon}
          </div>
          <div style={{ flex: 1, minWidth: 0 }}>
            <Text type="secondary" style={{ fontSize: 13 }}>
              {title}
            </Text>
            <div
              style={{
                fontSize: 22,
                fontWeight: 600,
                color: "var(--gl-color-text)",
                lineHeight: 1.2,
              }}
            >
              {value}
            </div>
            {subtitle && (
              <Text type="secondary" style={{ fontSize: 12 }}>
                {subtitle}
              </Text>
            )}
          </div>
        </div>
      </Card>
    </Col>
  );
}

interface BreakdownCardProps<T> {
  title: string;
  data: T[];
  columns: unknown[];
  loading?: boolean;
  rowKey: (record: T) => string;
}

function BreakdownCard<T extends object>({
  title,
  data,
  columns,
  loading,
  rowKey,
}: BreakdownCardProps<T>) {
  const { t } = useTranslation();
  return (
    <Card
      title={<span style={{ fontWeight: 600 }}>{title}</span>}
      loading={loading}
      variant="borderless"
      style={{ borderRadius: 12, boxShadow: "0 1px 2px rgba(0,0,0,0.06)", height: "100%" }}
    >
      <Table
        size="small"
        rowKey={rowKey}
        dataSource={data}
        columns={columns as never}
        pagination={false}
        locale={{ emptyText: <Text type="secondary">{t("noData")}</Text> }}
      />
    </Card>
  );
}

// Re-export for lazy route typing
export type { UsageStats, ProviderUsageBreakdown, DailyUsageBreakdown, ProtocolUsageBreakdown };
