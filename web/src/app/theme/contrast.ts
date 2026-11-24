import { BRAND_PALETTE } from "./brand-palette";

/** WCAG 2.1 相对亮度。 */
function channelToLinear(value: number): number {
  const c = value / 255;
  return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
}

type Rgba = { r: number; g: number; b: number; a: number };

const HEX_PATTERN = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i;
const RGBA_PATTERN = /^rgba?\(([^)]+)\)$/i;

/** 解析 `#rgb`、`#rrggbb`、`rgb()`、`rgba()`；非法输入抛错而不是静默返回 NaN。 */
export function parseColor(color: string): Rgba {
  const value = color.trim();
  if (HEX_PATTERN.test(value)) {
    const hex = value.slice(1);
    const full =
      hex.length === 3
        ? hex
            .split("")
            .map((ch) => ch + ch)
            .join("")
        : hex;
    return {
      r: Number.parseInt(full.slice(0, 2), 16),
      g: Number.parseInt(full.slice(2, 4), 16),
      b: Number.parseInt(full.slice(4, 6), 16),
      a: 1,
    };
  }
  const match = RGBA_PATTERN.exec(value);
  if (match) {
    const parts = match[1].split(",").map((part) => part.trim());
    if (parts.length < 3) throw new Error(`Unsupported color: ${color}`);
    return {
      r: Number(parts[0]),
      g: Number(parts[1]),
      b: Number(parts[2]),
      a: parts.length > 3 ? Number(parts[3]) : 1,
    };
  }
  throw new Error(`Unsupported color: ${color}`);
}

/** 把（可能半透明的）前景色合成到背景色上。 */
export function compositeOver(foreground: string, background: string): Rgba {
  const fg = parseColor(foreground);
  if (fg.a >= 1) return fg;
  const bg = parseColor(background);
  const mix = (f: number, b: number) => Math.round(f * fg.a + b * (1 - fg.a));
  return { r: mix(fg.r, bg.r), g: mix(fg.g, bg.g), b: mix(fg.b, bg.b), a: 1 };
}

function luminanceOf({ r, g, b }: Rgba): number {
  return 0.2126 * channelToLinear(r) + 0.7152 * channelToLinear(g) + 0.0722 * channelToLinear(b);
}

export function relativeLuminance(color: string, background = "#ffffff"): number {
  return luminanceOf(compositeOver(color, background));
}

/**
 * WCAG 2.1 对比度，取值 1–21。
 *
 * 半透明前景先按 `background` 合成再计算（透明文本的真实对比度取决于其落底），
 * 因此比较「次要文本 vs 卡片底色」时必须传入真实底色。
 */
export function contrastRatio(foreground: string, background: string): number {
  const a = relativeLuminance(foreground, background);
  const b = relativeLuminance(background);
  const [hi, lo] = a >= b ? [a, b] : [b, a];
  return (hi + 0.05) / (lo + 0.05);
}

/** WCAG 2.1 AA 正文对比度门槛。 */
export const AA_TEXT_CONTRAST = 4.5;

/**
 * 选择实心主色底上的文字色：在品牌前景色候选（白 / 正文色）里取对比度更高者。
 *
 * 统一由一个函数决定可读性，避免每个主题各写一份「白字还是深字」的静态映射
 * （暗色主题品牌色明亮，静态白字必然不达标）。
 */
export function pickReadableText(background: string): string {
  const candidates = [BRAND_PALETTE.light.colorBgContainer, BRAND_PALETTE.light.colorText];
  return candidates.reduce((best, candidate) =>
    contrastRatio(candidate, background) > contrastRatio(best, background) ? candidate : best,
  );
}

/**
 * 把前景色向白色方向提亮，直到与背景的对比度 ≥ target。
 *
 * 暗黑模式下组织 `colorPrimary` 直接作为链接/文字按钮前景时（antd `colorLink`），
 * 深品牌色在深底上往往只有 3:1 左右，达不到 AA 正文 4.5:1；按步长混白直到达标，
 * 仍保持品牌色相。找不到达标值时回退纯白。
 */
export function lightenToContrast(
  foreground: string,
  background: string,
  target = AA_TEXT_CONTRAST,
): string {
  if (contrastRatio(foreground, background) >= target) return foreground;
  const fg = parseColor(foreground);
  for (let step = 1; step <= 20; step++) {
    const t = step / 20;
    const mix = (v: number) => Math.round(v + (255 - v) * t);
    const candidate = `rgb(${mix(fg.r)}, ${mix(fg.g)}, ${mix(fg.b)})`;
    if (contrastRatio(candidate, background) >= target) return candidate;
  }
  return "#ffffff";
}

/**
 * 把前景色向黑色方向压暗，直到与背景的对比度 ≥ target（lightenToContrast 的浅色对偶）。
 *
 * 浅色模式下组织覆盖的 `colorError` 会作为危险链接/文字按钮前景，antd 默认
 * #ff4d4f 在白底只有 3.26:1（axe color-contrast serious）；按步长混黑直到达标，
 * 仍保持色相。找不到达标值时回退纯黑。
 */
export function darkenToContrast(
  foreground: string,
  background: string,
  target = AA_TEXT_CONTRAST,
): string {
  if (contrastRatio(foreground, background) >= target) return foreground;
  const fg = parseColor(foreground);
  for (let step = 1; step <= 20; step++) {
    const t = step / 20;
    const mix = (v: number) => Math.round(v * (1 - t));
    const candidate = `rgb(${mix(fg.r)}, ${mix(fg.g)}, ${mix(fg.b)})`;
    if (contrastRatio(candidate, background) >= target) return candidate;
  }
  return "#000000";
}
