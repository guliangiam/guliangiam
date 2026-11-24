import { Input, Select, InputNumber } from "antd";
import type { FilterDropdownProps } from "antd/es/table/interface";
import type { ReactNode } from "react";

export type ColumnType =
  | "text"
  | "string"
  | "email"
  | "uuid"
  | "number"
  | "integer"
  | "boolean"
  | "enum"
  | "date"
  | "datetime"
  | "url";

export type SearchableColumn = {
  dataIndex: string;
  label?: string;
  /**
   * Column data type — determines which search control is rendered in the
   * column dropdown. Falls back to Input for unknown/omitted types.
   *
   *   text/string/email/uuid/url → Input (子串搜索)
   *   number/integer             → InputNumber (范围 min/max)
   *   boolean                    → Select[true/false]
   *   enum                       → Select 多选（需传 enumOptions）
   *   date/datetime              → DatePicker.RangePicker
   */
  columnType?: ColumnType;
  /** 枚举选项（type="enum" 时使用） */
  enumOptions?: Array<{ label: string; value: string }>;
};

/**
 * Factory that returns a filterDropdown renderer for antd Table columns.
 * The returned function closes over the supplied params and produces the
 * UI (search control + search/reset buttons) for one searchable column.
 */
export function createColumnSearchDropdown(params: {
  searchable: SearchableColumn;
  isServerMode: boolean;
  t: (key: string) => string;
  onColumnSearch?: (field: string, value: string) => void;
  setSearch: (v: string) => void;
  setSearchedColumn: (col: string) => void;
}): (filterProps: FilterDropdownProps) => ReactNode {
  const { searchable, isServerMode, t, onColumnSearch, setSearch, setSearchedColumn } = params;

  const colType = searchable.columnType ?? "text";
  const isNumber = ["number", "integer"].includes(colType);
  const isBoolean = colType === "boolean";
  const isEnum = colType === "enum";

  return ({ setSelectedKeys, selectedKeys, confirm, clearFilters }: FilterDropdownProps) => {
    const placeholder = searchable.label || `Search ${searchable.dataIndex}`;
    const trigger = (value: string) => {
      if (isServerMode && onColumnSearch) {
        onColumnSearch(searchable.dataIndex, value);
      } else {
        setSearch(value);
        setSearchedColumn(searchable.dataIndex);
      }
      confirm();
    };
    const clearAll = () => {
      clearFilters?.();
      if (isServerMode && onColumnSearch) {
        onColumnSearch(searchable.dataIndex, "");
      } else {
        setSearch("");
        setSearchedColumn("");
      }
    };

    // 按类型渲染控件
    let control: ReactNode;
    if (isNumber) {
      control = (
        <div style={{ display: "flex", gap: 4, marginBottom: 8 }}>
          <InputNumber
            placeholder="min"
            size="small"
            style={{ width: "48%" }}
            onChange={(v) => setSelectedKeys(v ? [String(v)] : [])}
            onPressEnter={() => trigger((selectedKeys as string[])?.[0] ?? "")}
          />
          <InputNumber
            placeholder="max"
            size="small"
            style={{ width: "48%" }}
            onPressEnter={() => trigger((selectedKeys as string[])?.[0] ?? "")}
          />
        </div>
      );
    } else if (isBoolean) {
      control = (
        <Select
          placeholder={placeholder}
          aria-label={placeholder}
          allowClear
          size="small"
          style={{ width: "100%", marginBottom: 8 }}
          options={[
            { label: "True", value: "true" },
            { label: "False", value: "false" },
          ]}
          onChange={(v) => trigger(v ?? "")}
        />
      );
    } else if (isEnum && searchable.enumOptions) {
      control = (
        <Select
          mode="multiple"
          placeholder={placeholder}
          aria-label={placeholder}
          allowClear
          size="small"
          style={{ width: "100%", marginBottom: 8 }}
          options={searchable.enumOptions}
          onChange={(vs) => trigger((vs as string[]).join(","))}
        />
      );
    } else {
      // text fallback (default) — also handles `isText` since it falls through
      control = (
        <Input
          placeholder={placeholder}
          value={(selectedKeys as string[])?.[0]}
          onChange={(e) => setSelectedKeys(e.target.value ? [e.target.value] : [])}
          onPressEnter={() => trigger((selectedKeys as string[])?.[0] || "")}
          style={{ marginBottom: 8, display: "block" }}
        />
      );
    }

    return (
      <div style={{ padding: 8, minWidth: 200 }}>
        {control}
        <div style={{ display: "flex", gap: 8 }}>
          <button
            type="button"
            aria-label={t("search")}
            onClick={() => {
              trigger((selectedKeys as string[])?.[0] || "");
            }}
            style={{
              padding: "4px 12px",
              borderRadius: 6,
              background: "var(--gl-color-primary)",
              color: "#fff",
              border: "none",
              cursor: "pointer",
            }}
          >
            {t("search")}
          </button>
          <button
            type="button"
            aria-label={t("reset")}
            onClick={clearAll}
            style={{
              padding: "4px 12px",
              borderRadius: 6,
              background: "var(--gl-color-bg-elevated)",
              border: "1px solid var(--gl-color-border)",
              cursor: "pointer",
            }}
          >
            {t("reset")}
          </button>
        </div>
      </div>
    );
  };
}
