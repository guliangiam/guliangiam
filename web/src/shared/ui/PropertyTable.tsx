import { AutoComplete, Button, Input, Popconfirm, Space, Table, Typography } from "antd";
import { PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import { type FC, useCallback, useMemo } from "react";
import { useTranslation } from "react-i18next";
import type { CustomAttributeDefinition } from "@/shared/api/iam/user";

export type PropertyTableItem = {
  key: string;
  value: string;
  /** 接口原始 JSON 值；仅用于未编辑行原样回传，不参与展示。 */
  rawValue?: unknown;
};

type Props = {
  value: PropertyTableItem[];
  onChange: (value: PropertyTableItem[]) => void;
  readOnly?: boolean;
  /** Tenant-defined keys. They are suggestions only: free-form keys stay valid. */
  definitions?: CustomAttributeDefinition[];
};

const PropertyTable: FC<Props> = ({ value, onChange, readOnly, definitions }) => {
  const { t: tr } = useTranslation("resource");
  const keyOptions = useMemo(
    () =>
      (definitions ?? [])
        .filter((definition) => definition.enabled)
        .map((definition) => ({
          value: definition.key,
          label: definition.description
            ? `${definition.key} · ${definition.description}`
            : definition.key,
        })),
    [definitions],
  );
  const handleAdd = useCallback(() => {
    onChange([...value, { key: "", value: "" }]);
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
    (index: number, field: "key" | "value", val: string) => {
      const next = [...value];
      next[index] = { ...next[index], [field]: val };
      onChange(next);
    },
    [value, onChange],
  );

  const columns = [
    {
      title: tr("user.properties.key", "Key"),
      dataIndex: "key",
      render: (_: string, __: PropertyTableItem, index: number) =>
        readOnly ? (
          value[index]?.key
        ) : (
          <AutoComplete
            value={value[index]?.key}
            options={keyOptions}
            onChange={(val) => handleChange(index, "key", val)}
            placeholder={tr("user.properties.keyPlaceholder", "Property key")}
            size="small"
            style={{ width: "100%" }}
            filterOption={(input, option) =>
              String(option?.value ?? "")
                .toLowerCase()
                .includes(input.toLowerCase())
            }
          />
        ),
    },
    {
      title: tr("user.properties.value", "Value"),
      dataIndex: "value",
      render: (_: string, __: PropertyTableItem, index: number) =>
        readOnly ? (
          value[index]?.value
        ) : (
          <Input
            value={value[index]?.value}
            onChange={(e) => handleChange(index, "value", e.target.value)}
            placeholder={tr("user.properties.valuePlaceholder", "Property value")}
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
            render: (_: unknown, __: PropertyTableItem, index: number) => (
              <Popconfirm
                title={tr("user.properties.deleteConfirm", "Delete this property?")}
                onConfirm={() => handleRemove(index)}
              >
                <Button type="text" danger size="small" icon={<DeleteOutlined />} />
              </Popconfirm>
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
      {!readOnly && keyOptions.length > 0 && (
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {tr(
            "user.properties.definitionsHint",
            "{{count}} tenant definitions available as key suggestions",
            {
              count: keyOptions.length,
            },
          )}
        </Typography.Text>
      )}
      {!readOnly && (
        <Button type="dashed" onClick={handleAdd} icon={<PlusOutlined />} size="small">
          {tr("user.properties.add", "Add Property")}
        </Button>
      )}
    </Space>
  );
};

export default PropertyTable;
