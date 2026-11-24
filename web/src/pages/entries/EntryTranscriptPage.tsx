import { EntryTranscriptPageCard } from "./EntryTranscriptPageCard";
import { useEntryTranscriptPageController } from "./useEntryTranscriptPageController";

export default function EntryTranscriptPage() {
  const controller = useEntryTranscriptPageController();

  return <EntryTranscriptPageCard {...controller} />;
}
