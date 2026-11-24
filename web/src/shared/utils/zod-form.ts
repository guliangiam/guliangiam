import type { ZodError, ZodIssue } from "zod";
import type { FormInstance } from "antd";

/**
 * Apply Zod validation errors to an Ant Design form instance.
 * Converts Zod issue paths to Ant Design field name arrays and
 * sets the errors on the form so they appear inline.
 *
 * `messageFor` lets a page replace the schema's English message with a
 * localized one without changing the schema contract itself.
 */
export function applyZodErrorsToForm<T extends Record<string, unknown>>(
  form: FormInstance<T>,
  error: ZodError,
  messageFor?: (issue: ZodIssue) => string | undefined,
) {
  const fields = error.issues.map((issue) => ({
    name: issue.path.length === 1 ? issue.path[0] : issue.path,
    errors: [messageFor?.(issue) ?? issue.message],
  }));
  form.setFields(fields as Parameters<typeof form.setFields>[0]);
}
