import { LoginOverlaySection } from "./LoginOverlaySection";
import { LoginPanelSection } from "./LoginPanelSection";
import { useLoginPageController } from "./useLoginPageController";
import { LoginShellLayout } from "./LoginShellLayout";
import { LoginSigninContent } from "./LoginSigninContent";

export default function LoginPage() {
  const { shellProps, overlaySectionProps, panelSectionProps, signinContentProps } =
    useLoginPageController();

  return (
    <LoginShellLayout {...shellProps} overlays={<LoginOverlaySection {...overlaySectionProps} />}>
      <LoginPanelSection
        {...panelSectionProps}
        signinContent={<LoginSigninContent {...signinContentProps} />}
      />
    </LoginShellLayout>
  );
}
