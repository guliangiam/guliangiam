// =============================================================================
// AI Gateway — Aliases 列表页
// =============================================================================
//
// 对应后端 admin_chi.go: GET/POST /api/v1/ai/aliases + PATCH/DELETE /api/v1/ai/aliases/{id}
// 逻辑模型别名（调用方可见名），由 Route 绑定到具体 Model。
// 完整 CRUD：Create Modal + Edit Modal + Delete。
// =============================================================================

import { Button, Form, Input, Modal, Popconfirm, Select, Space, Tag, Tooltip, message } from "antd";
import { PlusOutlined, DeleteOutlined, EditOutlined } from "@ant-design/icons";
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
  createAlias,
  deleteAlias,
  listAliases,
  updateAlias,
  type AliasCreate,
  type AliasResponse,
  type AliasUpdate,
  type ListPageParams,
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

export default function AIGatewayAliasesPage() {
  const { t: tc } = useTranslation("common");
  const account = useSessionStore((state) => state.account);

  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<AliasResponse | null>(null);
  const [createForm] = Form.useForm();
  const [editForm] = Form.useForm();

  // ── useCrudList（server 分页模式） ──
  const list = useCrudList<AliasResponse, ListPageParams, string>({
    buildQueryKey: (params) =>
      ["aigateway", "aliases", params.page, params.pageSize, params.search ?? ""] as const,
    buildParams: ({ page, pageSize, search }) => ({ page, pageSize, search }),
    paginatedFn: async (params) => {
      const response = await listAliases(params);
      const all = response?.items ?? [];
      // 后端列表无 keyword 参数，搜索只能客户端过滤（FE-C-14）；无搜索时保留
      // 服务端 total 以维持分页，搜索时以过滤后的条数作为 total。
      const searching = Boolean(params.search?.trim());
      const items = searching
        ? all.filter((it) => matchSearch(it, params.search, ["alias", "displayName"]))
        : all;
      return { items, total: searching ? items.length : (response?.page?.total ?? items.length) };
    },
    initialPageSize: AI_GATEWAY_DEFAULT_PAGE_SIZE,
    deleteFn: deleteAlias,
    invalidateKeysOnDelete: [["aigateway", "aliases"]],
    onDeleteSuccess: () => message.success(tc("aigateway.aliasDeleted", "Alias deleted")),
    enabled: account !== null,
  });

  const createMutation = useMutation({
    mutationFn: (body: AliasCreate) => createAlias(body),
    onSuccess: () => {
      list.refetch();
      setCreateOpen(false);
      createForm.resetFields();
      message.success(tc("aigateway.aliasCreated", "Alias created"));
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(createForm, err, buildServerFieldResolver(createForm), tc);
      }
      message.error(localizedError(err, tc, "common:error"));
    },
  });

  const editMutation = useMutation({
    mutationFn: ({ id, body }: { id: string; body: AliasUpdate }) => updateAlias(id, body),
    onSuccess: () => {
      list.refetch();
      setEditTarget(null);
      editForm.resetFields();
      message.success(tc("aigateway.aliasUpdated", "Alias updated"));
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(editForm, err, buildServerFieldResolver(editForm), tc);
      }
      message.error(localizedError(err, tc, "common:error"));
    },
  });

  const openEdit = (record: AliasResponse) => {
    setEditTarget(record);
  };

  const handleCreate = async () => {
    const values = await createForm.validateFields();
    await createMutation.mutateAsync({
      alias: values.alias,
      displayName: values.displayName,
      status: values.status ?? "active",
    });
  };

  const handleEdit = async () => {
    if (!editTarget) return;
    const values = await editForm.validateFields();
    const body: AliasUpdate = {};
    if (values.displayName !== undefined) body.displayName = values.displayName;
    if (values.status !== undefined) body.status = values.status;
    await editMutation.mutateAsync({ id: editTarget.id, body });
  };

  const columns: ColumnsType<AliasResponse> = [
    {
      title: tc("aigateway.alias", "Alias"),
      dataIndex: "alias",
      key: "alias",
      width: 200,
      fixed: "left",
      sorter: (a, b) => a.alias.localeCompare(b.alias),
      render: (alias: string, record) => (
        <Space orientation="vertical" size={0}>
          <span style={{ fontWeight: 500 }}>`{alias}`</span>
          {record.displayName && (
            <span style={{ color: "var(--gl-color-text-tertiary)", fontSize: 12 }}>
              {record.displayName}
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
        { text: tc("aigateway.disabled", "Disabled"), value: "disabled" },
      ],
      onFilter: (value, record) => record.status === value,
      render: (s: string) => <Tag color={s === "active" ? "green" : "default"}>{s}</Tag>,
    },
    {
      title: tc("aigateway.createdAt", "Created"),
      dataIndex: "createdAt",
      key: "createdAt",
      width: 180,
      render: (v: string) => new Date(v).toLocaleString(),
    },
    {
      title: tc("actions", "Actions"),
      key: "actions",
      width: 160,
      fixed: "right",
      render: (_: unknown, record) => (
        <Space size={4}>
          <PermissionGuard action="aigateway-alias:write" fallback={null}>
            <Tooltip title={tc("edit", "Edit")}>
              <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(record)} />
            </Tooltip>
          </PermissionGuard>
          <PermissionGuard action="aigateway-alias:delete" fallback={null}>
            <Tooltip title={tc("delete", "Delete")}>
              <Popconfirm
                title={tc("aigateway.confirmDeleteAlias")}
                description={tc("aigateway.confirmDeleteAliasDesc")}
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
      {renderCrudListView<AliasResponse, ListPageParams, string>(
        {
          title: tc("nav.aiGateway.aliases", "Aliases"),
          description: tc(
            "aigateway.aliasesDescription",
            "Logical model names that callers use. Routes bind each alias to concrete models.",
          ),
          searchPlaceholder: tc("aigateway.searchAliases", "Search aliases..."),
          rowKey: "id",
          enableBulkDelete: true,
          bulkDeletePermission: {
            permission: "aigateway-alias:delete",
            resource: "aigateway-aliases",
          },
          toolbar: (
            <PermissionGuard action="aigateway-alias:write" fallback={null}>
              <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
                {tc("aigateway.createAlias", "New Alias")}
              </Button>
            </PermissionGuard>
          ),
          columns,
        },
        list,
      )}

      {/* Create Modal */}
      <Modal
        title={tc("aigateway.createAlias", "New Alias")}
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
            label={tc("aigateway.alias", "Alias")}
            name="alias"
            rules={[
              {
                required: true,
                message: tc("aigateway.aliasRequired", "Enter an alias name"),
              },
              {
                pattern: /^[a-zA-Z0-9][a-zA-Z0-9_\-./]*$/,
                message: tc(
                  "aigateway.aliasInvalid",
                  "Alias must start with a letter or digit and contain only letters, digits, _ - . /",
                ),
              },
            ]}
          >
            <Input placeholder="e.g. gpt-4o, claude-prod" />
          </Form.Item>
          <Form.Item label={tc("aigateway.displayName", "Display name")} name="displayName">
            <Input
              placeholder={tc(
                "aigateway.displayNamePlaceholder",
                "Human-friendly label (optional)",
              )}
            />
          </Form.Item>
          <Form.Item label={tc("aigateway.status", "Status")} name="status" initialValue="active">
            <Select options={STATUS_OPTIONS} />
          </Form.Item>
        </Form>
      </Modal>

      {/* Edit Modal */}
      <Modal
        title={tc("aigateway.editAlias", "Edit Alias")}
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
            initialValues={{ displayName: editTarget.displayName, status: editTarget.status }}
          >
            {/* alias — locked (read-only display) */}
            <Form.Item label={tc("aigateway.alias", "Alias")}>
              <code style={{ fontSize: 13 }}>{editTarget.alias}</code>
            </Form.Item>
            <Form.Item label={tc("aigateway.displayName", "Display name")} name="displayName">
              <Input />
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
