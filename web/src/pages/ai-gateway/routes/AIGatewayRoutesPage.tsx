// =============================================================================
// AI Gateway — Routes 列表页
// =============================================================================
//
// 对应后端 admin_chi.go: GET/POST /api/v1/ai/routes + PATCH/DELETE /api/v1/ai/routes/{id}
// 别名 → 物理模型的候选路由，附带 priority + weight 决定 failover 顺序。
// RouteResponse 后端已 JOIN 好 aliasName/modelName/providerName。
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
  createRoute,
  deleteRoute,
  listAliases,
  listModels,
  listRoutes,
  updateRoute,
  type AliasResponse,
  type ListPageParams,
  type ModelResponse,
  type RouteCreate,
  type RouteResponse,
  type RouteUpdate,
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

export default function AIGatewayRoutesPage() {
  const { t: tc } = useTranslation("common");
  const account = useSessionStore((state) => state.account);

  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<RouteResponse | null>(null);
  const [createForm] = Form.useForm();
  const [editForm] = Form.useForm();

  // ── useCrudList（server 分页模式） ──
  const list = useCrudList<RouteResponse, ListPageParams, string>({
    buildQueryKey: (params) =>
      ["aigateway", "routes", params.page, params.pageSize, params.search ?? ""] as const,
    buildParams: ({ page, pageSize, search }) => ({ page, pageSize, search }),
    paginatedFn: async (params) => {
      const response = await listRoutes(params);
      const all = response?.items ?? [];
      // 后端列表无 keyword 参数，搜索只能客户端过滤（FE-C-14）；无搜索时保留
      // 服务端 total 以维持分页，搜索时以过滤后的条数作为 total。
      const searching = Boolean(params.search?.trim());
      const items = searching
        ? all.filter((it) =>
            matchSearch(it, params.search, ["aliasName", "modelName", "providerName"]),
          )
        : all;
      return { items, total: searching ? items.length : (response?.page?.total ?? items.length) };
    },
    initialPageSize: AI_GATEWAY_DEFAULT_PAGE_SIZE,
    deleteFn: deleteRoute,
    invalidateKeysOnDelete: [["aigateway", "routes"]],
    onDeleteSuccess: () => message.success(tc("aigateway.routeDeleted", "Route deleted")),
    enabled: account !== null,
  });

  // Alias / Model lazy-load 数据源（两个 Modal 共用，缓存）
  const aliasOptionsQuery = useQuery({
    queryKey: ["aigateway", "aliases", "select-options"],
    queryFn: () => collectAllPages((params) => listAliases(params)),
    select: (data) =>
      (data ?? []).map((a: AliasResponse) => ({
        value: a.id,
        label: `${a.alias} (${a.displayName || "no label"})`,
      })),
    enabled: account !== null && (createOpen || editTarget !== null),
  });

  const modelOptionsQuery = useQuery({
    queryKey: ["aigateway", "models", "select-options"],
    queryFn: () => collectAllPages((params) => listModels(params)),
    select: (data) =>
      (data ?? []).map((m: ModelResponse) => ({
        value: m.id,
        label: `${m.providerModelName} (${m.displayName || "no label"})`,
      })),
    enabled: account !== null && (createOpen || editTarget !== null),
  });

  const createMutation = useMutation({
    mutationFn: (body: RouteCreate) => createRoute(body),
    onSuccess: () => {
      list.refetch();
      setCreateOpen(false);
      createForm.resetFields();
      message.success(tc("aigateway.routeCreated", "Route created"));
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(createForm, err, buildServerFieldResolver(createForm), tc);
      }
      message.error(localizedError(err, tc, "common:error"));
    },
  });

  const editMutation = useMutation({
    mutationFn: ({ id, body }: { id: string; body: RouteUpdate }) => updateRoute(id, body),
    onSuccess: () => {
      list.refetch();
      setEditTarget(null);
      editForm.resetFields();
      message.success(tc("aigateway.routeUpdated", "Route updated"));
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(editForm, err, buildServerFieldResolver(editForm), tc);
      }
      message.error(localizedError(err, tc, "common:error"));
    },
  });

  const openCreate = () => {
    createForm.setFieldsValue({
      priority: 0,
      weight: 100,
      status: "active",
    });
    setCreateOpen(true);
  };

  const openEdit = (record: RouteResponse) => {
    setEditTarget(record);
  };

  const handleCreate = async () => {
    const values = await createForm.validateFields();
    await createMutation.mutateAsync({
      aliasId: values.aliasId,
      modelId: values.modelId,
      priority: values.priority ?? 0,
      weight: values.weight ?? 100,
      status: values.status ?? "active",
      // organizationId 由后端 transport 层从 session 注入
    });
  };

  const handleEdit = async () => {
    if (!editTarget) return;
    const values = await editForm.validateFields();
    const body: RouteUpdate = {};
    if (values.modelId !== undefined) body.modelId = values.modelId;
    if (values.priority !== undefined) body.priority = values.priority;
    if (values.weight !== undefined) body.weight = values.weight;
    if (values.status !== undefined) body.status = values.status;
    await editMutation.mutateAsync({ id: editTarget.id, body });
  };

  const columns: ColumnsType<RouteResponse> = [
    {
      title: tc("aigateway.alias", "Alias"),
      dataIndex: "aliasName",
      key: "aliasName",
      width: 200,
      fixed: "left",
      sorter: (a, b) => (a.aliasName ?? "").localeCompare(b.aliasName ?? ""),
      render: (name: string | undefined, record) => (
        <Tooltip title={record.aliasId}>
          <span style={{ fontWeight: 500 }}>{name ?? "—"}</span>
        </Tooltip>
      ),
    },
    {
      title: "→",
      key: "arrow",
      width: 40,
      align: "center",
      render: () => <span style={{ color: "var(--gl-color-text-tertiary)" }}>→</span>,
    },
    {
      title: tc("aigateway.model", "Model"),
      dataIndex: "modelName",
      key: "modelName",
      render: (name: string | undefined, record) => (
        <Tooltip title={record.modelId}>
          <Space>
            <span>{name ?? "—"}</span>
            {record.providerName && (
              <Tag color="blue" style={{ margin: 0 }}>
                {record.providerName}
              </Tag>
            )}
          </Space>
        </Tooltip>
      ),
    },
    {
      title: tc("aigateway.priority", "Priority"),
      dataIndex: "priority",
      key: "priority",
      width: 80,
      sorter: (a, b) => a.priority - b.priority,
      align: "right",
    },
    {
      title: tc("aigateway.weight", "Weight"),
      dataIndex: "weight",
      key: "weight",
      width: 80,
      sorter: (a, b) => a.weight - b.weight,
      align: "right",
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
          <PermissionGuard action="aigateway-route:write" fallback={null}>
            <Tooltip title={tc("edit", "Edit")}>
              <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(record)} />
            </Tooltip>
          </PermissionGuard>
          <PermissionGuard action="aigateway-route:delete" fallback={null}>
            <Tooltip title={tc("delete", "Delete")}>
              <Popconfirm
                title={tc("aigateway.confirmDeleteRoute")}
                description={tc("aigateway.confirmDeleteRouteDesc")}
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
      {renderCrudListView<RouteResponse, ListPageParams, string>(
        {
          title: tc("nav.aiGateway.routes", "Routes"),
          description: tc(
            "aigateway.routesDescription",
            "Bind aliases to concrete models. Priority + weight determine failover order.",
          ),
          searchPlaceholder: tc("aigateway.searchRoutes", "Search routes..."),
          rowKey: "id",
          enableBulkDelete: true,
          bulkDeletePermission: {
            permission: "aigateway-route:delete",
            resource: "aigateway-routes",
          },
          toolbar: (
            <PermissionGuard action="aigateway-route:write" fallback={null}>
              <Button type="primary" icon={<PlusOutlined />} onClick={() => openCreate()}>
                {tc("aigateway.createRoute", "New Route")}
              </Button>
            </PermissionGuard>
          ),
          columns,
        },
        list,
      )}

      {/* Create Modal */}
      <Modal
        title={tc("aigateway.createRoute", "New Route")}
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
            name="aliasId"
            rules={[{ required: true, message: tc("aigateway.aliasRequired", "Select an alias") }]}
          >
            <Select
              showSearch
              placeholder={tc("aigateway.selectAlias", "Pick an alias...")}
              loading={aliasOptionsQuery.isLoading}
              options={aliasOptionsQuery.data ?? []}
              filterOption={(input, option) =>
                (option?.label ?? "").toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
          <Form.Item
            label={tc("aigateway.model", "Model")}
            name="modelId"
            rules={[{ required: true, message: tc("aigateway.modelRequired", "Select a model") }]}
          >
            <Select
              showSearch
              placeholder={tc("aigateway.selectModel", "Pick a model...")}
              loading={modelOptionsQuery.isLoading}
              options={modelOptionsQuery.data ?? []}
              filterOption={(input, option) =>
                (option?.label ?? "").toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
          <Form.Item
            label={tc("aigateway.priority", "Priority")}
            name="priority"
            tooltip={tc("aigateway.priorityHint", "Lower = higher failover priority")}
          >
            <InputNumber min={0} style={{ width: "100%" }} />
          </Form.Item>
          <Form.Item
            label={tc("aigateway.weight", "Weight")}
            name="weight"
            tooltip={tc("aigateway.weightHint", "Round-robin weight within same priority")}
          >
            <InputNumber min={0} style={{ width: "100%" }} />
          </Form.Item>
          <Form.Item label={tc("aigateway.status", "Status")} name="status">
            <Select options={STATUS_OPTIONS} />
          </Form.Item>
        </Form>
      </Modal>

      {/* Edit Modal */}
      <Modal
        title={tc("aigateway.editRoute", "Edit Route")}
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
              modelId: editTarget.modelId,
              priority: editTarget.priority,
              weight: editTarget.weight,
              status: editTarget.status,
            }}
          >
            {/* aliasId — locked (read-only code display) */}
            <Form.Item label={tc("aigateway.alias", "Alias")}>
              <code style={{ fontSize: 13 }}>{editTarget.aliasName ?? editTarget.aliasId}</code>
            </Form.Item>
            <Form.Item
              label={tc("aigateway.model", "Model")}
              name="modelId"
              rules={[{ required: true, message: tc("aigateway.modelRequired", "Select a model") }]}
            >
              <Select
                showSearch
                placeholder={tc("aigateway.selectModel", "Pick a model...")}
                loading={modelOptionsQuery.isLoading}
                options={modelOptionsQuery.data ?? []}
                filterOption={(input, option) =>
                  (option?.label ?? "").toLowerCase().includes(input.toLowerCase())
                }
              />
            </Form.Item>
            <Form.Item label={tc("aigateway.priority", "Priority")} name="priority">
              <InputNumber min={0} style={{ width: "100%" }} />
            </Form.Item>
            <Form.Item label={tc("aigateway.weight", "Weight")} name="weight">
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
