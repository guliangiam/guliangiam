import { beforeEach, describe, expect, it, vi } from "vitest";
import { listDomains } from "./b2b";

const mockRequestDocumentJson = vi.fn<(...args: unknown[]) => unknown>();
vi.mock("@/shared/api/http", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/shared/api/http")>();
  return {
    ...actual,
    requestDocumentJson: (...args: unknown[]) => mockRequestDocumentJson(...args),
  };
});

const domain = {
  id: "domain-1",
  organizationId: "org-1",
  domain: "corp.example.com",
  verificationToken: "token-1",
  status: "pending",
  createdAt: "2026-10-27T08:00:00Z",
  updatedAt: "2026-10-27T08:00:00Z",
};

describe("B2B domain API", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("uses the backend pageSize contract and parses page metadata", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      items: [domain],
      page: { page: 2, pageSize: 50, total: 51 },
    });

    const result = await listDomains({
      organizationId: "org-1",
      page: 2,
      pageSize: 50,
      status: "pending",
    });

    expect(mockRequestDocumentJson).toHaveBeenCalledWith(
      "/api/v1/organizations/org-1/domains?page=2&pageSize=50&status=pending",
      { method: "GET" },
    );
    expect(result.page).toEqual({ page: 2, pageSize: 50, total: 51 });
    expect(result.items[0]).toEqual(domain);
  });
});
