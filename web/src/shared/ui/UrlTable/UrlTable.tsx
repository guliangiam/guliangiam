import { Button, Input, Space, Table } from "antd";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import { type FC, useCallback } from "react";

type Props = {
  value?: string[];
  onChange?: (urls: string[]) => void;
  placeholder?: string;
  readOnly?: boolean;
};

const UrlTable: FC<Props> = ({ value = [], onChange, placeholder, readOnly }) => {
  const handleAdd = useCallback(() => {
    onChange?.([...value, ""]);
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
    (index: number, val: string) => {
      const next = [...value];
      next[index] = val;
      onChange?.(next);
    },
    [value, onChange],
  );

  const columns = [
    {
      title: "URL",
      dataIndex: "url",
      width: "90%",
      render: (_: string, __: Record<string, unknown>, index: number) =>
        readOnly ? (
          value[index] || "-"
        ) : (
          <Input
            value={value[index]}
            onChange={(e) => handleChange(index, e.target.value)}
            placeholder={placeholder || "https://example.com"}
            size="small"
          />
        ),
    },
    {
      title: "",
      width: "10%",
      render: (_: unknown, __: Record<string, unknown>, index: number) =>
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
        dataSource={value.map((url, i) => ({ url, key: i }))}
        pagination={false}
        size="small"
        bordered
      />
      {!readOnly && (
        <Space style={{ marginTop: 8 }}>
          <Button type="dashed" onClick={handleAdd} icon={<PlusOutlined />} size="small">
            Add URL
          </Button>
        </Space>
      )}
    </div>
  );
};

export { UrlTable };
