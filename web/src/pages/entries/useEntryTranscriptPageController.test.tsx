import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { createTestQueryClient } from "@/test-utils/render";
import { HttpError } from "@/shared/api/http";
import {
  useEntryTranscriptPageController,
  TRANSCRIPT_PAGE_SIZE,
  type EntryTranscriptReadyState,
} from "./useEntryTranscriptPageController";

const transcriptMock =
  vi.fn<(id: string, params?: { cursor?: string; limit?: number }) => Promise<unknown>>();

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key, i18n: { language: "en" } }),
}));

vi.mock("@/shared/api/iam/entry", () => ({
  getIamEntryTranscript: (id: string, params?: { cursor?: string; limit?: number }) =>
    transcriptMock(id, params),
}));

function message(index: number, overrides: Record<string, unknown> = {}) {
  return {
    index,
    kind: "log",
    timestamp: `2026-01-01T00:00:0${index}Z`,
    body: { message: `record ${index}` },
    bodyBytes: 32,
    ...overrides,
  };
}

function renderController(entryId = "entry-1") {
  const queryClient = createTestQueryClient();
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[`/entries/${entryId}/transcript`]}>
        <Routes>
          <Route path="/entries/:id/transcript" element={<>{children}</>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );

  return renderHook(() => useEntryTranscriptPageController(), { wrapper });
}

function ready(
  state: ReturnType<typeof useEntryTranscriptPageController>,
): EntryTranscriptReadyState {
  if (state.status !== "ready") throw new Error(`expected ready state, got ${state.status}`);
  return state;
}

describe("useEntryTranscriptPageController", () => {
  beforeEach(() => {
    transcriptMock.mockReset();
  });

  it("reads the first page without a cursor", async () => {
    transcriptMock.mockResolvedValue({ messages: [message(0)], nextCursor: "" });

    const { result } = renderController();

    await waitFor(() => {
      expect(result.current.status).toBe("ready");
    });

    expect(transcriptMock).toHaveBeenCalledWith("entry-1", {
      cursor: undefined,
      limit: TRANSCRIPT_PAGE_SIZE,
    });
    expect(ready(result.current).messages).toHaveLength(1);
    expect(ready(result.current).hasMore).toBe(false);
  });

  it("hands the server's cursor back verbatim and appends the next page", async () => {
    transcriptMock
      .mockResolvedValueOnce({ messages: [message(0)], nextCursor: "page-2" })
      .mockResolvedValueOnce({ messages: [message(1)], nextCursor: "" });

    const { result } = renderController();

    await waitFor(() => {
      expect(result.current.status).toBe("ready");
    });
    expect(ready(result.current).hasMore).toBe(true);

    ready(result.current).onLoadMore();

    await waitFor(() => {
      expect(ready(result.current).messages).toHaveLength(2);
    });

    // The console must never construct a cursor of its own (42.3.13 条 1).
    expect(transcriptMock).toHaveBeenLastCalledWith("entry-1", {
      cursor: "page-2",
      limit: TRANSCRIPT_PAGE_SIZE,
    });
    expect(ready(result.current).messages.map((m) => m.index)).toEqual([0, 1]);
    expect(ready(result.current).hasMore).toBe(false);
  });

  it("does not treat an empty cursor as a page to fetch", async () => {
    transcriptMock.mockResolvedValue({ messages: [message(0)], nextCursor: "" });

    const { result } = renderController();

    await waitFor(() => {
      expect(result.current.status).toBe("ready");
    });

    ready(result.current).onLoadMore();

    await waitFor(() => {
      expect(transcriptMock).toHaveBeenCalledTimes(1);
    });
  });

  it("reports an absent entry and a foreign entry with a single not-found state", async () => {
    transcriptMock.mockRejectedValue(new HttpError(404, {}, "not found"));

    const { result } = renderController();

    await waitFor(() => {
      expect(result.current.status).toBe("notFound");
    });

    expect(result.current.entryId).toBe("entry-1");
  });

  it("does not call the read face when the route carries no entry id", async () => {
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={createTestQueryClient()}>
        <MemoryRouter initialEntries={["/entries/transcript"]}>{children}</MemoryRouter>
      </QueryClientProvider>
    );

    const { result } = renderHook(() => useEntryTranscriptPageController(), { wrapper });

    await waitFor(() => {
      expect(result.current.status).toBe("notFound");
    });

    expect(result.current.entryId).toBe("");
    expect(transcriptMock).not.toHaveBeenCalled();
  });

  it("surfaces a failed first read as an error state that a retry can clear", async () => {
    transcriptMock.mockRejectedValueOnce(new Error("network down"));

    const { result } = renderController();

    await waitFor(() => {
      expect(result.current.status).toBe("error");
    });
    if (result.current.status !== "error") throw new Error("expected error state");
    expect(result.current.errorMessage).toBe("network down");

    transcriptMock.mockResolvedValueOnce({ messages: [message(0)], nextCursor: "" });
    result.current.onRetry();

    await waitFor(() => {
      expect(result.current.status).toBe("ready");
    });
    expect(ready(result.current).messages).toHaveLength(1);
  });

  it("keeps the messages already read when a later page fails", async () => {
    transcriptMock
      .mockResolvedValueOnce({ messages: [message(0)], nextCursor: "page-2" })
      .mockRejectedValueOnce(new Error("network down"));

    const { result } = renderController();

    await waitFor(() => {
      expect(result.current.status).toBe("ready");
    });

    ready(result.current).onLoadMore();

    await waitFor(() => {
      expect(ready(result.current).loadMoreFailed).toBe(true);
    });

    // A "load more" failure is not a page failure: hiding the part of the
    // sequence that did load would lose evidence the operator already had.
    expect(ready(result.current).messages).toHaveLength(1);
    expect(ready(result.current).hasMore).toBe(true);
  });

  it("unions the redacted field paths reported across pages", async () => {
    transcriptMock
      .mockResolvedValueOnce({
        messages: [message(0)],
        nextCursor: "page-2",
        redactedFields: ["attributes.token"],
      })
      .mockResolvedValueOnce({
        messages: [message(1)],
        nextCursor: "",
        redactedFields: ["headers.authorization", "attributes.token"],
      });

    const { result } = renderController();

    await waitFor(() => {
      expect(result.current.status).toBe("ready");
    });
    ready(result.current).onLoadMore();

    await waitFor(() => {
      expect(ready(result.current).redactedFields).toHaveLength(2);
    });

    expect(ready(result.current).redactedFields).toEqual([
      "attributes.token",
      "headers.authorization",
    ]);
  });

  it("marks an oversized record as omitted instead of rendering its body", async () => {
    transcriptMock.mockResolvedValue({
      messages: [message(0, { body: undefined, bodyBytes: 70000, bodyOmitted: true })],
      nextCursor: "",
    });

    const { result } = renderController();

    await waitFor(() => {
      expect(result.current.status).toBe("ready");
    });

    const [record] = ready(result.current).messages;
    expect(record.bodyOmitted).toBe(true);
    expect(record.bodyText).toBe("");
    expect(record.bodyBytes).toBe(70000);
  });

  it("renders the served body verbatim and never re-processes it", async () => {
    transcriptMock.mockResolvedValue({
      messages: [message(0, { body: { password: "***" } })],
      nextCursor: "",
    });

    const { result } = renderController();

    await waitFor(() => {
      expect(result.current.status).toBe("ready");
    });

    // Redaction is the server's job (42.3.13 条 2); the console only formats.
    expect(ready(result.current).messages[0].bodyText).toBe(
      JSON.stringify({ password: "***" }, null, 2),
    );
  });
});
