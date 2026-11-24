import Modal from "antd/es/modal";
import { useTranslation } from "react-i18next";
import { useSessionTimeout } from "@/shared/hooks/useSessionTimeout";

export function SessionTimeoutDialog() {
  const { t } = useTranslation();
  const { state, extendSession, expireNow, minutesRemaining } = useSessionTimeout();

  if (state !== "warning") return null;

  return (
    <Modal
      open
      closable={false}
      /* WCAG 2.4.3 Focus Order — ensure focus trap is active; keyboard=true (default) allows Esc (C-05/K-01) */
      keyboard
      title={t("sessionWarningTitle")}
      okText={t("extendSession")}
      cancelText={t("signOut")}
      onOk={extendSession}
      onCancel={expireNow}
      mask={{ closable: false }}
      width={420}
    >
      <p>{t("sessionWarningMessage", { minutes: minutesRemaining })}</p>
    </Modal>
  );
}
