import { Tabs, Typography } from "antd";
import { useTranslation } from "react-i18next";
import { HelpLabel } from "@/shared/ui/FieldHelp";
import RebacNamespacesTab from "./RebacNamespacesTab";
import RebacTuplesTab from "./RebacTuplesTab";

export default function RebacPage() {
  const { t } = useTranslation("common");

  return (
    <div className="gl-resource-page">
      <div className="gl-panel gl-resource-card">
        <div className="gl-resource-card__header">
          <div>
            <Typography.Title level={2} style={{ margin: 0 }}>
              {t("rebac.title")}
            </Typography.Title>
            <Typography.Text type="secondary">{t("rebac.description")}</Typography.Text>
          </div>
        </div>
        <Tabs
          className="gl-tabs-card"
          defaultActiveKey="namespaces"
          items={[
            {
              key: "namespaces",
              label: (
                <HelpLabel helpKey="tabs.rebac.namespaces">{t("rebac.namespaces.tab")}</HelpLabel>
              ),
              children: <RebacNamespacesTab />,
            },
            {
              key: "tuples",
              label: <HelpLabel helpKey="tabs.rebac.tuples">{t("rebac.tuples.tab")}</HelpLabel>,
              children: <RebacTuplesTab />,
            },
          ]}
        />
      </div>
    </div>
  );
}
