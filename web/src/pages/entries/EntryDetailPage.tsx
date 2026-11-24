import { useEntryDetailPageController } from "./useEntryDetailPageController";
import { EntryDetailPageView } from "./EntryDetailPageView";

export default function EntryDetailPage() {
  const ctrl = useEntryDetailPageController();
  return <EntryDetailPageView ctrl={ctrl} />;
}
