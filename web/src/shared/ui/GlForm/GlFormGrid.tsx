import React from "react";
import "./GlForm.css";
import "./GlFormGrid.css";

/**
 * GlFormGrid — 响应式表单网格容器。
 *
 * 问题：Casdoor FormGrid 是简单的 CSS Grid 包装，没有断点感知。
 * GuliangIAM 方案：默认 auto-fit minmax(280px, 1fr)，在 md/lg 断点
 * 按 data-columns-* 属性切换为固定列数，同时支持 colSpan 让单个 GlFormRow
 * 跨多列。
 *
 * 断点：
 *   < md (768px) : 默认 1-2 列自适应
 *   >= md (768px): 2 列（或 columns.md 覆盖）
 *   >= lg (1024px): 3 列（或 columns.lg 覆盖）
 *
 * 用法：
 *   <GlFormGrid columns={{ default: 1, md: 2, lg: 3 }}>
 *     <GlFormRow label="名称">...</GlFormRow>
 *     <GlFormRow label="类型">...</GlFormRow>
 *     <GlFormRow label="描述" colSpan={2}>...</GlFormRow>
 *   </GlFormGrid>
 */
export interface GlFormGridProps {
  /** 列数。数字：所有断点同数；对象：按断点指定 */
  columns?: number | { default?: number; md?: number; lg?: number };
  /** 间距。数字（px）或对象。默认 16px 水平 + 8px 垂直 */
  gap?: number | string | { default?: number | string; md?: number | string; lg?: number | string };
  /** 子元素（通常是 GlFormRow） */
  children: React.ReactNode;
}

const BREAKPOINTS = { md: 768, lg: 1024 } as const;

function resolveColumns(input: GlFormGridProps["columns"]): {
  default?: number;
  md?: number;
  lg?: number;
} {
  if (typeof input === "number") return { default: input, md: input, lg: input };
  return input ?? {};
}

function resolveGap(input: GlFormGridProps["gap"]): { default?: string; md?: string; lg?: string } {
  if (input === undefined) return {};
  if (typeof input === "number")
    return { default: `${input}px`, md: `${input}px`, lg: `${input}px` };
  if (typeof input === "string") return { default: input, md: input, lg: input };
  const out: { default?: string; md?: string; lg?: string } = {};
  const target = out as Record<string, string | undefined>;
  for (const [k, v] of Object.entries(input)) {
    target[k] = typeof v === "number" ? `${v}px` : v;
  }
  return out;
}

export const GlFormGrid: React.FC<GlFormGridProps> = ({ columns, gap, children }) => {
  const cols = resolveColumns(columns);
  const gaps = resolveGap(gap);

  const style: React.CSSProperties = {
    display: "grid",
    gap: gaps.default ?? undefined,
    gridTemplateColumns: cols.default ? `repeat(${cols.default}, minmax(0, 1fr))` : undefined,
  };

  const dataAttrs: Record<string, string> = {};
  if (cols.md) dataAttrs["data-columns-md"] = String(cols.md);
  if (cols.lg) dataAttrs["data-columns-lg"] = String(cols.lg);
  if (gaps.md) dataAttrs["data-gap-md"] = gaps.md;
  if (gaps.lg) dataAttrs["data-gap-lg"] = gaps.lg;

  return (
    <div className="gl-form-grid" style={style} {...dataAttrs} data-testid="gl-form-grid">
      {children}
    </div>
  );
};

// 断点常量导出，方便外部扩展
export const GL_FORM_GRID_BREAKPOINTS = BREAKPOINTS;
