import { useState } from "react";
import { App, Button, Card, Input, Result, Space, Steps, Typography } from "antd";
import { CheckCircleOutlined } from "@ant-design/icons";
import { deviceVerify } from "@/shared/api/iam/device";
import { useTranslation } from "react-i18next";
import { localizedError } from "@/shared/lib/server-error";

export default function DeviceVerifyPage() {
  const { message } = App.useApp();
  const { t } = useTranslation("common");
  const [userCode, setUserCode] = useState("");
  const [step, setStep] = useState<0 | 1 | 2>(0);
  const [verifying, setVerifying] = useState(false);

  const handleVerify = async () => {
    // 提交期间禁用入口，防止双击双发。
    if (verifying) return;
    const code = userCode.trim();
    if (!code) {
      message.error(t("enterUserCode"));
      return;
    }

    setVerifying(true);
    setStep(1);
    try {
      await deviceVerify(code);
      setStep(2);
      message.success(t("deviceAuthorized"));
    } catch (err: unknown) {
      setStep(0);
      const msg = localizedError(err, t, "verifyFailed");
      message.error(msg);
    } finally {
      setVerifying(false);
    }
  };

  return (
    <div className="gl-standalone-page">
      <Card
        style={{ width: 460, maxWidth: "calc(100vw - 32px)", boxShadow: "var(--gl-shadow-md)" }}
      >
        <Space orientation="vertical" size="large" style={{ width: "100%" }}>
          <div style={{ textAlign: "center" }}>
            <Typography.Title level={3}>{t("deviceAuth")}</Typography.Title>
            <Typography.Text type="secondary">{t("deviceAuthDesc")}</Typography.Text>
          </div>

          <Steps
            current={step}
            items={[{ title: t("enterCode") }, { title: t("confirmAuth") }]}
            style={{ marginBottom: "var(--gl-space-4)" }}
          />

          {step < 2 && (
            <Space orientation="vertical" size="middle" style={{ width: "100%" }}>
              <Input
                size="large"
                placeholder="XXXX-XXXX"
                value={userCode}
                disabled={verifying}
                onChange={(e) => setUserCode(e.target.value.toUpperCase())}
                maxLength={9}
                style={{
                  textAlign: "center",
                  fontSize: "var(--gl-font-size-xl)",
                  letterSpacing: "var(--gl-space-1)",
                }}
                onPressEnter={handleVerify}
              />
              <Button
                type="primary"
                size="large"
                block
                icon={<CheckCircleOutlined />}
                loading={verifying}
                disabled={verifying}
                onClick={handleVerify}
              >
                {t("authorizeDevice")}
              </Button>
            </Space>
          )}

          {step === 2 && (
            <Result status="success" title={t("authSuccess")} subTitle={t("authSuccessDesc")} />
          )}
        </Space>
      </Card>
    </div>
  );
}
