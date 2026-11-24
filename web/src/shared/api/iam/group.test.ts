import { beforeEach, describe, expect, it, vi } from "vitest";
import { getGroupsPaginated } from "./group";

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

describe("getGroupsPaginated", () => {
  beforeEach(() => {
    mockRequestDocumentJson.mockReset();
    mockRequestDocumentJson.mockResolvedValue(emptyPage);
  });

  // `GroupListParams` declared `sortBy`/`sortOrder` but the function never sent
  // them, so a column sort could not reach the server.
  it("sends the sort the params declare", async () => {
    await getGroupsPaginated({ sortBy: "createdAt", sortOrder: "desc" });
    const url = lastUrl();
    expect(url).toContain("sortField=createdAt");
    expect(url).toContain("sortOrder=desc");
  });

  // 后端列表只识别 status=active/inactive；isEnabled 等参数服务端不读取，
  // 发送它们只会让筛选静默失效。
  it("sends the status filter the server honours", async () => {
    await getGroupsPaginated({ status: "inactive" });
    const url = lastUrl();
    expect(url).toContain("status=inactive");
    expect(url).not.toContain("isEnabled=");
    expect(url).not.toContain("type=");
  });

  // G7-s：type 闭集过滤已全链接通，列表必须把筛选值送到服务端。
  it("sends the type filter the server honours", async () => {
    await getGroupsPaginated({ type: "department" });
    expect(lastUrl()).toContain("type=department");
  });

  it("skips an empty type rather than sending a blank parameter", async () => {
    await getGroupsPaginated({ type: "" });
    expect(lastUrl()).not.toContain("type=");
  });

  it("skips an empty status rather than sending a blank parameter", async () => {
    await getGroupsPaginated({ status: "" });
    expect(lastUrl()).not.toContain("status=");
  });

  it("keeps the pre-existing search and organization filters", async () => {
    await getGroupsPaginated({ search: "eng", organizationId: "org-1", status: "active" });
    const url = lastUrl();
    expect(url).toContain("keyword=eng");
    expect(url).toContain("organizationId=org-1");
    expect(url).toContain("status=active");
  });
});
