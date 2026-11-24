// =============================================================================
// AI Gateway — Models 列表页
// =============================================================================
//
// 对应后端 admin_chi.go: GET/POST /api/v1/ai/models + PATCH/DELETE /api/v1/ai/models/{id}
// 物理模型（供应商侧实际存在的模型），由 Provider 管理。
// 完整 CRUD：Create Modal + Edit Modal + Delete。
// =============================================================================

import {
  Button,
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
  createModel,
  deleteModel,
  listModels,
  listProviders,
  updateModel,
  type ListPageParams,
  type ModelCreate,
  type ModelResponse,
  type ModelUpdate,
  type ProviderResponse,
} from "@/shared/api/aigateway";
import { isHttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  localizedError,
} from "@/shared/lib/server-error";

const STATUS_OPTIONS = [
  { value: "active", label: "Active" },
  { value: "disabled", label: "Disabled" },
];

export default function AIGatewayModelsPage() {
  const { t: tc } = useTranslation("common");
  const account = useSessionStore((state) => state.account);

  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<ModelResponse | null>(null);
  const [createForm] = Form.useForm();
  const [editForm] = Form.useForm();

  // ── useCrudList（server 分页模式） ──
  const list = useCrudList<ModelResponse, ListPageParams, string>({
    buildQueryKey: (params) =>
      ["aigateway", "models", params.page, params.pageSize, params.search ?? ""] as const,
    buildParams: ({ page, pageSize, search }) => ({ page, pageSize, search }),
    paginatedFn: async (params) => {
      const response = await listModels(params);
      const all = response?.items ?? [];
      // 后端列表无 keyword 参数，搜索只能客户端过滤（FE-C-14）；无搜索时保留
      // 服务端 total 以维持分页，搜索时以过滤后的条数作为 total。
      const searching = Boolean(params.search?.trim());
      const items = searching
        ? all.filter((it) => matchSearch(it, params.search, ["providerModelName", "displayName"]))
        : all;
      return { items, total: searching ? items.length : (response?.page?.total ?? items.length) };
    },
    initialPageSize: AI_GATEWAY_DEFAULT_PAGE_SIZE,
    deleteFn: deleteModel,
    invalidateKeysOnDelete: [["aigateway", "models"]],
    onDeleteSuccess: () => message.success(tc("aigateway.modelDeleted", "Model deleted")),
    enabled: account !== null,
  });

  // Provider list for the provider Select dropdown — lazy load when modal opens
  const providersQuery = useQuery<ProviderResponse[]>({
    queryKey: ["aigateway", "providers", "model-select-options"],
    queryFn: () => collectAllPages((params) => listProviders(params)),
    enabled: account !== null && (createOpen || editTarget !== null),
  });

  const createMutation = useMutation({
    mutationFn: (body: ModelCreate) => createModel(body),
    onSuccess: () => {
      list.refetch();
      setCreateOpen(false);
      createForm.resetFields();
      message.success(tc("aigateway.modelCreated", "Model created"));
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(createForm, err, buildServerFieldResolver(createForm), tc);
      }
      message.error(localizedError(err, tc, "common:error"));
    },
  });

  const editMutation = useMutation({
    mutationFn: ({ id, body }: { id: string; body: ModelUpdate }) => updateModel(id, body),
    onSuccess: () => {
      list.refetch();
      setEditTarget(null);
      editForm.resetFields();
      message.success(tc("aigateway.modelUpdated", "Model updated"));
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(editForm, err, buildServerFieldResolver(editForm), tc);
      }
      message.error(localizedError(err, tc, "common:error"));
    },
  });

  const openCreate = () => {
    setCreateOpen(true);
  };

  const openEdit = (record: ModelResponse) => {
    setEditTarget(record);
  };

  const handleCreate = async () => {
    const values = await createForm.validateFields();
    await createMutation.mutateAsync({
      providerId: values.providerId,
      providerModelName: values.providerModelName,
      displayName: values.displayName,
      maxContextTokens: values.maxContextTokens ?? null,
      maxOutputTokens: values.maxOutputTokens ?? null,
      status: values.status ?? "active",
    });
  };

  const handleEdit = async () => {
    if (!editTarget) return;
    const values = await editForm.validateFields();
    const body: ModelUpdate = {};
    if (values.displayName !== undefined) body.displayName = values.displayName;
    if (values.maxContextTokens !== undefined)
      body.maxContextTokens = values.maxContextTokens ?? null;
    if (values.maxOutputTokens !== undefined) body.maxOutputTokens = values.maxOutputTokens ?? null;
    if (values.status !== undefined) body.status = values.status;
    await editMutation.mutateAsync({ id: editTarget.id, body });
  };

  // Helper to find provider label for a given providerId
  const providerLabel = (providerId: string) => {
    const providers = providersQuery.data ?? [];
    const p = providers.find((provider) => provider.id === providerId);
    if (!p) return providerId.length > 8 ? `${providerId.slice(0, 8)}…` : providerId;
    return `${p.name} (${p.providerType})`;
  };

  const providerOptions = (providersQuery.data ?? []).map((p) => ({
    value: p.id,
    label: `${p.name} (${p.providerType})`,
  }));

  const columns: ColumnsType<ModelResponse> = [
    {
      title: tc("aigateway.providerModelName", "Provider Model Name"),
      dataIndex: "providerModelName",
      key: "providerModelName",
      width: 200,
      fixed: "left",
      sorter: (a, b) => a.providerModelName.localeCompare(b.providerModelName),
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
      title: tc("aigateway.providerId", "Provider"),
      dataIndex: "providerId",
      key: "providerId",
      width: 180,
      ellipsis: true,
      render: (id: string) => (
        <Tooltip title={id}>
          <code style={{ fontSize: 12 }}>{providerLabel(id)}</code>
        </Tooltip>
      ),
    },
    {
      title: tc("aigateway.maxContextTokens", "Context"),
      dataIndex: "maxContextTokens",
      key: "maxContextTokens",
      width: 100,
      align: "right",
      render: (v: number | null | undefined) => (v ? v.toLocaleString() : "—"),
    },
    {
      title: tc("aigateway.maxOutputTokens", "Output"),
      dataIndex: "maxOutputTokens",
      key: "maxOutputTokens",
      width: 100,
      align: "right",
      render: (v: number | null | undefined) => (v ? v.toLocaleString() : "—"),
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
        const color = status === "active" ? "green" : "default";
        return <Tag color={color}>{status}</Tag>;
      },
    },
    {
      title: tc("actions", "Actions"),
      width: 160,
      key: "actions",
      fixed: "right",
      render: (_: unknown, record) => (
        <Space size={4}>
          <PermissionGuard action="aigateway-model:write" fallback={null}>
            <Tooltip title={tc("edit", "Edit")}>
              <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(record)} />
            </Tooltip>
          </PermissionGuard>
          <PermissionGuard action="aigateway-model:delete" fallback={null}>
            <Tooltip title={tc("delete", "Delete")}>
              <Popconfirm
                title={tc("aigateway.confirmDeleteModel")}
                description={tc("aigateway.confirmDeleteModelDesc")}
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
      {renderCrudListView<ModelResponse, ListPageParams, string>(
        {
          title: tc("nav.aiGateway.models", "Models"),
          description: tc(
            "aigateway.modelsDescription",
            "Physical models registered on upstream providers.",
          ),
          searchPlaceholder: tc("aigateway.searchModels", "Search models..."),
          rowKey: "id",
          enableBulkDelete: true,
          bulkDeletePermission: {
            permission: "aigateway-model:delete",
            resource: "aigateway-models",
          },
          toolbar: (
            <PermissionGuard action="aigateway-model:write" fallback={null}>
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
                {tc("aigateway.createModel", "New Model")}
              </Button>
            </PermissionGuard>
          ),
          columns,
        },
        list,
      )}

      {/* Create Modal */}
      <Modal
        title={tc("aigateway.createModel", "New Model")}
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
            label={tc("aigateway.providerId", "Provider")}
            name="providerId"
            rules={[
              { required: true, message: tc("aigateway.providerRequired", "Select a provider") },
            ]}
          >
            <Select
              options={providerOptions}
              showSearch
              placeholder={tc("aigateway.selectProvider", "Pick a provider...")}
              loading={providersQuery.isLoading}
              filterOption={(input, option) =>
                (option?.label ?? "").toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
          <Form.Item
            label={tc("aigateway.providerModelName", "Provider model name")}
            name="providerModelName"
            rules={[
              {
                required: true,
                message: tc(
                  "aigateway.providerModelNameRequired",
                  "Enter the model name as registered on the provider",
                ),
              },
            ]}
          >
            <Input placeholder="e.g. gpt-4o-mini" />
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
            label={tc("aigateway.maxContextTokens", "Max context tokens")}
            name="maxContextTokens"
          >
            <InputNumber
              min={0}
              placeholder={tc("aigateway.leaveBlankDefault", "Leave blank for default")}
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item
            label={tc("aigateway.maxOutputTokens", "Max output tokens")}
            name="maxOutputTokens"
          >
            <InputNumber
              min={0}
              placeholder={tc("aigateway.leaveBlankDefault", "Leave blank for default")}
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item label={tc("aigateway.status", "Status")} name="status" initialValue="active">
            <Select options={STATUS_OPTIONS} />
          </Form.Item>
        </Form>
      </Modal>

      {/* Edit Modal */}
      <Modal
        title={tc("aigateway.editModel", "Edit Model")}
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
              displayName: editTarget.displayName,
              maxContextTokens: editTarget.maxContextTokens,
              maxOutputTokens: editTarget.maxOutputTokens,
              status: editTarget.status,
            }}
          >
            {/* providerId — locked (read-only display) */}
            <Form.Item label={tc("aigateway.providerId", "Provider")}>
              <code style={{ fontSize: 13 }}>{providerLabel(editTarget.providerId)}</code>
            </Form.Item>
            {/* providerModelName — locked (read-only display) */}
            <Form.Item label={tc("aigateway.providerModelName", "Provider model name")}>
              <code style={{ fontSize: 13 }}>{editTarget.providerModelName}</code>
            </Form.Item>
            <Form.Item label={tc("aigateway.displayName", "Display name")} name="displayName">
              <Input />
            </Form.Item>
            <Form.Item
              label={tc("aigateway.maxContextTokens", "Max context tokens")}
              name="maxContextTokens"
            >
              <InputNumber min={0} style={{ width: "100%" }} />
            </Form.Item>
            <Form.Item
              label={tc("aigateway.maxOutputTokens", "Max output tokens")}
              name="maxOutputTokens"
            >
              <InputNumber min={0} style={{ width: "100%" }} />
            </Form.Item>
            <Form.Item label={tc("aigateway.status", "Status")} name="status">
              <Select options={STATUS_OPTIONS} />
            </Form.Item>
          </Form>
        )}
      </Modal>
    </>
  );
}
