import { App, Button, Popconfirm, Tag } from "antd";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getIamFormsPaginated, deleteIamForm } from "@/shared/api/iam";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { LinkButton } from "@/shared/ui/LinkButton";
import { localizedError } from "@/shared/lib/server-error";

type FormRow = {
  id: string;
  name: string;
  displayName?: string;
  isEnabled: boolean;
};

type FormParams = {
  page: number;
  pageSize: number;
  search?: string;
};

export default function FormsPage() {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { message } = App.useApp();
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<FormRow, FormParams, string>({
    // ── UI ──
    title: tr("form.title"),
    description: tr("form.description"),
    searchPlaceholder: tr("form.searchPlaceholder"),
    rowKey: "id",
    toolbar: (
      <PermissionGuard action="form:write" resource="forms" fallback={null}>
        <LinkButton type="primary" to="/forms/new">
          {tr("form.newForm")}
        </LinkButton>
      </PermissionGuard>
    ),
    columns: [
      {
        title: t("name"),
        dataIndex: "name",
        key: "name",
        width: 200,
        fixed: "left",
        render: (v: string, record: FormRow) => <Link to={`/forms/${record.id}`}>{v}</Link>,
      },
      { title: t("displayName"), dataIndex: "displayName", key: "displayName" },
      {
        title: t("statusLabel"),
        dataIndex: "isEnabled",
        key: "isEnabled",
        render: (v: boolean) =>
          v ? <Tag color="green">{t("enabled")}</Tag> : <Tag color="red">{t("disabled")}</Tag>,
      },
      {
        title: t("actions"),
        key: "actions",
        width: 160,
        fixed: "right",
        render: (_, record) => (
          <div style={{ display: "flex", gap: 4, flexWrap: "nowrap" }}>
            <LinkButton type="link" size="small" to={`/forms/${record.id}`}>
              {t("edit")}
            </LinkButton>
            <Popconfirm
              title={t("confirmDelete")}
              onConfirm={() =>
                deleteIamForm(record.id)
                  .then(() => message.success(t("deleteSuccess")))
                  .catch((e) => message.error(localizedError(e, tr, "common:error")))
              }
            >
              <Button type="link" size="small" danger>
                {t("delete")}
              </Button>
            </Popconfirm>
          </div>
        ),
      },
    ],

    // ── Hooks ──
    enabled: account !== null,
    buildQueryKey: (p) => ["forms", "paginated", p.page, p.pageSize, p.search] as const,
    buildParams: ({ page, pageSize, search }) => ({
      page,
      pageSize,
      search: search || undefined,
    }),
    paginatedFn: getIamFormsPaginated,
    deleteFn: deleteIamForm,
    invalidateKeysOnDelete: [["forms"]],
    onDeleteSuccess: () => message.success(t("deleteSuccess")),
    enableBulkDelete: true,
  });
}
