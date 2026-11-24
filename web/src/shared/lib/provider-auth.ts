import type { ApplicationItem } from "@/shared/types/domain";

export type DirectAuthProvider = {
  name?: string | null;
  type?: string | null;
  clientId?: string | null;
  clientId2?: string | null;
  domain?: string | null;
  subType?: string | null;
  method?: string | null;
  disableSsl?: boolean | null;
  signName?: string | null;
  scopes?: string | null;
  customAuthUrl?: string | null;
  customLogo?: string | null;
  enablePkce?: boolean | null;
  appId?: string | null;
};

type AuthInfo = {
  endpoint: string;
  endpoint2?: string;
  internalEndpoint?: string;
  mpEndpoint?: string;
  mpScope?: string;
  scope?: string;
  silentEndpoint?: string;
};

const authInfo: Record<string, AuthInfo> = {
  Google: {
    scope: "profile+email",
    endpoint: "https://accounts.google.com/signin/oauth",
  },
  GitHub: {
    scope: "user:email+read:user",
    endpoint: "https://github.com/login/oauth/authorize",
  },
  QQ: {
    scope: "get_user_info",
    endpoint: "https://graph.qq.com/oauth2.0/authorize",
  },
  WeChat: {
    scope: "snsapi_login",
    endpoint: "https://open.weixin.qq.com/connect/qrconnect",
    mpScope: "snsapi_userinfo",
    mpEndpoint: "https://open.weixin.qq.com/connect/oauth2/authorize",
  },
  WeChatMiniProgram: { endpoint: "https://mp.weixin.qq.com/" },
  Facebook: {
    scope: "email,public_profile",
    endpoint: "https://www.facebook.com/dialog/oauth",
  },
  DingTalk: {
    scope: "openid",
    endpoint: "https://login.dingtalk.com/oauth2/auth",
  },
  Weibo: { scope: "email", endpoint: "https://api.weibo.com/oauth2/authorize" },
  Gitee: {
    scope: "user_info%20emails",
    endpoint: "https://gitee.com/oauth/authorize",
  },
  LinkedIn: {
    scope: "r_liteprofile%20r_emailaddress",
    endpoint: "https://www.linkedin.com/oauth/v2/authorization",
  },
  WeCom: {
    scope: "snsapi_userinfo",
    endpoint: "https://login.work.weixin.qq.com/wwlogin/sso/login",
    silentEndpoint: "https://open.weixin.qq.com/connect/oauth2/authorize",
    internalEndpoint: "https://login.work.weixin.qq.com/wwlogin/sso/login",
  },
  Lark: {
    endpoint: "https://open.feishu.cn/open-apis/authen/v1/index",
    endpoint2: "https://accounts.larksuite.com/open-apis/authen/v1/authorize",
  },
  GitLab: {
    scope: "read_user+profile",
    endpoint: "https://gitlab.com/oauth/authorize",
  },
  ADFS: { scope: "openid", endpoint: "http://example.com" },
  Baidu: {
    scope: "basic",
    endpoint: "http://openapi.baidu.com/oauth/2.0/authorize",
  },
  Alipay: {
    scope: "basic",
    endpoint: "https://openauth.alipay.com/oauth2/publicAppAuthorize.htm",
  },
  GuliangIAM: {
    scope: "openid%20profile%20email",
    endpoint: "http://example.com",
  },
  Infoflow: { endpoint: "https://xpc.im.baidu.com/oauth2/authorize" },
  Apple: {
    scope: "name%20email",
    endpoint: "https://appleid.apple.com/auth/authorize",
  },
  AzureAD: {
    scope: "user.read",
    endpoint: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
  },
  AzureADB2C: {
    scope: "openid",
    endpoint: "https://tenant.b2clogin.com/tenant.onmicrosoft.com/userflow/oauth2/v2.0/authorize",
  },
  Slack: { scope: "users:read", endpoint: "https://slack.com/oauth/authorize" },
  Steam: { endpoint: "https://steamcommunity.com/openid/login" },
  Okta: { scope: "openid%20profile%20email", endpoint: "http://example.com" },
  Douyin: {
    scope: "user_info",
    endpoint: "https://open.douyin.com/platform/oauth/connect",
  },
  Kwai: {
    scope: "user_info",
    endpoint: "https://open.kuaishou.com/oauth2/connect",
  },
  Custom: { endpoint: "https://example.com/" },
  Bilibili: {
    endpoint: "https://passport.bilibili.com/register/pc_oauth2.html",
  },
  Line: {
    scope: "profile%20openid%20email",
    endpoint: "https://access.line.me/oauth2/v2.1/authorize",
  },
  Amazon: { scope: "profile", endpoint: "https://www.amazon.com/ap/oa" },
  Auth0: {
    scope: "openid%20profile%20email",
    endpoint: "http://auth0.com/authorize",
  },
  BattleNet: {
    scope: "openid",
    endpoint: "https://oauth.battlenet.com.cn/authorize",
  },
  Bitbucket: {
    scope: "account",
    endpoint: "https://bitbucket.org/site/oauth2/authorize",
  },
  Box: {
    scope: "root_readwrite",
    endpoint: "https://account.box.com/api/oauth2/authorize",
  },
  CloudFoundry: {
    scope: "cloud_controller.read",
    endpoint: "https://login.cloudfoundry.org/oauth/authorize",
  },
  Dailymotion: {
    scope: "userinfo",
    endpoint: "https://api.dailymotion.com/oauth/authorize",
  },
  Deezer: {
    scope: "basic_access",
    endpoint: "https://connect.deezer.com/oauth/auth.php",
  },
  DigitalOcean: {
    scope: "read",
    endpoint: "https://cloud.digitalocean.com/v1/oauth/authorize",
  },
  Discord: {
    scope: "identify%20email",
    endpoint: "https://discord.com/api/oauth2/authorize",
  },
  Dropbox: {
    scope: "account_info.read",
    endpoint: "https://www.dropbox.com/oauth2/authorize",
  },
  EveOnline: {
    scope: "publicData",
    endpoint: "https://login.eveonline.com/oauth/authorize",
  },
  Fitbit: {
    scope:
      "activity%20heartrate%20location%20nutrition%20profile%20settings%20sleep%20social%20weight",
    endpoint: "https://www.fitbit.com/oauth2/authorize",
  },
  Gitea: {
    scope: "user:email",
    endpoint: "https://gitea.com/login/oauth/authorize",
  },
  Heroku: {
    scope: "global",
    endpoint: "https://id.heroku.com/oauth/authorize",
  },
  InfluxCloud: {
    scope: "read:org",
    endpoint: "https://cloud2.influxdata.com/oauth/authorize",
  },
  Instagram: {
    scope: "user_profile",
    endpoint: "https://api.instagram.com/oauth/authorize",
  },
  Intercom: { scope: "user.read", endpoint: "https://app.intercom.com/oauth" },
  Kakao: {
    scope: "account_email",
    endpoint: "https://kauth.kakao.com/oauth/authorize",
  },
  Lastfm: { scope: "user_read", endpoint: "https://www.last.fm/api/auth" },
  Mailru: { scope: "userinfo", endpoint: "https://oauth.mail.ru/login" },
  MailRu: { scope: "userinfo", endpoint: "https://oauth.mail.ru/login" },
  Meetup: {
    scope: "basic",
    endpoint: "https://secure.meetup.com/oauth2/authorize",
  },
  MicrosoftOnline: {
    scope: "openid%20profile%20email",
    endpoint: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
  },
  Naver: {
    scope: "profile",
    endpoint: "https://nid.naver.com/oauth2.0/authorize",
  },
  Nextcloud: {
    scope: "openid%20profile%20email",
    endpoint: "https://cloud.example.org/apps/oauth2/authorize",
  },
  OneDrive: {
    scope: "offline_access%20onedrive.readonly",
    endpoint: "https://login.live.com/oauth20_authorize.srf",
  },
  Oura: {
    scope: "personal",
    endpoint: "https://cloud.ouraring.com/oauth/authorize",
  },
  Patreon: {
    scope: "identity",
    endpoint: "https://www.patreon.com/oauth2/authorize",
  },
  PayPal: {
    scope: "openid%20profile%20email",
    endpoint: "https://www.sandbox.paypal.com/connect",
  },
  SalesForce: {
    scope: "openid%20profile%20email",
    endpoint: "https://login.salesforce.com/services/oauth2/authorize",
  },
  Shopify: {
    scope: "read_products",
    endpoint: "https://myshopify.com/admin/oauth/authorize",
  },
  Soundcloud: {
    scope: "non-expiring",
    endpoint: "https://api.soundcloud.com/connect",
  },
  SoundCloud: {
    scope: "non-expiring",
    endpoint: "https://api.soundcloud.com/connect",
  },
  Spotify: {
    scope: "user-read-email",
    endpoint: "https://accounts.spotify.com/authorize",
  },
  Strava: { scope: "read", endpoint: "https://www.strava.com/oauth/authorize" },
  Stripe: {
    scope: "read_only",
    endpoint: "https://connect.stripe.com/oauth/authorize",
  },
  TikTok: {
    scope: "user.info.basic",
    endpoint: "https://www.tiktok.com/auth/authorize/",
  },
  Tumblr: {
    scope: "basic",
    endpoint: "https://www.tumblr.com/oauth2/authorize",
  },
  Twitch: {
    scope: "user_read",
    endpoint: "https://id.twitch.tv/oauth2/authorize",
  },
  Twitter: {
    scope: "users.read%20tweet.read",
    endpoint: "https://twitter.com/i/oauth2/authorize",
  },
  Telegram: { scope: "", endpoint: "https://core.telegram.org/widgets/login" },
  Typetalk: { scope: "my", endpoint: "https://typetalk.com/oauth2/authorize" },
  Uber: {
    scope: "profile",
    endpoint: "https://login.uber.com/oauth/v2/authorize",
  },
  VK: { scope: "email", endpoint: "https://oauth.vk.com/authorize" },
  Wepay: {
    scope: "manage_accounts%20view_user",
    endpoint: "https://www.wepay.com/v2/oauth2/authorize",
  },
  Xero: {
    scope: "openid%20profile%20email",
    endpoint: "https://login.xero.com/identity/connect/authorize",
  },
  Yahoo: {
    scope: "openid%20profile%20email",
    endpoint: "https://api.login.yahoo.com/oauth2/request_auth",
  },
  Yammer: {
    scope: "user",
    endpoint: "https://www.yammer.com/oauth2/authorize",
  },
  Yandex: {
    scope: "login:email",
    endpoint: "https://oauth.yandex.com/authorize",
  },
  Zoom: { scope: "user:read", endpoint: "https://zoom.us/oauth/authorize" },
};

const simpleCodeProviders = new Set([
  "Google",
  "GitHub",
  "Facebook",
  "Weibo",
  "Gitee",
  "LinkedIn",
  "GitLab",
  "AzureAD",
  "Slack",
  "Line",
  "Amazon",
  "Auth0",
  "BattleNet",
  "Bitbucket",
  "Box",
  "CloudFoundry",
  "Dailymotion",
  "DigitalOcean",
  "Discord",
  "Dropbox",
  "EveOnline",
  "Gitea",
  "Heroku",
  "InfluxCloud",
  "Instagram",
  "Intercom",
  "Kakao",
  "Mailru",
  "MailRu",
  "Meetup",
  "MicrosoftOnline",
  "Naver",
  "Nextcloud",
  "OneDrive",
  "Oura",
  "Patreon",
  "PayPal",
  "SalesForce",
  "Soundcloud",
  "SoundCloud",
  "Spotify",
  "Strava",
  "Stripe",
  "Tumblr",
  "Twitch",
  "Typetalk",
  "Uber",
  "VK",
  "Wepay",
  "Xero",
  "Yahoo",
  "Yammer",
  "Yandex",
  "Zoom",
]);

function getAuthInfo(type?: string | null) {
  if (!type) {
    return null;
  }

  const normalizedType = type.startsWith("Custom") ? "Custom" : type;
  return authInfo[normalizedType] ?? null;
}

function getStringField(source: unknown, key: string) {
  if (!source || typeof source !== "object") {
    return "";
  }

  const value = (source as Record<string, unknown>)[key];
  return typeof value === "string" ? value : "";
}

function isSharedApplication(application: ApplicationItem | null) {
  if (!application) {
    return false;
  }

  return (application as Record<string, unknown>).isShared === true;
}

function base64UrlEncode(bytes: Uint8Array) {
  return btoa(String.fromCharCode(...bytes))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=/g, "");
}

function generateCodeVerifier() {
  const bytes = new Uint8Array(32);
  window.crypto.getRandomValues(bytes);
  return base64UrlEncode(bytes);
}

async function generateCodeChallenge(verifier: string) {
  const digest = await window.crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier));
  return base64UrlEncode(new Uint8Array(digest));
}

function storeCodeVerifier(state: string, verifier: string) {
  window.sessionStorage.setItem(`pkce_verifier_${state}`, verifier);
}

function getStateFromQueryParams(
  applicationName: string,
  providerName: string,
  method: string,
  isShortState: boolean,
) {
  let query = window.location.search;
  query = `${query}&application=${encodeURIComponent(applicationName)}&provider=${encodeURIComponent(providerName)}&method=${method}`;

  if (method === "link") {
    query = `${query}&from=${encodeURIComponent(`${window.location.pathname}${window.location.search}`)}`;
  }

  if (!isShortState) {
    return window.btoa(query);
  }

  window.sessionStorage.setItem(providerName, query);
  return providerName;
}

export function getQueryParamsFromState(state: string) {
  const query = window.sessionStorage.getItem(state);
  if (query !== null) {
    return query;
  }

  return window.atob(state);
}

export function getDirectWeb3ProviderCallbackUrl(
  application: ApplicationItem | null,
  provider: DirectAuthProvider,
  method = "signup",
) {
  if (!application || !provider.name || !provider.type) {
    return null;
  }

  let applicationName = application.name;
  if (isSharedApplication(application) && application.organization) {
    applicationName = `${application.name}-org-${application.organization}`;
  }

  const state = getStateFromQueryParams(applicationName, provider.name, method, false);
  const redirectOrigin =
    getStringField(application, "forcedRedirectOrigin") || window.location.origin;

  return `${redirectOrigin}/callback?state=${state}`;
}

export function canStartDirectOAuthProvider(provider: DirectAuthProvider) {
  if (!provider.name || !provider.type) {
    return false;
  }

  if (provider.type === "WeChatMiniProgram") {
    return false;
  }

  if (provider.type === "Telegram") {
    return true;
  }

  if (provider.type === "WeChat") {
    if (navigator.userAgent.includes("MicroMessenger")) {
      return Boolean(provider.clientId2);
    }

    if (provider.clientId2 && provider.disableSsl && provider.signName === "media") {
      return false;
    }

    return Boolean(provider.clientId);
  }

  if (provider.type === "WeCom") {
    if (!provider.clientId) {
      return false;
    }

    if (provider.subType === "Internal" && provider.method === "Normal") {
      return Boolean(provider.appId);
    }

    return true;
  }

  if (provider.type === "AzureADB2C") {
    return Boolean(provider.clientId && provider.domain && provider.appId);
  }

  if (provider.type === "Custom") {
    return Boolean(provider.clientId && provider.customAuthUrl);
  }

  if (provider.type.startsWith("Custom")) {
    return Boolean(provider.clientId && provider.customAuthUrl);
  }

  return Boolean(provider.clientId && getAuthInfo(provider.type));
}

export async function getDirectOAuthProviderAuthUrl(
  application: ApplicationItem | null,
  provider: DirectAuthProvider,
  method = "signup",
  code?: string,
) {
  if (!application || !provider.type || !provider.name) {
    return null;
  }

  const normalizedType = provider.type.startsWith("Custom") ? "Custom" : provider.type;
  const providerAuthInfo = getAuthInfo(provider.type);
  if (!providerAuthInfo) {
    return null;
  }

  let endpoint = providerAuthInfo.endpoint;
  const redirectOrigin =
    getStringField(application, "forcedRedirectOrigin") || window.location.origin;
  let redirectUri = `${redirectOrigin}/callback`;
  let scope = provider.scopes?.trim() ? provider.scopes : (providerAuthInfo.scope ?? "");
  const isShortState =
    (provider.type === "WeChat" && navigator.userAgent.includes("MicroMessenger")) ||
    provider.type === "Twitter";
  let applicationName = application.name;

  if (isSharedApplication(application) && application.organization) {
    applicationName = `${application.name}-org-${application.organization}`;
  }

  const state = getStateFromQueryParams(applicationName, provider.name, method, isShortState);
  const codeVerifier = generateCodeVerifier();
  const codeChallenge = await generateCodeChallenge(codeVerifier);
  storeCodeVerifier(state, codeVerifier);

  if (provider.type === "AzureAD" && provider.domain) {
    endpoint = endpoint.replace("common", provider.domain);
  } else if (provider.type === "Apple") {
    redirectUri = `${redirectOrigin}/api/callback`;
  } else if (provider.type === "Google" && provider.disableSsl) {
    scope += "+https://www.googleapis.com/auth/user.phonenumbers.read";
  } else if (provider.type === "Nextcloud" && provider.domain) {
    endpoint = `${provider.domain}/apps/oauth2/authorize`;
  } else if (provider.type === "Lark" && provider.disableSsl && providerAuthInfo.endpoint2) {
    endpoint = providerAuthInfo.endpoint2;
  }

  if (simpleCodeProviders.has(provider.type)) {
    return `${endpoint}?client_id=${provider.clientId}&redirect_uri=${redirectUri}&scope=${scope}&response_type=code&state=${state}`;
  }

  if (provider.type === "QQ") {
    return `${endpoint}?response_type=code&client_id=${provider.clientId}&redirect_uri=${encodeURIComponent(redirectUri)}&state=${encodeURIComponent(state)}&scope=${encodeURIComponent(scope)}`;
  }

  if (provider.type === "AzureADB2C") {
    return `https://${provider.domain}.b2clogin.com/${provider.domain}.onmicrosoft.com/${provider.appId}/oauth2/v2.0/authorize?client_id=${provider.clientId}&nonce=${crypto.randomUUID()}&redirect_uri=${encodeURIComponent(redirectUri)}&scope=${scope}&response_type=code&state=${state}&prompt=login`;
  }

  if (provider.type === "DingTalk") {
    return `${endpoint}?client_id=${provider.clientId}&redirect_uri=${redirectUri}&scope=${scope}&response_type=code&prompt=login%20consent&state=${state}`;
  }

  if (provider.type === "WeChat") {
    if (navigator.userAgent.includes("MicroMessenger")) {
      return `${providerAuthInfo.mpEndpoint}?appid=${provider.clientId2}&redirect_uri=${redirectUri}&state=${state}&scope=${providerAuthInfo.mpScope}&response_type=code#wechat_redirect`;
    }

    if (provider.clientId2 && provider.disableSsl && provider.signName === "media") {
      if (!code) {
        return null;
      }

      return `${redirectOrigin}/callback?state=${state}&code=${`wechat_oa:${code}`}`;
    }

    return `${endpoint}?appid=${provider.clientId}&redirect_uri=${redirectUri}&scope=${scope}&response_type=code&state=${state}#wechat_redirect`;
  }

  if (provider.type === "WeCom") {
    if (provider.subType === "Internal") {
      if (provider.method === "Silent") {
        return `${providerAuthInfo.silentEndpoint}?appid=${provider.clientId}&redirect_uri=${redirectUri}&state=${state}&scope=${scope}&response_type=code#wechat_redirect`;
      }

      if (provider.method === "Normal") {
        return `${providerAuthInfo.internalEndpoint}?login_type=CorpApp&appid=${provider.clientId}&agentid=${provider.appId}&redirect_uri=${redirectUri}&state=${state}`;
      }

      return null;
    }

    if (provider.subType === "Third-party") {
      if (provider.method === "Silent") {
        return `${providerAuthInfo.silentEndpoint}?appid=${provider.clientId}&redirect_uri=${redirectUri}&state=${state}&scope=${scope}&response_type=code#wechat_redirect`;
      }

      if (provider.method === "Normal") {
        return `${providerAuthInfo.endpoint}?login_type=ServiceApp&appid=${provider.clientId}&redirect_uri=${redirectUri}&state=${state}`;
      }
    }

    return null;
  }

  if (provider.type === "Lark") {
    if (provider.disableSsl) {
      redirectUri = encodeURIComponent(redirectUri);
    }

    return `${endpoint}?app_id=${provider.clientId}&redirect_uri=${redirectUri}&state=${state}`;
  }

  if (provider.type === "ADFS") {
    return `${provider.domain}/adfs/oauth2/authorize?client_id=${provider.clientId}&redirect_uri=${redirectUri}&state=${state}&response_type=code&nonce=${crypto.randomUUID()}&scope=openid`;
  }

  if (provider.type === "Baidu") {
    return `${endpoint}?client_id=${provider.clientId}&redirect_uri=${redirectUri}&state=${state}&response_type=code&scope=${scope}&display=popup`;
  }

  if (provider.type === "Alipay") {
    return `${endpoint}?app_id=${provider.clientId}&scope=auth_user&redirect_uri=${redirectUri}&state=${state}&response_type=code&scope=${scope}&display=popup`;
  }

  if (provider.type === "GuliangIAM") {
    return `${provider.domain}/login/oauth/authorize?client_id=${provider.clientId}&redirect_uri=${redirectUri}&state=${state}&response_type=code&scope=${scope}`;
  }

  if (provider.type === "Infoflow") {
    return `${endpoint}?appid=${provider.clientId}&redirect_uri=${redirectUri}?state=${state}`;
  }

  if (provider.type === "Apple") {
    return `${endpoint}?client_id=${provider.clientId}&redirect_uri=${redirectUri}&state=${state}&response_type=code%20id_token&scope=${scope}&response_mode=form_post`;
  }

  if (provider.type === "Steam") {
    return `${endpoint}?openid.claimed_id=http://specs.openid.net/auth/2.0/identifier_select&openid.identity=http://specs.openid.net/auth/2.0/identifier_select&openid.mode=checkid_setup&openid.ns=http://specs.openid.net/auth/2.0&openid.realm=${redirectOrigin}&openid.return_to=${redirectUri}?state=${state}`;
  }

  if (provider.type === "Okta") {
    return `${provider.domain}/v1/authorize?client_id=${provider.clientId}&redirect_uri=${redirectUri}&state=${state}&response_type=code&scope=${scope}`;
  }

  if (provider.type === "Douyin" || provider.type === "TikTok") {
    return `${endpoint}?client_key=${provider.clientId}&redirect_uri=${redirectUri}&state=${state}&response_type=code&scope=${scope}`;
  }

  if (provider.type === "Kwai") {
    return `${endpoint}?app_id=${provider.clientId}&redirect_uri=${redirectUri}&state=${state}&response_type=code&scope=${scope}`;
  }

  if (normalizedType === "Custom") {
    const customAuthUrl = provider.customAuthUrl;
    if (!customAuthUrl) {
      return null;
    }

    let authUrl = `${customAuthUrl}?client_id=${provider.clientId}&redirect_uri=${redirectUri}&scope=${scope}&response_type=code&state=${state}`;
    if (provider.enablePkce) {
      authUrl += `&code_challenge=${codeChallenge}&code_challenge_method=S256`;
    }
    return authUrl;
  }

  if (provider.type === "Bilibili") {
    return `${endpoint}#/?client_id=${provider.clientId}&return_url=${redirectUri}&state=${state}&response_type=code`;
  }

  if (provider.type === "Deezer") {
    return `${endpoint}?app_id=${provider.clientId}&redirect_uri=${redirectUri}&perms=${scope}`;
  }

  if (provider.type === "Lastfm") {
    return `${endpoint}?api_key=${provider.clientId}&cb=${redirectUri}`;
  }

  if (provider.type === "Shopify") {
    return `${endpoint}?client_id=${provider.clientId}&redirect_uri=${redirectUri}&scope=${scope}&state=${state}&grant_options[]=per-user`;
  }

  if (provider.type === "Twitter" || provider.type === "Fitbit") {
    return `${endpoint}?client_id=${provider.clientId}&redirect_uri=${redirectUri}&state=${state}&response_type=code&scope=${scope}&code_challenge=${codeChallenge}&code_challenge_method=S256`;
  }

  if (provider.type === "Telegram") {
    return `${redirectOrigin}/telegram-login?state=${state}`;
  }

  return null;
}
