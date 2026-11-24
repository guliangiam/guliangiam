/**
 * Organization 详情页 · 基本与品牌分组的品牌字段。
 * 字段实现与创建页共用 `OrganizationBrandingFields`。
 */
import type { ApplicationOption } from "@/shared/api/iam/application";
import { OrganizationBrandingFields } from "../sections/OrganizationBrandingFields";

type BrandingTabProps = {
  organizationName: string;
  applicationOptions?: ApplicationOption[];
  applicationsLoading?: boolean;
};

export function BrandingTab({
  organizationName,
  applicationOptions = [],
  applicationsLoading = false,
}: BrandingTabProps) {
  return (
    <OrganizationBrandingFields
      mode="edit"
      organizationName={organizationName}
      applicationOptions={applicationOptions}
      applicationsLoading={applicationsLoading}
    />
  );
}
