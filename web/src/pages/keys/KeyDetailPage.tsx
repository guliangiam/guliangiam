import { useKeyDetailPageController } from "./useKeyDetailPageController";
import { KeyDetailPageView } from "./KeyDetailPageView";

export default function KeyDetailPage() {
  const ctrl = useKeyDetailPageController();
  return <KeyDetailPageView ctrl={ctrl} />;
}
