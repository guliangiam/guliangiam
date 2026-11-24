import { useTranslation } from "react-i18next";
import ManagedAccountTable from "@/shared/ui/ManagedAccountTable";
import type { ManagedAccountItem } from "@/shared/api/iam/user";

interface ManagedAccountsTabProps {
  managedAccounts: ManagedAccountItem[];
  onChange: (value: ManagedAccountItem[]) => void;
  /** users:write; without it the table stays read-only. */
  canWrite?: boolean;
}

/**
 * Managed accounts are persisted through `PATCH /users/{id}/collections`
 * together with the other JSONB collections; secrets are only submitted when the
 * operator types a new one, so keeping the stored password is the default.
 */
export default function ManagedAccountsTab({
  managedAccounts,
  onChange,
  canWrite = false,
}: ManagedAccountsTabProps) {
  const { t } = useTranslation("common");
  return (
    <>
      <ManagedAccountTable value={managedAccounts} onChange={onChange} readOnly={!canWrite} />
      {managedAccounts.length === 0 && (
        <span className="ant-typography ant-typography-secondary">{t("noData", "No data")}</span>
      )}
    </>
  );
}
