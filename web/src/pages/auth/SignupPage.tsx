import { Alert, Button, Form, Input } from "antd";
import { Trans } from "react-i18next";
import { useSignupPageController } from "./useSignupPageController";
import { SignupFormItems } from "./SignupFormItems";
import { SignupAgreementModal } from "./SignupAgreementModal";
import { CaptchaChallengeField } from "@/shared/ui/CaptchaChallengeField";
import { LoginShellLayout } from "./LoginShellLayout";

export default function SignupPage() {
  const {
    t,
    ta,
    navigate,
    form,
    agreedToTerms,
    setAgreedToTerms,
    agreementModalOpen,
    setAgreementModalOpen,
    passwordValue,
    organizationSlug,
    isOAuthFlow,
    application,
    signupMutation,
    signupItems,
    invitationCode,
    invitationInfo,
    invitationChecking,
    invitationInvalid,
    onFinish,
    handleRegisterClick,
    handleAgreeAndContinue,
    handleSendVerificationCode,
    sendVerificationCodeDisabled,
    captchaRequired,
    captchaState,
    captchaSeed,
    captchaToken,
    setCaptchaToken,
    reloadCaptcha,
    outerBackground,
    formPositionStyles,
    isSplitLayout,
    customSignupHtml,
    signupDisabled,
  } = useSignupPageController();

  return (
    <LoginShellLayout
      application={application ?? null}
      customSigninHtml={customSignupHtml}
      outerBackground={outerBackground}
      formPositionStyles={formPositionStyles}
      isSplitLayout={isSplitLayout}
    >
      <div>
        <h1 className="gl-auth-form__title">{t("auth:createAccount")}</h1>
        <p className="gl-auth-form__subtitle">
          <Trans
            i18nKey="auth:registerInOrg"
            values={{ org: organizationSlug }}
            components={{ org: <strong /> }}
          />
        </p>
      </div>

      {signupDisabled ? (
        <Alert type="warning" showIcon title={ta("signupDisabled")} />
      ) : (
        <>
          {invitationCode ? (
            <Alert
              style={{ marginBottom: 16 }}
              type={invitationInvalid ? "error" : invitationChecking ? "info" : "success"}
              showIcon
              title={
                invitationInvalid
                  ? ta("invitationInvalidOnSignup", "Invitation code is invalid or expired")
                  : invitationChecking
                    ? ta("invitationChecking", "Checking invitation code…")
                    : ta("invitationValidOnSignup", "Invitation accepted: {{email}}", {
                        email: invitationInfo?.email ?? "",
                      })
              }
            />
          ) : null}

          <Form
            form={form}
            layout="vertical"
            scrollToFirstError
            onFinish={onFinish}
            autoComplete="off"
          >
            <Form.Item name="organizationSlug" initialValue={organizationSlug} hidden>
              <Input />
            </Form.Item>

            <SignupFormItems
              items={signupItems}
              passwordValue={passwordValue}
              showCaptcha={!application}
              agreedToTerms={agreedToTerms}
              onAgreedToTermsChange={setAgreedToTerms}
              onOpenAgreement={() => setAgreementModalOpen(true)}
              onSendVerificationCode={handleSendVerificationCode}
              sendVerificationCodeDisabled={sendVerificationCodeDisabled}
              t={t}
            />

            {captchaRequired && (
              <CaptchaChallengeField
                state={captchaState}
                seed={captchaSeed}
                token={captchaToken}
                onChange={setCaptchaToken}
                reload={reloadCaptcha}
              />
            )}

            <Form.Item>
              <Button
                type="primary"
                htmlType="button"
                onClick={handleRegisterClick}
                loading={signupMutation.isPending}
                block
                disabled={signupMutation.isPending || invitationChecking || invitationInvalid}
              >
                {t("auth:signup")}
              </Button>
            </Form.Item>
          </Form>

          <SignupAgreementModal
            open={agreementModalOpen}
            termsOfUse={application?.termsOfUse}
            privacyPolicy={application?.privacyPolicy}
            onAgree={handleAgreeAndContinue}
            onCancel={() => setAgreementModalOpen(false)}
          />
        </>
      )}

      <div style={{ textAlign: "center" }}>
        {t("auth:alreadyHaveAccount")}{" "}
        <Button
          type="link"
          onClick={() =>
            navigate(isOAuthFlow ? `/login/oauth/authorize${location.search}` : "/login", {
              replace: true,
            })
          }
        >
          {t("auth:signIn")}
        </Button>
      </div>
    </LoginShellLayout>
  );
}
