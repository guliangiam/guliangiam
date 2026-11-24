import type { OAuthAuthorizeContext } from "@/shared/lib/auth-finishers";

export type OAuthQueryContext = OAuthAuthorizeContext & {
  nonce: string;
  codeChallenge: string;
  codeChallengeMethod: string;
  resource: string;
};

export function parseOAuthQueryContext(search: string): OAuthQueryContext | null {
  const params = new URLSearchParams(search);
  const clientId = params.get("client_id") ?? "";
  const redirectUri = params.get("redirect_uri") ?? "";

  if (!clientId || !redirectUri) {
    return null;
  }

  return {
    clientId,
    redirectUri,
    responseType: params.get("response_type") ?? "code",
    scope: params.get("scope") ?? "",
    state: params.get("state") ?? "",
    nonce: params.get("nonce") ?? "",
    codeChallenge: params.get("code_challenge") ?? "",
    codeChallengeMethod: params.get("code_challenge_method") ?? "",
    resource: params.get("resource") ?? "",
  };
}
