import { describe, expect, it } from "vitest";
import { formatFileSize } from "./file-size";

describe("formatFileSize", () => {
  it("keeps sub-kilobyte values in bytes", () => {
    expect(formatFileSize(0)).toBe("0 B");
    expect(formatFileSize(1)).toBe("1 B");
    expect(formatFileSize(1023)).toBe("1023 B");
  });

  it("uses binary units with up to two decimals", () => {
    expect(formatFileSize(1024)).toBe("1 KB");
    expect(formatFileSize(1536)).toBe("1.5 KB");
    expect(formatFileSize(1024 ** 2)).toBe("1 MB");
    expect(formatFileSize(1.5 * 1024 ** 3)).toBe("1.5 GB");
    expect(formatFileSize(1024 ** 5)).toBe("1 PB");
  });

  it("returns a placeholder for missing or invalid values", () => {
    expect(formatFileSize(undefined)).toBe("-");
    expect(formatFileSize(null)).toBe("-");
    expect(formatFileSize(-1)).toBe("-");
    expect(formatFileSize(Number.NaN)).toBe("-");
    expect(formatFileSize(Number.POSITIVE_INFINITY)).toBe("-");
  });
});
