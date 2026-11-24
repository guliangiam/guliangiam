/**
 * Organization 详情页辅助工具
 *
 * 从 OrganizationDetailPage.tsx 拆分而来，职责：
 *   - 数据序列化/反序列化（JSON 安全解析、嵌套抽取）
 *   - 类型归一化（string[] 清理、灵活 boolean 转换）
 */
import i18n from "@/shared/i18n";

/** Pretty-print any value as JSON string. Undefined/null → empty object `{}`. */
export function formatJson(value: unknown) {
  return JSON.stringify(value ?? {}, null, 2);
}

/**
 * Parse a JSON object string.
 *
 * Returns `{}` for empty input; throws for non-object values (arrays, primitives, null).
 * `Record<string, unknown>` is intentional — JSON values are heterogeneous by nature.
 */
export function parseJsonObject(label: string, value?: string) {
  const raw = value?.trim() ?? "";
  if (raw === "") return {} as Record<string, unknown>;
  const parsed = JSON.parse(raw) as unknown;
  if (parsed === null || Array.isArray(parsed) || typeof parsed !== "object") {
    throw new Error(i18n.t("resource:organization.mustBeJsonObject", { label }));
  }
  return parsed as Record<string, unknown>;
}

/** Filter an unknown value down to a clean string array. */
export function getStringArray(value: unknown) {
  return Array.isArray(value)
    ? value.filter((item): item is string => typeof item === "string" && item.trim() !== "")
    : [];
}

/** Coerce a flexible boolean source (boolean / number / string) into boolean | undefined. */
export function getFlexibleBoolean(value: unknown): boolean | undefined {
  switch (typeof value) {
    case "boolean":
      return value;
    case "number":
      return value !== 0;
    case "string":
      return value.trim().toLowerCase() === "true";
    default:
      return undefined;
  }
}

/** Extract a nested JSON object by key from a heterogeneous payload. */
export function getNestedObject(value: unknown, key: string) {
  if (!value || typeof value !== "object" || Array.isArray(value))
    return {} as Record<string, unknown>;
  const nested = (value as Record<string, unknown>)[key];
  return nested && typeof nested === "object" && !Array.isArray(nested)
    ? (nested as Record<string, unknown>)
    : {};
}
