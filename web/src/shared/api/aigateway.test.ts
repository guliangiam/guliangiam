import { beforeEach, describe, expect, it, vi } from "vitest";

const mockRequestDocumentJson = vi.fn<(...args: unknown[]) => unknown>();
vi.mock("@/shared/api/http", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/shared/api/http")>();
  return {
    ...actual,
    requestDocumentJson: (...args: unknown[]) => mockRequestDocumentJson(...args),
  };
});

import { collectAllPages, listProviders, listUsageLogs } from "./aigateway";

describe("aigateway admin list pagination", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("sends the backend default page and pageSize when omitted", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      items: [],
      page: { page: 1, pageSize: 20, total: 0 },
    });

    await listProviders();

    expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/ai/providers?page=1&pageSize=20");
  });

  it("changes the request URL when the page changes", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      items: [],
      page: { page: 2, pageSize: 20, total: 41 },
    });

    const result = await listProviders({ page: 2, pageSize: 20 });

    expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/ai/providers?page=2&pageSize=20");
    expect(result?.page.total).toBe(41);
  });

  it("pages usage logs instead of requesting the whole set", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      items: [],
      page: { page: 1, pageSize: 20, total: 0 },
    });

    await listUsageLogs({ page: 1, pageSize: 20 });

    expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/ai/usage?page=1&pageSize=20");
  });
});

describe("collectAllPages", () => {
  it("walks the reported total for callers without a pagination control", async () => {
    const fetchPage = vi.fn<
      (params: { page?: number }) => Promise<{
        items: number[];
        page: { page: number; pageSize: number; total: number };
      }>
    >(async ({ page }) => ({
      items: page === 2 ? [3] : [1, 2],
      page: { page: page ?? 1, pageSize: 20, total: 3 },
    }));

    const items = await collectAllPages(fetchPage);

    expect(items).toEqual([1, 2, 3]);
    expect(fetchPage).toHaveBeenCalledTimes(2);
  });
});
