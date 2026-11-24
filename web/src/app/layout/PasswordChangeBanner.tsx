import Alert from "antd/es/alert";
import Button from "antd/es/button";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import { useSessionStore } from "@/features/session/session-store";

/**
 * 强制改密提示条。
 *
 * 后端 PasswordGate 默认即 enforce（AUTH_PASSWORD_CHANGE_ENFORCEMENT 未配置时
 * DefaultPasswordGateMode=enforce）：need_update_password 或密码超期时对非豁免
 * 路由硬拦截，返回 403 并携带 changePasswordUrl=/account。本提示条在拦截发生前
 * 给出主动引导，按钮跳 /account 与门禁 meta 同一目标，避免用户被 403 阻断却不知
 * 去向。会话标记由 user_credentials.need_update_password 派生（needUpdatePassword）。
 */
export function PasswordChangeBanner() {
  const { t } = useTranslation("common");
  const navigate = useNavigate();
  const needUpdatePassword = useSessionStore((state) => state.needUpdatePassword);

  if (!needUpdatePassword) {
    return null;
  }

  return (
    <Alert
      type="warning"
      showIcon
      style={{ marginBottom: "var(--gl-space-4)" }}
      title={t("auth:needUpdatePasswordBanner")}
      action={
        <Button size="small" onClick={() => navigate("/account")}>
          {t("changePassword", "Change Password")}
        </Button>
      }
    />
  );
}
