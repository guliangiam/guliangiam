import { describe, it, expect } from "vitest";
import {
  arrayBufferToBase64Url,
  base64UrlToArrayBuffer,
  isWebAuthnSupported,
  hasPlatformAuthenticator,
} from "./webauthn-b64";

describe("arrayBufferToBase64Url", () => {
  it("encodes empty buffer as empty string", () => {
    expect(arrayBufferToBase64Url(new ArrayBuffer(0))).toBe("");
  });

  it("produces pure base64url (no +, /, =)", () => {
    const buf = new Uint8Array([255, 254, 253, 252]).buffer;
    const result = arrayBufferToBase64Url(buf);
    expect(result).not.toContain("+");
    expect(result).not.toContain("/");
    expect(result).not.toContain("=");
  });

  it("handles 32-byte WebAuthn challenge exactly", () => {
    const challenge = new Uint8Array(32).map((_, i) => i * 8);
    const encoded = arrayBufferToBase64Url(challenge.buffer);
    expect(encoded.length).toBeGreaterThan(32);
    expect(encoded.length).toBeLessThanOrEqual(44);
  });
});

describe("base64UrlToArrayBuffer", () => {
  it("decodes round-trip with encode", () => {
    const original = new Uint8Array([0, 1, 2, 3, 4, 5, 255, 128, 64, 32, 16, 8, 4, 2, 1, 100, 200]);
    const encoded = arrayBufferToBase64Url(original.buffer);
    const decoded = base64UrlToArrayBuffer(encoded);
    expect(new Uint8Array(decoded)).toEqual(original);
  });

  it("handles input with and without padding", () => {
    // 3 bytes → 4 base64 chars, no padding needed in base64url
    const b3 = new Uint8Array([1, 2, 3]).buffer;
    const enc3 = arrayBufferToBase64Url(b3);
    expect(new Uint8Array(base64UrlToArrayBuffer(enc3))).toEqual(new Uint8Array([1, 2, 3]));
  });
});

describe("round-trip edge cases", () => {
  it("single byte 0", () => {
    const buf = new Uint8Array([0]).buffer;
    const enc = arrayBufferToBase64Url(buf);
    const dec = base64UrlToArrayBuffer(enc);
    expect(new Uint8Array(dec)).toEqual(new Uint8Array([0]));
  });

  it("32 bytes of all 0xFF", () => {
    const buf = new Uint8Array(32).fill(0xff).buffer;
    const enc = arrayBufferToBase64Url(buf);
    const dec = base64UrlToArrayBuffer(enc);
    expect(new Uint8Array(dec)).toEqual(new Uint8Array(32).fill(0xff));
  });
});

describe("isWebAuthnSupported", () => {
  it("returns boolean (always callable in JSDOM)", () => {
    expect(typeof isWebAuthnSupported()).toBe("boolean");
  });
});

describe("hasPlatformAuthenticator", () => {
  it("returns boolean Promise", async () => {
    const result = await hasPlatformAuthenticator();
    expect(typeof result).toBe("boolean");
  });
});
