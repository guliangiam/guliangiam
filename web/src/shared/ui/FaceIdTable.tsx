import { Button, Image, Space, Table, Tag } from "antd";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import type { FC } from "react";
import { useTranslation } from "react-i18next";
import type { FaceIdItem } from "@/shared/api/iam/user";

type Props = {
  value: FaceIdItem[];
  onRemove?: (index: number) => void;
  readOnly?: boolean;
  /** 是否在表格中展示 Provider 列（后端返回 provider 数据时启用）。 */
  showProviderColumn?: boolean;
  /** "注册新 FaceID" 按钮的回调。传入时在表格上方展示按钮。 */
  onRegisterNew?: () => void;
  /** 当前选中的 FaceID Provider（外部管理），供父组件同步上下文。 */
  selectedProvider?: string;
};

const FaceIdTable: FC<Props> = ({
  value,
  onRemove,
  readOnly,
  showProviderColumn = false,
  onRegisterNew,
}) => {
  const { t } = useTranslation();

  const columns = [
    {
      title: "ID",
      dataIndex: "id",
      key: "id",
      render: (id: string) => <Tag>{id}</Tag>,
    },
    {
      title: t("resource:user.faceIdImage", "Image"),
      dataIndex: "imageUrl",
      key: "imageUrl",
      render: (url: string) =>
        url ? (
          <Image
            src={url}
            alt="Face ID"
            width={48}
            height={48}
            style={{ objectFit: "cover", borderRadius: 4 }}
          />
        ) : (
          "-"
        ),
    },
    ...(showProviderColumn
      ? [
          {
            title: t("resource:user.faceIdProvider", "Provider"),
            dataIndex: "provider",
            key: "provider",
            render: (provider?: string) => provider || "-",
          },
        ]
      : []),
    {
      title: t("resource:user.registeredAt", "Registered At"),
      dataIndex: "createdAt",
      key: "createdAt",
    },
    ...(readOnly || !onRemove
      ? []
      : [
          {
            title: "",
            width: 48,
            key: "actions",
            render: (_: unknown, __: FaceIdItem, index: number) => (
              <Button
                type="text"
                danger
                size="small"
                icon={<DeleteOutlined />}
                onClick={() => onRemove(index)}
              />
            ),
          },
        ]),
  ];

  return (
    <Space orientation="vertical" style={{ width: "100%" }} size="middle">
      {onRegisterNew && !readOnly && (
        <div>
          <Button type="primary" size="small" icon={<PlusOutlined />} onClick={onRegisterNew}>
            {t("resource:user.faceIdRegisterNew", "Register New Face ID")}
          </Button>
        </div>
      )}
      <Table
        dataSource={value.map((item, i) => ({ ...item, _idx: i }))}
        columns={columns}
        pagination={false}
        size="small"
        rowKey="_idx"
      />
    </Space>
  );
};

export default FaceIdTable;
