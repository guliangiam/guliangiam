import { useState } from "react";
import { App, Button, Dropdown, Popconfirm, Space, Tag } from "antd";
import { DownOutlined } from "@ant-design/icons";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import { useSessionStore } from "@/features/session/session-store";
import { renderCrudListView } from "@/shared/ui/CrudListPage";
import { useCrudList } from "@/shared/hooks/useCrudList";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { ProviderCategoryBadge } from "@/shared/ui/ProviderCategoryBadge";
import { ProviderIcon } from "@/shared/ui/ProviderIcon";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { usePermissions } from "@/shared/hooks/usePermissions";
import {
  getUnifiedProvidersPaginated,
  deleteProvider,
  type ProviderListParams,
  type UnifiedProviderRow,
  type UnifiedProviderSource,
} from "@/shared/api/iam/provider-extended";
import {
  deleteIamCaptchaProvider,
  deleteIamNotificationProvider,
  deleteIamFaceIdProvider,
  getIamNotificationProvider,
  patchIamNotificationProvider,
} from "@/shared/api/iam";
import { queryKeys } from "@/shared/api/query-keys";
import {
  IDVERIFICATION_FILTER_TYPES,
  PROVIDER_FILTER_CATEGORIES,
  PROVIDER_FILTER_TYPES,
} from "@/shared/hooks/useProviderCategories";
import {
  captchaProviderTypeOptions,
  notificationProviderTypeOptions,
} from "@/shared/constants/enums";
import { LinkButton } from "@/shared/ui/LinkButton";
import { localizedError } from "@/shared/lib/server-error";

// 详情级端点对平台管理员要求显式 organizationId（会话组织回落会 404），
// 删除键必须携带行所属组织与来源（决定走统一还是专用端点）。
type ProviderDeleteKey = {
  id: string;
  organizationId?: string;
  source: UnifiedProviderSource;
};

const EDIT_PATH_BY_SOURCE: Record<UnifiedProviderSource, string> = {
  unified: "providers",
  captcha: "captcha-providers",
  notification: "notification-providers",
  "face-id": "face-id-providers",
};

function editPath(record: UnifiedProviderRow): string {
  return `/${EDIT_PATH_BY_SOURCE[record.source]}/${record.id}?organizationId=${record.organizationId}`;
}

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
    <Space size="small">
      <Button size="small" onClick={onEnable} loading={loading} disabled={loading}>
        {enableLabel}
      </Button>
      <Button size="small" onClick={onDisable} loading={loading} disabled={loading}>
        {disableLabel}
      </Button>
    </Space>
  );
}

const VALID_CATEGORIES = new Set(PROVIDER_FILTER_CATEGORIES.map((option) => option.value));

/**
 * 提供商统一列表：验证码 / 通知 / 人脸识别三类不再用独立标签页物理隔离，
 * 数据由 getUnifiedProvidersPaginated 聚合四个来源，「类别」+「身份提供商类型」
 * 两个字段是唯一分类依据；操作按行来源分流回各自端点。租户隔离由各端点
 * 服务端 trustedOrgID 裁决保证，organizationId 仅平台管理员生效。
 */
export default function ProvidersPage() {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { nameMap: orgNameMap } = useOrganizationNames();
  const { message } = App.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const organization = useSessionStore((state) => state.organization);
  const account = useSessionStore((state) => state.account);
  const { hasAction } = usePermissions();

  const [orgFilter, setOrgFilter] = useState(organization?.id ?? "");

  // 深链 /providers?category=Captcha 等直接预置类别筛选。
  const [searchParams] = useSearchParams();
  const categoryParam = searchParams.get("category");
  const initialFilters =
    categoryParam && VALID_CATEGORIES.has(categoryParam) ? { category: categoryParam } : undefined;

  const list = useCrudList<UnifiedProviderRow, ProviderListParams, ProviderDeleteKey>({
    initialFilters,
    buildQueryKey: (p) =>
      [
        "providers",
        "paginated",
        p.organizationId,
        p.category,
        p.providerType,
        p.page,
        p.pageSize,
        p.search,
      ] as const,
    buildParams: ({ page, pageSize, search, filters }) => ({
      page,
      pageSize,
      search: search || undefined,
      organizationId: orgFilter || undefined,
      category: filters.category || undefined,
      // 人脸识别提供商沿用原面板：无类型筛选，忽略残留的类型值。
      providerType: filters.category === "FaceId" ? undefined : filters.type || undefined,
    }),
    paginatedFn: getUnifiedProvidersPaginated,
    deleteFn: (key) => {
      switch (key.source) {
        case "captcha":
          return deleteIamCaptchaProvider(key.id, key.organizationId);
        case "notification":
          return deleteIamNotificationProvider(key.id, key.organizationId);
        case "face-id":
          return deleteIamFaceIdProvider(key.id, key.organizationId);
        default:
          return deleteProvider(key.id, key.organizationId);
      }
    },
    invalidateKeysOnDelete: [
      ["providers"],
      ["captcha-providers"],
      ["notification-providers"],
      ["face-id-providers"],
    ],
    enabled: organization !== null,
    onDeleteSuccess: () => message.success(t("deleted")),
    onDeleteError: (error) => message.error(localizedError(error, t, "deleteFailed")),
  });

  const selectedCategory = list.filters.category;
  const typeFilterOptions =
    selectedCategory === "Captcha"
      ? captchaProviderTypeOptions()
      : selectedCategory === "Notification"
        ? notificationProviderTypeOptions()
        : selectedCategory === "IDVerification"
          ? [...IDVERIFICATION_FILTER_TYPES]
          : [...PROVIDER_FILTER_TYPES];

  const createMenuItems = [
    hasAction("captcha-provider:write")
      ? {
          key: "captcha",
          label: tr("captchaProvider.newProvider"),
          onClick: () => navigate("/providers/new?category=Captcha"),
        }
      : null,
    hasAction("notification-provider:write")
      ? {
          key: "notification",
          label: tr("notificationProvider.newProvider", "New Provider"),
          onClick: () => navigate("/notification-providers/new"),
        }
      : null,
    hasAction("face-id-provider:write")
      ? {
          key: "face-id",
          label: tr("faceId.newProvider"),
          onClick: () => navigate("/face-id-providers/new"),
        }
      : null,
  ].filter((item): item is NonNullable<typeof item> => item !== null);

  const [bulkLoading, setBulkLoading] = useState(false);

  const unifiedView = renderCrudListView<UnifiedProviderRow, ProviderListParams, ProviderDeleteKey>(
    {
      title: t("providers"),
      description: t("providerListDesc"),
      searchPlaceholder: t("placeholder.searchByName"),
      // 不同来源表可能复用同一 UUID（如 providers 与 face_id_providers 各有一条
      // 同 id 记录），聚合后必须用「来源:id」作为行键，否则 React 重复 key 会
      // 在筛选切换时残留旧行、勾选串行。
      rowKey: (record) => `${record.source}:${record.id}`,
      rowKeyToDeleteKey: (record) => ({
        id: record.id,
        organizationId: record.organizationId,
        source: record.source,
      }),
      filters: [
        {
          key: "category",
          label: t("category"),
          options: [...PROVIDER_FILTER_CATEGORIES],
        },
        // 人脸识别提供商沿用原面板：不提供类型筛选。
        ...(selectedCategory === "FaceId"
          ? []
          : [
              {
                key: "type",
                label: t("subtype"),
                options: [...typeFilterOptions],
              },
            ]),
      ],
      enableBulkDelete: true,
      bulkDeletePermission: { permission: "provider:delete", resource: "providers" },
      showRefresh: true,
      bulkActions: ({ selectedKeys, records }) => {
        // 沿用原通知面板：批量启停只作用于选中的通知提供商行。
        const targets = records.filter(
          (record) =>
            selectedKeys.includes(`${record.source}:${record.id}`) &&
            record.source === "notification",
        );
        if (targets.length === 0) return null;
        const applyEnabled = async (isEnabled: boolean) => {
          const todo = targets.filter((record) => record.enabled !== isEnabled);
          if (todo.length === 0) return;
          const results = await Promise.allSettled(
            todo.map(async (record) => {
              // revision 缺失时回源详情取真实值；兜底 0 会绕过后端乐观锁。
              const revision =
                record.revision ??
                (await getIamNotificationProvider(record.id, record.organizationId)).revision;
              return patchIamNotificationProvider(
                record.id,
                { isEnabled, revision },
                record.organizationId,
              );
            }),
          );
          // 无论成败都刷新列表：部分成功时开关状态也必须与服务端一致。
          queryClient.invalidateQueries({ queryKey: queryKeys.notificationProviders.all });
          queryClient.invalidateQueries({ queryKey: ["providers"] });
          if (results.some((result) => result.status === "rejected")) {
            message.error(t("operationFailed"));
          } else {
            message.success(t("updateSuccess"));
          }
        };
        return renderBulkEnableActions({
          enableLabel: t("action.enable"),
          disableLabel: t("action.disable"),
          onEnable: () => {
            setBulkLoading(true);
            void applyEnabled(true).finally(() => setBulkLoading(false));
          },
          onDisable: () => {
            setBulkLoading(true);
            void applyEnabled(false).finally(() => setBulkLoading(false));
          },
          loading: bulkLoading,
        });
      },
      toolbar: (
        <Space>
          {account?.isAdmin && (
            <OrganizationSelect
              value={orgFilter}
              onChange={(v) => {
                setOrgFilter(v);
                list.serverPagination.onChange(1, list.pagination.pageSize);
              }}
              style={{ width: 180 }}
              allowClear
              autoFillSessionOrg={false}
            />
          )}
          <PermissionGuard action="provider:write" resource="providers" fallback={null}>
            {createMenuItems.length > 0 ? (
              <Space.Compact>
                <Button type="primary" onClick={() => navigate("/providers/new")}>
                  {t("create")}
                </Button>
                <Dropdown menu={{ items: createMenuItems }} trigger={["click"]}>
                  <Button type="primary" icon={<DownOutlined />} aria-label={t("moreActions")} />
                </Dropdown>
              </Space.Compact>
            ) : (
              <Button type="primary" onClick={() => navigate("/providers/new")}>
                {t("create")}
              </Button>
            )}
          </PermissionGuard>
        </Space>
      ),
      columns: [
        {
          title: t("name"),
          dataIndex: "name",
          key: "name",
          width: 200,
          fixed: "left",
          render: (name: string, record: UnifiedProviderRow) => (
            <Space size={8}>
              <ProviderIcon type={record.type} size={22} />
              <Link to={editPath(record)}>{name}</Link>
            </Space>
          ),
        },
        { title: t("displayName"), dataIndex: "displayName", key: "displayName" },
        {
          title: t("category"),
          dataIndex: "category",
          key: "category",
          render: (v: string) => <ProviderCategoryBadge category={v} />,
        },
        { title: t("subtype"), dataIndex: "type", key: "type" },
        { title: t("clientId"), dataIndex: "clientId", key: "clientId" },
        {
          title: t("status"),
          dataIndex: "enabled",
          key: "enabled",
          width: 90,
          render: (v: boolean) =>
            v ? <Tag color="green">{t("enabled")}</Tag> : <Tag color="red">{t("disabled")}</Tag>,
        },
        {
          title: t("organization"),
          dataIndex: "organizationId",
          key: "organizationId",
          render: (v: string) => (v ? orgNameMap.get(v) || v : "-"),
        },
        {
          title: t("actions"),
          width: 200,
          key: "actions",
          fixed: "right",
          render: (_: unknown, record: UnifiedProviderRow) => {
            const editButton = (
              <LinkButton type="link" size="small" to={editPath(record)}>
                {t("edit")}
              </LinkButton>
            );
            const deleteButton = (
              <Popconfirm
                title={t("confirmDelete")}
                description={record.name}
                okText={t("delete")}
                cancelText={t("cancel")}
                okButtonProps={{ danger: true, loading: list.deleteMutation.isPending }}
                onConfirm={() =>
                  list.deleteMutation.mutate({
                    id: record.id,
                    organizationId: record.organizationId,
                    source: record.source,
                  })
                }
              >
                <Button type="link" danger size="small">
                  {t("delete")}
                </Button>
              </Popconfirm>
            );
            // 统一行沿用原统一 Tab 的行级权限守卫；专用行沿用原面板（不加行级守卫）。
            if (record.source === "unified") {
              return (
                <Space size={0}>
                  <PermissionGuard action="provider:write" resource="providers" fallback={null}>
                    {editButton}
                  </PermissionGuard>
                  <PermissionGuard action="provider:delete" resource="providers" fallback={null}>
                    {deleteButton}
                  </PermissionGuard>
                </Space>
              );
            }
            return (
              <Space size={0}>
                {editButton}
                {deleteButton}
              </Space>
            );
          },
        },
      ],
    },
    list,
  );

  return unifiedView;
}
