import { useTicketDetailPageController } from "./useTicketDetailPageController";
import { TicketDetailPageView } from "./TicketDetailPageView";

export default function TicketDetailPage() {
  const ctrl = useTicketDetailPageController();
  return <TicketDetailPageView ctrl={ctrl} />;
}
