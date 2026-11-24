import { useOrderDetailPageController } from "./useOrderDetailPageController";
import { OrderDetailPageView } from "./OrderDetailPageView";

export default function OrderDetailPage() {
  const ctrl = useOrderDetailPageController();
  return <OrderDetailPageView ctrl={ctrl} />;
}
