import { Tabs, Typography } from "antd";
import { useTranslation } from "react-i18next";
import { HelpLabel } from "@/shared/ui/FieldHelp";
import RiskSignalsTab from "./RiskSignalsTab";
import RiskDecisionsTab from "./RiskDecisionsTab";
import RiskSessionGovernanceTab from "./RiskSessionGovernanceTab";

export default function RiskPage() {
  const { t } = useTranslation("common");

  return (
    <div className="gl-resource-page">
      <div className="gl-panel gl-resource-card">
        <div className="gl-resource-card__header">
          <div>
            <Typography.Title level={2} style={{ margin: 0 }}>
              {t("nav.risk")}
            </Typography.Title>
            <Typography.Text type="secondary">{t("riskSubtitle")}</Typography.Text>
          </div>
        </div>
        <Tabs
          className="gl-tabs-card"
          defaultActiveKey="signals"
          items={[
            {
              key: "signals",
              label: (
                <HelpLabel helpKey="tabs.risk.signals">
                  {t("risk.signals", "Risk Signals")}
                </HelpLabel>
              ),
              children: <RiskSignalsTab />,
            },
            {
              key: "decisions",
              label: (
                <HelpLabel helpKey="tabs.risk.decisions">
                  {t("risk.decisions", "Decisions")}
                </HelpLabel>
              ),
              children: <RiskDecisionsTab />,
            },
            {
              key: "governance",
              label: (
                <HelpLabel helpKey="tabs.risk.sessionGovernance">
                  {t("risk.sessionGovernance", "Session Governance")}
                </HelpLabel>
              ),
              children: <RiskSessionGovernanceTab />,
            },
          ]}
        />
      </div>
    </div>
  );
}
