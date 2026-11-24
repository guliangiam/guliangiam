import { Alert, App, Button, Modal } from "antd";
import { QRCodeSVG } from "qrcode.react";
import { useTranslation } from "react-i18next";

type IdvVerifyQrModalProps = {
  open: boolean;
  /** 厂商托管页地址：桌面操作员无法自拍，交给本人手机扫码完成刷脸。 */
  verifyUrl: string;
  onClose: () => void;
};

/**
 * 管理端发起实名认证后的扫码弹窗。
 *
 * 操作员在桌面点「发起实名认证」时，浏览器采集的是操作员自己的 metaInfo
 * （deviceType=web），而刷脸必须由被管理用户本人在手机上完成，因此不能像自服务
 * 页那样 `location.assign` 把操作员甩走：改为展示托管页二维码，用户扫码后在本机
 * 完成活体，页面靠 useIdvPendingPolling 轮询结果收尾。
 */
export default function IdvVerifyQrModal({ open, verifyUrl, onClose }: IdvVerifyQrModalProps) {
  const { message } = App.useApp();
  const { t } = useTranslation();
  // 非安全上下文（如 http://内网 IP）下 navigator.clipboard 不存在：只隐藏复制入口，
  // 扫码本身仍然可用，不给用户一个必然失败的按钮。
  const canCopy = typeof navigator !== "undefined" && !!navigator.clipboard;

  const copyLink = async () => {
    try {
      await navigator.clipboard.writeText(verifyUrl);
      message.success(t("common:copySuccess"));
    } catch {
      message.error(t("common:copyFailed"));
    }
  };

  return (
    <Modal
      title={t("verifyIdQrTitle")}
      open={open}
      onCancel={onClose}
      footer={<Button onClick={onClose}>{t("common:close")}</Button>}
    >
      <div style={{ display: "flex", flexDirection: "column", gap: 12, alignItems: "center" }}>
        <Alert
          showIcon
          type="info"
          title={t("verifyIdQrHint")}
          style={{ width: "100%", alignItems: "center" }}
        />
        <QRCodeSVG value={verifyUrl} size={220} />
        {canCopy ? <Button onClick={() => void copyLink()}>{t("verifyIdCopyLink")}</Button> : null}
      </div>
    </Modal>
  );
}
