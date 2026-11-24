/**
 * Organization 详情页 · 认证与安全分组的访问控制字段。
 * 字段实现与创建页共用 `OrganizationSecurityFields`。
 */
import { OrganizationSecurityFields } from "../sections/OrganizationSecurityFields";

export function SecurityPolicyTab({
  section = "all",
}: {
  section?: "access" | "mfa" | "session" | "all";
}) {
  return <OrganizationSecurityFields mode="edit" section={section} />;
}
