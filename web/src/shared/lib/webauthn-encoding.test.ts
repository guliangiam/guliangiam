import { describe, it, expect } from "vitest";

// Pure utility: WebAuthn base64url encoding/decoding
function webAuthnBufferDecode(value: string) {
  let normalized = value.replace(/-/g, "+").replace(/_/g, "/");
  while (normalized.length % 4) {
    normalized += "=";
  }
  return Uint8Array.from(atob(normalized), (character) => character.charCodeAt(0));
}

function webAuthnBufferEncode(value: ArrayBuffer | null) {
  if (!value) {
    return "";
  }
  return btoa(String.fromCharCode(...new Uint8Array(value)))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=/g, "");
}

describe("webAuthnBufferEncode", () => {
  it("encodes null as empty string", () => {
    expect(webAuthnBufferEncode(null)).toBe("");
  });

  it("encodes empty buffer", () => {
    const buffer = new Uint8Array(0).buffer;
    expect(webAuthnBufferEncode(buffer)).toBe("");
  });

  it("encodes simple bytes", () => {
    const buffer = new Uint8Array([1, 2, 3]).buffer;
    const result = webAuthnBufferEncode(buffer);
    expect(result).toBeTruthy();
    expect(result).not.toContain("+");
    expect(result).not.toContain("/");
    expect(result).not.toContain("=");
  });

  it("produces base64url format (no +/= characters)", () => {
    const buffer = new Uint8Array([255, 254, 253, 252]).buffer;
    const result = webAuthnBufferEncode(buffer);
    expect(result).not.toContain("+");
    expect(result).not.toContain("/");
    expect(result).not.toContain("=");
  });
});

describe("webAuthnBufferDecode", () => {
  it("decodes base64url string", () => {
    const encoded = "AQID"; // base64 of [1, 2, 3]
    const result = webAuthnBufferDecode(encoded);
    expect(Array.from(result)).toEqual([1, 2, 3]);
  });

  it("handles base64url with - and _", () => {
    // First encode, then decode to verify round-trip
    const original = new Uint8Array([72, 101, 108, 108, 111]);
    const encoded = webAuthnBufferEncode(original.buffer);
    const decoded = webAuthnBufferDecode(encoded);
    expect(Array.from(decoded)).toEqual(Array.from(original));
  });
});

describe("webAuthn round-trip", () => {
  it("encode then decode preserves data", () => {
    const testCases = [
      new Uint8Array([0]),
      new Uint8Array([1, 2, 3, 4, 5]),
      new Uint8Array([255, 128, 64, 32, 16, 8, 4, 2, 1]),
      new Uint8Array(Array.from({ length: 32 }, (_, i) => i * 8)),
    ];

    for (const original of testCases) {
      const encoded = webAuthnBufferEncode(original.buffer);
      const decoded = webAuthnBufferDecode(encoded);
      expect(Array.from(decoded)).toEqual(Array.from(original));
    }
  });
});
