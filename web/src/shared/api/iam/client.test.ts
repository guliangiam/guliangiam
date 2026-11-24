import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  buildQueryString,
  iamList,
  iamListWithPagination,
  iamGet,
  iamCreate,
  iamPost,
  iamPatch,
  iamUpdate,
  iamDelete,
  iamPostRaw,
} from "./client";

// Mock the HTTP module
const mockRequestDocumentJson = vi.fn<(...args: unknown[]) => unknown>();
vi.mock("@/shared/api/http", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/shared/api/http")>();
  return {
    ...actual,
    requestDocumentJson: (...args: unknown[]) => mockRequestDocumentJson(...args),
  };
});

describe("buildQueryString", () => {
  it("returns default pagination when no params", () => {
    const qs = buildQueryString();
    expect(qs).toContain("page=1");
    expect(qs).toContain("pageSize=50");
  });

  it("includes page and pageSize when provided", () => {
    const qs = buildQueryString({ page: 2, pageSize: 20 });
    expect(qs).toContain("page=2");
    expect(qs).toContain("pageSize=20");
  });

  // The wire name is `keyword`: every list handler reads
  // `r.URL.Query().Get("keyword")`. Sending `search` made the search box a
  // silent no-op, which is what `scripts/check-api-params.py` now prevents.
  it("sends the free-text filter as keyword", () => {
    const qs = buildQueryString({ search: "test query" });
    expect(qs).toContain("keyword=test%20query");
    expect(qs).not.toContain("search=");
  });

  // The wire name is `sortField` (the handlers read `sortField`).
  it("sends the sort column as sortField and keeps sortOrder", () => {
    const qs = buildQueryString({ sortBy: "name", sortOrder: "asc" });
    expect(qs).toContain("sortField=name");
    expect(qs).not.toContain("sortBy=");
    expect(qs).toContain("sortOrder=asc");
  });

  it("includes organizationId when provided", () => {
    const qs = buildQueryString({ organizationId: "org-123" });
    expect(qs).toContain("organizationId=org-123");
  });

  it("includes owner when provided", () => {
    const qs = buildQueryString({ owner: "admin" });
    expect(qs).toContain("owner=admin");
  });

  it("includes status when provided", () => {
    const qs = buildQueryString({ status: "active" });
    expect(qs).toContain("status=active");
  });

  it("combines multiple params correctly", () => {
    const qs = buildQueryString({
      page: 3,
      pageSize: 10,
      search: "hello",
      organizationId: "org-1",
    });
    expect(qs).toContain("page=3");
    expect(qs).toContain("pageSize=10");
    expect(qs).toContain("keyword=hello");
    expect(qs).toContain("organizationId=org-1");
  });

  it("encodes special characters in the keyword", () => {
    const qs = buildQueryString({ search: "hello&world" });
    expect(qs).toContain("keyword=hello%26world");
  });

  it("encodes special characters in the sort field", () => {
    const qs = buildQueryString({ sortBy: "field name" });
    expect(qs).toContain("sortField=field%20name");
  });

  it("does not include optional params when undefined", () => {
    const qs = buildQueryString({ page: 1 });
    expect(qs).not.toContain("keyword=");
    expect(qs).not.toContain("sortField=");
    expect(qs).not.toContain("sortOrder=");
    expect(qs).not.toContain("organizationId=");
    expect(qs).not.toContain("owner=");
    expect(qs).not.toContain("status=");
  });
});

describe("iamList", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls requestDocumentJson with correct URL and query string", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      items: [{ id: "1", name: "test" }],
      page: { page: 1, pageSize: 50, total: 1 },
    });

    await iamList("/api/v1/users");

    expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/users?page=1&pageSize=50");
  });

  it("appends query string with separator when path already has query params", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      items: [],
      page: { page: 1, pageSize: 50, total: 0 },
    });

    await iamList("/api/v1/users?org=org-1");

    expect(mockRequestDocumentJson).toHaveBeenCalledWith(
      "/api/v1/users?org=org-1&page=1&pageSize=50",
    );
  });

  it("returns items from the response", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      items: [
        { id: "1", name: "Alice" },
        { id: "2", name: "Bob" },
      ],
      page: { page: 1, pageSize: 50, total: 2 },
    });

    const result = await iamList<{ id: string; name: string }>("/api/v1/users");
    expect(result).toHaveLength(2);
    expect(result[0].name).toBe("Alice");
  });

  it("returns empty array when items is empty", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      items: [],
      page: { page: 1, pageSize: 50, total: 0 },
    });

    const result = await iamList("/api/v1/users");
    expect(result).toEqual([]);
  });

  it("validates items with itemSchema when provided", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      items: [{ id: "1", name: "Alice" }],
      page: { page: 1, pageSize: 50, total: 1 },
    });

    const { z } = await import("zod");
    const userSchema = z.object({ id: z.string(), name: z.string() });

    const result = await iamList("/api/v1/users", undefined, userSchema);
    expect(result).toHaveLength(1);
    expect(result[0].name).toBe("Alice");
  });

  it("throws on invalid response shape (missing items)", async () => {
    mockRequestDocumentJson.mockResolvedValue({ notItems: true });

    await expect(iamList("/api/v1/users")).rejects.toThrow(/items/i);
  });
});

describe("iamListWithPagination", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns items with pagination info", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      items: [{ id: "1", name: "Alice" }],
      page: { page: 2, pageSize: 10, total: 50 },
    });

    const result = await iamListWithPagination<{ id: string; name: string }>("/api/v1/users");
    expect(result.items).toHaveLength(1);
    expect(result.page).toBe(2);
    expect(result.pageSize).toBe(10);
    expect(result.total).toBe(50);
  });

  it("uses default pagination when page info is missing", async () => {
    mockRequestDocumentJson.mockResolvedValue({
      items: [],
    });

    const result = await iamListWithPagination("/api/v1/users");
    expect(result.page).toBe(1);
    expect(result.pageSize).toBe(50);
    expect(result.total).toBe(0);
  });
});

describe("iamGet", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls requestDocumentJson with the path and no-store cache", async () => {
    mockRequestDocumentJson.mockResolvedValue({ id: "1", name: "test" });

    await iamGet("/api/v1/users/1");

    // FE-B-03：详情 GET 必须绕过浏览器 HTTP 缓存（后端 ETag max-age=30），
    // 否则保存后重取命中旧体、revision 停留旧值导致二次保存 409。
    expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/users/1", {
      cache: "no-store",
    });
  });

  it("returns typed response", async () => {
    mockRequestDocumentJson.mockResolvedValue({ id: "1", name: "Alice" });

    const result = await iamGet<{ id: string; name: string }>("/api/v1/users/1");
    expect(result.id).toBe("1");
    expect(result.name).toBe("Alice");
  });

  it("validates response with schema when provided", async () => {
    mockRequestDocumentJson.mockResolvedValue({ id: "1", name: "Alice" });

    const { z } = await import("zod");
    const schema = z.object({ id: z.string(), name: z.string() });

    const result = await iamGet("/api/v1/users/1", schema);
    expect(result.id).toBe("1");
  });
});

describe("iamCreate", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("sends POST request with JSON body", async () => {
    mockRequestDocumentJson.mockResolvedValue({ id: "1" });

    await iamCreate("/api/v1/users", { name: "Alice" });

    expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/users", {
      method: "POST",
      body: JSON.stringify({ name: "Alice" }),
      headers: { "Content-Type": "application/json" },
    });
  });
});

describe("iamPost", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("sends POST request and returns typed response", async () => {
    mockRequestDocumentJson.mockResolvedValue({ id: "1", name: "Alice" });

    const result = await iamPost<{ id: string }>("/api/v1/users", { name: "Alice" });

    expect(result.id).toBe("1");
    expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/users", {
      method: "POST",
      body: JSON.stringify({ name: "Alice" }),
      headers: { "Content-Type": "application/json" },
    });
  });

  it("validates response with schema when provided", async () => {
    mockRequestDocumentJson.mockResolvedValue({ id: "1", name: "Alice" });

    const { z } = await import("zod");
    const schema = z.object({ id: z.string(), name: z.string() });

    const result = await iamPost("/api/v1/users", { name: "Alice" }, schema);
    expect(result.name).toBe("Alice");
  });
});

describe("iamPatch", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("sends PATCH request with JSON body", async () => {
    mockRequestDocumentJson.mockResolvedValue({ id: "1" });

    await iamPatch("/api/v1/users/1", { name: "Updated" });

    expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/users/1", {
      method: "PATCH",
      body: JSON.stringify({ name: "Updated" }),
      headers: { "Content-Type": "application/json" },
    });
  });
});

describe("iamUpdate", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("sends PUT request with JSON body", async () => {
    mockRequestDocumentJson.mockResolvedValue({ id: "1" });

    await iamUpdate("/api/v1/users/1", { name: "Updated" });

    expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/users/1", {
      method: "PUT",
      body: JSON.stringify({ name: "Updated" }),
      headers: { "Content-Type": "application/json" },
    });
  });
});

describe("iamDelete", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("sends DELETE request", async () => {
    mockRequestDocumentJson.mockResolvedValue(undefined);

    await iamDelete("/api/v1/users/1");

    expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/users/1", {
      method: "DELETE",
    });
  });
});

describe("iamPostRaw", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("sends POST request without body", async () => {
    mockRequestDocumentJson.mockResolvedValue(undefined);

    await iamPostRaw("/api/v1/action");

    expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/action", {
      method: "POST",
    });
  });

  it("sends POST request with JSON body when body is provided", async () => {
    mockRequestDocumentJson.mockResolvedValue(undefined);

    await iamPostRaw("/api/v1/action", { key: "value" });

    expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/action", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key: "value" }),
    });
  });
});

describe("request interceptor and error handling", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("propagates errors from requestDocumentJson", async () => {
    mockRequestDocumentJson.mockRejectedValue(new Error("Network error"));

    await expect(iamGet("/api/v1/users/1")).rejects.toThrow("Network error");
  });

  it("propagates HTTP errors from requestDocumentJson", async () => {
    const { HttpError } = await import("@/shared/api/http");
    const error = new HttpError(401, null, "Unauthorized");
    mockRequestDocumentJson.mockRejectedValue(error);

    await expect(iamGet("/api/v1/users/1")).rejects.toThrow("Unauthorized");
  });

  it("propagates 403 errors from requestDocumentJson", async () => {
    const { HttpError } = await import("@/shared/api/http");
    const error = new HttpError(403, null, "Forbidden");
    mockRequestDocumentJson.mockRejectedValue(error);

    await expect(iamGet("/api/v1/users/1")).rejects.toThrow("Forbidden");
  });

  it("propagates 404 errors from requestDocumentJson", async () => {
    const { HttpError } = await import("@/shared/api/http");
    const error = new HttpError(404, null, "Not Found");
    mockRequestDocumentJson.mockRejectedValue(error);

    await expect(iamGet("/api/v1/users/1")).rejects.toThrow("Not Found");
  });
});
