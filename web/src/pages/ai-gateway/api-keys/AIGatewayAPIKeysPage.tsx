// =============================================================================
// AI Gateway — API Keys 列表页
// =============================================================================
//
// 对应后端 admin_chi.go: GET/POST /api/v1/ai/api-keys
//                      PATCH/DELETE /api/v1/ai/api-keys/{id}
//                      POST /api/v1/ai/api-keys/{id}/revoke
// 数据面 API Key（哈希存储），用于调用三协议面推理入口。
// 安全要点：plaintext 只在 Create 响应里返回一次。
// =============================================================================

import {
  Button,
  DatePicker,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Tag,
  Tooltip,
  message,
  Alert,
} from "antd";
import {
  DeleteOutlined,
  StopOutlined,
  EditOutlined,
  KeyOutlined,
  CopyOutlined,
  ExclamationCircleOutlined,
} from "@ant-design/icons";
import { useQuery, useMutation } from "@tanstack/react-query";
import type { ColumnsType } from "antd/es/table";
import { useTranslation } from "react-i18next";
import { useState } from "react";
import dayjs, { type Dayjs } from "dayjs";

import { useSessionStore } from "@/features/session/session-store";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { renderCrudListView } from "@/shared/ui/CrudListPage";
import { useCrudList } from "@/shared/hooks/useCrudList";
import {
  AI_GATEWAY_DEFAULT_PAGE_SIZE,
  matchSearch,
  collectAllPages,
  createAPIKey,
  deleteAPIKey,
  listAPIKeys,
  listAliases,
  revokeAPIKey,
  updateAPIKey,
  type APIKeyCreate,
  type APIKeyResponse,
  type APIKeyUpdate,
  type AliasResponse,
  type ListPageParams,
} from "@/shared/api/aigateway";
import { isHttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  localizedError,
} from "@/shared/lib/server-error";

const CURRENCY_OPTIONS = [
  { value: "USD", label: "USD ($)" },
  { value: "CNY", label: "CNY (¥)" },
  { value: "EUR", label: "EUR (€)" },
];

const APIKEY_STATUS_OPTIONS = [
  { value: "active", label: "Active" },
  { value: "disabled", label: "Disabled" },
];

export default function AIGatewayAPIKeysPage() {
  const { t: tc } = useTranslation("common");
  const account = useSessionStore((state) => state.account);

  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<APIKeyResponse | null>(null);
  const [createForm] = Form.useForm();
  const [editForm] = Form.useForm();

  // Create 成功后的 plaintext 展示 Modal
  const [keySuccessOpen, setKeySuccessOpen] = useState(false);
  const [createdPlaintext, setCreatedPlaintext] = useState<string | null>(null);

  // ── useCrudList（server 分页模式） ──
  const list = useCrudList<APIKeyResponse, ListPageParams, string>({
    buildQueryKey: (params) =>
      ["aigateway", "api-keys", params.page, params.pageSize, params.search ?? ""] as const,
    buildParams: ({ page, pageSize, search }) => ({ page, pageSize, search }),
    paginatedFn: async (params) => {
      const response = await listAPIKeys(params);
      const all = response?.items ?? [];
      // 后端列表无 keyword 参数，搜索只能客户端过滤（FE-C-14）；无搜索时保留
      // 服务端 total 以维持分页，搜索时以过滤后的条数作为 total。
      const searching = Boolean(params.search?.trim());
      const items = searching
        ? all.filter((it) => matchSearch(it, params.search, ["name", "keyPrefix"]))
        : all;
      return { items, total: searching ? items.length : (response?.page?.total ?? items.length) };
    },
    initialPageSize: AI_GATEWAY_DEFAULT_PAGE_SIZE,
    deleteFn: deleteAPIKey,
    invalidateKeysOnDelete: [["aigateway", "api-keys"]],
    onDeleteSuccess: () => message.success(tc("aigateway.apiKeyDeleted", "API key deleted")),
    enabled: account !== null,
  });

  // Alias 多选数据源
  const aliasOptionsQuery = useQuery({
    queryKey: ["aigateway", "aliases", "apikey-select-options"],
    queryFn: () => collectAllPages((params) => listAliases(params)),
    select: (data) =>
      (data ?? []).map((a: AliasResponse) => ({
        value: a.alias, // API Key 用 alias 字符串做 allow-list
        label: `${a.alias} (${a.displayName || "no label"})`,
      })),
    enabled: account !== null && (createOpen || editTarget !== null),
  });

  const revokeMutation = useMutation({
    mutationFn: (id: string) => revokeAPIKey(id),
    onSuccess: () => {
      list.refetch();
      message.success(tc("aigateway.revokeSuccess", "API key revoked"));
    },
    onError: (err) => message.error(localizedError(err, tc, "common:error")),
  });

  const createMutation = useMutation({
    mutationFn: (body: APIKeyCreate) => createAPIKey(body),
    onSuccess: (resp) => {
      list.refetch();
      setCreateOpen(false);
      createForm.resetFields();
      if (resp?.plaintext) {
        setCreatedPlaintext(resp.plaintext);
        setKeySuccessOpen(true);
      } else {
        message.success(tc("aigateway.apiKeyCreated", "API key created"));
      }
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(createForm, err, buildServerFieldResolver(createForm), tc);
      }
      message.error(localizedError(err, tc, "common:error"));
    },
  });

  const editMutation = useMutation({
    mutationFn: ({ id, body }: { id: string; body: APIKeyUpdate }) => updateAPIKey(id, body),
    onSuccess: () => {
      list.refetch();
      setEditTarget(null);
      editForm.resetFields();
      message.success(tc("aigateway.apiKeyUpdated", "API key updated"));
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(editForm, err, buildServerFieldResolver(editForm), tc);
      }
      message.error(localizedError(err, tc, "common:error"));
    },
  });

  const openCreate = () => setCreateOpen(true);

  const openEdit = (record: APIKeyResponse) => {
    setEditTarget(record);
  };

  const handleCreate = async () => {
    const values = await createForm.validateFields();
    const body: APIKeyCreate = {
      name: values.name,
      currency: values.currency ?? "USD",
    };
    if (values.allowedAliases && values.allowedAliases.length > 0) {
      body.allowedAliases = values.allowedAliases;
    }
    if (values.rpmLimit !== undefined && values.rpmLimit !== null) {
      body.rpmLimit = values.rpmLimit;
    }
    if (values.tpmLimit !== undefined && values.tpmLimit !== null) {
      body.tpmLimit = values.tpmLimit;
    }
    if (values.monthlyBudget !== undefined && values.monthlyBudget !== null) {
      body.monthlyBudget = values.monthlyBudget;
    }
    if (values.expiresAt) {
      // 只选日期时语义为「当天结束过期」，序列化回完整 RFC3339。
      body.expiresAt = (values.expiresAt as Dayjs).endOf("day").toISOString();
    }
    await createMutation.mutateAsync(body);
  };

  const handleEdit = async () => {
    if (!editTarget) return;
    const values = await editForm.validateFields();
    const body: APIKeyUpdate = {};
    if (values.name !== undefined) body.name = values.name;
    if (values.allowedAliases !== undefined) body.allowedAliases = values.allowedAliases;
    if (values.rpmLimit !== undefined) body.rpmLimit = values.rpmLimit;
    if (values.tpmLimit !== undefined) body.tpmLimit = values.tpmLimit;
    if (values.monthlyBudget !== undefined) body.monthlyBudget = values.monthlyBudget;
    if (values.currency !== undefined) body.currency = values.currency;
    if (values.expiresAt !== undefined) {
      // DatePicker 清空时为 null；未改动时回传原完整时间戳，保证 ISO 往返。
      body.expiresAt = values.expiresAt ? (values.expiresAt as Dayjs).toISOString() : null;
    }
    if (values.status !== undefined) body.status = values.status;
    await editMutation.mutateAsync({ id: editTarget.id, body });
  };

  const handleCopyKey = async () => {
    if (!createdPlaintext) return;
    try {
      await navigator.clipboard.writeText(createdPlaintext);
      message.success(tc("aigateway.keyCopied", "Key copied to clipboard"));
    } catch {
      message.error(tc("aigateway.copyFailed", "Copy failed — please copy manually"));
    }
  };

  const columns: ColumnsType<APIKeyResponse> = [
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
          <code style={{ fontSize: 12, color: "var(--gl-color-text-tertiary)" }}>
            {record.keyPrefix}…
          </code>
        </Space>
      ),
    },
    {
      title: tc("aigateway.allowedAliases", "Allowed Aliases"),
      dataIndex: "allowedAliases",
      key: "allowedAliases",
      render: (aliases: string[] | undefined) =>
        aliases && aliases.length > 0 ? (
          <Space size={[4, 4]} wrap>
            {aliases.map((a) => (
              <Tag key={a}>{a}</Tag>
            ))}
          </Space>
        ) : (
          <span style={{ color: "var(--gl-color-text-tertiary)" }}>
            {tc("aigateway.allAliases", "All aliases allowed")}
          </span>
        ),
    },
    {
      title: tc("aigateway.limits", "Limits"),
      key: "limits",
      width: 200,
      render: (_: unknown, record) => (
        <Space orientation="vertical" size={0} style={{ fontSize: 12 }}>
          {record.rpmLimit !== null && record.rpmLimit !== undefined && (
            <span>RPM: {record.rpmLimit.toLocaleString()}</span>
          )}
          {record.tpmLimit !== null && record.tpmLimit !== undefined && (
            <span>TPM: {record.tpmLimit.toLocaleString()}</span>
          )}
          {record.monthlyBudget !== null && record.monthlyBudget !== undefined && (
            <span>
              Budget: {record.currency ?? "USD"} {record.monthlyBudget.toFixed(2)}
            </span>
          )}
          {(record.rpmLimit === null || record.rpmLimit === undefined) &&
            (record.tpmLimit === null || record.tpmLimit === undefined) &&
            (record.monthlyBudget === null || record.monthlyBudget === undefined) && (
              <span style={{ color: "var(--gl-color-text-tertiary)" }}>
                {tc("aigateway.noLimits", "No limits set")}
              </span>
            )}
        </Space>
      ),
    },
    {
      title: tc("aigateway.status", "Status"),
      dataIndex: "status",
      key: "status",
      width: 100,
      filters: [
        { text: tc("aigateway.active", "Active"), value: "active" },
        { text: tc("aigateway.revoked", "Revoked"), value: "revoked" },
        { text: tc("aigateway.disabled", "Disabled"), value: "disabled" },
      ],
      onFilter: (value, record) => record.status === value,
      render: (s: string) => {
        const color = s === "active" ? "green" : s === "revoked" ? "red" : "default";
        return <Tag color={color}>{s}</Tag>;
      },
    },
    {
      title: tc("aigateway.expiresAt", "Expires"),
      dataIndex: "expiresAt",
      key: "expiresAt",
      width: 160,
      render: (v: string | null | undefined) =>
        v ? new Date(v).toLocaleDateString() : tc("aigateway.never", "Never"),
    },
    {
      title: tc("actions", "Actions"),
      key: "actions",
      width: 240,
      fixed: "right",
      render: (_: unknown, record) => (
        <Space size={4}>
          <PermissionGuard action="aigateway-api-key:write" fallback={null}>
            <Tooltip title={tc("edit", "Edit")}>
              <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(record)} />
            </Tooltip>
          </PermissionGuard>
          {record.status === "active" && (
            <PermissionGuard action="aigateway-api-key:write" fallback={null}>
              <Tooltip title={tc("aigateway.revoke", "Revoke")}>
                <Popconfirm
                  title={tc("aigateway.confirmRevokeKey")}
                  description={tc("aigateway.confirmRevokeKeyDesc")}
                  okText={tc("aigateway.revoke", "Revoke")}
                  cancelText={tc("cancel")}
                  okButtonProps={{ danger: true, loading: revokeMutation.isPending }}
                  onConfirm={() => revokeMutation.mutate(record.id)}
                >
                  <Button size="small" danger icon={<StopOutlined />} />
                </Popconfirm>
              </Tooltip>
            </PermissionGuard>
          )}
          <PermissionGuard action="aigateway-api-key:delete" fallback={null}>
            <Tooltip title={tc("delete", "Delete")}>
              <Popconfirm
                title={tc("aigateway.confirmDeleteKey")}
                description={tc("aigateway.confirmDeleteKeyDesc")}
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
      {renderCrudListView<APIKeyResponse, ListPageParams, string>(
        {
          title: tc("nav.aiGateway.apiKeys", "API Keys"),
          description: tc(
            "aigateway.apiKeysDescription",
            "Data-plane API keys for calling the AI Gateway inference faces. Keys are stored as SHA-256 hashes — the plaintext appears only once on creation.",
          ),
          searchPlaceholder: tc("aigateway.searchAPIKeys", "Search API keys..."),
          rowKey: "id",
          enableBulkDelete: true,
          bulkDeletePermission: {
            permission: "aigateway-api-key:delete",
            resource: "aigateway-api-keys",
          },
          toolbar: (
            <PermissionGuard action="aigateway-api-key:write" fallback={null}>
              <Button type="primary" icon={<KeyOutlined />} onClick={() => openCreate()}>
                {tc("aigateway.createAPIKey", "New API Key")}
              </Button>
            </PermissionGuard>
          ),
          columns,
        },
        list,
      )}

      {/* Create Modal */}
      <Modal
        title={tc("aigateway.createAPIKey", "New API Key")}
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
        <Form
          form={createForm}
          layout="vertical"
          preserve={false}
          initialValues={{ currency: "USD", allowedAliases: [] }}
        >
          <Form.Item
            label={tc("aigateway.name", "Name")}
            name="name"
            rules={[
              {
                required: true,
                message: tc("aigateway.apiKeyNameRequired", "Enter a key name"),
              },
            ]}
          >
            <Input
              placeholder={tc("aigateway.apiKeyNamePlaceholder", "e.g. client-a-production")}
            />
          </Form.Item>
          <Form.Item
            label={tc("aigateway.allowedAliases", "Allowed aliases")}
            name="allowedAliases"
            tooltip={tc(
              "aigateway.allowedAliasesHint",
              "Leave empty to allow all aliases. Select specific ones to scope this key.",
            )}
          >
            <Select
              mode="multiple"
              showSearch
              placeholder={tc("aigateway.selectAliases", "Pick aliases...")}
              loading={aliasOptionsQuery.isLoading}
              options={aliasOptionsQuery.data ?? []}
              filterOption={(input, option) =>
                (option?.label ?? "").toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
          <Space.Compact style={{ width: "100%" }}>
            <Form.Item
              label={tc("aigateway.rpmLimit", "RPM limit")}
              name="rpmLimit"
              style={{ width: "50%" }}
            >
              <InputNumber
                min={0}
                step={1}
                style={{ width: "100%" }}
                placeholder={tc("aigateway.leaveBlankUnlimited", "Unlimited")}
              />
            </Form.Item>
            <Form.Item
              label={tc("aigateway.tpmLimit", "TPM limit")}
              name="tpmLimit"
              style={{ width: "50%" }}
            >
              <InputNumber
                min={0}
                step={100}
                style={{ width: "100%" }}
                placeholder={tc("aigateway.leaveBlankUnlimited", "Unlimited")}
              />
            </Form.Item>
          </Space.Compact>
          <Space.Compact style={{ width: "100%" }}>
            <Form.Item
              label={tc("aigateway.monthlyBudget", "Monthly budget")}
              name="monthlyBudget"
              style={{ width: "60%" }}
            >
              <InputNumber
                min={0}
                step={0.01}
                style={{ width: "100%" }}
                placeholder={tc("aigateway.leaveBlankUnlimited", "Unlimited")}
              />
            </Form.Item>
            <Form.Item
              label={tc("aigateway.currency", "Currency")}
              name="currency"
              style={{ width: "40%" }}
            >
              <Select options={CURRENCY_OPTIONS} />
            </Form.Item>
          </Space.Compact>
          <Form.Item label={tc("aigateway.expiresAt", "Expires at")} name="expiresAt">
            <DatePicker
              style={{ width: "100%" }}
              placeholder={tc("aigateway.expiresAtPlaceholder", "Leave blank for never")}
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* Edit Modal */}
      <Modal
        title={tc("aigateway.editAPIKey", "Edit API Key")}
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
              allowedAliases: editTarget.allowedAliases ?? [],
              rpmLimit: editTarget.rpmLimit ?? undefined,
              tpmLimit: editTarget.tpmLimit ?? undefined,
              monthlyBudget: editTarget.monthlyBudget ?? undefined,
              currency: editTarget.currency ?? "USD",
              expiresAt: editTarget.expiresAt ? dayjs(editTarget.expiresAt) : undefined,
              status: editTarget.status,
            }}
          >
            {/* key prefix — read-only display */}
            <Form.Item label={tc("aigateway.keyPrefix", "Key prefix")}>
              <code style={{ fontSize: 13 }}>{editTarget.keyPrefix}…</code>
            </Form.Item>
            <Form.Item
              label={tc("aigateway.name", "Name")}
              name="name"
              rules={[
                {
                  required: true,
                  message: tc("aigateway.apiKeyNameRequired", "Enter a key name"),
                },
              ]}
            >
              <Input />
            </Form.Item>
            <Form.Item
              label={tc("aigateway.allowedAliases", "Allowed aliases")}
              name="allowedAliases"
            >
              <Select
                mode="multiple"
                showSearch
                placeholder={tc("aigateway.selectAliases", "Pick aliases...")}
                loading={aliasOptionsQuery.isLoading}
                options={aliasOptionsQuery.data ?? []}
                filterOption={(input, option) =>
                  (option?.label ?? "").toLowerCase().includes(input.toLowerCase())
                }
              />
            </Form.Item>
            <Space.Compact style={{ width: "100%" }}>
              <Form.Item
                label={tc("aigateway.rpmLimit", "RPM limit")}
                name="rpmLimit"
                style={{ width: "50%" }}
              >
                <InputNumber
                  min={0}
                  step={1}
                  style={{ width: "100%" }}
                  placeholder={tc("aigateway.leaveBlankUnlimited", "Unlimited")}
                />
              </Form.Item>
              <Form.Item
                label={tc("aigateway.tpmLimit", "TPM limit")}
                name="tpmLimit"
                style={{ width: "50%" }}
              >
                <InputNumber
                  min={0}
                  step={100}
                  style={{ width: "100%" }}
                  placeholder={tc("aigateway.leaveBlankUnlimited", "Unlimited")}
                />
              </Form.Item>
            </Space.Compact>
            <Space.Compact style={{ width: "100%" }}>
              <Form.Item
                label={tc("aigateway.monthlyBudget", "Monthly budget")}
                name="monthlyBudget"
                style={{ width: "60%" }}
              >
                <InputNumber
                  min={0}
                  step={0.01}
                  style={{ width: "100%" }}
                  placeholder={tc("aigateway.leaveBlankUnlimited", "Unlimited")}
                />
              </Form.Item>
              <Form.Item
                label={tc("aigateway.currency", "Currency")}
                name="currency"
                style={{ width: "40%" }}
              >
                <Select options={CURRENCY_OPTIONS} />
              </Form.Item>
            </Space.Compact>
            <Form.Item
              label={tc("aigateway.expiresAt", "Expires at")}
              name="expiresAt"
              extra={tc("aigateway.expiresAtEditHint", "Leave blank to clear expiration.")}
            >
              <DatePicker showTime style={{ width: "100%" }} />
            </Form.Item>
            <Form.Item label={tc("aigateway.status", "Status")} name="status">
              <Select options={APIKEY_STATUS_OPTIONS} />
            </Form.Item>
          </Form>
        )}
      </Modal>

      {/* Create 成功后展示 plaintext key 的 Modal */}
      <Modal
        title={
          <>
            <KeyOutlined style={{ marginRight: 8 }} />
            {tc("aigateway.apiKeyCreated", "API key created")}
          </>
        }
        open={keySuccessOpen}
        onCancel={() => {
          setKeySuccessOpen(false);
          setCreatedPlaintext(null);
        }}
        footer={[
          <Button key="copy" type="primary" icon={<CopyOutlined />} onClick={handleCopyKey}>
            {tc("aigateway.copyKey", "Copy key")}
          </Button>,
          <Button
            key="close"
            onClick={() => {
              setKeySuccessOpen(false);
              setCreatedPlaintext(null);
            }}
          >
            {tc("close", "Close")}
          </Button>,
        ]}
        destroyOnHidden
        width={560}
      >
        <Alert
          type="warning"
          showIcon
          icon={<ExclamationCircleOutlined />}
          title={tc(
            "aigateway.keyWarnNeverShowAgain",
            "This key will never be shown again. Copy it now.",
          )}
          style={{ marginBottom: 16 }}
        />
        {createdPlaintext && (
          <div
            style={{
              background: "#001529",
              color: "#d6deeb",
              padding: "16px 20px",
              borderRadius: 6,
              fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
              fontSize: 13,
              wordBreak: "break-all",
              userSelect: "all",
            }}
          >
            {createdPlaintext}
          </div>
        )}
      </Modal>
    </>
  );
}
