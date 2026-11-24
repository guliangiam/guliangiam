import { beforeEach, describe, expect, it, vi } from "vitest";
import { HttpError } from "@/shared/api/http";
import { getUser, getUserByOrganizationSlugAndUsername } from "./user";

const mockRequestDocumentJson = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("@/shared/api/http", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/shared/api/http")>();
  return {
    ...actual,
    requestDocumentJson: (...args: unknown[]) => mockRequestDocumentJson(...args),
  };
});

// user.ts 内部的 getOrganizationIdentityBySlug 直接请求 by-slug 端点，
// 因此用 URL 驱动的 http mock 覆盖三个环节：slug 解析、用户定位、详情读取。
const minimalListItem = {
  id: "user-bob",
  organizationId: "org-other",
  username: "bob",
  displayName: "Bob",
  avatarUrl: "",
  email: "bob@example.com",
  phone: "",
  userType: "normal-user",
  status: "active",
  isAdmin: false,
  isForbidden: false,
  createdAt: "2026-01-01T00:00:00Z",
  revision: 1,
};

const minimalUserDetail = {
  ...minimalListItem,
  externalId: "",
  preferredMfaType: "",
  mfaSetupSummary: [],
  profile: {},
  credentialSummary: {},
  identities: [],
  mfaMethods: [],
  webauthnCredentialCount: 0,
  roleBindings: [],
  groupMemberships: [],
  consentSummary: {},
};

describe("admin cross-tenant user detail access", () => {
  beforeEach(() => {
    mockRequestDocumentJson.mockReset();
  });

  it("resolves slug → org id, locates the user by keyword and carries the target organization on the detail GET", async () => {
    mockRequestDocumentJson.mockImplementation((url: unknown) => {
      const path = String(url);
      if (path.startsWith("/api/v1/organizations/by-slug/")) {
        return Promise.resolve({ id: "org-other", slug: "other-org" });
      }
      if (path.startsWith("/api/v1/users?")) {
        return Promise.resolve({
          items: [minimalListItem],
          page: { page: 1, pageSize: 50, total: 1 },
        });
      }
      return Promise.resolve(minimalUserDetail);
    });

    const detail = await getUserByOrganizationSlugAndUsername("other-org", "bob");

    expect(detail?.id).toBe("user-bob");
    const calls = mockRequestDocumentJson.mock.calls.map((call) => String(call[0]));
    // 列表必须按目标组织过滤并用 keyword 精确定位（组织用户 >50 条时首页
    // 翻页找不到目标用户的回归防线）。
    const listUrl = calls.find((url) => url.startsWith("/api/v1/users?"));
    expect(listUrl).toContain("organizationId=org-other");
    expect(listUrl).toContain("keyword=bob");
    // 详情 GET 必须显式携带目标组织：admin 会话组织 ≠ 用户所属组织时，
    // 缺省请求会落到会话组织导致 404。
    const detailUrl = calls.find((url) => url.startsWith("/api/v1/users/user-bob"));
    expect(detailUrl).toContain("organizationId=org-other");
  });

  it("returns null without any request when the organization slug cannot be resolved", async () => {
    mockRequestDocumentJson.mockRejectedValue(new HttpError(404, null, "not found"));

    const detail = await getUserByOrganizationSlugAndUsername("unknown-org", "bob");

    expect(detail).toBeNull();
    const userCalls = mockRequestDocumentJson.mock.calls.filter((call) =>
      String(call[0]).startsWith("/api/v1/users"),
    );
    expect(userCalls).toHaveLength(0);
  });

  it("propagates non-404 organization resolution failures instead of returning null", async () => {
    mockRequestDocumentJson.mockRejectedValue(new HttpError(403, null, "forbidden"));

    await expect(getUserByOrganizationSlugAndUsername("other-org", "bob")).rejects.toThrow(
      "forbidden",
    );
  });

  it("returns null without a detail GET when the keyword search does not match the username", async () => {
    mockRequestDocumentJson.mockImplementation((url: unknown) => {
      const path = String(url);
      if (path.startsWith("/api/v1/organizations/by-slug/")) {
        return Promise.resolve({ id: "org-other", slug: "other-org" });
      }
      return Promise.resolve({
        items: [],
        page: { page: 1, pageSize: 50, total: 0 },
      });
    });

    const detail = await getUserByOrganizationSlugAndUsername("other-org", "nobody");

    expect(detail).toBeNull();
    const detailCalls = mockRequestDocumentJson.mock.calls.filter((call) =>
      String(call[0]).startsWith("/api/v1/users/user-"),
    );
    expect(detailCalls).toHaveLength(0);
  });

  it("getUser omits the organization query when no organizationId is given (tenant user path unchanged)", async () => {
    mockRequestDocumentJson.mockResolvedValue(minimalUserDetail);

    await getUser("user-bob");

    expect(String(mockRequestDocumentJson.mock.calls[0][0])).toBe("/api/v1/users/user-bob");
  });

  it("getUser appends the organization query when given", async () => {
    mockRequestDocumentJson.mockResolvedValue(minimalUserDetail);

    await getUser("user-bob", "org-other");

    expect(String(mockRequestDocumentJson.mock.calls[0][0])).toBe(
      "/api/v1/users/user-bob?organizationId=org-other",
    );
  });
});
