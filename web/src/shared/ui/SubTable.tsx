import { useState } from "react";
import { Button, Card, Empty, Input, Popconfirm, Select, Space, Table, Typography } from "antd";
import { DeleteOutlined, EditOutlined, PlusOutlined, SaveOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { useTranslation } from "react-i18next";

type EditOption = {
  label: string;
  value: string;
};

type EditableColumnDef = {
  title: string;
  dataIndex: string;
  editable?: boolean;
  editType?: "input" | "select";
  editOptions?: EditOption[];
  width?: number | string;
  render?: (value: unknown, record: Record<string, unknown>, index: number) => React.ReactNode;
};

type SubTableProps = {
  title: string;
  columns: EditableColumnDef[];
  dataSource: Record<string, unknown>[];
  onChange: (data: Record<string, unknown>[]) => void;
  addable?: boolean;
  deletable?: boolean;
  editable?: boolean;
  rowKey?: string;
  addDefaults?: Record<string, unknown>;
};

export function SubTable({
  title,
  columns,
  dataSource,
  onChange,
  addable = true,
  deletable = true,
  editable = true,
  rowKey = "id",
  addDefaults = {},
}: SubTableProps) {
  const { t } = useTranslation("common");
  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [editingRecord, setEditingRecord] = useState<Record<string, unknown> | null>(null);

  const handleAdd = () => {
    const newRecord: Record<string, unknown> = {
      ...addDefaults,
      [rowKey]: `new_${Date.now()}`,
    };
    onChange([...dataSource, newRecord]);
    if (editable) {
      setEditingKey(newRecord[rowKey] as string);
      setEditingRecord({ ...newRecord });
    }
  };

  const handleDelete = (key: string) => {
    onChange(dataSource.filter((item) => String(item[rowKey]) !== key));
  };

  const handleEdit = (record: Record<string, unknown>) => {
    setEditingKey(String(record[rowKey]));
    setEditingRecord({ ...record });
  };

  const handleSave = () => {
    if (!editingRecord) return;
    const updated = dataSource.map((item) =>
      String(item[rowKey]) === editingKey ? { ...editingRecord } : item,
    );
    onChange(updated);
    setEditingKey(null);
    setEditingRecord(null);
  };

  const handleCancel = () => {
    setEditingKey(null);
    setEditingRecord(null);
  };

  const handleFieldChange = (dataIndex: string, value: unknown) => {
    if (!editingRecord) return;
    setEditingRecord({ ...editingRecord, [dataIndex]: value });
  };

  const tableColumns: ColumnsType<Record<string, unknown>> = [
    ...columns.map((col) => ({
      title: col.title,
      dataIndex: col.dataIndex,
      key: col.dataIndex,
      width: col.width,
      render: (value: unknown, record: Record<string, unknown>, index: number) => {
        const isEditing = editingKey === String(record[rowKey]);
        if (isEditing && col.editable !== false) {
          if (col.editType === "select" && col.editOptions) {
            return (
              <Select
                size="small"
                value={String(editingRecord?.[col.dataIndex] ?? value ?? "")}
                onChange={(val) => handleFieldChange(col.dataIndex, val)}
                options={col.editOptions}
                style={{ width: "100%" }}
              />
            );
          }
          return (
            <Input
              size="small"
              value={String(editingRecord?.[col.dataIndex] ?? value ?? "")}
              onChange={(e) => handleFieldChange(col.dataIndex, e.target.value)}
            />
          );
        }
        if (col.render) return col.render(value, record, index);
        return value as React.ReactNode;
      },
    })),
    ...(editable || deletable
      ? [
          {
            title: t("actions"),
            key: "__actions__",
            width: 120,
            render: (_: unknown, record: Record<string, unknown>) => {
              const isEditing = editingKey === String(record[rowKey]);
              return (
                <Space size="small">
                  {isEditing ? (
                    <>
                      <Button size="small" type="link" icon={<SaveOutlined />} onClick={handleSave}>
                        {t("save")}
                      </Button>
                      <Button size="small" type="link" onClick={handleCancel}>
                        {t("cancel")}
                      </Button>
                    </>
                  ) : (
                    <>
                      {editable && (
                        <Button
                          size="small"
                          type="link"
                          icon={<EditOutlined />}
                          onClick={() => handleEdit(record)}
                        />
                      )}
                      {deletable && (
                        <Popconfirm
                          title={t("deleteConfirm")}
                          onConfirm={() => handleDelete(String(record[rowKey]))}
                        >
                          <Button size="small" type="link" danger icon={<DeleteOutlined />} />
                        </Popconfirm>
                      )}
                    </>
                  )}
                </Space>
              );
            },
          },
        ]
      : []),
  ];

  return (
    <Card
      title={<Typography.Text strong>{title}</Typography.Text>}
      size="small"
      variant="borderless"
      extra={
        addable && (
          <Button size="small" type="dashed" icon={<PlusOutlined />} onClick={handleAdd}>
            {t("form.addRow")}
          </Button>
        )
      }
      style={{ marginBottom: "var(--gl-space-4)" }}
    >
      {dataSource.length === 0 ? (
        <Empty description={t("noData")} image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : (
        <Table
          dataSource={dataSource}
          columns={tableColumns}
          rowKey={rowKey}
          pagination={false}
          size="small"
          scroll={{ x: 600 }}
        />
      )}
    </Card>
  );
}
