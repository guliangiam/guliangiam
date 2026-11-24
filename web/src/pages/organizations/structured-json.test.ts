import { describe, expect, it } from "vitest";
import {
  parseStructuredJson,
  serializeStructuredJson,
  serializeStructuredJsonDraft,
  type StructuredJsonRow,
} from "./structured-json";

describe("structured-json", () => {
  it("treats empty input as an empty object", () => {
    expect(parseStructuredJson("")).toEqual([]);
    expect(parseStructuredJson(undefined)).toEqual([]);
  });

  it("preserves primitive value types", () => {
    const rows = parseStructuredJson(
      JSON.stringify({ enabled: true, retries: 3, region: "cn-north", nested: { a: 1 } }),
    );
    expect(rows).not.toBeNull();
    expect(rows?.map((row) => row.kind)).toEqual(["boolean", "number", "string", "json"]);
    expect(rows?.[0].value).toBe(true);
    expect(rows?.[1].value).toBe(3);
  });

  it("rejects non-object and invalid JSON", () => {
    expect(parseStructuredJson("[1,2]")).toBeNull();
    expect(parseStructuredJson("42")).toBeNull();
    expect(parseStructuredJson("{")).toBeNull();
  });

  it("round-trips a flat object", () => {
    const rows = parseStructuredJson(
      JSON.stringify({ a: "x", b: false, c: 1 }),
    ) as StructuredJsonRow[];
    expect(serializeStructuredJson(rows)).toBe(JSON.stringify({ a: "x", b: false, c: 1 }, null, 2));
  });

  it("refuses empty or duplicate keys", () => {
    const base: StructuredJsonRow = { id: 1, key: "", kind: "string", value: "x" };
    expect(serializeStructuredJson([base])).toBeNull();
    expect(
      serializeStructuredJson([
        { id: 1, key: "a", kind: "string", value: "1" },
        { id: 2, key: "a", kind: "string", value: "2" },
      ]),
    ).toBeNull();
  });

  it("refuses invalid nested JSON values", () => {
    expect(serializeStructuredJson([{ id: 1, key: "a", kind: "json", value: "{" }])).toBeNull();
  });

  it("drafts invalid rows so switching to advanced mode keeps visible content", () => {
    const rows: StructuredJsonRow[] = [
      { id: 1, key: "", kind: "string", value: "x" },
      { id: 2, key: "a", kind: "json", value: "{" },
    ];
    const draft = serializeStructuredJsonDraft(rows);
    expect(draft).toContain('"": "x"');
    expect(draft).toContain('"a": {');
    expect(serializeStructuredJson(rows)).toBeNull();
  });

  it("drafts empty rows as an empty object", () => {
    expect(serializeStructuredJsonDraft([])).toBe("{}");
  });
});
