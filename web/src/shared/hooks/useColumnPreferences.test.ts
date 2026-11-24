import { describe, it, expect, beforeEach, vi, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useColumnPreferences } from "./useColumnPreferences";

const TABLE_ID = "test-users-table";
const COLS = [
  { dataIndex: "id" },
  { dataIndex: "name" },
  { dataIndex: "email" },
  { dataIndex: "status" },
];

const STORAGE_KEY = `gl:col-pref:${TABLE_ID}`;

describe("useColumnPreferences", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  describe("initial state (no persisted prefs)", () => {
    it("defaults ALL columns visible", () => {
      const { result } = renderHook(() => useColumnPreferences(TABLE_ID, COLS, true));
      expect(result.current.visible.size).toBe(4);
      COLS.forEach((c) => expect(result.current.visible.has(String(c.dataIndex))).toBe(true));
      expect(result.current.dirty).toBe(false);
    });

    it("respects enabled=false → visible=all, toggle no-op", () => {
      const { result } = renderHook(() => useColumnPreferences(undefined, COLS, false));
      expect(result.current.visible.size).toBe(4);
      act(() => result.current.toggle("name"));
      expect(result.current.visible.size).toBe(4); // no-op
    });
  });

  describe("localStorage restore", () => {
    it("restores saved prefs on mount", () => {
      localStorage.setItem(STORAGE_KEY, JSON.stringify({ v: 1, visible: ["id", "name"] }));
      const { result } = renderHook(() => useColumnPreferences(TABLE_ID, COLS, true));
      expect(result.current.visible.has("id")).toBe(true);
      expect(result.current.visible.has("name")).toBe(true);
      expect(result.current.visible.has("email")).toBe(false);
      expect(result.current.visible.has("status")).toBe(false);
      expect(result.current.dirty).toBe(true);
    });

    it("discards prefs for columns that no longer exist", async () => {
      localStorage.setItem(STORAGE_KEY, JSON.stringify({ v: 1, visible: ["id", "deletedCol"] }));
      const { result } = renderHook(() => useColumnPreferences(TABLE_ID, COLS, true));
      await act(async () => {
        vi.runAllTimers();
      });
      expect(result.current.visible.has("id")).toBe(true);
      expect(result.current.visible.has("deletedCol")).toBe(false);
    });

    it("falls back to all-visible when stored data is corrupted", async () => {
      localStorage.setItem(STORAGE_KEY, "not-json{{{");
      const { result } = renderHook(() => useColumnPreferences(TABLE_ID, COLS, true));
      await act(async () => {
        vi.runAllTimers();
      });
      // 损坏 → catch → 保持 defaultVisible (全部可见)
      expect(result.current.visible.size).toBe(4);
    });
  });

  describe("toggle", () => {
    it("hides a column, persists to localStorage (debounced)", async () => {
      const { result } = renderHook(() => useColumnPreferences(TABLE_ID, COLS, true));
      act(() => result.current.toggle("email"));
      expect(result.current.visible.has("email")).toBe(false);
      expect(result.current.visible.size).toBe(3);

      // debounce 300ms 还没到
      expect(localStorage.getItem(STORAGE_KEY)).toBeNull();

      // 推进 timer → persist
      act(() => vi.advanceTimersByTime(400));
      const stored = JSON.parse(localStorage.getItem(STORAGE_KEY)!);
      expect(stored.visible).not.toContain("email");
      expect(stored.visible).toContain("id");
    });

    it("keeps at least 1 column visible", () => {
      const singleCol = [{ dataIndex: "id" }];
      const { result } = renderHook(() => useColumnPreferences(TABLE_ID, singleCol, true));
      act(() => result.current.toggle("id"));
      expect(result.current.visible.size).toBe(1); // 不能 hide 最后一个
    });
  });

  describe("new column auto-sync (dirty=false)", () => {
    it("auto-adds new dataIndex when columns grow + not dirty", async () => {
      const initialCols = [{ dataIndex: "id" }, { dataIndex: "name" }];
      const { result, rerender } = renderHook(
        ({ cols }) => useColumnPreferences(TABLE_ID, cols, true),
        { initialProps: { cols: initialCols } },
      );

      // 无 dirty → 全可见
      expect(result.current.visible.size).toBe(2);

      // 翻页 / schema 变更 → columns 变多 (加了 email)
      const expanded = [{ dataIndex: "id" }, { dataIndex: "name" }, { dataIndex: "email" }];
      rerender({ cols: expanded });

      // sync effect 触发 → 新列自动可见
      await act(async () => {
        vi.runAllTimers();
      });
      expect(result.current.visible.has("email")).toBe(true);
      expect(result.current.visible.size).toBe(3);
      expect(result.current.dirty).toBe(false);
    });

    it("does NOT auto-add when dirty=true (user has custom)", async () => {
      const { result, rerender } = renderHook(
        ({ cols }) => useColumnPreferences(TABLE_ID, cols, true),
        { initialProps: { cols: COLS } },
      );
      // 用户手动 hide 一个 → dirty=true
      act(() => result.current.toggle("email"));
      expect(result.current.dirty).toBe(true);

      // columns 扩展
      const expanded = [...COLS, { dataIndex: "newCol" }];
      rerender({ cols: expanded });
      await act(async () => {
        vi.runAllTimers();
      });

      // dirty=true → 不自动加 newCol
      expect(result.current.visible.has("newCol")).toBe(false);
      expect(result.current.visible.has("email")).toBe(false); // 用户隐藏保留
    });
  });

  describe("showAll / hideAll / reset", () => {
    it("showAll 恢复全部可见 + dirty=true", () => {
      const { result } = renderHook(() => useColumnPreferences(TABLE_ID, COLS, true));
      act(() => result.current.toggle("email"));
      act(() => result.current.showAll());
      expect(result.current.visible.size).toBe(4);
      expect(result.current.dirty).toBe(true);
    });

    it("hideAll 只留第一列", () => {
      const { result } = renderHook(() => useColumnPreferences(TABLE_ID, COLS, true));
      act(() => result.current.hideAll());
      expect(result.current.visible.size).toBe(1);
      expect(result.current.visible.has("id")).toBe(true);
    });

    it("reset 清 localStorage + 全可见", async () => {
      localStorage.setItem(STORAGE_KEY, JSON.stringify({ v: 1, visible: ["id"] }));
      const { result } = renderHook(() => useColumnPreferences(TABLE_ID, COLS, true));
      await act(async () => {
        vi.runAllTimers();
      });
      expect(result.current.dirty).toBe(true);

      act(() => result.current.reset());
      expect(localStorage.getItem(STORAGE_KEY)).toBeNull();
      expect(result.current.visible.size).toBe(4);
      expect(result.current.dirty).toBe(false);
    });
  });

  describe("defaultHidden columns", () => {
    const COLS_WITH_HIDDEN = [...COLS, { dataIndex: "tag" }, { dataIndex: "registerSource" }];
    const DEFAULT_HIDDEN = ["tag", "registerSource"];

    it("hides default-hidden columns on first visit, rest stays visible", () => {
      const { result } = renderHook(() =>
        useColumnPreferences(TABLE_ID, COLS_WITH_HIDDEN, true, DEFAULT_HIDDEN),
      );
      expect(result.current.visible.size).toBe(4);
      expect(result.current.visible.has("tag")).toBe(false);
      expect(result.current.visible.has("registerSource")).toBe(false);
      expect(result.current.visible.has("id")).toBe(true);
      expect(result.current.dirty).toBe(false);
    });

    it("keeps default-hidden columns toggleable via the column menu", () => {
      const { result } = renderHook(() =>
        useColumnPreferences(TABLE_ID, COLS_WITH_HIDDEN, true, DEFAULT_HIDDEN),
      );
      act(() => result.current.toggle("tag"));
      expect(result.current.visible.has("tag")).toBe(true);
      expect(result.current.dirty).toBe(true);
    });

    it("honours a persisted choice that includes a default-hidden column", () => {
      localStorage.setItem(STORAGE_KEY, JSON.stringify({ v: 1, visible: ["id", "tag"] }));
      const { result } = renderHook(() =>
        useColumnPreferences(TABLE_ID, COLS_WITH_HIDDEN, true, DEFAULT_HIDDEN),
      );
      expect(result.current.visible.has("tag")).toBe(true);
      expect(result.current.visible.has("registerSource")).toBe(false);
      expect(result.current.dirty).toBe(true);
    });

    it("does not auto-reveal a default-hidden column added later", async () => {
      const initialCols = [{ dataIndex: "id" }, { dataIndex: "name" }];
      const { result, rerender } = renderHook(
        ({ cols }) => useColumnPreferences(TABLE_ID, cols, true, DEFAULT_HIDDEN),
        { initialProps: { cols: initialCols } },
      );
      rerender({ cols: [...initialCols, { dataIndex: "tag" }] });
      await act(async () => {
        vi.runAllTimers();
      });
      expect(result.current.visible.has("tag")).toBe(false);
      expect(result.current.dirty).toBe(false);
    });

    it("showAll reveals default-hidden columns explicitly", () => {
      const { result } = renderHook(() =>
        useColumnPreferences(TABLE_ID, COLS_WITH_HIDDEN, true, DEFAULT_HIDDEN),
      );
      act(() => result.current.showAll());
      expect(result.current.visible.size).toBe(6);
      expect(result.current.visible.has("tag")).toBe(true);
    });

    it("reset returns to default visibility and keeps default-hidden columns hidden", async () => {
      localStorage.setItem(
        STORAGE_KEY,
        JSON.stringify({ v: 1, visible: ["id", "name", "tag", "registerSource"] }),
      );
      const { result } = renderHook(() =>
        useColumnPreferences(TABLE_ID, COLS_WITH_HIDDEN, true, DEFAULT_HIDDEN),
      );
      await act(async () => {
        vi.runAllTimers();
      });

      act(() => result.current.reset());
      expect(localStorage.getItem(STORAGE_KEY)).toBeNull();
      expect(result.current.visible.size).toBe(4);
      expect(result.current.visible.has("tag")).toBe(false);
      expect(result.current.dirty).toBe(false);
    });
  });

  describe("Set reference stability on actions", () => {
    it("toggle returns a new Set (not mutation)", () => {
      const { result } = renderHook(() => useColumnPreferences(TABLE_ID, COLS, true));
      const before = result.current.visible;
      act(() => result.current.toggle("email"));
      expect(result.current.visible).not.toBe(before); // 新引用
    });

    it("consecutive toggles produce stable visible downstream", () => {
      const { result } = renderHook(() => useColumnPreferences(TABLE_ID, COLS, true));
      const refs: Set<string>[] = [];
      refs.push(result.current.visible);
      act(() => result.current.toggle("email"));
      refs.push(result.current.visible);
      act(() => result.current.toggle("email")); // 切回来
      refs.push(result.current.visible);

      // 每次 toggle 后 Set 引用变 → 但 content 有意义
      expect(refs[0].has("email")).toBe(true);
      expect(refs[1].has("email")).toBe(false);
      expect(refs[2].has("email")).toBe(true);
    });
  });
});
