import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  getIamEnforcers,
  getIamEnforcersPaginated,
  getIamEnforcer,
  createIamEnforcer,
  patchIamEnforcer,
  deleteIamEnforcer,
  getEnforcerPolicies,
  addEnforcerPolicy,
  removeEnforcerPolicy,
  updateEnforcerPolicy,
  replaceEnforcerPolicies,
  enforceIamEnforcer,
  batchEnforceIamEnforcer,
  getIamEnforcerSubjects,
  getIamEnforcerObjects,
  getIamEnforcerActions,
  getIamEnforcerRoles,
  type Policy,
} from "./enforcer";

// Mock the IAM client module
const mockIamList = vi.fn<(...args: unknown[]) => unknown>();
const mockIamListWithPagination = vi.fn<(...args: unknown[]) => unknown>();
const mockIamGet = vi.fn<(...args: unknown[]) => unknown>();
const mockIamCreate = vi.fn<(...args: unknown[]) => unknown>();
const mockIamPatch = vi.fn<(...args: unknown[]) => unknown>();
const mockIamDelete = vi.fn<(...args: unknown[]) => unknown>();
const mockIamUpdate = vi.fn<(...args: unknown[]) => unknown>();
const mockIamPostRaw = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("./client", () => ({
  iamList: (...args: unknown[]) => mockIamList(...args),
  iamListWithPagination: (...args: unknown[]) => mockIamListWithPagination(...args),
  iamGet: (...args: unknown[]) => mockIamGet(...args),
  iamCreate: (...args: unknown[]) => mockIamCreate(...args),
  iamPatch: (...args: unknown[]) => mockIamPatch(...args),
  iamDelete: (...args: unknown[]) => mockIamDelete(...args),
  iamUpdate: (...args: unknown[]) => mockIamUpdate(...args),
  iamPostRaw: (...args: unknown[]) => mockIamPostRaw(...args),
}));

describe("Enforcer API", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // ── List & Paginated ──

  describe("getIamEnforcers", () => {
    it("calls iamList with correct path", async () => {
      mockIamList.mockResolvedValue([]);
      await getIamEnforcers();
      expect(mockIamList).toHaveBeenCalledWith("/api/v1/enforcers", undefined);
    });

    it("passes params to iamList", async () => {
      mockIamList.mockResolvedValue([]);
      await getIamEnforcers({ page: 2, pageSize: 10 });
      expect(mockIamList).toHaveBeenCalledWith("/api/v1/enforcers", { page: 2, pageSize: 10 });
    });
  });

  describe("getIamEnforcersPaginated", () => {
    it("calls iamListWithPagination with correct path", async () => {
      mockIamListWithPagination.mockResolvedValue({ items: [], page: 1, pageSize: 50, total: 0 });
      await getIamEnforcersPaginated();
      expect(mockIamListWithPagination).toHaveBeenCalledWith("/api/v1/enforcers", undefined);
    });
  });

  // ── Detail ──

  describe("getIamEnforcer", () => {
    it("calls iamGet with correct path", async () => {
      mockIamGet.mockResolvedValue({ id: "enc-1", name: "test" });
      await getIamEnforcer("enc-1");
      expect(mockIamGet).toHaveBeenCalledWith("/api/v1/enforcers/enc-1");
    });
  });

  // ── Create ──

  describe("createIamEnforcer", () => {
    it("calls iamCreate with correct path and payload", async () => {
      mockIamCreate.mockResolvedValue({ id: "enc-new" });
      const payload = {
        organizationId: "org-1",
        name: "enforcer-1",
        displayName: "Enforcer 1",
        type: "RBAC",
      };
      await createIamEnforcer(payload);
      expect(mockIamCreate).toHaveBeenCalledWith("/api/v1/enforcers", payload);
    });
  });

  // ── Patch ──

  describe("patchIamEnforcer", () => {
    it("calls iamPatch with correct path and payload", async () => {
      mockIamPatch.mockResolvedValue(undefined);
      const payload = { displayName: "Updated", revision: 1 };
      await patchIamEnforcer("enc-1", payload);
      expect(mockIamPatch).toHaveBeenCalledWith("/api/v1/enforcers/enc-1", payload);
    });
  });

  // ── Delete ──

  describe("deleteIamEnforcer", () => {
    it("calls iamDelete with correct path", async () => {
      mockIamDelete.mockResolvedValue(undefined);
      await deleteIamEnforcer("enc-1");
      expect(mockIamDelete).toHaveBeenCalledWith("/api/v1/enforcers/enc-1");
    });
  });

  // ── Structured Policy ──

  describe("getEnforcerPolicies", () => {
    it("maps raw policy rows to Policy objects", async () => {
      mockIamGet.mockResolvedValue({ data: [["p", "alice", "data1", "read"]] });
      const policies = await getEnforcerPolicies("enc-1");
      expect(policies).toEqual([{ ptype: "p", subject: "alice", object: "data1", action: "read" }]);
    });

    it("handles empty policy list", async () => {
      mockIamGet.mockResolvedValue({ data: [] });
      const policies = await getEnforcerPolicies("enc-1");
      expect(policies).toEqual([]);
    });

    it("defaults missing fields", async () => {
      mockIamGet.mockResolvedValue({ data: [[undefined, undefined, undefined, undefined]] });
      const policies = await getEnforcerPolicies("enc-1");
      expect(policies[0]).toEqual({ ptype: "p", subject: "", object: "", action: "" });
    });
  });

  describe("addEnforcerPolicy", () => {
    it("calls iamCreate with correct path and policy", async () => {
      mockIamCreate.mockResolvedValue(undefined);
      const policy: Policy = { ptype: "p", subject: "bob", object: "data2", action: "write" };
      await addEnforcerPolicy("enc-1", policy);
      expect(mockIamCreate).toHaveBeenCalledWith("/api/v1/enforcers/enc-1/policies", {
        pType: "p",
        rules: ["bob", "data2", "write"],
      });
    });
  });

  describe("removeEnforcerPolicy", () => {
    it("identifies the rule in the DELETE body rather than the path", async () => {
      mockIamDelete.mockResolvedValue(undefined);
      const policy: Policy = { ptype: "p", subject: "alice", object: "data1", action: "read" };
      await removeEnforcerPolicy("enc-1", policy);
      // There is no `/policies/{index}` route; the handler decodes a
      // PolicyRequest from the body, exactly like addEnforcerPolicy.
      expect(mockIamDelete).toHaveBeenCalledWith("/api/v1/enforcers/enc-1/policies", {
        pType: "p",
        rules: ["alice", "data1", "read"],
      });
    });
  });

  describe("updateEnforcerPolicy", () => {
    it("renames a rule through PUT with both full Casbin rows", async () => {
      mockIamUpdate.mockResolvedValue(undefined);
      const oldPolicy: Policy = { ptype: "p", subject: "alice", object: "data1", action: "read" };
      const newPolicy: Policy = { ptype: "p", subject: "alice", object: "data1", action: "write" };
      await updateEnforcerPolicy("enc-1", oldPolicy, newPolicy);
      // Enforcer.UpdatePolicy matches on the whole rule, so the ptype element
      // must be present on both sides.
      expect(mockIamUpdate).toHaveBeenCalledWith("/api/v1/enforcers/enc-1/policies", {
        oldPolicy: ["p", "alice", "data1", "read"],
        newPolicy: ["p", "alice", "data1", "write"],
      });
    });
  });

  describe("replaceEnforcerPolicies", () => {
    it("posts full Casbin rows to the import-policies endpoint", async () => {
      mockIamCreate.mockResolvedValue(undefined);
      const policies: Policy[] = [
        { ptype: "p", subject: "alice", object: "data1", action: "read" },
        { ptype: "p", subject: "bob", object: "data2", action: "write" },
      ];
      await replaceEnforcerPolicies("enc-1", policies);
      // `PUT /policies` decodes {oldPolicy,newPolicy} and renames one rule; it is
      // not a replace-all. Only `/import-policies` has that semantics, and the
      // service feeds each row straight to Casbin.AddPolicy.
      expect(mockIamCreate).toHaveBeenCalledWith("/api/v1/enforcers/enc-1/import-policies", {
        policies: [
          ["p", "alice", "data1", "read"],
          ["p", "bob", "data2", "write"],
        ],
      });
    });
  });

  // ── Enforce ──

  describe("enforceIamEnforcer", () => {
    it("returns true when data is true", async () => {
      mockIamPostRaw.mockResolvedValue({ data: true });
      const result = await enforceIamEnforcer("enc-1", { v0: "alice", v1: "data1", v2: "read" });
      expect(result).toBe(true);
    });

    it("returns true when raw result is true", async () => {
      mockIamPostRaw.mockResolvedValue(true);
      const result = await enforceIamEnforcer("enc-1", { v0: "alice", v1: "data1", v2: "read" });
      expect(result).toBe(true);
    });

    it("returns false when data is false", async () => {
      mockIamPostRaw.mockResolvedValue({ data: false });
      const result = await enforceIamEnforcer("enc-1", { v0: "alice", v1: "data1", v2: "write" });
      expect(result).toBe(false);
    });
  });

  describe("batchEnforceIamEnforcer", () => {
    it("returns boolean array from data", async () => {
      mockIamPostRaw.mockResolvedValue({ data: [true, false, true] });
      const result = await batchEnforceIamEnforcer("enc-1", {
        requests: [
          { v0: "alice", v1: "data1", v2: "read" },
          { v0: "bob", v1: "data1", v2: "read" },
          { v0: "alice", v1: "data2", v2: "write" },
        ],
      });
      expect(result).toEqual([true, false, true]);
    });

    it("returns empty array when data is missing", async () => {
      mockIamPostRaw.mockResolvedValue({});
      const result = await batchEnforceIamEnforcer("enc-1", { requests: [] });
      expect(result).toEqual([]);
    });
  });

  // ── Metadata ──

  describe("getIamEnforcerSubjects", () => {
    it("returns subjects from data", async () => {
      mockIamGet.mockResolvedValue({ data: ["alice", "bob"] });
      const result = await getIamEnforcerSubjects("enc-1");
      expect(result).toEqual(["alice", "bob"]);
    });
  });

  describe("getIamEnforcerObjects", () => {
    it("returns objects from data", async () => {
      mockIamGet.mockResolvedValue({ data: ["data1", "data2"] });
      const result = await getIamEnforcerObjects("enc-1");
      expect(result).toEqual(["data1", "data2"]);
    });
  });

  describe("getIamEnforcerActions", () => {
    it("returns actions from data", async () => {
      mockIamGet.mockResolvedValue({ data: ["read", "write"] });
      const result = await getIamEnforcerActions("enc-1");
      expect(result).toEqual(["read", "write"]);
    });
  });

  describe("getIamEnforcerRoles", () => {
    it("returns roles from data", async () => {
      mockIamGet.mockResolvedValue({ data: ["admin", "user"] });
      const result = await getIamEnforcerRoles("enc-1");
      expect(result).toEqual(["admin", "user"]);
    });
  });
});
