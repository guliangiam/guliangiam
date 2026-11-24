import { Avatar, type AvatarProps } from "antd";
import { type FC, useMemo } from "react";

/**
 * Brand configuration for each OAuth/SAML/Web3 provider.
 * - bg: brand background color for the avatar
 * - fg: text color on the avatar
 * - label: short display text (1-3 chars)
 */
type ProviderBrand = {
  bg: string;
  fg: string;
  label: string;
};

const PROVIDER_BRANDS: Record<string, ProviderBrand> = {
  // === Major OAuth Providers ===
  GitHub: { bg: "#24292f", fg: "#ffffff", label: "" },
  Google: { bg: "#ffffff", fg: "#4285f4", label: "G" },
  AzureAD: { bg: "#0078d4", fg: "#ffffff", label: "" },
  AzureADB2C: { bg: "#0078d4", fg: "#ffffff", label: "B2C" },
  WeChat: { bg: "#07c160", fg: "#ffffff", label: "" },
  DingTalk: { bg: "#0089ff", fg: "#ffffff", label: "" },
  Lark: { bg: "#3370ff", fg: "#ffffff", label: "" },
  Facebook: { bg: "#1877f2", fg: "#ffffff", label: "f" },
  Twitter: { bg: "#1d9bf0", fg: "#ffffff", label: "X" },
  LinkedIn: { bg: "#0a66c2", fg: "#ffffff", label: "in" },
  GitLab: { bg: "#fc6d26", fg: "#ffffff", label: "" },
  Slack: { bg: "#4a154b", fg: "#ffffff", label: "S" },
  Discord: { bg: "#5865f2", fg: "#ffffff", label: "" },
  Apple: { bg: "#000000", fg: "#ffffff", label: "" },
  Okta: { bg: "#007dc1", fg: "#ffffff", label: "" },
  ADFS: { bg: "#0078d4", fg: "#ffffff", label: "AD" },
  GuliangIAM: { bg: "#1677ff", fg: "#ffffff", label: "GL" },

  // === Chinese Providers ===
  Alipay: { bg: "#1677ff", fg: "#ffffff", label: "" },
  Baidu: { bg: "#2932e1", fg: "#ffffff", label: "" },
  Douyin: { bg: "#000000", fg: "#fe2c55", label: "" },
  QQ: { bg: "#12b7f5", fg: "#ffffff", label: "QQ" },
  Weibo: { bg: "#e6162d", fg: "#ffffff", label: "" },
  Bilibili: { bg: "#00a1d6", fg: "#ffffff", label: "B" },
  Gitee: { bg: "#c71d23", fg: "#ffffff", label: "" },
  Kwai: { bg: "#ff4906", fg: "#ffffff", label: "" },
  WeCom: { bg: "#07c160", fg: "#ffffff", label: "" },
  Infoflow: { bg: "#2b7ce9", fg: "#ffffff", label: "" },
  Steam: { bg: "#1b2838", fg: "#66c0f4", label: "" },
  Telegram: { bg: "#26a5e4", fg: "#ffffff", label: "" },

  // === International Providers ===
  Amazon: { bg: "#ff9900", fg: "#000000", label: "a" },
  Auth0: { bg: "#eb5424", fg: "#ffffff", label: "" },
  BattleNet: { bg: "#005c99", fg: "#ffffff", label: "" },
  Bitbucket: { bg: "#0052cc", fg: "#ffffff", label: "" },
  Box: { bg: "#0061d5", fg: "#ffffff", label: "" },
  CloudFoundry: { bg: "#0c5d9b", fg: "#ffffff", label: "CF" },
  Dailymotion: { bg: "#00aaff", fg: "#ffffff", label: "" },
  Deezer: { bg: "#a238ff", fg: "#ffffff", label: "" },
  DigitalOcean: { bg: "#0080ff", fg: "#ffffff", label: "" },
  Dropbox: { bg: "#0061ff", fg: "#ffffff", label: "" },
  EveOnline: { bg: "#e23028", fg: "#ffffff", label: "" },
  Fitbit: { bg: "#00b0b9", fg: "#ffffff", label: "" },
  Gitea: { bg: "#609926", fg: "#ffffff", label: "" },
  Heroku: { bg: "#430098", fg: "#ffffff", label: "" },
  InfluxCloud: { bg: "#5951f8", fg: "#ffffff", label: "" },
  Instagram: { bg: "#e4405f", fg: "#ffffff", label: "" },
  Intercom: { bg: "#1f8ded", fg: "#ffffff", label: "" },
  Kakao: { bg: "#fee500", fg: "#000000", label: "" },
  Lastfm: { bg: "#d51007", fg: "#ffffff", label: "" },
  Mailru: { bg: "#005ff9", fg: "#ffffff", label: "" },
  MailRu: { bg: "#005ff9", fg: "#ffffff", label: "" },
  Meetup: { bg: "#ed1c40", fg: "#ffffff", label: "" },
  MicrosoftOnline: { bg: "#00a4ef", fg: "#ffffff", label: "" },
  Naver: { bg: "#03c75a", fg: "#ffffff", label: "N" },
  Nextcloud: { bg: "#0082c9", fg: "#ffffff", label: "" },
  OneDrive: { bg: "#0078d4", fg: "#ffffff", label: "" },
  Oura: { bg: "#538bf5", fg: "#ffffff", label: "" },
  Patreon: { bg: "#ff424d", fg: "#ffffff", label: "" },
  PayPal: { bg: "#003087", fg: "#ffffff", label: "" },
  SalesForce: { bg: "#00a1e0", fg: "#ffffff", label: "" },
  Shopify: { bg: "#96bf48", fg: "#ffffff", label: "" },
  Soundcloud: { bg: "#ff5500", fg: "#ffffff", label: "" },
  SoundCloud: { bg: "#ff5500", fg: "#ffffff", label: "" },
  Spotify: { bg: "#1db954", fg: "#ffffff", label: "" },
  Strava: { bg: "#fc4c02", fg: "#ffffff", label: "" },
  Stripe: { bg: "#635bff", fg: "#ffffff", label: "" },
  Tumblr: { bg: "#36465d", fg: "#ffffff", label: "" },
  Twitch: { bg: "#9146ff", fg: "#ffffff", label: "" },
  Typetalk: { bg: "#2b6084", fg: "#ffffff", label: "" },
  Uber: { bg: "#000000", fg: "#ffffff", label: "" },
  VK: { bg: "#0077ff", fg: "#ffffff", label: "" },
  Wepay: { bg: "#003087", fg: "#ffffff", label: "" },
  Xero: { bg: "#13b5ea", fg: "#ffffff", label: "" },
  Yahoo: { bg: "#6001d2", fg: "#ffffff", label: "" },
  Yammer: { bg: "#106ebe", fg: "#ffffff", label: "" },
  Yandex: { bg: "#fc3f1d", fg: "#ffffff", label: "" },
  Zoom: { bg: "#2d8cff", fg: "#ffffff", label: "" },
  TikTok: { bg: "#000000", fg: "#fe2c55", label: "" },
  Line: { bg: "#00b900", fg: "#ffffff", label: "" },

  // === SAML ===
  SAML: { bg: "#6b7280", fg: "#ffffff", label: "S" },

  // === Custom ===
  Custom: { bg: "#8b5cf6", fg: "#ffffff", label: "" },

  // === WeChat sub-types ===
  WeChatMiniProgram: { bg: "#07c160", fg: "#ffffff", label: "MP" },
};

/**
 * SVG path data for providers with recognizable icons.
 * These are simplified brand marks for use in Avatar components.
 */
const PROVIDER_SVG_PATHS: Record<string, string> = {
  GitHub:
    "M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z",
  Google:
    "M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 01-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z",
  GitLab:
    "M12 2L4.27 7.56l2.97 9.14h9.52l2.97-9.14L12 2zm0 2.48l4.97 3.4-1.9 5.84H8.93L7.03 7.88 12 4.48z",
  Discord:
    "M20.317 4.37a19.791 19.791 0 00-4.885-1.515.074.074 0 00-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 00-5.487 0 12.64 12.64 0 00-.617-1.25.077.077 0 00-.079-.037A19.736 19.736 0 003.677 4.37a.07.07 0 00-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 00.031.057 19.9 19.9 0 005.993 3.03.078.078 0 00.084-.028c.462-.63.874-1.295 1.226-1.994a.076.076 0 00-.041-.106 13.107 13.107 0 01-1.872-.892.077.077 0 01-.008-.128 10.2 10.2 0 00.372-.292.074.074 0 01.077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 01.078.01c.12.098.246.198.373.292a.077.077 0 01-.006.127 12.299 12.299 0 01-1.873.892.077.077 0 00-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 00.084.028 19.839 19.839 0 006.002-3.03.077.077 0 00.032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 00-.031-.03z",
  Slack:
    "M5.042 15.165a2.083 2.083 0 11-4.167 0 2.083 2.083 0 014.167 0zm9.126-4.042a2.083 2.083 0 11-4.167 0 2.083 2.083 0 014.167 0zm-4.126 4.042a2.083 2.083 0 11-4.167 0 2.083 2.083 0 014.167 0zm9.126-4.042a2.083 2.083 0 11-4.167 0 2.083 2.083 0 014.167 0z",
  Apple:
    "M17.05 20.28c-.98.95-2.05.88-3.08.4-1.09-.5-2.08-.48-3.24 0-1.44.62-2.2.44-3.06-.4C2.79 15.25 3.51 7.59 9.05 7.31c1.35.07 2.29.74 3.08.8 1.18-.24 2.31-.93 3.57-.84 1.51.12 2.65.72 3.4 1.8-3.12 1.87-2.38 5.98.48 7.13-.57 1.5-1.31 2.99-2.54 4.09zM12.03 7.25c-.15-2.23 1.66-4.07 3.74-4.25.29 2.58-2.34 4.5-3.74 4.25z",
  MetaMask:
    "M21.33 2L12 9.2 2.67 2 1 3.33V20.67L2.67 22H21.33L23 20.67V3.33L21.33 2zM12 12.8l-4-3.2V20H4V4.67l8 6.4 8-6.4V20h-4V9.6l-4 3.2z",
  WeChat:
    "M8.5 6C5.46 6 3 8.24 3 11c0 1.56.75 2.97 1.94 3.93L4.5 17l2.1-1.05c.6.2 1.24.3 1.9.3.32 0 .63-.03.94-.08A5.58 5.58 0 019 14.5C9 11.46 11.46 9 14.5 9c.17 0 .34.01.5.03C14.17 7.28 11.55 6 8.5 6zm14 8.5c0-2.49-2.46-4.5-5.5-4.5S11.5 12.01 11.5 14.5s2.46 4.5 5.5 4.5c.52 0 1.02-.07 1.49-.19L20.5 20l-.39-1.56C21.13 17.59 22.5 16.17 22.5 14.5z",
};

function getProviderBrand(type: string): ProviderBrand {
  const key = type?.trim();
  if (!key) return { bg: "#6b7280", fg: "#ffffff", label: "?" };
  // Try exact match first, then case-insensitive
  return (
    PROVIDER_BRANDS[key] ??
    PROVIDER_BRANDS[key.charAt(0).toUpperCase() + key.slice(1)] ?? {
      bg: "#6b7280",
      fg: "#ffffff",
      label: key.slice(0, 2).toUpperCase(),
    }
  );
}

type ProviderIconProps = {
  type: string;
  size?: number;
} & Omit<AvatarProps, "size" | "style">;

/**
 * Renders a branded avatar icon for an OAuth/SAML/Web3 provider.
 * Uses SVG brand marks where available, falling back to colored initials.
 */
export const ProviderIcon: FC<ProviderIconProps> = ({ type, size = 32, ...rest }) => {
  const brand = useMemo(() => getProviderBrand(type), [type]);
  const svgPath =
    PROVIDER_SVG_PATHS[type?.trim()] ??
    PROVIDER_SVG_PATHS[type?.charAt(0).toUpperCase() + type?.slice(1)];

  if (svgPath) {
    return (
      <Avatar size={size} style={{ backgroundColor: brand.bg, flexShrink: 0 }} {...rest}>
        {/* WCAG 1.1.1 Non-text Content — SVG needs role="img" and aria-label (A-01) */}
        <svg
          role="img"
          aria-label={`${type} provider icon`}
          width={size * 0.6}
          height={size * 0.6}
          viewBox="0 0 24 24"
          fill={brand.fg}
          xmlns="http://www.w3.org/2000/svg"
        >
          <title>{type} icon</title>
          <path d={svgPath} />
        </svg>
      </Avatar>
    );
  }

  return (
    <Avatar
      size={size}
      style={{
        backgroundColor: brand.bg,
        color: brand.fg,
        fontSize: size * 0.4,
        fontWeight: 600,
        flexShrink: 0,
      }}
      {...rest}
    >
      {brand.label}
    </Avatar>
  );
};

/**
 * Returns the brand color for a provider type (useful for button styling).
 */
// oxlint-disable-next-line react/only-export-components -- 品牌色查询与 ProviderIcon 共用 PROVIDER_BRANDS，拆分需改动 ProviderIcon/index.ts 的导出方
export function getProviderColor(type: string): string {
  return getProviderBrand(type).bg;
}

// oxlint-disable-next-line react/only-export-components -- 品牌表是 ProviderIcon 的数据依赖，外部经 ProviderIcon/index.ts 复用
export { PROVIDER_BRANDS, type ProviderBrand };
