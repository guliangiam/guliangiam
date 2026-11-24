import { useCallback } from "react";
import type { FormInstance } from "antd";
import { useCaptchaFlow } from "@/shared/hooks/useCaptchaFlow";
import { useLoginFormDefaults } from "@/shared/hooks/useLoginFormDefaults";
import type { LoginMethodKey } from "@/shared/lib/auth-signin-items";
import type { LoginPageAuthContextState } from "./useLoginPageAuthContext";

type LoginFormFlowValues = {
  organization: string;
  application: string;
  countryCode?: string;
  username: string;
  password?: string;
  code?: string;
  captchaToken?: string;
  agreement?: boolean;
  autoSignin?: boolean;
};

type UseLoginPageFormFlowsValues = {
  form: FormInstance<LoginFormFlowValues>;
  loginMethod: LoginMethodKey;
  setLoginMethod: (value: LoginMethodKey) => void;
  authContext: LoginPageAuthContextState;
};

export function useLoginPageFormFlows(values: UseLoginPageFormFlowsValues) {
  const { authContext, form, loginMethod, setLoginMethod } = values;
  const {
    agreementDefaultValue,
    authApplicationQuery,
    autoSigninDefaultValue,
    defaultCountryCode,
    enabledSigninMethodTabs,
    hasFaceIdProvider,
    loginDefaults,
    resolvedApplication,
    signinMethodTabs,
    watchedApplication,
    watchedOrganization,
    watchedUsername,
  } = authContext;

  const resetCaptchaToken = useCallback(() => {
    form.setFieldValue("captchaToken", "");
  }, [form]);

  const { captchaState, captchaRequired, captchaReloadSeed, reloadCaptcha } = useCaptchaFlow({
    application: resolvedApplication,
    loginMethod,
    watchedOrganization,
    watchedUsername,
    watchedApplication,
    resetCaptchaToken,
  });

  // 扫码登录的状态由 useLoginPageAuthActions 持有（它才拿得到登录成功后的后处理），
  // 这里只负责表单默认值与验证码。
  useLoginFormDefaults({
    form,
    loginMethod,
    setLoginMethod,
    resolvedApplication,
    hasFaceIdProvider,
    signinMethodTabs,
    enabledSigninMethodTabs,
    agreementDefaultValue,
    autoSigninDefaultValue,
    defaultCountryCode,
    loginDefaults,
    authApplicationName: authApplicationQuery.data?.name,
    authApplicationOrganization: authApplicationQuery.data?.organization,
  });

  return {
    captchaState,
    captchaRequired,
    captchaReloadSeed,
    reloadCaptcha,
  };
}

export type LoginPageFormFlowsState = ReturnType<typeof useLoginPageFormFlows>;
