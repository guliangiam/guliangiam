import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { useAdapterDetailPageController } from "./useAdapterDetailPageController";
import { AdapterDetailPageView } from "./AdapterDetailPageView";

export default function AdapterDetailPage() {
  return (
    <PermissionGuard action="adapter:read">
      <AdapterDetailPageWorkspace />
    </PermissionGuard>
  );
}

function AdapterDetailPageWorkspace() {
  const ctrl = useAdapterDetailPageController();
  return <AdapterDetailPageView ctrl={ctrl} />;
}
