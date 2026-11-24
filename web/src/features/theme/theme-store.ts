import { create } from "zustand";
import { BRAND_PALETTE, THEME_TOKEN_CSS_VARS } from "@/app/theme/brand-palette";

type ThemeMode = "light" | "dark" | "system";
type ResolvedTheme = "light" | "dark";

type ThemeConfig = {
  primaryColor: string;
  darkPrimaryColor: string;
  borderRadius: number;
};

export type { ThemeConfig };

/** 主题编辑器选定的自定义值；为 `null` 时使用品牌默认值。 */
export function resolvePrimaryColor(resolved: ResolvedTheme, config: ThemeConfig): string {
  const override = resolved === "dark" ? config.darkPrimaryColor : config.primaryColor;
  return override || BRAND_PALETTE[resolved].colorPrimary;
}

type ThemeState = {
  themeMode: ThemeMode;
  resolvedTheme: ResolvedTheme;
  config: ThemeConfig;
  toggleTheme: () => void;
  setThemeMode: (mode: ThemeMode) => void;
  setConfig: (config: Partial<ThemeConfig>) => void;
  applyConfig: (config: ThemeConfig) => void;
};

function resolveTheme(mode: ThemeMode): ResolvedTheme {
  if (mode === "system") {
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }
  return mode;
}

function getInitialTheme(): ThemeMode {
  if (typeof window === "undefined") return "light";
  const stored = localStorage.getItem("gl-theme-mode");
  if (stored === "dark" || stored === "light" || stored === "system") return stored;
  return "system";
}

export const DEFAULT_CONFIG: ThemeConfig = {
  primaryColor: BRAND_PALETTE.light.colorPrimary,
  darkPrimaryColor: BRAND_PALETTE.dark.colorPrimary,
  borderRadius: 10,
};

function getInitialConfig(): ThemeConfig {
  if (typeof window === "undefined") return DEFAULT_CONFIG;
  try {
    const stored = localStorage.getItem("gl-theme-config");
    if (stored) {
      const parsed = JSON.parse(stored);
      return { ...DEFAULT_CONFIG, ...parsed };
    }
  } catch {
    // ignore invalid JSON
  }
  return DEFAULT_CONFIG;
}

function applyThemeToDocument(resolved: ResolvedTheme) {
  document.documentElement.setAttribute("data-theme", resolved);
}

/**
 * 把主题编辑器选定的主色写回 CSS 令牌。
 *
 * 只写 `--gl-color-primary`：`--gl-color-primary-bg` 在 `styles.css` 里由
 * `color-mix()` 从它派生，因此派生色自动跟随，不需要第二处手写。
 * antd 侧通过 `ConfigProvider` 的 `colorPrimary` 令牌消费同一份配置
 * （`AppProviders` 调用 `resolvePrimaryColor`），两边同源。
 *
 * inline 样式优先级高于 `[data-theme="dark"]` 选择器，因此必须按当前解析
 * 主题写值：写死浅色主色会让深色模式整站主色停留在浅色值（对比度不达标）。
 */
function applyConfigToDocument(config: ThemeConfig, resolved: ResolvedTheme) {
  const root = document.documentElement;
  root.style.setProperty(THEME_TOKEN_CSS_VARS.colorPrimary, resolvePrimaryColor(resolved, config));
}

export const useThemeStore = create<ThemeState>((set, get) => {
  const initial = getInitialTheme();
  const resolved = resolveTheme(initial);
  applyThemeToDocument(resolved);

  const initialConfig = getInitialConfig();
  applyConfigToDocument(initialConfig, resolved);

  // Listen for system theme changes when in "system" mode
  if (typeof window !== "undefined") {
    const mql = window.matchMedia("(prefers-color-scheme: dark)");
    const handler = () => {
      if (get().themeMode === "system") {
        const newResolved = resolveTheme("system");
        applyThemeToDocument(newResolved);
        applyConfigToDocument(get().config, newResolved);
        set({ resolvedTheme: newResolved });
      }
    };
    mql.addEventListener("change", handler);
  }

  return {
    themeMode: initial,
    resolvedTheme: resolved,
    config: initialConfig,
    toggleTheme: () =>
      set((state) => {
        const cycle: ThemeMode[] = ["light", "dark", "system"];
        const idx = cycle.indexOf(state.themeMode);
        const next = cycle[(idx + 1) % cycle.length];
        const newResolved = resolveTheme(next);
        localStorage.setItem("gl-theme-mode", next);
        applyThemeToDocument(newResolved);
        applyConfigToDocument(state.config, newResolved);
        return { themeMode: next, resolvedTheme: newResolved };
      }),
    setThemeMode: (mode) => {
      const newResolved = resolveTheme(mode);
      localStorage.setItem("gl-theme-mode", mode);
      applyThemeToDocument(newResolved);
      applyConfigToDocument(get().config, newResolved);
      set({ themeMode: mode, resolvedTheme: newResolved });
    },
    setConfig: (partial) => {
      const newConfig = { ...get().config, ...partial };
      localStorage.setItem("gl-theme-config", JSON.stringify(newConfig));
      applyConfigToDocument(newConfig, get().resolvedTheme);
      set({ config: newConfig });
    },
    applyConfig: (config) => {
      localStorage.setItem("gl-theme-config", JSON.stringify(config));
      applyConfigToDocument(config, get().resolvedTheme);
      set({ config });
    },
  };
});
