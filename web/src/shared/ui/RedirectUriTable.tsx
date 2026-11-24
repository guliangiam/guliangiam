import { Button, Input, Space, Table } from "antd";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import { type FC, useCallback } from "react";

export type RedirectUri = {
  uri: string;
};

type Props = {
  value: RedirectUri[];
  onChange: (value: RedirectUri[]) => void;
  readOnly?: boolean;
};

const URL_REGEX = /^https?:\/\/.+/i;

const RedirectUriTable: FC<Props> = ({ value, onChange, readOnly }) => {
  const handleAdd = useCallback(() => {
    onChange([...value, { uri: "" }]);
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
    (index: number, val: string) => {
      const next = [...value];
      next[index] = { ...next[index], uri: val };
      onChange(next);
    },
    [value, onChange],
  );

  const columns = [
    {
      title: "Redirect URI",
      dataIndex: "uri",
      width: "85%",
      render: (_: string, __: RedirectUri, index: number) =>
        readOnly ? (
          <span
            style={{
              color: URL_REGEX.test(value[index]?.uri) ? undefined : "var(--gl-color-error)",
            }}
          >
            {value[index]?.uri || "-"}
          </span>
        ) : (
          <Input
            value={value[index]?.uri}
            onChange={(e) => handleChange(index, e.target.value)}
            placeholder="https://example.com/callback"
            size="small"
            status={value[index]?.uri && !URL_REGEX.test(value[index].uri) ? "error" : undefined}
          />
        ),
    },
    {
      title: "",
      width: "15%",
      render: (_: unknown, __: RedirectUri, index: number) =>
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
            Add URI
          </Button>
        </Space>
      )}
    </div>
  );
};

export default RedirectUriTable;
