/**
 * 组织配置里的 `*ExtraJson` / `*AssetsJson` 字段的结构化编辑器。
 *
 * 默认按「键 = 值」行编辑；值保留 JSON 原生类型（字符串/数字/布尔），
 * 嵌套对象与数组退化为逐行 JSON 校验输入。需要直接改整段文本时切换到
 * 高级 JSON 模式，行为与旧 `JsonEditor` 一致。
 *
 * 非法草稿不会写入表单字段，但会通过 `onInvalidChange` 上报给
 * `StructuredJsonField`，由后者的 `Form.Item` 校验阻断保存，并显示具体行号。
 */
import { Button, Checkbox, Form, Input, InputNumber, Switch, Tooltip } from "antd";
import DeleteOutlined from "@ant-design/icons/DeleteOutlined";
import ExclamationCircleOutlined from "@ant-design/icons/ExclamationCircleOutlined";
import { useCallback, useEffect, useRef, useState, type AriaAttributes } from "react";
import { useTranslation } from "react-i18next";
import { JsonEditor } from "@/shared/ui/JsonEditor";
import {
  isValidJson,
  nextRowId,
  parseStructuredJson,
  serializeStructuredJson,
  serializeStructuredJsonDraft,
  type StructuredJsonRow,
} from "./structured-json";

type StructuredJsonEditorProps = {
  value?: string;
  onChange?: (value: string) => void;
  /** 报告当前草稿是否非法；message 为 null 表示可提交。 */
  onInvalidChange?: (message: string | null, mode: "structured" | "advanced") => void;
  rows?: number;
  /** 字段分组名称，用于 `role="group"` 的读屏名称。 */
  label?: string;
  /** antd `Form.Item` 注入的控件 id，供其 `<label htmlFor>` 关联。 */
  id?: string;
  "aria-invalid"?: AriaAttributes["aria-invalid"];
  "aria-describedby"?: string;
};

type StructuredJsonFieldProps = {
  name: string;
  label?: string;
  rows?: number;
};

/**
 * 用「外部值 + 行草稿」成对状态实现受控同步：
 * 外部值变化时按新值重算行；用户编辑时把草稿留在本地，避免无效草稿被
 * 每一轮渲染覆盖，也避免在 effect 里同步 setState。
 */
type EditorState = {
  source: string | undefined;
  rows: StructuredJsonRow[];
};

function invalidRowNumbers(rows: StructuredJsonRow[]): number[] {
  const duplicates = new Set<string>();
  const seen = new Set<string>();
  for (const row of rows) {
    const key = row.key.trim();
    if (!key) continue;
    if (seen.has(key)) duplicates.add(key);
    seen.add(key);
  }

  const invalid: number[] = [];
  rows.forEach((row, index) => {
    const key = row.key.trim();
    const badNumber = row.kind === "number" && !Number.isFinite(Number(row.value));
    const badNested = row.kind === "json" && !isValidJson(String(row.value));
    if (!key || duplicates.has(key) || badNumber || badNested) invalid.push(index + 1);
  });
  return invalid;
}

function duplicateKeys(rows: StructuredJsonRow[]): Set<string> {
  const seen = new Set<string>();
  const duplicates = new Set<string>();
  for (const row of rows) {
    const key = row.key.trim();
    if (!key) continue;
    if (seen.has(key)) duplicates.add(key);
    seen.add(key);
  }
  return duplicates;
}

export function StructuredJsonEditor({
  value,
  onChange,
  onInvalidChange,
  rows = 4,
  label,
  id,
  "aria-invalid": ariaInvalid,
  "aria-describedby": ariaDescribedBy,
}: StructuredJsonEditorProps) {
  const { t } = useTranslation("common");
  const { t: tr } = useTranslation("resource");
  const [advanced, setAdvanced] = useState(false);
  const [state, setState] = useState<EditorState>(() => ({
    source: value,
    rows: parseStructuredJson(value) ?? [],
  }));

  const parsed = parseStructuredJson(value);
  const canStructure = parsed !== null;
  const rowsState = state.source === value ? state.rows : (parsed ?? []);
  const showAdvanced = advanced || !canStructure;

  const commit = (nextRows: StructuredJsonRow[]) => {
    const text = serializeStructuredJson(nextRows);
    if (text === null) {
      setState({ source: value, rows: nextRows });
      return;
    }
    setState({ source: text, rows: nextRows });
    onChange?.(text);
  };

  const updateRow = (rowId: number, patch: Partial<StructuredJsonRow>) => {
    commit(rowsState.map((row) => (row.id === rowId ? { ...row, ...patch } : row)));
  };

  const handleToggleAdvanced = (checked: boolean) => {
    // 切到高级模式时把（可能非法的）草稿带过去，避免用户已看到的内容丢失。
    if (checked) {
      const text = serializeStructuredJson(rowsState) ?? serializeStructuredJsonDraft(rowsState);
      setState({ source: text, rows: rowsState });
      if (text !== value) onChange?.(text);
    }
    setAdvanced(checked);
  };

  const invalidMessage = showAdvanced
    ? value && value.trim() !== "" && !isValidJson(value)
      ? t("invalidFormat")
      : null
    : (() => {
        const lines = invalidRowNumbers(rowsState);
        return lines.length > 0 ? tr("organization.invalidRows", { rows: lines.join(", ") }) : null;
      })();

  const reportedRef = useRef<string | null>(null);
  useEffect(() => {
    if (reportedRef.current === invalidMessage) return;
    reportedRef.current = invalidMessage;
    onInvalidChange?.(invalidMessage, showAdvanced ? "advanced" : "structured");
  }, [invalidMessage, showAdvanced, onInvalidChange]);

  const errorId = id ? `${id}-error` : undefined;
  const invalidRows = new Set(invalidRowNumbers(rowsState));
  const duplicates = duplicateKeys(rowsState);

  const describedBy =
    [ariaDescribedBy, !showAdvanced && invalidMessage ? errorId : undefined]
      .filter(Boolean)
      .join(" ") || undefined;

  return (
    <fieldset
      aria-label={label}
      aria-describedby={describedBy}
      style={{ border: 0, margin: 0, padding: 0, minWidth: 0 }}
    >
      <Checkbox
        id={showAdvanced ? id : undefined}
        checked={showAdvanced}
        disabled={!canStructure}
        onChange={(event) => handleToggleAdvanced(event.target.checked)}
        style={{ marginBottom: 8 }}
      >
        {tr("organization.advancedJsonMode")}
      </Checkbox>

      {showAdvanced ? (
        <JsonEditor
          value={value}
          onChange={onChange}
          rows={rows}
          error={invalidMessage ?? undefined}
        />
      ) : (
        <div>
          <div
            style={{
              display: "flex",
              gap: 8,
              marginBottom: 4,
              fontSize: 12,
              opacity: 0.65,
            }}
            aria-hidden
          >
            <span style={{ width: 200 }}>{t("key")}</span>
            <span style={{ flex: 1 }}>{t("value")}</span>
            <span style={{ width: 32 }} />
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            {rowsState.map((row, index) => {
              const line = index + 1;
              const rowInvalid = invalidRows.has(line);
              const keyEmpty = row.key.trim() === "";
              const keyDuplicate = duplicates.has(row.key.trim());
              const keyInvalid = keyEmpty || keyDuplicate;
              const valueInvalid =
                (row.kind === "number" && !Number.isFinite(Number(row.value))) ||
                (row.kind === "json" && !isValidJson(String(row.value)));
              const keyLabel = tr("organization.rowFieldAria", {
                field: t("key"),
                row: line,
              });
              const valueLabel = tr("organization.rowFieldAria", {
                field: t("value"),
                row: line,
              });
              return (
                <div key={row.id} style={{ display: "flex", gap: 8, alignItems: "flex-start" }}>
                  <div style={{ display: "flex", gap: 4, alignItems: "center", width: 200 }}>
                    <Input
                      id={index === 0 ? id : undefined}
                      value={row.key}
                      status={keyInvalid ? "error" : undefined}
                      placeholder={t("key")}
                      aria-label={keyLabel}
                      aria-invalid={
                        index === 0
                          ? keyInvalid || ariaInvalid || undefined
                          : keyInvalid || undefined
                      }
                      aria-describedby={rowInvalid ? errorId : undefined}
                      onChange={(event) => updateRow(row.id, { key: event.target.value })}
                    />
                    {keyDuplicate && (
                      <Tooltip title={tr("organization.duplicateKey")}>
                        <ExclamationCircleOutlined
                          aria-label={tr("organization.duplicateKey")}
                          style={{ color: "var(--gl-color-error)" }}
                        />
                      </Tooltip>
                    )}
                  </div>
                  <div style={{ flex: 1 }}>
                    {row.kind === "boolean" ? (
                      <Switch
                        checked={Boolean(row.value)}
                        aria-label={valueLabel}
                        aria-invalid={valueInvalid || undefined}
                        aria-describedby={rowInvalid ? errorId : undefined}
                        onChange={(checked) => updateRow(row.id, { value: checked })}
                      />
                    ) : row.kind === "number" ? (
                      <InputNumber
                        value={Number(row.value)}
                        style={{ width: "100%" }}
                        aria-label={valueLabel}
                        aria-invalid={valueInvalid || undefined}
                        aria-describedby={rowInvalid ? errorId : undefined}
                        onChange={(next) => updateRow(row.id, { value: next ?? 0 })}
                      />
                    ) : row.kind === "json" ? (
                      <Input.TextArea
                        value={String(row.value)}
                        status={valueInvalid ? "error" : undefined}
                        autoSize={{ minRows: 2, maxRows: 6 }}
                        style={{ fontFamily: "var(--gl-font-mono)" }}
                        aria-label={valueLabel}
                        aria-invalid={valueInvalid || undefined}
                        aria-describedby={rowInvalid ? errorId : undefined}
                        onChange={(event) => updateRow(row.id, { value: event.target.value })}
                      />
                    ) : (
                      <Input
                        value={String(row.value)}
                        aria-label={valueLabel}
                        aria-invalid={valueInvalid || undefined}
                        aria-describedby={rowInvalid ? errorId : undefined}
                        onChange={(event) => updateRow(row.id, { value: event.target.value })}
                      />
                    )}
                  </div>
                  <Tooltip title={tr("organization.rowDeleteAria", { row: line })}>
                    <Button
                      type="text"
                      danger
                      aria-label={tr("organization.rowDeleteAria", { row: line })}
                      icon={<DeleteOutlined />}
                      onClick={() => commit(rowsState.filter((current) => current.id !== row.id))}
                    />
                  </Tooltip>
                </div>
              );
            })}
          </div>

          <Button
            id={rowsState.length === 0 ? id : undefined}
            type="dashed"
            size="small"
            style={{ marginTop: 8 }}
            onClick={() =>
              commit([...rowsState, { id: nextRowId(), key: "", kind: "string", value: "" }])
            }
          >
            {tr("organization.addEntry")}
          </Button>

          {invalidMessage && (
            <div id={errorId} role="alert" style={{ marginTop: 8, color: "var(--gl-color-error)" }}>
              {invalidMessage}
            </div>
          )}
        </div>
      )}
    </fieldset>
  );
}

/**
 * 表单字段包装：把编辑器的非法草稿转成 `Form.Item` 校验失败，
 * 使全局保存被阻断（`ResourceFormPage` 保存前会 `validateFields()`）。
 */
export function StructuredJsonField({ name, label, rows = 4 }: StructuredJsonFieldProps) {
  const [invalid, setInvalid] = useState<{
    message: string | null;
    mode: "structured" | "advanced";
  }>({ message: null, mode: "structured" });
  const invalidRef = useRef<string | null>(null);

  const handleInvalidChange = useCallback(
    (message: string | null, mode: "structured" | "advanced") => {
      invalidRef.current = message;
      setInvalid({ message, mode });
    },
    [],
  );

  return (
    <Form.Item
      name={name}
      label={label}
      // 高级模式由 JsonEditor 自己展示本地化语法错误，避免与 help 重复。
      validateStatus={invalid.message && invalid.mode === "structured" ? "error" : undefined}
      help={invalid.mode === "structured" ? (invalid.message ?? undefined) : undefined}
      rules={[
        {
          validator: () =>
            invalidRef.current ? Promise.reject(new Error(invalidRef.current)) : Promise.resolve(),
        },
      ]}
    >
      <StructuredJsonEditor label={label} rows={rows} onInvalidChange={handleInvalidChange} />
    </Form.Item>
  );
}
