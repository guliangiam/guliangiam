import { expect } from "vitest";

/**
 * Asserts that a discriminated union value carries the expected `kind` and
 * narrows its type for the remainder of the test body.
 *
 * An assertion function keeps `expect` out of conditional blocks
 * (`vitest/no-conditional-expect`) while preserving precise types, and it fails
 * loudly instead of silently skipping assertions when the discriminant is wrong.
 */
export function expectKind<T extends { kind: string }, K extends T["kind"]>(
  value: T,
  kind: K,
): asserts value is Extract<T, { kind: K }> {
  expect(value.kind, `expected kind ${kind}, got ${value.kind}`).toBe(kind);
  if (value.kind !== kind) {
    throw new Error(`expected kind ${kind}, got ${value.kind}`);
  }
}

/**
 * Companion to {@link expectKind} for values discriminated by `status`.
 */
export function expectStatus<T extends { status: string }, S extends T["status"]>(
  value: T,
  status: S,
): asserts value is Extract<T, { status: S }> {
  expect(value.status, `expected status ${status}, got ${value.status}`).toBe(status);
  if (value.status !== status) {
    throw new Error(`expected status ${status}, got ${value.status}`);
  }
}
