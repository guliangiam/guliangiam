import { Button, Empty, Space, Table, Tag } from "antd";
import type { TablePaginationConfig } from "antd";
import { DeleteOutlined, EyeOutlined } from "@ant-design/icons";
import { memo, useCallback } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import type { IamTransactionItem } from "@/shared/api/iam/transaction";

const STATUS_COLOR_MAP: Record<string, string> = {
  completed: "green",
  pending: "orange",
  failed: "red",
  refunded: "purple",
  cancelled: "default",
};

type TransactionTableProps = {
  value: IamTransactionItem[];
  onView?: (id: string) => void;
  onDelete?: (id: string) => void;
  readOnly?: boolean;
  /** 详情页内嵌子表使用受控分页；默认不分页。 */
  pagination?: TablePaginationConfig | false;
};

export const TransactionTable = memo(function TransactionTable({
  value,
  onView,
  onDelete,
  readOnly = false,
  pagination = false,
}: TransactionTableProps) {
  const { t } = useTranslation("common");
  const navigate = useNavigate();

  const handleView = useCallback(
    (id: string) => {
      if (onView) {
        onView(id);
      } else {
        navigate(`/transactions/${id}`);
      }
    },
    [navigate, onView],
  );

  if (value.length === 0) {
    return <Empty description={t("noData")} image={Empty.PRESENTED_IMAGE_SIMPLE} />;
  }

  const columns = [
    {
      title: t("resource:transaction.type"),
      dataIndex: "transactionType",
      key: "transactionType",
      width: 120,
      render: (type: string) => <Tag>{type}</Tag>,
    },
    {
      title: t("resource:transaction.amount"),
      key: "amount",
      width: 140,
      render: (_: unknown, record: IamTransactionItem) => (
        <span>
          {record.currency ? `${record.currency} ` : ""}
          {record.amount?.toFixed(2) ?? "0.00"}
        </span>
      ),
    },
    {
      title: t("resource:transaction.status"),
      dataIndex: "status",
      key: "status",
      width: 110,
      render: (status: string) => <Tag color={STATUS_COLOR_MAP[status] ?? "default"}>{status}</Tag>,
    },
    {
      title: t("resource:transaction.createdAt"),
      dataIndex: "createdAt",
      key: "createdAt",
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
            width: 80,
            render: (_: unknown, record: IamTransactionItem) => (
              <Space size={4}>
                <Button
                  type="text"
                  size="small"
                  icon={<EyeOutlined />}
                  onClick={() => handleView(record.id)}
                />
                {onDelete && (
                  <Button
                    type="text"
                    size="small"
                    danger
                    icon={<DeleteOutlined />}
                    onClick={() => onDelete(record.id)}
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
      pagination={pagination}
      size="small"
      scroll={{ x: "max-content" }}
    />
  );
});
