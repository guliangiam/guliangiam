import { Button, Input, Space, Table } from "antd";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import { type FC, useCallback } from "react";

export type ScopeItem = {
  name: string;
  displayName: string;
};

type Props = {
  value?: ScopeItem[];
  onChange?: (value: ScopeItem[]) => void;
  readOnly?: boolean;
};

const ScopeTable: FC<Props> = ({ value = [], onChange, readOnly }) => {
  const handleAdd = useCallback(() => {
    onChange?.([...value, { name: "", displayName: "" }]);
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
    (index: number, field: keyof ScopeItem, val: string) => {
      const next = [...value];
      next[index] = { ...next[index], [field]: val };
      onChange?.(next);
    },
    [value, onChange],
  );

  const columns = [
    {
      title: "Name",
      dataIndex: "name",
      width: "45%",
      render: (_: string, __: ScopeItem, index: number) =>
        readOnly ? (
          value[index]?.name || "-"
        ) : (
          <Input
            value={value[index]?.name}
            onChange={(e) => handleChange(index, "name", e.target.value)}
            placeholder="Scope name"
            size="small"
          />
        ),
    },
    {
      title: "Display Name",
      dataIndex: "displayName",
      width: "45%",
      render: (_: string, __: ScopeItem, index: number) =>
        readOnly ? (
          value[index]?.displayName || "-"
        ) : (
          <Input
            value={value[index]?.displayName}
            onChange={(e) => handleChange(index, "displayName", e.target.value)}
            placeholder="Display name"
            size="small"
          />
        ),
    },
    {
      title: "",
      width: "10%",
      render: (_: unknown, __: ScopeItem, index: number) =>
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
            Add Scope
          </Button>
        </Space>
      )}
    </div>
  );
};

export { ScopeTable };
