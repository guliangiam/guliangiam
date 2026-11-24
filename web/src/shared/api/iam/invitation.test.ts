import { beforeEach, describe, expect, it, vi } from "vitest";
import { getInvitations, getInvitationsPaginated } from "./invitation";

const mockRequestDocumentJson = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("@/shared/api/http", () => ({
  requestDocumentJson: (...args: unknown[]) => mockRequestDocumentJson(...args),
}));

const emptyPage = {
  items: [],
  page: { page: 1, pageSize: 50, total: 0 },
};

function lastUrl(): string {
  return String(mockRequestDocumentJson.mock.calls.at(-1)?.[0] ?? "");
}

describe("getInvitationsPaginated", () => {
  beforeEach(() => {
    mockRequestDocumentJson.mockReset();
    mockRequestDocumentJson.mockResolvedValue(emptyPage);
  });

  it("sends pagination, keyword, organization and status the server honours", async () => {
    await getInvitationsPaginated({
      page: 2,
      pageSize: 20,
      search: "alice",
      organizationId: "org-1",
      status: "pending",
    });
    const url = lastUrl();
    expect(url).toContain("page=2");
    expect(url).toContain("pageSize=20");
    expect(url).toContain("keyword=alice");
    expect(url).toContain("organizationId=org-1");
    expect(url).toContain("status=pending");
  });

  it("skips empty filters rather than sending blank parameters", async () => {
    await getInvitationsPaginated({ search: "", organizationId: "", status: "" });
    const url = lastUrl();
    expect(url).not.toContain("keyword=");
    expect(url).not.toContain("organizationId=");
    expect(url).not.toContain("status=");
  });

  it("unwraps the page envelope for the crud list shell", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      items: [],
      page: { page: 3, pageSize: 20, total: 57 },
    });
    const result = await getInvitationsPaginated({ page: 3, pageSize: 20 });
    expect(result).toEqual({ items: [], page: 3, pageSize: 20, total: 57 });
  });
});

describe("getInvitations", () => {
  beforeEach(() => {
    mockRequestDocumentJson.mockReset();
    mockRequestDocumentJson.mockResolvedValue(emptyPage);
  });

  it("supports the keyword filter used by detail-page targeted resolution", async () => {
    await getInvitations("org-1", undefined, "alice@example.com");
    const url = lastUrl();
    expect(url).toContain("organizationId=org-1");
    expect(url).toContain("keyword=alice%40example.com");
    expect(url).not.toContain("status=");
  });
});
