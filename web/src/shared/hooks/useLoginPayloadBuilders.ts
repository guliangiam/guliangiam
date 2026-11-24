import { useCallback } from "react";
import type { OAuthQueryContext } from "@/shared/lib/auth-oauth-context";
import { getCaptchaClientSecret, type CaptchaState } from "@/shared/hooks/useCaptchaFlow";

export type ProtocolContextPayload = {
  type?: string;
  clientId?: string;
  redirectUri?: string;
  state?: string;
  scope?: string;
  service?: string;
  relayState?: string;
  samlRequest?: string;
};

export type CaptchaPayload = {
  captchaType?: string;
  captchaToken?: string;
  clientSecret?: string;
};

type UseLoginPayloadBuildersValues = {
  isSamlLogin: boolean;
  isCasLogin: boolean;
  oauthContext: OAuthQueryContext | null;
  casService?: string | null;
  samlRelayState?: string | null;
  samlRequest?: string | null;
  captchaRequired: boolean;
  captchaState: CaptchaState | null;
};

export function useLoginPayloadBuilders(values: UseLoginPayloadBuildersValues) {
  const {
    captchaRequired,
    captchaState,
    casService,
    isCasLogin,
    isSamlLogin,
    oauthContext,
    samlRelayState,
    samlRequest,
  } = values;

  const createProtocolContextPayload = useCallback(
    (): ProtocolContextPayload => ({
      type: isSamlLogin ? "saml" : isCasLogin ? "cas" : (oauthContext?.responseType ?? "login"),
      clientId: oauthContext?.clientId,
      redirectUri: oauthContext?.redirectUri,
      state: oauthContext?.state,
      scope: oauthContext?.scope,
      service: casService ?? undefined,
      relayState: samlRelayState ?? undefined,
      samlRequest: samlRequest ?? undefined,
    }),
    [casService, isCasLogin, isSamlLogin, oauthContext, samlRelayState, samlRequest],
  );

  const createCaptchaPayload = useCallback(
    (captchaToken?: string): CaptchaPayload => ({
      captchaType: captchaRequired ? (captchaState?.type ?? "none") : undefined,
      captchaToken: captchaRequired ? captchaToken : undefined,
      clientSecret: captchaRequired ? getCaptchaClientSecret(captchaState) : undefined,
    }),
    [captchaRequired, captchaState],
  );

  return {
    createProtocolContextPayload,
    createCaptchaPayload,
  };
}
