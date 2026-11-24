import { Tabs, Typography } from "antd";
import { useTranslation } from "react-i18next";
import { HelpLabel } from "@/shared/ui/FieldHelp";
import ActionListTab from "./ActionListTab";
import ActionVersionsTab from "./ActionVersionsTab";
import ActionLogsTab from "./ActionLogsTab";

export default function ActionsPage() {
  const { t } = useTranslation("common");

  return (
    <div className="gl-resource-page">
      <div className="gl-panel gl-resource-card">
        <div className="gl-resource-card__header">
          <div>
            <Typography.Title level={2} style={{ margin: 0 }}>
              {t("identityActions.title")}
            </Typography.Title>
            <Typography.Text type="secondary">{t("identityActions.description")}</Typography.Text>
          </div>
        </div>
        <Tabs
          className="gl-tabs-card"
          defaultActiveKey="list"
          items={[
            {
              key: "list",
              label: (
                <HelpLabel helpKey="tabs.actions.list">
                  {t("nav.actions.list", "Identity Actions")}
                </HelpLabel>
              ),
              children: <ActionListTab />,
            },
            {
              key: "versions",
              label: (
                <HelpLabel helpKey="tabs.actions.versions">
                  {t("nav.actions.versions", "Versions")}
                </HelpLabel>
              ),
              children: <ActionVersionsTab />,
            },
            {
              key: "logs",
              label: (
                <HelpLabel helpKey="tabs.actions.executionLogs">
                  {t("nav.actions.executionLogs", "Execution Logs")}
                </HelpLabel>
              ),
              children: <ActionLogsTab />,
            },
          ]}
        />
      </div>
    </div>
  );
}
