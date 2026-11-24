import { useTranslation } from "react-i18next";
import {
  BarChartOutlined,
  FilterOutlined,
  ReloadOutlined,
  RetweetOutlined,
  ThunderboltOutlined,
} from "@ant-design/icons";
import { Button, Form, Input, Modal, Select, Space, Statistic, Table, Tag, message } from "antd";
import type { ColumnsType } from "antd/es/table";
import { useMemo, useState } from "react";
import { useMeteringAggregates, useRefreshMeteringAggregate } from "@/shared/hooks/use-metering";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import type { MeteringAggregateItem } from "@/shared/api/iam/metering";

/** Period shapes accepted by backend RefreshAggregateCommand (metering.md). */
const PERIOD_OPTIONS = [
  { value: "hour", label: "hour (YYYY-MM-DDTHH)" },
  { value: "day", label: "day (YYYY-MM-DD)" },
  { value: "month", label: "month (YYYY-MM)" },
];

/** Metric names produced by the outbox billing bridge. */
const METRIC_OPTIONS = [
  { value: "ai.usage.tokens", label: "ai.usage.tokens — LLM token consumption" },
  { value: "ai.usage.cost", label: "ai.usage.cost — monetary cost (milliyuan)" },
];

/** Tag color per metric — same palette as events tab for visual consistency. */
const METRIC_COLORS: Record<string, string> = {
  "ai.usage.tokens": "blue",
  "ai.usage.cost": "gold",
};

type Filters = {
  period?: string;
  subject?: string;
  metric?: string;
};

export default function MeteringAggregatesTab() {
  const { t } = useTranslation("common");
  const [filters, setFilters] = useState<Filters>({});

  const query = useMeteringAggregates({
    period: filters.period,
    subject: filters.subject || undefined,
    metric: filters.metric,
  });

  const refreshMutation = useRefreshMeteringAggregate();
  const [refreshOpen, setRefreshOpen] = useState(false);
  const [refreshForm] = Form.useForm<{ period: string; metric: string; subject?: string }>();

  const columns = useMemo<ColumnsType<MeteringAggregateItem>>(
    () => [
      {
        title: t("metering.aggregates.period", "Period"),
        dataIndex: "period",
        key: "period",
        width: 140,
        render: (v: string) => (
          <code
            style={{
              fontSize: "var(--gl-font-size-xs)",
              color: "var(--gl-color-text-secondary)",
            }}
          >
            {v}
          </code>
        ),
      },
      {
        title: t("metering.aggregates.metric", "Metric"),
        dataIndex: "metric",
        key: "metric",
        width: 180,
        render: (v: string) => <Tag color={METRIC_COLORS[v] ?? "purple"}>{v}</Tag>,
      },
      {
        title: t("metering.aggregates.subject", "Subject"),
        dataIndex: "subject",
        key: "subject",
        width: 220,
        ellipsis: true,
        render: (v: string) => (
          <code style={{ fontSize: "var(--gl-font-size-xs)" }}>{v || "—"}</code>
        ),
      },
      {
        title: t("metering.aggregates.quantity", "Aggregated Quantity"),
        dataIndex: "quantity",
        key: "quantity",
        width: 180,
        align: "right",
        render: (v: number, row) => (
          <span style={{ fontVariantNumeric: "tabular-nums", fontWeight: 600 }}>
            {Number(v).toLocaleString()}
            {row.unit && (
              <span
                style={{
                  color: "var(--gl-color-text-tertiary)",
                  marginLeft: "var(--gl-space-1)",
                  fontSize: "var(--gl-font-size-xs)",
                }}
              >
                {row.unit}
              </span>
            )}
          </span>
        ),
      },
      {
        title: t("metering.aggregates.calculatedAt", "Calculated At"),
        dataIndex: "calculatedAt",
        key: "calculatedAt",
        width: 180,
        render: (v: string) => (
          <span
            style={{
              fontSize: "var(--gl-font-size-xs)",
              color: "var(--gl-color-text-secondary)",
            }}
          >
            {new Date(v).toLocaleString()}
          </span>
        ),
      },
      {
        title: t("metering.aggregates.createdAt", "Created At"),
        dataIndex: "createdAt",
        key: "createdAt",
        width: 180,
        render: (v: string) => (
          <span
            style={{
              fontSize: "var(--gl-font-size-xs)",
              color: "var(--gl-color-text-tertiary)",
            }}
          >
            {new Date(v).toLocaleString()}
          </span>
        ),
      },
    ],
    [t],
  );

  // ── Header summary stats ──
  const rows = query.data?.items ?? [];
  const totalTokens = rows
    .filter((r) => r.metric === "ai.usage.tokens")
    .reduce((s, r) => s + r.quantity, 0);
  const totalCostMilli = rows
    .filter((r) => r.metric === "ai.usage.cost")
    .reduce((s, r) => s + r.quantity, 0);

  const openRefreshDialog = () => {
    // Pre-fill with current filter values as defaults
    refreshForm.setFieldsValue({
      period: filters.period || "hour",
      metric: filters.metric || "ai.usage.tokens",
      subject: filters.subject || "",
    });
    setRefreshOpen(true);
  };

  const submitRefresh = async () => {
    try {
      const vals = await refreshForm.validateFields();
      await refreshMutation.mutateAsync(vals);
      message.success(
        t(
          "metering.aggregates.refresh.success",
          "Refreshed aggregate for {period} / {metric}",
          vals,
        ),
      );
      setRefreshOpen(false);
    } catch {
      // validation error or server error — form shows inline, mutation shows error toast
    }
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--gl-space-3)" }}>
      {/* ── Summary stats (dense, top strip) ── */}
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fit, minmax(160px, 1fr))",
          gap: "var(--gl-space-3)",
          padding: "var(--gl-space-3) var(--gl-space-4)",
          borderRadius: "var(--gl-radius-sm)",
          background: "var(--gl-color-bg)",
          border: "1px solid var(--gl-color-border)",
        }}
      >
        <Statistic
          title={t("metering.aggregates.summary.rows", "Aggregate cells visible")}
          value={rows.length}
          prefix={<BarChartOutlined />}
          loading={query.isLoading}
        />
        <Statistic
          title={t("metering.aggregates.summary.tokens", "Total tokens (filtered)")}
          value={totalTokens}
          styles={{ content: { fontVariantNumeric: "tabular-nums" } }}
          loading={query.isLoading}
        />
        <Statistic
          title={t("metering.aggregates.summary.cost", "Total cost (milli-yuan)")}
          value={totalCostMilli}
          styles={{ content: { fontVariantNumeric: "tabular-nums" } }}
          loading={query.isLoading}
          prefix={<ThunderboltOutlined />}
        />
      </div>

      {/* ── Filter Bar ── */}
      <Form
        layout="inline"
        onFinish={(v: Filters) => setFilters(v)}
        initialValues={filters}
        style={{ gap: "var(--gl-space-3)" }}
      >
        <Form.Item name="period">
          <Select
            allowClear
            placeholder={t("metering.aggregates.filter.period", "All periods")}
            style={{ width: 200 }}
            options={PERIOD_OPTIONS}
          />
        </Form.Item>
        <Form.Item name="metric">
          <Select
            allowClear
            placeholder={t("metering.aggregates.filter.metric", "All metrics")}
            style={{ width: 260 }}
            options={METRIC_OPTIONS}
          />
        </Form.Item>
        <Form.Item name="subject">
          <Input
            allowClear
            placeholder={t("metering.aggregates.filter.subject", "Subject / api_key_id")}
            style={{ width: 260 }}
          />
        </Form.Item>
        <Form.Item>
          <Space>
            <Button icon={<FilterOutlined />} type="primary" htmlType="submit">
              {t("metering.aggregates.filter.apply", "Apply")}
            </Button>
            <Button onClick={() => setFilters({})} icon={<ReloadOutlined />}>
              {t("metering.aggregates.filter.reset", "Reset")}
            </Button>
            <PermissionGuard action="metering:write" resource="metering" fallback={null}>
              <Button
                icon={<RetweetOutlined />}
                onClick={openRefreshDialog}
                loading={refreshMutation.isPending}
                disabled={!query.data?.page?.total || query.data.page.total === 0}
              >
                {t("metering.aggregates.refresh.btn", "Manual Refresh…")}
              </Button>
            </PermissionGuard>
          </Space>
        </Form.Item>
      </Form>

      {/* ── Table ── */}
      <Table<MeteringAggregateItem>
        rowKey="id"
        columns={columns}
        loading={query.isLoading}
        dataSource={rows}
        size="small"
        scroll={{ x: 1100 }}
        pagination={{
          total: query.data?.page?.total ?? 0,
          pageSize: 50,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) =>
            t("metering.aggregates.pagination.total", "{total} aggregate cells", { total }),
        }}
      />

      {/* ── Manual Refresh Dialog ── */}
      <Modal
        open={refreshOpen}
        title={t("metering.aggregates.refresh.title", "Manual Refresh Aggregate")}
        onCancel={() => setRefreshOpen(false)}
        confirmLoading={refreshMutation.isPending}
        onOk={submitRefresh}
        okText={t("metering.aggregates.refresh.ok", "Refresh Now")}
        cancelText={t("actions.cancel", "Cancel")}
        width={480}
      >
        <div
          style={{
            marginBottom: "var(--gl-space-3)",
            color: "var(--gl-color-text-secondary)",
            fontSize: "var(--gl-font-size-sm)",
          }}
        >
          {t(
            "metering.aggregates.refresh.description",
            "This re-rolls a single aggregate cell by summing all raw metering_events that fall into the given window. Idempotent — safe to re-run. The AggregateWorker normally handles this on a 5-minute schedule; use this to back-fill or fix missed windows.",
          )}
        </div>
        <Form
          form={refreshForm}
          layout="vertical"
          preserve={false}
          initialValues={{ period: "hour", metric: "ai.usage.tokens", subject: "" }}
        >
          <Form.Item
            name="period"
            label={t(
              "metering.aggregates.refresh.period",
              "Period (YYYY-MM-DDTHH / YYYY-MM-DD / YYYY-MM)",
            )}
            rules={[{ required: true, message: t("metering.validation.required", "Required") }]}
          >
            <Input placeholder="2026-10-15T11" allowClear />
          </Form.Item>
          <Form.Item
            name="metric"
            label={t("metering.aggregates.refresh.metric", "Metric name")}
            rules={[{ required: true, message: t("metering.validation.required", "Required") }]}
          >
            <Select options={METRIC_OPTIONS} />
          </Form.Item>
          <Form.Item
            name="subject"
            label={t("metering.aggregates.refresh.subject", "Subject (optional — api_key_id)")}
          >
            <Input
              placeholder={t(
                "metering.aggregates.refresh.subjectPlaceholder",
                "empty = org-level rollup",
              )}
              allowClear
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
