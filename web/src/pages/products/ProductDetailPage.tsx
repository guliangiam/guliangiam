import { useProductDetailPageController } from "./useProductDetailPageController";
import { ProductDetailPageView } from "./ProductDetailPageView";

export default function ProductDetailPage() {
  const ctrl = useProductDetailPageController();
  return <ProductDetailPageView ctrl={ctrl} />;
}
