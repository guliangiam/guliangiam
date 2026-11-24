import { describe, expect, it } from "vitest";
import { pickActiveSection } from "./scrollspy";

describe("pickActiveSection", () => {
  it("returns the section whose top sits exactly on the band", () => {
    const sections = [
      { key: "basic", top: 0 },
      { key: "advanced", top: 1000 },
    ];

    expect(pickActiveSection(sections, 1000, false)).toBe("advanced");
  });

  it("does not let a long previous section steal the next one", () => {
    // 上一个分区很长（top 远在上方），旧实现会把 top 更小的它当成首个可见项。
    const sections = [
      { key: "oidc", top: 0 },
      { key: "saml", top: 2400 },
    ];

    expect(pickActiveSection(sections, 2400, false)).toBe("saml");
  });

  it("keeps the previous section while the band is inside it", () => {
    const sections = [
      { key: "basic", top: 0 },
      { key: "credentials", top: 500 },
    ];

    expect(pickActiveSection(sections, 320, false)).toBe("basic");
  });

  it("falls back to the first section when every top is below the band", () => {
    const sections = [
      { key: "basic", top: 120 },
      { key: "advanced", top: 800 },
    ];

    expect(pickActiveSection(sections, 40, false)).toBe("basic");
  });

  it("selects the last section when the container is scrolled to the bottom", () => {
    const sections = [
      { key: "basic", top: 0 },
      { key: "advanced", top: 2400 },
    ];

    expect(pickActiveSection(sections, 600, true)).toBe("advanced");
  });

  it("sorts unordered input before deciding", () => {
    const sections = [
      { key: "advanced", top: 900 },
      { key: "basic", top: 0 },
    ];

    expect(pickActiveSection(sections, 950, false)).toBe("advanced");
  });

  it("returns undefined when there are no sections", () => {
    expect(pickActiveSection([], 100, false)).toBeUndefined();
  });
});
