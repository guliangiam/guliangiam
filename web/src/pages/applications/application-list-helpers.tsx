import { Button, Skeleton, Space, Typography } from "antd";
import type { ApplicationDeleteSummary } from "@/shared/api/iam/application";

export const CATEGORY_COLORS: Record<string, string> = {
  identity: "blue",
  web: "cyan",
  spa: "purple",
  native: "orange",
  machine: "gold",
  service: "green",
  agent: "magenta",
};

export const CATEGORY_VALUES = [
  "identity",
  "web",
  "spa",
  "native",
  "machine",
  "service",
  "agent",
] as const;

export const APP_STATUS_VALUES = ["active", "inactive"] as const;

export type ApplicationRow = {
  id?: string;
  name: string;
  displayName?: string;
  category?: string;
  organizationId?: string;
  clientId?: string;
  redirectUris?: string[] | null;
  providers?: string[] | null;
  status?: string;
  createdAt?: string;
};

export function renderDeleteDescription(
  applicationName: string,
  options: {
    summary?: ApplicationDeleteSummary;
    isLoading: boolean;
    hasError: boolean;
    onRetry?: () => void;
  },
  tr: (key: string, fallback: string) => string,
) {
  if (options.isLoading) {
    return (
      <Space size="small">
        <Skeleton active paragraph={{ rows: 1 }} title={false} style={{ width: 120 }} />
        <Typography.Text>{`${applicationName}. ${tr("application.deleteImpactLoading", "Loading delete impact...")}`}</Typography.Text>
      </Space>
    );
  }

  if (options.hasError) {
    return (
      <Space orientation="vertical" size={4}>
        <Typography.Text type="danger">{`${applicationName}. ${tr("application.deleteImpactFailed", "Failed to load delete impact")}.`}</Typography.Text>
        <Button
          type="link"
          size="small"
          style={{ padding: 0, alignSelf: "flex-start" }}
          onClick={options.onRetry}
        >
          {tr("application.retryLoad", "Retry loading")}
        </Button>
      </Space>
    );
  }

  const summary = options.summary;
  if (!summary) {
    return `${applicationName}. ${tr("application.deleteImpactNotReady", "Delete impact not ready yet")}.`;
  }

  const dependencies: string[] = [];
  if (summary.providerBindingCount > 0)
    dependencies.push(
      `${tr("application.providerBindings", "Provider Bindings")} ${summary.providerBindingCount}`,
    );
  if (summary.activeTokenCount > 0)
    dependencies.push(
      `${tr("application.activeTokens", "Active Tokens")} ${summary.activeTokenCount}`,
    );
  if (summary.activeConsentCount > 0)
    dependencies.push(
      `${tr("application.activeConsents", "Active Authorization Consents")} ${summary.activeConsentCount}`,
    );

  if (dependencies.length === 0) {
    return `${applicationName}. ${tr("application.deleteIrreversible", "Deletion is irreversible")}.`;
  }

  return `${applicationName}. ${tr("application.deleteWillClean", "Deletion will also clean up")} ${dependencies.join(", ")}.`;
}
