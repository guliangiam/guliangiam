import { Button, Input, Space, Table } from "antd";
import { PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import { type FC, useCallback } from "react";
import type { ManagedAccountItem } from "@/shared/api/iam/user";

type Props = {
  value: ManagedAccountItem[];
  onChange: (value: ManagedAccountItem[]) => void;
  readOnly?: boolean;
};

const ManagedAccountTable: FC<Props> = ({ value, onChange, readOnly }) => {
  const handleAdd = useCallback(() => {
    onChange([
      ...value,
      {
        application: "",
        username: "",
        password: "",
        passwordSet: false,
        passwordMasked: "",
        signinUrl: "",
      },
    ]);
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
    (index: number, field: keyof ManagedAccountItem, val: string) => {
      const next = [...value];
      next[index] = { ...next[index], [field]: val };
      onChange(next);
    },
    [value, onChange],
  );

  const columns = [
    {
      title: "Application",
      dataIndex: "application",
      render: (_: string, __: ManagedAccountItem, index: number) =>
        readOnly ? (
          value[index]?.application
        ) : (
          <Input
            value={value[index]?.application}
            onChange={(e) => handleChange(index, "application", e.target.value)}
            placeholder="Application"
            size="small"
          />
        ),
    },
    {
      title: "Username",
      dataIndex: "username",
      render: (_: string, __: ManagedAccountItem, index: number) =>
        readOnly ? (
          value[index]?.username
        ) : (
          <Input
            value={value[index]?.username}
            onChange={(e) => handleChange(index, "username", e.target.value)}
            placeholder="Username"
            size="small"
          />
        ),
    },
    {
      title: "Password",
      dataIndex: "password",
      render: (_: string, __: ManagedAccountItem, index: number) =>
        readOnly ? (
          value[index]?.passwordMasked || (value[index]?.passwordSet ? "••••••••" : "-")
        ) : (
          <Space orientation="vertical" size={0} style={{ width: "100%" }}>
            <Input.Password
              value={value[index]?.password}
              onChange={(e) => handleChange(index, "password", e.target.value)}
              placeholder="Password"
              size="small"
            />
            {/* 已存密码只以掩码回显：输入框留空表示保留原值。 */}
            {value[index]?.passwordSet && !value[index]?.password && (
              <span className="ant-typography ant-typography-secondary" style={{ fontSize: 12 }}>
                {value[index]?.passwordMasked || "••••••••"}
              </span>
            )}
          </Space>
        ),
    },
    {
      title: "Signin URL",
      dataIndex: "signinUrl",
      render: (_: string, __: ManagedAccountItem, index: number) =>
        readOnly ? (
          value[index]?.signinUrl
        ) : (
          <Input
            value={value[index]?.signinUrl}
            onChange={(e) => handleChange(index, "signinUrl", e.target.value)}
            placeholder="Signin URL"
            size="small"
          />
        ),
    },
    ...(readOnly
      ? []
      : [
          {
            title: "",
            width: 48,
            render: (_: unknown, __: ManagedAccountItem, index: number) => (
              <Button
                type="text"
                danger
                size="small"
                icon={<DeleteOutlined />}
                onClick={() => handleRemove(index)}
              />
            ),
          },
        ]),
  ];

  return (
    <Space orientation="vertical" style={{ width: "100%" }}>
      <Table
        dataSource={value.map((item, i) => ({ ...item, _idx: i }))}
        columns={columns}
        pagination={false}
        size="small"
        rowKey="_idx"
      />
      {!readOnly && (
        <Button type="dashed" onClick={handleAdd} icon={<PlusOutlined />} size="small">
          Add Account
        </Button>
      )}
    </Space>
  );
};

export default ManagedAccountTable;
