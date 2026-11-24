import { useTransactionDetailPageController } from "./useTransactionDetailPageController";
import { TransactionDetailPageView } from "./TransactionDetailPageView";

export default function TransactionDetailPage() {
  const ctrl = useTransactionDetailPageController();
  return <TransactionDetailPageView ctrl={ctrl} />;
}
