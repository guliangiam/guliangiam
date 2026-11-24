/**
 * 页内分区导航（副菜单）落位偏好的本机持久化。
 *
 * 纯 UI 状态：写 `localStorage`，键 `guliangiam.sectionNavScheme`，不进服务端缓存。
 * 覆盖链为「本机偏好 → 组织级默认（后端预留）→ 系统默认 rail-right」；当前仅实现
 * 首尾两级，中间级接入后只需扩展 {@link resolveSectionNavScheme}，壳层代码不变。
 */
import { create } from "zustand";
import { persist } from "zustand/middleware";
import { DEFAULT_SECTION_NAV_SCHEME, isSectionNavScheme, type SectionNavScheme } from "./types";

export type SectionNavState = {
  /** 本机偏好；`null` 表示从未选择过，走系统默认。 */
  scheme: SectionNavScheme | null;
  /** 最近一次选择时间（ISO8601）；`null` 表示当前取的是系统默认。 */
  updatedAt: string | null;
  setScheme: (scheme: SectionNavScheme) => void;
  resetScheme: () => void;
};

export const SECTION_NAV_STORAGE_KEY = "guliangiam.sectionNavScheme";

export const useSectionNavStore = create<SectionNavState>()(
  persist(
    (set) => ({
      scheme: null,
      updatedAt: null,
      setScheme: (scheme) => set({ scheme, updatedAt: new Date().toISOString() }),
      resetScheme: () => set({ scheme: null, updatedAt: null }),
    }),
    {
      name: SECTION_NAV_STORAGE_KEY,
      partialize: (state) => ({ scheme: state.scheme, updatedAt: state.updatedAt }),
      // 读取失败 / 非法值 / 缺失值一律回退系统默认，不抛错、不留空态。
      merge: (persisted, current) => {
        const incoming = persisted as Partial<SectionNavState> | undefined;
        if (!incoming || !isSectionNavScheme(incoming.scheme)) {
          return { ...current, scheme: null, updatedAt: null };
        }
        return {
          ...current,
          scheme: incoming.scheme,
          updatedAt: typeof incoming.updatedAt === "string" ? incoming.updatedAt : null,
        };
      },
    },
  ),
);

/**
 * 解析当前生效方案。本机偏好缺失时回退系统默认；组织级默认接入后在此处插入
 * 优先级，调用方（壳层）无需感知。
 */
export function resolveSectionNavScheme(state: {
  scheme: SectionNavScheme | null;
}): SectionNavScheme {
  return state.scheme ?? DEFAULT_SECTION_NAV_SCHEME;
}

/** 选择器：供组件订阅解析后的方案，避免订阅整个 store。 */
export const selectResolvedSectionNavScheme = (state: SectionNavState): SectionNavScheme =>
  resolveSectionNavScheme(state);
