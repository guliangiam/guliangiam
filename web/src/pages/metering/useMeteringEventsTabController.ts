import { useState } from "react";
import { useMeteringEvents } from "@/shared/hooks/use-metering";
import type { MeteringEventFilters } from "./metering-events-columns";

export function useMeteringEventsTabController() {
  const [filters, setFilters] = useState<MeteringEventFilters>({});

  const query = useMeteringEvents({
    eventType: filters.eventType,
    subject: filters.subject || undefined,
    from: filters.timeRange?.[0],
    to: filters.timeRange?.[1],
  });

  return { filters, setFilters, query };
}
