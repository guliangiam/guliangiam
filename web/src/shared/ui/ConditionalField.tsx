import type { ReactNode } from "react";
import { memo } from "react";
import { Form } from "antd";

type ConditionalFieldProps = {
  dependencies: string[];
  when: (values: Record<string, unknown>) => boolean;
  children: ReactNode;
};

export const ConditionalField = memo(function ConditionalField({
  dependencies,
  when,
  children,
}: ConditionalFieldProps) {
  return (
    <Form.Item
      noStyle
      shouldUpdate={(prev, cur) => dependencies.some((dep) => prev[dep] !== cur[dep])}
    >
      {({ getFieldsValue }) =>
        when(getFieldsValue() as Record<string, unknown>) ? children : null
      }
    </Form.Item>
  );
});
