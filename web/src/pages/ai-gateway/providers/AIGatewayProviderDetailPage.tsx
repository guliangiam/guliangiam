// =============================================================================
// AI Gateway — Provider Detail（完整实现）
// =============================================================================
//
// 路由: /ai/providers/:id
// 后端: GET /api/v1/ai/providers/{id}
//       PATCH /api/v1/ai/providers/{id}
//       POST /api/v1/ai/providers/{id}:test
//       GET /api/v1/ai/providers/health/{providerId}
//       GET /api/v1/ai/models（过滤 providerId）
// =============================================================================

import {
  ApiOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  CloudServerOutlined,
  DeleteOutlined,
  DisconnectOutlined,
  EditOutlined,
  ExperimentOutlined,
  KeyOutlined,
  LinkOutlined,
  SettingOutlined,
  ThunderboltOutlined,
  WarningOutlined,
} from "@ant-design/icons";
import {
  Alert,
  Button,
  Card,
  Col,
  Descriptions,
  Divider,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Row,
  Space,
  Spin,
  Statistic,
  Table,
  Tag,
  Typography,
  message,
} from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ColumnsType } from "antd/es/table";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useEffect, useState } from "react";

import {
  collectAllPages,
  deleteProvider,
  getProvider,
  getProviderHealth,
  listModels,
  testProvider,
  updateProvider,
  type ModelResponse,
  type ProviderResponse,
  type ProviderTestResult,
  type ProviderUpdate,
  type ProviderHealthResponse,
} from "@/shared/api/aigateway";

import { GlFormRow } from "@/shared/ui/GlForm";
import { FormSection } from "@/shared/ui/FormSection";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { ResourceFormPage } from "@/shared/ui/ResourceFormPage";
import { isHttpError } from "@/shared/api/http";
import { applyServerFieldErrors, buildServerFieldResolver } from "@/shared/lib/server-error";

const { Text, Link: AntLink } = Typography;

const PROVIDER_TYPE_LABEL: Record<string, string> = {
  openai: "OpenAI Compatible",
  anthropic: "Anthropic",
  gemini: "Google Gemini",
  deepseek: "DeepSeek",
  glm: "Zhipu GLM",
  kimi: "Kimi / Moonshot",
  qwen: "Alibaba Qwen",
  seed: "ByteDance Seed",
  ollama: "Ollama (local)",
  vllm: "vLLM (local)",
  custom: "Custom / BYO",
};

const PROTOCOL_LABEL: Record<string, string> = {
  openai: "OpenAI",
  anthropic: "Anthropic",
  gemini: "Gemini",
};

const STATUS_COLOR: Record<string, string> = {
  active: "green",
  testing: "gold",
  disabled: "default",
};

const HEALTH_STATE_ICON: Record<string, React.ReactNode> = {
  healthy: <CheckCircleOutlined />,
  degraded: <WarningOutlined />,
  open: <DisconnectOutlined />,
  half_open: <ExperimentOutlined />,
};

// ── Main page ────────────────────────────────────────────────────────

export default function AIGatewayProviderDetailPage() {
  const { id = "" } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { t: tc } = useTranslation("common");
  const queryClient = useQueryClient();

  const [editOpen, setEditOpen] = useState(false);

  // ── Queries ──────────────────────────────────────────────────────────
  const providerQ = useQuery({
    queryKey: ["aigateway", "provider", id],
    queryFn: () => getProvider(id),
    enabled: !!id,
  });

  const healthQ = useQuery({
    queryKey: ["aigateway", "provider-health", id],
    queryFn: () => getProviderHealth(id),
    enabled: !!id,
  });

  const modelsQ = useQuery({
    queryKey: ["aigateway", "models", "provider-detail"],
    queryFn: () => collectAllPages((params) => listModels(params)),
  });

  // ── Mutations ───────────────────────────────────────────────────────
  const updateMut = useMutation({
    mutationFn: (body: ProviderUpdate) => updateProvider(id, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["aigateway", "provider", id] });
      message.success(tc("aigateway.updated"));
      setEditOpen(false);
    },
    onError: () => message.error(tc("aigateway.updateFailed")),
  });

  const deleteMut = useMutation({
    mutationFn: () => deleteProvider(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["aigateway", "providers"] });
      message.success(tc("aigateway.deleted"));
      navigate("/ai/providers");
    },
    onError: () => message.error(tc("aigateway.deleteFailed")),
  });

  const testMut = useMutation({
    mutationFn: () => testProvider(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["aigateway", "provider-health", id] });
    },
  });

  const provider = providerQ.data;
  const health = healthQ.data;
  const providerModels = (modelsQ.data ?? []).filter((m) => m.providerId === id);

  if (providerQ.error || (!providerQ.isLoading && !provider)) {
    return (
      <ResultEmpty
        title={tc("aigateway.notFound")}
        extra={<Link to="/ai/providers">← {tc("backToList", "Back to providers")}</Link>}
      />
    );
  }

  return (
    <ResourceFormPage
      title={provider?.displayName || provider?.name || id}
      description={
        provider ? (PROVIDER_TYPE_LABEL[provider.providerType] ?? provider.providerType) : ""
      }
      loading={providerQ.isLoading}
      saving={updateMut.isPending}
      noForm
      onCancel={() => navigate("/ai/providers")}
      extra={
        provider ? (
          <>
            <Tag color={STATUS_COLOR[provider.status] ?? "default"}>{provider.status}</Tag>
            <PermissionGuard action="aigateway-provider:write" fallback={null}>
              <Button
                icon={<ExperimentOutlined />}
                loading={testMut.isPending}
                onClick={() => testMut.mutate()}
              >
                {tc("aigateway.testConnectivity")}
              </Button>
            </PermissionGuard>
            <PermissionGuard action="aigateway-provider:write" fallback={null}>
              <Button icon={<EditOutlined />} onClick={() => setEditOpen(true)}>
                {tc("edit", "Edit")}
              </Button>
            </PermissionGuard>
            <PermissionGuard action="aigateway-provider:delete" fallback={null}>
              <Popconfirm
                title={tc("aigateway.confirmDelete")}
                description={tc("aigateway.confirmDeleteBody")}
                okText={tc("delete", "Delete")}
                okButtonProps={{ danger: true }}
                cancelText={tc("cancel", "Cancel")}
                onConfirm={() => deleteMut.mutate()}
              >
                <Button danger icon={<DeleteOutlined />}>
                  {tc("delete", "Delete")}
                </Button>
              </Popconfirm>
            </PermissionGuard>
          </>
        ) : null
      }
    >
      {provider ? (
        <div style={{ display: "flex", flexDirection: "column", gap: 24 }}>
          {/* ── Top row: Info grid + Health card ────────────────────── */}
          <Row gutter={24}>
            <Col span={16}>
              <ProviderInfoGrid provider={provider} />
            </Col>
            <Col span={8}>
              <HealthCard
                health={health}
                loading={healthQ.isLoading}
                testResult={testMut.data}
                testLoading={testMut.isPending}
              />
            </Col>
          </Row>

          {/* ── Connected models ─────────────────────────────────────── */}
          <Card
            title={
              <Space>
                <ApiOutlined />
                <span>
                  {tc("aigateway.connectedModels")}
                  {` `}
                  <Text type="secondary">({providerModels.length})</Text>
                </span>
              </Space>
            }
            size="small"
          >
            {modelsQ.isLoading ? (
              <Spin />
            ) : providerModels.length === 0 ? (
              <Empty description={tc("aigateway.noModels")} />
            ) : (
              <ModelTable models={providerModels} />
            )}
          </Card>

          {/* ── Edit modal ────────────────────────────────────────────── */}
          <EditModal
            open={editOpen}
            initial={provider}
            loading={updateMut.isPending}
            onCancel={() => setEditOpen(false)}
            onOk={(body) => updateMut.mutateAsync(body).then(() => undefined)}
          />
        </div>
      ) : null}
    </ResourceFormPage>
  );
}

// =============================================================================
// Sub components
// =============================================================================

function ResultEmpty({ title, extra }: { title: string; extra?: React.ReactNode }) {
  return (
    <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={title}>
      {extra}
    </Empty>
  );
}

function ProviderInfoGrid({ provider }: { provider: ProviderResponse }) {
  const { t: tc } = useTranslation("common");
  return (
    <FormSection title={tc("aigateway.providerConfiguration")}>
      <Descriptions column={{ xs: 1, sm: 2 }} size="small" bordered>
        <Descriptions.Item label={tc("aigateway.name")}>
          <Space>
            <strong>{provider.name}</strong>
            {provider.displayName && <Text type="secondary">({provider.displayName})</Text>}
          </Space>
        </Descriptions.Item>
        <Descriptions.Item label={tc("aigateway.providerType")}>
          <Tag color="blue">
            {PROVIDER_TYPE_LABEL[provider.providerType] ?? provider.providerType}
          </Tag>
        </Descriptions.Item>

        <Descriptions.Item label={tc("aigateway.preferredProtocol")}>
          <Tag>
            {PROTOCOL_LABEL[provider.preferredProtocol ?? ""] ?? provider.preferredProtocol ?? "—"}
          </Tag>
        </Descriptions.Item>
        <Descriptions.Item label={tc("aigateway.supportedProtocols")}>
          {provider.supportedProtocols && provider.supportedProtocols.length > 0 ? (
            <Space wrap>
              {provider.supportedProtocols.map((p) => (
                <Tag key={p} color="cyan">
                  {PROTOCOL_LABEL[p] ?? p}
                </Tag>
              ))}
            </Space>
          ) : (
            <Text type="secondary">—</Text>
          )}
        </Descriptions.Item>

        <Descriptions.Item label={tc("aigateway.baseUrl")} span={{ xs: 1, sm: 2 }}>
          <AntLink href={provider.baseUrl} target="_blank" rel="noreferrer">
            <LinkOutlined /> {provider.baseUrl}
          </AntLink>
        </Descriptions.Item>

        <Descriptions.Item label={tc("aigateway.apiKeyFingerprint")}>
          <Space>
            <KeyOutlined />
            <Text code>{provider.apiKeyFingerprint}</Text>
          </Space>
        </Descriptions.Item>
        <Descriptions.Item label={tc("aigateway.status")}>
          <Tag color={STATUS_COLOR[provider.status] ?? "default"}>{provider.status}</Tag>
        </Descriptions.Item>

        <Descriptions.Item label={tc("aigateway.timeoutMs")}>
          <Space>
            <ClockCircleOutlined /> {provider.requestTimeoutMs}
          </Space>
        </Descriptions.Item>
        <Descriptions.Item label={tc("aigateway.maxRetries")}>
          <Space>
            <ThunderboltOutlined /> {provider.maxRetries}
          </Space>
        </Descriptions.Item>

        <Descriptions.Item label={tc("aigateway.priority")}>
          <Space>
            <SettingOutlined /> {provider.priority}
          </Space>
        </Descriptions.Item>
        <Descriptions.Item label={tc("aigateway.weight")}>
          <Space>
            <CloudServerOutlined /> {provider.weight}
          </Space>
        </Descriptions.Item>

        <Descriptions.Item label={tc("aigateway.rpmLimit")}>
          {provider.rpmLimit !== null && provider.rpmLimit !== undefined
            ? `${provider.rpmLimit} / min`
            : "—"}
        </Descriptions.Item>
        <Descriptions.Item label={tc("aigateway.tpmLimit")}>
          {provider.tpmLimit !== null && provider.tpmLimit !== undefined
            ? `${provider.tpmLimit} / min`
            : "—"}
        </Descriptions.Item>

        <Descriptions.Item label={tc("createdAt")}>
          {formatDate(provider.createdAt)}
        </Descriptions.Item>
        <Descriptions.Item label={tc("updatedAt")}>
          {formatDate(provider.updatedAt)}
        </Descriptions.Item>
      </Descriptions>
    </FormSection>
  );
}

function HealthCard({
  health,
  loading,
  testResult,
  testLoading,
}: {
  health: ProviderHealthResponse | undefined;
  loading: boolean;
  testResult: ProviderTestResult | undefined;
  testLoading: boolean;
}) {
  const { t: tc } = useTranslation("common");
  if (loading || testLoading) {
    return (
      <FormSection title={tc("nav.aiGateway.health")}>
        <Spin />
      </FormSection>
    );
  }

  const state = testResult
    ? testResult.reachable
      ? "healthy"
      : "open"
    : (health?.state ?? "unknown");

  return (
    <FormSection title={tc("aigateway.circuitBreaker")}>
      <Row gutter={16}>
        <Col span={12}>
          <Statistic
            title={tc("aigateway.state")}
            value={state}
            prefix={HEALTH_STATE_ICON[state]}
            styles={{ content: { color: healthStateColor(state) } }}
          />
        </Col>
        <Col span={12}>
          <Statistic
            title={tc("aigateway.consecutiveFailures")}
            value={health?.consecutiveFailures ?? 0}
            suffix="/ 5"
          />
        </Col>
      </Row>

      {testResult && (
        <Alert
          style={{ marginTop: 16 }}
          type={testResult.reachable ? "success" : "error"}
          showIcon
          title={
            testResult.reachable
              ? tc("aigateway.reachableLatency", { latency: testResult.latencyMs ?? "?" })
              : (testResult.error ?? tc("aigateway.unreachable"))
          }
        />
      )}

      {health?.openUntil && state === "open" && (
        <Text type="secondary" style={{ fontSize: 12 }}>
          {tc("aigateway.reentersHalfOpen", { date: formatDate(health.openUntil) })}
        </Text>
      )}
    </FormSection>
  );
}

function ModelTable({ models }: { models: ModelResponse[] }) {
  const { t: tc } = useTranslation("common");
  const columns: ColumnsType<ModelResponse> = [
    {
      title: tc("aigateway.providerModelName"),
      dataIndex: "providerModelName",
      render: (name: string, row) => (
        <Space orientation="vertical" size={0}>
          <strong>{name}</strong>
          {row.displayName && (
            <Text type="secondary" style={{ fontSize: 12 }}>
              {row.displayName}
            </Text>
          )}
        </Space>
      ),
    },
    {
      title: tc("aigateway.contextTokens"),
      dataIndex: "maxContextTokens",
      align: "right",
      render: (v: number | null) => v ?? "—",
    },
    {
      title: tc("aigateway.outputTokens"),
      dataIndex: "maxOutputTokens",
      align: "right",
      render: (v: number | null) => v ?? "—",
    },
    {
      title: tc("aigateway.status"),
      dataIndex: "status",
      render: (s: string) => <Tag color={s === "active" ? "green" : "default"}>{s}</Tag>,
    },
    {
      title: tc("createdAt"),
      dataIndex: "createdAt",
      render: (d: string) => formatDate(d),
    },
  ];

  return (
    <Table size="small" dataSource={models} columns={columns} rowKey="id" pagination={false} />
  );
}

function EditModal({
  open,
  initial,
  loading,
  onCancel,
  onOk,
}: {
  open: boolean;
  initial: ProviderResponse;
  loading: boolean;
  onCancel: () => void;
  onOk: (body: ProviderUpdate) => void | Promise<void>;
}) {
  const { t: tc } = useTranslation("common");
  const [form] = Form.useForm<ProviderUpdate>();

  useEffect(() => {
    if (open) {
      form.setFieldsValue({
        displayName: initial.displayName ?? "",
        requestTimeoutMs: initial.requestTimeoutMs,
        maxRetries: initial.maxRetries,
        priority: initial.priority,
        weight: initial.weight,
        rpmLimit: initial.rpmLimit,
        tpmLimit: initial.tpmLimit,
      });
    }
  }, [open, form, initial]);

  return (
    <Modal
      title={`${tc("edit")} · ${initial.name}`}
      open={open}
      confirmLoading={loading}
      onCancel={onCancel}
      onOk={async () => {
        const values = await form.validateFields();
        try {
          await onOk(values);
        } catch (error) {
          // mutation.onError 已提示；字段红字落在弹窗表单上，保留输入供重试。
          if (isHttpError(error)) {
            applyServerFieldErrors(form, error, buildServerFieldResolver<ProviderUpdate>(form), tc);
          }
        }
      }}
      destroyOnHidden
      width={520}
    >
      <Form form={form} layout="vertical">
        <Divider>{tc("aigateway.identitySection")}</Divider>
        <Form.Item name="displayName">
          <GlFormRow label={tc("aigateway.displayName")}>
            <Input placeholder={tc("aigateway.displayNamePlaceholder")} />
          </GlFormRow>
        </Form.Item>

        <Divider>{tc("aigateway.circuitAndRetry")}</Divider>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="requestTimeoutMs">
              <GlFormRow label={tc("aigateway.timeoutMs")}>
                <InputNumber min={1000} step={1000} style={{ width: "100%" }} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="maxRetries">
              <GlFormRow label={tc("aigateway.maxRetries")}>
                <InputNumber min={0} max={5} style={{ width: "100%" }} />
              </GlFormRow>
            </Form.Item>
          </Col>
        </Row>

        <Divider>{tc("aigateway.routingSection")}</Divider>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="priority">
              <GlFormRow label={tc("aigateway.priorityHintHighWins")}>
                <InputNumber min={0} step={10} style={{ width: "100%" }} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="weight">
              <GlFormRow label={tc("aigateway.weightHintLoadBalanced")}>
                <InputNumber min={1} step={1} style={{ width: "100%" }} />
              </GlFormRow>
            </Form.Item>
          </Col>
        </Row>

        <Divider>{tc("aigateway.quotaSection")}</Divider>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="rpmLimit">
              <GlFormRow label={tc("aigateway.rpmLimit")}>
                <InputNumber min={0} placeholder="∞" style={{ width: "100%" }} />
              </GlFormRow>
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="tpmLimit">
              <GlFormRow label={tc("aigateway.tpmLimit")}>
                <InputNumber min={0} placeholder="∞" style={{ width: "100%" }} />
              </GlFormRow>
            </Form.Item>
          </Col>
        </Row>
      </Form>
    </Modal>
  );
}

// =============================================================================
// Helpers
// =============================================================================

function formatDate(iso?: string | null): string {
  if (!iso) return "—";
  try {
    const d = new Date(iso);
    return d.toLocaleString();
  } catch {
    return iso;
  }
}

function healthStateColor(state: string): string {
  switch (state) {
    case "healthy":
      return "var(--gl-color-success-text)";
    case "degraded":
      return "var(--gl-color-warning)";
    case "open":
      return "var(--gl-color-error-text)";
    case "half_open":
      return "var(--gl-color-warning)";
    default:
      return "var(--gl-color-text-tertiary)";
  }
}
