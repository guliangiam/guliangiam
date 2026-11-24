import type { StorybookConfig } from "@storybook/react-vite";

const config: StorybookConfig = {
  // ── 故事文件搜索路径 ──
  stories: ["../src/shared/ui/**/*.stories.tsx", "../src/shared/ui/**/*.mdx"],

  // ── 附加组件 ──
  // storybook 10 把 essentials + interactions 内置到主包了，
  // 不再需要单独声明这两个 addon，否则会报 "Could not resolve addon"。
  addons: ["@storybook/addon-a11y"],

  // ── 文档模式：自动生成文档页 ──
  docs: {
    autodocs: true,
    defaultName: "文档",
  },

  // ── 使用 Vite builder（与项目构建工具一致） ──
  framework: {
    name: "@storybook/react-vite",
    options: {},
  },

  // ── Vite 配置扩展 ──
  viteFinal: async (config) => {
    // 添加 @ 路径别名，与 vite.config.ts 保持一致
    if (config.resolve) {
      config.resolve.alias = {
        ...config.resolve.alias,
        "@": "/src",
      };
    }

    // 排除 i18next-browser-languagedetector 的 CommonJS 兼容问题
    if (config.optimizeDeps) {
      config.optimizeDeps.include = [
        ...(config.optimizeDeps.include || []),
        "i18next",
        "react-i18next",
      ];
    }

    return config;
  },

  // ── TypeScript docgen 配置 ──
  // `@joshwooding/vite-plugin-react-docgen-typescript` 与 Vite 8 / TypeScript 7
  // 不兼容（build 时 fileExists undefined）。Storybook 10 的内置类型推断已覆盖
  // 组件 props；关闭外部 docgen，避免构建链路被第三方插件版本锁死。
  typescript: {
    reactDocgen: false,
  },
};

export default config;
