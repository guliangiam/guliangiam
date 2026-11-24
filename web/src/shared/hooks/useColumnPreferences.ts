import { useCallback, useEffect, useMemo, useRef, useState } from "react";

/**
 * useColumnPreferences — 表格列显隐持久化到 localStorage。
 *
 * 设计要点（来自经验反例）：
 *   1. restore-phase guard：挂载时先从 localStorage 读一次、合并到列、
 *      在此之前禁止任何写回，避免首帧默认状态覆盖用户保存的偏好。
 *   2. 新列兼容：localStorage 里只存 dataIndex 列表，列定义里出现的新 dataIndex
 *      不在保存列表中时 → 默认可见（defaultHidden 中的列除外）。
 *   3. 废弃列清理：dataIndex 在保存列表里但当前 columns 已不存在 → 丢弃。
 *   4. 单写入口：toggle/showAll/hideAll 都走 setVisible，避免多处散落写。
 *   5. 命名空间隔离：key = `gl:col-pref:${tableId}`，每个表独立。
 *   6. 默认隐藏列（defaultHidden）：首次进入时这些列不在可见集合里，但仍出现在列菜单中；
 *      用户显式开启后以持久化集合为准。columns 变化不会自动显示默认隐藏列。
 */

const DEFAULT_VERSION = 1;
const DEBOUNCE_MS = 300; // localStorage 写的 debounce，避免快速切换列时 IO 风暴

interface StoredPrefs {
  v: number;
  visible: string[];
}

/**
 * @param tableId 唯一表标识，构成 key `gl:col-pref:${tableId}`
 * @param columns 当前列定义（antd ColumnsType），用于提取 dataIndex 集合
 * @param opt-in 启用持久化。false 时 hook 返回 visible=全部、所有切换操作 no-op。
 * @param defaultHidden 首次进入时不显示的 dataIndex；用户仍可在列菜单中开启。
 */
export function useColumnPreferences(
  tableId: string | undefined,
  columns: Array<{ dataIndex?: string | number }>,
  enabled: boolean = true,
  defaultHidden: readonly string[] = [],
) {
  // 全部有效 dataIndex 集合（从 columns 提取）
  const allDataIndexes = useMemo(() => {
    const set = new Set<string>();
    columns.forEach((c) => {
      if (c.dataIndex) set.add(String(c.dataIndex));
    });
    return set;
  }, [columns]);

  // 默认隐藏列用内容键比较：调用方常传内联数组，按内容而不是引用 memo，避免每帧重建 Set
  const hiddenKey = defaultHidden.join("\u0000");
  const hiddenIndexes = useMemo(
    () => new Set(hiddenKey ? hiddenKey.split("\u0000") : []),
    [hiddenKey],
  );

  // 初始状态：未持久化 → 全部可见减去 defaultHidden
  const defaultVisible = useMemo(() => {
    const set = new Set<string>();
    allDataIndexes.forEach((di) => {
      if (!hiddenIndexes.has(di)) set.add(di);
    });
    return set;
  }, [allDataIndexes, hiddenIndexes]);

  const [visible, setVisibleState] = useState<Set<string>>(defaultVisible);
  const [dirty, setDirty] = useState(false);
  // restore-phase: mounted 时从 localStorage 读一次，在此之前 suppress 写
  const restoreDoneRef = useRef(false);
  // restore 刚做完且 dirty=true 时, sync effect 需要跳过首次错误执行
  const restoredDirtyRef = useRef(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // 用 ref 跟踪 current visible, 让 setVisible 能读到最新值 (避免闭包陈旧)
  const currentVisibleRef = useRef<Set<string>>(visible);
  useEffect(() => {
    currentVisibleRef.current = visible;
  }, [visible]);

  // --- Restore phase: 挂载后读一次 localStorage ------------------------
  // deps 刻意只留 [enabled, tableId]: allDataIndexes 变不应该触发重新 restore,
  // 它只影响"废弃列清理"——我们用下面的 sync effect 单独处理新列加入。
  useEffect(() => {
    if (!enabled || !tableId || restoreDoneRef.current) return;
    restoreDoneRef.current = true;

    try {
      const raw = localStorage.getItem(`gl:col-pref:${tableId}`);
      if (!raw) return; // 无持久化 → 保持 defaultVisible (全部可见减去 defaultHidden)
      const parsed: StoredPrefs = JSON.parse(raw);
      if (!parsed || parsed.v !== DEFAULT_VERSION || !Array.isArray(parsed.visible)) return;

      // 只保留在当前 columns 中仍存在的 dataIndex（废弃列清理）
      const restored = new Set<string>();
      parsed.visible.forEach((di) => {
        if (allDataIndexes.has(di)) restored.add(di);
      });

      // 如果恢复的集合跟"默认可见"不同，说明用户改过 → dirty=true
      const isDirty =
        restored.size !== defaultVisible.size ||
        Array.from(defaultVisible).some((di) => !restored.has(di));
      restoredDirtyRef.current = isDirty; // 供 sync effect 同步读取 (无 state 延迟)
      // One-time localStorage restore; restoreDoneRef guards against re-entry.
      // eslint-disable-next-line react/set-state-in-effect
      setVisibleState(restored.size > 0 ? restored : defaultVisible);
      // eslint-disable-next-line react/set-state-in-effect
      setDirty(isDirty);
    } catch {
      // 损坏 → 丢弃，回退默认可见（defaultHidden 仍保持隐藏）
    }
  }, [enabled, tableId, allDataIndexes, defaultVisible]);

  // --- Sync phase: 新列自动可见（未 dirty 时）----------------------------
  // 当 columns 定义变化（比如新增 expandIcon 列 / sort 列）:
  //   - dirty=false  → 用户没改过显隐, 自动把新 dataIndex 加入 visible
  //   - dirty=true   → 用户改过, 尊重用户选择, 不自动加
  // 用 restoredDirtyRef (同步 ref) 代替 state dirty, 避免 restore effect batch 后
  // sync effect 在 dirty state 更新前错误执行一次的 race condition.
  useEffect(() => {
    if (!restoreDoneRef.current) return; // restore 还没跑完
    if (dirty || restoredDirtyRef.current) return; // 用户改过, 不 sync

    setVisibleState((prev) => {
      let changed = false;
      const next = new Set(prev);
      allDataIndexes.forEach((di) => {
        // 默认隐藏列不参与自动同步：新列默认隐藏，用户需要显式在列菜单开启
        if (!hiddenIndexes.has(di) && !next.has(di)) {
          next.add(di);
          changed = true;
        }
      });
      return changed ? next : prev;
    });
  }, [allDataIndexes, dirty, hiddenIndexes]);

  // --- 持久化写入（debounced，单一写入口） ------------------------------
  const persist = useCallback(
    (next: Set<string>) => {
      if (!enabled || !tableId) return;
      if (debounceRef.current) clearTimeout(debounceRef.current);
      debounceRef.current = setTimeout(() => {
        try {
          const payload: StoredPrefs = { v: DEFAULT_VERSION, visible: Array.from(next) };
          localStorage.setItem(`gl:col-pref:${tableId}`, JSON.stringify(payload));
        } catch {
          // quota/private mode → 静默
        }
      }, DEBOUNCE_MS);
    },
    [enabled, tableId],
  );

  // --- 写操作（都走这里） ----------------------------------------------
  const setVisible = useCallback(
    (next: Set<string> | ((prev: Set<string>) => Set<string>)) => {
      const resolved =
        typeof next === "function"
          ? (next as (prev: Set<string>) => Set<string>)(currentVisibleRef.current)
          : next;
      currentVisibleRef.current = resolved;
      setVisibleState(resolved);
      // restore 完成后才允许写
      if (restoreDoneRef.current) {
        persist(resolved);
        setDirty(true);
      }
    },
    [persist],
  );

  // 用 ref 同步跟踪 current visible, 这样 setVisible 回调版本能读到最新值

  const toggle = useCallback(
    (dataIndex: string) => {
      if (!enabled) return;
      setVisible((prev: Set<string>) => {
        const next = new Set(prev);
        if (next.has(dataIndex)) {
          // 至少留 1 列可见
          if (next.size <= 1) return prev;
          next.delete(dataIndex);
        } else {
          next.add(dataIndex);
        }
        return next;
      });
    },
    [setVisible, enabled],
  );

  const showAll = useCallback(() => {
    if (!enabled) return;
    setVisible(new Set(allDataIndexes));
  }, [setVisible, allDataIndexes, enabled]);

  const hideAll = useCallback(() => {
    if (!enabled) return;
    // 留第一个非默认隐藏列；全部列都默认隐藏时退回收缩到第一列
    const first =
      Array.from(allDataIndexes).find((di) => !hiddenIndexes.has(di)) ??
      allDataIndexes.values().next().value;
    if (!first) return;
    setVisible(new Set([first]));
  }, [setVisible, allDataIndexes, hiddenIndexes, enabled]);

  const reset = useCallback(() => {
    if (!enabled || !tableId) return;
    try {
      localStorage.removeItem(`gl:col-pref:${tableId}`);
    } catch {
      /* noop */
    }
    // reset 回到默认可见集合（默认隐藏列保持隐藏）
    setVisibleState(new Set(defaultVisible));
    setDirty(false);
  }, [enabled, tableId, defaultVisible]);

  // --- Unmount cleanup --------------------------------------------------
  useEffect(
    () => () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    },
    [],
  );

  return {
    visible,
    dirty,
    toggle,
    showAll,
    hideAll,
    reset,
    allDataIndexes,
  };
}
