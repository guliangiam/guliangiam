import { useAgentDetailPageController } from "./useAgentDetailPageController";
import { AgentDetailPageView } from "./AgentDetailPageView";

export default function AgentDetailPage() {
  const ctrl = useAgentDetailPageController();
  return <AgentDetailPageView ctrl={ctrl} />;
}
