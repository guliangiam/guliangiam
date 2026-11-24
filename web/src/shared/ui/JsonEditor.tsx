import { Input, Typography } from "antd";
import { useCallback, useState } from "react";

type JsonEditorProps = {
  value?: string;
  onChange?: (value: string) => void;
  readOnly?: boolean;
  rows?: number;
  placeholder?: string;
  error?: string;
};

function tryFormatJson(str: string): { formatted: string; valid: boolean } {
  if (!str.trim()) {
    return { formatted: "", valid: true };
  }
  try {
    const parsed = JSON.parse(str);
    return { formatted: JSON.stringify(parsed, null, 2), valid: true };
  } catch {
    return { formatted: str, valid: false };
  }
}

export function JsonEditor({
  value = "",
  onChange,
  readOnly = false,
  rows = 8,
  placeholder = '{\n  "key": "value"\n}',
  error,
}: JsonEditorProps) {
  const [internalError, setInternalError] = useState<string | null>(null);

  const handleChange = useCallback(
    (e: React.ChangeEvent<HTMLTextAreaElement>) => {
      const newVal = e.target.value;
      onChange?.(newVal);

      if (newVal.trim()) {
        try {
          JSON.parse(newVal);
          setInternalError(null);
        } catch (err) {
          setInternalError((err as SyntaxError).message);
        }
      } else {
        setInternalError(null);
      }
    },
    [onChange],
  );

  const handleBlur = useCallback(() => {
    if (value.trim()) {
      try {
        const parsed = JSON.parse(value);
        const formatted = JSON.stringify(parsed, null, 2);
        onChange?.(formatted);
        setInternalError(null);
      } catch {
        // keep as-is if invalid
      }
    }
  }, [value, onChange]);

  const displayError = error ?? internalError;

  if (readOnly) {
    const { formatted } = tryFormatJson(value);
    return (
      <pre className="gl-json-editor-readonly" style={{ maxHeight: rows * 21 + 16 }}>
        {formatted || value || "-"}
      </pre>
    );
  }

  return (
    <div>
      <Input.TextArea
        value={value}
        onChange={handleChange}
        onBlur={handleBlur}
        rows={rows}
        placeholder={placeholder}
        style={{
          fontFamily: "var(--gl-font-mono)",
          fontSize: "var(--gl-font-size-sm)",
        }}
        status={displayError ? "error" : undefined}
      />
      {displayError && (
        <Typography.Text type="danger" style={{ fontSize: "var(--gl-font-size-xs)" }}>
          {displayError}
        </Typography.Text>
      )}
    </div>
  );
}
