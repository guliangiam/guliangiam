/**
 * Security utilities for URL validation.
 *
 * Prevents Open Redirect attacks by validating redirect URLs
 * against a strict allowlist of same-origin patterns.
 */

/**
 * Validates whether a redirect URL is safe (same-origin only).
 *
 * Safe patterns:
 * - Same-origin path: `/foo/bar` (starts with `/` but NOT `//`)
 * - Same-origin full URL: `https://current-host/foo`
 *
 * Rejected patterns:
 * - Protocol-relative: `//evil.com`
 * - JavaScript protocol: `javascript:...`
 * - Data protocol: `data:...`
 * - Any external origin
 *
 * @param url - The redirect URL to validate
 * @returns `true` if the URL is safe for redirect, `false` otherwise
 */
export function isSafeRedirectUrl(url: string): boolean {
  if (!url || typeof url !== "string") {
    return false;
  }

  const trimmed = url.trim();

  // Reject empty strings
  if (trimmed === "") {
    return false;
  }

  // Reject protocol-relative URLs (e.g. //evil.com)
  if (trimmed.startsWith("//")) {
    return false;
  }

  // Reject javascript: and data: protocols (case-insensitive)
  const lower = trimmed.toLowerCase();
  if (lower.startsWith("javascript:") || lower.startsWith("data:")) {
    return false;
  }

  // Allow same-origin path references (starts with / but not //)
  if (trimmed.startsWith("/")) {
    return true;
  }

  // Allow same-origin full URLs
  try {
    const parsed = new URL(trimmed, window.location.origin);
    return parsed.origin === window.location.origin;
  } catch {
    return false;
  }
}

/**
 * Returns a safe redirect URL. If the provided URL is not safe,
 * falls back to the given default path.
 *
 * @param url - The redirect URL to validate
 * @param fallback - The fallback path if URL is unsafe (default: "/")
 * @returns The original URL if safe, otherwise the fallback
 */
export function safeRedirectUrl(url: string, fallback: string = "/"): string {
  return isSafeRedirectUrl(url) ? url : fallback;
}

/**
 * Validates whether an external URL is safe to open (http/https only).
 *
 * Prevents XSS via javascript:, data:, vbscript: and other dangerous protocols.
 * Only absolute URLs with http: or https: scheme are considered safe.
 *
 * @param url - The URL to validate
 * @returns `true` if the URL uses http or https protocol, `false` otherwise
 */
export function isSafeExternalUrl(url: string): boolean {
  if (!url || typeof url !== "string") {
    return false;
  }

  const trimmed = url.trim();
  if (trimmed === "") {
    return false;
  }

  try {
    const parsed = new URL(trimmed);
    return parsed.protocol === "http:" || parsed.protocol === "https:";
  } catch {
    return false;
  }
}
