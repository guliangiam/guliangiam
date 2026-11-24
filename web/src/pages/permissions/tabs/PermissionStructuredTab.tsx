/**
 * 权限详情页「结构化编辑器」Tab
 *
 * 从 PermissionDetailPage.tsx 拆分而来。资源类型切换会决定资源字段的
 * 输入模式（API → 多选、其它 → tags），因此本组件自行 watch 表单。
 */
import type { FormInstance } from "antd";
import { Form, Select } from "antd";
import { useTranslation } from "react-i18next";
import {
  API_ACTIONS,
  APPLICATION_ACTIONS,
  EFFECT_OPTIONS,
  RESOURCE_TYPE_OPTIONS,
} from "../permission-detail-helpers";
import type { PermissionFormValues } from "./types";

type PermissionStructuredTabProps = {
  form: FormInstance<PermissionFormValues>;
};

export default function PermissionStructuredTab({ form }: PermissionStructuredTabProps) {
  const { t: tr } = useTranslation("resource");

  const resourceType = Form.useWatch("resourceType", form) ?? "Application";
  const isApi = resourceType === "API";
  const actionOptions = isApi ? API_ACTIONS : APPLICATION_ACTIONS;

  return (
    <>
      <Form.Item label={tr("permission.resourceType")} name="resourceType">
        <Select
          showSearch
          allowClear
          options={RESOURCE_TYPE_OPTIONS}
          placeholder={tr("permission.selectResourceType")}
        />
      </Form.Item>
      <Form.Item label={tr("permission.resources")} name="resources">
        <Select
          mode={isApi ? "multiple" : "tags"}
          allowClear
          placeholder={tr("permission.resourcesPlaceholder")}
          tokenSeparators={[","]}
        />
      </Form.Item>
      <Form.Item label={tr("permission.actions")} name="actions">
        <Select
          mode="multiple"
          allowClear
          placeholder={tr("permission.selectActions")}
          options={actionOptions}
        />
      </Form.Item>
      <Form.Item label={tr("permission.effect")} name="effect">
        <Select options={EFFECT_OPTIONS} />
      </Form.Item>
    </>
  );
}
