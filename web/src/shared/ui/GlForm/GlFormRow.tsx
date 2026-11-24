import "./GlForm.css";

import React from "react";
import { Form } from "antd";
import { HelpTip, useHelpText } from "@/shared/ui/FieldHelp";

/**
 * GlFormRow — 行级表单容器，自动桥接 antd Form.Item 的 validation 状态。
 *
 * 问题：Casdoor 的 FormRow 是纯展示层，与 antd Form.Item 的 validation
 * 状态完全隔离——开发者要手动写 Form.Item + 错误展示 + shake 动画。
 *
 * GuliangIAM 方案：GlFormRow 消费 antd 6 的 Form.Item.useStatus()，
 * 自动联动 errors → 行级 shake + label 红色 + feedback 文案渲染，
 * validating → loading indicator。
 *
 * 用法：
 *   <Form.Item name="email" rules={[...]}>
 *     <GlFormRow label="邮箱" required>
 *       <Input placeholder="your@email.com" />
 *     </GlFormRow>
 *   </Form.Item>
 *
 * 注意：GlFormRow 不替代 Form.Item — validation rules / name / dependencies
 * 仍然写在外层 Form.Item 上。GlFormRow 只负责视觉桥接。
 */
export interface GlFormRowProps {
  /** 等同于 antd Form.Item 的 label，支持传入 i18n key 字符串 */
  label?: React.ReactNode;
  /** 帮助文案（Form.Item.extra 语义） */
  extra?: React.ReactNode;
  /** 名称后问号图标的说明文本（显式给定，优先级最高） */
  help?: React.ReactNode;
  /** help 命名空间内的帮助键（如 `users.email`），未登记时交给全局注入器兜底 */
  helpKey?: string;
  /** 是否为必填——自动渲染红色星号 */
  required?: boolean;
  /** inline 模式：label 在左，控件在右（单行紧凑布局） */
  inline?: boolean;
  /** GlFormGrid 容器内使用，指定列跨度（1-12，自动映射 grid-column: span N / 12） */
  colSpan?: number;
  /** 子元素：应放在 antd Form.Item 内部，GlFormRow 从中读取 context */
  children: React.ReactNode;
}

/**
 * GlFormRow 是 Form.Item 的直接子元素，antd 会把受控属性（value/onChange/id/
 * aria-*）通过 cloneElement 注入到这里。这些属性必须继续透传给内部控件，
 * 否则表单既收不到用户输入（双向绑定失效），label 也无法关联到输入框。
 */
type InjectedControlProps = Record<string, unknown>;

export const GlFormRow: React.FC<GlFormRowProps & InjectedControlProps> = ({
  label,
  required,
  inline,
  colSpan,
  extra,
  help,
  helpKey,
  children,
  ...controlProps
}) => {
  // antd 4.22+ / antd 6：返回 { status, errors, warnings, validateTrigger }
  // 必须位于 Form.Item 之内，否则 antd 6 发 useStatus 控制台警告（E2E 控制台门禁拦截）。
  const itemStatus = Form.Item.useStatus();
  const helpText = useHelpText(helpKey);
  const resolvedHelp = help ?? helpText;
  const hasErrors = (itemStatus?.errors?.length ?? 0) > 0;
  const showError = hasErrors; // useStatus 的 errors 已合并 touched 语义（验证过时才会非空）
  const isValidating = itemStatus?.status === "validating";

  const className = [
    "gl-form-row",
    showError && "gl-form-row--error",
    isValidating && "gl-form-row--validating",
    inline && "gl-form-row--inline",
  ]
    .filter(Boolean)
    .join(" ");

  const gridStyle: React.CSSProperties | undefined =
    colSpan !== undefined
      ? { gridColumn: `span ${clamp(colSpan, 1, 12)} / span ${clamp(colSpan, 1, 12)}` }
      : undefined;

  const fieldId = typeof controlProps.id === "string" ? controlProps.id : undefined;
  const control = React.isValidElement(children)
    ? React.cloneElement(children as React.ReactElement<InjectedControlProps>, controlProps)
    : children;

  return (
    <div className={className} style={gridStyle}>
      {label !== undefined && label !== null && (
        <label
          className={`gl-form-row__label${required ? " gl-form-row__label-required" : ""}`}
          htmlFor={fieldId}
          data-gl-help-done={resolvedHelp ? "1" : undefined}
        >
          {label}
          {resolvedHelp ? <HelpTip text={resolvedHelp} /> : null}
          {isValidating && <span className="gl-form-row__indicator" aria-hidden />}
        </label>
      )}
      <span className="gl-form-row__control">{control}</span>
      {/* extra 的 id 与 Form.Item 生成的 aria-describedby（`${fieldId}_extra`）对齐。 */}
      {extra && (
        <span className="gl-form-row__extra" id={fieldId ? `${fieldId}_extra` : undefined}>
          {extra}
        </span>
      )}
    </div>
  );
};

function clamp(n: number, lo: number, hi: number): number {
  return Math.min(Math.max(n, lo), hi);
}
