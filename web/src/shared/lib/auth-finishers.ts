import { t } from "i18next";

export type AuthLoginPayload = {
  organization: string;
  application: string;
};

export type OAuthAuthorizeContext = {
  clientId: string;
  redirectUri: string;
  responseType: string;
  scope: string;
  state: string;
};

export type CasLoginContext = {
  applicationId: string;
  service: string;
};

export type SamlLoginContext = {
  owner: string;
  applicationName: string;
  relayState: string;
  samlRequest: string;
  username?: string;
  loginHint?: string;
};

export function persistLastLogin(payload: AuthLoginPayload) {
  localStorage.setItem("lastLoginOrg", payload.organization);
  localStorage.setItem("lastLoginApplication", payload.application);
}

// INC-FE-05：授权页早退只在 URL 携带真实授权参数时成立。无参手访
// /login/oauth/authorize 时把空 search 交给 authorize 端点只会得到整页
// 裸 JSON 错误，用户搁浅；此时必须回落控制台登录流。
export function hasOauthAuthorizeParams(search: string): boolean {
  const params = new URLSearchParams(search);
  return Boolean(params.get("client_id") && params.get("redirect_uri"));
}

export async function finishConsoleLogin(
  payload: AuthLoginPayload,
  deps: {
    invalidateSession: () => Promise<void>;
    navigate: (to: string, options?: { replace?: boolean }) => void;
    returnTo: string;
    showSuccess: (content: string) => void;
  },
) {
  persistLastLogin(payload);
  await deps.invalidateSession();
  deps.showSuccess(t("auth:loginSuccess"));
  deps.navigate(deps.returnTo, { replace: true });
}
