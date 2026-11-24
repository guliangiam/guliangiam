import { useState, useCallback } from "react";
import { Col, Input, Row, Typography, Button, Space } from "antd";
import { PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";

type UserMappingFieldsProps = {
  value?: string;
  onChange?: (value: string) => void;
};

const COMMON_FIELDS = [
  { key: "id", label: "ID" },
  { key: "username", label: "Username" },
  { key: "displayName", label: "Display Name" },
  { key: "email", label: "Email" },
  { key: "phone", label: "Phone" },
  { key: "avatar", label: "Avatar" },
  { key: "firstName", label: "First Name" },
  { key: "lastName", label: "Last Name" },
  { key: "address", label: "Address" },
];

type ParsedMappings = {
  common: Record<string, string>;
  custom: { key: string; value: string }[];
};

// parseUserMappings is pure so it can run during render.
function parseUserMappings(value: string | undefined): ParsedMappings {
  if (!value) {
    return { common: {}, custom: [] };
  }
  try {
    const parsed = JSON.parse(value) as Record<string, string>;
    const commonKeys = new Set(COMMON_FIELDS.map((f) => f.key));
    const common: Record<string, string> = {};
    const custom: { key: string; value: string }[] = [];
    for (const [k, v] of Object.entries(parsed)) {
      if (commonKeys.has(k)) {
        common[k] = v;
      } else {
        custom.push({ key: k, value: v });
      }
    }
    return { common, custom };
  } catch {
    return { common: {}, custom: [] };
  }
}

export function UserMappingFields({ value, onChange }: UserMappingFieldsProps) {
  const { t } = useTranslation();
  const [mappings, setMappings] = useState<Record<string, string>>({});
  const [customFields, setCustomFields] = useState<{ key: string; value: string }[]>([]);
  const [syncedValue, setSyncedValue] = useState(value);

  // Adjusting state during render is React's documented alternative to syncing
  // a prop into state from an effect: the incoming JSON string is re-parsed as
  // soon as it changes, without starting an extra render pass.
  if (value !== syncedValue) {
    setSyncedValue(value);
    const parsed = parseUserMappings(value);
    setMappings(parsed.common);
    setCustomFields(parsed.custom);
  }

  const emitChange = useCallback(
    (common: Record<string, string>, custom: { key: string; value: string }[]) => {
      const merged: Record<string, string> = {};
      for (const [k, v] of Object.entries(common)) {
        if (v) merged[k] = v;
      }
      for (const { key, value: val } of custom) {
        if (key && val) merged[key] = val;
      }
      onChange?.(Object.keys(merged).length > 0 ? JSON.stringify(merged, null, 2) : "");
    },
    [onChange],
  );

  const handleCommonChange = (fieldKey: string, providerAttr: string) => {
    const updated = { ...mappings, [fieldKey]: providerAttr };
    setMappings(updated);
    emitChange(updated, customFields);
  };

  const handleCustomKeyChange = (index: number, newKey: string) => {
    const updated = [...customFields];
    updated[index] = { ...updated[index], key: newKey };
    setCustomFields(updated);
    emitChange(mappings, updated);
  };

  const handleCustomValueChange = (index: number, newValue: string) => {
    const updated = [...customFields];
    updated[index] = { ...updated[index], value: newValue };
    setCustomFields(updated);
    emitChange(mappings, updated);
  };

  const addCustomField = () => {
    const updated = [...customFields, { key: "", value: "" }];
    setCustomFields(updated);
  };

  const removeCustomField = (index: number) => {
    const updated = customFields.filter((_, i) => i !== index);
    setCustomFields(updated);
    emitChange(mappings, updated);
  };

  return (
    <div>
      <Typography.Text type="secondary" style={{ display: "block", marginBottom: 8 }}>
        {t(
          "provider.userMappingDesc",
          "Map provider attributes to local user fields. Enter the provider attribute name for each field.",
        )}
      </Typography.Text>

      <Row gutter={[8, 8]}>
        {COMMON_FIELDS.map((field) => (
          <Col span={12} key={field.key}>
            <Space.Compact style={{ width: "100%" }}>
              <Input
                style={{ width: 140, flex: "none" }}
                value={field.label}
                disabled
                size="small"
              />
              <Input
                placeholder="e.g. sub"
                value={mappings[field.key] ?? ""}
                onChange={(e) => handleCommonChange(field.key, e.target.value)}
                size="small"
              />
            </Space.Compact>
          </Col>
        ))}
      </Row>

      {customFields.length > 0 && (
        <div style={{ marginTop: 8 }}>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {t("provider.customMappings", "Custom Mappings")}
          </Typography.Text>
          {customFields.map((cf, i) => (
            <Row key={i} gutter={8} style={{ marginTop: 4 }}>
              <Col span={6}>
                <Input
                  placeholder={t("provider.localField", "Local field")}
                  value={cf.key}
                  onChange={(e) => handleCustomKeyChange(i, e.target.value)}
                  size="small"
                />
              </Col>
              <Col span={16}>
                <Input
                  placeholder={t("provider.providerAttr", "Provider attribute")}
                  value={cf.value}
                  onChange={(e) => handleCustomValueChange(i, e.target.value)}
                  size="small"
                />
              </Col>
              <Col span={2}>
                <Button
                  type="text"
                  danger
                  size="small"
                  icon={<DeleteOutlined />}
                  onClick={() => removeCustomField(i)}
                />
              </Col>
            </Row>
          ))}
        </div>
      )}

      <Button
        type="dashed"
        size="small"
        icon={<PlusOutlined />}
        onClick={addCustomField}
        style={{ marginTop: 8 }}
      >
        {t("provider.addCustomMapping", "Add Custom Mapping")}
      </Button>
    </div>
  );
}
