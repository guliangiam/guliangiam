import { useCallback, useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router-dom";
import { Tabs, Typography } from "antd";
import { useSessionStore } from "@/features/session/session-store";
import { HelpLabel } from "@/shared/ui/FieldHelp";
import { PermissionGuard } from "@/shared/ui/PermissionGuard";
import { IdentityPoolsTab } from "./IdentityPoolsTab";
import { FederationTrustsTab } from "./FederationTrustsTab";
import { ServiceAccountsTab } from "./ServiceAccountsTab";

export default function WorkloadPage() {
  return (
    <PermissionGuard action="identity-pools:read">
      <WorkloadWorkspace />
    </PermissionGuard>
  );
}

function WorkloadWorkspace() {
  const { t: tc } = useTranslation("common");
  const organizationId = useSessionStore((state) => state.organization?.id ?? undefined);
  const [searchParams, setSearchParams] = useSearchParams();
  const previousOrganizationId = useRef(organizationId);
  const activePoolId = searchParams.get("poolId") || undefined;

  useEffect(() => {
    if (previousOrganizationId.current === organizationId) return;
    const hadOrganization = Boolean(previousOrganizationId.current);
    previousOrganizationId.current = organizationId;
    if (!hadOrganization) return;

    setSearchParams(
      (current) => {
        const next = new URLSearchParams(current);
        next.delete("poolId");
        return next;
      },
      { replace: true },
    );
  }, [organizationId, setSearchParams]);

  const selectPool = useCallback(
    (id: string) => {
      setSearchParams(
        (current) => {
          const next = new URLSearchParams(current);
          next.set("poolId", id);
          return next;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  const tabs = [
    {
      key: "pools",
      label: (
        <HelpLabel helpKey="tabs.workload.pools">
          {tc("nav.workload.pools", { defaultValue: "Identity Pools" })}
        </HelpLabel>
      ),
      children: <IdentityPoolsTab onSelectPool={selectPool} activePoolId={activePoolId} />,
    },
    {
      key: "trusts",
      label: (
        <HelpLabel helpKey="tabs.workload.trusts">
          {tc("nav.workload.trusts", { defaultValue: "Federation Trusts" })}
        </HelpLabel>
      ),
      children: <FederationTrustsTab key={activePoolId ?? "no-pool"} poolId={activePoolId} />,
    },
    {
      key: "serviceAccounts",
      label: (
        <HelpLabel helpKey="tabs.workload.serviceAccounts">
          {tc("nav.workload.serviceAccounts", { defaultValue: "Service Accounts" })}
        </HelpLabel>
      ),
      children: <ServiceAccountsTab key={activePoolId ?? "no-pool"} poolId={activePoolId} />,
    },
  ];

  return (
    <div className="gl-resource-page">
      <div className="gl-panel gl-resource-card">
        <div className="gl-resource-card__header">
          <div>
            <Typography.Title level={2} style={{ margin: 0 }}>
              {tc("workload.title", { defaultValue: "Workload Identity" })}
            </Typography.Title>
            <Typography.Text type="secondary">
              {tc("workload.description", {
                defaultValue: "Manage identity pools, federation trusts, and service accounts.",
              })}
            </Typography.Text>
          </div>
        </div>
        <Tabs className="gl-tabs-card" items={tabs} destroyOnHidden />
      </div>
    </div>
  );
}
