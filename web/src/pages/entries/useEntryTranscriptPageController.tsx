import { useCallback, useMemo } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { isHttpError } from "@/shared/api/http";
import { getIamEntryTranscript, type IamEntryTranscriptMessage } from "@/shared/api/iam/entry";
import { queryKeys } from "@/shared/api/query-keys";
import { localizedError } from "@/shared/lib/server-error";

/**
 * Page size for one transcript read. It is the console's own read-cost choice,
 * not a contract: the server clamps anything above its own maximum and refuses a
 * malformed value rather than silently substituting one.
 */
export const TRANSCRIPT_PAGE_SIZE = 50;

export type EntryTranscriptMessageView = {
  index: number;
  kind: string;
  scope?: string;
  name?: string;
  timestamp: string;
  /** Pretty-printed body. Already redacted by the server; never re-processed here. */
  bodyText: string;
  bodyBytes: number;
  bodyOmitted: boolean;
};

export type EntryTranscriptReadyState = {
  status: "ready";
  entryId: string;
  messages: EntryTranscriptMessageView[];
  /** Field paths the server removed, so "absent" is distinguishable from "redacted". */
  redactedFields: string[];
  hasMore: boolean;
  isLoadingMore: boolean;
  isRefreshing: boolean;
  /** A "load more" attempt failed. The messages already read stay on screen. */
  loadMoreFailed: boolean;
  onLoadMore: () => void;
  onRefresh: () => void;
};

export type EntryTranscriptNotFoundState = {
  status: "notFound";
  entryId: string;
};

export type EntryTranscriptErrorState = {
  status: "error";
  entryId: string;
  errorMessage: string;
  onRetry: () => void;
};

export type EntryTranscriptLoadingState = {
  status: "loading";
  entryId: string;
};

export type EntryTranscriptPageControllerState =
  | EntryTranscriptReadyState
  | EntryTranscriptNotFoundState
  | EntryTranscriptErrorState
  | EntryTranscriptLoadingState;

function toView(message: IamEntryTranscriptMessage): EntryTranscriptMessageView {
  return {
    index: message.index,
    kind: message.kind,
    scope: message.scope,
    name: message.name,
    timestamp: message.timestamp,
    bodyText: message.body === undefined ? "" : JSON.stringify(message.body, null, 2),
    bodyBytes: message.bodyBytes,
    bodyOmitted: message.bodyOmitted === true,
  };
}

/**
 * Controller for the entry transcript viewer (设计报告 28.10.5 FE-015).
 *
 * The viewer walks one entry's message sequence in append order, using the opaque
 * cursor the server hands back. It is read-only by design: there is no edit, no
 * delete and no "view raw" path, and the console does not redact — the server
 * does, before the record leaves it (42.3.13 条 2、4、6).
 *
 * An absent entry and one owned by another tenant are both answered 404, and the
 * page renders a single "not found" state for both. A distinct "exists but
 * forbidden" message would rebuild the existence oracle the 404 exists to prevent
 * (43.5.2).
 */
export function useEntryTranscriptPageController(): EntryTranscriptPageControllerState {
  const { t } = useTranslation("resource");
  const { id = "" } = useParams<{ id: string }>();

  const query = useInfiniteQuery({
    queryKey: queryKeys.entryTranscript.page(id),
    queryFn: ({ pageParam }) =>
      getIamEntryTranscript(id, { cursor: pageParam, limit: TRANSCRIPT_PAGE_SIZE }),
    initialPageParam: undefined as string | undefined,
    // An empty cursor means the sequence ended. Handing it back would be a
    // malformed cursor, which the server refuses instead of serving page one again.
    getNextPageParam: (lastPage) => lastPage.nextCursor || undefined,
    enabled: id.length > 0,
  });

  const messages = useMemo(
    () => (query.data?.pages ?? []).flatMap((page) => page.messages.map(toView)),
    [query.data],
  );

  // Redaction is reported per response, but the notice describes the whole
  // sequence: a reader who pages past a credential must still be told that
  // something was removed from the transcript they are reading.
  const redactedFields = useMemo(() => {
    const fields = new Set<string>();
    for (const page of query.data?.pages ?? []) {
      for (const field of page.redactedFields ?? []) {
        fields.add(field);
      }
    }
    return [...fields].sort();
  }, [query.data]);

  const onLoadMore = useCallback(() => {
    void query.fetchNextPage();
  }, [query]);

  const onRefresh = useCallback(() => {
    void query.refetch();
  }, [query]);

  const onRetry = useCallback(() => {
    void query.refetch();
  }, [query]);

  const pageCount = query.data?.pages.length ?? 0;

  if (id.length === 0 || (isHttpError(query.error) && query.error.status === 404)) {
    return { status: "notFound", entryId: id };
  }

  if (query.isPending) {
    return { status: "loading", entryId: id };
  }

  // A failure after the first page is a failure of the last "load more", not of
  // the page: discarding the messages the operator already read would hide the
  // part of the sequence that did load.
  if (query.isError && pageCount === 0) {
    return {
      status: "error",
      entryId: id,
      errorMessage: localizedError(query.error, t, "entry.transcriptLoadFailed"),
      onRetry,
    };
  }

  return {
    status: "ready",
    entryId: id,
    messages,
    redactedFields,
    hasMore: query.hasNextPage,
    isLoadingMore: query.isFetchingNextPage,
    isRefreshing: query.isRefetching && !query.isFetchingNextPage,
    loadMoreFailed: query.isFetchNextPageError,
    onLoadMore,
    onRefresh,
  };
}
