import { useState, useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Button, Tag, Popconfirm, Upload, App, Space } from "antd";
import { UploadOutlined } from "@ant-design/icons";
import { Link, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import {
  deleteGroup,
  importGroups,
  importGroupsFromExcel,
  type ImportGroupItem,
  getGroupsPaginated,
  type GroupListParams,
} from "@/shared/api/iam/group";
import { renderCrudListView } from "@/shared/ui/CrudListPage";
import { useCrudList } from "@/shared/hooks/useCrudList";
import { OrganizationSelect } from "@/shared/ui/OrganizationSelect";
import { useOrganizationNames } from "@/shared/hooks/useResourceNames";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { BatchUploadModal } from "@/shared/components/BatchUploadModal";
import { LinkButton } from "@/shared/ui/LinkButton";
import { groupPath } from "@/shared/lib/routePaths";
import { localizedError } from "@/shared/lib/server-error";

type GroupRow = {
  id: string;
  owner: string;
  name: string;
  displayName?: string;
  type?: string;
  isEnabled?: boolean;
};

// 平台管理员跨组织删组必须显式携带行所属组织（owner 即 organizationId），
// 否则后端回落会话组织 → 404。
type GroupDeleteKey = { id: string; organizationId?: string };

export default function GroupsPage() {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);
  const organization = useSessionStore((state) => state.organization);
  const queryClient = useQueryClient();

  const { nameMap: orgNameMap } = useOrganizationNames();

  // ── 页面特有的状态（renderCrudListPage 覆盖不到）──
  // 组织列表行的「组」快捷入口会带 ?organizationId=<id>，进入即建立组织上下文。
  // URL 是唯一可信来源：显式选择记录在 orgSelection 中，且只在它绑定的 URL 上下文
  // 仍然成立时生效；前进/后退或同页跳转改变 URL 后它会自动失效，不会覆盖 URL。
  // 保留显式清除（值为 ""）语义，避免回落到会话组织而无法查看全部组织。
  const [searchParams, setSearchParams] = useSearchParams();
  const orgContext = searchParams.get("organizationId") ?? "";
  const [orgSelection, setOrgSelection] = useState<{ context: string; value: string } | null>(null);
  const orgFilter =
    orgSelection && orgSelection.context === orgContext
      ? orgSelection.value
      : orgContext || (organization?.id ?? "");
  const [uploadModalOpen, setUploadModalOpen] = useState(false);
  // 单入口导入：表格文件选中后先暂存，由 Excel 弹窗沿用原上传链路。
  const [pendingExcelFile, setPendingExcelFile] = useState<File | null>(null);

  // 选择/清除组织时同步 URL：清除即移除 organizationId，避免上下文被静默丢弃。
  const handleOrgChange = useCallback(
    (value: string) => {
      const next = value || "";
      // 选择结果与被写回的 URL 上下文绑定；URL 后续被历史导航改变时自动失效。
      setOrgSelection({ context: next, value: next });
      setSearchParams(
        (prev) => {
          const params = new URLSearchParams(prev);
          if (next) params.set("organizationId", next);
          else params.delete("organizationId");
          return params;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  // JSON import
  const handleImportJson = useCallback(
    (file: File) => {
      const reader = new FileReader();
      reader.onload = (e) => {
        try {
          const data = JSON.parse(e.target?.result as string) as ImportGroupItem[];
          const orgId = (orgFilter || account?.organization?.id) ?? "";
          importGroups(orgId, data).then(
            (result) => {
              message.success(
                tc("importSuccessWithCount", { succeeded: result.succeeded, total: result.total }),
              );
              if (result.failed > 0) {
                message.warning(`${tc("importPartial")}: ${result.errors?.slice(0, 3).join("; ")}`);
              }
              void queryClient.invalidateQueries({ queryKey: ["groups"] });
              return;
            },
            (err) => {
              message.error(`${tc("operationFailed")}: ${localizedError(err, tc, "error")}`);
            },
          );
        } catch {
          message.error(tc("invalidJsonFile"));
        }
      };
      reader.readAsText(file);
      return false; // prevent default upload
    },
    [account, orgFilter, message, tc, queryClient],
  );

  // 一个「导入」入口按扩展名分流：.json 走批量 JSON 接口，表格文件走 Excel 上传弹窗。
  const handleImportFile = useCallback(
    (file: File) => {
      if (/\.json$/i.test(file.name)) return handleImportJson(file);
      setPendingExcelFile(file);
      setUploadModalOpen(true);
      return false;
    },
    [handleImportJson],
  );

  const closeUploadModal = useCallback(() => {
    setUploadModalOpen(false);
    setPendingExcelFile(null);
  }, []);

  // 闭集 virtual/physical/department（G7-s）：筛选与列展示共用同一词表。
  const groupTypeOptions = [
    { label: t("group.typeVirtual"), value: "virtual" },
    { label: t("group.typePhysical"), value: "physical" },
    { label: t("group.typeDepartment"), value: "department" },
  ];

  // 删除只保留一条实现路径：行内确认与批量删除共用同一个 deleteMutation，
  // 由它统一负责 DELETE 请求、列表失效与成功提示。两者的删除键不同：行内直接
  // 传 record.id；批量选中的是 rowKey（`owner/name`），由 rowKeyToDeleteKey
  // 映射回 record.id——后端只有单段路由 DELETE /api/v1/groups/{id}。
  const list = useCrudList<GroupRow, GroupListParams, GroupDeleteKey>({
    buildQueryKey: (p) =>
      [
        "groups",
        "paginated",
        p.page,
        p.pageSize,
        p.search,
        p.sortBy,
        p.sortOrder,
        p.organizationId,
        p.status,
        p.type,
      ] as const,
    buildParams: ({ page, pageSize, search, filters, sortField, sortOrder }) => ({
      page,
      pageSize,
      search: search || undefined,
      organizationId: orgFilter || undefined,
      sortBy: sortField || undefined,
      sortOrder: sortOrder ? (sortOrder === "ascend" ? "asc" : "desc") : undefined,
      // 后端 status 词表是 active/inactive（sql.go CASE 映射），不是 enabled/disabled。
      status: filters.isEnabled
        ? filters.isEnabled === "enabled"
          ? "active"
          : "inactive"
        : undefined,
      type: filters.type || undefined,
    }),
    paginatedFn: getGroupsPaginated,
    deleteFn: (key) => deleteGroup(key.id, key.organizationId),
    invalidateKeysOnDelete: [["groups"]],
    onDeleteSuccess: () => message.success(tc("deleteSuccess")),
    enabled: account !== null,
    defaultSortField: "name",
    defaultSortOrder: "ascend",
  });

  return renderCrudListView<GroupRow, GroupListParams, GroupDeleteKey>(
    {
      // ── UI ──
      title: t("group.title"),
      description: t("group.description"),
      searchPlaceholder: t("group.searchPlaceholder"),
      rowKey: (r) => `${r.owner}/${r.name}`,
      rowKeyToDeleteKey: (r) => ({ id: r.id, organizationId: r.owner }),
      defaultSortField: "name",
      defaultSortOrder: "ascend",
      searchableColumns: [
        { dataIndex: "name", label: tc("name") },
        { dataIndex: "displayName", label: tc("displayName") },
      ],
      filters: [
        {
          key: "isEnabled",
          label: tc("statusLabel"),
          options: [
            { label: tc("enabled"), value: "enabled" },
            { label: tc("disabled"), value: "disabled" },
          ],
        },
        {
          key: "type",
          label: tc("type"),
          options: groupTypeOptions,
        },
      ],

      columns: [
        {
          title: tc("name"),
          dataIndex: "name",
          key: "name",
          width: 200,
          fixed: "left",
          sorter: true,
          render: (v: string, r: GroupRow) =>
            r.owner && r.name ? <Link to={groupPath(r.owner, r.name)}>{v}</Link> : v,
        },
        { title: tc("displayName"), dataIndex: "displayName", key: "displayName", sorter: true },
        {
          title: tc("type"),
          dataIndex: "type",
          key: "type",
          render: (v: string) => groupTypeOptions.find((o) => o.value === v)?.label ?? (v || "—"),
        },
        {
          title: tc("statusLabel"),
          dataIndex: "isEnabled",
          key: "isEnabled",
          render: (v: boolean) =>
            v ? <Tag color="green">{tc("enabled")}</Tag> : <Tag color="red">{tc("disabled")}</Tag>,
        },
        {
          title: tc("actions"),
          key: "actions",
          width: 160,
          fixed: "right",
          render: (_, record) => (
            <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
              <LinkButton type="link" size="small" to={groupPath(record.owner, record.name)}>
                {tc("edit")}
              </LinkButton>
              <PermissionGuard action="group:delete" resource="groups" fallback={null}>
                <Popconfirm
                  title={tc("confirmDelete")}
                  description={record.displayName || record.name}
                  okText={tc("delete")}
                  cancelText={tc("cancel")}
                  okButtonProps={{ danger: true, loading: list.deleteMutation.isPending }}
                  onConfirm={() =>
                    list.deleteMutation.mutate({ id: record.id, organizationId: record.owner })
                  }
                >
                  <Button type="link" size="small" danger>
                    {tc("delete")}
                  </Button>
                </Popconfirm>
              </PermissionGuard>
            </div>
          ),
        },
      ],

      // ── Bulk Delete ──
      enableBulkDelete: true,
      bulkDeletePermission: { permission: "group:delete", resource: "groups" },

      // ── Toolbar ──
      toolbar: (
        <Space>
          {account?.isAdmin ? (
            <OrganizationSelect
              value={orgFilter}
              onChange={handleOrgChange}
              style={{ width: 180 }}
              allowClear
            />
          ) : orgContext ? (
            // 非平台管理员从组织列表快捷入口进入时，也要让组织上下文可见、可清除。
            <Tag closable onClose={() => handleOrgChange("")} color="blue">
              {tc("organization")}: {orgNameMap.get(orgContext) ?? orgContext}
            </Tag>
          ) : null}
          <PermissionGuard action="group:write" resource="groups" fallback={null}>
            <Upload
              accept=".json,.xlsx,.xls,.csv"
              showUploadList={false}
              beforeUpload={handleImportFile}
            >
              <Button icon={<UploadOutlined />}>{tc("import")}</Button>
            </Upload>
          </PermissionGuard>
          <BatchUploadModal
            open={uploadModalOpen}
            onClose={closeUploadModal}
            initialFile={pendingExcelFile}
            onUpload={async (file) => {
              const result = await importGroupsFromExcel(
                file,
                orgFilter || organization?.id || undefined,
              );
              await queryClient.invalidateQueries({ queryKey: ["groups"] });
              return result;
            }}
            title={t("group.uploadTitle")}
            template={{
              fileName: "groups-import-template.csv",
              // 列顺序与后端 ImportGroupsFromExcel 解析一致：Name, DisplayName, Type, ParentID
              columns: ["Name", "DisplayName", "Type", "ParentID"],
              sample: ["engineering", "工程部", "Physical", ""],
            }}
          />
          <PermissionGuard action="group:write" resource="groups" fallback={null}>
            <LinkButton type="primary" to="/groups/new">
              {t("group.newGroup")}
            </LinkButton>
          </PermissionGuard>
        </Space>
      ),
    },
    list,
  );
}
