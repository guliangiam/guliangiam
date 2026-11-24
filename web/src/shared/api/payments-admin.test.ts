import { beforeEach, describe, expect, it, vi } from "vitest";

const mockRequestDocumentJson = vi.fn<(...args: unknown[]) => unknown>();
vi.mock("@/shared/api/http", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/shared/api/http")>();
  return {
    ...actual,
    requestDocumentJson: (...args: unknown[]) => mockRequestDocumentJson(...args),
  };
});

import { listAttemptsByPayment, listRefundsByPayment } from "./payments-admin";

describe("payments history pagination", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("sends page and pageSize for attempts", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      items: [],
      page: { page: 2, pageSize: 20, total: 201 },
    });

    const result = await listAttemptsByPayment("pay-1", { page: 2, pageSize: 20 });

    expect(mockRequestDocumentJson).toHaveBeenCalledWith(
      "/api/v1/payments/pay-1/attempts?page=2&pageSize=20",
    );
    expect(result?.page.total).toBe(201);
  });

  it("defaults attempts to the first served page", async () => {
    mockRequestDocumentJson.mockResolvedValue(undefined);

    await listAttemptsByPayment("pay-1");

    expect(mockRequestDocumentJson).toHaveBeenCalledWith(
      "/api/v1/payments/pay-1/attempts?page=1&pageSize=20",
    );
  });

  it("sends page and pageSize for refunds", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      items: [],
      page: { page: 3, pageSize: 100, total: 201 },
    });

    await listRefundsByPayment("pay-1", { page: 3, pageSize: 100 });

    expect(mockRequestDocumentJson).toHaveBeenCalledWith(
      "/api/v1/payments/pay-1/refunds?page=3&pageSize=100",
    );
  });
});
