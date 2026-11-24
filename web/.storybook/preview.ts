import type { Preview } from "@storybook/react";
import "@storybook/addon-a11y/preview";

// ── 引入 i18n 初始化（确保 useTranslation 可用） ──
import "@/shared/i18n";

// ── 引入全局样式 ──
import "@/styles.css";

const preview: Preview = {
  parameters: {
    // ── 控件面板配置 ──
    controls: {
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/i,
      },
      sort: "requiredFirst",
    },

    // ── 文档配置 ──
    docs: {
      source: {
        // 自动生成源码展示
        type: "dynamic",
        excludeDecorators: true,
      },
      canvas: {
        sourceState: "shown",
      },
      toc: true, // 启用目录
    },

    // ── 布局配置 ──
    layout: "centered",

    // ── 视口配置（响应式预览） ──
    viewport: {
      viewport: {
        mobile: {
          name: "移动端",
          styles: { width: "375px", height: "812px" },
        },
        tablet: {
          name: "平板",
          styles: { width: "768px", height: "1024px" },
        },
        desktop: {
          name: "桌面端",
          styles: { width: "1280px", height: "800px" },
        },
      },
    },

    // ── 背景配置 ──
    backgrounds: {
      disable: true,
    },

    // ── 国际化配置 ──
    locale: "zh-CN",
    locales: {
      "zh-CN": "简体中文",
      en: "English",
    },
  },

  // ── 全局装饰器：提供 i18n 上下文 ──
  decorators: [],

  // ── 全局类型标签 ──
  tags: ["autodocs"],
};

export default preview;
