import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { useOrgDomainsPageController } from "./useOrgDomainsPageController";
import { OrgDomainsPageView } from "./OrgDomainsPageView";

export default function OrgDomainsPage() {
  return (
    <PermissionGuard action="b2b:read">
      <OrgDomainsWorkspace />
    </PermissionGuard>
  );
}

function OrgDomainsWorkspace() {
  const ctrl = useOrgDomainsPageController();
  return <OrgDomainsPageView ctrl={ctrl} />;
}
