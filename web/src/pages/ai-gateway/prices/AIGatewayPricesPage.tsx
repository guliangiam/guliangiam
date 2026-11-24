// =============================================================================
// AI Gateway — Price Versions 列表页
// =============================================================================
//
// 对应后端 admin_chi.go: GET /api/v1/ai/prices
// 模型价格快照（成本计费用）。version_no 按 model 自增，后端 CreatePrice
// 自动分配，前端 Create 提交后由后端生成。
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
import { DeleteOutlined, EditOutlined, PlusOutlined } from "@ant-design/icons";
import { useMutation, useQuery } from "@tanstack/react-query";
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
  createPrice,
  deletePrice,
  listModels,
  listPrices,
  updatePrice,
  type ListPageParams,
  type ModelResponse,
  type PriceVersionResponse,
  type PriceVersionUpdate,
} from "@/shared/api/aigateway";
import { isHttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  localizedError,
} from "@/shared/lib/server-error";

export default function AIGatewayPricesPage() {
  const { t: tc } = useTranslation("common");
  const account = useSessionStore((state) => state.account);

  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<PriceVersionResponse | null>(null);
  const [createForm] = Form.useForm();
  const [editForm] = Form.useForm();

  const nowIso = new Date().toISOString();
  const tomorrowIso = new Date(Date.now() + 24 * 3600 * 1000).toISOString();
  const nextWeekIso = new Date(Date.now() + 7 * 24 * 3600 * 1000).toISOString();

  // ── useCrudList（server 分页模式） ──
  const list = useCrudList<PriceVersionResponse, ListPageParams, string>({
    buildQueryKey: (params) =>
      ["aigateway", "prices", params.page, params.pageSize, params.search ?? ""] as const,
    buildParams: ({ page, pageSize, search }) => ({ page, pageSize, search }),
    paginatedFn: async (params) => {
      const response = await listPrices(params);
      const all = response?.items ?? [];
      // 后端列表无 keyword 参数，搜索只能客户端过滤（FE-C-14）；无搜索时保留
      // 服务端 total 以维持分页，搜索时以过滤后的条数作为 total。
      const searching = Boolean(params.search?.trim());
      const items = searching
        ? all.filter((it) => matchSearch(it, params.search, ["modelId", "currency"]))
        : all;
      return { items, total: searching ? items.length : (response?.page?.total ?? items.length) };
    },
    initialPageSize: AI_GATEWAY_DEFAULT_PAGE_SIZE,
    deleteFn: deletePrice,
    invalidateKeysOnDelete: [["aigateway", "prices"]],
    onDeleteSuccess: () => message.success(tc("aigateway.priceDeleted", "Price version deleted")),
    enabled: account !== null,
  });

  const modelsQuery = useQuery({
    queryKey: ["aigateway", "models", "price-select-options"],
    queryFn: () => collectAllPages((params) => listModels(params)),
    select: (data) => {
      return (data ?? []).map((m: ModelResponse) => ({
        value: m.id,
        label: m.displayName || m.providerModelName || m.id,
      }));
    },
    enabled: account !== null && createOpen, // 只在打开 Create 时拉
  });

  const createMutation = useMutation({
    mutationFn: createPrice,
    onSuccess: () => {
      list.refetch();
      setCreateOpen(false);
      createForm.resetFields();
      message.success(tc("aigateway.priceCreated", "Price version created"));
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(createForm, err, buildServerFieldResolver(createForm), tc);
      }
      message.error(localizedError(err, tc, "common:error"));
    },
  });

  const editMutation = useMutation({
    mutationFn: ({ id, body }: { id: string; body: PriceVersionUpdate }) => updatePrice(id, body),
    onSuccess: () => {
      list.refetch();
      setEditTarget(null);
      editForm.resetFields();
      message.success(tc("aigateway.priceUpdated", "Price version updated"));
    },
    onError: (err) => {
      if (isHttpError(err)) {
        applyServerFieldErrors(editForm, err, buildServerFieldResolver(editForm), tc);
      }
      message.error(localizedError(err, tc, "common:error"));
    },
  });

  const openEdit = (record: PriceVersionResponse) => {
    setEditTarget(record);
  };

  const handleCreate = async () => {
    const values = await createForm.validateFields();
    await createMutation.mutateAsync({
      modelId: values.modelId,
      inputPricePer1M: values.inputPricePer1M,
      outputPricePer1M: values.outputPricePer1M,
      currency: values.currency ?? "USD",
      effectiveFrom: values.effectiveFrom,
    });
  };

  const handleEdit = async () => {
    if (!editTarget) return;
    const values = await editForm.validateFields();
    const body: PriceVersionUpdate = {};
    if (values.inputPricePer1M !== undefined) body.inputPricePer1M = values.inputPricePer1M;
    if (values.outputPricePer1M !== undefined) body.outputPricePer1M = values.outputPricePer1M;
    if (values.currency !== undefined) body.currency = values.currency;
    if (values.effectiveFrom !== undefined) {
      body.effectiveFrom = values.effectiveFrom;
    }
    await editMutation.mutateAsync({ id: editTarget.id, body });
  };

  const formatPer1M = (v: number, currency: string) => `${currency} ${v.toFixed(4)}/1M`;

  const columns: ColumnsType<PriceVersionResponse> = [
    {
      title: tc("aigateway.model", "Model"),
      dataIndex: "modelId",
      key: "modelId",
      width: 200,
      fixed: "left",
      ellipsis: true,
      render: (id: string) => (
        <Tooltip title={id}>
          <code style={{ fontSize: 12 }}>{id.slice(0, 16)}…</code>
        </Tooltip>
      ),
    },
    {
      title: tc("aigateway.versionNo", "Version"),
      dataIndex: "versionNo",
      key: "versionNo",
      width: 100,
      align: "center",
      sorter: (a, b) => a.versionNo - b.versionNo,
      render: (v: number) => <Tag color="blue">v{v}</Tag>,
    },
    {
      title: tc("aigateway.inputPrice", "Input Price"),
      dataIndex: "inputPricePer1M",
      key: "inputPricePer1M",
      width: 160,
      align: "right",
      sorter: (a, b) => a.inputPricePer1M - b.inputPricePer1M,
      render: (v: number, record) => formatPer1M(v, record.currency),
    },
    {
      title: tc("aigateway.outputPrice", "Output Price"),
      dataIndex: "outputPricePer1M",
      key: "outputPricePer1M",
      width: 160,
      align: "right",
      sorter: (a, b) => a.outputPricePer1M - b.outputPricePer1M,
      render: (v: number, record) => formatPer1M(v, record.currency),
    },
    {
      title: tc("aigateway.effectiveFrom", "Effective From"),
      dataIndex: "effectiveFrom",
      key: "effectiveFrom",
      width: 180,
      sorter: (a, b) => new Date(a.effectiveFrom).getTime() - new Date(b.effectiveFrom).getTime(),
      render: (v: string) => new Date(v).toLocaleString(),
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
          <PermissionGuard action="aigateway-price:write" fallback={null}>
            <Tooltip title={tc("edit", "Edit")}>
              <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(record)} />
            </Tooltip>
          </PermissionGuard>
          <PermissionGuard action="aigateway-price:delete" fallback={null}>
            <Tooltip title={tc("delete", "Delete")}>
              <Popconfirm
                title={tc("aigateway.confirmDeletePrice")}
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
      {renderCrudListView<PriceVersionResponse, ListPageParams, string>(
        {
          title: tc("nav.aiGateway.prices", "Price Versions"),
          description: tc(
            "aigateway.pricesDescription",
            "Per-model price snapshots used for cost accounting. Create a new version to reprice future calls.",
          ),
          searchPlaceholder: tc("aigateway.searchPrices", "Search prices..."),
          rowKey: "id",
          enableBulkDelete: true,
          bulkDeletePermission: {
            permission: "aigateway-price:delete",
            resource: "aigateway-prices",
          },
          toolbar: (
            <PermissionGuard action="aigateway-price:write" fallback={null}>
              <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
                {tc("aigateway.createPrice", "New Price Version")}
              </Button>
            </PermissionGuard>
          ),
          columns,
        },
        list,
      )}

      {/* Create Modal */}
      <Modal
        title={tc("aigateway.createPrice", "New Price Version")}
        open={createOpen}
        onCancel={() => {
          setCreateOpen(false);
          createForm.resetFields();
        }}
        onOk={() => handleCreate()}
        confirmLoading={createMutation.isPending}
        destroyOnHidden
      >
        <Form form={createForm} layout="vertical" preserve={false}>
          <Form.Item
            label={tc("aigateway.model", "Model")}
            name="modelId"
            rules={[{ required: true, message: tc("aigateway.modelRequired", "Select a model") }]}
          >
            <Select
              placeholder={tc("aigateway.selectModel", "Select a model...")}
              loading={modelsQuery.isLoading}
              options={modelsQuery.data ?? []}
              showSearch
              optionFilterProp="label"
            />
          </Form.Item>
          <Form.Item
            label={tc("aigateway.inputPrice", "Input Price per 1M tokens")}
            name="inputPricePer1M"
            rules={[
              { required: true, message: tc("aigateway.priceRequired", "Enter input price") },
            ]}
          >
            <InputNumber min={0} step={0.0001} precision={4} style={{ width: "100%" }} />
          </Form.Item>
          <Form.Item
            label={tc("aigateway.outputPrice", "Output Price per 1M tokens")}
            name="outputPricePer1M"
            rules={[
              { required: true, message: tc("aigateway.priceRequired", "Enter output price") },
            ]}
          >
            <InputNumber min={0} step={0.0001} precision={4} style={{ width: "100%" }} />
          </Form.Item>
          <Form.Item
            label={tc("aigateway.currency", "Currency")}
            name="currency"
            initialValue="USD"
          >
            <Select
              options={[
                { value: "USD", label: "USD" },
                { value: "CNY", label: "CNY" },
                { value: "EUR", label: "EUR" },
              ]}
            />
          </Form.Item>
          <Form.Item
            label={tc("aigateway.effectiveFrom", "Effective From")}
            name="effectiveFrom"
            rules={[
              { required: true, message: tc("aigateway.effectiveRequired", "Pick effective time") },
            ]}
            extra={tc(
              "aigateway.effectiveHint",
              "Prices only take effect at or after this time. Future-dated records are reserved and skipped by cost calculation.",
            )}
          >
            <Select
              options={[
                {
                  value: nowIso,
                  label: tc("aigateway.now", "Now (immediate)"),
                },
                {
                  value: tomorrowIso,
                  label: tc("aigateway.tomorrow", "Tomorrow"),
                },
                {
                  value: nextWeekIso,
                  label: tc("aigateway.nextWeek", "Next week"),
                },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* Edit Modal */}
      <Modal
        title={tc("aigateway.editPrice", "Edit Price Version")}
        open={editTarget !== null}
        onCancel={() => {
          setEditTarget(null);
          editForm.resetFields();
        }}
        onOk={() => handleEdit()}
        confirmLoading={editMutation.isPending}
        destroyOnHidden
      >
        {editTarget && (
          <Form
            form={editForm}
            layout="vertical"
            preserve={false}
            initialValues={{
              inputPricePer1M: editTarget.inputPricePer1M,
              outputPricePer1M: editTarget.outputPricePer1M,
              currency: editTarget.currency,
              // 与「保持当前」选项的 value（完整 RFC3339）一致，避免 slice 后既不匹配选项又非法提交。
              effectiveFrom: editTarget.effectiveFrom ?? undefined,
            }}
          >
            <Form.Item label={tc("aigateway.model", "Model")}>
              <code style={{ fontSize: 12 }}>{editTarget.modelId}</code>
              <div style={{ fontSize: 12, color: "var(--gl-color-text-tertiary)" }}>
                {tc("aigateway.modelLocked", "Model is locked after creation")}
              </div>
            </Form.Item>
            <Form.Item label={tc("aigateway.versionNo", "Version")}>
              <Tag color="blue">v{editTarget.versionNo}</Tag>
              <span style={{ marginLeft: 8, fontSize: 12, color: "var(--gl-color-text-tertiary)" }}>
                {tc("aigateway.versionLocked", "Version is auto-assigned and immutable")}
              </span>
            </Form.Item>
            <Form.Item
              label={tc("aigateway.inputPrice", "Input Price per 1M tokens")}
              name="inputPricePer1M"
              rules={[
                { required: true, message: tc("aigateway.priceRequired", "Enter input price") },
              ]}
            >
              <InputNumber min={0} step={0.0001} precision={4} style={{ width: "100%" }} />
            </Form.Item>
            <Form.Item
              label={tc("aigateway.outputPrice", "Output Price per 1M tokens")}
              name="outputPricePer1M"
              rules={[
                { required: true, message: tc("aigateway.priceRequired", "Enter output price") },
              ]}
            >
              <InputNumber min={0} step={0.0001} precision={4} style={{ width: "100%" }} />
            </Form.Item>
            <Form.Item label={tc("aigateway.currency", "Currency")} name="currency">
              <Select
                options={[
                  { value: "USD", label: "USD" },
                  { value: "CNY", label: "CNY" },
                  { value: "EUR", label: "EUR" },
                ]}
              />
            </Form.Item>
            <Form.Item label={tc("aigateway.effectiveFrom", "Effective From")} name="effectiveFrom">
              <Select
                options={[
                  {
                    value: editTarget.effectiveFrom,
                    label: tc("aigateway.keepCurrent", "Keep current"),
                  },
                  {
                    value: nowIso,
                    label: tc("aigateway.now", "Now"),
                  },
                  {
                    value: tomorrowIso,
                    label: tc("aigateway.tomorrow", "Tomorrow"),
                  },
                ]}
              />
            </Form.Item>
          </Form>
        )}
      </Modal>
    </>
  );
}
