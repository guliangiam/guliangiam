import { Button, Space, Skeleton, Typography } from "antd";
import type { ApplicationDeleteSummary } from "@/shared/api/iam/application";

/** 导出下载：设计口径文件名 {name}.json（applications.md 导出节），列表与详情共用。 */
export function downloadApplicationExport(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `${name}.json`;
  a.click();
  URL.revokeObjectURL(url);
}

/**
 * Format a datetime string for display
 */
export function formatDateTime(value: string) {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }

  return parsed.toLocaleString();
}

/**
 * Deduplicate and clean up a string list
 */
export function normalizeStringList(values?: string[]) {
  if (!Array.isArray(values)) {
    return [] as string[];
  }

  const seen = new Set<string>();
  return values
    .map((value) => value.trim())
    .filter((value) => value !== "")
    .filter((value) => {
      if (seen.has(value)) {
        return false;
      }
      seen.add(value);
      return true;
    });
}

/**
 * Generate a slug: lowercase, hyphen-separated
 */
export function normalizeSlug(value?: string) {
  return (value ?? "")
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .replace(/-{2,}/g, "-");
}

/**
 * Format the description text for the delete confirmation popover
 */
export function formatDeleteDescription(
  tr: (key: string, options?: Record<string, unknown>) => string,
  slugOrName: string,
  summary?: Pick<
    ApplicationDeleteSummary,
    "providerBindingCount" | "activeTokenCount" | "activeConsentCount"
  >,
  options?: { isLoading?: boolean; hasError?: boolean; onRetry?: () => void },
) {
  if (options?.isLoading) {
    return (
      <Space size="small">
        <Skeleton active paragraph={{ rows: 1 }} title={false} style={{ width: 120 }} />
        <Typography.Text>
          {tr("application.deletingImpactLoading", { name: slugOrName })}
        </Typography.Text>
      </Space>
    );
  }

  if (options?.hasError) {
    return (
      <Space orientation="vertical" size={4}>
        <Typography.Text type="danger">
          {tr("application.deletingImpactError", { name: slugOrName })}
        </Typography.Text>
        <Button
          type="link"
          size="small"
          style={{ padding: 0, alignSelf: "flex-start" }}
          onClick={options.onRetry}
        >
          {tr("application.retryLoading")}
        </Button>
      </Space>
    );
  }

  if (!summary) {
    return tr("application.deletingImpactNotReady", { name: slugOrName });
  }

  const dependencies: string[] = [];

  if (summary.providerBindingCount > 0) {
    dependencies.push(
      tr("application.providerBindingCount", { count: summary.providerBindingCount }),
    );
  }
  if (summary.activeTokenCount > 0) {
    dependencies.push(tr("application.activeTokenCount", { count: summary.activeTokenCount }));
  }
  if (summary.activeConsentCount > 0) {
    dependencies.push(tr("application.activeConsentCount", { count: summary.activeConsentCount }));
  }

  if (dependencies.length === 0) {
    return tr("application.deletingImpactClean", { name: slugOrName });
  }

  return tr("application.deletingImpactWithDeps", {
    name: slugOrName,
    dependencies: dependencies.join(", "),
  });
}

/**
 * Format the Token Endpoint Auth Method display value with i18n
 */
export function formatTokenEndpointAuthMethod(
  value: string,
  tr?: (key: string, fallback: string) => string,
) {
  switch (value) {
    case "client_secret_basic":
      return tr
        ? tr("application.tokenEndpointAuthMethodOption.clientSecretBasic", "client_secret_basic")
        : "client_secret_basic";
    case "client_secret_post":
      return tr
        ? tr("application.tokenEndpointAuthMethodOption.clientSecretPost", "client_secret_post")
        : "client_secret_post";
    default:
      return tr
        ? tr("application.tokenEndpointAuthMethodOption.none", "Public client (none)")
        : "none";
  }
}

/**
 * Deep equality over heterogeneous values.
 *
 * Used to compare config/provider-bindings snapshots against the initial
 * baseline so the form's "dirty" state reflects an actual change rather
 * than a no-op re-render. `Record<string, unknown>` is intentional here:
 * the snapshots are the output of normalize helpers whose shape we don't
 * want to lock in the helper signature.
 */
export function deepEqual(a: unknown, b: unknown): boolean {
  if (a === b) return true;
  if (a === null || a === undefined || b === null || b === undefined) return false;
  if (typeof a !== typeof b) return false;
  if (typeof a !== "object") return false;
  if (Array.isArray(a) !== Array.isArray(b)) return false;
  if (Array.isArray(a)) {
    if (a.length !== (b as unknown[]).length) return false;
    return a.every((item, i) => deepEqual(item, (b as unknown[])[i]));
  }
  const objA = a as Record<string, unknown>;
  const objB = b as Record<string, unknown>;
  const keysA = Object.keys(objA);
  const keysB = Object.keys(objB);
  if (keysA.length !== keysB.length) return false;
  for (const key of keysA) {
    if (!deepEqual(objA[key], objB[key])) return false;
  }
  return true;
}
