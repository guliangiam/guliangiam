// =============================================================================
// Commerce — Payments Reconciliation (对账) Admin Page
// =============================================================================
//
// 后端: RegisterReconChiRoutes (design doc §8.2)
//   GET    /api/v1/pay-recon-batches               List batches (paginated)
//   GET    /api/v1/pay-recon-batches/:id/items     Diff items for one batch
//   POST   /api/v1/pay-recon-items/:id:resolve     Manual resolution of one item
//   POST   /api/v1/pay-recon:run                   Manual trigger (new or existing)
//
// 页面结构:
//   ┌──────────────────────────────────────┐
//   │ Title · [Run Reconciliation]         │
//   ├──────────────────────────────────────┤
//   │ Stats: Batches / Completed / Failed  │
//   ├──────────────────────────────────────┤
//   │ Batches List (filterable)            │
//   │  ↓ click row                         │
//   │ Items Table (diff detail)            │
//   │   → Resolve Item Modal (operator+note)│
//   └──────────────────────────────────────┘
// =============================================================================

import {
  Alert,
  Button,
  Card,
  Col,
  DatePicker,
  Descriptions,
  Empty,
  Form,
  Input,
  Modal,
  Row,
  Segmented,
  Select,
  Space,
  Statistic,
  Table,
  Tag,
  Tooltip,
  Typography,
  message,
} from "antd";
import {
  ApiOutlined,
  CheckCircleOutlined,
  CloudServerOutlined,
  DownloadOutlined,
  ExperimentOutlined,
  FireOutlined,
  StopOutlined,
  SyncOutlined,
  WarningOutlined,
} from "@ant-design/icons";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { FormInstance } from "antd";
import type { ColumnsType } from "antd/es/table";
import { useTranslation } from "react-i18next";
import dayjs from "dayjs";
import { useState } from "react";

import {
  RECON_BATCH_STATUS,
  RECON_DIFF_TYPE,
  listReconBatches,
  approveReconItem,
  listReconItems,
  resolveReconItem,
  runRecon,
  type ReconBatchListItem,
  type ReconBatchStatus,
  type ReconItemView,
} from "@/shared/api/payments-admin";
import { isHttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  localizedError,
} from "@/shared/lib/server-error";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { useSessionStore } from "@/features/session/session-store";
import { usePayChannels, type PayChannelOption } from "./usePayChannels";

const { Title, Text } = Typography;
const QUERY_KEY = ["commerce", "recon"];

// ── Main page ────────────────────────────────────────────────────────

export default function PaymentsReconPage() {
  const queryClient = useQueryClient();
  const [tc] = useTranslation("common");
  const organization = useSessionStore((state) => state.organization);

  const [selectedBatchId, setSelectedBatchId] = useState<string | null>(null);
  const [filterStatus, setFilterStatus] = useState<ReconBatchStatus | "all">("all");
  const [filterChannel, setFilterChannel] = useState<string | undefined>();
  const [runOpen, setRunOpen] = useState(false);
  const [resolveOpen, setResolveOpen] = useState<ReconItemView | null>(null);
  const { options: channelOptions, labels: channelLabels } = usePayChannels();
  const [runForm] = Form.useForm();
  const [resolveForm] = Form.useForm();

  // ── Queries ──────────────────────────────────────────────────────────
  const batchesQ = useQuery({
    queryKey: QUERY_KEY,
    queryFn: () => listReconBatches({ page: 1, pageSize: 100 }),
  });

  const itemsQ = useQuery({
    queryKey: [...QUERY_KEY, "items", selectedBatchId],
    queryFn: () => (selectedBatchId ? listReconItems(selectedBatchId) : undefined),
    enabled: !!selectedBatchId,
  });

  // ── Mutations ───────────────────────────────────────────────────────
  const runMut = useMutation({
    mutationFn: runRecon,
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
      message.success(
        tc("commerce.recon.triggered", "Reconciliation triggered (batch: {id})", {
          id: data?.batchId?.slice(-8) ?? "?",
        }),
      );
      setRunOpen(false);
      // 自动选中新 batch
      if (data?.batchId) setSelectedBatchId(data.batchId);
    },
    onError: (error) => {
      if (isHttpError(error)) {
        applyServerFieldErrors(runForm, error, buildServerFieldResolver(runForm), tc);
      }
      message.error(tc("commerce.recon.triggerFailed", "Run failed — check logs"));
    },
  });

  const approveMut = useMutation({
    mutationFn: (vars: { id: string; reason?: string }) =>
      approveReconItem(vars.id, { reason: vars.reason }),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [...QUERY_KEY, "items", selectedBatchId],
      });
      message.success(tc("commerce.recon.approved", "Item approved"));
    },
    onError: () => message.error(tc("commerce.recon.approveFailed", "Approve failed")),
  });

  const resolveMut = useMutation({
    mutationFn: (vars: { id: string; operator: string; note?: string }) =>
      resolveReconItem(vars.id, { operator: vars.operator, note: vars.note }),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [...QUERY_KEY, "items", selectedBatchId],
      });
      message.success(tc("commerce.recon.resolved", "Item resolved"));
      setResolveOpen(null);
    },
    onError: (error) => {
      if (isHttpError(error)) {
        applyServerFieldErrors(resolveForm, error, buildServerFieldResolver(resolveForm), tc);
      }
      message.error(tc("commerce.recon.resolveFailed", "Resolve failed"));
    },
  });

  const batches = batchesQ.data?.items ?? [];
  const batchesTotal = batchesQ.data?.page?.total ?? 0;

  const filteredBatches = batches.filter(
    (b) =>
      (filterStatus === "all" || b.status === filterStatus) &&
      (!filterChannel || b.channel === filterChannel),
  );

  // 统计
  const stats = {
    total: batchesTotal,
    pending: batches.filter((b) => b.status === "pending").length,
    running: batches.filter((b) => b.status === "running").length,
    completed: batches.filter((b) => b.status === "completed").length,
    failed: batches.filter((b) => b.status === "failed").length,
    totalDiffs: batches.reduce((acc, b) => acc + b.diffCount, 0),
  };

  const selectedBatch = batches.find((b) => b.id === selectedBatchId);
  const items = itemsQ.data ?? [];

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 24 }}>
      {/* ── Title ──────────────────────────────────────────────────── */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <Title level={3} style={{ margin: 0 }}>
            <DownloadOutlined /> {tc("commerce.recon.title", "Reconciliation")}
          </Title>
          <Text type="secondary">
            {tc(
              "commerce.recon.subtitle",
              "Daily reconciliation of local pay_attempts vs channel settlement reports",
            )}
          </Text>
        </div>
        <PermissionGuard action="pay-recon:write" fallback={null}>
          <Button type="primary" icon={<FireOutlined />} onClick={() => setRunOpen(true)}>
            {tc("commerce.recon.runButton", "Run Reconciliation")}
          </Button>
        </PermissionGuard>
      </div>

      {/* ── Stats Row ──────────────────────────────────────────────── */}
      <Row gutter={16}>
        <Col span={4}>
          <Card size="small">
            <Statistic
              title={tc("commerce.recon.totalBatches", "Total Batches")}
              value={stats.total}
              prefix={<ApiOutlined />}
            />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic
              title={tc("commerce.recon.pending", "Pending")}
              value={stats.pending}
              styles={{ content: { color: "var(--gl-color-text-tertiary)" } }}
            />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic
              title={tc("commerce.recon.running", "Running")}
              value={stats.running}
              styles={{ content: { color: "var(--gl-color-primary)" } }}
              prefix={<SyncOutlined spin />}
            />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic
              title={tc("commerce.recon.completed", "Completed")}
              value={stats.completed}
              styles={{ content: { color: "var(--gl-color-success-text)" } }}
              prefix={<CheckCircleOutlined />}
            />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic
              title={tc("commerce.recon.failed", "Failed")}
              value={stats.failed}
              styles={{ content: { color: "var(--gl-color-error-text)" } }}
              prefix={<WarningOutlined />}
            />
          </Card>
        </Col>
        <Col span={4}>
          <Card size="small">
            <Statistic
              title={tc("commerce.recon.totalDiffs", "Total Diffs")}
              value={stats.totalDiffs}
              styles={{
                content: {
                  color:
                    stats.totalDiffs > 0
                      ? "var(--gl-color-warning)"
                      : "var(--gl-color-success-text)",
                },
              }}
            />
          </Card>
        </Col>
      </Row>

      {/* ── Batches + Items 两列布局 ──────────────────────────────── */}
      <Row gutter={24}>
        {/* Left: Batches list */}
        <Col span={12}>
          <Card
            size="small"
            title={
              <Space>
                <CloudServerOutlined />
                {tc("commerce.recon.batches", "Recon Batches")}
              </Space>
            }
            extra={
              <Space>
                <Select
                  size="small"
                  allowClear
                  aria-label={tc("commerce.recon.filterChannel", "All channels")}
                  placeholder={tc("commerce.recon.filterChannel", "All channels")}
                  style={{ width: 140 }}
                  value={filterChannel}
                  onChange={setFilterChannel}
                  options={channelOptions}
                />
                <Segmented
                  size="small"
                  value={filterStatus}
                  onChange={(v) => setFilterStatus(v as ReconBatchStatus | "all")}
                  options={[
                    { value: "all", label: tc("common.all", "All") },
                    ...RECON_BATCH_STATUS.map((s) => ({
                      value: s.value,
                      label: s.label,
                    })),
                  ]}
                />
              </Space>
            }
          >
            {batchesQ.isLoading ? (
              <div style={{ padding: 32, textAlign: "center" }}>
                <SyncOutlined spin />
              </div>
            ) : filteredBatches.length === 0 ? (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description={tc(
                  "commerce.recon.noBatches",
                  "No batches match current filters. Click 'Run Reconciliation' to trigger one.",
                )}
              />
            ) : (
              <BatchesTable
                batches={filteredBatches}
                channelLabels={channelLabels}
                selectedId={selectedBatchId}
                onSelect={(id) => setSelectedBatchId(id)}
              />
            )}
          </Card>
        </Col>

        {/* Right: Items detail */}
        <Col span={12}>
          <Card
            size="small"
            title={
              <Space>
                <ExperimentOutlined />
                {tc("commerce.recon.items", "Diff Items")}
                {selectedBatch && (
                  <Tag
                    color={RECON_BATCH_STATUS.find((s) => s.value === selectedBatch.status)?.color}
                  >
                    {selectedBatch.channel} · {selectedBatch.batchDate} ·{selectedBatch.diffCount}{" "}
                    diffs
                  </Tag>
                )}
              </Space>
            }
          >
            {!selectedBatchId ? (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description={tc(
                  "commerce.recon.pickBatch",
                  "Select a batch on the left to view its diff items",
                )}
              />
            ) : itemsQ.isLoading ? (
              <div style={{ padding: 32, textAlign: "center" }}>
                <SyncOutlined spin />
              </div>
            ) : itemsQ.isError ? (
              <Alert
                type="error"
                showIcon
                icon={<WarningOutlined />}
                title={localizedError(itemsQ.error, tc, "commerce.recon.itemsLoadFailed")}
                action={
                  <Button size="small" onClick={() => itemsQ.refetch()}>
                    {tc("action.retry", "Retry")}
                  </Button>
                }
              />
            ) : items.length === 0 ? (
              <Alert
                type="success"
                showIcon
                icon={<CheckCircleOutlined />}
                title={tc(
                  "commerce.recon.noDiffs",
                  "This batch has zero diffs — local ledger matches channel settlement.",
                )}
              />
            ) : (
              <ItemsTable
                items={items}
                onResolve={(item) => setResolveOpen(item)}
                onApprove={(item) => approveMut.mutate({ id: item.id })}
                approveLoading={approveMut.isPending}
              />
            )}
          </Card>
        </Col>
      </Row>

      {/* ── Run Reconciliation Modal ───────────────────────────────── */}
      <RunReconModal
        form={runForm}
        open={runOpen}
        channelOptions={channelOptions}
        organizationId={organization?.id ?? undefined}
        loading={runMut.isPending}
        onCancel={() => setRunOpen(false)}
        onRun={(body) => runMut.mutateAsync(body).then(() => undefined)}
      />

      {/* ── Resolve Item Modal ───────────────────────────────────── */}
      {resolveOpen && (
        <ResolveModal
          form={resolveForm}
          open
          item={resolveOpen}
          loading={resolveMut.isPending}
          onCancel={() => setResolveOpen(null)}
          onSubmit={(operator, note) =>
            resolveMut.mutateAsync({ id: resolveOpen.id, operator, note }).then(() => undefined)
          }
        />
      )}
    </div>
  );
}

// =============================================================================
// Sub components
// =============================================================================

function BatchesTable({
  batches,
  channelLabels,
  selectedId,
  onSelect,
}: {
  batches: ReconBatchListItem[];
  channelLabels: Record<string, string>;
  selectedId: string | null;
  onSelect: (id: string) => void;
}) {
  const [tc] = useTranslation("common");
  const columns: ColumnsType<ReconBatchListItem> = [
    {
      title: tc("commerce.recon.batchDate", "Date"),
      dataIndex: "batchDate",
      width: 110,
      fixed: "left",
      sorter: (a, b) => new Date(a.batchDate).getTime() - new Date(b.batchDate).getTime(),
    },
    {
      title: tc("commerce.recon.channel", "Channel"),
      dataIndex: "channel",
      width: 130,
      render: (ch: string) => <strong>{channelLabels[ch] ?? ch}</strong>,
    },
    {
      title: tc("commerce.recon.status", "Status"),
      dataIndex: "status",
      width: 90,
      render: (s: ReconBatchStatus) => {
        const opt = RECON_BATCH_STATUS.find((x) => x.value === s);
        return <Tag color={opt?.color}>{opt?.label ?? s}</Tag>;
      },
    },
    {
      title: tc("commerce.recon.diffs", "Diffs"),
      dataIndex: "diffCount",
      width: 80,
      align: "right",
      sorter: (a, b) => a.diffCount - b.diffCount,
      render: (n: number) => (n > 0 ? <Tag color="volcano">{n}</Tag> : <Tag color="green">0</Tag>),
    },
    {
      title: tc("commerce.recon.totals", "Local / Channel"),
      width: 150,
      render: (_, row) => (
        <Text type="secondary">
          {row.totalLocal} / {row.totalChannel}
        </Text>
      ),
    },
    {
      title: tc("commerce.recon.updated", "Updated"),
      dataIndex: "updatedAt",
      width: 160,
      render: (d: string) => <Text style={{ fontSize: 12 }}>{formatDate(d)}</Text>,
    },
  ];

  return (
    <Table
      size="small"
      rowKey="id"
      columns={columns}
      dataSource={batches}
      rowSelection={{
        type: "radio",
        selectedRowKeys: selectedId ? [selectedId] : [],
        onChange: (keys) => onSelect(keys[0] as string),
      }}
      pagination={{ pageSize: 10, hideOnSinglePage: true }}
    />
  );
}

function ItemsTable({
  items,
  onResolve,
  onApprove,
  approveLoading,
}: {
  items: ReconItemView[];
  onResolve: (item: ReconItemView) => void;
  onApprove: (item: ReconItemView) => void;
  approveLoading: boolean;
}) {
  const [tc] = useTranslation("common");

  const amountStr = (v: number | null | undefined) =>
    v === null || v === undefined ? "—" : v.toFixed(2);

  const columns: ColumnsType<ReconItemView> = [
    {
      title: tc("commerce.recon.diffType", "Diff Type"),
      dataIndex: "diffType",
      width: 140,
      fixed: "left",
      render: (d: string) => {
        const opt = RECON_DIFF_TYPE.find((x) => x.value === d);
        return <Tag color={opt?.color}>{opt?.label ?? d}</Tag>;
      },
    },
    {
      title: tc("commerce.recon.channelOrderNo", "Channel Order No."),
      dataIndex: "channelOrderNo",
      width: 180,
      render: (v: string | null) => v ?? <Text type="secondary">—</Text>,
    },
    {
      title: tc("commerce.recon.localPaymentId", "Local Payment ID"),
      dataIndex: "localPaymentId",
      width: 180,
      render: (v: string | null) => v ?? <Text type="secondary">—</Text>,
    },
    {
      title: tc("commerce.recon.channelAmount", "Ch. Amount"),
      dataIndex: "channelAmount",
      width: 110,
      align: "right",
      render: (v: number | null) => amountStr(v),
    },
    {
      title: tc("commerce.recon.localAmount", "Local Amount"),
      dataIndex: "localAmount",
      width: 110,
      align: "right",
      render: (v: number | null) => amountStr(v),
    },
    {
      title: tc("commerce.recon.channelState", "Ch. State"),
      dataIndex: "channelState",
      width: 100,
      render: (v: string | null) => v ?? <Text type="secondary">—</Text>,
    },
    {
      title: tc("commerce.recon.localState", "Local State"),
      dataIndex: "localState",
      width: 100,
      render: (v: string | null) => v ?? <Text type="secondary">—</Text>,
    },
    {
      title: tc("commerce.recon.resolved", "Resolved"),
      dataIndex: "resolved",
      width: 100,
      render: (r: boolean, row) =>
        r ? (
          <Tooltip
            title={tc("commerce.recon.resolvedBy", "Resolved by: {name}", {
              name: row.resolvedBy ?? "?",
            })}
          >
            <Tag color="green">
              <CheckCircleOutlined /> {tc("common.yes", "Yes")}
            </Tag>
          </Tooltip>
        ) : (
          <Tag color="red">
            <StopOutlined /> {tc("common.no", "No")}
          </Tag>
        ),
    },
    {
      title: tc("commerce.recon.actions", "Action"),
      width: 160,
      key: "actions",
      fixed: "right",
      render: (_, row) => (
        <Space size="small">
          <PermissionGuard action="pay-recon-items:write" fallback={null}>
            <Button
              size="small"
              type="primary"
              disabled={row.resolved}
              onClick={() => onResolve(row)}
            >
              {tc("commerce.recon.resolve", "Resolve")}
            </Button>
          </PermissionGuard>
          <PermissionGuard action="pay-recon-items:approve" fallback={null}>
            <Button
              size="small"
              disabled={!row.resolved || row.resolutionState === "approved"}
              loading={approveLoading}
              onClick={() => onApprove(row)}
            >
              {tc("commerce.recon.approve", "Approve")}
            </Button>
          </PermissionGuard>
        </Space>
      ),
    },
  ];

  return (
    <Table
      size="small"
      rowKey="id"
      columns={columns}
      dataSource={items}
      scroll={{ x: 1100 }}
      pagination={{ pageSize: 15, hideOnSinglePage: true }}
    />
  );
}

// ── Run Reconciliation Modal ─────────────────────────────────────────

function RunReconModal({
  form,
  open,
  channelOptions,
  organizationId,
  loading,
  onCancel,
  onRun,
}: {
  form: FormInstance;
  open: boolean;
  channelOptions: PayChannelOption[];
  organizationId?: string;
  loading: boolean;
  onCancel: () => void;
  onRun: (body: {
    organizationId?: string;
    channel?: string;
    batchDate?: string;
    batchId?: string;
  }) => void | Promise<void>;
}) {
  const [tc] = useTranslation("common");

  return (
    <Modal
      open={open}
      title={tc("commerce.recon.runTitle", "Run Reconciliation")}
      confirmLoading={loading}
      onCancel={() => {
        form.resetFields();
        onCancel();
      }}
      onOk={async () => {
        const v = await form.validateFields();
        // 后端 DisallowUnknownFields：mode 是纯前端态，绝不能进请求体；
        // batchDate 必须是 YYYY-MM-DD/RFC3339 字符串（dayjs 对象序列化带毫秒会被拒）。
        const body =
          v.mode === "existing"
            ? { organizationId, batchId: v.batchId }
            : {
                organizationId,
                channel: v.channel,
                batchDate: (v.batchDate as dayjs.Dayjs).format("YYYY-MM-DD"),
              };
        try {
          await onRun(body);
        } catch {
          // mutation.onError 已提示；保留输入供重试。
        }
      }}
      width={520}
      destroyOnHidden
    >
      <Alert
        type="info"
        showIcon
        icon={<FireOutlined />}
        style={{ marginBottom: 16 }}
        title={tc(
          "commerce.recon.runInfo",
          "Two modes:\n(1) 指定 batchId → 触发已有 pending batch 立即执行\n(2) 新建 → 选择 channel + batchDate，先 CreateBatch(pending) 再立即执行",
        )}
      />
      <Form
        form={form}
        layout="vertical"
        initialValues={{
          mode: "create",
          batchDate: dayjs().subtract(1, "day"),
        }}
        preserve={false}
      >
        <Form.Item
          label={tc("commerce.recon.mode", "Mode")}
          name="mode"
          rules={[{ required: true }]}
        >
          <Segmented
            block
            options={[
              { value: "create", label: tc("commerce.recon.modeCreate", "New Batch") },
              { value: "existing", label: tc("commerce.recon.modeExisting", "Existing batchId") },
            ]}
          />
        </Form.Item>

        <Form.Item noStyle shouldUpdate={(prev, cur) => prev.mode !== cur.mode}>
          {({ getFieldValue }) => {
            const mode = getFieldValue("mode");
            if (mode === "existing") {
              return (
                <Form.Item
                  label={tc("commerce.recon.batchId", "Batch ID")}
                  name="batchId"
                  rules={[{ required: true }]}
                >
                  <Input placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" />
                </Form.Item>
              );
            }
            return (
              <>
                <Form.Item
                  label={tc("commerce.recon.channel", "Channel")}
                  name="channel"
                  rules={[{ required: true }]}
                >
                  <Select
                    showSearch
                    placeholder={tc("commerce.recon.selectChannel", "Select channel")}
                    options={channelOptions}
                  />
                </Form.Item>
                <Form.Item
                  label={tc("commerce.recon.batchDate", "Batch Date")}
                  name="batchDate"
                  rules={[{ required: true }]}
                >
                  <DatePicker style={{ width: "100%" }} format="YYYY-MM-DD" />
                </Form.Item>
              </>
            );
          }}
        </Form.Item>
      </Form>
    </Modal>
  );
}

// ── Resolve Modal ─────────────────────────────────────────────────────

function ResolveModal({
  form,
  open,
  item,
  loading,
  onCancel,
  onSubmit,
}: {
  form: FormInstance;
  open: boolean;
  item: ReconItemView;
  loading: boolean;
  onCancel: () => void;
  onSubmit: (operator: string, note: string) => void | Promise<void>;
}) {
  const [tc] = useTranslation("common");

  const diffOpt = RECON_DIFF_TYPE.find((x) => x.value === item.diffType);
  const amountStr = (v: number | null | undefined) =>
    v === null || v === undefined ? "—" : v.toFixed(2);

  return (
    <Modal
      open={open}
      title={tc("commerce.recon.resolveTitle", "Resolve Reconciliation Item")}
      confirmLoading={loading}
      onCancel={onCancel}
      onOk={async () => {
        const v = await form.validateFields();
        try {
          await onSubmit(v.operator, v.note ?? "");
        } catch {
          // mutation.onError 已提示；保留输入供重试。
        }
      }}
      width={560}
      destroyOnHidden
    >
      {/* Context Descriptions */}
      <Descriptions column={{ xs: 1, sm: 2 }} size="small" bordered style={{ marginBottom: 16 }}>
        <Descriptions.Item label={tc("commerce.recon.diffType", "Diff Type")}>
          <Tag color={diffOpt?.color}>{diffOpt?.label ?? item.diffType}</Tag>
        </Descriptions.Item>
        <Descriptions.Item label={tc("commerce.recon.batchId", "Batch")}>
          {item.batchId.slice(-12)}
        </Descriptions.Item>
        <Descriptions.Item label={tc("commerce.recon.channelOrderNo", "Channel Order No.")}>
          {item.channelOrderNo ?? "—"}
        </Descriptions.Item>
        <Descriptions.Item label={tc("commerce.recon.localPaymentId", "Local Payment")}>
          {item.localPaymentId ?? "—"}
        </Descriptions.Item>
        <Descriptions.Item label={tc("commerce.recon.channelAmount", "Ch. Amount")}>
          {amountStr(item.channelAmount)}
        </Descriptions.Item>
        <Descriptions.Item label={tc("commerce.recon.localAmount", "Local Amount")}>
          {amountStr(item.localAmount)}
        </Descriptions.Item>
        <Descriptions.Item label={tc("commerce.recon.channelState", "Ch. State")}>
          {item.channelState ?? "—"}
        </Descriptions.Item>
        <Descriptions.Item label={tc("commerce.recon.localState", "Local State")}>
          {item.localState ?? "—"}
        </Descriptions.Item>
      </Descriptions>

      <Alert
        type="warning"
        showIcon
        icon={<WarningOutlined />}
        style={{ marginBottom: 16 }}
        title={tc(
          "commerce.recon.resolveWarn",
          "Resolving a diff is an accounting decision. Operator name is recorded for audit trail.",
        )}
      />

      <Form form={form} layout="vertical" preserve={false}>
        <Form.Item
          label={tc("commerce.recon.operator", "Operator")}
          name="operator"
          rules={[{ required: true }]}
        >
          <Input placeholder="e.g. alice@company.com" />
        </Form.Item>
        <Form.Item label={tc("commerce.recon.note", "Note")} name="note">
          <Input.TextArea
            rows={3}
            placeholder={tc(
              "commerce.recon.notePlaceholder",
              "Reason for resolution, evidence reference, etc.",
            )}
          />
        </Form.Item>
      </Form>
    </Modal>
  );
}

// ── Helpers ────────────────────────────────────────────────────────────

function formatDate(iso?: string | null): string {
  if (!iso) return "—";
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}
