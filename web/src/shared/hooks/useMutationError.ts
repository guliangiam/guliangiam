/**
 * Unified mutation error handler.
 *
 * Checks for special HTTP error codes (401/403/404) that should be handled
 * by the global handler in AppProviders. For other errors, displays the
 * provided fallback message or the error's own message.
 *
 * Usage in mutations:
 *   onError: (error) => handleMutationError(error, t('common.saveFailed'))
 */
import { getErrorDetails, isHttpError, type HttpError } from "@/shared/api/http";
import { appMessage } from "@/shared/lib/app-message";

/**
 * Build a field-level explanation from the 422 `error.details` payload.
 *
 * The backend puts structured detail there (`{field, value, max}` for a
 * rejected pageSize, `{field, value, allowed}` for an unknown sortField). The
 * prose in `message` is accurate but not actionable — "sortField must be one of:
 * createdAt, …" leaves the reader to work out which request field that is.
 */
function describeUnprocessable(entity: HttpError): string | undefined {
  const details = getErrorDetails(entity);
  if (!details) return undefined;
  const field = typeof details.field === "string" ? details.field : undefined;
  if (!field) return undefined;

  if (Array.isArray(details.allowed) && details.allowed.length > 0) {
    return `${field}: ${String(details.value ?? "")} is not allowed (expected one of ${details.allowed.join(", ")})`;
  }
  if (typeof details.max === "number") {
    return `${field}: ${String(details.value ?? "")} is out of range (max ${details.max})`;
  }
  return `${field}: ${entity.message}`;
}

/**
 * Handle a mutation error with a localized fallback message.
 *
 * - 401: Silently ignored (redirect handled by http.ts)
 * - 403: Shows "forbidden" message
 * - 404: Shows "resource not found" message
 * - 422: Shows the offending field from `error.details` when the server sent one
 * - Other errors: Shows the fallback message, or error.message if available
 */
export function handleMutationError(error: unknown, fallbackMessage?: string): void {
  if (isHttpError(error)) {
    switch (error.status) {
      case 401:
        // Auth error - redirect handled by http.ts
        return;
      case 403:
        appMessage().error(error.message || "Forbidden");
        return;
      case 404:
        appMessage().error(error.message || "Resource not found");
        return;
      case 409:
        appMessage().error(error.message || "Conflict: data was modified by another user");
        return;
      case 422:
        appMessage().error(describeUnprocessable(error) || error.message || "Request rejected");
        return;
      default:
        appMessage().error(error.message || fallbackMessage || "Operation failed");
        return;
    }
  }

  if (error instanceof Error) {
    appMessage().error(error.message || fallbackMessage || "Operation failed");
    return;
  }

  appMessage().error(fallbackMessage || "Operation failed");
}
