import { Button, Input, Select, Space, Table, Tag } from "antd";
import { PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import { type FC, useCallback } from "react";
import type { ConsentRecordItem } from "@/shared/api/iam/user";

type Props = {
  value: ConsentRecordItem[];
  onChange: (value: ConsentRecordItem[]) => void;
  readOnly?: boolean;
};

const ConsentTable: FC<Props> = ({ value, onChange, readOnly }) => {
  const handleAdd = useCallback(() => {
    onChange([...value, { application: "", scopes: [], grantedAt: "" }]);
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
    (index: number, field: keyof ConsentRecordItem, val: string | string[]) => {
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
      render: (_: string, __: ConsentRecordItem, index: number) =>
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
      title: "Scopes",
      dataIndex: "scopes",
      render: (_: string[], __: ConsentRecordItem, index: number) =>
        readOnly ? (
          (value[index]?.scopes ?? []).map((s) => <Tag key={s}>{s}</Tag>)
        ) : (
          <Select
            mode="tags"
            value={value[index]?.scopes ?? []}
            onChange={(val) => handleChange(index, "scopes", val)}
            placeholder="Scopes"
            size="small"
            style={{ width: "100%" }}
          />
        ),
    },
    {
      title: "Granted At",
      dataIndex: "grantedAt",
      render: (_: string, __: ConsentRecordItem, index: number) => value[index]?.grantedAt,
    },
    ...(readOnly
      ? []
      : [
          {
            title: "",
            width: 48,
            render: (_: unknown, __: ConsentRecordItem, index: number) => (
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
          Add Consent
        </Button>
      )}
    </Space>
  );
};

export default ConsentTable;
