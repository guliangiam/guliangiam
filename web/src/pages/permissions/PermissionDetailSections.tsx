/**
 * PermissionDetailPage 的 FormSection 块
 *
 * 从 PermissionDetailPage.tsx 拆分而来：
 *   - PermissionCasbinSection       Casbin 模型 / 适配器 / 应用 / 排序
 *   - PermissionSubResourcesSection 子用户 / 子用户组 / 子角色 / 域
 *
 * 分页选择器 loader 由共享的 UserPaginateSelect / GroupPaginateSelect /
 * RolePaginateSelect 组件内部 useCallback 管理，避免 PaginateSelect
 * 依赖 loader 身份而触发的加载时序问题。
 */
import { Form, Input, Select } from "antd";
import { useTranslation } from "react-i18next";
import type { ApplicationOption } from "@/shared/api/iam/application";
import { DATABASE_ADAPTER_OPTIONS } from "@/shared/constants/enums";
import { FormSection } from "@/shared/ui/FormSection";
import { UserPaginateSelect } from "@/shared/ui/UserPaginateSelect";
import { GroupPaginateSelect } from "@/shared/ui/GroupPaginateSelect";
import { RolePaginateSelect } from "@/shared/ui/RolePaginateSelect";

import { PERMISSION_MODEL_OPTIONS } from "./permission-options";

type PermissionCasbinSectionProps = {
  appOptions: ApplicationOption[] | undefined;
  appOptionsLoading: boolean;
};

export function PermissionCasbinSection({
  appOptions,
  appOptionsLoading,
}: PermissionCasbinSectionProps) {
  const { t: tr } = useTranslation("resource");

  return (
    <FormSection title={tr("permission.casbinConfig")}>
      <Form.Item label={tr("permission.model")} name="model">
        <Select
          allowClear
          showSearch
          placeholder={tr("permission.selectModel")}
          options={PERMISSION_MODEL_OPTIONS}
        />
      </Form.Item>
      <Form.Item label={tr("permission.adapter")} name="adapter">
        <Select
          allowClear
          showSearch
          placeholder={tr("permission.selectAdapter")}
          options={DATABASE_ADAPTER_OPTIONS}
        />
      </Form.Item>
      <Form.Item label={tr("permission.application")} name="application">
        <Select
          allowClear
          showSearch
          placeholder={tr("permission.selectApplication")}
          // label 统一为「displayName (slug)」形态；value 用应用 id，与创建页 ApplicationSelect 一致。
          options={(appOptions ?? []).map((item) => ({
            label: item.displayName ? `${item.displayName} (${item.slug})` : item.slug,
            value: item.id,
          }))}
          loading={appOptionsLoading}
        />
      </Form.Item>
      <Form.Item label={tr("permission.sorter")} name="sorter">
        <Input />
      </Form.Item>
    </FormSection>
  );
}

export function PermissionSubResourcesSection() {
  const { t: tr } = useTranslation("resource");

  return (
    <FormSection title={tr("permission.subResources")}>
      <Form.Item label={tr("permission.subUsers")} name="users">
        <UserPaginateSelect
          placeholder={tr("permission.searchUsers")}
          allOptionValue="*"
          allOptionLabel={tr("permission.allUsers")}
        />
      </Form.Item>
      <Form.Item label={tr("permission.subGroups")} name="groups">
        <GroupPaginateSelect
          placeholder={tr("permission.searchGroups")}
          allOptionValue="*"
          allOptionLabel={tr("permission.allGroups")}
        />
      </Form.Item>
      <Form.Item label={tr("permission.subRoles")} name="roles">
        <RolePaginateSelect placeholder={tr("permission.searchRoles")} />
      </Form.Item>
      <Form.Item label={tr("permission.subDomains")} name="domains">
        <Select mode="tags" placeholder={tr("permission.domainPlaceholder")} />
      </Form.Item>
    </FormSection>
  );
}
