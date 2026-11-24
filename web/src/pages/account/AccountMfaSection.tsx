import { useState } from "react";
import {
  Alert,
  App,
  Button,
  Input,
  Modal,
  Popconfirm,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
} from "antd";
import { MobileOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { QRCodeSVG } from "qrcode.react";
import { useTranslation } from "react-i18next";
import type { UseMutationResult } from "@tanstack/react-query";
import type { UserMfaMethod, UserMfaSetupInitiateResult } from "@/shared/api/iam/user-types";
import { FormSection } from "@/shared/ui/FormSection";

type AccountMfaSectionProps = {
  mfaMethods: UserMfaMethod[];
  preferredMfa: string | undefined;
  mfaSetupMutation: UseMutationResult<UserMfaSetupInitiateResult, Error, string>;
  mfaVerifyMutation: UseMutationResult<
    void,
    Error,
    { mfaType: string; passcode: string; secret?: string }
  >;
  mfaEnableMutation: UseMutationResult<
    void,
    Error,
    { mfaType: string; secret?: string; passcode: string; recoveryCodes: string[] }
  >;
  mfaDisableMutation: UseMutationResult<void, Error, string>;
  setPreferredMfaMutation: UseMutationResult<void, Error, string>;
};

type SetupStep = "scan" | "verify" | "recovery";

/** TOTP 绑定弹窗内容 — 扫码阶段 */
function renderScanStep({
  url,
  secret,
  scanLabel,
  secretLabel,
}: {
  url: string;
  secret?: string;
  scanLabel: string;
  secretLabel: string;
}) {
  return (
    <div style={{ textAlign: "center" }}>
      <Typography.Paragraph>{scanLabel}</Typography.Paragraph>
      <div
        style={{
          display: "inline-block",
          padding: 16,
          background: "#fff",
          border: "1px solid #f0f0f0",
          borderRadius: 8,
        }}
      >
        <QRCodeSVG value={url} size={200} />
      </div>
      <Typography.Paragraph style={{ marginTop: 12 }}>
        <Typography.Text type="secondary">{secretLabel}</Typography.Text>
      </Typography.Paragraph>
      <Typography.Text copyable code>
        {secret ?? ""}
      </Typography.Text>
    </div>
  );
}

export function AccountMfaSection({
  mfaMethods,
  preferredMfa,
  mfaSetupMutation,
  mfaVerifyMutation,
  mfaEnableMutation,
  mfaDisableMutation,
  setPreferredMfaMutation,
}: AccountMfaSectionProps) {
  const { t } = useTranslation("resource");
  const { modal } = App.useApp();

  // TOTP 三步流状态
  const [setupOpen, setSetupOpen] = useState(false);
  const [setupStep, setSetupStep] = useState<SetupStep>("scan");
  const [initiated, setInitiated] = useState<UserMfaSetupInitiateResult | null>(null);
  const [passcode, setPasscode] = useState("");
  const [verifyError, setVerifyError] = useState<string | null>(null);

  const handleSetupTotp = () => {
    setSetupStep("scan");
    setPasscode("");
    setVerifyError(null);
    setInitiated(null);
    setSetupOpen(true);
    mfaSetupMutation.mutate("totp", {
      onSuccess: (data) => {
        if (data?.url) {
          setInitiated(data);
        }
      },
    });
  };

  const handleVerify = () => {
    if (!passcode.trim()) {
      setVerifyError(t("account.mfaEnterCode"));
      return;
    }
    setVerifyError(null);
    mfaVerifyMutation.mutate(
      { mfaType: "totp", passcode: passcode.trim(), secret: initiated?.secret },
      {
        onSuccess: () => {
          setSetupStep("recovery");
        },
        onError: () => {
          // error message already shown by mutation onError; keep modal open for retry
        },
      },
    );
  };

  const handleEnable = () => {
    if (!passcode.trim()) {
      setVerifyError(t("account.mfaEnterCode"));
      return;
    }
    setVerifyError(null);
    mfaEnableMutation.mutate(
      {
        mfaType: "totp",
        secret: initiated?.secret,
        passcode: passcode.trim(),
        recoveryCodes: initiated?.recoveryCodes ?? [],
      },
      {
        onSuccess: () => {
          setSetupOpen(false);
          setInitiated(null);
          setPasscode("");
        },
      },
    );
  };

  const handleCloseSetup = () => {
    if (mfaVerifyMutation.isPending || mfaEnableMutation.isPending) return;
    setSetupOpen(false);
    setInitiated(null);
    setPasscode("");
    setVerifyError(null);
  };

  return (
    <FormSection title={t("account.mfaSection")}>
      {mfaMethods.length === 0 ? (
        <Alert type="info" title={t("account.noMfaEnabled")} />
      ) : (
        <Table
          size="small"
          pagination={false}
          dataSource={mfaMethods.map((m, i: number) => ({ key: i, ...m }))}
          columns={[
            {
              title: t("account.mfaType"),
              dataIndex: "methodType",
              render: (v: string) => <Tag icon={<SafetyCertificateOutlined />}>{v}</Tag>,
            },
            {
              title: t("account.mfaEnabled"),
              dataIndex: "enabled",
              render: (v: boolean, record: UserMfaMethod) => (
                <Switch
                  size="small"
                  checked={v}
                  onChange={(checked) => {
                    if (checked) {
                      // 开启方向不能静默 no-op：走绑定引导流程重新 enrol。
                      handleSetupTotp();
                    } else {
                      // 关闭方向与 Actions 列的 Popconfirm 一致，先确认再禁用，
                      // 避免开关绕过确认直接停用 MFA。
                      modal.confirm({
                        title: t("common:disableMfaConfirm"),
                        okText: t("account.disable"),
                        cancelText: t("common:cancel"),
                        okButtonProps: { danger: true },
                        onOk: () => mfaDisableMutation.mutate(record.methodType),
                      });
                    }
                  }}
                />
              ),
            },
            {
              title: t("account.mfaPreferred"),
              dataIndex: "methodType",
              render: (v: string) => (
                <Switch
                  size="small"
                  checked={v === preferredMfa}
                  onChange={(checked) => {
                    if (checked) {
                      setPreferredMfaMutation.mutate(v);
                    }
                    // 取消 preferred：后端需支持清空，这里暂不允许取消
                    // （preferred 必须指向一个已启用的方法）
                  }}
                />
              ),
            },
            {
              title: t("account.mfaActions"),
              dataIndex: "methodType",
              render: (v: string) => (
                <Popconfirm
                  title={t("common:disableMfaConfirm")}
                  onConfirm={() => mfaDisableMutation.mutate(v)}
                  okText={t("account.disable")}
                  cancelText={t("common:cancel")}
                >
                  <Button size="small" danger loading={mfaDisableMutation.isPending}>
                    {t("account.disable")}
                  </Button>
                </Popconfirm>
              ),
            },
          ]}
        />
      )}
      <Space style={{ marginTop: 12 }}>
        <Button
          icon={<MobileOutlined />}
          onClick={handleSetupTotp}
          loading={mfaSetupMutation.isPending}
        >
          {t("account.setupTotp")}
        </Button>
      </Space>

      <Modal
        title={t("account.mfaSetupTitle")}
        open={setupOpen}
        onCancel={handleCloseSetup}
        footer={null}
        width={480}
        mask={{ closable: false }}
        destroyOnHidden
      >
        {setupStep === "scan" && initiated?.url ? (
          renderScanStep({
            url: initiated.url,
            secret: initiated.secret,
            scanLabel: t("account.mfaScanQRCode"),
            secretLabel: t("account.mfaSecretLabel"),
          })
        ) : setupStep === "scan" ? (
          <div style={{ textAlign: "center", padding: "40px 0" }}>{t("account.mfaPreparing")}</div>
        ) : null}

        {setupStep === "verify" && (
          <Space orientation="vertical" style={{ width: "100%" }} size={16}>
            <Typography.Paragraph>{t("account.mfaEnterCode")}</Typography.Paragraph>
            <Input
              size="large"
              placeholder="000000"
              value={passcode}
              maxLength={6}
              onChange={(e) => setPasscode(e.target.value)}
              onPressEnter={handleVerify}
              style={{ textAlign: "center", fontSize: 24, letterSpacing: 8 }}
            />
            {verifyError ? <Typography.Text type="danger">{verifyError}</Typography.Text> : null}
            <Button
              type="primary"
              block
              loading={mfaVerifyMutation.isPending}
              onClick={handleVerify}
            >
              {t("account.mfaVerify")}
            </Button>
          </Space>
        )}

        {setupStep === "recovery" && (
          <Space orientation="vertical" style={{ width: "100%" }} size={16}>
            <Alert
              type="success"
              showIcon
              title={t("account.mfaRecoveryCodes")}
              description={t("account.mfaRecoveryCodesDesc")}
            />
            <div
              style={{
                background: "#f6f6f6",
                padding: 12,
                borderRadius: 6,
                fontFamily: "monospace",
                fontSize: 13,
                lineHeight: 1.8,
              }}
            >
              {(initiated?.recoveryCodes ?? []).map((code) => (
                <div key={code}>{code}</div>
              ))}
            </div>
            <Typography.Text copyable>
              {(initiated?.recoveryCodes ?? []).join("\n")}
            </Typography.Text>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              {t("account.mfaEnterCode")}
            </Typography.Paragraph>
            <Input
              size="large"
              placeholder="000000"
              value={passcode}
              maxLength={6}
              onChange={(e) => setPasscode(e.target.value)}
              onPressEnter={handleEnable}
              style={{ textAlign: "center", fontSize: 24, letterSpacing: 8 }}
            />
            {verifyError ? <Typography.Text type="danger">{verifyError}</Typography.Text> : null}
            <Button
              type="primary"
              block
              loading={mfaEnableMutation.isPending}
              onClick={handleEnable}
            >
              {t("account.mfaEnable")}
            </Button>
          </Space>
        )}

        {setupStep === "scan" && initiated?.url ? (
          <div style={{ marginTop: 16, textAlign: "right" }}>
            <Button type="primary" onClick={() => setSetupStep("verify")}>
              {t("account.mfaNext")}
            </Button>
          </div>
        ) : null}
      </Modal>
    </FormSection>
  );
}
