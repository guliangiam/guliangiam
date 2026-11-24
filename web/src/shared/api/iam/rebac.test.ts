import { describe, it, expect, vi, beforeEach } from "vitest";
import { createReBACTuple, type ReBACTupleItem } from "./rebac";

const mockRequestDocumentJson = vi.fn<(...args: unknown[]) => unknown>();

vi.mock("@/shared/api/http", () => ({
  requestDocumentJson: (...args: unknown[]) => mockRequestDocumentJson(...args),
}));

describe("ReBAC API", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("createReBACTuple", () => {
    const tupleResponse: ReBACTupleItem = {
      id: "tuple-1",
      organizationId: "org-1",
      namespaceId: "ns-1",
      subject: "User:user-456",
      relation: "editor",
      object: "Document:doc-123",
      zookie: "z1",
      createdAt: "2026-11-19T00:00:00Z",
    };

    it("sends a flat single-tuple body aligned with backend CreateTupleRequest", async () => {
      mockRequestDocumentJson.mockResolvedValue(tupleResponse);
      await createReBACTuple("ns-1", {
        subject: "User:user-456",
        relation: "editor",
        object: "Document:doc-123",
      });
      expect(mockRequestDocumentJson).toHaveBeenCalledWith("/api/v1/rebac/namespaces/ns-1/tuples", {
        method: "POST",
        body: JSON.stringify({
          subject: "User:user-456",
          relation: "editor",
          object: "Document:doc-123",
        }),
      });
      const [, options] = mockRequestDocumentJson.mock.calls[0] as [string, { body: string }];
      const parsed = JSON.parse(options.body) as Record<string, unknown>;
      expect(parsed).not.toHaveProperty("tuples");
      expect(parsed).not.toHaveProperty("namespaceId");
    });

    it("includes subjectRelation only when provided", async () => {
      mockRequestDocumentJson.mockResolvedValue(tupleResponse);
      await createReBACTuple("ns-1", {
        subject: "Group:g-1",
        subjectRelation: "member",
        relation: "viewer",
        object: "Document:doc-9",
      });
      const [, options] = mockRequestDocumentJson.mock.calls[0] as [string, { body: string }];
      expect(JSON.parse(options.body)).toEqual({
        subject: "Group:g-1",
        relation: "viewer",
        object: "Document:doc-9",
        subjectRelation: "member",
      });
    });

    it("parses the single TupleResponse object (not {ids})", async () => {
      mockRequestDocumentJson.mockResolvedValue(tupleResponse);
      const result = await createReBACTuple("ns-1", {
        subject: "User:user-456",
        relation: "editor",
        object: "Document:doc-123",
      });
      expect(result).toEqual(tupleResponse);
      expect(result.id).toBe("tuple-1");
    });
  });
});
