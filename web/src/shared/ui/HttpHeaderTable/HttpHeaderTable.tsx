import { Button, Input, Space, Table } from "antd";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import { type FC, useCallback } from "react";
import { useTranslation } from "react-i18next";

export type HttpHeaderItem = {
  key: string;
  value: string;
};

type Props = {
  value?: HttpHeaderItem[];
  onChange?: (value: HttpHeaderItem[]) => void;
  readOnly?: boolean;
};

const HttpHeaderTable: FC<Props> = ({ value = [], onChange, readOnly }) => {
  const { t } = useTranslation("common");

  const handleAdd = useCallback(() => {
    onChange?.([...value, { key: "", value: "" }]);
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
    (index: number, field: keyof HttpHeaderItem, val: string) => {
      const next = [...value];
      next[index] = { ...next[index], [field]: val };
      onChange?.(next);
    },
    [value, onChange],
  );

  const columns = [
    {
      title: t("key"),
      dataIndex: "headerKey",
      width: "45%",
      render: (_: string, __: HttpHeaderItem, index: number) =>
        readOnly ? (
          value[index]?.key || "-"
        ) : (
          <Input
            value={value[index]?.key}
            onChange={(e) => handleChange(index, "key", e.target.value)}
            placeholder={t("key")}
            size="small"
          />
        ),
    },
    {
      title: t("value"),
      dataIndex: "headerValue",
      width: "45%",
      render: (_: string, __: HttpHeaderItem, index: number) =>
        readOnly ? (
          value[index]?.value || "-"
        ) : (
          <Input
            value={value[index]?.value}
            onChange={(e) => handleChange(index, "value", e.target.value)}
            placeholder={t("value")}
            size="small"
          />
        ),
    },
    {
      title: "",
      width: "10%",
      render: (_: unknown, __: HttpHeaderItem, index: number) =>
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
        dataSource={value.map((item, i) => ({ ...item, _key: i }))}
        rowKey="_key"
        pagination={false}
        size="small"
        bordered
      />
      {!readOnly && (
        <Space style={{ marginTop: 8 }}>
          <Button type="dashed" onClick={handleAdd} icon={<PlusOutlined />} size="small">
            {t("addHeader")}
          </Button>
        </Space>
      )}
    </div>
  );
};

export { HttpHeaderTable };
