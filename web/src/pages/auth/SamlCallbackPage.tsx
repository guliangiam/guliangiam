import { useSamlCallbackPageController } from "./useSamlCallbackPageController";
import { SamlCallbackPageCard } from "./SamlCallbackPageCard";

export default function SamlCallbackPage() {
  const controller = useSamlCallbackPageController();

  return <SamlCallbackPageCard {...controller} />;
}
