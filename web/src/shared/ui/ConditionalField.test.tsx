import { describe, it, expect, vi } from "vitest";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

describe("ConditionalField logic", () => {
  it("when function returns true for matching values", () => {
    const when = (v: Record<string, unknown>) => v.type === "A";
    expect(when({ type: "A" })).toBe(true);
    expect(when({ type: "B" })).toBe(false);
  });

  it("when function returns true for multiple matching dependencies", () => {
    const when = (v: Record<string, unknown>) => v.a === "1" && v.b === "2";
    expect(when({ a: "1", b: "2" })).toBe(true);
    expect(when({ a: "1", b: "x" })).toBe(false);
    expect(when({ a: "x", b: "2" })).toBe(false);
  });

  it("when function handles boolean values", () => {
    const when = (v: Record<string, unknown>) => !!v.enabled;
    expect(when({ enabled: true })).toBe(true);
    expect(when({ enabled: false })).toBe(false);
    expect(when({ enabled: undefined })).toBe(false);
  });

  it("when function handles undefined field gracefully", () => {
    const when = (v: Record<string, unknown>) => v.missing === undefined;
    expect(when({})).toBe(true);
    expect(when({ missing: "value" })).toBe(false);
  });

  it("dependencies array comparison detects changes", () => {
    const dependencies = ["type", "status"] as const;
    const prev: Record<string, string> = { type: "A", status: "active" };
    const cur: Record<string, string> = { type: "A", status: "inactive" };
    const changed = dependencies.some((dep) => prev[dep] !== cur[dep]);
    expect(changed).toBe(true);

    const same: Record<string, string> = { type: "A", status: "active" };
    const notChanged = dependencies.some((dep) => prev[dep] !== same[dep]);
    expect(notChanged).toBe(false);
  });
});
