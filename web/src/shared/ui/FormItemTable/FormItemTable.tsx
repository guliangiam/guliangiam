import { Button, Input, Select, Space, Switch, Table } from "antd";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import { type FC, useCallback } from "react";

export type FormItemValue = {
  label: string;
  name: string;
  type: string;
  required: boolean;
};

const FORM_TYPE_OPTIONS = [
  { label: "Text", value: "text" },
  { label: "Number", value: "number" },
  { label: "Email", value: "email" },
  { label: "Phone", value: "phone" },
  { label: "Password", value: "password" },
  { label: "Select", value: "select" },
  { label: "Textarea", value: "textarea" },
  { label: "Switch", value: "switch" },
];

type Props = {
  value?: FormItemValue[];
  onChange?: (value: FormItemValue[]) => void;
  readOnly?: boolean;
};

const FormItemTable: FC<Props> = ({ value = [], onChange, readOnly }) => {
  const handleAdd = useCallback(() => {
    onChange?.([...value, { label: "", name: "", type: "text", required: false }]);
  }, [value, onChange]);

  const handleRemove = useCallback(
    (index: number) => {
      const next = [...value];
      next.splice(index, 1);
      onChange?.(next);
    },
    [value, onChange],
  );

  const handleChange = useCallback(
    (index: number, field: keyof FormItemValue, val: string | boolean) => {
      const next = [...value];
      next[index] = { ...next[index], [field]: val };
      onChange?.(next);
    },
    [value, onChange],
  );

  const columns = [
    {
      title: "Label",
      dataIndex: "label",
      width: "22%",
      render: (_: string, __: FormItemValue, index: number) =>
        readOnly ? (
          value[index]?.label || "-"
        ) : (
          <Input
            value={value[index]?.label}
            onChange={(e) => handleChange(index, "label", e.target.value)}
            placeholder="Label"
            size="small"
          />
        ),
    },
    {
      title: "Name",
      dataIndex: "name",
      width: "22%",
      render: (_: string, __: FormItemValue, index: number) =>
        readOnly ? (
          value[index]?.name || "-"
        ) : (
          <Input
            value={value[index]?.name}
            onChange={(e) => handleChange(index, "name", e.target.value)}
            placeholder="Field name"
            size="small"
          />
        ),
    },
    {
      title: "Type",
      dataIndex: "type",
      width: "22%",
      render: (_: string, __: FormItemValue, index: number) =>
        readOnly ? (
          value[index]?.type || "-"
        ) : (
          <Select
            value={value[index]?.type || "text"}
            onChange={(v) => handleChange(index, "type", v)}
            options={FORM_TYPE_OPTIONS}
            size="small"
            style={{ width: "100%" }}
          />
        ),
    },
    {
      title: "Required",
      dataIndex: "required",
      width: "14%",
      render: (_: unknown, __: FormItemValue, index: number) =>
        readOnly ? (
          value[index]?.required ? (
            "Yes"
          ) : (
            "No"
          )
        ) : (
          <Switch
            checked={value[index]?.required}
            onChange={(checked) => handleChange(index, "required", checked)}
            size="small"
          />
        ),
    },
    {
      title: "",
      width: "20%",
      render: (_: unknown, __: FormItemValue, index: number) =>
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
    <div className="gl-editable-table">
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
            Add Item
          </Button>
        </Space>
      )}
    </div>
  );
};

export { FormItemTable };
