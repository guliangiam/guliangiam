import { beforeEach, describe, expect, it, vi } from "vitest";
import { getOnlineMcpServers } from "./mcp-server";

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

describe("getOnlineMcpServers", () => {
  beforeEach(() => {
    mockIamGet.mockReset();
    mockIamPostRaw.mockReset();
    mockIamGet.mockResolvedValue([]);
  });

  // The handler registers `router.Get("/api/v1/mcp-servers/online")`. The console
  // used to POST here, which matched no route, so the store page never loaded.
  it("issues a GET, not a POST", () => {
    getOnlineMcpServers();

    expect(mockIamGet).toHaveBeenCalledTimes(1);
    expect(mockIamPostRaw).not.toHaveBeenCalled();
  });

  it("requests the path the handler registers", () => {
    getOnlineMcpServers();
    expect(mockIamGet).toHaveBeenCalledWith("/api/v1/mcp-servers/online");
  });

  // The handler reads `registryUrl` from the query string, so a caller-supplied
  // registry has to travel as a parameter rather than a request body.
  it("passes a registry url as the query parameter the handler reads", () => {
    getOnlineMcpServers("https://registry.example.com/servers.json");
    expect(mockIamGet).toHaveBeenCalledWith(
      "/api/v1/mcp-servers/online?registryUrl=https%3A%2F%2Fregistry.example.com%2Fservers.json",
    );
  });

  it("omits the query string when no registry is given", () => {
    getOnlineMcpServers();
    const path = String(mockIamGet.mock.calls[0][0]);
    expect(path).not.toContain("?");
    expect(path).not.toContain("registryUrl");
  });

  it("returns the array the server sends", async () => {
    const servers = [
      { name: "filesystem", description: "File access", url: "http://x/mcp", category: "Local" },
    ];
    mockIamGet.mockResolvedValue(servers);
    await expect(getOnlineMcpServers()).resolves.toEqual(servers);
  });
});
