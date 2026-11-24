import type { ReactNode } from "react";
import { Alert, Avatar, Button, Space, Typography, theme } from "antd";
import { useTranslation } from "react-i18next";
import type { SigninItemConfig } from "@/shared/lib/auth-signin-items";
import { sanitizeHtml } from "@/shared/lib/dom-purify";
import { compositeOver, parseColor, pickReadableText } from "@/app/theme/contrast";

type RenderSigninItemCss = (item: SigninItemConfig) => ReactNode;

export type { RenderSigninItemCss };

export type BaseSigninItemProps = {
  item: SigninItemConfig;
  itemKey: string;
  renderSigninItemCss: RenderSigninItemCss;
};

// ─────────────────────────────────────────────
// Protocol Alerts
// ─────────────────────────────────────────────

type ProtocolAlertsProps = {
  authApplicationError?: string;
  authApplicationName: string;
  brandedApplicationError?: string;
  isBrandedApplicationError: boolean;
  isCasLogin: boolean;
  isOauthAuthorize: boolean;
  isSamlLogin: boolean;
  isSupportedOauthResponseType: boolean;
  loginDefaultsApplication: string;
  mfaSetupCompleted: boolean;
  pendingMfa: boolean;
  samlApplicationName?: string;
  samlRequest?: string;
  service?: string;
  unsupportedOauthResponseType?: string;
};

export function ProtocolAlerts({
  authApplicationError,
  authApplicationName,
  isBrandedApplicationError,
  isCasLogin,
  isOauthAuthorize,
  isSamlLogin,
  isSupportedOauthResponseType,
  loginDefaultsApplication,
  mfaSetupCompleted,
  pendingMfa,
  samlApplicationName,
  samlRequest,
  service,
  unsupportedOauthResponseType,
}: ProtocolAlertsProps) {
  const { t } = useTranslation("auth");
  return (
    <>
      {isOauthAuthorize ? (
        <Alert
          showIcon
          type={isSupportedOauthResponseType ? "info" : "warning"}
          title={
            isSupportedOauthResponseType
              ? t("oauthResolved", { appName: authApplicationName || loginDefaultsApplication })
              : t("oauthUnsupportedType", { type: unsupportedOauthResponseType || "unknown" })
          }
          description={authApplicationError}
        />
      ) : null}

      {isCasLogin ? (
        <Alert
          showIcon
          type="info"
          title={t("casResolved", { appName: authApplicationName || loginDefaultsApplication })}
          description={service ? t("casWithService", { service }) : t("casNoService")}
        />
      ) : null}

      {isSamlLogin ? (
        <Alert
          showIcon
          type={samlRequest ? "info" : "warning"}
          title={t("samlResolved", { appName: samlApplicationName || loginDefaultsApplication })}
          description={samlRequest ? t("samlWithRequest") : t("samlNoRequest")}
        />
      ) : null}

      {mfaSetupCompleted && !pendingMfa ? (
        <Alert
          showIcon
          type="success"
          title={t("mfaConfigured")}
          description={t("mfaConfiguredDesc")}
        />
      ) : null}

      {isBrandedApplicationError && !isOauthAuthorize && !isCasLogin ? (
        <Alert showIcon type="warning" title={t("appBrandLoadFailed")} />
      ) : null}
    </>
  );
}

// ─────────────────────────────────────────────
// Login Method Support Alert
// ─────────────────────────────────────────────

type LoginMethodSupportAlertProps = {
  currentLoginMethodSupported: boolean;
  signinMethodTabsCount: number;
};

export function LoginMethodSupportAlert({
  currentLoginMethodSupported,
  signinMethodTabsCount,
}: LoginMethodSupportAlertProps) {
  const { t } = useTranslation("auth");
  if (currentLoginMethodSupported || signinMethodTabsCount === 0) {
    return null;
  }

  return (
    <Alert
      showIcon
      type="warning"
      title={t("firstLoginMethodNotMigrated")}
      description={t("firstLoginMethodNotMigratedDesc")}
    />
  );
}

// ─────────────────────────────────────────────
// Split Layout Side
// ─────────────────────────────────────────────

type SplitLayoutSideProps = {
  description?: string | null;
  displayName?: string | null;
  isSplitLayout: boolean;
  name?: string | null;
  sideHtml?: string | null;
};

export function SplitLayoutSide({
  description,
  displayName,
  isSplitLayout,
  name,
  sideHtml,
}: SplitLayoutSideProps) {
  const { t } = useTranslation("auth");
  const { token } = theme.useToken();
  if (!isSplitLayout) {
    return null;
  }

  // 侧栏品牌面跟随解析后的主色（个人主题 + 组织主题覆盖），不再写死 teal。
  // 顶部混入更多黑形成纵深渐变；文字色按较浅的底（bottom）取可读前景，
  // 浅色主色（组织自定义）时自动改深色字，保证两种主题模式都达 AA。
  const primary = token.colorPrimary;
  const toRgb = (c: { r: number; g: number; b: number }) => `rgb(${c.r}, ${c.g}, ${c.b})`;
  const top = toRgb(compositeOver("rgba(0, 0, 0, 0.35)", primary));
  const bottom = toRgb(compositeOver("rgba(0, 0, 0, 0.12)", primary));
  const onPrimary = pickReadableText(bottom);
  const text = parseColor(onPrimary);
  const onPrimaryMuted = toRgb(
    compositeOver(`rgba(${text.r}, ${text.g}, ${text.b}, 0.88)`, bottom),
  );

  return (
    <div
      className="side-image"
      style={{
        borderRadius: 20,
        padding: 24,
        background: `linear-gradient(180deg, ${top} 0%, ${bottom} 100%)`,
        color: onPrimary,
        minHeight: 420,
        display: "flex",
        flexDirection: "column",
        justifyContent: "space-between",
      }}
    >
      {sideHtml ? (
        <div
          className="login-side-html"
          dangerouslySetInnerHTML={{ __html: sanitizeHtml(sideHtml ?? "") }}
        />
      ) : (
        <Space orientation="vertical" size={16}>
          <Typography.Title level={3} style={{ color: "inherit", margin: 0 }}>
            {displayName || name || "Admin"}
          </Typography.Title>
          <Typography.Paragraph style={{ color: onPrimaryMuted, marginBottom: 0 }}>
            {description || t("formDescription")}
          </Typography.Paragraph>
        </Space>
      )}
    </div>
  );
}

// ─────────────────────────────────────────────
// Form Heading
// ─────────────────────────────────────────────

type FormHeadingProps = {
  description: string;
  title: string;
  visible: boolean;
};

export function FormHeading({ description, title, visible }: FormHeadingProps) {
  if (!visible) {
    return null;
  }

  return (
    <>
      <Typography.Title level={2} style={{ marginBottom: 8 }}>
        {title}
      </Typography.Title>
      <Typography.Paragraph style={{ marginBottom: 0 }}>{description}</Typography.Paragraph>
    </>
  );
}

// ─────────────────────────────────────────────
// Provider Only Intro
// ─────────────────────────────────────────────

type ProviderOnlyIntroProps = {
  applicationName: string;
  homepageUrl?: string | null;
  visible: boolean;
};

export function ProviderOnlyIntro({
  applicationName,
  homepageUrl,
  visible,
}: ProviderOnlyIntroProps) {
  const { t } = useTranslation("auth");
  if (!visible) {
    return null;
  }

  return (
    <Typography.Paragraph style={{ marginBottom: 0, fontSize: 16, textAlign: "left" }}>
      {t("visit")}{" "}
      {homepageUrl ? (
        <a href={homepageUrl} rel="noreferrer" target="_blank">
          <strong>{applicationName}</strong>
        </a>
      ) : (
        <strong>{applicationName}</strong>
      )}
      ：
    </Typography.Paragraph>
  );
}

// ─────────────────────────────────────────────
// Signed In Continuation
// ─────────────────────────────────────────────

type SignedInContinuationProps = {
  account: {
    avatar?: string | null;
    displayName?: string | null;
    name: string;
    owner: string;
  } | null;
  accountLabel: string;
  onContinue: () => void;
  onUseOther: () => void;
  visible: boolean;
};

export function SignedInContinuation({
  account,
  accountLabel,
  onContinue,
  onUseOther,
  visible,
}: SignedInContinuationProps) {
  const { t } = useTranslation("auth");
  if (!visible || !account) {
    return null;
  }

  return (
    <Space orientation="vertical" size={16} style={{ width: "100%" }}>
      <Typography.Text style={{ fontSize: 16 }}>{t("continueWithAccount")}</Typography.Text>
      <Button
        block
        size="large"
        type="default"
        onClick={onContinue}
        style={{
          height: "auto",
          padding: 16,
          display: "flex",
          alignItems: "center",
          justifyContent: "flex-start",
          gap: 12,
        }}
      >
        <Avatar src={account.avatar} size={40}>
          {(account.displayName || account.name).slice(0, 1)}
        </Avatar>
        <Space orientation="vertical" size={2} style={{ alignItems: "flex-start" }}>
          <Typography.Text strong>{accountLabel}</Typography.Text>
          <Typography.Text type="secondary">{account.owner}</Typography.Text>
        </Space>
      </Button>
      <Button type="link" block onClick={onUseOther}>
        {t("orLoginOther")}
      </Button>
    </Space>
  );
}
