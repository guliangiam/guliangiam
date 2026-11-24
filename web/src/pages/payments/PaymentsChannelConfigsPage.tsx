// =============================================================================
// Commerce — Payments Channel Configs 管理页
// =============================================================================
//
// 后端: /api/v1/pay-channel-configs
// 设计文档: G6 §P2 版本化渠道配置 + 状态机 (draft → testing → active/disabled)
//
// 职责：列出所有渠道版本配置，支持 Create / Transition 状态转换。
// 同一 (org, channel, env) 自动递增 version_no — 后端 ChannelConfigService.Create 负责。
// =============================================================================

import {
  ApiOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  CloudServerOutlined,
  ExperimentOutlined,
  KeyOutlined,
  MinusCircleOutlined,
  PlusOutlined,
  StopOutlined,
  ThunderboltOutlined,
  WarningOutlined,
} from "@ant-design/icons";
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Descriptions,
  Empty,
  Form,
  Input,
  Modal,
  Row,
  Segmented,
  Select,
  Space,
  Spin,
  Statistic,
  Table,
  Tag,
  Typography,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { FormInstance } from "antd";
import type { ColumnsType } from "antd/es/table";
import { useTranslation } from "react-i18next";
import { useState } from "react";

import { useSessionStore } from "@/features/session/session-store";

import {
  CHANNEL_STATUS_OPTIONS,
  ENVIRONMENT_OPTIONS,
  createChannelConfig,
  getChannelConfig,
  listChannelConfigs,
  transitionChannelConfig,
  type ChannelConfigResponse,
  type ChannelConfigStatus,
} from "@/shared/api/payments-admin";
import { isHttpError } from "@/shared/api/http";
import { applyServerFieldErrors, buildServerFieldResolver } from "@/shared/lib/server-error";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import {
  MAX_CHANNEL_CONFIG_BYTES,
  buildChannelConfigPayload,
  validateChannelConfigText,
  type ChannelConfigFormValues,
} from "./channel-config-payload";
import { assembleChannelTemplateConfig, getChannelTemplate } from "./channel-field-templates";
import { usePayChannels, type PayChannelOption } from "./usePayChannels";

const { Title, Text } = Typography;
const QUERY_KEY = ["payments-admin", "channel-configs"];

// ── Main page ────────────────────────────────────────────────────────

export default function PaymentsChannelConfigsPage() {
  return (
    <PermissionGuard action="pay-channel-configs:read">
      <PaymentsChannelConfigsWorkspace />
    </PermissionGuard>
  );
}

function PaymentsChannelConfigsWorkspace() {
  const queryClient = useQueryClient();
  const [tc] = useTranslation("common");
  const { message } = App.useApp();
  const organization = useSessionStore((state) => state.organization);

  const [createOpen, setCreateOpen] = useState(false);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [transitionOpen, setTransitionOpen] = useState<string | null>(null);
  const [filterChannel, setFilterChannel] = useState<string | undefined>();
  const [filterEnv, setFilterEnv] = useState<string | undefined>();
  const [filterStatus, setFilterStatus] = useState<ChannelConfigStatus | undefined>();
  const { options: channelOptions, labels: channelLabels } = usePayChannels();
  const [createForm] = Form.useForm();

  const listQ = useQuery({
    queryKey: QUERY_KEY,
    queryFn: () => listChannelConfigs(),
  });

  const detailQ = useQuery({
    queryKey: [...QUERY_KEY, "detail", selectedId],
    queryFn: () => (selectedId ? getChannelConfig(selectedId) : undefined),
    enabled: !!selectedId,
  });

  const createMut = useMutation({
    mutationFn: createChannelConfig,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
      message.success(tc("paymentsAdmin.configCreated", "Channel config created"));
      setCreateOpen(false);
    },
    onError: (error) => {
      if (isHttpError(error)) {
        applyServerFieldErrors(createForm, error, buildServerFieldResolver(createForm), tc);
      }
      message.error(tc("paymentsAdmin.createFailed", "Create failed"));
    },
  });

  const transitionMut = useMutation({
    mutationFn: (vars: { id: string; status: ChannelConfigStatus }) =>
      transitionChannelConfig(vars.id, vars.status),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: [...QUERY_KEY, "detail", selectedId] });
      message.success(tc("paymentsAdmin.stateChanged", "State transitioned"));
      setTransitionOpen(null);
    },
    onError: () => message.error(tc("paymentsAdmin.transitionFailed", "Transition failed")),
  });

  const all = listQ.data?.items ?? [];
  const filtered = all.filter(
    (c) =>
      (!filterChannel || c.channel === filterChannel) &&
      (!filterEnv || c.environment === filterEnv) &&
      (!filterStatus || c.status === filterStatus),
  );

  // 摘要统计
  const stats = {
    total: all.length,
    active: all.filter((c) => c.status === "active").length,
    testing: all.filter((c) => c.status === "testing").length,
    disabled: all.filter((c) => c.status === "disabled").length,
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 24 }}>
      {/* ── Title + Toolbar ──────────────────────────────────────── */}
      <div
        style={{ display: "flex", alignItems: "center", gap: 12, justifyContent: "space-between" }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <Title level={3} style={{ margin: 0 }}>
            <CloudServerOutlined /> {tc("paymentsAdmin.channelConfigs", "Channel Configs")}
          </Title>
        </div>
        <PermissionGuard action="pay-channel-configs:write" fallback={null}>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
            {tc("paymentsAdmin.createConfig", "New Channel Config")}
          </Button>
        </PermissionGuard>
      </div>

      {/* ── Stats Row ────────────────────────────────────────────── */}
      <Row gutter={16}>
        <Col span={6}>
          <Card size="small">
            <Statistic
              title={tc("paymentsAdmin.totalVersions", "Total Versions")}
              value={stats.total}
              prefix={<ApiOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic
              title={tc("paymentsAdmin.active", "Active")}
              value={stats.active}
              styles={{ content: { color: "var(--gl-color-success-text)" } }}
              prefix={<CheckCircleOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic
              title={tc("paymentsAdmin.testing", "Testing")}
              value={stats.testing}
              styles={{ content: { color: "var(--gl-color-warning)" } }}
              prefix={<ExperimentOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic
              title={tc("paymentsAdmin.disabled", "Disabled")}
              value={stats.disabled}
              styles={{ content: { color: "var(--gl-color-text-tertiary)" } }}
              prefix={<MinusCircleOutlined />}
            />
          </Card>
        </Col>
      </Row>

      {/* ── Filters + Table ──────────────────────────────────────── */}
      <Card size="small">
        <Space style={{ marginBottom: 16 }} wrap>
          <span>{tc("paymentsAdmin.filterChannel", "Channel")}:</span>
          <Select
            allowClear
            aria-label={tc("paymentsAdmin.filterChannel", "Channel")}
            placeholder={tc("common.all", "All")}
            style={{ width: 160 }}
            value={filterChannel}
            onChange={setFilterChannel}
            options={channelOptions}
          />
          <span>{tc("paymentsAdmin.filterEnv", "Environment")}:</span>
          <Segmented
            value={filterEnv ?? "all"}
            onChange={(v) => setFilterEnv(v === "all" ? undefined : v)}
            options={[
              { value: "all", label: tc("common.all", "All") },
              ...ENVIRONMENT_OPTIONS.map((e) => ({ value: e.value, label: e.label })),
            ]}
          />
          <span>{tc("paymentsAdmin.filterStatus", "Status")}:</span>
          <Segmented
            value={filterStatus ?? "all"}
            onChange={(v) => setFilterStatus(v === "all" ? undefined : (v as ChannelConfigStatus))}
            options={[
              { value: "all", label: tc("common.all", "All") },
              ...CHANNEL_STATUS_OPTIONS.map((s) => ({ value: s.value, label: s.label })),
            ]}
          />
        </Space>

        {listQ.isLoading ? (
          <div style={{ padding: 48, textAlign: "center" }}>
            <Spin />
          </div>
        ) : filtered.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={tc(
              "paymentsAdmin.noConfigs",
              "No channel configs match the current filters",
            )}
          />
        ) : (
          <ConfigTable
            configs={filtered}
            channelLabels={channelLabels}
            onSelect={(id) => setSelectedId(id)}
            onTransition={(id) => setTransitionOpen(id)}
            selectedId={selectedId}
          />
        )}
      </Card>

      {/* ── Detail panel ─────────────────────────────────────────── */}
      {selectedId && (
        <DetailPanel
          config={detailQ.data}
          channelLabels={channelLabels}
          loading={detailQ.isLoading}
          onClose={() => setSelectedId(null)}
        />
      )}

      {/* ── Create Modal ──────────────────────────────────────────── */}
      <CreateModal
        form={createForm}
        open={createOpen}
        channelOptions={channelOptions}
        onCancel={() => setCreateOpen(false)}
        loading={createMut.isPending}
        onSubmit={(values) =>
          createMut
            .mutateAsync({
              ...buildChannelConfigPayload(values),
              organizationId: organization?.id ?? "",
            })
            .then(() => undefined)
        }
        existingChannels={all.map((c) => c.channel)}
      />

      {/* ── Transition Modal ───────────────────────────────────────── */}
      {transitionOpen && (
        <TransitionModal
          open
          currentStatus={all.find((c) => c.id === transitionOpen)?.status ?? "draft"}
          loading={transitionMut.isPending}
          onCancel={() => setTransitionOpen(null)}
          onSubmit={(status) =>
            transitionMut.mutateAsync({ id: transitionOpen, status }).then(() => undefined)
          }
        />
      )}
    </div>
  );
}

// =============================================================================
// Sub components
// =============================================================================

function ConfigTable({
  configs,
  channelLabels,
  selectedId,
  onSelect,
  onTransition,
}: {
  configs: ChannelConfigResponse[];
  channelLabels: Record<string, string>;
  selectedId: string | null;
  onSelect: (id: string) => void;
  onTransition: (id: string) => void;
}) {
  const [tc] = useTranslation("common");
  const columns: ColumnsType<ChannelConfigResponse> = [
    {
      title: tc("paymentsAdmin.channel", "Channel"),
      dataIndex: "channel",
      width: 160,
      fixed: "left",
      render: (ch: string) => (
        <Space>
          <ApiOutlined />
          <strong>{channelLabels[ch] ?? ch}</strong>
        </Space>
      ),
    },
    {
      title: tc("paymentsAdmin.version", "Version"),
      dataIndex: "versionNo",
      width: 80,
      align: "center",
      render: (v: number) => <Tag color="geekblue">v{v}</Tag>,
    },
    {
      title: tc("paymentsAdmin.env", "Env"),
      dataIndex: "environment",
      width: 120,
      render: (env: string) => {
        const opt = ENVIRONMENT_OPTIONS.find((e) => e.value === env);
        return <Tag color={opt?.color}>{opt?.label ?? env}</Tag>;
      },
    },
    {
      title: tc("paymentsAdmin.status", "Status"),
      dataIndex: "status",
      width: 120,
      render: (s: ChannelConfigStatus) => {
        const opt = CHANNEL_STATUS_OPTIONS.find((x) => x.value === s);
        return <Tag color={opt?.color ?? "default"}>{opt?.label ?? s}</Tag>;
      },
    },
    {
      title: tc("paymentsAdmin.fingerprint", "Fingerprint"),
      dataIndex: "configFingerprint",
      width: 120,
      render: (f: string) => (
        <Text code copyable={{ text: f }} style={{ fontSize: 12 }}>
          {f}
        </Text>
      ),
    },
    {
      title: tc("paymentsAdmin.updated", "Updated"),
      dataIndex: "updatedAt",
      width: 180,
      render: (d: string) => (
        <Space>
          <ClockCircleOutlined />
          {formatDate(d)}
        </Space>
      ),
    },
    {
      title: tc("paymentsAdmin.actions", "Actions"),
      key: "actions",
      fixed: "right",
      width: 180,
      render: (_, row) => (
        <Space>
          <Button size="small" onClick={() => onSelect(row.id)}>
            {tc("common.view", "View")}
          </Button>
          <PermissionGuard action="pay-channel-configs:write" fallback={null}>
            <Button size="small" type="primary" onClick={() => onTransition(row.id)}>
              {tc("paymentsAdmin.transition", "Transition")}
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
      dataSource={configs}
      scroll={{ x: 1000 }}
      rowSelection={{
        type: "radio",
        selectedRowKeys: selectedId ? [selectedId] : [],
        onChange: (keys) => onSelect(keys[0] as string),
      }}
      pagination={false}
    />
  );
}

function DetailPanel({
  config,
  channelLabels,
  loading,
  onClose,
}: {
  config: ChannelConfigResponse | undefined;
  channelLabels: Record<string, string>;
  loading: boolean;
  onClose: () => void;
}) {
  const [tc] = useTranslation("common");

  if (loading) {
    return (
      <Card size="small" title={tc("paymentsAdmin.configDetail", "Config Detail")}>
        <Spin />
      </Card>
    );
  }

  if (!config) return null;

  const channelLabel = channelLabels[config.channel] ?? config.channel;
  const envOpt = ENVIRONMENT_OPTIONS.find((e) => e.value === config.environment);
  const statusOpt = CHANNEL_STATUS_OPTIONS.find((s) => s.value === config.status);

  return (
    <Card
      size="small"
      title={
        <Space>
          <CloudServerOutlined />
          {channelLabel} · v{config.versionNo}
          <Tag color={statusOpt?.color}>{statusOpt?.label ?? config.status}</Tag>
        </Space>
      }
      extra={
        <Button size="small" onClick={onClose}>
          ✕
        </Button>
      }
    >
      <Descriptions column={{ xs: 1, sm: 2 }} size="small" bordered>
        <Descriptions.Item label={tc("paymentsAdmin.channel", "Channel")}>
          {channelLabel}
        </Descriptions.Item>
        <Descriptions.Item label={tc("paymentsAdmin.env", "Environment")}>
          <Tag color={envOpt?.color}>{envOpt?.label ?? config.environment}</Tag>
        </Descriptions.Item>
        <Descriptions.Item label={tc("paymentsAdmin.status", "Status")}>
          <Tag color={statusOpt?.color}>{statusOpt?.label ?? config.status}</Tag>
        </Descriptions.Item>
        <Descriptions.Item label={tc("paymentsAdmin.version", "Version")}>
          v{config.versionNo}
        </Descriptions.Item>
        <Descriptions.Item label={tc("paymentsAdmin.fingerprint", "Fingerprint")}>
          <Space>
            <KeyOutlined />
            <Text code copyable>
              {config.configFingerprint}
            </Text>
          </Space>
        </Descriptions.Item>
        <Descriptions.Item label={tc("paymentsAdmin.revision", "Revision")}>
          {config.revision}
        </Descriptions.Item>
        <Descriptions.Item label={tc("paymentsAdmin.created", "Created")}>
          {formatDate(config.createdAt)}
        </Descriptions.Item>
        <Descriptions.Item label={tc("paymentsAdmin.updated", "Updated")}>
          {formatDate(config.updatedAt)}
        </Descriptions.Item>
        {config.testPassedAt && (
          <Descriptions.Item label={tc("paymentsAdmin.testedAt", "Tested At")}>
            <ExperimentOutlined /> {formatDate(config.testPassedAt)}
          </Descriptions.Item>
        )}
        {config.activatedAt && (
          <Descriptions.Item label={tc("paymentsAdmin.activatedAt", "Activated At")}>
            <CheckCircleOutlined /> {formatDate(config.activatedAt)}
          </Descriptions.Item>
        )}
        {config.disabledAt && (
          <Descriptions.Item label={tc("paymentsAdmin.disabledAt", "Disabled At")}>
            <StopOutlined /> {formatDate(config.disabledAt)}
          </Descriptions.Item>
        )}
        {config.configSet && config.configSet.length > 0 && (
          <Descriptions.Item
            label={tc("paymentsAdmin.configFields", "Config Fields")}
            span={{ xs: 1, sm: 2 }}
          >
            <Space wrap>
              {config.configSet.map((f) => (
                <Tag key={f}>{f}</Tag>
              ))}
            </Space>
          </Descriptions.Item>
        )}
      </Descriptions>
    </Card>
  );
}

function CreateModal({
  form,
  open,
  channelOptions,
  onCancel,
  onSubmit,
  loading,
  existingChannels,
}: {
  form: FormInstance;
  open: boolean;
  channelOptions: PayChannelOption[];
  onCancel: () => void;
  onSubmit: (values: ChannelConfigFormValues) => void | Promise<void>;
  loading: boolean;
  existingChannels: string[];
}) {
  const [tc] = useTranslation("common");
  const watchedChannel = Form.useWatch("channel", form) as string | undefined;
  const template = getChannelTemplate(watchedChannel);

  return (
    <Modal
      open={open}
      title={tc("paymentsAdmin.createConfig", "New Channel Config")}
      confirmLoading={loading}
      onCancel={() => {
        form.resetFields();
        onCancel();
      }}
      onOk={async () => {
        const v = await form.validateFields();
        const values: ChannelConfigFormValues = template
          ? {
              channel: v.channel,
              environment: v.environment,
              config: assembleChannelTemplateConfig(template, v.environment, v.fields ?? {}),
            }
          : v;
        try {
          await onSubmit(values);
          form.resetFields();
        } catch {
          // mutation.onError 已提示；保留输入供用户重试。
        }
      }}
      width={560}
      destroyOnHidden
    >
      <Alert
        type="info"
        showIcon
        icon={<WarningOutlined />}
        style={{ marginBottom: 16 }}
        title={tc(
          "paymentsAdmin.versionAutoIncrement",
          "Version number auto-increments per (organization, channel, environment). The plaintext config is encrypted by the server before storage.",
        )}
      />

      <Form form={form} layout="vertical" preserve={false}>
        <Form.Item
          label={tc("paymentsAdmin.channel", "Channel")}
          name="channel"
          rules={[{ required: true, message: tc("paymentsAdmin.required", "Required") }]}
        >
          <Select
            showSearch
            placeholder={tc("paymentsAdmin.selectChannel", "Select channel")}
            options={channelOptions}
          />
        </Form.Item>

        <Form.Item
          label={tc("paymentsAdmin.env", "Environment")}
          name="environment"
          initialValue="sandbox"
          rules={[{ required: true }]}
        >
          <Segmented block options={ENVIRONMENT_OPTIONS} />
        </Form.Item>

        {template ? (
          <>
            <Alert
              type="success"
              showIcon
              style={{ marginBottom: 16 }}
              title={tc(
                "paymentsAdmin.templateNote",
                "These fields are assembled into the channel config JSON and encrypted by the server. Sandbox environments set isSandbox automatically.",
              )}
            />
            {template.map((field) => (
              <Form.Item
                key={field.key}
                label={tc(field.labelKey, field.labelFallback)}
                name={["fields", field.key]}
                rules={
                  field.required
                    ? [{ required: true, message: tc("paymentsAdmin.required", "Required") }]
                    : undefined
                }
                extra={field.extraKey ? tc(field.extraKey, field.extraFallback ?? "") : undefined}
              >
                {field.control === "password" ? (
                  <Input.Password placeholder={field.placeholder} autoComplete="new-password" />
                ) : field.control === "textarea" ? (
                  <Input.TextArea
                    autoSize={{ minRows: 3, maxRows: 8 }}
                    placeholder={field.placeholder}
                  />
                ) : (
                  <Input placeholder={field.placeholder} />
                )}
              </Form.Item>
            ))}
          </>
        ) : (
          <Form.Item
            label={tc("paymentsAdmin.config", "Config (JSON)")}
            name="config"
            rules={[
              {
                required: true,
                message: tc("paymentsAdmin.configRequired", "Config is required"),
              },
              {
                validator: (_, value: string) => {
                  const issue = validateChannelConfigText(value);
                  if (issue === null || issue === "required") {
                    return Promise.resolve();
                  }
                  const errorMessage =
                    issue === "tooLarge"
                      ? tc(
                          "paymentsAdmin.configTooLarge",
                          `Config must not exceed ${MAX_CHANNEL_CONFIG_BYTES} bytes`,
                        )
                      : tc("paymentsAdmin.configInvalid", "Config must be a non-empty JSON object");
                  return Promise.reject(new Error(errorMessage));
                },
              },
            ]}
            extra={tc(
              "paymentsAdmin.configExtra",
              "Stored encrypted by the server; only the fingerprint is shown afterwards.",
            )}
          >
            <Input.TextArea
              autoSize={{ minRows: 5, maxRows: 12 }}
              placeholder={tc(
                "paymentsAdmin.configPlaceholder",
                'Plaintext JSON object, e.g. {"appId":"...","privateKey":"..."}. The server encrypts it before storage.',
              )}
            />
          </Form.Item>
        )}
      </Form>

      {existingChannels.length > 0 && (
        <div style={{ marginTop: 12, fontSize: 12, color: "var(--gl-color-text-tertiary)" }}>
          <ThunderboltOutlined />{" "}
          {tc("paymentsAdmin.existingChannels", "Existing channels in this org:")}{" "}
          {Array.from(new Set(existingChannels)).join(", ")}
        </div>
      )}
    </Modal>
  );
}

function TransitionModal({
  open,
  currentStatus,
  loading,
  onCancel,
  onSubmit,
}: {
  open: boolean;
  currentStatus: ChannelConfigStatus;
  loading: boolean;
  onCancel: () => void;
  onSubmit: (status: ChannelConfigStatus) => void | Promise<void>;
}) {
  const [tc] = useTranslation("common");

  // 合法状态转换路径 — 与 Go PayChannelConfig.CanTransitionTo 保持同步
  const nextOptions: Record<ChannelConfigStatus, ChannelConfigStatus[]> = {
    draft: ["testing", "disabled"],
    testing: ["active", "draft", "disabled"],
    active: ["disabled", "draft"],
    disabled: ["draft", "testing"],
  };

  const options = (nextOptions[currentStatus] ?? []).map((s) => {
    const opt = CHANNEL_STATUS_OPTIONS.find((x) => x.value === s)!;
    return { value: s, label: opt.label, color: opt.color };
  });

  // 受控选择：点击只更新选中态，确认（OK）才真正提交，避免误切状态。
  // 必须用 null 而非 undefined：rc-segmented 对 undefined 退化为非受控并
  // 默认选中第一项，造成「视觉已选但 OK 永远禁用」的死锁。
  const [selected, setSelected] = useState<ChannelConfigStatus | null>(null);

  return (
    <Modal
      open={open}
      title={tc("paymentsAdmin.transitionTitle", "State Transition")}
      confirmLoading={loading}
      onCancel={onCancel}
      okButtonProps={{ disabled: !selected }}
      onOk={async () => {
        if (!selected) return;
        try {
          await onSubmit(selected);
        } catch {
          // mutation.onError 已提示；保留弹窗供重试。
        }
      }}
      width={480}
      destroyOnHidden
    >
      <div style={{ marginBottom: 16 }}>
        <Text strong>{tc("paymentsAdmin.currentStatus", "Current Status")}:</Text>{" "}
        <Tag color={CHANNEL_STATUS_OPTIONS.find((s) => s.value === currentStatus)?.color}>
          {CHANNEL_STATUS_OPTIONS.find((s) => s.value === currentStatus)?.label}
        </Tag>
      </div>
      <Alert
        type="info"
        showIcon
        title={tc("paymentsAdmin.selectTransition", "Select target state:")}
        style={{ marginBottom: 12 }}
      />
      <Segmented<ChannelConfigStatus | null>
        block
        value={selected}
        options={options.map((o) => ({
          value: o.value,
          label: <Tag color={o.color}>{o.label}</Tag>,
        }))}
        onChange={(v) => setSelected(v)}
      />
    </Modal>
  );
}

// ── Helpers ────────────────────────────────────────────────────────────────

function formatDate(iso?: string | null): string {
  if (!iso) return "—";
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}
