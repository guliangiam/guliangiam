import { describe, expect, it } from "vitest";
import type { IamMcpServerDetail } from "@/shared/api/iam/mcp-server";
import {
  buildMcpServerFormValues,
  buildMcpServerPatchPayload,
  type McpServerFormValues,
} from "./mcp-server-patch-payload";

const detail: IamMcpServerDetail = {
  id: "mcp-1",
  organizationId: "org-1",
  name: "server",
  displayName: "Server",
  url: "https://mcp.example.com",
  application: "app-1",
  isEnabled: true,
  createdAt: "2026-11-03T00:00:00Z",
  updatedAt: "2026-11-03T00:00:00Z",
  tokenSet: true,
  tokenMasked: "to***23",
  tools: [],
  revision: 7,
};

const baseValues: McpServerFormValues = {
  displayName: "Server",
  url: "https://mcp.example.com",
  token: "",
  application: "app-1",
  isEnabled: true,
};

const requestBody = (payload: unknown) => JSON.stringify(payload);

describe("buildMcpServerFormValues", () => {
  it("never prefills the token input from the detail response", () => {
    const values = buildMcpServerFormValues(detail);

    expect(values).not.toHaveProperty("token");
    expect(requestBody(values)).not.toContain("token");
    expect(requestBody(values)).not.toContain("to***23");
  });
});

describe("buildMcpServerPatchPayload", () => {
  it("omits token when the input was typed and then cleared", () => {
    const payload = buildMcpServerPatchPayload({ ...baseValues, token: "" }, 7);

    expect(payload).not.toHaveProperty("token");
    expect(requestBody(payload)).not.toContain("token");
  });

  it("treats a whitespace-only token as blank", () => {
    const payload = buildMcpServerPatchPayload({ ...baseValues, token: "   " }, 7);

    expect(payload).not.toHaveProperty("token");
    expect(requestBody(payload)).not.toContain("token");
  });

  it("omits token when the field was never touched", () => {
    const payload = buildMcpServerPatchPayload({ ...baseValues, token: undefined }, 7);

    expect(requestBody(payload)).not.toContain("token");
  });

  it("sends a trimmed replacement only when a new token was typed", () => {
    const payload = buildMcpServerPatchPayload({ ...baseValues, token: "  new-token  " }, 7);

    expect(payload).toMatchObject({
      displayName: "Server",
      url: "https://mcp.example.com",
      token: "new-token",
      application: "app-1",
      isEnabled: true,
      revision: 7,
    });
  });
});
