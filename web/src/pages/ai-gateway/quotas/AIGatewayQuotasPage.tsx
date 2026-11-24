// =============================================================================
// AI Gateway — Quotas 列表页
// =============================================================================
//
// 对应后端 admin_chi.go: GET/POST /api/v1/ai/quotas + PATCH/DELETE /api/v1/ai/quotas/{id}
// 配额策略（RPM/TPM/cost），scope 决定适用范围：
//   organization → 整组织
//   api_key → 特定 API Key（scopeRef = apiKeyId）
//   alias → 特定别名（scopeRef = alias）
// =============================================================================

import {
  Button,
  Form,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Tag,
  Tooltip,
  message,
} from "antd";
import { PlusOutlined, DeleteOutlined, EditOutlined } from "@ant-design/icons";
import { useQuery, useMutation } from "@tanstack/react-query";
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
  collectAllPages,
  createQuota,
  deleteQuota,
  listAPIKeys,
  listAliases,
  listQuotas,
  updateQuota,
  type APIKeyResponse,
  type AliasResponse,
  type ListPageParams,
  type QuotaCreate,
  type QuotaResponse,
  type QuotaUpdate,
} from "@/shared/api/aigateway";
import { isHttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  localizedError,
} from "@/shared/lib/server-error";

const SCOPE_OPTIONS = [
  { value: "organization", label: "Organization" },
  { value: "api_key", label: "API Key" },
  { value: "alias", label: "Alias" },
];

const LIMIT_TYPE_OPTIONS = [
  { value: "rpm", label: "RPM (Requests Per Minute)" },
  { value: "tpm", label: "TPM (Tokens Per Minute)" },
  { value: "cost", label: "Cost (monthly budget)" },
];

const RESET_CYCLE_OPTIONS = [
  { value: "daily", label: "Daily" },
  { value: "monthly", label: "Monthly" },
];

const STATUS_OPTIONS = [
  { value: "active", label: "Active" },
  { value: "disabled", label: "Disabled" },
];

const SCOPE_COLORS: Record<string, string> = {
  organization: "purple",
  api_key: "geekblue",
  alias: "cyan",
};

const LIMIT_TYPE_LABELS: Record<string, string> = {
  rpm: "RPM",
  tpm: "TPM",
  cost: "Cost",
};

const RESET_CYCLE_LABELS: Record<string, string> = {
  daily: "Daily",
  monthly: "Monthly",
};

export default function AIGatewayQuotasPage() {
  const { t: tc } = useTranslation("common");
  const account = useSessionStore((state) => state.account);

  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<QuotaResponse | null>(null);
  const [createForm] = Form.useForm();
  const [editForm] = Form.useForm();

  // scope 切换时重置 scopeRef
  const handleScopeChange = (_: string, scopeItem: { value: string }) => {
    createForm.setFieldsValue({ scopeRef: undefined });
    if (scopeItem.value === "organization") {
      createForm.setFieldsValue({ scopeRef: undefined });
    }
  };

  // ── useCrudList（server 分页模式） ──
  const list = useCrudList<QuotaResponse, ListPageParams, string>({
    buildQueryKey: (params) =>
      ["aigateway", "quotas", params.page, params.pageSize, params.search ?? ""] as const,
    buildParams: ({ page, pageSize, search }) => ({ page, pageSize, search }),
    paginatedFn: async (params) => {
      const response = await listQuotas(params);
      const all = response?.items ?? [];
      // 后端列表无 keyword 参数，搜索只能客户端过滤（FE-C-14）；无搜索时保留
      // 服务端 total 以维持分页，搜索时以过滤后的条数作为 total。
      const searching = Boolean(params.search?.trim());
      const items = searching
        ? all.filter((it) => matchSearch(it, params.search, ["scope", "scopeRef", "limitType"]))
        : all;
      return { items, total: searching ? items.length : (response?.page?.total ?? items.length) };
    },
    initialPageSize: AI_GATEWAY_DEFAULT_PAGE_SIZE,
    deleteFn: deleteQuota,
    invalidateKeysOnDelete: [["aigateway", "quotas"]],
    onDeleteSuccess: () => message.success(tc("aigateway.quotaDeleted", "Quota deleted")),
    enabled: account !== null,
  });

  // scopeRef 的两个懒加载数据源
  const aliasRefOptionsQuery = useQuery({
    queryKey: ["aigateway", "aliases", "quota-ref-options"],
    queryFn: () => collectAllPages((params) => listAliases(params)),
    select: (data) =>
      (data ?? []).map((a: AliasResponse) => ({
        value: a.alias,
        label: `${a.alias} (${a.displayName || "no label"})`,
      })),
    enabled: account !== null && (createOpen || editTarget !== null),
  });

  const apiKeyRefOptionsQuery = useQuery({
    queryKey: ["aigateway", "apikeys", "quota-ref-options"],
    queryFn: () => collectAllPages((params) => listAPIKeys(params)),
    select: (data) =>
      (data ?? []).map((k: APIKeyResponse) => ({
        value: k.id,
        label: `${k.name} (prefix ${k.keyPrefix}…)`,
      })),
    enabled: account !== null && (createOpen || editTarget !== null),
  });

  const createMutation = useMutation({
    mutationFn: (body: QuotaCreate) => createQuota(body),
    onSuccess: () => {
      list.refetch();
      setCreateOpen(false);
      createForm.resetFields();
      message.success(tc("aigateway.quotaCreated", "Quota created"));
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(createForm, err, buildServerFieldResolver(createForm), tc);
      }
      message.error(localizedError(err, tc, "common:error"));
    },
  });

  const editMutation = useMutation({
    mutationFn: ({ id, body }: { id: string; body: QuotaUpdate }) => updateQuota(id, body),
    onSuccess: () => {
      list.refetch();
      setEditTarget(null);
      editForm.resetFields();
      message.success(tc("aigateway.quotaUpdated", "Quota updated"));
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(editForm, err, buildServerFieldResolver(editForm), tc);
      }
      message.error(localizedError(err, tc, "common:error"));
    },
  });

  const openCreate = () => setCreateOpen(true);

  const openEdit = (record: QuotaResponse) => {
    setEditTarget(record);
  };

  const handleCreate = async () => {
    const values = await createForm.validateFields();
    const body: QuotaCreate = {
      scope: values.scope,
      limitType: values.limitType,
      limitValue: values.limitValue,
      resetCycle: values.resetCycle,
      status: values.status ?? "active",
    };
    if (values.scope !== "organization" && values.scopeRef) {
      body.scopeRef = values.scopeRef;
    }
    await createMutation.mutateAsync(body);
  };

  const handleEdit = async () => {
    if (!editTarget) return;
    const values = await editForm.validateFields();
    const body: QuotaUpdate = {};
    // 后端 ScopeRef 为 *string 且仅在非 nil 时赋值：JSON null 会反序列化成 nil
    // 而被跳过，唯有空串能真正清空，故清空时显式发 ""。
    body.scopeRef = values.scopeRef ?? "";
    if (values.limitType !== undefined) body.limitType = values.limitType;
    if (values.limitValue !== undefined) body.limitValue = values.limitValue;
    if (values.resetCycle !== undefined) body.resetCycle = values.resetCycle;
    if (values.status !== undefined) body.status = values.status;
    await editMutation.mutateAsync({ id: editTarget.id, body });
  };

  const columns: ColumnsType<QuotaResponse> = [
    {
      title: tc("aigateway.scope", "Scope"),
      dataIndex: "scope",
      key: "scope",
      width: 140,
      fixed: "left",
      sorter: (a, b) => a.scope.localeCompare(b.scope),
      filters: [
        { text: "Organization", value: "organization" },
        { text: "API Key", value: "api_key" },
        { text: "Alias", value: "alias" },
      ],
      onFilter: (v, record) => record.scope === v,
      render: (scope: string, record) => (
        <Space>
          <Tag color={SCOPE_COLORS[scope] ?? "default"}>{scope}</Tag>
          {record.scopeRef && (
            <Tooltip title={record.scopeRef}>
              <code style={{ fontSize: 12 }}>{record.scopeRef}</code>
            </Tooltip>
          )}
        </Space>
      ),
    },
    {
      title: tc("aigateway.limitType", "Limit Type"),
      dataIndex: "limitType",
      key: "limitType",
      width: 120,
      filters: [
        { text: "RPM", value: "rpm" },
        { text: "TPM", value: "tpm" },
        { text: "Cost", value: "cost" },
      ],
      onFilter: (v, record) => record.limitType === v,
      render: (t: string) => <Tag color="orange">{LIMIT_TYPE_LABELS[t] ?? t}</Tag>,
    },
    {
      title: tc("aigateway.limitValue", "Limit"),
      dataIndex: "limitValue",
      key: "limitValue",
      width: 140,
      align: "right",
      sorter: (a, b) => a.limitValue - b.limitValue,
      render: (v: number, record) =>
        record.limitType === "cost" ? v.toFixed(4) : v.toLocaleString(),
    },
    {
      title: tc("aigateway.resetCycle", "Reset"),
      dataIndex: "resetCycle",
      key: "resetCycle",
      width: 100,
      render: (c: string) => RESET_CYCLE_LABELS[c] ?? c,
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
      render: (s: string) => <Tag color={s === "active" ? "green" : "default"}>{s}</Tag>,
    },
    {
      title: tc("actions", "Actions"),
      key: "actions",
      width: 180,
      fixed: "right",
      render: (_: unknown, record) => (
        <Space size={4}>
          <PermissionGuard action="aigateway-quota:write" fallback={null}>
            <Tooltip title={tc("edit", "Edit")}>
              <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(record)} />
            </Tooltip>
          </PermissionGuard>
          <PermissionGuard action="aigateway-quota:delete" fallback={null}>
            <Tooltip title={tc("delete", "Delete")}>
              <Popconfirm
                title={tc("aigateway.confirmDeleteQuota")}
                description={tc("aigateway.confirmDeleteQuotaDesc")}
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
      {renderCrudListView<QuotaResponse, ListPageParams, string>(
        {
          title: tc("nav.aiGateway.quotas", "Quotas"),
          description: tc(
            "aigateway.quotasDescription",
            "Rate limits and budgets. Scope determines whether the limit applies to the whole org, a specific API key, or a single alias.",
          ),
          searchPlaceholder: tc("aigateway.searchQuotas", "Search quotas..."),
          rowKey: "id",
          enableBulkDelete: true,
          bulkDeletePermission: {
            permission: "aigateway-quota:delete",
            resource: "aigateway-quotas",
          },
          toolbar: (
            <PermissionGuard action="aigateway-quota:write" fallback={null}>
              <Button type="primary" icon={<PlusOutlined />} onClick={() => openCreate()}>
                {tc("aigateway.createQuota", "New Quota")}
              </Button>
            </PermissionGuard>
          ),
          columns,
        },
        list,
      )}

      {/* Create Modal */}
      <Modal
        title={tc("aigateway.createQuota", "New Quota")}
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
        <QuotaCreateForm
          form={createForm}
          aliasRefLoading={aliasRefOptionsQuery.isLoading}
          apiKeyRefLoading={apiKeyRefOptionsQuery.isLoading}
          aliasRefOptions={aliasRefOptionsQuery.data ?? []}
          apiKeyRefOptions={apiKeyRefOptionsQuery.data ?? []}
          onScopeChange={handleScopeChange}
        />
      </Modal>

      {/* Edit Modal */}
      <Modal
        title={tc("aigateway.editQuota", "Edit Quota")}
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
          <QuotaEditForm
            form={editForm}
            target={editTarget}
            aliasRefLoading={aliasRefOptionsQuery.isLoading}
            apiKeyRefLoading={apiKeyRefOptionsQuery.isLoading}
            aliasRefOptions={aliasRefOptionsQuery.data ?? []}
            apiKeyRefOptions={apiKeyRefOptionsQuery.data ?? []}
          />
        )}
      </Modal>
    </>
  );
}

// ── Create Form (scopeRef 根据 scope 显隐) ──────────────────────────────────

interface QuotaCreateFormProps {
  form: ReturnType<typeof Form.useForm>[0];
  aliasRefLoading: boolean;
  apiKeyRefLoading: boolean;
  aliasRefOptions: { value: string; label: string }[];
  apiKeyRefOptions: { value: string; label: string }[];
  onScopeChange: (value: string, option: { value: string }) => void;
}

function QuotaCreateForm({
  form,
  aliasRefLoading,
  apiKeyRefLoading,
  aliasRefOptions,
  apiKeyRefOptions,
  onScopeChange,
}: QuotaCreateFormProps) {
  const { t: tc } = useTranslation("common");
  const scope = Form.useWatch("scope", form);
  const showScopeRef = scope && scope !== "organization";
  const isApiKeyScope = scope === "api_key";

  // scopeRef required when scope != organization
  const scopeRefRules = [
    {
      validator: (_: unknown, value: unknown) => {
        if (scope && scope !== "organization") {
          if (value === undefined || value === null || value === "") {
            return Promise.reject(
              new Error(
                scope === "api_key"
                  ? "Select an API key to scope this quota to"
                  : "Select an alias to scope this quota to",
              ),
            );
          }
        }
        return Promise.resolve();
      },
    },
  ];

  return (
    <Form
      form={form}
      layout="vertical"
      preserve={false}
      initialValues={{
        scope: "organization",
        limitType: "rpm",
        resetCycle: "monthly",
        status: "active",
      }}
    >
      <Form.Item
        label={tc("aigateway.scope", "Scope")}
        name="scope"
        rules={[{ required: true, message: "Select a scope" }]}
      >
        <Select
          options={SCOPE_OPTIONS}
          onChange={(v, o) => onScopeChange(v as string, o as { value: string })}
        />
      </Form.Item>

      {showScopeRef && (
        <Form.Item
          label={tc("aigateway.scopeRef", "Scope target")}
          name="scopeRef"
          rules={scopeRefRules}
          tooltip={
            isApiKeyScope
              ? tc("aigateway.scopeRefApiKeyHint", "Specific API Key this limit applies to")
              : tc("aigateway.scopeRefAliasHint", "Specific alias this limit applies to")
          }
        >
          {isApiKeyScope ? (
            <Select
              showSearch
              placeholder={tc("aigateway.selectAPIKey", "Pick an API key...")}
              loading={apiKeyRefLoading}
              options={apiKeyRefOptions}
              filterOption={(input, option) =>
                (option?.label ?? "").toLowerCase().includes(input.toLowerCase())
              }
            />
          ) : (
            <Select
              showSearch
              placeholder={tc("aigateway.selectAlias", "Pick an alias...")}
              loading={aliasRefLoading}
              options={aliasRefOptions}
              filterOption={(input, option) =>
                (option?.label ?? "").toLowerCase().includes(input.toLowerCase())
              }
            />
          )}
        </Form.Item>
      )}

      <Form.Item
        label={tc("aigateway.limitType", "Limit type")}
        name="limitType"
        rules={[{ required: true, message: "Select a limit type" }]}
      >
        <Select options={LIMIT_TYPE_OPTIONS} />
      </Form.Item>
      <Form.Item
        label={tc("aigateway.limitValue", "Limit value")}
        name="limitValue"
        rules={[{ required: true, message: "Enter a limit value" }]}
      >
        <InputNumber min={0} step={0.01} style={{ width: "100%" }} />
      </Form.Item>
      <Form.Item
        label={tc("aigateway.resetCycle", "Reset cycle")}
        name="resetCycle"
        rules={[{ required: true, message: "Select a reset cycle" }]}
      >
        <Select options={RESET_CYCLE_OPTIONS} />
      </Form.Item>
      <Form.Item label={tc("aigateway.status", "Status")} name="status">
        <Select options={STATUS_OPTIONS} />
      </Form.Item>
    </Form>
  );
}

// ── Edit Form (scope LOCKED) ────────────────────────────────────────────────

interface QuotaEditFormProps {
  form: ReturnType<typeof Form.useForm>[0];
  target: QuotaResponse;
  aliasRefLoading: boolean;
  apiKeyRefLoading: boolean;
  aliasRefOptions: { value: string; label: string }[];
  apiKeyRefOptions: { value: string; label: string }[];
}

function QuotaEditForm({
  form,
  target,
  aliasRefLoading,
  apiKeyRefLoading,
  aliasRefOptions,
  apiKeyRefOptions,
}: QuotaEditFormProps) {
  const { t: tc } = useTranslation("common");
  const isApiKeyScope = target.scope === "api_key";
  const showScopeRef = target.scope !== "organization";

  return (
    <Form
      form={form}
      layout="vertical"
      preserve={false}
      initialValues={{
        scopeRef: target.scopeRef ?? undefined,
        limitType: target.limitType,
        limitValue: target.limitValue,
        resetCycle: target.resetCycle,
        status: target.status,
      }}
    >
      {/* scope — locked (read-only display) */}
      <Form.Item label={tc("aigateway.scope", "Scope")}>
        <Tag color={SCOPE_COLORS[target.scope] ?? "default"}>{target.scope}</Tag>
      </Form.Item>

      {showScopeRef && (
        <Form.Item label={tc("aigateway.scopeRef", "Scope target")} name="scopeRef">
          {isApiKeyScope ? (
            <Select
              showSearch
              placeholder={tc("aigateway.selectAPIKey", "Pick an API key...")}
              loading={apiKeyRefLoading}
              options={apiKeyRefOptions}
              filterOption={(input, option) =>
                (option?.label ?? "").toLowerCase().includes(input.toLowerCase())
              }
              allowClear
            />
          ) : (
            <Select
              showSearch
              placeholder={tc("aigateway.selectAlias", "Pick an alias...")}
              loading={aliasRefLoading}
              options={aliasRefOptions}
              filterOption={(input, option) =>
                (option?.label ?? "").toLowerCase().includes(input.toLowerCase())
              }
              allowClear
            />
          )}
        </Form.Item>
      )}

      <Form.Item
        label={tc("aigateway.limitType", "Limit type")}
        name="limitType"
        rules={[{ required: true, message: "Select a limit type" }]}
      >
        <Select options={LIMIT_TYPE_OPTIONS} />
      </Form.Item>
      <Form.Item
        label={tc("aigateway.limitValue", "Limit value")}
        name="limitValue"
        rules={[{ required: true, message: "Enter a limit value" }]}
      >
        <InputNumber min={0} step={0.01} style={{ width: "100%" }} />
      </Form.Item>
      <Form.Item
        label={tc("aigateway.resetCycle", "Reset cycle")}
        name="resetCycle"
        rules={[{ required: true, message: "Select a reset cycle" }]}
      >
        <Select options={RESET_CYCLE_OPTIONS} />
      </Form.Item>
      <Form.Item label={tc("aigateway.status", "Status")} name="status">
        <Select options={STATUS_OPTIONS} />
      </Form.Item>
    </Form>
  );
}
