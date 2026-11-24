import { describe, it, expect, vi } from "vitest";
import { applyZodErrorsToForm } from "./zod-form";
import { z } from "zod";
import type { FormInstance } from "antd";

// Mock antd FormInstance
function createMockForm() {
  const fields: Array<{ name: unknown; errors: string[] }> = [];
  return {
    setFields: vi.fn<(...args: unknown[]) => unknown>(
      (input: Array<{ name: unknown; errors: string[] }>) => {
        fields.push(...input);
      },
    ),
    getFields: () => fields,
  } as unknown as FormInstance<Record<string, unknown>>;
}

describe("applyZodErrorsToForm", () => {
  it("applies single field error to form", () => {
    const schema = z.object({
      username: z.string().min(1, "Username is required"),
    });
    const form = createMockForm();
    const result = schema.safeParse({ username: "" });

    if (result.success) {
      throw new Error("expected schema validation to fail");
    }
    applyZodErrorsToForm(form, result.error);
    expect(form.setFields).toHaveBeenCalled();
    const calledFields = (form.setFields as ReturnType<typeof vi.fn>).mock.calls[0][0];
    expect(calledFields).toHaveLength(1);
    expect(calledFields[0].errors).toContain("Username is required");
  });

  it("applies multiple field errors to form", () => {
    const schema = z.object({
      username: z.string().min(1, "Username is required"),
      email: z.string().email("Invalid email"),
    });
    const form = createMockForm();
    const result = schema.safeParse({ username: "", email: "not-email" });

    if (result.success) {
      throw new Error("expected schema validation to fail");
    }
    applyZodErrorsToForm(form, result.error);
    expect(form.setFields).toHaveBeenCalled();
    const calledFields = (form.setFields as ReturnType<typeof vi.fn>).mock.calls[0][0];
    expect(calledFields.length).toBeGreaterThanOrEqual(2);
  });

  it("handles nested path errors", () => {
    const schema = z.object({
      profile: z.object({
        displayName: z.string().min(1, "Display name is required"),
      }),
    });
    const form = createMockForm();
    const result = schema.safeParse({ profile: { displayName: "" } });

    if (result.success) {
      throw new Error("expected schema validation to fail");
    }
    applyZodErrorsToForm(form, result.error);
    expect(form.setFields).toHaveBeenCalled();
    const calledFields = (form.setFields as ReturnType<typeof vi.fn>).mock.calls[0][0];
    // Nested paths should be arrays
    expect(calledFields[0].name).toBeDefined();
  });

  it("handles single-element path as scalar name", () => {
    const schema = z.object({
      name: z.string().min(1, "Name is required"),
    });
    const form = createMockForm();
    const result = schema.safeParse({ name: "" });

    if (result.success) {
      throw new Error("expected schema validation to fail");
    }
    applyZodErrorsToForm(form, result.error);
    const calledFields = (form.setFields as ReturnType<typeof vi.fn>).mock.calls[0][0];
    // Single path element should be the string itself, not an array
    expect(calledFields[0].name).toBe("name");
  });

  it("handles multi-element path as array name", () => {
    const schema = z.object({
      profile: z.object({
        email: z.string().email("Invalid email"),
      }),
    });
    const form = createMockForm();
    const result = schema.safeParse({ profile: { email: "bad" } });

    if (result.success) {
      throw new Error("expected schema validation to fail");
    }
    applyZodErrorsToForm(form, result.error);
    const calledFields = (form.setFields as ReturnType<typeof vi.fn>).mock.calls[0][0];
    // Multi-element path should be an array
    expect(Array.isArray(calledFields[0].name)).toBe(true);
  });
});
