/**
 * 表单字段矩阵测试用的探针：记录当前渲染树里所有带 `name` 的 `Form.Item` / `Form.List`。
 *
 * 只在测试里替换 antd 的 `Form.Item` / `Form.List`，不改产品代码；`recordedFormItemNames`
 * 由每个 `ResourceFormPage` 渲染周期清空，因此读到的是「最后一次渲染实际挂载的字段」，
 * 而不是历史上出现过的字段并集。
 */
import type { ComponentType, ReactElement } from "react";

export const recordedFormItemNames = new Set<string>();

export function formatFormFieldName(name: unknown): string {
  if (Array.isArray(name)) {
    return name
      .map((segment) => (typeof segment === "number" ? `_${segment}` : String(segment)))
      .join(".")
      .replace(/\._/g, "_");
  }
  return String(name);
}

type AnyProps = Record<string, unknown>;

export function instrumentForm(FormObject: Record<string, unknown>): Record<string, unknown> {
  const ActualItem = FormObject.Item as ComponentType<AnyProps>;
  const ActualList = FormObject.List as ComponentType<AnyProps>;

  const Item = Object.assign((props: AnyProps): ReactElement => {
    if (props.name !== undefined) recordedFormItemNames.add(formatFormFieldName(props.name));
    return <ActualItem {...props} />;
  }, ActualItem);
  const List = Object.assign((props: AnyProps): ReactElement => {
    if (props.name !== undefined) recordedFormItemNames.add(formatFormFieldName(props.name));
    return <ActualList {...props} />;
  }, ActualList);

  return Object.assign({}, FormObject, { Item, List });
}
