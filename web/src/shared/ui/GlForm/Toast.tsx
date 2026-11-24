import React from "react";
import { message, Empty as AntEmpty, Button } from "antd";
import type { MessageInstance } from "antd/es/message/interface";

/**
 * GuliangIAM 统一 Toast 封装 — antd message 的薄桥接层。
 *
 * 设计目标：
 *   - 集中 antd message API（避免业务代码直接 import 'antd'）
 *   - 统一 duration + placement 默认值
 *   - 统一 loading spinner 管理（loading → success/error）
 *   - 预留未来 i18n 包装 / custom icon / error code 映射的接入点
 *
 * 为什么不用 sonner？antd message 已经有完整 stacking key + hook API，
 * 且项目已全量 antd 6，减少依赖。
 */

let toastInstance: MessageInstance | null = null;

/** 初始化 — AppProviders 里调一次即可。也可以不调（antd message auto-init）。 */
// oxlint-disable-next-line react/only-export-components -- 本模块是 toast 单例 + GlEmpty 的混合模块，拆分需改动 GlForm/index.ts 的导出方
export function initGlToast(instance: MessageInstance) {
  toastInstance = instance;
}

const defaultInstance = () => toastInstance ?? message;

const TOAST_KEY = "gl-toast"; // 统一 key — 所有 toast 堆叠到同一容器

const DEFAULT_DURATION = 3; // 秒

// oxlint-disable-next-line react/only-export-components -- toast 是与 GlEmpty 同文件的门面对象，拆分需改动 GlForm/index.ts 的导出方
export const toast = {
  success(content: React.ReactNode, duration = DEFAULT_DURATION) {
    defaultInstance().success({ content, duration, key: TOAST_KEY });
  },

  error(content: React.ReactNode, duration = DEFAULT_DURATION) {
    defaultInstance().error({ content, duration, key: TOAST_KEY });
  },

  warning(content: React.ReactNode, duration = DEFAULT_DURATION) {
    defaultInstance().warning({ content, duration, key: TOAST_KEY });
  },

  info(content: React.ReactNode, duration = DEFAULT_DURATION) {
    defaultInstance().info({ content, duration, key: TOAST_KEY });
  },

  /**
   * loading → success/error 配对快捷方法。
   * 返回 dismisser 函数 — 调用时把 loading 替换成 success/error，或直接关闭。
   */
  async withLoading<T>(
    asyncFn: () => Promise<T>,
    {
      loadingText = "Loading…",
      successText,
      errorText,
    }: {
      loadingText?: React.ReactNode;
      successText?: React.ReactNode | ((result: T) => React.ReactNode);
      errorText?: React.ReactNode | ((err: unknown) => React.ReactNode);
    } = {},
  ): Promise<T> {
    const key = `loading-${Date.now()}`;
    defaultInstance().loading({ content: loadingText, key, duration: 0 });
    try {
      const result = await asyncFn();
      if (successText !== undefined) {
        const content = typeof successText === "function" ? successText(result) : successText;
        defaultInstance().success({ content, key });
      } else {
        defaultInstance().destroy(key);
      }
      return result;
    } catch (err) {
      const content =
        typeof errorText === "function"
          ? errorText(err)
          : (errorText ?? (err as Error)?.message ?? "Error");
      defaultInstance().error({ content, key });
      throw err;
    }
  },

  /** 销毁指定 key 或全部 toast */
  destroy(key?: string) {
    if (key) defaultInstance().destroy(key);
    else defaultInstance().destroy();
  },
};

// --------------------------------------------------------------------------
// GlEmpty — 空状态 CTA 组件
// --------------------------------------------------------------------------

export interface GlEmptyProps {
  description?: React.ReactNode;
  /** CTA button — 通常是"新建 X" */
  action?: React.ReactNode;
  /** 预设类型 */
  type?: "default" | "simple";
  className?: string;
}

export const GlEmpty: React.FC<GlEmptyProps> = ({
  description,
  action,
  type = "default",
  className,
}) => {
  const emptyProps = type === "simple" ? { image: AntEmpty.PRESENTED_IMAGE_SIMPLE } : undefined;

  return (
    <div className={`gl-empty ${className ?? ""}`}>
      <AntEmpty {...emptyProps} description={description}>
        {action && <Button type="primary">{action}</Button>}
      </AntEmpty>
    </div>
  );
};
