import { Alert } from "antd";
import { memo, type ReactNode } from "react";

type ErrorAlertProps = {
  /** Main error title */
  title?: string;
  /** Detailed error description */
  description?: ReactNode;
  /** Error category for styling */
  variant?: "auth" | "network" | "mfa" | "account" | "generic";
  /** Whether the alert can be closed */
  closable?: boolean;
  /** Close callback */
  onClose?: () => void;
};

const variantConfig: Record<string, { type: "error" | "warning" | "info" }> = {
  auth: { type: "error" },
  network: { type: "warning" },
  mfa: { type: "info" },
  account: { type: "error" },
  generic: { type: "error" },
};

export const ErrorAlert = memo(function ErrorAlert({
  title,
  description,
  variant = "generic",
  closable = false,
  onClose,
}: ErrorAlertProps) {
  if (!title && !description) return null;

  const config = variantConfig[variant] || variantConfig.generic;

  return (
    <Alert
      type={config.type}
      showIcon
      closable={closable}
      onClose={onClose}
      title={title}
      description={description}
      style={{ marginBottom: 16 }}
    />
  );
});
