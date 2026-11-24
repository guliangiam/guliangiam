import { Button, Input, Select, Space, Switch, Table } from "antd";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import { type FC, useCallback } from "react";

import { useTranslation } from "react-i18next";
export type SyncerColumnMapping = {
  sourceFieldName: string;
  type: string;
  isKey: boolean;
  columnName: string;
};

const SOURCE_FIELD_OPTIONS = [
  "Name",
  "DisplayName",
  "FirstName",
  "LastName",
  "Email",
  "EmailVerified",
  "Phone",
  "CountryCode",
  "Region",
  "Location",
  "Address",
  "Affiliation",
  "Title",
  "Homepage",
  "Bio",
  "Tag",
  "Language",
  "Gender",
  "Birthday",
  "Education",
  "Score",
  "Karma",
  "Ranking",
  "IsAdmin",
  "IsForbidden",
  "IsDeleted",
  "Avatar",
  "Type",
  "IdCardType",
  "IdCard",
  "RealName",
  "IsVerified",
  "SignupApplication",
  "IpWhitelist",
];

const TYPE_OPTIONS = [
  { label: "String", value: "string" },
  { label: "Integer", value: "int" },
  { label: "Boolean", value: "bool" },
  { label: "Datetime", value: "datetime" },
  { label: "Float", value: "float" },
];

type Props = {
  value?: SyncerColumnMapping[];
  onChange?: (value: SyncerColumnMapping[]) => void;
  readOnly?: boolean;
};

const SyncerColumnMappingEditor: FC<Props> = ({ value, onChange, readOnly }) => {
  const { t } = useTranslation("common");
  const v = value ?? [];
  const emit = onChange ?? (() => {});

  const handleAdd = useCallback(() => {
    emit([...v, { sourceFieldName: "", type: "string", isKey: false, columnName: "" }]);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [v, emit]);

  const handleRemove = useCallback(
    (index: number) => {
      const next = [...v];
      next.splice(index, 1);
      emit(next);
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [v, emit],
  );

  const handleChange = useCallback(
    (index: number, field: keyof SyncerColumnMapping, val: string | boolean) => {
      const next = [...v];
      next[index] = { ...next[index], [field]: val };
      emit(next);
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [v, emit],
  );

  const columns = [
    {
      title: "Column Name",
      dataIndex: "columnName",
      width: "30%",
      render: (_: string, __: SyncerColumnMapping, index: number) =>
        readOnly ? (
          v[index]?.columnName
        ) : (
          <Input
            value={v[index]?.columnName}
            onChange={(e) => handleChange(index, "columnName", e.target.value)}
            placeholder="source_column"
            size="small"
          />
        ),
    },
    {
      title: "Type",
      dataIndex: "type",
      width: "15%",
      render: (_: string, __: SyncerColumnMapping, index: number) =>
        readOnly ? (
          v[index]?.type
        ) : (
          <Select
            value={v[index]?.type}
            onChange={(val) => handleChange(index, "type", val)}
            options={TYPE_OPTIONS}
            size="small"
            style={{ width: "100%" }}
          />
        ),
    },
    {
      title: "Mapped Field",
      dataIndex: "sourceFieldName",
      width: "35%",
      render: (_: string, __: SyncerColumnMapping, index: number) =>
        readOnly ? (
          v[index]?.sourceFieldName
        ) : (
          <Select
            value={v[index]?.sourceFieldName}
            onChange={(val) => handleChange(index, "sourceFieldName", val)}
            options={SOURCE_FIELD_OPTIONS.map((f) => ({ label: f, value: f }))}
            showSearch
            allowClear
            placeholder={t("selectField")}
            size="small"
            style={{ width: "100%" }}
          />
        ),
    },
    {
      title: "Is Key",
      dataIndex: "isKey",
      width: "10%",
      render: (_: string, __: SyncerColumnMapping, index: number) =>
        readOnly ? (
          v[index]?.isKey ? (
            "Yes"
          ) : (
            "No"
          )
        ) : (
          <Switch
            checked={v[index]?.isKey}
            onChange={(checked) => handleChange(index, "isKey", checked)}
            size="small"
          />
        ),
    },
    {
      title: "",
      width: "10%",
      render: (_: unknown, __: SyncerColumnMapping, index: number) =>
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
        dataSource={v.map((item, i) => ({ ...item, key: i }))}
        pagination={false}
        size="small"
        bordered
      />
      {!readOnly && (
        <Space style={{ marginTop: 8 }}>
          <Button type="dashed" onClick={handleAdd} icon={<PlusOutlined />} size="small">
            Add Column
          </Button>
        </Space>
      )}
    </div>
  );
};

export { SyncerColumnMappingEditor };
