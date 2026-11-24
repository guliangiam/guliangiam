import { App } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { queryKeys } from "@/shared/api/query-keys";
import {
  ingestMeteringEvents,
  listMeteringAggregates,
  listMeteringEvents,
  refreshMeteringAggregate,
  type IngestMeteringEventsRequest,
  type ListMeteringAggregatesParams,
  type ListMeteringEventsParams,
} from "@/shared/api/iam/metering";
import { localizedError } from "@/shared/lib/server-error";

// ─────────────────────────────────────────────
// Metering domain hooks — aligned with backend
// EventDTO / AggregateDTO + chi.go filter params
// ─────────────────────────────────────────────

// Events are append-heavy; aggregates are pre-rolled by the backend.
export const EVENT_STALE_MS = 10_000;
export const AGGREGATE_STALE_MS = 60_000;

export function useMeteringEvents(params?: ListMeteringEventsParams) {
  return useQuery({
    queryKey: queryKeys.metering.events(params ?? {}),
    queryFn: () => listMeteringEvents(params),
    staleTime: EVENT_STALE_MS,
    refetchInterval: EVENT_STALE_MS,
  });
}

export function useMeteringAggregates(params?: ListMeteringAggregatesParams) {
  return useQuery({
    queryKey: queryKeys.metering.aggregates(params ?? {}),
    queryFn: () => listMeteringAggregates(params),
    staleTime: AGGREGATE_STALE_MS,
  });
}

export function useIngestMeteringEvents() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: IngestMeteringEventsRequest) => ingestMeteringEvents(input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.metering.all });
    },
  });
}

/**
 * Manual single-cell re-roll — calls POST /metering/aggregates/refresh.
 * Pass { period, metric, subject? } — same as backend RefreshAggregateCommand.
 */
export function useRefreshMeteringAggregate() {
  const qc = useQueryClient();
  const { message } = App.useApp();
  const { t } = useTranslation("common");
  return useMutation({
    mutationFn: refreshMeteringAggregate,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.metering.aggregates({}) });
    },
    onError: (err) => message.error(localizedError(err, t, "common:error")),
  });
}
