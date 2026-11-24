import type { ApplicationClientCertificateInput } from "@/shared/api/iam/application";

const PEM_BLOCK_PATTERN = /-----BEGIN CERTIFICATE-----([\s\S]*?)-----END CERTIFICATE-----/;
const BASE64URL_PATTERN = /^[A-Za-z0-9_-]{43}$/;

/** Decode the base64 body of the first CERTIFICATE block into DER bytes. */
export function certificateDerFromPem(pem: string): ArrayBuffer | null {
  const match = PEM_BLOCK_PATTERN.exec(pem);
  if (!match) {
    return null;
  }
  const base64 = match[1].replace(/\s+/g, "");
  if (base64 === "") {
    return null;
  }
  try {
    const binary = atob(base64);
    const buffer = new ArrayBuffer(binary.length);
    const bytes = new Uint8Array(buffer);
    for (let index = 0; index < binary.length; index += 1) {
      bytes[index] = binary.charCodeAt(index);
    }
    return buffer;
  } catch {
    return null;
  }
}

/** RFC 8705 x5t#S256: base64url without padding, always 43 characters. */
export function base64UrlFromBytes(bytes: Uint8Array): string {
  let binary = "";
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

/**
 * Compute the certificate thumbprint in the browser. The server recomputes it
 * and rejects a mismatch, so this value is never trusted on its own; it exists
 * so the operator can see which certificate is about to be registered.
 */
export async function certificateFingerprint(pem: string): Promise<string> {
  const der = certificateDerFromPem(pem);
  if (!der) {
    throw new Error("invalidCertificatePem");
  }
  if (!globalThis.crypto?.subtle) {
    throw new Error("certificateFingerprintUnavailable");
  }
  const digest = await globalThis.crypto.subtle.digest("SHA-256", der);
  return base64UrlFromBytes(new Uint8Array(digest));
}

export function isCertificateFingerprint(value: string): boolean {
  return BASE64URL_PATTERN.test(value);
}

/** Turn the editor rows into API items; empty rows are dropped. */
export async function buildClientCertificateInputs(
  entries: Array<{ certificate?: string }> | undefined,
  selfSigned: boolean,
): Promise<ApplicationClientCertificateInput[]> {
  const items: ApplicationClientCertificateInput[] = [];
  for (const entry of entries ?? []) {
    const certificate = entry.certificate?.trim() ?? "";
    if (certificate === "") {
      continue;
    }
    items.push({
      certificate,
      fingerprintSha256: await certificateFingerprint(certificate),
      selfSigned,
    });
  }
  return items;
}
