/**
 * 结构化 JSON 编辑器的纯函数层（便于单测，不依赖 React）。
 */
export type StructuredJsonKind = "string" | "number" | "boolean" | "json";

export type StructuredJsonRow = {
  id: number;
  key: string;
  kind: StructuredJsonKind;
  value: string | number | boolean;
};

let rowSequence = 0;
export function nextRowId(): number {
  rowSequence += 1;
  return rowSequence;
}

export function toRow(key: string, value: unknown): StructuredJsonRow {
  if (typeof value === "string") {
    return { id: nextRowId(), key, kind: "string", value };
  }
  if (typeof value === "number") {
    return { id: nextRowId(), key, kind: "number", value };
  }
  if (typeof value === "boolean") {
    return { id: nextRowId(), key, kind: "boolean", value };
  }
  return { id: nextRowId(), key, kind: "json", value: JSON.stringify(value, null, 2) || "null" };
}

/** 解析顶层对象；空文本视为空对象；数组/标量/非法 JSON 返回 null（只能走高级模式）。 */
export function parseStructuredJson(value?: string): StructuredJsonRow[] | null {
  if (!value || !value.trim()) return [];
  try {
    const parsed: unknown = JSON.parse(value);
    if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) return null;
    return Object.entries(parsed as Record<string, unknown>).map(([key, item]) => toRow(key, item));
  } catch {
    return null;
  }
}

/** 序列化回 JSON 文本；键为空/重复或嵌套值非法时返回 null，避免写到一半的数据。 */
export function serializeStructuredJson(rows: StructuredJsonRow[]): string | null {
  const object: Record<string, unknown> = {};
  for (const row of rows) {
    const key = row.key.trim();
    if (!key || Object.prototype.hasOwnProperty.call(object, key)) return null;
    if (row.kind === "json") {
      try {
        object[key] = JSON.parse(String(row.value));
      } catch {
        return null;
      }
    } else if (row.kind === "number") {
      if (typeof row.value !== "number" || Number.isNaN(row.value)) return null;
      object[key] = row.value;
    } else {
      object[key] = row.value;
    }
  }
  return JSON.stringify(object, null, 2);
}

/**
 * 宽松序列化：空键/重复键/非法嵌套值无法走严格序列化，但在切到高级
 * JSON 模式时必须把用户当前看到的行原样带过去，避免草稿被静默丢弃。
 * 产出的文本可能语义不合法（空键、重复键），由高级模式的语法校验继续提示。
 */
export function serializeStructuredJsonDraft(rows: StructuredJsonRow[]): string {
  if (rows.length === 0) return "{}";
  const lines = rows.map((row) => {
    const key = JSON.stringify(row.key.trim());
    let rendered: string;
    if (row.kind === "json") {
      rendered = String(row.value);
    } else if (row.kind === "number") {
      rendered =
        typeof row.value === "number" && Number.isFinite(row.value) ? String(row.value) : "null";
    } else if (row.kind === "boolean") {
      rendered = row.value === true ? "true" : "false";
    } else {
      rendered = JSON.stringify(String(row.value));
    }
    return `  ${key}: ${rendered}`;
  });
  return `{\n${lines.join(",\n")}\n}`;
}

export function isValidJson(value: string): boolean {
  if (!value.trim()) return false;
  try {
    JSON.parse(value);
    return true;
  } catch {
    return false;
  }
}
