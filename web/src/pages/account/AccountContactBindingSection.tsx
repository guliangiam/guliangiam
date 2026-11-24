import { useState } from "react";
import { App, Button, Form, Input, Select, Typography } from "antd";
import { useTranslation } from "react-i18next";
import { updateOwnAccount } from "@/shared/api/iam/auth";
import { sendVerificationCode } from "@/shared/lib/auth-verification-shim";
import { CountryCodeSelect } from "@/shared/ui/CountryCodeSelect";
import { CaptchaChallengeField } from "@/shared/ui/CaptchaChallengeField";
import { useCaptchaChallenge } from "@/shared/hooks/useCaptchaChallenge";
import { toDialCode, toE164 } from "@/shared/ui/country-data";
import { isHttpError } from "@/shared/api/http";
import {
  applyServerFieldErrors,
  buildServerFieldResolver,
  localizedError,
} from "@/shared/lib/server-error";

type ContactType = "email" | "phone";

type ContactFormValues = {
  type: ContactType;
  recipient: string;
  countryCode?: string;
  verificationCode: string;
};

type AccountContactBindingSectionProps = {
  organizationId: string;
  onBound: () => void;
};

/**
 * 账号安全：绑定（或更换）自己的手机/邮箱。
 *
 * 走认证面的自助端点 PATCH /api/v1/auth/account：身份取自会话（扫码注册的 guest
 * 账号也能认领自己），绑定必须带发给**新联系方式**的验证码；发送验证码复用统一的
 * 验证码接口。绑定成功后回调 onBound 让账号上下文就地刷新。
 */
export function AccountContactBindingSection({
  onBound,
  organizationId,
}: AccountContactBindingSectionProps) {
  const { message } = App.useApp();
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");
  const [form] = Form.useForm<ContactFormValues>();
  const [sending, setSending] = useState(false);
  const [binding, setBinding] = useState(false);
  const captcha = useCaptchaChallenge(organizationId, Boolean(organizationId));
  const contactType = Form.useWatch("type", form) as ContactType | undefined;
  const countryCode = Form.useWatch("countryCode", form) as string | undefined;
  const dialCode = toDialCode(countryCode ?? "");

  const buildRecipient = (values: ContactFormValues) => {
    if (values.type === "email") return values.recipient.trim();
    if (!countryCode) {
      message.error(tc("auth:selectCountryCode"));
      return "";
    }
    const recipient = toE164(countryCode, values.recipient);
    if (!recipient) {
      message.error(tc("form.invalidPhone"));
      return "";
    }
    return recipient;
  };

  const handleSendCode = async () => {
    let values: ContactFormValues;
    try {
      values = await form.validateFields(
        contactType === "phone" ? ["type", "recipient", "countryCode"] : ["type", "recipient"],
      );
    } catch {
      return;
    }

    if (!organizationId) {
      message.error(tc("auth:sendCodeFailed"));
      return;
    }

    const recipient = buildRecipient(values);
    if (!recipient) return;

    if (captcha.required && !captcha.token) {
      message.error(tc("auth:completeCaptchaFirst"));
      return;
    }

    setSending(true);
    try {
      await sendVerificationCode({
        organizationId,
        dest: recipient,
        type: values.type,
        countryCode,
        invalidPhoneMessage: tc("form.invalidPhone"),
        captchaToken: captcha.required ? captcha.token : undefined,
        captchaId: captcha.required ? captcha.captchaId : undefined,
      });
      message.success(tc("auth:codeSent"));
      void captcha.reload();
    } catch (error) {
      void captcha.reload();
      message.error(localizedError(error, tc, "auth:sendCodeFailed"));
    } finally {
      setSending(false);
    }
  };

  const handleFinish = async (values: ContactFormValues) => {
    const recipient = buildRecipient(values);
    if (!recipient) return;
    setBinding(true);
    try {
      const verificationCode = values.verificationCode.trim();
      await updateOwnAccount(
        values.type === "email"
          ? { email: recipient, verificationCode }
          : // Numbers are stored as E.164; self-service binding carries no region field.
            { phone: recipient, verificationCode },
      );
      message.success(t("account.profileUpdated"));
      form.resetFields(["recipient", "verificationCode"]);
      onBound();
    } catch (error) {
      if (isHttpError(error)) {
        applyServerFieldErrors(form, error, buildServerFieldResolver<ContactFormValues>(form), t);
      }
      message.error(localizedError(error, t, "account.updateFailed"));
    } finally {
      setBinding(false);
    }
  };

  return (
    <Form form={form} layout="vertical" initialValues={{ type: "phone" }} onFinish={handleFinish}>
      <Typography.Title level={5}>
        {t("account.email")} / {t("account.phone")}
      </Typography.Title>
      <Form.Item name="type" rules={[{ required: true }]}>
        <Select
          aria-label={tc("type")}
          options={[
            { value: "phone", label: t("account.phone") },
            { value: "email", label: t("account.email") },
          ]}
        />
      </Form.Item>
      {contactType === "phone" ? (
        <Form.Item
          name="countryCode"
          rules={[{ required: true, message: tc("auth:selectCountryCode") }]}
        >
          <CountryCodeSelect ariaLabel={tc("auth:countryCode")} style={{ width: "100%" }} />
        </Form.Item>
      ) : null}
      <Form.Item name="recipient" rules={[{ required: true }]}>
        {/* prefix 常驻渲染：动态增删前后缀节点会触发 antd 的 DOM 重排警告 */}
        <Input
          placeholder={contactType === "email" ? t("account.email") : t("account.phone")}
          prefix={<span>{contactType === "phone" ? dialCode : ""}</span>}
        />
      </Form.Item>
      {captcha.required && (
        <CaptchaChallengeField
          state={captcha.state}
          seed={captcha.seed}
          token={captcha.token}
          onChange={captcha.setToken}
          reload={captcha.reload}
        />
      )}
      {/* Form.Item 直接包 Input：id/aria-required 才能落到 input 元素上
          （包 Space.Compact 会把可访问性属性注到无 role 的 div，axe 判非法） */}
      <Form.Item name="verificationCode" rules={[{ required: true }]}>
        <Input
          placeholder={tc("auth:enterCode")}
          aria-label={tc("auth:verificationCode")}
          suffix={
            <Button
              type="link"
              size="small"
              style={{ paddingInline: 0 }}
              onClick={() => void handleSendCode()}
              loading={sending}
            >
              {tc("auth:sendCode")}
            </Button>
          }
        />
      </Form.Item>
      <Form.Item>
        <Button type="primary" htmlType="submit" loading={binding}>
          {tc("auth:bind")}
        </Button>
      </Form.Item>
    </Form>
  );
}
