import { Tabs, Typography } from "antd";
import { useTranslation } from "react-i18next";
import { HelpLabel } from "@/shared/ui/FieldHelp";
import MeteringEventsTab from "./MeteringEventsTab";
import MeteringAggregatesTab from "./MeteringAggregatesTab";

export default function MeteringPage() {
  const { t } = useTranslation("common");

  return (
    <div className="gl-resource-page">
      <div className="gl-panel gl-resource-card">
        <div className="gl-resource-card__header">
          <div>
            <Typography.Title level={2} style={{ margin: 0 }}>
              {t("nav.metering")}
            </Typography.Title>
            <Typography.Text type="secondary">{t("meteringSubtitle")}</Typography.Text>
          </div>
        </div>
        <Tabs
          className="gl-tabs-card"
          defaultActiveKey="events"
          items={[
            {
              key: "events",
              label: (
                <HelpLabel helpKey="tabs.metering.events">
                  {t("nav.metering.events", "Metering Events")}
                </HelpLabel>
              ),
              children: <MeteringEventsTab />,
            },
            {
              key: "aggregates",
              label: (
                <HelpLabel helpKey="tabs.metering.aggregates">
                  {t("nav.metering.aggregates", "Aggregates")}
                </HelpLabel>
              ),
              children: <MeteringAggregatesTab />,
            },
          ]}
        />
      </div>
    </div>
  );
}
