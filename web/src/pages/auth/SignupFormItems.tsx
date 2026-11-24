import { Button, Checkbox, Form, Input, Progress, Select } from "antd";
import type { SignupItem } from "@/shared/api/iam/application";
import { SendCodeInput } from "@/shared/ui/SendCodeInput";
import {
  getFieldConfig,
  getPasswordStrength,
  getPasswordStrengthColor,
} from "./useSignupPageController";

type Translate = (key: string, options?: Record<string, unknown>) => string;

type SignupFormItemsProps = {
  /** 已按 visible 过滤的注册项（含/不含 Agreement、Captcha 均可，内部拆分）。 */
  items: SignupItem[];
  /** 当前 password 字段值（Form.useWatch 结果），用于强度条。 */
  passwordValue: string;
  /** 额外强制显示验证码输入（真实页在应用解析失败时为 true）。 */
  showCaptcha: boolean;
  /** 无 Agreement 注册项时的受控勾选态（预览可省略 → 非受控）。 */
  agreedToTerms?: boolean;
  onAgreedToTermsChange?: (checked: boolean) => void;
  /** 点击打开《使用条款》《隐私政策》弹窗（真实页提供，预览可省略）。 */
  onOpenAgreement?: () => void;
  /**
   * 真实注册页提供：点击「发送验证码」时调用（内部读当前 email/phone 合成收件人发码）。
   * 预览帧省略 → 验证码字段降级为纯输入框（无发码上下文）。
   */
  onSendVerificationCode?: () => Promise<void>;
  /** 收件人（email/phone）尚未填写时禁用发码按钮，输入框仍可填写。 */
  sendVerificationCodeDisabled?: boolean;
  t: Translate;
};

/**
 * 注册表单字段共享渲染 —— 真实 SignupPage 与配置页预览帧的唯一渲染源。
 *
 * 对标 casdoor ApplicationPreview：预览直接渲染真实组件树，
 * 字段 switch（select/hidden/textarea/password/input）、密码强度条、
 * 验证码输入、协议勾选的语义全部集中在这里，杜绝两套代码漂移。
 */
export function SignupFormItems({
  items,
  passwordValue,
  showCaptcha,
  agreedToTerms,
  onAgreedToTermsChange,
  onOpenAgreement,
  onSendVerificationCode,
  sendVerificationCodeDisabled,
  t,
}: SignupFormItemsProps) {
  const agreementItem = items.find((item) => item.name === "Agreement");
  const formItems = items.filter((item) => item.name !== "Agreement" && item.name !== "Captcha");
  // 验证码字段与「人机验证（Captcha）」正交：仅当注册项采集了可验证收件人
  // （Email / Phone）时才有意义——后端也只在 email/phone 存在时消费该码
  // （resolveVerificationType）。此前误绑 Captcha 注册项，导致配置了人机验证
  // 却未采集联系方式时凭空出现一个无法发码的验证码框。
  const hasContactItem = formItems.some((item) => item.name === "Email" || item.name === "Phone");

  const passwordStrength = getPasswordStrength(passwordValue);
  const strengthLabel =
    passwordStrength < 30
      ? t("auth:passwordWeak")
      : passwordStrength < 60
        ? t("auth:passwordFair")
        : t("auth:passwordStrong");

  return (
    <>
      {formItems.map((item) => {
        const config = getFieldConfig(item, t as never);
        // Select 控件有 options 时渲染为下拉选择器
        if (config.inputType === "select") {
          const selectOptions = (config.options ?? []).map((opt) => ({
            label: opt,
            value: opt,
          }));
          return (
            <Form.Item
              key={config.name}
              name={config.name}
              label={config.label}
              rules={config.rules}
            >
              <Select
                placeholder={config.placeholder}
                options={selectOptions.length > 0 ? selectOptions : undefined}
                showSearch={selectOptions.length > 0 ? true : undefined}
              />
            </Form.Item>
          );
        }
        if (config.inputType === "hidden") {
          return (
            <Form.Item key={config.name} name={config.name} hidden>
              <Input />
            </Form.Item>
          );
        }
        return (
          <Form.Item key={config.name} name={config.name} label={config.label} rules={config.rules}>
            {config.inputType === "textarea" ? (
              <Input.TextArea placeholder={config.placeholder} rows={3} />
            ) : config.inputType === "password" ? (
              <Input.Password placeholder={config.placeholder} autoComplete={config.autoComplete} />
            ) : (
              <Input
                type={
                  config.inputType === "email"
                    ? "email"
                    : config.inputType === "tel"
                      ? "tel"
                      : "text"
                }
                placeholder={config.placeholder}
                autoComplete={config.autoComplete}
              />
            )}
          </Form.Item>
        );
      })}

      {passwordValue && (
        <div className="gl-password-strength">
          <Progress
            percent={passwordStrength}
            strokeColor={getPasswordStrengthColor(passwordStrength)}
            size="small"
            format={() => strengthLabel}
          />
        </div>
      )}

      {(hasContactItem || showCaptcha) && (
        <Form.Item name="verificationCode" label={t("auth:verificationCode")}>
          {onSendVerificationCode ? (
            <SendCodeInput
              onSendCode={onSendVerificationCode}
              sendDisabled={sendVerificationCodeDisabled}
              placeholder={t("auth:enterCodeIfRequired")}
              sendButtonText={t("auth:sendCode")}
              autoComplete="one-time-code"
            />
          ) : (
            <Input placeholder={t("auth:enterCodeIfRequired")} autoComplete="one-time-code" />
          )}
        </Form.Item>
      )}

      {agreementItem ? (
        <Form.Item
          name="agreement"
          valuePropName="checked"
          rules={
            agreementItem.required
              ? [
                  {
                    validator: (_, value) =>
                      value
                        ? Promise.resolve()
                        : Promise.reject(new Error(t("auth:acceptTermsFirst"))),
                  },
                ]
              : undefined
          }
        >
          <Checkbox>{t("auth:agreeTerms")}</Checkbox>
        </Form.Item>
      ) : (
        <Form.Item>
          <Checkbox
            checked={agreedToTerms}
            onChange={(e) => onAgreedToTermsChange?.(e.target.checked)}
          >
            {t("auth:agreeTerms")}
          </Checkbox>
        </Form.Item>
      )}

      {onOpenAgreement && (
        <div style={{ marginTop: -12, marginBottom: 12 }}>
          <Button type="link" size="small" style={{ padding: 0 }} onClick={onOpenAgreement}>
            {t("auth:viewTermsAndPrivacy")}
          </Button>
        </div>
      )}
    </>
  );
}
