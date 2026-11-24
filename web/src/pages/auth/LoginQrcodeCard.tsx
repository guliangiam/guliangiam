import { useEffect, useState } from "react";
import { QRCodeSVG } from "qrcode.react";
import { Alert, Button, Space, Spin, Typography } from "antd";
import { useTranslation } from "react-i18next";
import type { QrSigninState } from "@/shared/hooks/useQrSigninFlow";

const { Text } = Typography;

type QrSigninCardProps = {
  state: QrSigninState;
  onRefresh: () => void;
};

function formatCountdown(seconds: number) {
  const minutes = Math.floor(seconds / 60);
  return `${minutes}:${(seconds % 60).toString().padStart(2, "0")}`;
}

/**
 * 扫码登录卡片：只负责把 useQrSigninFlow 的状态渲染出来，不发起任何请求。
 * 二维码渲染的是服务端给的 `code` 载荷（不是 URL），倒计时用服务端 expiresIn。
 */
export default function QrSigninCard({ onRefresh, state }: QrSigninCardProps) {
  const { t } = useTranslation("auth");
  // 倒计时的初值来自服务端的 expiresIn；新会话由调用方以 key={state.code} 重新挂载
  // 本组件来重置（在 effect 里 setState 会触发级联渲染，且要额外维护重置时机）。
  const [countdown, setCountdown] = useState(state.expiresIn ?? 0);

  useEffect(() => {
    if (state.status !== "pending" && state.status !== "scanned") {
      return;
    }
    if (countdown <= 0) {
      return;
    }
    const timer = setInterval(() => setCountdown((current) => Math.max(current - 1, 0)), 1000);
    return () => clearInterval(timer);
  }, [countdown, state.status]);

  if (state.status === "failed") {
    return (
      <Space orientation="vertical" size={16} style={{ width: "100%" }}>
        <Alert
          showIcon
          type="error"
          title={t("login.qrcode.title")}
          description={state.error ?? t("loginFailed")}
        />
        <Button onClick={onRefresh}>{t("login.qrcode.refreshQrCode")}</Button>
      </Space>
    );
  }

  const expired =
    state.status === "expired" ||
    ((state.status === "pending" || state.status === "scanned") && countdown <= 0);

  if (expired) {
    return (
      <Space orientation="vertical" size={16} style={{ width: "100%" }}>
        <Alert
          showIcon
          type="warning"
          title={t("login.qrcode.title")}
          description={t("login.qrcode.expired")}
        />
        <Button type="primary" onClick={onRefresh}>
          {t("login.qrcode.refreshQrCode")}
        </Button>
      </Space>
    );
  }

  if (state.status === "confirmed") {
    return <Alert showIcon type="success" title={t("login.qrcode.success")} />;
  }

  if (state.status === "scanned") {
    return (
      <Space orientation="vertical" size={16} style={{ width: "100%" }}>
        <Alert
          showIcon
          type="info"
          title={t("login.qrcode.title")}
          description={t("login.qrcode.scanned")}
        />
      </Space>
    );
  }

  if (!state.code) {
    return (
      <Space orientation="vertical" size={16} style={{ width: "100%", alignItems: "center" }}>
        <Alert
          showIcon
          type="info"
          title={t("login.qrcode.title")}
          description={t("login.qrcode.scanPrompt")}
        />
        <Spin />
      </Space>
    );
  }

  return (
    <Space orientation="vertical" size={12} style={{ width: "100%" }}>
      <Alert
        showIcon
        type="info"
        title={t("login.qrcode.title")}
        description={t("login.qrcode.scanPrompt")}
      />
      <div style={{ display: "flex", justifyContent: "center" }}>
        <QRCodeSVG value={state.code} size={220} />
      </div>
      <Text type="secondary" style={{ textAlign: "center", display: "block" }}>
        {t("login.qrcode.expiresIn", { time: formatCountdown(countdown) })}
      </Text>
      <div style={{ display: "flex", justifyContent: "center" }}>
        <Button onClick={onRefresh}>{t("login.qrcode.refreshQrCode")}</Button>
      </div>
    </Space>
  );
}
