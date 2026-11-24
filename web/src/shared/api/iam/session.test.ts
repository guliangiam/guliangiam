import { beforeEach, describe, expect, it, vi } from "vitest";
import { getSessionsPaginated } from "./session";

const mockRequestDocumentJson = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("@/shared/api/http", () => ({
  requestDocumentJson: (...args: unknown[]) => mockRequestDocumentJson(...args),
}));

const emptyPage = {
  items: [],
  page: { page: 1, pageSize: 20, total: 0 },
};

describe("getSessionsPaginated", () => {
  beforeEach(() => {
    mockRequestDocumentJson.mockReset();
    mockRequestDocumentJson.mockResolvedValue(emptyPage);
  });

  it("sends page, pageSize and activeOnly by default", async () => {
    await getSessionsPaginated();
    const url = String(mockRequestDocumentJson.mock.calls[0][0]);
    expect(url).toContain("page=1");
    expect(url).toContain("pageSize=50");
    expect(url).toContain("activeOnly=true");
  });

  // `sessions.application_id` backs the per-application sign-in log. The handler
  // read the parameter all along; the client simply never sent it.
  it("forwards applicationId when given", async () => {
    await getSessionsPaginated({ applicationId: "app-1" });
    const url = String(mockRequestDocumentJson.mock.calls[0][0]);
    expect(url).toContain("applicationId=app-1");
  });

  it("omits applicationId when absent", async () => {
    await getSessionsPaginated({ organizationId: "org-1" });
    const url = String(mockRequestDocumentJson.mock.calls[0][0]);
    expect(url).not.toContain("applicationId=");
    expect(url).toContain("organizationId=org-1");
  });

  it("maps a search term onto the keyword parameter the handler reads", async () => {
    await getSessionsPaginated({ search: "alice" });
    const url = String(mockRequestDocumentJson.mock.calls[0][0]);
    expect(url).toContain("keyword=alice");
    expect(url).not.toContain("search=");
  });

  it("can request the full history including inactive sessions", async () => {
    await getSessionsPaginated({ activeOnly: false });
    const url = String(mockRequestDocumentJson.mock.calls[0][0]);
    expect(url).toContain("activeOnly=false");
  });
});
