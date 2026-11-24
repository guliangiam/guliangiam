import { Card } from "antd";
import { useTranslation } from "react-i18next";
import { SectionNavShell } from "@/features/section-nav/SectionNavShell";
import type { SectionNavItem } from "@/features/section-nav/types";
import { FormSection } from "@/shared/ui/FormSection";
import { useAccountPage } from "./useAccountPage";
import { AccountProfileSection } from "./AccountProfileSection";
import { AccountRealNameSection } from "./AccountRealNameSection";
import { AccountLoginsSection } from "./AccountLoginsSection";
import { AccountConsentsSection } from "./AccountConsentsSection";
import { AccountContactBindingSection } from "./AccountContactBindingSection";
import { AccountPasswordSection } from "./AccountPasswordSection";
import { AccountMfaSection } from "./AccountMfaSection";
import { AccountPasskeysSection } from "./AccountPasskeysSection";
import { AccountFaceIdSection } from "./AccountFaceIdSection";
import { AccountSessionSection } from "./AccountSessionSection";
import { AccountWeb3AddressSection } from "./AccountWeb3AddressSection";

export default function AccountPage() {
  const state = useAccountPage();
  const { t } = useTranslation("resource");
  const { t: tc } = useTranslation("common");

  if (state.accountQuery.isLoading) {
    return (
      <div className="gl-content__inner--reading">
        <Card className="gl-panel gl-resource-card" variant="borderless" loading />
      </div>
    );
  }

  const sectionItems: SectionNavItem[] = [
    { key: "profile", anchor: "account-section-profile", label: t("account.profile") },
    { key: "realname", anchor: "account-section-realname", label: tc("realName", "Real Name") },
    { key: "logins", anchor: "account-section-logins", label: t("user.thirdPartyLogin") },
    { key: "security", anchor: "account-section-security", label: t("account.security") },
    { key: "sessions", anchor: "account-section-sessions", label: t("account.sessions") },
  ];

  return (
    <div className="gl-content__inner--reading">
      <Card className="gl-panel gl-resource-card" variant="borderless" style={{ borderRadius: 12 }}>
        <SectionNavShell items={sectionItems}>
          <div className="gl-section-anchors">
            <div id="account-section-profile" className="gl-section-anchor">
              <AccountProfileSection
                account={state.account}
                user={state.user}
                profileForm={state.profileForm}
                updateProfileMutation={state.updateProfileMutation}
                accountItems={state.accountItems}
              />
            </div>

            <div id="account-section-realname" className="gl-section-anchor">
              <FormSection title={t("user.realNameInfoSection")}>
                <AccountRealNameSection
                  user={state.user}
                  realNameForm={state.realNameForm}
                  updateProfileMutation={state.updateProfileMutation}
                  verifyIdMutation={state.verifyIdMutation}
                  idvCollecting={state.idvCollecting}
                />
              </FormSection>
            </div>

            <div id="account-section-logins" className="gl-section-anchor">
              <FormSection title={t("user.thirdPartyLogin")}>
                <AccountLoginsSection
                  identities={state.identities}
                  providers={state.providers}
                  deleteIdentityMutation={state.deleteIdentityMutation}
                  onLinkIdentity={state.linkIdentity}
                />
              </FormSection>
              <AccountConsentsSection consentsQuery={state.consentsQuery} />
            </div>

            <div id="account-section-security" className="gl-section-anchor">
              <div className="gl-section-stack">
                <AccountContactBindingSection
                  organizationId={state.account?.organization?.id ?? ""}
                  onBound={() => {
                    void state.accountQuery.refetch();
                  }}
                />
                <AccountPasswordSection userId={state.userId ?? ""} />
                <AccountMfaSection
                  mfaMethods={state.mfaMethods}
                  preferredMfa={state.preferredMfa}
                  mfaSetupMutation={state.mfaSetupMutation}
                  mfaVerifyMutation={state.mfaVerifyMutation}
                  mfaEnableMutation={state.mfaEnableMutation}
                  mfaDisableMutation={state.mfaDisableMutation}
                  setPreferredMfaMutation={state.setPreferredMfaMutation}
                />
                <AccountPasskeysSection
                  organizationSlug={state.account?.organization?.name ?? ""}
                  username={state.account?.account?.name ?? ""}
                />
                <AccountFaceIdSection
                  user={state.user}
                  faceIds={state.faceIds}
                  registerFaceIdMutation={state.registerFaceIdMutation}
                  revokeFaceIdMutation={state.revokeFaceIdMutation}
                  onGoToRealName={() =>
                    document
                      .getElementById("account-section-realname")
                      ?.scrollIntoView({ behavior: "smooth" })
                  }
                />
                <AccountWeb3AddressSection
                  deviceInfo={state.currentSessionQuery.data?.deviceInfo}
                />
              </div>
            </div>

            <div id="account-section-sessions" className="gl-section-anchor">
              <AccountSessionSection
                sessionsQuery={state.sessionsQuery}
                currentSessionQuery={state.currentSessionQuery}
                revokeSessionMutation={state.revokeSessionMutation}
                revokeOthersMutation={state.revokeOthersMutation}
              />
            </div>
          </div>
        </SectionNavShell>
      </Card>
    </div>
  );
}
