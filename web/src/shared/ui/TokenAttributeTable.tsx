import { useTranslation } from "react-i18next";
import { Button, Input, Select, Space, Table } from "antd";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import { type FC, useCallback } from "react";

export type TokenAttribute = {
  name: string;
  sourceField: string;
  transform?: string;
};

const USER_FIELD_OPTIONS = [
  "id",
  "username",
  "displayName",
  "email",
  "phone",
  "avatar",
  "firstName",
  "lastName",
  "organization",
  "isAdmin",
  "tag",
  "score",
  "balance",
  "createdAt",
].map((f) => ({ label: f, value: f }));

type Props = {
  value: TokenAttribute[];
  onChange: (value: TokenAttribute[]) => void;
  readOnly?: boolean;
};

const TokenAttributeTable: FC<Props> = ({ value, onChange, readOnly }) => {
  const { t } = useTranslation("common");

  const handleAdd = useCallback(() => {
    onChange([...value, { name: "", sourceField: "", transform: "" }]);
  }, [value, onChange]);

  const handleRemove = useCallback(
    (index: number) => {
      const next = [...value];
      next.splice(index, 1);
      onChange(next);
    },
    [value, onChange],
  );

  const handleChange = useCallback(
    (index: number, field: keyof TokenAttribute, val: string) => {
      const next = [...value];
      next[index] = { ...next[index], [field]: val };
      onChange(next);
    },
    [value, onChange],
  );

  const columns = [
    {
      title: "Attribute Name",
      dataIndex: "name",
      width: "30%",
      render: (_: string, __: TokenAttribute, index: number) =>
        readOnly ? (
          value[index]?.name || "-"
        ) : (
          <Input
            value={value[index]?.name}
            onChange={(e) => handleChange(index, "name", e.target.value)}
            placeholder="Attribute name"
            size="small"
          />
        ),
    },
    {
      title: "Source Field",
      dataIndex: "sourceField",
      width: "35%",
      render: (_: string, __: TokenAttribute, index: number) =>
        readOnly ? (
          value[index]?.sourceField || "-"
        ) : (
          <Select
            value={value[index]?.sourceField || undefined}
            onChange={(v) => handleChange(index, "sourceField", v)}
            options={USER_FIELD_OPTIONS}
            showSearch
            allowClear
            placeholder={t("selectUserField")}
            size="small"
            style={{ width: "100%" }}
          />
        ),
    },
    {
      title: "Transform",
      dataIndex: "transform",
      width: "25%",
      render: (_: string, __: TokenAttribute, index: number) =>
        readOnly ? (
          value[index]?.transform || "-"
        ) : (
          <Input
            value={value[index]?.transform}
            onChange={(e) => handleChange(index, "transform", e.target.value)}
            placeholder="Optional transform"
            size="small"
          />
        ),
    },
    {
      title: "",
      width: "10%",
      render: (_: unknown, __: TokenAttribute, index: number) =>
        readOnly ? null : (
          <Button
            type="text"
            danger
            icon={<DeleteOutlined />}
            onClick={() => handleRemove(index)}
            size="small"
          />
        ),
    },
  ];

  return (
    <div>
      <Table
        columns={columns}
        dataSource={value.map((item, i) => ({ ...item, key: i }))}
        pagination={false}
        size="small"
        bordered
      />
      {!readOnly && (
        <Space style={{ marginTop: 8 }}>
          <Button type="dashed" onClick={handleAdd} icon={<PlusOutlined />} size="small">
            Add Attribute
          </Button>
        </Space>
      )}
    </div>
  );
};

export default TokenAttributeTable;
