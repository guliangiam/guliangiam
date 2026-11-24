/**
 * Organization 详情页 · 基本与品牌分组的基础字段。
 * 字段实现与创建页共用 `OrganizationBasicFields`。
 */
import type { DeploymentRegionOption } from "@/shared/api/iam/organization";
import { OrganizationBasicFields } from "../sections/OrganizationBasicFields";

type BasicInfoTabProps = {
  currentOrganizationId?: string;
  regionOptions?: DeploymentRegionOption[];
};

export function BasicInfoTab({ currentOrganizationId, regionOptions = [] }: BasicInfoTabProps) {
  return (
    <OrganizationBasicFields
      mode="edit"
      currentOrganizationId={currentOrganizationId}
      regionOptions={regionOptions}
    />
  );
}
