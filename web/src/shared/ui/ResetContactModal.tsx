import { useState, useCallback, type FC } from "react";
import { App, Button, Form, Input, Modal, Space, Typography } from "antd";
import { MailOutlined, PhoneOutlined } from "@ant-design/icons";
import { useTranslation } from "react-i18next";
import { requestDocumentJson } from "@/shared/api/http";
import {
  getUser,
  updateUserProfile,
  sendUserContactVerification,
  verifyUserContactVerification,
} from "@/shared/api/iam/user";
import { CountryCodeSelect } from "@/shared/ui/CountryCodeSelect";
import {
  getCountryName,
  normalizeIsoCountryCode,
  toDialCode,
  toE164,
} from "@/shared/ui/country-data";
import { SendCodeInput } from "@/shared/ui/SendCodeInput";

import { AUTH_VERIFICATION_CODE_SEND, AUTH_VERIFICATION_CODE_VERIFY } from "@/shared/api/paths";

/**
 * 区分「提供商未配置/不可用」（后端 503 unavailable 语义）与真实发送失败：
 * 前者引导用户去提供商配置页，后者提示稍后重试。
 */
function contactSendErrorMessage(t: (key: string) => string, e: unknown): string {
  const raw = e instanceof Error ? e.message : String(e);
  return /provider|not configured|not available|unavailable|disabled/i.test(raw)
    ? t("verification.providerMissing")
    : t("verification.codeSendFailed");
}

/**
 * PUT /profile 是全量替换（缺省字段即清空）。把 GET 回显的档案字段原样映射回
 * payload，再覆盖新联系方式，避免姓名/证件/自定义属性等字段被部分提交清空。
 * 键集合与 internal/transport UpdateUserProfileRequest 的 JSON 字段一一对应。
 * 证件号三字段例外（规范 9.5）：响应只有 Set/Masked 对，这里映射结果恒为
 * 空串，而 PUT 对空证件号的语义是「保持存储值」，不会误清。
 */
function profileToPayload(profile: Record<string, unknown>): Record<string, unknown> {
  const text = (v: unknown): string => (typeof v === "string" ? v : "");
  const properties =
    profile.properties && typeof profile.properties === "object"
      ? (profile.properties as Record<string, unknown>)
      : {};
  return {
    displayName: text(profile.displayName),
    firstName: text(profile.firstName),
    lastName: text(profile.lastName),
    avatarUrl: text(profile.avatarUrl),
    email: text(profile.email),
    phone: text(profile.phone),
    countryCode: text(profile.countryCode),
    region: text(profile.region),
    location: text(profile.location),
    birthday: text(profile.birthday),
    gender: text(profile.gender),
    bio: text(profile.bio),
    realName: text(profile.realName),
    idCardType: text(profile.idCardType),
    idCard: text(profile.idCard),
    passportNo: text(profile.passportNo),
    driverLicenseNo: text(profile.driverLicenseNo),
    idCardExpiry: text(profile.idCardExpiry),
    issueCountry: text(profile.issueCountry),
    issuingAuthority: text(profile.issuingAuthority),
    passportPhotoUrl: text(profile.passportPhotoUrl),
    driverLicenseFrontUrl: text(profile.driverLicenseFrontUrl),
    driverLicenseBackUrl: text(profile.driverLicenseBackUrl),
    affiliation: text(profile.affiliation),
    homepage: text(profile.homepage),
    education: text(profile.education),
    title: text(profile.title),
    idCardFront: text(profile.idCardFront),
    idCardBack: text(profile.idCardBack),
    idCardWithPerson: text(profile.idCardWithPerson),
    language: text(profile.language),
    properties,
  };
}

type ResetContactModalProps = {
  userId: string;
  organizationId?: string;
  mode: "email" | "phone";
  currentCountryCode?: string;
  disabled?: boolean;
  onSuccess?: () => void;
};

const ResetContactModal: FC<ResetContactModalProps> = ({
  userId,
  organizationId,
  mode,
  currentCountryCode,
  disabled,
  onSuccess,
}) => {
  const { t, i18n } = useTranslation();
  const { message } = App.useApp();
  const [open, setOpen] = useState(false);
  const [confirmLoading, setConfirmLoading] = useState(false);
  const [newContact, setNewContact] = useState("");
  // `countryCode` is ISO 3166-1 alpha-2. Legacy `+86`-style values are left
  // unset so the user must re-select a country instead of guessing one.
  const [countryCode, setCountryCode] = useState(() => normalizeIsoCountryCode(currentCountryCode));
  const [code, setCode] = useState("");
  const [step, setStep] = useState<"input" | "verify" | "finalized">("input");

  const language = i18n.resolvedLanguage ?? i18n.language;
  const dialCode = toDialCode(countryCode);
  const countryName = getCountryName(countryCode, language);

  const phoneRecipient = useCallback(() => {
    if (!countryCode) {
      message.warning(t("auth:selectCountryCode"));
      return "";
    }
    const recipient = toE164(countryCode, newContact);
    if (!recipient) {
      message.warning(t("auth:enterValidPhone"));
      return "";
    }
    return recipient;
  }, [countryCode, message, newContact, t]);

  const currentRecipient = useCallback(() => {
    return mode === "email" ? newContact.trim() : phoneRecipient();
  }, [mode, newContact, phoneRecipient]);

  const sendCode = useCallback(async () => {
    // finalized 阶段改走用户级端点：目标为已保存的新联系方式，验证通过后会
    // 在服务端翻转 email_verified/phone_verified 标记（generic auth 端点无此语义）。
    if (step === "finalized") {
      try {
        await sendUserContactVerification(userId, mode, organizationId);
        message.success(t("verification.codeSent"));
      } catch (e) {
        message.error(contactSendErrorMessage(t, e));
      }
      return;
    }
    const recipient = currentRecipient();
    if (!recipient) {
      if (mode === "email") message.warning(t("enterContactFirst"));
      return;
    }
    try {
      await requestDocumentJson(AUTH_VERIFICATION_CODE_SEND, {
        method: "POST",
        body: JSON.stringify({
          type: mode === "email" ? "email" : "phone",
          recipient,
        }),
      });
      message.success(t("verification.codeSent"));
      // 如果还在 input step，推进到 verify step。
      // 如果已经在 finalized step（update 成功后），停留在原地让用户输入 code。
      if (step === "input") setStep("verify");
    } catch (e) {
      message.error(contactSendErrorMessage(t, e));
    }
  }, [currentRecipient, mode, message, organizationId, step, t, userId]);

  const handleOk = useCallback(async () => {
    if (!newContact || !code) return;
    const recipient = currentRecipient();
    if (!recipient) return;
    setConfirmLoading(true);
    try {
      try {
        await requestDocumentJson(AUTH_VERIFICATION_CODE_VERIFY, {
          method: "POST",
          body: JSON.stringify({
            type: mode === "email" ? "email" : "phone",
            recipient,
            code,
          }),
        });
      } catch {
        message.error(t("verifyCodeFailed"));
        return;
      }
      try {
        // 先取当前完整档案再合并新联系方式：PUT /profile 是全量替换，
        // 部分 payload 会把姓名/证件/自定义属性等其余字段清空。
        const detail = await getUser(userId, organizationId);
        const payload = profileToPayload(detail.profile ?? {});
        if (mode === "email") {
          payload.email = recipient;
        } else {
          // Store E.164 in `phone`; `countryCode` keeps the ISO alpha-2 region.
          payload.phone = recipient;
          payload.countryCode = countryCode ?? "";
        }
        await updateUserProfile(userId, payload, organizationId);
      } catch {
        message.error(t("resource:user.contactUpdateFailed"));
        return;
      }
      message.success(t("contactUpdated"));
      // update 成功后不关闭 modal —— 进入 finalized step，
      // 展示"已更新"状态并提供再次发送验证码的入口，让用户可选地对新联系方式做二次验证。
      setCode("");
      setStep("finalized");
      onSuccess?.();
    } finally {
      setConfirmLoading(false);
    }
  }, [
    mode,
    newContact,
    currentRecipient,
    countryCode,
    code,
    userId,
    organizationId,
    message,
    t,
    onSuccess,
  ]);

  /** finalized step 里用户输入验证码做二次确认：走用户级端点以翻转 verified 标记。 */
  const handleFinalizedVerify = useCallback(async () => {
    if (!code) return;
    setConfirmLoading(true);
    try {
      await verifyUserContactVerification(userId, mode, code, organizationId);
      message.success(t("resource:user.contactVerified", "Contact verified"));
      // 二次验证通过，正式关闭并刷新页面（verified 标记已在服务端翻转）。
      setOpen(false);
      setNewContact("");
      setCode("");
      setStep("input");
      onSuccess?.();
    } catch {
      message.error(t("verifyCodeFailed"));
    } finally {
      setConfirmLoading(false);
    }
  }, [code, mode, userId, organizationId, message, t, onSuccess]);

  const handleClose = useCallback(() => {
    setOpen(false);
    setNewContact("");
    setCode("");
    setStep("input");
  }, []);

  const handleCancel = useCallback(() => {
    if (step === "finalized") {
      // finalized step 把 Cancel 也视为关闭（因为修改已经落库了）
      handleClose();
    } else {
      setOpen(false);
      setNewContact("");
      setCode("");
      setStep("input");
    }
  }, [step, handleClose]);

  const title = mode === "email" ? t("resetEmail") : t("resetPhone");

  const recipientLabel =
    mode === "email" ? newContact : `${countryName || countryCode} (${dialCode}) ${newContact}`;

  const footer =
    step === "finalized" ? (
      <Space>
        <Button onClick={handleClose}>{t("common:close", { defaultValue: "Close" })}</Button>
        <Button
          type="primary"
          loading={confirmLoading}
          disabled={!code}
          onClick={handleFinalizedVerify}
        >
          {t("common:confirm")}
        </Button>
      </Space>
    ) : null;

  return (
    <>
      <Button
        size="small"
        icon={mode === "email" ? <MailOutlined /> : <PhoneOutlined />}
        disabled={disabled}
        onClick={() => setOpen(true)}
      >
        {title}
      </Button>
      <Modal
        title={title}
        open={open}
        okText={step === "verify" ? t("common:confirm") : t("next")}
        cancelText={
          step === "finalized" ? t("common:close", { defaultValue: "Close" }) : t("common:cancel")
        }
        confirmLoading={confirmLoading}
        onCancel={handleCancel}
        onOk={step === "finalized" ? undefined : step === "verify" ? handleOk : () => sendCode()}
        okButtonProps={{
          disabled:
            step === "input"
              ? !newContact || (mode === "phone" && !countryCode)
              : step === "verify"
                ? !code
                : undefined,
        }}
        footer={footer}
        width={480}
        destroyOnHidden
      >
        <Space orientation="vertical" style={{ width: "100%" }} size="middle">
          {step === "input" ? (
            <>
              <Typography.Text type="secondary">
                {mode === "email" ? t("resetEmailDesc") : t("resetPhoneDesc")}
              </Typography.Text>
              {mode === "phone" && (
                <Form.Item label={t("resource:user.countryCode")} style={{ marginBottom: 8 }}>
                  <Space.Compact style={{ width: "100%" }}>
                    <CountryCodeSelect
                      value={countryCode}
                      onChange={(val: string) => setCountryCode(val)}
                      ariaLabel={t("resource:user.countryCode")}
                      style={{ width: "100%" }}
                    />
                    <Typography.Text
                      type="secondary"
                      style={{ display: "flex", alignItems: "center", paddingInline: 12 }}
                    >
                      {dialCode || "—"}
                    </Typography.Text>
                  </Space.Compact>
                </Form.Item>
              )}
              <Form.Item
                label={mode === "email" ? t("newEmail") : t("newPhone")}
                style={{ marginBottom: 0 }}
              >
                <Input
                  value={newContact}
                  onChange={(e) => setNewContact(e.target.value)}
                  placeholder={mode === "email" ? "user@example.com" : "13800138000"}
                  type={mode === "email" ? "email" : "tel"}
                  prefix={mode === "phone" ? dialCode || undefined : undefined}
                />
              </Form.Item>
            </>
          ) : step === "verify" ? (
            <>
              <Typography.Text type="secondary">
                {t("enterCodeDesc", { recipient: recipientLabel })}
              </Typography.Text>
              <Form.Item label={t("auth:verificationCode")} style={{ marginBottom: 0 }}>
                <SendCodeInput
                  value={code}
                  onChange={setCode}
                  onSendCode={sendCode}
                  placeholder={t("enterCode", "Enter verification code")}
                  sendButtonText={t("auth:resendCode")}
                />
              </Form.Item>
            </>
          ) : (
            // finalized: update 已成功落库，展示状态并提供可选的再次验证入口
            <>
              <Typography.Text strong style={{ color: "var(--ant-color-success)" }}>
                {t("resource:user.contactUpdatedTitle", "Contact updated successfully")}
              </Typography.Text>
              <Typography.Text type="secondary">
                {t(
                  "resource:user.contactUpdatedHint",
                  "Your new {{type}} has been updated to {{recipient}}. You can optionally send another verification code to confirm you still have access.",
                  {
                    type: mode === "email" ? t("email") : t("resource:user.phone"),
                    recipient: recipientLabel,
                  },
                )}
              </Typography.Text>
              <Form.Item label={t("auth:verificationCode")} style={{ marginBottom: 0 }}>
                <SendCodeInput
                  value={code}
                  onChange={setCode}
                  onSendCode={sendCode}
                  placeholder={t("enterCode", "Enter verification code (optional)")}
                  sendButtonText={t("auth:resendCode")}
                />
              </Form.Item>
            </>
          )}
        </Space>
      </Modal>
    </>
  );
};

export { ResetContactModal };
export default ResetContactModal;
