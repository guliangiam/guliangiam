import { useMutation } from "@tanstack/react-query";
import type { FormInstance } from "antd";
import { useTranslation } from "react-i18next";
import { sendVerificationCode } from "@/shared/lib/auth-verification-shim";
import { serverErrorMessage } from "@/shared/lib/server-error";
import type { CaptchaState } from "@/shared/lib/auth-captcha-shim";
import { isVerificationCodeMethod, type LoginMethodKey } from "@/shared/lib/auth-signin-items";
import type { ApplicationItem } from "@/shared/types/domain";

type VerificationCodeFormValues = {
  organization: string;
  application: string;
  username: string;
  countryCode?: string;
  captchaToken?: string;
};

function getVerificationCodeTarget(username: string, loginMethod: LoginMethodKey) {
  if (loginMethod === "verificationCodeEmail") {
    return { type: "email" as const, dest: username };
  }

  if (loginMethod === "verificationCodePhone") {
    return { type: "phone" as const, dest: username };
  }

  if (username.includes("@")) {
    return { type: "email" as const, dest: username };
  }

  return { type: "phone" as const, dest: username };
}

type UseVerificationCodeFlowValues<TValues extends VerificationCodeFormValues> = {
  form: FormInstance<TValues>;
  loginMethod: LoginMethodKey;
  application: ApplicationItem | null;
  captchaRequired: boolean;
  captchaState: CaptchaState | null;
  showSuccess: (message: string) => void;
  showError: (message: string) => void;
};

export function useVerificationCodeFlow<TValues extends VerificationCodeFormValues>(
  values: UseVerificationCodeFlowValues<TValues>,
) {
  const { captchaRequired, captchaState, form, loginMethod, showError, showSuccess } = values;
  const { t } = useTranslation();

  return useMutation({
    mutationFn: async () => {
      const fieldsToValidate = ["organization", "application", "username"];
      if (loginMethod === "verificationCodePhone") {
        fieldsToValidate.push("countryCode");
      }

      const formValues = await form.validateFields(fieldsToValidate);
      if (!isVerificationCodeMethod(loginMethod)) {
        throw new Error(t("verification.methodNotSupported"));
      }

      const target = getVerificationCodeTarget(formValues.username, loginMethod);
      const captchaToken = (form as FormInstance).getFieldValue("captchaToken") as
        | string
        | undefined;

      if (captchaRequired && !captchaToken) {
        throw new Error(
          captchaState?.type === "default"
            ? t("verification.inputImageCaptcha")
            : t("verification.completeCaptchaFirst"),
        );
      }

      return sendVerificationCode({
        organizationId: formValues.organization,
        // 应用级提供商优先级：登录用户所在应用已绑定的短信/邮件提供商先于组织级。
        applicationId: values.application?.id ?? undefined,
        dest: target.dest,
        type: target.type,
        countryCode: target.type === "phone" ? formValues.countryCode : undefined,
        invalidPhoneMessage: t("form.invalidPhone"),
        captchaToken: captchaRequired ? captchaToken : undefined,
        captchaId: captchaRequired ? (captchaState?.captchaId ?? undefined) : undefined,
      });
    },
    onSuccess: () => {
      showSuccess(t("verification.codeSent"));
    },
    onError: (error) => {
      showError(serverErrorMessage(error, t, "verification.codeSendFailed"));
    },
  });
}
