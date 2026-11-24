import { Form } from "antd";
import type { FormInstance } from "antd";
import { useCallback, useEffect, useRef, useState } from "react";

type Snapshot = Record<string, unknown>;

type HistoryState = {
  stack: Snapshot[];
  index: number;
};

type Options = {
  /** 实体键：切换应用/新建态时清空历史，避免跨实体撤销。 */
  entityKey: string;
  /** 程序化写回表单后触发（setFieldsValue 不经过 onValuesChange，脏检查需手动补）。 */
  onApply?: () => void;
  debounceMs?: number;
  limit?: number;
};

function pick<T extends object>(form: FormInstance<T>, fields: readonly string[]): Snapshot {
  const values = form.getFieldsValue(true) as Record<string, unknown>;
  const out: Snapshot = {};
  for (const field of fields) out[field] = values[field];
  return structuredClone(out);
}

/** 界面定制字段组的撤销/重做历史。只快照指定字段，不波及其他 Tab；
 * 用户输入防抖入栈，undo/redo 写回时不入栈。 */
export function useUiConfigHistory<T extends object>(
  form: FormInstance<T>,
  fields: readonly string[],
  { entityKey, onApply, debounceMs = 300, limit = 50 }: Options,
) {
  const [history, setHistory] = useState<HistoryState>({ stack: [], index: -1 });
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const selector = useCallback(
    (store: T) => fields.map((field) => (store as Record<string, unknown>)[field]),
    [fields],
  );
  const watched = Form.useWatch(selector, { form, preserve: true });

  // 实体切换：重置历史并同步捕获基线。基线不能靠防抖定时器补捕——
  // 挂载期的定时器会被 watch 初始更新的重渲染清掉，首次真正入栈时
  // 表单可能已被编辑，导致起点丢失（canUndo 永远为 false）。
  // entityKey 由父组件在灌值完成后才更新，此处快照到的即是新实体的值。
  useEffect(() => {
    if (timerRef.current) clearTimeout(timerRef.current);
    // oxlint-disable-next-line react/set-state-in-effect -- 实体切换是外部实体同步点，历史必须随实体键重置
    setHistory({ stack: [pick(form, fields)], index: 0 });
  }, [entityKey, form, fields]);

  useEffect(() => {
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => {
      const snapshot = pick(form, fields);
      setHistory((prev) => {
        if (prev.stack.length === 0) return { stack: [snapshot], index: 0 };
        const base = prev.stack.slice(0, prev.index + 1);
        const last = base[base.length - 1];
        if (last && JSON.stringify(last) === JSON.stringify(snapshot)) return prev;
        const stack = [...base, snapshot].slice(-limit);
        return { stack, index: stack.length - 1 };
      });
    }, debounceMs);
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, [watched, form, fields, debounceMs, limit]);

  const apply = useCallback(
    (snapshot: Snapshot) => {
      if (timerRef.current) clearTimeout(timerRef.current);
      form.setFieldsValue(snapshot as Partial<T>);
      onApply?.();
    },
    [form, onApply],
  );

  const undo = useCallback(() => {
    if (history.index <= 0) return;
    const next = history.index - 1;
    setHistory({ stack: history.stack, index: next });
    apply(history.stack[next]);
  }, [history, apply]);

  const redo = useCallback(() => {
    if (history.index >= history.stack.length - 1) return;
    const next = history.index + 1;
    setHistory({ stack: history.stack, index: next });
    apply(history.stack[next]);
  }, [history, apply]);

  return {
    canUndo: history.index > 0,
    canRedo: history.index >= 0 && history.index < history.stack.length - 1,
    undo,
    redo,
  };
}
