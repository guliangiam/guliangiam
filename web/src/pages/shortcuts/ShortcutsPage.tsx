import { Card, Col, Row, Typography } from "antd";
import {
  AppstoreOutlined,
  PlusOutlined,
  SafetyCertificateOutlined,
  SearchOutlined,
  SendOutlined,
  SettingOutlined,
  TeamOutlined,
  UserOutlined,
} from "@ant-design/icons";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";

const { Title, Paragraph } = Typography;

type ShortcutItem = {
  key: string;
  titleKey: string;
  descKey: string;
  icon: React.ReactNode;
  path: string;
  color: string;
};

const shortcuts: ShortcutItem[] = [
  {
    key: "create-user",
    titleKey: "shortcuts.createUser",
    descKey: "shortcuts.createUserDesc",
    icon: <UserOutlined />,
    path: "/users/new",
    color: "var(--gl-color-primary)",
  },
  {
    key: "create-application",
    titleKey: "shortcuts.createApplication",
    descKey: "shortcuts.createApplicationDesc",
    icon: <AppstoreOutlined />,
    path: "/applications/new",
    color: "var(--gl-color-success)",
  },
  {
    key: "configure-sso",
    titleKey: "shortcuts.configureSso",
    descKey: "shortcuts.configureSsoDesc",
    icon: <SafetyCertificateOutlined />,
    path: "/providers",
    color: "#722ed1",
  },
  {
    key: "manage-organization",
    titleKey: "shortcuts.manageOrganization",
    descKey: "shortcuts.manageOrganizationDesc",
    icon: <TeamOutlined />,
    path: "/organizations",
    color: "#fa8c16",
  },
  {
    key: "ldap-sync",
    titleKey: "shortcuts.ldapSync",
    descKey: "shortcuts.ldapSyncDesc",
    icon: <SearchOutlined />,
    path: "/ldaps",
    color: "#13c2c2",
  },
  {
    key: "view-sessions",
    titleKey: "shortcuts.viewSessions",
    descKey: "shortcuts.viewSessionsDesc",
    icon: <SendOutlined />,
    path: "/sessions",
    color: "#eb2f96",
  },
  {
    key: "edit-permissions",
    titleKey: "shortcuts.editPermissions",
    descKey: "shortcuts.editPermissionsDesc",
    icon: <PlusOutlined />,
    path: "/permissions",
    color: "#f5222d",
  },
  {
    key: "system-settings",
    titleKey: "shortcuts.systemSettings",
    descKey: "shortcuts.systemSettingsDesc",
    icon: <SettingOutlined />,
    path: "/system-info",
    color: "var(--gl-color-text-secondary)",
  },
];

export default function ShortcutsPage() {
  const { t } = useTranslation("common");

  return (
    <div className="gl-page">
      <div style={{ marginBottom: 24 }}>
        <Title level={3} style={{ margin: 0 }}>
          {t("shortcuts.title")}
        </Title>
        <Paragraph type="secondary">{t("shortcuts.subtitle")}</Paragraph>
      </div>
      <Row gutter={[16, 16]}>
        {shortcuts.map((item) => (
          <Col key={item.key} xs={24} sm={12} md={8} lg={6}>
            {/* WCAG 2.1.1 / 4.1.2 — a real anchor keeps native keyboard and
                assistive-technology semantics (C-02). 不要另外加 aria-label：
                卡片内有标题与描述两段可见文本，显式名称只含标题会触发
                label-content-name-mismatch。 */}
            <Link to={item.path} style={{ display: "block", color: "inherit" }}>
              <Card hoverable style={{ cursor: "pointer" }}>
                <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
                  <div
                    style={{
                      width: 40,
                      height: 40,
                      borderRadius: 8,
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      backgroundColor: `color-mix(in srgb, ${item.color} 8%, transparent)`,
                      color: item.color,
                      fontSize: 20,
                      flexShrink: 0,
                    }}
                  >
                    {item.icon}
                  </div>
                  <div style={{ minWidth: 0 }}>
                    <div style={{ fontWeight: 600 }}>{t(item.titleKey)}</div>
                    <div
                      style={{
                        fontSize: 12,
                        color: "var(--ant-color-text-secondary)",
                        whiteSpace: "nowrap",
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                      }}
                    >
                      {t(item.descKey)}
                    </div>
                  </div>
                </div>
              </Card>
            </Link>
          </Col>
        ))}
      </Row>
    </div>
  );
}
