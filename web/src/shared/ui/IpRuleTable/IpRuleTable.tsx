import { Button, Input, Select, Space, Table } from "antd";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import { type FC, useCallback } from "react";

export type IpRuleItem = {
  ip: string;
  description: string;
  action: string;
};

const IP_ACTION_OPTIONS = [
  { label: "Allow", value: "Allow" },
  { label: "Deny", value: "Deny" },
];

type Props = {
  value?: IpRuleItem[];
  onChange?: (value: IpRuleItem[]) => void;
  readOnly?: boolean;
};

const IpRuleTable: FC<Props> = ({ value = [], onChange, readOnly }) => {
  const handleAdd = useCallback(() => {
    onChange?.([...value, { ip: "", description: "", action: "Allow" }]);
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
    (index: number, field: keyof IpRuleItem, val: string) => {
      const next = [...value];
      next[index] = { ...next[index], [field]: val };
      onChange?.(next);
    },
    [value, onChange],
  );

  const columns = [
    {
      title: "IP/CIDR",
      dataIndex: "ip",
      width: "30%",
      render: (_: string, __: IpRuleItem, index: number) =>
        readOnly ? (
          value[index]?.ip || "-"
        ) : (
          <Input
            value={value[index]?.ip}
            onChange={(e) => handleChange(index, "ip", e.target.value)}
            placeholder="192.168.1.0/24"
            size="small"
          />
        ),
    },
    {
      title: "Description",
      dataIndex: "description",
      width: "35%",
      render: (_: string, __: IpRuleItem, index: number) =>
        readOnly ? (
          value[index]?.description || "-"
        ) : (
          <Input
            value={value[index]?.description}
            onChange={(e) => handleChange(index, "description", e.target.value)}
            placeholder="Description"
            size="small"
          />
        ),
    },
    {
      title: "Action",
      dataIndex: "action",
      width: "20%",
      render: (_: string, __: IpRuleItem, index: number) =>
        readOnly ? (
          value[index]?.action || "-"
        ) : (
          <Select
            value={value[index]?.action || "Allow"}
            onChange={(v) => handleChange(index, "action", v)}
            options={IP_ACTION_OPTIONS}
            size="small"
            style={{ width: "100%" }}
          />
        ),
    },
    {
      title: "",
      width: "15%",
      render: (_: unknown, __: IpRuleItem, index: number) =>
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
            Add Rule
          </Button>
        </Space>
      )}
    </div>
  );
};

export { IpRuleTable };
