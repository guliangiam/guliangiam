import { useEffect, useRef, useState, type ReactNode } from "react";

type MonacoApi = typeof import("monaco-editor");
type MonacoEditorOptions = NonNullable<Parameters<MonacoApi["editor"]["create"]>[1]>;
type MonacoEditorInstance = ReturnType<MonacoApi["editor"]["create"]>;

let monacoPromise: Promise<MonacoApi> | null = null;

function loadMonaco(): Promise<MonacoApi> {
  monacoPromise ??= import("@/shared/ui/monaco-setup").then((module) => module.monaco);
  return monacoPromise;
}

export interface MonacoTextEditorProps {
  value: string;
  language: string;
  theme: string;
  height?: number | string;
  options?: MonacoEditorOptions;
  onChange?: (value: string) => void;
  fallback?: ReactNode;
}

/**
 * Monaco 的受控包装：与原先 @monaco-editor/react 的 props 语义保持一致
 * （value / language / theme / height / options / onChange），但运行时来自本地
 * 打包的 monaco-editor，首次渲染时才按需加载 chunk，不进入首屏静态依赖。
 */
export function MonacoTextEditor({
  value,
  language,
  theme,
  height = 300,
  options,
  onChange,
  fallback,
}: MonacoTextEditorProps) {
  const hostRef = useRef<HTMLDivElement | null>(null);
  const editorRef = useRef<MonacoEditorInstance | null>(null);
  const valueRef = useRef(value);
  const onChangeRef = useRef(onChange);
  const initialRef = useRef({ language, theme, options });
  const [monacoApi, setMonacoApi] = useState<MonacoApi | null>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    onChangeRef.current = onChange;
  }, [onChange]);

  useEffect(() => {
    let disposed = false;
    let contentListener: { dispose(): void } | undefined;
    const initial = initialRef.current;

    const emitChange = (next: string) => {
      if (next === valueRef.current) {
        return;
      }
      valueRef.current = next;
      onChangeRef.current?.(next);
    };

    const createEditor = async () => {
      const monaco = await loadMonaco();
      if (disposed || !hostRef.current) {
        return;
      }
      const editor = monaco.editor.create(hostRef.current, {
        value: valueRef.current,
        language: initial.language,
        theme: initial.theme,
        ...initial.options,
        automaticLayout: true,
      });
      editorRef.current = editor;
      contentListener = editor.onDidChangeModelContent(() => {
        emitChange(editor.getValue());
      });
      setMonacoApi(monaco);
      setReady(true);
    };

    void createEditor();

    return () => {
      disposed = true;
      contentListener?.dispose();
      editorRef.current?.dispose();
      editorRef.current = null;
    };
  }, []);

  useEffect(() => {
    valueRef.current = value;
    const editor = editorRef.current;
    if (editor && editor.getValue() !== value) {
      editor.setValue(value);
    }
  }, [value]);

  useEffect(() => {
    const model = editorRef.current?.getModel();
    if (!monacoApi || !model || model.getLanguageId() === language) {
      return;
    }
    monacoApi.editor.setModelLanguage(model, language);
  }, [monacoApi, language]);

  useEffect(() => {
    monacoApi?.editor.setTheme(theme);
  }, [monacoApi, theme]);

  useEffect(() => {
    editorRef.current?.updateOptions({ ...options, automaticLayout: true });
  }, [options]);

  return (
    <div
      style={{ position: "relative", height: typeof height === "number" ? `${height}px` : height }}
    >
      <div ref={hostRef} style={{ position: "absolute", inset: 0 }} />
      {!ready && fallback ? <div style={{ position: "absolute", inset: 0 }}>{fallback}</div> : null}
    </div>
  );
}
