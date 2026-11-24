export type TokenField = {
  key: string;
  type: "color" | "number" | "switch";
  defaultValue: string | number | boolean;
  min?: number;
  max?: number;
};

export const THEME_TOKEN_FIELDS: TokenField[] = [
  { key: "colorPrimary", type: "color", defaultValue: "#0b8073" },
  { key: "colorInfo", type: "color", defaultValue: "#0b8073" },
  { key: "colorSuccess", type: "color", defaultValue: "#52c41a" },
  { key: "colorWarning", type: "color", defaultValue: "#faad14" },
  { key: "colorError", type: "color", defaultValue: "#ff4d4f" },
  { key: "colorBgBase", type: "color", defaultValue: "#ffffff" },
  { key: "colorTextBase", type: "color", defaultValue: "#000000" },
  { key: "colorBgContainer", type: "color", defaultValue: "#ffffff" },
  { key: "colorBorder", type: "color", defaultValue: "#d9d9d9" },
  { key: "borderRadius", type: "number", defaultValue: 8, min: 0, max: 16 },
  { key: "fontSize", type: "number", defaultValue: 14, min: 12, max: 20 },
  { key: "wireframe", type: "switch", defaultValue: false },
];

/** 默认 tokens JSON（「恢复默认」与新建表单共用）。 */
export function defaultThemeTokensJson(): string {
  const tokens = Object.fromEntries(
    THEME_TOKEN_FIELDS.map((field) => [field.key, field.defaultValue]),
  );
  return JSON.stringify(tokens, null, 2);
}
