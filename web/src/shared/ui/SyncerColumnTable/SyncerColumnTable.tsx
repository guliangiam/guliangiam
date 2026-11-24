import { Button, Input, Select, Space, Table } from "antd";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import { type FC, useCallback } from "react";

export type SyncerColumnItem = {
  columnName: string;
  columnType: string;
  sourceFieldName: string;
};

const COLUMN_TYPE_OPTIONS = [
  { label: "String", value: "string" },
  { label: "Integer", value: "int" },
  { label: "Boolean", value: "bool" },
  { label: "Datetime", value: "datetime" },
];

type Props = {
  value?: SyncerColumnItem[];
  onChange?: (value: SyncerColumnItem[]) => void;
  readOnly?: boolean;
};

const SyncerColumnTable: FC<Props> = ({ value = [], onChange, readOnly }) => {
  const handleAdd = useCallback(() => {
    onChange?.([...value, { columnName: "", columnType: "string", sourceFieldName: "" }]);
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
    (index: number, field: keyof SyncerColumnItem, val: string) => {
      const next = [...value];
      next[index] = { ...next[index], [field]: val };
      onChange?.(next);
    },
    [value, onChange],
  );

  const columns = [
    {
      title: "Column Name",
      dataIndex: "columnName",
      width: "30%",
      render: (_: string, __: SyncerColumnItem, index: number) =>
        readOnly ? (
          value[index]?.columnName || "-"
        ) : (
          <Input
            value={value[index]?.columnName}
            onChange={(e) => handleChange(index, "columnName", e.target.value)}
            placeholder="Column name"
            size="small"
          />
        ),
    },
    {
      title: "Column Type",
      dataIndex: "columnType",
      width: "20%",
      render: (_: string, __: SyncerColumnItem, index: number) =>
        readOnly ? (
          value[index]?.columnType || "-"
        ) : (
          <Select
            value={value[index]?.columnType || "string"}
            onChange={(v) => handleChange(index, "columnType", v)}
            options={COLUMN_TYPE_OPTIONS}
            size="small"
            style={{ width: "100%" }}
          />
        ),
    },
    {
      title: "Source Field",
      dataIndex: "sourceFieldName",
      width: "40%",
      render: (_: string, __: SyncerColumnItem, index: number) =>
        readOnly ? (
          value[index]?.sourceFieldName || "-"
        ) : (
          <Input
            value={value[index]?.sourceFieldName}
            onChange={(e) => handleChange(index, "sourceFieldName", e.target.value)}
            placeholder="Source field name"
            size="small"
          />
        ),
    },
    {
      title: "",
      width: "10%",
      render: (_: unknown, __: SyncerColumnItem, index: number) =>
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
            Add Column
          </Button>
        </Space>
      )}
    </div>
  );
};

export { SyncerColumnTable };
