import { Button, Empty, Space, Table, Tag } from "antd";
import { DeleteOutlined, LinkOutlined, SyncOutlined } from "@ant-design/icons";
import { useCallback } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import type { IamLdapItem } from "@/shared/api/iam/ldap";

type LdapTableProps = {
  value: IamLdapItem[];
  organizationId: string;
  onSync?: (ldapId: string) => void;
  onDelete?: (ldapId: string) => void;
  readOnly?: boolean;
};

export function LdapTable({ value, onSync, onDelete, readOnly = false }: LdapTableProps) {
  const { t } = useTranslation("common");
  const navigate = useNavigate();

  const handleNavigate = useCallback(
    (id: string) => {
      navigate(`/ldaps/${id}`);
    },
    [navigate],
  );

  if (value.length === 0) {
    return <Empty description={t("noData")} image={Empty.PRESENTED_IMAGE_SIMPLE} />;
  }

  const columns = [
    {
      title: t("resource:ldap.serverName"),
      dataIndex: "displayName",
      key: "displayName",
      render: (text: string, record: IamLdapItem) => (
        <Button type="link" size="small" onClick={() => handleNavigate(record.id)}>
          {text || ((record as Record<string, unknown>).serverName as string) || record.name}
        </Button>
      ),
    },
    {
      title: t("resource:ldap.server"),
      key: "server",
      width: 200,
      render: (_: unknown, record: IamLdapItem) => (
        <span>
          {record.host}:{record.port}
          {(record as Record<string, unknown>).enableSsl ? (
            <Tag color="blue" style={{ marginLeft: 4 }}>
              SSL
            </Tag>
          ) : null}
        </span>
      ),
    },
    {
      title: t("resource:ldap.baseDn"),
      dataIndex: "baseDn",
      key: "baseDn",
      ellipsis: true,
      width: 250,
    },
    {
      title: t("resource:ldap.syncInterval"),
      key: "syncInterval",
      width: 120,
      render: (_: unknown, record: IamLdapItem) => {
        const interval = ((record as Record<string, unknown>).autoSync as number) ?? 0;
        if (!interval) return <Tag>{t("disabled")}</Tag>;
        const mins = Math.round(interval / 60);
        return <Tag color="green">{mins} min</Tag>;
      },
    },
    {
      title: t("resource:ldap.lastSyncAt"),
      dataIndex: "lastSyncAt",
      key: "lastSyncAt",
      width: 160,
      ellipsis: true,
      render: (val: string) => val || "-",
    },
    ...(readOnly
      ? []
      : [
          {
            title: t("actions"),
            key: "actions",
            width: 120,
            render: (_: unknown, record: IamLdapItem) => (
              <Space size={4}>
                {onSync && (
                  <Button
                    type="text"
                    size="small"
                    icon={<SyncOutlined />}
                    onClick={() => onSync(record.id)}
                    title={t("resource:ldap.syncAction")}
                  />
                )}
                <Button
                  type="text"
                  size="small"
                  icon={<LinkOutlined />}
                  onClick={() => handleNavigate(record.id)}
                  title={t("edit")}
                />
                {onDelete && (
                  <Button
                    type="text"
                    size="small"
                    danger
                    icon={<DeleteOutlined />}
                    onClick={() => onDelete(record.id)}
                    title={t("delete")}
                  />
                )}
              </Space>
            ),
          },
        ]),
  ];

  return (
    <Table
      dataSource={value}
      columns={columns}
      rowKey="id"
      pagination={false}
      size="small"
      scroll={{ x: "max-content" }}
    />
  );
}
