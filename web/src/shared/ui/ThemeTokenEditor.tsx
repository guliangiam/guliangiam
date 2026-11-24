import { useMemo, memo } from "react";
import { Button, ColorPicker, Form, InputNumber, Switch } from "antd";
import type { Color } from "antd/es/color-picker";
import { ReloadOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";

import { THEME_TOKEN_FIELDS, defaultThemeTokensJson } from "./ThemeTokenEditor.constants";

type ThemeTokenEditorProps = {
  value?: string;
  onChange?: (value: string) => void;
};

type ParsedTokens = { tokens: Record<string, unknown>; parseError: boolean };

// parseThemeTokens is pure: the token map is derived from the JSON string during
// render instead of being mirrored into state inside an effect.
function parseThemeTokens(value: string | undefined): ParsedTokens {
  if (!value || value.trim() === "") {
    return { tokens: {}, parseError: false };
  }
  try {
    const parsed = JSON.parse(value) as unknown;
    if (typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)) {
      return { tokens: parsed as Record<string, unknown>, parseError: false };
    }
    return { tokens: {}, parseError: true };
  } catch {
    return { tokens: {}, parseError: true };
  }
}

export const ThemeTokenEditor = memo(function ThemeTokenEditor({
  value,
  onChange,
}: ThemeTokenEditorProps) {
  const { t: tCommon } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const { tokens, parseError } = useMemo(() => parseThemeTokens(value), [value]);

  const updateToken = (key: string, val: unknown) => {
    const updated = { ...tokens, [key]: val };
    onChange?.(JSON.stringify(updated, null, 2));
  };

  const handleColorChange = (key: string, color: Color) => {
    updateToken(key, color.toHexString());
  };

  if (parseError) {
    return (
      <div style={{ padding: 8 }}>
        <p style={{ color: "var(--ant-color-text-secondary)", margin: 0 }}>
          {tCommon("theme.tokenParseError")}
        </p>
      </div>
    );
  }

  return (
    <div>
      <div style={{ display: "flex", justifyContent: "flex-end", marginBottom: 8 }}>
        <Button
          size="small"
          icon={<ReloadOutlined />}
          onClick={() => onChange?.(defaultThemeTokensJson())}
        >
          {tr("organization.themeTokensReset")}
        </Button>
      </div>
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))",
          gap: "8px 16px",
        }}
      >
        {THEME_TOKEN_FIELDS.map((field) => {
          const currentValue = tokens[field.key];
          const label = tr(`organization.themeTokens.${field.key}`);
          const help =
            field.type === "number"
              ? tr("organization.themeTokensHelp.number", {
                  min: field.min,
                  max: field.max,
                  defaultValue: field.defaultValue,
                })
              : field.type === "color"
                ? tr("organization.themeTokensHelp.color", { defaultValue: field.defaultValue })
                : tr("organization.themeTokensHelp.switch", { defaultValue: field.defaultValue });

          if (field.type === "color") {
            return (
              <Form.Item key={field.key} label={label} extra={help} style={{ marginBottom: 8 }}>
                <ColorPicker
                  value={
                    typeof currentValue === "string"
                      ? currentValue
                      : typeof field.defaultValue === "string"
                        ? field.defaultValue
                        : undefined
                  }
                  onChangeComplete={(color) => handleColorChange(field.key, color)}
                  showText
                />
              </Form.Item>
            );
          }

          if (field.type === "number") {
            const numeric = typeof currentValue === "number" ? currentValue : undefined;
            const invalid =
              numeric !== undefined &&
              ((field.min !== undefined && numeric < field.min) ||
                (field.max !== undefined && numeric > field.max));
            return (
              <Form.Item
                key={field.key}
                label={label}
                extra={help}
                validateStatus={invalid ? "error" : undefined}
                help={invalid ? tr("organization.themeTokensHelp.outOfRange") : undefined}
                style={{ marginBottom: 8 }}
              >
                <InputNumber
                  value={numeric}
                  min={field.min}
                  max={field.max}
                  placeholder={String(field.defaultValue)}
                  style={{ width: "100%" }}
                  onChange={(v) => updateToken(field.key, v)}
                />
              </Form.Item>
            );
          }

          return (
            <Form.Item
              key={field.key}
              label={label}
              extra={help}
              valuePropName="checked"
              style={{ marginBottom: 8 }}
            >
              <Switch
                checked={currentValue === true || currentValue === "true"}
                onChange={(v) => updateToken(field.key, v)}
              />
            </Form.Item>
          );
        })}
      </div>
    </div>
  );
});
