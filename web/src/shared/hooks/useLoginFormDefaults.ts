import { useEffect } from "react";
import type { FormInstance } from "antd";
import type { LoginMethodKey } from "@/shared/lib/auth-signin-items";
import type { ApplicationItem } from "@/shared/types/domain";

type SigninMethodTabItem = {
  key: LoginMethodKey;
  disabled?: boolean;
};

type UseLoginFormDefaultsValues = {
  form: FormInstance<{
    organization: string;
    application: string;
    countryCode?: string;
    username: string;
    password?: string;
    code?: string;
    captchaToken?: string;
    agreement?: boolean;
    autoSignin?: boolean;
  }>;
  loginMethod: LoginMethodKey;
  setLoginMethod: (value: LoginMethodKey) => void;
  resolvedApplication: ApplicationItem | null;
  // hasFaceIdProvider 保留字段——signinMethodTabs 已在 useLoginPageViewModel 中
  // 通过完整 SigninCapabilities（含浏览器能力）生成，默认方法直接从 enabled tabs 取。
  hasFaceIdProvider: boolean;
  signinMethodTabs: SigninMethodTabItem[];
  enabledSigninMethodTabs: SigninMethodTabItem[];
  agreementDefaultValue: boolean;
  autoSigninDefaultValue: boolean;
  defaultCountryCode?: string;
  loginDefaults: {
    organization: string;
    application: string;
    username: string;
  };
  authApplicationName?: string | null;
  authApplicationOrganization?: string | null;
};

export function useLoginFormDefaults(values: UseLoginFormDefaultsValues) {
  const {
    agreementDefaultValue,
    authApplicationName,
    authApplicationOrganization,
    autoSigninDefaultValue,
    defaultCountryCode,
    enabledSigninMethodTabs,
    form,
    hasFaceIdProvider: _hasFaceIdProvider,
    loginDefaults,
    loginMethod,
    resolvedApplication: _resolvedApplication,
    setLoginMethod,
    signinMethodTabs,
  } = values;

  useEffect(() => {
    // 只在当前 loginMethod 不在 enabledSigninMethodTabs 里时才重置。
    // 这避免了每次 render 新数组引用都无条件 setLoginMethod，
    // 覆盖掉用户手动点击 Tab 的切换。
    const enabledKeys = new Set(enabledSigninMethodTabs.map((t) => t.key));
    if (loginMethod && enabledKeys.has(loginMethod)) {
      return; // 当前方法仍可用 — 尊重用户选择，不重置
    }

    // 当前方法不在 enabled 里（或首次 mount），选第一个可用的
    const nextMethod =
      enabledSigninMethodTabs[0]?.key ?? signinMethodTabs.find((item) => !item.disabled)?.key;
    if (nextMethod && nextMethod !== loginMethod) {
      setLoginMethod(nextMethod as LoginMethodKey);
    }
  }, [enabledSigninMethodTabs, loginMethod, setLoginMethod, signinMethodTabs]);

  useEffect(() => {
    form.setFieldValue("password", "");
    form.setFieldValue("code", "");
    form.setFieldValue("captchaToken", "");
    // oxlint-disable-next-line react/exhaustive-effect-dependencies -- loginMethod 是触发键：切换登录方式时清空上一次的凭据字段，effect 体内不读取它
  }, [form, loginMethod]);

  useEffect(() => {
    form.setFieldValue("agreement", agreementDefaultValue);
  }, [agreementDefaultValue, form]);

  useEffect(() => {
    form.setFieldValue("autoSignin", autoSigninDefaultValue);
  }, [autoSigninDefaultValue, form]);

  useEffect(() => {
    form.setFieldsValue({
      organization: authApplicationOrganization || loginDefaults.organization,
      application: authApplicationName || loginDefaults.application,
      countryCode: defaultCountryCode,
      username: loginDefaults.username,
      password: "",
    });
  }, [
    authApplicationName,
    authApplicationOrganization,
    defaultCountryCode,
    form,
    loginDefaults.application,
    loginDefaults.organization,
    loginDefaults.username,
  ]);
}
