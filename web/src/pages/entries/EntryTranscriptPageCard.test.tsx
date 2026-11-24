import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import type { ReactNode } from "react";
import { EntryTranscriptPageCard } from "./EntryTranscriptPageCard";
import type { EntryTranscriptPageControllerState } from "./useEntryTranscriptPageController";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    // The card's only use of interpolation is the count it was handed, so the
    // mock keeps the arguments visible to the assertions instead of dropping them.
    t: (key: string, options?: Record<string, unknown>) =>
      options ? `${key}|${JSON.stringify(options)}` : key,
    i18n: { language: "en" },
  }),
}));

function message(index: number, overrides: Record<string, unknown> = {}) {
  return {
    index,
    kind: "log",
    timestamp: "2026-01-01T00:00:00Z",
    bodyText: `{"message":"record ${index}"}`,
    bodyBytes: 32,
    bodyOmitted: false,
    ...overrides,
  };
}

function readyState(
  overrides: Partial<Extract<EntryTranscriptPageControllerState, { status: "ready" }>> = {},
): Extract<EntryTranscriptPageControllerState, { status: "ready" }> {
  return {
    status: "ready",
    entryId: "entry-1",
    messages: [message(0)],
    redactedFields: [],
    hasMore: false,
    isLoadingMore: false,
    isRefreshing: false,
    loadMoreFailed: false,
    onLoadMore: vi.fn<() => void>(),
    onRefresh: vi.fn<() => void>(),
    ...overrides,
  };
}

function renderCard(state: EntryTranscriptPageControllerState) {
  const wrapper = ({ children }: { children: ReactNode }) => (
    <MemoryRouter>{children}</MemoryRouter>
  );
  return render(<EntryTranscriptPageCard {...state} />, { wrapper });
}

describe("EntryTranscriptPageCard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows a skeleton while the first page is in flight", () => {
    const { container } = renderCard({ status: "loading", entryId: "entry-1" });

    expect(screen.getByText("entry.transcriptTitle")).toBeInTheDocument();
    expect(container.querySelector(".ant-skeleton")).not.toBeNull();
    // The page must not render a message list it has no facts for.
    expect(container.querySelector(".ant-timeline")).toBeNull();
  });

  it("renders the served messages in append order with their index and kind", () => {
    renderCard(readyState({ messages: [message(0), message(1, { kind: "span" })] }));

    expect(screen.getByText("#0")).toBeInTheDocument();
    expect(screen.getByText("#1")).toBeInTheDocument();
    expect(screen.getByText("span")).toBeInTheDocument();
    expect(screen.getByText('{"message":"record 1"}')).toBeInTheDocument();
  });

  it("offers load-more only while the server reports a further page", () => {
    const onLoadMore = vi.fn<() => void>();
    const { unmount } = renderCard(readyState({ hasMore: true, onLoadMore }));

    screen.getByText("entry.transcriptLoadMore").click();
    expect(onLoadMore).toHaveBeenCalledTimes(1);
    unmount();

    renderCard(readyState({ hasMore: false }));
    expect(screen.queryByText("entry.transcriptLoadMore")).toBeNull();
    expect(screen.getByText("entry.transcriptEnd")).toBeInTheDocument();
  });

  it("keeps the loaded messages on screen when loading more fails", () => {
    renderCard(readyState({ hasMore: true, loadMoreFailed: true }));

    expect(screen.getByText("entry.transcriptLoadMoreFailed")).toBeInTheDocument();
    expect(screen.getByText("#0")).toBeInTheDocument();
  });

  it("reports how many sensitive values the server removed, and where", () => {
    renderCard(readyState({ redactedFields: ["headers.authorization", "attributes.token"] }));

    expect(screen.getByText('entry.transcriptRedactedNotice|{"count":2}')).toBeInTheDocument();
    expect(screen.getByText("headers.authorization, attributes.token")).toBeInTheDocument();
  });

  it("omits no-redaction notice when the server removed nothing", () => {
    renderCard(readyState({ redactedFields: [] }));

    expect(screen.queryByText(/entry.transcriptRedactedNotice/)).toBeNull();
  });

  it("serves an oversized record as a marker rather than as a body", () => {
    renderCard(
      readyState({
        messages: [message(0, { bodyText: "", bodyBytes: 70000, bodyOmitted: true })],
      }),
    );

    expect(screen.getByText('entry.transcriptBodyOmitted|{"bytes":70000}')).toBeInTheDocument();
    expect(screen.queryByText('{"message":"record 0"}')).toBeNull();
  });

  it("distinguishes an empty body from an omitted one", () => {
    renderCard(readyState({ messages: [message(0, { bodyText: "" })] }));

    expect(screen.getByText("entry.transcriptBodyEmpty")).toBeInTheDocument();
    expect(screen.queryByText(/entry.transcriptBodyOmitted/)).toBeNull();
  });

  it("renders one not-found state for both an absent entry and a foreign one", () => {
    renderCard({ status: "notFound", entryId: "entry-1" });

    expect(screen.getByText("entry.transcriptNotFound")).toBeInTheDocument();
    // A distinct "forbidden" wording would rebuild the existence oracle that the
    // server's 404 removes (43.5.2).
    expect(screen.queryByText(/forbidden/i)).toBeNull();
    expect(screen.getByText("entry.transcriptBackToList")).toBeInTheDocument();
  });

  it("renders an error state whose retry is the controller's", () => {
    const onRetry = vi.fn<() => void>();
    renderCard({ status: "error", entryId: "entry-1", errorMessage: "network down", onRetry });

    expect(screen.getByText("network down")).toBeInTheDocument();
    screen.getByText("entry.transcriptRetry").click();
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("shows an empty-sequence state instead of a bare timeline", () => {
    const { container } = renderCard(readyState({ messages: [] }));

    expect(screen.getByText("entry.transcriptEmpty")).toBeInTheDocument();
    expect(container.querySelector(".ant-timeline")).toBeNull();
  });

  it("exposes no control that writes to the sequence", () => {
    renderCard(readyState());

    for (const label of ["Delete", "Edit", "Save"]) {
      expect(screen.queryByText(label)).toBeNull();
    }
  });
});
