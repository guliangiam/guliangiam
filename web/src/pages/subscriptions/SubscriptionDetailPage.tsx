import { useSubscriptionDetailPageController } from "./useSubscriptionDetailPageController";
import { SubscriptionDetailPageView } from "./SubscriptionDetailPageView";

export default function SubscriptionDetailPage() {
  const ctrl = useSubscriptionDetailPageController();
  return <SubscriptionDetailPageView ctrl={ctrl} />;
}
