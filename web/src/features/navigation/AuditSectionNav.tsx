import { Segmented } from "antd";
import { useLocation, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

export function AuditSectionNav() {
  const { t } = useTranslation("resource");
  const location = useLocation();
  const navigate = useNavigate();

  const sectionItems = [
    { label: t("common:nav.loginLogs"), value: "/audit/login-logs" },
    { label: t("common:nav.operationLogs"), value: "/audit/operation-logs" },
    { label: t("common:nav.errorLogs"), value: "/audit/error-logs" },
    { label: t("common:nav.sessions"), value: "/sessions" },
    { label: t("common:nav.records"), value: "/records" },
    { label: t("common:nav.tokens"), value: "/tokens" },
    { label: t("common:nav.verifications"), value: "/verifications" },
  ];

  const activeSection =
    sectionItems.find(
      (item) => location.pathname === item.value || location.pathname.startsWith(`${item.value}/`),
    )?.value ?? "/sessions";

  return (
    <div className="gl-subnav gl-panel">
      <Segmented
        block
        options={sectionItems}
        value={activeSection}
        onChange={(value) => {
          navigate(String(value));
        }}
      />
    </div>
  );
}
