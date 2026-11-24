import { lazy, type ComponentProps } from "react";
import { ProtocolAlerts } from "./LoginAuxiliarySigninItems";
import { LoginSigninForm } from "./LoginSigninForm";

const CaptchaWidget = lazy(async () => {
  const module = await import("./CaptchaWidget");
  return { default: module.CaptchaWidget };
});

type LoginSigninContentProps = {
  protocolAlertsProps: ComponentProps<typeof ProtocolAlerts>;
  loginSigninFormProps: Omit<ComponentProps<typeof LoginSigninForm>, "CaptchaWidget">;
};

export function LoginSigninContent(props: LoginSigninContentProps) {
  const { loginSigninFormProps, protocolAlertsProps } = props;

  return (
    <>
      <ProtocolAlerts {...protocolAlertsProps} />
      <LoginSigninForm {...loginSigninFormProps} CaptchaWidget={CaptchaWidget} />
    </>
  );
}
