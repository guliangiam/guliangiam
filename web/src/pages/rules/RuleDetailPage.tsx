import { useRuleDetailPageController } from "./useRuleDetailPageController";
import { RuleDetailPageView } from "./RuleDetailPageView";

export default function RuleDetailPage() {
  const ctrl = useRuleDetailPageController();
  return <RuleDetailPageView ctrl={ctrl} />;
}
