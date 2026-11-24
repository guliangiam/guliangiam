import { useCouponDetailPageController } from "./useCouponDetailPageController";
import { CouponDetailPageView } from "./CouponDetailPageView";

export default function CouponDetailPage() {
  const ctrl = useCouponDetailPageController();
  return <CouponDetailPageView ctrl={ctrl} />;
}
