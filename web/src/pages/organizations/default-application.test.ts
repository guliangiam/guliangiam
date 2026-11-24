import { describe, expect, it } from "vitest";
import { defaultApplicationLabel } from "./default-application";

const names = new Map<string, string>([["app-1", "Console"]]);

describe("defaultApplicationLabel", () => {
  it("returns the resolved application name", () => {
    expect(defaultApplicationLabel("app-1", names)).toEqual({ label: "Console", title: "app-1" });
  });

  it("falls back to the id with a title when the name is unknown", () => {
    expect(defaultApplicationLabel("app-missing", names)).toEqual({
      label: "app-missing",
      title: "app-missing",
    });
  });

  it("renders nothing for an empty value", () => {
    expect(defaultApplicationLabel(undefined, names)).toBeNull();
    expect(defaultApplicationLabel("  ", names)).toBeNull();
  });
});
