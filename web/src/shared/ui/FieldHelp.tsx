import { QuestionCircleOutlined } from "@ant-design/icons";
import Tooltip from "antd/es/tooltip";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useHelpText } from "./help-text";

export { useHelpText } from "./help-text";

/**
 * 全局字段/功能帮助提示。所有页面共用同一图标、同一提示框样式与同一交互
 * （悬停 + 键盘 focus 均可触发），说明文本只来自 i18n 资源，不做 HTML 注入。
 *
 * 图标必须是**非 widget 角色**（`span[role=img]`）：本组件会被嵌入 Tabs 标签、
 * Card 标题等交互容器（`HelpLabel` 与全局注入器共用同一渲染），若渲染为
 * `<button>` 会触发 axe-core `nested-interactive`（serious）——其
 * `getFocusableDescendants` 仅豁免非 widget 角色，`tabindex=-1` 不豁免。
 * `tabIndex={0}` 保住键盘聚焦（Tooltip focus 触发与焦点环不变）。
 */
export interface HelpTipProps {
  text: ReactNode;
  ariaLabel?: string;
  className?: string;
}

export function HelpTip({ text, ariaLabel, className }: HelpTipProps) {
  const { t } = useTranslation("common");
  return (
    <Tooltip
      title={text}
      placement="topLeft"
      trigger={["hover", "focus"]}
      mouseEnterDelay={0.15}
      styles={{ root: { maxWidth: 320 } }}
    >
      {/* jsx-a11y 规则在此刻意豁免：role="img"（非 widget）是 axe `nested-interactive`
          豁免的前提，`tabIndex={0}` 承担 Tooltip 键盘 focus 触发，`onClick` 仅拦截冒泡
          （防点击图标误切 Tabs）；原生 <img> 无法承载图标子节点与焦点环。 */}
      {/* eslint-disable jsx-a11y/click-events-have-key-events, jsx-a11y/no-noninteractive-element-interactions, jsx-a11y/no-noninteractive-tabindex, jsx-a11y/prefer-tag-over-role */}
      <span
        className={className ? `gl-help-tip ${className}` : "gl-help-tip"}
        role="img"
        tabIndex={0}
        aria-label={ariaLabel ?? t("helpIcon")}
        onClick={(event) => {
          event.preventDefault();
          event.stopPropagation();
        }}
      >
        <QuestionCircleOutlined aria-hidden />
      </span>
      {/* eslint-enable jsx-a11y/click-events-have-key-events, jsx-a11y/no-noninteractive-element-interactions, jsx-a11y/no-noninteractive-tabindex, jsx-a11y/prefer-tag-over-role */}
    </Tooltip>
  );
}

export interface HelpLabelProps {
  children: ReactNode;
  /** help 命名空间内的键（如 `users.email`） */
  helpKey?: string;
  /** 直接给定说明文本，优先级高于 helpKey */
  text?: ReactNode;
  className?: string;
}

/**
 * 文本 + 问号图标。用于 Tabs 标签、Card 标题、独立功能点（开关/按钮）等
 * 不是 `GlFormRow` / `Form.Item` label 的位置。
 */
export function HelpLabel({ children, helpKey, text, className }: HelpLabelProps) {
  const helpText = useHelpText(helpKey);
  const resolved = text ?? helpText;
  return (
    <span className={className ? `gl-help-label ${className}` : "gl-help-label"}>
      {children}
      {resolved ? <HelpTip text={resolved} /> : null}
    </span>
  );
}
