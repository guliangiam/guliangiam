import { Alert, Table, Tag } from "antd";
import type { UseQueryResult } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { FormSection } from "@/shared/ui/FormSection";
import type { UserConsentItem } from "@/shared/api/iam/user-types";

type AccountConsentsSectionProps = {
  consentsQuery: UseQueryResult<UserConsentItem[], Error>;
};

/**
 * 自助授权记录（只读）。数据来自会话绑定的 GET /auth/consents。
 * 自助撤销 consent 端点尚不存在，故本段不提供撤销入口（不复用 admin
 * revokeConsent，避免普通成员触发 403）。
 */
export function AccountConsentsSection({ consentsQuery }: AccountConsentsSectionProps) {
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");

  const consents = consentsQuery.data ?? [];

  const columns = [
    {
      title: t("user.consents.application"),
      dataIndex: "applicationSlug",
      key: "applicationSlug",
      render: (v: string, record: UserConsentItem) => v || record.applicationId,
    },
    {
      title: t("user.consents.scopes"),
      dataIndex: "grantedScopes",
      key: "grantedScopes",
      render: (v: string[]) => v?.map((s) => <Tag key={s}>{s}</Tag>),
    },
    {
      title: t("user.consents.grantedAt"),
      dataIndex: "createdAt",
      key: "createdAt",
      render: (v: string) => (v ? new Date(v).toLocaleString() : "-"),
    },
    {
      title: t("user.consents.status"),
      key: "status",
      render: (_: unknown, record: UserConsentItem) =>
        record.revokedAt ? (
          <Tag color="red">{t("user.consents.revoked")}</Tag>
        ) : (
          <Tag color="green">{t("user.consents.active")}</Tag>
        ),
    },
  ];

  return (
    <FormSection title={t("user.tabs.consents")}>
      {!consentsQuery.isLoading && consents.length === 0 ? (
        <Alert type="info" showIcon title={tc("noConsents")} />
      ) : (
        <Table<UserConsentItem>
          dataSource={consents}
          columns={columns}
          loading={consentsQuery.isLoading}
          pagination={false}
          size="small"
          rowKey="id"
        />
      )}
    </FormSection>
  );
}
