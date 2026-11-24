import { t } from "i18next";
import { useMutation } from "@tanstack/react-query";
import { isHttpError } from "@/shared/api/http";
import { serverErrorMessage } from "@/shared/lib/server-error";
import { passkeyErrorMessage } from "@/pages/auth/passkey-error";
import {
  getSigninMethodName,
  getLoginVerificationChannel,
  isPasswordMethod,
  isVerificationCodeMethod,
  type LoginMethodKey,
} from "@/shared/lib/auth-signin-items";
import {
  ldapLogin,
  loginWithPassword,
  type LoginPayload,
  type LoginResult,
} from "@/shared/api/iam/auth";

type LoginSubmissionValuesBase = {
  organization: string;
  application: string;
  username: string;
  password?: string;
  code?: string;
  captchaToken?: string;
};

type LoginSubmissionPayloadBase = {
  organization: string;
  application: string;
  username: string;
  password?: string;
  code?: string;
  signinMethod?: string;
};

type LoginMutationResult<TPayload> = {
  response: LoginResult;
  formPayload: TPayload;
  apiPayload: LoginPayload;
};

type UseLoginSubmissionFlowValues<
  TValues extends LoginSubmissionValuesBase,
  TPayload extends LoginSubmissionPayloadBase,
> = {
  loginMethod: LoginMethodKey;
  captchaRequired: boolean;
  captchaProviderType?: string | null;
  /** 服务端生成挑战（default/go_captcha）的 captchaId，随登录请求上送绑定答案。 */
  captchaId?: string | null;
  resolvedApplicationId?: string | null;
  createProtocolContextPayload: () => Omit<TPayload, keyof LoginSubmissionPayloadBase>;
  createCaptchaPayload: (captchaToken?: string) => Partial<TPayload>;
  signInWithWebAuthn: (values: TValues) => Promise<void>;
  signInWithFaceId: (values: TValues) => Promise<void>;
  onSuccess: (values: {
    response: LoginResult;
    formPayload: TPayload;
    apiPayload: LoginPayload;
  }) => Promise<void>;
  showError: (message: string) => void;
};

export function useLoginSubmissionFlow<
  TValues extends LoginSubmissionValuesBase,
  TPayload extends LoginSubmissionPayloadBase,
>(values: UseLoginSubmissionFlowValues<TValues, TPayload>) {
  const {
    captchaId,
    captchaProviderType,
    captchaRequired,
    createCaptchaPayload,
    createProtocolContextPayload,
    loginMethod,
    onSuccess,
    resolvedApplicationId,
    showError,
    signInWithFaceId,
    signInWithWebAuthn,
  } = values;

  return useMutation<LoginMutationResult<TPayload> | null, Error, TValues>({
    mutationFn: async (formValues: TValues): Promise<LoginMutationResult<TPayload> | null> => {
      if (captchaRequired && !formValues.captchaToken) {
        throw new Error(
          captchaProviderType === "Default"
            ? t("auth:signin.captcha.imageFirst")
            : t("auth:signin.captcha.verifyFirst"),
        );
      }

      if (loginMethod === "webauthn") {
        await signInWithWebAuthn(formValues);
        return null;
      }

      if (loginMethod === "faceId") {
        await signInWithFaceId(formValues);
        return null;
      }

      const formPayload = {
        ...createProtocolContextPayload(),
        signinMethod: getSigninMethodName(loginMethod),
        organization: formValues.organization,
        username: formValues.username,
        password: isPasswordMethod(loginMethod) ? formValues.password : "",
        code: isVerificationCodeMethod(loginMethod) ? formValues.code : undefined,
        application: formValues.application,
        ...createCaptchaPayload(formValues.captchaToken),
      } as TPayload;

      if (!resolvedApplicationId) {
        throw new Error(t("auth:signin.appNotResolved"));
      }

      // 组织启用 captcha 时人机答案必须随请求上送：后端 EnforceCaptcha 按
      // 服务端解析的 provider fail-close 校验（id 绑定 default/go_captcha 挑战）。
      const captchaFields = captchaRequired
        ? {
            captchaToken: formValues.captchaToken,
            captchaId: captchaId ?? undefined,
          }
        : {};

      const codeLogin = isVerificationCodeMethod(loginMethod);
      const apiPayload: LoginPayload = {
        organizationSlug: formValues.organization,
        username: formValues.username,
        // 后端拒绝 password 与 verificationCode 同送：码登录必须置空 password。
        password: codeLogin ? "" : (formValues.password ?? ""),
        applicationId: resolvedApplicationId,
        ...(codeLogin
          ? {
              verificationCode: formValues.code,
              verificationType: getLoginVerificationChannel(formValues.username, loginMethod),
            }
          : {}),
        ...captchaFields,
      };

      // LDAP tab 必须走 /auth/login/ldap 目录绑定通道：混入密码登录会按本地
      // 口令哈希校验，目录凭据永远失败。
      const response =
        loginMethod === "ldap"
          ? await ldapLogin({
              organizationSlug: formValues.organization,
              username: formValues.username,
              password: formValues.password ?? "",
              applicationId: resolvedApplicationId,
              ...captchaFields,
            })
          : await loginWithPassword(apiPayload);
      return { response, formPayload, apiPayload };
    },
    onSuccess: async (result) => {
      if (!result) {
        return;
      }

      await onSuccess({
        response: result.response,
        formPayload: result.formPayload,
        apiPayload: result.apiPayload,
      });
    },
    onError: (error) => {
      if (loginMethod === "webauthn") {
        // 用户取消（浏览器 NotAllowedError）不是故障，给取消文案；其余错误复用
        // passkeyErrorMessage，读取 details.reason / [reason: xxx] 给出准确原因
        // （凭证不存在、已撤销、计数器异常、challenge 过期等）。
        const cancelled =
          typeof error === "object" &&
          error !== null &&
          (error as { name?: unknown }).name === "NotAllowedError";
        showError(
          cancelled
            ? t("auth:passkey.authenticationCancelled")
            : passkeyErrorMessage(error, "login", t),
        );
        return;
      }

      // 网络中断 / 5xx 与凭据错误区分：后端不可用时不应只显示通用「登录失败」，
      // 给出可操作的网络错误文案（RT-09）。
      const isNetworkOrServer = !isHttpError(error) || (isHttpError(error) && error.status >= 500);
      const message = isNetworkOrServer
        ? t("auth:signin.networkError", "无法连接到服务器，请检查网络后重试")
        : isHttpError(error) && error.status === 401
          ? t("auth:invalidCredentials")
          : isHttpError(error) && error.status === 403
            ? t("auth:accountDisabled")
            : serverErrorMessage(error, t, "auth:signin.loginFailed");
      showError(message);
    },
  });
}
