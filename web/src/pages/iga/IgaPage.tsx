import { Tabs, Typography } from "antd";
import { useTranslation } from "react-i18next";
import { HelpLabel } from "@/shared/ui/FieldHelp";
import IgaAccessCampaignsTab from "./IgaAccessCampaignsTab";
import IgaSODRulesTab from "./IgaSODRulesTab";
import IgaAccessLifecycleTab from "./IgaAccessLifecycleTab";
import IgaAccessRequestsTab from "./IgaAccessRequestsTab";
import IgaPrivilegedSessionsTab from "./IgaPrivilegedSessionsTab";

export default function IgaPage() {
  const { t: tc } = useTranslation("common");

  return (
    <div className="gl-resource-page">
      <div className="gl-panel gl-resource-card">
        <div className="gl-resource-card__header">
          <div>
            <Typography.Title level={2} style={{ margin: 0 }}>
              {tc("nav.iga.title")}
            </Typography.Title>
            <Typography.Text type="secondary">{tc("iga.description")}</Typography.Text>
          </div>
        </div>
        <Tabs
          className="gl-tabs-card"
          defaultActiveKey="requests"
          items={[
            {
              key: "requests",
              label: <HelpLabel helpKey="tabs.iga.requests">{tc("nav.iga.requests")}</HelpLabel>,
              children: <IgaAccessRequestsTab />,
            },
            {
              key: "campaigns",
              label: <HelpLabel helpKey="tabs.iga.campaigns">{tc("nav.iga.campaigns")}</HelpLabel>,
              children: <IgaAccessCampaignsTab />,
            },
            {
              key: "sod-rules",
              label: <HelpLabel helpKey="tabs.iga.sodRules">{tc("nav.iga.sodRules")}</HelpLabel>,
              children: <IgaSODRulesTab />,
            },
            {
              key: "lifecycle",
              label: <HelpLabel helpKey="tabs.iga.lifecycle">{tc("nav.iga.lifecycle")}</HelpLabel>,
              children: <IgaAccessLifecycleTab />,
            },
            {
              key: "privileged-sessions",
              label: (
                <HelpLabel helpKey="tabs.iga.privilegedSessions">
                  {tc("nav.iga.privilegedSessions")}
                </HelpLabel>
              ),
              children: <IgaPrivilegedSessionsTab />,
            },
          ]}
        />
      </div>
    </div>
  );
}
