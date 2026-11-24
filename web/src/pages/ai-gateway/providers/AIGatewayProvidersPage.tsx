// =============================================================================
// AI Gateway — Providers 列表页
// =============================================================================
//
// 对应后端 admin_chi.go: GET/POST /api/v1/ai/providers + PATCH/DELETE /api/v1/ai/providers/{id}
// 完整 CRUD：Create Modal + Edit Modal + Delete + Test Connectivity Modal。
// =============================================================================

import { Button, Form, Input, Modal, Popconfirm, Select, Space, Tag, Tooltip, message } from "antd";
import {
  PlusOutlined,
  DeleteOutlined,
  ApiOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  EditOutlined,
  KeyOutlined,
  LinkOutlined,
} from "@ant-design/icons";
import { useMutation } from "@tanstack/react-query";
import type { ColumnsType } from "antd/es/table";
import { useTranslation } from "react-i18next";
import { useState } from "react";

import { useSessionStore } from "@/features/session/session-store";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { renderCrudListView } from "@/shared/ui/CrudListPage";
import { useCrudList } from "@/shared/hooks/useCrudList";
import {
  AI_GATEWAY_DEFAULT_PAGE_SIZE,
  matchSearch,
  createProvider,
  deleteProvider,
  listProviders,
  testProvider,
  updateProvider,
  type ListPageParams,
  type ProviderCreate,
  type ProviderResponse,
  type ProviderTestResult,
  type ProviderUpdate,
} from "@/shared/api/aigateway";
import { isHttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  localizedError,
} from "@/shared/lib/server-error";

const PROVIDER_TYPES = [
  { value: "openai", label: "OpenAI" },
  { value: "anthropic", label: "Anthropic" },
  { value: "gemini", label: "Google Gemini" },
  { value: "deepseek", label: "DeepSeek" },
  { value: "glm", label: "Zhipu GLM" },
  { value: "kimi", label: "Kimi / Moonshot" },
  { value: "qwen", label: "Alibaba Qwen" },
  { value: "seed", label: "ByteDance Seed" },
  { value: "ollama", label: "Ollama (local)" },
  { value: "vllm", label: "vLLM (local)" },
  { value: "custom", label: "Custom / BYO" },
];

const PROTOCOLS = [
  { value: "openai", label: "OpenAI-compatible" },
  { value: "anthropic", label: "Anthropic native" },
  { value: "gemini", label: "Gemini native" },
];

const STATUS_OPTIONS = [
  { value: "active", label: "Active" },
  { value: "disabled", label: "Disabled" },
  { value: "testing", label: "Testing" },
];

/** 批量启停按钮组 — 文案与回调由调用方传入，避免在页面组件内定义内嵌组件。 */
function renderBulkEnableActions({
  enableLabel,
  disableLabel,
  onEnable,
  onDisable,
  loading,
}: {
  enableLabel: string;
  disableLabel: string;
  onEnable: () => void;
  onDisable: () => void;
  loading?: boolean;
}) {
  return (
    <PermissionGuard action="aigateway-provider:write" fallback={null}>
      <Space size="small">
        <Button size="small" onClick={onEnable} loading={loading} disabled={loading}>
          {enableLabel}
        </Button>
        <Button size="small" onClick={onDisable} loading={loading} disabled={loading}>
          {disableLabel}
        </Button>
      </Space>
    </PermissionGuard>
  );
}

export default function AIGatewayProvidersPage() {
  const { t: tc } = useTranslation("common");
  const account = useSessionStore((state) => state.account);

  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<ProviderResponse | null>(null);
  const [createForm] = Form.useForm();
  const [editForm] = Form.useForm();

  const [testModalOpen, setTestModalOpen] = useState(false);
  const [testResult, setTestResult] = useState<ProviderTestResult | null>(null);
  const [testingId, setTestingId] = useState<string | null>(null);
  const [bulkLoading, setBulkLoading] = useState(false);

  // ── useCrudList（server 分页模式） ──
  const list = useCrudList<ProviderResponse, ListPageParams, string>({
    buildQueryKey: (params) =>
      ["aigateway", "providers", params.page, params.pageSize, params.search ?? ""] as const,
    buildParams: ({ page, pageSize, search }) => ({ page, pageSize, search }),
    paginatedFn: async (params) => {
      const response = await listProviders(params);
      const all = response?.items ?? [];
      // 后端列表无 keyword 参数，搜索只能客户端过滤（FE-C-14）；无搜索时保留
      // 服务端 total 以维持分页，搜索时以过滤后的条数作为 total。
      const searching = Boolean(params.search?.trim());
      const items = searching
        ? all.filter((it) =>
            matchSearch(it, params.search, ["name", "displayName", "providerType"]),
          )
        : all;
      return { items, total: searching ? items.length : (response?.page?.total ?? items.length) };
    },
    initialPageSize: AI_GATEWAY_DEFAULT_PAGE_SIZE,
    deleteFn: deleteProvider,
    invalidateKeysOnDelete: [["aigateway", "providers"]],
    onDeleteSuccess: () => message.success(tc("aigateway.providerDeleted", "Provider deleted")),
    enabled: account !== null,
  });

  const createMutation = useMutation({
    mutationFn: (body: ProviderCreate) => createProvider(body),
    onSuccess: () => {
      list.refetch();
      setCreateOpen(false);
      createForm.resetFields();
      message.success(tc("aigateway.providerCreated", "Provider created"));
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(createForm, err, buildServerFieldResolver(createForm), tc);
      }
      message.error(localizedError(err, tc, "common:error"));
    },
  });

  const editMutation = useMutation({
    mutationFn: ({ id, body }: { id: string; body: ProviderUpdate }) => updateProvider(id, body),
    onSuccess: () => {
      list.refetch();
      setEditTarget(null);
      editForm.resetFields();
      message.success(tc("aigateway.providerUpdated", "Provider updated"));
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(editForm, err, buildServerFieldResolver(editForm), tc);
      }
      message.error(localizedError(err, tc, "common:error"));
    },
  });

  const testMutation = useMutation({
    mutationFn: (id: string) => testProvider(id),
    onMutate: (id) => {
      setTestingId(id);
      setTestResult(null);
      setTestModalOpen(true);
    },
    onSuccess: (result) => {
      setTestResult(result ?? null);
      setTestingId(null);
    },
    onError: () => {
      setTestingId(null);
      message.error(tc("aigateway.testFailed", "Provider test failed"));
    },
  });

  const openEdit = (record: ProviderResponse) => {
    setEditTarget(record);
  };

  const handleCreate = async () => {
    const values = await createForm.validateFields();
    await createMutation.mutateAsync({
      name: values.name,
      displayName: values.displayName,
      providerType: values.providerType,
      baseUrl: values.baseUrl,
      apiKey: values.apiKey,
      status: values.status ?? "active",
      supportedProtocols: values.supportedProtocols,
      preferredProtocol: values.preferredProtocol,
    });
  };

  const handleEdit = async () => {
    if (!editTarget) return;
    const values = await editForm.validateFields();
    const body: ProviderUpdate = {};
    if (values.name !== undefined) body.name = values.name;
    if (values.displayName !== undefined) body.displayName = values.displayName;
    if (values.providerType !== undefined) body.providerType = values.providerType;
    if (values.baseUrl !== undefined) body.baseUrl = values.baseUrl;
    if (values.apiKey && values.apiKey.trim()) {
      body.apiKey = values.apiKey.trim(); // 只有填了才更新
    }
    if (values.status !== undefined) body.status = values.status;
    if (values.supportedProtocols !== undefined)
      body.supportedProtocols = values.supportedProtocols;
    if (values.preferredProtocol !== undefined) body.preferredProtocol = values.preferredProtocol;
    await editMutation.mutateAsync({ id: editTarget.id, body });
  };

  const columns: ColumnsType<ProviderResponse> = [
    {
      title: tc("aigateway.name", "Name"),
      dataIndex: "name",
      key: "name",
      width: 200,
      fixed: "left",
      sorter: (a, b) => a.name.localeCompare(b.name),
      render: (name: string, record) => (
        <Space orientation="vertical" size={0}>
          <span style={{ fontWeight: 500 }}>{name}</span>
          {record.displayName && (
            <span style={{ color: "var(--gl-color-text-tertiary)", fontSize: 12 }}>
              {record.displayName}
            </span>
          )}
        </Space>
      ),
    },
    {
      title: tc("aigateway.providerType", "Type"),
      dataIndex: "providerType",
      key: "providerType",
      width: 130,
      render: (type: string) => (
        <Tag color="geekblue">{PROVIDER_TYPES.find((p) => p.value === type)?.label ?? type}</Tag>
      ),
    },
    {
      title: tc("aigateway.baseUrl", "Base URL"),
      dataIndex: "baseUrl",
      key: "baseUrl",
      width: 220,
      ellipsis: true,
      render: (url: string) => (
        <Tooltip title={url}>
          <code style={{ fontSize: 12 }}>{url}</code>
        </Tooltip>
      ),
    },
    {
      title: tc("aigateway.status", "Status"),
      dataIndex: "status",
      key: "status",
      width: 100,
      filters: [
        { text: tc("aigateway.active", "Active"), value: "active" },
        { text: tc("aigateway.disabled", "Disabled"), value: "disabled" },
      ],
      onFilter: (value, record) => record.status === value,
      render: (status: string) => {
        const color = status === "active" ? "green" : status === "disabled" ? "default" : "orange";
        return <Tag color={color}>{status}</Tag>;
      },
    },
    {
      title: tc("aigateway.apiKeyFingerprint", "API Key"),
      dataIndex: "apiKeyFingerprint",
      key: "apiKeyFingerprint",
      width: 140,
      render: (fp: string) => (
        <Tooltip title={tc("aigateway.apiKeyFingerprintHint", "Last 4 chars + hash")}>
          <code style={{ fontSize: 12, color: "var(--gl-color-text-tertiary)" }}>{fp}</code>
        </Tooltip>
      ),
    },
    {
      title: tc("aigateway.supportedProtocols", "Protocols"),
      dataIndex: "supportedProtocols",
      key: "supportedProtocols",
      width: 180,
      render: (protocols: string[] | undefined) => (
        <Space size={[4, 4]} wrap>
          {(protocols ?? []).map((p) => (
            <Tag key={p} color="blue">
              {p}
            </Tag>
          ))}
        </Space>
      ),
    },
    {
      title: tc("actions", "Actions"),
      key: "actions",
      width: 240,
      fixed: "right",
      render: (_: unknown, record) => (
        <Space size={4}>
          <PermissionGuard action="aigateway-provider:write" fallback={null}>
            <Tooltip title={tc("edit", "Edit")}>
              <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(record)} />
            </Tooltip>
          </PermissionGuard>
          <PermissionGuard action="aigateway-provider:write" fallback={null}>
            <Tooltip title={tc("aigateway.test", "Test connectivity")}>
              <Button
                size="small"
                icon={<ApiOutlined />}
                loading={testingId === record.id}
                onClick={() => testMutation.mutate(record.id)}
              />
            </Tooltip>
          </PermissionGuard>
          <PermissionGuard action="aigateway-provider:delete" fallback={null}>
            <Tooltip title={tc("delete", "Delete")}>
              <Popconfirm
                title={tc("aigateway.confirmDeleteProvider")}
                description={tc("aigateway.confirmDeleteProviderDesc")}
                okText={tc("delete")}
                cancelText={tc("cancel")}
                okButtonProps={{ danger: true, loading: list.deleteMutation.isPending }}
                onConfirm={() => list.deleteMutation.mutate(record.id)}
              >
                <Button size="small" danger icon={<DeleteOutlined />} />
              </Popconfirm>
            </Tooltip>
          </PermissionGuard>
        </Space>
      ),
    },
  ];

  return (
    <>
      {renderCrudListView<ProviderResponse, ListPageParams, string>(
        {
          title: tc("nav.aiGateway.providers", "Providers"),
          description: tc(
            "aigateway.providersDescription",
            "LLM vendor registrations. Configure API keys, base URLs, and protocol preferences.",
          ),
          searchPlaceholder: tc("aigateway.searchProviders", "Search providers..."),
          rowKey: "id",
          enableBulkDelete: true,
          bulkDeletePermission: {
            permission: "aigateway-provider:delete",
            resource: "aigateway-providers",
          },
          toolbar: (
            <PermissionGuard action="aigateway-provider:write" fallback={null}>
              <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
                {tc("aigateway.createProvider", "New Provider")}
              </Button>
            </PermissionGuard>
          ),
          columns,
          bulkActions: ({ selectedKeys, records }) => {
            const targets = records.filter((record) => selectedKeys.includes(record.id));
            const applyStatus = async (status: "active" | "disabled") => {
              const todo = targets.filter((record) => record.status !== status);
              if (todo.length === 0) return;
              const results = await Promise.allSettled(
                todo.map((record) => updateProvider(record.id, { status })),
              );
              // 部分失败也要 refetch：成功项的状态已变，列表不能停留在旧快照。
              list.refetch();
              const failures = results.flatMap((result, index) =>
                result.status === "rejected"
                  ? [`${todo[index].name}: ${localizedError(result.reason, tc, "common:error")}`]
                  : [],
              );
              if (failures.length > 0) {
                message.warning(
                  `${tc("bulkStatusPartial", {
                    succeeded: todo.length - failures.length,
                    failed: failures.length,
                    total: todo.length,
                  })} ${failures.join("; ")}`,
                );
              } else {
                message.success(tc("updateSuccess", "Updated successfully"));
              }
            };
            return renderBulkEnableActions({
              enableLabel: tc("action.enable", "Enable"),
              disableLabel: tc("action.disable", "Disable"),
              onEnable: () => {
                setBulkLoading(true);
                void applyStatus("active").finally(() => setBulkLoading(false));
              },
              onDisable: () => {
                setBulkLoading(true);
                void applyStatus("disabled").finally(() => setBulkLoading(false));
              },
              loading: bulkLoading,
            });
          },
        },
        list,
      )}

      {/* Create Modal */}
      <Modal
        title={tc("aigateway.createProvider", "New Provider")}
        open={createOpen}
        onCancel={() => {
          setCreateOpen(false);
          createForm.resetFields();
        }}
        onOk={() => handleCreate()}
        confirmLoading={createMutation.isPending}
        destroyOnHidden
        width={600}
      >
        <Form form={createForm} layout="vertical" preserve={false}>
          <Form.Item
            label={tc("aigateway.name", "Name")}
            name="name"
            rules={[
              { required: true, message: tc("aigateway.nameRequired", "Enter a provider name") },
            ]}
          >
            <Input placeholder={tc("aigateway.namePlaceholder", "e.g. deepseek-production")} />
          </Form.Item>
          <Form.Item label={tc("aigateway.displayName", "Display name")} name="displayName">
            <Input
              placeholder={tc(
                "aigateway.displayNamePlaceholder",
                "Human-friendly label (optional)",
              )}
            />
          </Form.Item>
          <Form.Item
            label={tc("aigateway.providerType", "Provider type")}
            name="providerType"
            rules={[
              { required: true, message: tc("aigateway.typeRequired", "Select a provider type") },
            ]}
          >
            <Select options={PROVIDER_TYPES} />
          </Form.Item>
          <Form.Item
            label={
              <>
                <LinkOutlined /> {tc("aigateway.baseUrl", "Base URL")}
              </>
            }
            name="baseUrl"
            rules={[
              { required: true, message: tc("aigateway.baseUrlRequired", "Enter base URL") },
              {
                validator: (_, value: string) => {
                  if (!value) return Promise.resolve();
                  if (/^https?:\/\//i.test(value)) return Promise.resolve();
                  return Promise.reject(
                    new Error(
                      tc("aigateway.baseUrlInvalid", "URL must start with http:// or https://"),
                    ),
                  );
                },
              },
            ]}
          >
            <Input placeholder="https://api.deepseek.com/v1" />
          </Form.Item>
          <Form.Item
            label={
              <>
                <KeyOutlined /> {tc("aigateway.apiKey", "API Key")}
              </>
            }
            name="apiKey"
            rules={[{ required: true, message: tc("aigateway.apiKeyRequired", "Enter API key") }]}
            extra={tc("aigateway.apiKeyHint", "Stored encrypted. Never shown again after save.")}
          >
            <Input.Password placeholder="sk-..." />
          </Form.Item>
          <Form.Item label={tc("aigateway.status", "Status")} name="status" initialValue="active">
            <Select options={STATUS_OPTIONS} />
          </Form.Item>
          <Form.Item
            label={tc("aigateway.supportedProtocols", "Supported protocols")}
            name="supportedProtocols"
          >
            <Select
              mode="multiple"
              options={PROTOCOLS}
              placeholder={tc("aigateway.selectProtocols", "Pick one or more...")}
            />
          </Form.Item>
          <Form.Item
            label={tc("aigateway.preferredProtocol", "Preferred protocol")}
            name="preferredProtocol"
          >
            <Select
              options={PROTOCOLS}
              placeholder={tc("aigateway.leaveBlankDefault", "Leave blank for default")}
              allowClear
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* Edit Modal */}
      <Modal
        title={tc("aigateway.editProvider", "Edit Provider")}
        open={editTarget !== null}
        onCancel={() => {
          setEditTarget(null);
          editForm.resetFields();
        }}
        onOk={() => handleEdit()}
        confirmLoading={editMutation.isPending}
        destroyOnHidden
        width={600}
      >
        {editTarget && (
          <Form
            form={editForm}
            layout="vertical"
            preserve={false}
            initialValues={{
              name: editTarget.name,
              displayName: editTarget.displayName,
              providerType: editTarget.providerType,
              baseUrl: editTarget.baseUrl,
              // apiKey intentionally omitted — 留空即不改
              status: editTarget.status,
              supportedProtocols: editTarget.supportedProtocols,
              preferredProtocol: editTarget.preferredProtocol,
            }}
          >
            <Form.Item
              label={tc("aigateway.name", "Name")}
              name="name"
              rules={[
                { required: true, message: tc("aigateway.nameRequired", "Enter a provider name") },
              ]}
            >
              <Input />
            </Form.Item>
            <Form.Item label={tc("aigateway.displayName", "Display name")} name="displayName">
              <Input />
            </Form.Item>
            <Form.Item
              label={tc("aigateway.providerType", "Provider type")}
              name="providerType"
              rules={[
                { required: true, message: tc("aigateway.typeRequired", "Select a provider type") },
              ]}
            >
              <Select options={PROVIDER_TYPES} />
            </Form.Item>
            <Form.Item
              label={
                <>
                  <LinkOutlined /> {tc("aigateway.baseUrl", "Base URL")}
                </>
              }
              name="baseUrl"
              rules={[
                { required: true, message: tc("aigateway.baseUrlRequired", "Enter base URL") },
                {
                  validator: (_, value: string) => {
                    if (!value) return Promise.resolve();
                    if (/^https?:\/\//i.test(value)) return Promise.resolve();
                    return Promise.reject(
                      new Error(
                        tc("aigateway.baseUrlInvalid", "URL must start with http:// or https://"),
                      ),
                    );
                  },
                },
              ]}
            >
              <Input />
            </Form.Item>
            <Form.Item
              label={
                <>
                  <KeyOutlined /> {tc("aigateway.apiKey", "API Key")}
                </>
              }
              name="apiKey"
              extra={tc(
                "aigateway.apiKeyEditHint",
                "Leave blank to keep the current key. Paste a new value only to rotate it.",
              )}
            >
              <Input.Password
                placeholder={tc("aigateway.leaveBlankUnchanged", "Leave blank to keep current")}
              />
            </Form.Item>
            <Form.Item label={tc("aigateway.status", "Status")} name="status">
              <Select options={STATUS_OPTIONS} />
            </Form.Item>
            <Form.Item
              label={tc("aigateway.supportedProtocols", "Supported protocols")}
              name="supportedProtocols"
            >
              <Select mode="multiple" options={PROTOCOLS} />
            </Form.Item>
            <Form.Item
              label={tc("aigateway.preferredProtocol", "Preferred protocol")}
              name="preferredProtocol"
            >
              <Select options={PROTOCOLS} allowClear />
            </Form.Item>
          </Form>
        )}
      </Modal>

      {/* Test Modal（保持原样） */}
      <Modal
        open={testModalOpen}
        title={tc("aigateway.testProvider", "Provider connectivity test")}
        footer={[
          <Button key="close" onClick={() => setTestModalOpen(false)}>
            {tc("close", "Close")}
          </Button>,
        ]}
        onCancel={() => setTestModalOpen(false)}
      >
        {testingId ? (
          <div style={{ textAlign: "center", padding: "24px 0" }}>
            <ApiOutlined style={{ fontSize: 32 }} spin />
            <p style={{ marginTop: 16 }}>{tc("aigateway.testing", "Testing connectivity...")}</p>
          </div>
        ) : testResult ? (
          <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
            <Space align="center" size={8}>
              {testResult.reachable ? (
                <CheckCircleOutlined style={{ color: "var(--gl-color-success)", fontSize: 20 }} />
              ) : (
                <CloseCircleOutlined style={{ color: "var(--gl-color-error)", fontSize: 20 }} />
              )}
              <strong>
                {testResult.reachable
                  ? tc("aigateway.testReachable", "Provider is reachable")
                  : tc("aigateway.testUnreachable", "Provider is NOT reachable")}
              </strong>
            </Space>
            <dl
              style={{
                display: "grid",
                gridTemplateColumns: "auto 1fr",
                gap: "4px 12px",
                margin: 0,
              }}
            >
              {testResult.latencyMs !== undefined && (
                <>
                  <dt>{tc("aigateway.latency", "Latency")}:</dt>
                  <dd>{testResult.latencyMs} ms</dd>
                </>
              )}
              {testResult.statusCode !== null && testResult.statusCode !== undefined && (
                <>
                  <dt>{tc("aigateway.statusCode", "Status code")}:</dt>
                  <dd>{testResult.statusCode}</dd>
                </>
              )}
              {testResult.error && (
                <>
                  <dt>{tc("aigateway.error", "Error")}:</dt>
                  <dd style={{ color: "var(--gl-color-error)" }}>{testResult.error}</dd>
                </>
              )}
            </dl>
          </div>
        ) : null}
      </Modal>
    </>
  );
}
