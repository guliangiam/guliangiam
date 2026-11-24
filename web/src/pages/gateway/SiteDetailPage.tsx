import { useSiteDetailPageController } from "./useSiteDetailPageController";
import { SiteDetailPageView } from "./SiteDetailPageView";

export default function SiteDetailPage() {
  const ctrl = useSiteDetailPageController();
  return <SiteDetailPageView ctrl={ctrl} />;
}
