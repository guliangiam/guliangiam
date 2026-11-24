import { Button, Input, Select, Space, Table } from "antd";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import { type FC, useCallback } from "react";

import { useTranslation } from "react-i18next";
export type SamlAttribute = {
  name: string;
  value: string;
};

const USER_FIELD_OPTIONS = [
  "id",
  "username",
  "displayName",
  "email",
  "phone",
  "firstName",
  "lastName",
  "organization",
  "avatar",
  "isAdmin",
  "tag",
].map((f) => ({ label: `user.${f}`, value: `user.${f}` }));

type Props = {
  value: SamlAttribute[];
  onChange: (value: SamlAttribute[]) => void;
  readOnly?: boolean;
};

const SamlAttributeTable: FC<Props> = ({ value, onChange, readOnly }) => {
  const { t } = useTranslation("common");

  const handleAdd = useCallback(() => {
    onChange([...value, { name: "", value: "" }]);
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
    (index: number, field: keyof SamlAttribute, val: string) => {
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
      width: "40%",
      render: (_: string, __: SamlAttribute, index: number) =>
        readOnly ? (
          value[index]?.name || "-"
        ) : (
          <Input
            value={value[index]?.name}
            onChange={(e) => handleChange(index, "name", e.target.value)}
            placeholder="SAML attribute name"
            size="small"
          />
        ),
    },
    {
      title: "Value",
      dataIndex: "value",
      width: "50%",
      render: (_: string, __: SamlAttribute, index: number) =>
        readOnly ? (
          value[index]?.value || "-"
        ) : (
          <Select
            value={value[index]?.value || undefined}
            onChange={(v) => handleChange(index, "value", v)}
            options={[...USER_FIELD_OPTIONS, { label: "Custom value", value: "__custom__" }]}
            showSearch
            allowClear
            placeholder={t("selectUserFieldOrCustomValue")}
            size="small"
            style={{ width: "100%" }}
            filterOption={(input, option) =>
              (option?.label as string)?.toLowerCase().includes(input.toLowerCase()) ?? false
            }
          />
        ),
    },
    {
      title: "",
      width: "10%",
      render: (_: unknown, __: SamlAttribute, index: number) =>
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

export default SamlAttributeTable;
