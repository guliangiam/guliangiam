import { beforeEach, describe, expect, it, vi } from "vitest";
import { testIamLdapConnection } from "./ldap";

const mockIamGet = vi.fn<(...args: unknown[]) => unknown>();
const mockIamPostRaw = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("./client", () => ({
  iamList: vi.fn<(...args: unknown[]) => unknown>(),
  iamListWithPagination: vi.fn<(...args: unknown[]) => unknown>(),
  iamGet: (...args: unknown[]) => mockIamGet(...args),
  iamCreate: vi.fn<(...args: unknown[]) => unknown>(),
  iamPatch: vi.fn<(...args: unknown[]) => unknown>(),
  iamDelete: vi.fn<(...args: unknown[]) => unknown>(),
  iamUpdate: vi.fn<(...args: unknown[]) => unknown>(),
  iamPostRaw: (...args: unknown[]) => mockIamPostRaw(...args),
}));

describe("LDAP API", () => {
  beforeEach(() => {
    mockIamGet.mockReset();
    mockIamPostRaw.mockReset();
  });

  it("tests the connection through the {id}:test-connection action", async () => {
    mockIamPostRaw.mockResolvedValue(undefined);
    await testIamLdapConnection("ldap-1");
    expect(mockIamPostRaw).toHaveBeenCalledWith("/api/v1/ldaps/ldap-1/test-connection");
  });
});
