import { useState } from "react";
import { Alert, App, Button, Image, Input, Modal, Space, Tag, Typography } from "antd";
import { LoadingOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import { generateCaptcha, verifyCaptcha } from "@/shared/api/iam/auth";
import { isHttpError } from "@/shared/api/http";
import { localizedError, serverErrorMessage } from "@/shared/lib/server-error";
import { GoCaptchaInteraction } from "@/shared/ui/GoCaptchaInteraction";

const { Text } = Typography;

type CaptchaTestModalProps = {
  /** Provider id — 用于显示标识，实际测试走 organizationId 范围的 auth/captcha 端点。 */
  providerId: string;
  /** Provider type (e.g. "recaptcha", "hcaptcha", "turnstile")。 */
  providerType: string;
  /** Organization id — 传递给 auth/captcha 运行时端点。 */
  organizationId?: string;
  open: boolean;
  onClose: () => void;
};

/**
 * Captcha provider 专用测试模态框。
 *
 * 后端统一 /api/v1/providers/{id}/test-connection 对 captcha delegated kinds
 * (recaptcha, hcaptcha, turnstile, geetest, aliyun_captcha, recaptcha_v3)
 * 返回 422，因此统一详情页不能复用 TestConnectionButton。此组件改用
 * /api/v1/auth/captcha + /api/v1/auth/captcha/verify 运行时端点，
 * 在目标 organization 范围内生成并校验验证码。
 */
export function CaptchaTestModal(props: CaptchaTestModalProps) {
  // 切换 provider 时整体重挂载，避免上一次测试的图片/结果残留。
  return <CaptchaTestModalSession key={props.providerId} {...props} />;
}

function CaptchaTestModalSession({
  providerId,
  providerType,
  organizationId,
  open,
  onClose,
}: CaptchaTestModalProps) {
  const { t } = useTranslation("common");
  const { message } = App.useApp();

  const [captchaId, setCaptchaId] = useState<string>("");
  const [captchaImage, setCaptchaImage] = useState<string>("");
  const [captchaValue, setCaptchaValue] = useState<string>("");
  const [captchaType, setCaptchaType] = useState<string>("");
  const [captchaShape, setCaptchaShape] = useState<string>("");
  const [captchaSubImage, setCaptchaSubImage] = useState<string>("");
  const [verifyResult, setVerifyResult] = useState<boolean | null>(null);
  const [verifyError, setVerifyError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const canTest = !!organizationId;

  const resetSession = () => {
    setCaptchaId("");
    setCaptchaImage("");
    setCaptchaValue("");
    setCaptchaType("");
    setCaptchaShape("");
    setCaptchaSubImage("");
    setVerifyResult(null);
    setVerifyError(null);
    setLoading(false);
  };

  const handleGenerate = async () => {
    if (!organizationId) return;
    setLoading(true);
    setCaptchaImage("");
    setCaptchaId("");
    setCaptchaValue("");
    setCaptchaShape("");
    setCaptchaSubImage("");
    setVerifyResult(null);
    setVerifyError(null);
    try {
      const result = await generateCaptcha(organizationId);
      setCaptchaId(result.captchaId);
      setCaptchaImage(result.captchaImage);
      setCaptchaType(result.type);
      setCaptchaShape(result.shape ?? "");
      setCaptchaSubImage(result.captchaImageSub ?? "");
    } catch (err) {
      message.error(localizedError(err, t, "loadFailed"));
    } finally {
      setLoading(false);
    }
  };

  const handleVerify = async () => {
    if (!organizationId || !captchaId || !captchaValue) return;
    setLoading(true);
    setVerifyError(null);
    try {
      const result = await verifyCaptcha(organizationId, captchaId, captchaValue);
      setVerifyResult(result.valid);
    } catch (err) {
      // 4xx 是服务端对验证码的裁决（过期/不匹配）；网络中断与 5xx 不是「验证失败」。
      if (isHttpError(err) && err.status < 500) {
        setVerifyResult(false);
      } else {
        setVerifyResult(null);
        setVerifyError(serverErrorMessage(err, t, "captchaTestNetworkError"));
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      title={t("captchaTest")}
      open={open}
      onCancel={onClose}
      destroyOnHidden
      afterOpenChange={(visible) => {
        if (!visible) resetSession();
      }}
      footer={[
        <Button key="close" onClick={onClose}>
          {t("close")}
        </Button>,
      ]}
      width={520}
    >
      <Space orientation="vertical" style={{ width: "100%" }} size="middle">
        {/* 目标 provider 信息 */}
        <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
          <Text strong>{t("type")}:</Text>
          <Tag color="magenta">{providerType}</Tag>
          <Text type="secondary" style={{ marginLeft: 8 }}>
            ID: {providerId}
          </Text>
        </div>

        {!canTest && (
          <Alert
            type="warning"
            showIcon
            title={t("required")}
            description={t("captchaOrgRequired")}
          />
        )}

        {/* 生成验证码 */}
        <Space>
          <Button
            type="primary"
            onClick={handleGenerate}
            disabled={!canTest}
            loading={loading && !captchaId}
          >
            {loading && !captchaId ? <LoadingOutlined /> : null}
            {captchaImage ? t("regenerate") : t("generate")}
          </Button>
        </Space>

        {/* 验证码图片 / go_captcha 交互 */}
        {captchaImage && (
          <div>
            <Text strong>{t("captcha.preview")}:</Text>
            <div style={{ marginTop: 8 }}>
              {captchaType === "go_captcha" ? (
                <GoCaptchaInteraction
                  shape={captchaShape}
                  masterImage={captchaImage}
                  subImage={captchaSubImage}
                  onChange={setCaptchaValue}
                />
              ) : (
                <Image
                  src={`data:image/png;base64,${captchaImage}`}
                  alt="Captcha"
                  width={200}
                  preview={false}
                />
              )}
            </div>
          </div>
        )}

        {/* 输入验证码 */}
        {captchaId && captchaType !== "go_captcha" && (
          <div>
            <Text strong>{t("enterCaptchaText") || "输入验证码"}:</Text>
            <Space.Compact style={{ width: "100%", marginTop: 4 }}>
              <Input
                value={captchaValue}
                onChange={(e) => setCaptchaValue(e.target.value)}
                placeholder={t("enterCaptchaText") || "请输入验证码"}
                onPressEnter={handleVerify}
              />
              <Button type="primary" onClick={handleVerify} loading={loading && !!captchaId}>
                {t("verify") || "验证"}
              </Button>
            </Space.Compact>
          </div>
        )}

        {/* go_captcha 形态由交互组件产出答案，只需验证按钮 */}
        {captchaId && captchaType === "go_captcha" && (
          <Button type="primary" onClick={handleVerify} loading={loading} disabled={!captchaValue}>
            {t("verify") || "验证"}
          </Button>
        )}

        {/* 验证结果 */}
        {verifyResult !== null && (
          <Alert
            type={verifyResult ? "success" : "error"}
            showIcon
            title={
              verifyResult
                ? t("captchaTestSuccess") || "验证码验证成功！"
                : t("captchaTestFailed") || "验证码验证失败，请重试。"
            }
          />
        )}
        {verifyError !== null && <Alert type="error" showIcon title={verifyError} />}
      </Space>
    </Modal>
  );
}
