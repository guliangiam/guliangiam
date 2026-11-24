import { Card, Skeleton } from "antd";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { useSessionStore } from "@/features/session/session-store";
import { SectionNavShell } from "@/features/section-nav/SectionNavShell";
import type { SectionNavItem } from "@/features/section-nav/types";
import { queryKeys } from "@/shared/api/query-keys";
import { getOrganization } from "@/shared/api/iam";
import { GeneralSettingsTab } from "./tabs/GeneralSettingsTab";
import { ThemeSettingsTab } from "./tabs/ThemeSettingsTab";
import { SecurityPolicyTab } from "./tabs/SecurityPolicyTab";

export default function SettingsPage() {
  const { t } = useTranslation("resource");
  const queryClient = useQueryClient();
  const organization = useSessionStore((s) => s.organization);
  const orgId = organization?.id;

  const orgQuery = useQuery({
    // 与 invalidateDetail 的 queryKeys.organizations.detail 同键，否则保存后
    // 各 tab 共享的 settings/theme props 不刷新。
    queryKey: queryKeys.organizations.detail(orgId ?? ""),
    queryFn: () => getOrganization(orgId!),
    enabled: !!orgId,
  });

  if (!orgId) {
    return (
      <div className="gl-content__inner--reading">
        <Card className="gl-panel" variant="borderless">
          <p>{t("settings.noOrg")}</p>
        </Card>
      </div>
    );
  }

  if (orgQuery.isLoading) {
    return (
      <div className="gl-content__inner--reading">
        <Card className="gl-panel" variant="borderless">
          <Skeleton active paragraph={{ rows: 8 }} />
        </Card>
      </div>
    );
  }

  const settings = orgQuery.data?.settings ?? {};
  const theme = orgQuery.data?.theme ?? {};

  const invalidateDetail = () => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.organizations.detail(orgId) });
  };

  const sectionItems: SectionNavItem[] = [
    { key: "general", anchor: "settings-section-general", label: t("settings.generalTab") },
    { key: "theme", anchor: "settings-section-theme", label: t("settings.themeTab") },
    { key: "security", anchor: "settings-section-security", label: t("settings.securityTab") },
  ];

  return (
    <div className="gl-page gl-content__inner--reading">
      <h2>{t("settings.title")}</h2>
      <p style={{ color: "var(--ant-color-text-secondary)", marginBottom: 24 }}>
        {t("settings.description")}
      </p>
      <SectionNavShell items={sectionItems}>
        <div className="gl-section-anchors">
          <div id="settings-section-general" className="gl-section-anchor">
            <GeneralSettingsTab
              orgId={orgId}
              settings={settings}
              revision={orgQuery.data?.revision ?? 1}
              onSuccess={invalidateDetail}
            />
          </div>
          <div id="settings-section-theme" className="gl-section-anchor">
            <ThemeSettingsTab
              orgId={orgId}
              theme={theme}
              revision={orgQuery.data?.revision ?? 1}
              onSuccess={invalidateDetail}
            />
          </div>
          <div id="settings-section-security" className="gl-section-anchor">
            <SecurityPolicyTab
              orgId={orgId}
              settings={settings}
              revision={orgQuery.data?.revision ?? 1}
              onSuccess={invalidateDetail}
            />
          </div>
        </div>
      </SectionNavShell>
    </div>
  );
}
