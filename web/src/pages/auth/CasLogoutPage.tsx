import { useCasLogoutPageController } from "./useCasLogoutPageController";
import { CasLogoutPageCard } from "./CasLogoutPageCard";

export default function CasLogoutPage() {
  const controller = useCasLogoutPageController();

  return <CasLogoutPageCard {...controller} />;
}
