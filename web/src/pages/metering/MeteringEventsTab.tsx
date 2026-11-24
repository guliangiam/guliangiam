import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { FilterOutlined, ReloadOutlined } from "@ant-design/icons";
import { Button, DatePicker, Form, Input, Select, Space, Table } from "antd";
import {
  buildMeteringEventColumns,
  EVENT_TYPE_OPTIONS,
  type MeteringEventFilters,
} from "./metering-events-columns";
import { useMeteringEventsTabController } from "./useMeteringEventsTabController";

const { RangePicker } = DatePicker;

export default function MeteringEventsTab() {
  const { t } = useTranslation("common");
  const { filters, setFilters, query } = useMeteringEventsTabController();

  const columns = useMemo(
    () =>
      buildMeteringEventColumns({
        eventType: t("metering.events.eventType", "Event Type"),
        subject: t("metering.events.subject", "Subject"),
        quantity: t("metering.events.quantity", "Quantity"),
        occurredAt: t("metering.events.occurredAt", "Occurred At"),
        createdAt: t("metering.events.createdAt", "Created At"),
        metadata: t("metering.events.metadata", "Metadata"),
      }),
    [t],
  );

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--gl-space-3)" }}>
      {/* ── Filter Bar ── */}
      <Form
        layout="inline"
        onFinish={(v: MeteringEventFilters) => setFilters(v)}
        initialValues={filters}
        style={{ gap: "var(--gl-space-3)" }}
      >
        <Form.Item name="eventType">
          <Select
            allowClear
            aria-label={t("metering.events.filter.eventType", "All event types")}
            placeholder={t("metering.events.filter.eventType", "All event types")}
            style={{ width: 200 }}
            options={EVENT_TYPE_OPTIONS}
          />
        </Form.Item>
        <Form.Item name="subject">
          <Input
            allowClear
            aria-label={t("metering.events.filter.subject", "Subject / api_key_id")}
            placeholder={t("metering.events.filter.subject", "Subject / api_key_id")}
            style={{ width: 260 }}
          />
        </Form.Item>
        <Form.Item name="timeRange">
          <RangePicker
            showTime={{ format: "HH:mm" }}
            format="YYYY-MM-DD HH:mm"
            aria-label={t("metering.events.filter.timeRange", "Time range")}
            placeholder={[
              t("metering.events.filter.from", "From"),
              t("metering.events.filter.to", "To"),
            ]}
            onChange={(vals) => {
              // Ant Design returns moment[] or null
              if (!vals) {
                setFilters((f) => ({ ...f, timeRange: undefined }));
                return;
              }
              const [from, to] = vals;
              if (!from || !to) {
                setFilters((f) => ({ ...f, timeRange: undefined }));
                return;
              }
              setFilters((f) => ({
                ...f,
                timeRange: [from.toISOString(), to.toISOString()],
              }));
            }}
          />
        </Form.Item>
        <Form.Item>
          <Space>
            <Button icon={<FilterOutlined />} type="primary" htmlType="submit">
              {t("metering.events.filter.apply", "Apply")}
            </Button>
            <Button onClick={() => setFilters({})} icon={<ReloadOutlined />}>
              {t("metering.events.filter.reset", "Reset")}
            </Button>
          </Space>
        </Form.Item>
      </Form>

      {/* ── Table ── */}
      <Table
        rowKey="id"
        columns={columns}
        loading={query.isLoading}
        dataSource={query.data?.items ?? []}
        size="small"
        scroll={{ x: 1200 }}
        pagination={{
          total: query.data?.page?.total ?? 0,
          pageSize: 50,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) => t("metering.events.pagination.total", "{total} events", { total }),
        }}
      />
    </div>
  );
}
