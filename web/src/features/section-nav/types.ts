/**
 * 详情 / 配置页的页内分区导航（副菜单）共享契约。
 *
 * 核心原则：一套分区内容只渲染一次，三种落位（右锚点 / 左二级 / 顶部横条）
 * 只是外层壳的栅格与定位差异。方案键因此是闭集常量，非法或缺失值一律回退
 * 系统默认，不抛错、不留空态。
 */
export type SectionNavScheme = "rail-right" | "rail-left" | "rail-top";

/**
 * 方案闭集。用文件级静态常量声明，避免与调用点形成 TDZ 依赖
 * （对齐 docs/GuliangIAM-开发规范 §2.12.7）。
 */
export const SECTION_NAV_SCHEMES = ["rail-right", "rail-left", "rail-top"] as const;

export const DEFAULT_SECTION_NAV_SCHEME: SectionNavScheme = "rail-right";

export function isSectionNavScheme(value: unknown): value is SectionNavScheme {
  return typeof value === "string" && (SECTION_NAV_SCHEMES as readonly string[]).includes(value);
}

/**
 * 分区清单条目。导航条目与表单分区同源渲染（单一事实源），禁止两处手写重复列表。
 */
export type SectionNavItem = {
  key: string;
  /** 分区容器的 DOM id，供锚点定位与 Scrollspy 观察。 */
  anchor: string;
  label: string;
  /**
   * 分区在当前上下文不渲染时置 true（如无权限、类型不适用）：条目保留以维持
   * 稳定的分区清单，但不参与高亮、不可点击。
   */
  disabled?: boolean;
};
