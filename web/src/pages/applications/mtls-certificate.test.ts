import { describe, expect, it } from "vitest";
import {
  base64UrlFromBytes,
  certificateDerFromPem,
  certificateFingerprint,
  isCertificateFingerprint,
} from "./mtls-certificate";

const HELLO_PEM = "-----BEGIN CERTIFICATE-----\naGVsbG8=\n-----END CERTIFICATE-----";

describe("mtls-certificate", () => {
  it("decodes the base64 body of a PEM block", () => {
    const der = certificateDerFromPem(HELLO_PEM);
    expect(der).not.toBeNull();
    expect(new TextDecoder().decode(der!)).toBe("hello");
  });

  it("returns null for input without a CERTIFICATE block", () => {
    expect(certificateDerFromPem("not a pem")).toBeNull();
    expect(
      certificateDerFromPem("-----BEGIN CERTIFICATE-----\n\n-----END CERTIFICATE-----"),
    ).toBeNull();
  });

  it("derives a 43-character base64url x5t#S256 fingerprint", async () => {
    // SHA-256("hello") = 2cf24dba...b9824, base64url without padding.
    const fingerprint = await certificateFingerprint(HELLO_PEM);
    expect(fingerprint).toBe("LPJNul-wow4m6DsqxbninhsWHlwfp0JecwQzYpOLmCQ");
    expect(fingerprint).toHaveLength(43);
    expect(isCertificateFingerprint(fingerprint)).toBe(true);
  });

  it("rejects fingerprints that are not 43-character base64url", () => {
    expect(isCertificateFingerprint("A".repeat(42))).toBe(false);
    expect(isCertificateFingerprint("A".repeat(44))).toBe(false);
    expect(isCertificateFingerprint("A".repeat(42) + "+")).toBe(false);
  });

  it("encodes bytes with the url-safe alphabet and no padding", () => {
    expect(base64UrlFromBytes(new Uint8Array([0xfb, 0xff]))).toBe("-_8");
  });
});
