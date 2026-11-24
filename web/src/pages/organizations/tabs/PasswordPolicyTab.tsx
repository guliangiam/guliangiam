/**
 * Organization 详情页 · 认证与安全分组的密码策略字段。
 * 字段实现与创建页共用 `OrganizationPasswordFields`。
 */
import { OrganizationPasswordFields } from "../sections/OrganizationPasswordFields";

export function PasswordPolicyTab() {
  return <OrganizationPasswordFields mode="edit" />;
}
