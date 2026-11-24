import { Tag } from "antd";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { getVerificationsPaginated } from "@/shared/api/iam/verification";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";

type VerificationRow = {
  id: string;
  type: string;
  recipient: string;
  status: string;
  expiresAt?: string;
  createdAt?: string;
};

type VerificationParams = {
  page: number;
  pageSize: number;
  search?: string;
  type?: string;
  status?: string;
};

export default function VerificationsPage() {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const account = useSessionStore((state) => state.account);

  return renderCrudListPage<VerificationRow, VerificationParams, string>({
    // ── UI ──
    title: tr("verification.title"),
    description: tr("verification.description"),
    searchPlaceholder: tr("verification.searchPlaceholder"),
    rowKey: "id",
    filters: [
      {
        key: "type",
        label: t("type"),
        options: [
          { label: tr("verification.email"), value: "email" },
          { label: tr("verification.sms"), value: "sms" },
        ],
      },
      {
        key: "status",
        label: t("statusLabel"),
        options: [
          { label: tr("verification.sent"), value: "sent" },
          { label: tr("verification.used"), value: "used" },
          { label: tr("verification.expired"), value: "expired" },
        ],
      },
    ],
    columns: [
      { title: t("type"), dataIndex: "type", key: "type", width: 100 },
      { title: tr("verification.recipient"), dataIndex: "recipient", key: "recipient", width: 200 },
      {
        title: t("statusLabel"),
        dataIndex: "status",
        key: "status",
        width: 100,
        render: (v: string) => {
          const color = v === "used" ? "green" : v === "expired" ? "red" : "blue";
          return <Tag color={color}>{v}</Tag>;
        },
      },
      { title: tr("verification.expiresAt"), dataIndex: "expiresAt", key: "expiresAt", width: 180 },
      { title: tr("verification.createdAt"), dataIndex: "createdAt", key: "createdAt", width: 180 },
    ],

    // ── Hooks ──
    enabled: account !== null,
    buildQueryKey: (p) =>
      ["verifications", "paginated", p.page, p.pageSize, p.search, p.type, p.status] as const,
    buildParams: ({ page, pageSize, search, filters }) => ({
      page,
      pageSize,
      search: search || undefined,
      type: (filters.type as string | undefined) || undefined,
      status: (filters.status as string | undefined) || undefined,
    }),
    paginatedFn: getVerificationsPaginated,
  });
}
