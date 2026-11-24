import { Button, Input, Space, Switch, Table } from "antd";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import { type FC, useCallback } from "react";

export type SigninMethodItem = {
  name: string;
  displayName: string;
  enabled: boolean;
};

type Props = {
  value?: SigninMethodItem[];
  onChange?: (value: SigninMethodItem[]) => void;
  readOnly?: boolean;
};

const SigninMethodTable: FC<Props> = ({ value = [], onChange, readOnly }) => {
  const handleAdd = useCallback(() => {
    onChange?.([...value, { name: "", displayName: "", enabled: true }]);
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
    (index: number, field: keyof SigninMethodItem, val: string | boolean) => {
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
      width: "30%",
      render: (_: string, __: SigninMethodItem, index: number) =>
        readOnly ? (
          value[index]?.name || "-"
        ) : (
          <Input
            value={value[index]?.name}
            onChange={(e) => handleChange(index, "name", e.target.value)}
            placeholder="Method name"
            size="small"
          />
        ),
    },
    {
      title: "Display Name",
      dataIndex: "displayName",
      width: "35%",
      render: (_: string, __: SigninMethodItem, index: number) =>
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
      title: "Enabled",
      dataIndex: "enabled",
      width: "15%",
      render: (_: unknown, __: SigninMethodItem, index: number) =>
        readOnly ? (
          value[index]?.enabled ? (
            "Yes"
          ) : (
            "No"
          )
        ) : (
          <Switch
            checked={value[index]?.enabled}
            onChange={(checked) => handleChange(index, "enabled", checked)}
            size="small"
          />
        ),
    },
    {
      title: "",
      width: "20%",
      render: (_: unknown, __: SigninMethodItem, index: number) =>
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
            Add Method
          </Button>
        </Space>
      )}
    </div>
  );
};

export { SigninMethodTable };
