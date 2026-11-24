import { describe, expect, it } from "vitest";
import { agentPath, groupPath, mcpServerPath, permissionPath } from "./routePaths";

describe("frontend route paths", () => {
  it("builds the registered agent detail path", () => {
    expect(agentPath("agent-1")).toBe("/ai/agents/agent-1");
  });

  it("builds the registered MCP server detail path", () => {
    expect(mcpServerPath("server-1")).toBe("/ai/mcp-servers/server-1");
  });

  it("builds the registered group detail path from owner and name", () => {
    expect(groupPath("org-1", "admins")).toBe("/groups/org-1/admins");
  });

  it("builds the registered permission detail path from owner and name", () => {
    expect(permissionPath("org-1", "read-users")).toBe("/permissions/org-1/read-users");
  });

  it("encodes every dynamic segment", () => {
    expect(agentPath("a/b")).toBe("/ai/agents/a%2Fb");
    expect(mcpServerPath("a b")).toBe("/ai/mcp-servers/a%20b");
    expect(groupPath("org/team", "group name")).toBe("/groups/org%2Fteam/group%20name");
    expect(permissionPath("org/team", "permission/name")).toBe(
      "/permissions/org%2Fteam/permission%2Fname",
    );
  });
});
