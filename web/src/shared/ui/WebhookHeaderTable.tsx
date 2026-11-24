import { Button, Input, Space, Table } from "antd";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import { type FC, useCallback } from "react";

export type HeaderItem = {
  name: string;
  value: string;
};

type Props = {
  value: HeaderItem[];
  onChange: (value: HeaderItem[]) => void;
  readOnly?: boolean;
};

const WebhookHeaderTable: FC<Props> = ({ value, onChange, readOnly }) => {
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
    (index: number, field: keyof HeaderItem, val: string) => {
      const next = [...value];
      next[index] = { ...next[index], [field]: val };
      onChange(next);
    },
    [value, onChange],
  );

  const columns = [
    {
      title: "Name",
      dataIndex: "name",
      width: "40%",
      render: (_: string, __: HeaderItem, index: number) =>
        readOnly ? (
          value[index]?.name || "-"
        ) : (
          <Input
            value={value[index]?.name}
            onChange={(e) => handleChange(index, "name", e.target.value)}
            placeholder="Header name"
            size="small"
          />
        ),
    },
    {
      title: "Value",
      dataIndex: "value",
      width: "50%",
      render: (_: string, __: HeaderItem, index: number) =>
        readOnly ? (
          value[index]?.value || "-"
        ) : (
          <Input
            value={value[index]?.value}
            onChange={(e) => handleChange(index, "value", e.target.value)}
            placeholder="Header value"
            size="small"
          />
        ),
    },
    {
      title: "",
      width: "10%",
      render: (_: unknown, __: HeaderItem, index: number) =>
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
            Add Header
          </Button>
        </Space>
      )}
    </div>
  );
};

export default WebhookHeaderTable;
