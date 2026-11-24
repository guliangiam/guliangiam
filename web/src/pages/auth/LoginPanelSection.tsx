import { lazy, Suspense, type ReactNode } from "react";
import { Skeleton, Space } from "antd";
import { useTranslation } from "react-i18next";
import type { PendingMfaVerifyValues } from "@/shared/lib/auth-mfa-verify";
import { LOGIN_SHELL_CSS_SCOPE, type SigninItemConfig } from "@/shared/lib/auth-signin-items";
import type { MfaPropsItem, ApplicationItem } from "@/shared/types/domain";
import { sanitizeCss } from "@/shared/lib/dom-purify";
import { FormHeading, ProviderOnlyIntro, SignedInContinuation } from "./LoginAuxiliarySigninItems";
import { LoginHeaderSection } from "./LoginHeaderSection";

const MfaVerifyCard = lazy(async () => {
  const module = await import("./MfaVerifyCard");
  return { default: module.MfaVerifyCard };
});

type PendingMfaLike = {
  mfaProps: MfaPropsItem[];
  challengeState?: string;
  loginValues: {
    application: string;
  };
};

type SessionAccountLike = {
  avatar?: string | null;
  displayName?: string | null;
  name: string;
  owner: string;
} | null;

type LoginPanelSectionProps = {
  application: ApplicationItem | null;
  availableLanguages: string[];
  headerSigninItems: SigninItemConfig[];
  preferredLanguage: string;
  onChangeLanguage: (nextLanguage: string) => void;
  shouldShowFormHeading: boolean;
  shouldShowProviderOnlyIntro: boolean;
  shouldShowSignedInContinuation: boolean;
  sessionAccount: SessionAccountLike;
  sessionAccountLabel: string;
  onContinue: () => void;
  onUseOther: () => void;
  pendingMfa: PendingMfaLike | null;
  onBackPendingMfa: () => void;
  onVerifyPendingMfa?: (values: PendingMfaVerifyValues) => Promise<void>;
  signinContent: ReactNode;
};

function renderSigninItemCss(item: { customCss?: string | null }) {
  if (!item.customCss?.trim()) {
    return null;
  }

  return (
    <style
      dangerouslySetInnerHTML={{ __html: sanitizeCss(item.customCss, LOGIN_SHELL_CSS_SCOPE) }}
    />
  );
}

export function LoginPanelSection(props: LoginPanelSectionProps) {
  const {
    application,
    availableLanguages,
    headerSigninItems,
    onBackPendingMfa,
    onChangeLanguage,
    onContinue,
    onUseOther,
    onVerifyPendingMfa,
    pendingMfa,
    preferredLanguage,
    sessionAccount,
    sessionAccountLabel,
    shouldShowFormHeading,
    shouldShowProviderOnlyIntro,
    shouldShowSignedInContinuation,
    signinContent,
  } = props;

  const { t } = useTranslation("auth");

  return (
    <Space orientation="vertical" size={20} style={{ width: "100%" }}>
      <div>
        {application ? (
          <LoginHeaderSection
            availableLanguages={availableLanguages}
            headerSigninItems={headerSigninItems}
            logoAlt={application.displayName || application.name}
            logoUrl={application.logo}
            onBack={() => {
              window.history.back();
            }}
            onChangeLanguage={onChangeLanguage}
            preferredLanguage={preferredLanguage}
            renderSigninItemCss={renderSigninItemCss}
            resolvedApplicationName={application.name}
            resolvedApplicationOrganization={application.organization}
          />
        ) : null}

        <FormHeading
          description={application?.description || t("defaultFormDescription")}
          title={application?.displayName || "app-built-in"}
          visible={shouldShowFormHeading}
        />
        <ProviderOnlyIntro
          applicationName={application?.displayName || application?.name || "app-built-in"}
          homepageUrl={application?.homepageUrl}
          visible={shouldShowProviderOnlyIntro}
        />
      </div>

      <SignedInContinuation
        account={sessionAccount}
        accountLabel={sessionAccountLabel}
        onContinue={onContinue}
        onUseOther={onUseOther}
        visible={shouldShowSignedInContinuation}
      />

      {pendingMfa ? (
        <Suspense fallback={<Skeleton active paragraph={{ rows: 3 }} />}>
          <MfaVerifyCard
            mfaProps={pendingMfa.mfaProps}
            organizationId={application?.organization ?? undefined}
            applicationId={pendingMfa.loginValues.application || application?.id || undefined}
            challengeState={pendingMfa.challengeState}
            onBack={onBackPendingMfa}
            onVerify={onVerifyPendingMfa ?? (() => Promise.resolve())}
          />
        </Suspense>
      ) : shouldShowSignedInContinuation ? null : (
        signinContent
      )}
    </Space>
  );
}
