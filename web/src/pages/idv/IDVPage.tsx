import { Tag } from "antd";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { listIDVVerifications } from "@/shared/api/iam/idv";
import { renderCrudListPage } from "@/shared/ui/CrudListPage";

const STATUS_COLORS: Record<string, string> = {
  pending: "gold",
  in_progress: "blue",
  verified: "green",
  failed: "red",
  expired: "default",
};

type IDVRow = {
  id: string;
  userId?: string;
  provider: string;
  status: string;
  confidence?: number;
  initiatedAt?: string;
};

type IDVParams = {
  page: number;
  pageSize: number;
};

export default function IDVPage() {
  const { t: tc } = useTranslation("common");
  const account = useSessionStore((state) => state.account);
  const organization = useSessionStore((state) => state.organization);

  return renderCrudListPage<IDVRow, IDVParams, string>({
    // ── UI ──
    title: tc("nav.idv", { defaultValue: "Identity Verification" }),
    description: tc("idv.description", {
      defaultValue: "Identity verification records across providers (IDV)",
    }),
    rowKey: "id",
    columns: [
      {
        title: tc("idv.user", { defaultValue: "User" }),
        dataIndex: "userId",
        key: "userId",
        width: 200,
        fixed: "left",
        render: (v?: string) => v ?? "-",
      },
      {
        title: tc("idv.provider", { defaultValue: "Provider" }),
        dataIndex: "provider",
        key: "provider",
      },
      {
        title: tc("idv.status", { defaultValue: "Status" }),
        dataIndex: "status",
        key: "status",
        render: (s: string) => <Tag color={STATUS_COLORS[s] ?? "default"}>{s}</Tag>,
      },
      {
        title: tc("idv.confidence", { defaultValue: "Confidence" }),
        dataIndex: "confidence",
        key: "confidence",
        render: (c?: number | null) =>
          c !== null && c !== undefined ? `${Math.round(c * 100)}%` : "-",
      },
      {
        title: tc("idv.initiatedAt", { defaultValue: "Initiated" }),
        dataIndex: "initiatedAt",
        key: "initiatedAt",
      },
    ],

    // ── Hooks ──
    enabled: account !== null && !!organization,
    buildQueryKey: (p) =>
      ["idv-verifications", "paginated", organization?.id ?? "", p.page, p.pageSize] as const,
    buildParams: ({ page, pageSize }) => ({ page, pageSize }),
    paginatedFn: async (p) => {
      const result = await listIDVVerifications({
        // 后端按 UUID 校验 organizationId；传会话里的 organizations.id，
        // 不能用 slug 或占位符（"me" 会让 uuid 列报 22P02 / 400）。
        organizationId: organization?.id ?? "",
        page: p.page,
        pageSize: p.pageSize,
      });
      return {
        items: (result?.items ?? []) as IDVRow[],
        total: result?.page?.total ?? 0,
      };
    },
  });
}
