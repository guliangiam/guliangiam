import { useCallback } from "react";
import { MonacoTextEditor } from "@/shared/ui/MonacoTextEditor";
import { useThemeStore } from "@/features/theme/theme-store";

type CodeEditorLanguage =
  | "html"
  | "css"
  | "javascript"
  | "json"
  | "typescript"
  | "plaintext"
  | "xml";

interface CodeEditorProps {
  value?: string;
  onChange?: (value: string) => void;
  language?: CodeEditorLanguage;
  readOnly?: boolean;
  height?: number | string;
  placeholder?: string;
}

export function CodeEditor({
  value = "",
  onChange,
  language = "html",
  readOnly = false,
  height = 300,
  placeholder,
}: CodeEditorProps) {
  const resolvedTheme = useThemeStore((s) => s.resolvedTheme);
  const monacoTheme = resolvedTheme === "dark" ? "vs-dark" : "light";

  const handleChange = useCallback(
    (newValue: string) => {
      onChange?.(newValue);
    },
    [onChange],
  );

  return (
    <div
      style={{ border: "1px solid var(--ant-color-border)", borderRadius: 6, overflow: "hidden" }}
    >
      <MonacoTextEditor
        height={height}
        language={language}
        theme={monacoTheme}
        value={value || placeholder || ""}
        onChange={handleChange}
        options={{
          readOnly,
          minimap: { enabled: false },
          scrollBeyondLastLine: false,
          fontSize: 13,
          lineNumbers: "on",
          wordWrap: "on",
          automaticLayout: true,
          padding: { top: 8 },
          overviewRulerBorder: false,
          scrollbar: {
            verticalScrollbarSize: 8,
            horizontalScrollbarSize: 8,
          },
        }}
      />
    </div>
  );
}
