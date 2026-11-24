import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  getPermissions,
  getPermissionsPaginated,
  getPermission,
  createPermission,
  updatePermission,
  deletePermission,
  enforcePermission,
  batchEnforcePermission,
  getPermissionPolicy,
  getPermissionObjects,
  getPermissionActions,
  getPermissionRoles,
  updatePermissionPolicy,
} from "./permission";
import { permissionCreateSchema } from "./schemas";

// Mock the HTTP module
const mockRequestDocumentJson = vi.fn<(...args: unknown[]) => unknown>();
vi.mock("@/shared/api/http", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/shared/api/http")>();
  return {
    ...actual,
    requestDocumentJson: (...args: unknown[]) => mockRequestDocumentJson(...args),
  };
});

const mockPermListResponse = {
  items: [
    {
      id: "perm-1",
      organizationId: "org-1",
      name: "read-users",
      displayName: "Read Users",
      description: "Permission to read users",
      resourceType: "user",
      effect: "allow",
      isEnabled: true,
      submitter: "admin",
      approver: "admin",
      state: "approved",
      createdAt: "2024-01-01T00:00:00Z",
      updatedAt: "2024-01-02T00:00:00Z",
    },
  ],
  page: { page: 1, pageSize: 50, total: 1 },
};

describe("Permission API", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("getPermissions", () => {
    it("fetches permissions and maps to list items", async () => {
      mockRequestDocumentJson.mockResolvedValue(mockPermListResponse);

      const result = await getPermissions();
      expect(result).toHaveLength(1);
      expect(result[0].name).toBe("read-users");
      expect(result[0].owner).toBe("org-1");
      expect(result[0].isEnabled).toBe(true);
    });

    it("passes organizationId as query param", async () => {
      mockRequestDocumentJson.mockResolvedValue(mockPermListResponse);

      await getPermissions("org-1");
      const calledUrl = mockRequestDocumentJson.mock.calls[0][0] as string;
      expect(calledUrl).toContain("organizationId=org-1");
    });
  });

  describe("getPermissionsPaginated", () => {
    it("returns paginated result", async () => {
      mockRequestDocumentJson.mockResolvedValue(mockPermListResponse);

      const result = await getPermissionsPaginated({ page: 1, pageSize: 10 });
      expect(result.items).toHaveLength(1);
      expect(result.total).toBe(1);
    });

    it("passes search as keyword param", async () => {
      mockRequestDocumentJson.mockResolvedValue(mockPermListResponse);

      await getPermissionsPaginated({ search: "read" });
      const calledUrl = mockRequestDocumentJson.mock.calls[0][0] as string;
      expect(calledUrl).toContain("keyword=read");
    });
  });

  describe("getPermission", () => {
    it("fetches permission detail by id", async () => {
      mockRequestDocumentJson.mockResolvedValue({
        ...mockPermListResponse.items[0],
        users: ["user-1"],
        groups: [],
        roles: ["role-1"],
        domains: [],
        resources: ["users"],
        actions: ["read"],
        model: "RBAC",
        adapter: "file",
        sorter: "priority",
        application: "",
        approveTime: "2024-01-01T00:00:00Z",
        enforcerId: "enc-1",
        revision: 1,
      });

      const result = await getPermission("perm-1");
      expect(result.id).toBe("perm-1");
      expect(result.users).toEqual(["user-1"]);
      expect(result.roles).toEqual(["role-1"]);
      expect(result.sorter).toBe("priority");
      expect(result.revision).toBe(1);
    });
  });

  describe("createPermission", () => {
    it("sends POST request with payload", async () => {
      mockRequestDocumentJson.mockResolvedValue(undefined);

      const payload = {
        organizationId: "org-1",
        name: "write-users",
        displayName: "Write Users",
      };

      await createPermission(payload);
      expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/permissions", {
        method: "POST",
        body: JSON.stringify(payload),
        headers: { "Content-Type": "application/json" },
      });
    });
  });

  describe("updatePermission", () => {
    it("sends PATCH request with payload", async () => {
      mockRequestDocumentJson.mockResolvedValue(undefined);

      const payload = { displayName: "Updated Permission", revision: 1 };
      await updatePermission("perm-1", payload);

      expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/permissions/perm-1", {
        method: "PATCH",
        body: JSON.stringify(payload),
        headers: { "Content-Type": "application/json" },
      });
    });
  });

  describe("deletePermission", () => {
    it("sends DELETE request", async () => {
      mockRequestDocumentJson.mockResolvedValue(undefined);
      await deletePermission("perm-1");
      expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/permissions/perm-1", {
        method: "DELETE",
      });
    });
  });

  describe("enforcePermission", () => {
    it("returns true when permission is allowed (data field)", async () => {
      mockRequestDocumentJson.mockResolvedValue({ data: true });
      const result = await enforcePermission({ v0: "alice", v1: "users", v2: "read" });
      expect(result).toBe(true);
    });

    it("returns true when permission is allowed (allow field)", async () => {
      mockRequestDocumentJson.mockResolvedValue({ allow: true });
      const result = await enforcePermission({ v0: "alice", v1: "users", v2: "read" });
      expect(result).toBe(true);
    });

    it("returns false when permission is denied", async () => {
      mockRequestDocumentJson.mockResolvedValue({ data: false });
      const result = await enforcePermission({ v0: "bob", v1: "users", v2: "write" });
      expect(result).toBe(false);
    });
  });

  describe("batchEnforcePermission", () => {
    it("returns boolean array from data", async () => {
      mockRequestDocumentJson.mockResolvedValue({ data: [true, false] });
      const result = await batchEnforcePermission({
        requests: [
          { v0: "alice", v1: "users", v2: "read" },
          { v0: "bob", v1: "users", v2: "write" },
        ],
      });
      expect(result).toEqual([true, false]);
    });
  });

  describe("getPermissionPolicy", () => {
    it("returns policy rules from the policies envelope", async () => {
      mockRequestDocumentJson.mockResolvedValue({
        policies: [["p", "alice", "users", "read"]],
      });
      const result = await getPermissionPolicy("perm-1");
      expect(result).toEqual([["p", "alice", "users", "read"]]);
    });
  });

  describe("updatePermissionPolicy", () => {
    it("sends the bulk policies envelope the backend requires", async () => {
      mockRequestDocumentJson.mockResolvedValue(undefined);
      const payload = {
        policies: [{ pType: "p", rules: ["alice", "users", "read"] }],
      };

      await updatePermissionPolicy("perm-1", payload);

      expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/permissions/perm-1/policy", {
        method: "PUT",
        body: JSON.stringify(payload),
        headers: { "Content-Type": "application/json" },
      });
    });
  });

  describe("getPermissionObjects", () => {
    it("returns objects from data", async () => {
      mockRequestDocumentJson.mockResolvedValue({ data: ["users", "organizations"] });
      const result = await getPermissionObjects("perm-1");
      expect(result).toEqual(["users", "organizations"]);
    });
  });

  describe("getPermissionActions", () => {
    it("returns actions from data", async () => {
      mockRequestDocumentJson.mockResolvedValue({ data: ["read", "write"] });
      const result = await getPermissionActions("perm-1");
      expect(result).toEqual(["read", "write"]);
    });
  });

  describe("getPermissionRoles", () => {
    it("returns roles from data", async () => {
      mockRequestDocumentJson.mockResolvedValue({ data: ["admin", "user"] });
      const result = await getPermissionRoles("perm-1");
      expect(result).toEqual(["admin", "user"]);
    });
  });
});

describe("permissionCreateSchema", () => {
  const base = {
    organizationId: "org-1",
    name: "read-users",
    displayName: "Read Users",
    resourceType: "User",
    actions: ["Read"],
    sorter: "100",
  };

  it.each(["allow", "deny"])("accepts the form's lowercase effect %s", (effect) => {
    const result = permissionCreateSchema.safeParse({ ...base, effect });
    expect(result.success).toBe(true);
  });

  it.each(["Allow", "Deny"])("rejects the PascalCase effect %s", (effect) => {
    const result = permissionCreateSchema.safeParse({ ...base, effect });
    expect(result.success).toBe(false);
  });

  it("round-trips the backend's string sorter", () => {
    expect(permissionCreateSchema.safeParse({ ...base, effect: "allow" }).success).toBe(true);
  });

  it("rejects a numeric sorter because the API DTO is *string", () => {
    const result = permissionCreateSchema.safeParse({
      ...base,
      effect: "allow",
      sorter: 100,
    });
    expect(result.success).toBe(false);
  });
});
