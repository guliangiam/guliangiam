import { Button, Descriptions, Divider, Popconfirm, Space, Table, Tag } from "antd";
import { useTranslation } from "react-i18next";
import type { UseQueryResult, UseMutationResult } from "@tanstack/react-query";
import type { SessionItem, SessionDetail } from "@/shared/api/iam/session";
import { FormSection } from "@/shared/ui/FormSection";

type AccountSessionSectionProps = {
  sessionsQuery: UseQueryResult<SessionItem[], Error>;
  currentSessionQuery: UseQueryResult<SessionDetail, Error>;
  revokeSessionMutation: UseMutationResult<void, Error, string>;
  revokeOthersMutation: UseMutationResult<{ status: string; revokedCount: number }, Error, void>;
};

export function AccountSessionSection({
  sessionsQuery,
  currentSessionQuery,
  revokeSessionMutation,
  revokeOthersMutation,
}: AccountSessionSectionProps) {
  const { t } = useTranslation("resource");

  return (
    <FormSection title={t("account.sessions")}>
      {currentSessionQuery.data && (
        <>
          <Divider>{t("account.currentDevice")}</Divider>
          <Descriptions column={{ xs: 1, sm: 2 }} size="small">
            <Descriptions.Item label={t("account.ipAddress")}>
              {currentSessionQuery.data.ipAddress ?? "-"}
            </Descriptions.Item>
            <Descriptions.Item label={t("account.userAgent")}>
              {currentSessionQuery.data.userAgent ?? "-"}
            </Descriptions.Item>
            <Descriptions.Item label={t("account.lastSeen")}>
              {currentSessionQuery.data.lastSeenAt ?? "-"}
            </Descriptions.Item>
            <Descriptions.Item label={t("account.expiresAt")}>
              {currentSessionQuery.data.expiresAt ?? "-"}
            </Descriptions.Item>
          </Descriptions>
          <Divider style={{ margin: "12px 0" }} />
        </>
      )}
      <Space style={{ marginBottom: 12 }}>
        <Popconfirm
          title={t("common:revokeOtherSessionsConfirm")}
          onConfirm={() => revokeOthersMutation.mutate()}
          okText={t("account.revokeOtherSessions")}
          cancelText={t("common:cancel")}
        >
          <Button danger loading={revokeOthersMutation.isPending}>
            {t("account.revokeOtherSessions")}
          </Button>
        </Popconfirm>
      </Space>
      <Table
        size="small"
        loading={sessionsQuery.isLoading}
        dataSource={sessionsQuery.data ?? []}
        rowKey="id"
        columns={[
          { title: t("account.ipAddress"), dataIndex: "ipAddress", width: 140 },
          { title: t("account.userAgent"), dataIndex: "userAgent", ellipsis: true },
          { title: t("account.lastSeen"), dataIndex: "lastSeenAt", width: 170 },
          {
            title: t("account.sessionState"),
            dataIndex: "sessionState",
            width: 100,
            render: (v: string) => <Tag color={v === "active" ? "green" : "red"}>{v}</Tag>,
          },
          {
            title: t("account.actions"),
            width: 100,
            render: (_: unknown, record: SessionItem) =>
              record.sessionState === "active" ? (
                <Popconfirm
                  title={t("common:revokeSessionConfirm")}
                  onConfirm={() => revokeSessionMutation.mutate(record.id)}
                  okText={t("account.revoke")}
                  cancelText={t("common:cancel")}
                >
                  <Button size="small" danger loading={revokeSessionMutation.isPending}>
                    {t("account.revoke")}
                  </Button>
                </Popconfirm>
              ) : (
                <Tag>{t("account.revoked")}</Tag>
              ),
          },
        ]}
      />
    </FormSection>
  );
}
