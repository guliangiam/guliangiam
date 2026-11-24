import { useState, type ComponentProps } from "react";
import { App, Form } from "antd";
import { useLocation, useNavigate, useParams } from "react-router-dom";
import { useFaceRecognitionFlow } from "@/shared/hooks/useFaceRecognitionFlow";
import { useCasLoginPageActions } from "@/shared/hooks/useCasLoginPageActions";
import { type LoginMethodKey } from "@/shared/lib/auth-signin-items";
import { useSessionBootstrap } from "@/features/session/useSessionBootstrap";
import { type LoginPageFormValues, useLoginPageAuthActions } from "./useLoginPageAuthActions";
import { useLoginPageAuthContext } from "./useLoginPageAuthContext";
import { useLoginPageFormFlows } from "./useLoginPageFormFlows";
import { useLoginPagePresenterProps } from "./useLoginPagePresenterProps";
import { useLoginPageProviderActions } from "./useLoginPageProviderActions";
import { LoginShellLayout } from "./LoginShellLayout";

type LoginShellProps = Omit<ComponentProps<typeof LoginShellLayout>, "children" | "overlays">;

export function useLoginPageController() {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const location = useLocation();
  const { organizationName, owner, casApplicationName, applicationName } = useParams<{
    organizationName?: string;
    owner?: string;
    casApplicationName?: string;
    applicationName?: string;
  }>();
  const { isLoading: isSessionLoading, account: sessionAccount } = useSessionBootstrap();
  const [form] = Form.useForm<LoginPageFormValues>();
  const [loginMethod, setLoginMethod] = useState<LoginMethodKey>("password");
  const {
    faceRecognitionMode,
    requestFaceRecognition,
    handleFaceRecognitionCancel,
    handleFaceRecognitionVectorOk,
    handleFaceRecognitionImagesOk,
  } = useFaceRecognitionFlow();
  const { changeLanguage, preferredLanguage, storeSigninUrl } = useCasLoginPageActions();

  const authContext = useLoginPageAuthContext({
    form,
    loginMethod,
    pathname: location.pathname,
    search: location.search,
    state: location.state as {
      from?: { pathname?: string; search?: string };
    } | null,
    organizationName,
    owner,
    casApplicationName,
    applicationName,
    sessionAccount,
  });

  const formFlows = useLoginPageFormFlows({
    form,
    loginMethod,
    setLoginMethod,
    authContext,
  });

  const providerActions = useLoginPageProviderActions({
    form,
    pathname: location.pathname,
    search: location.search,
    navigate,
    authContext,
    showError: message.error,
  });

  const authActions = useLoginPageAuthActions({
    form,
    loginMethod,
    pathname: location.pathname,
    search: location.search,
    navigate,
    sessionAccount,
    authContext,
    formFlows,
    requestFaceRecognition,
    isSessionLoading,
    showSuccess: message.success,
    showError: message.error,
    showInfo: message.info,
  });

  const { overlaySectionProps, panelSectionProps, signinContentProps } = useLoginPagePresenterProps(
    {
      authContext,
      authActions,
      faceRecognition: {
        faceRecognitionMode,
        onFaceRecognitionCancel: handleFaceRecognitionCancel,
        onFaceRecognitionImagesOk: handleFaceRecognitionImagesOk,
        onFaceRecognitionVectorOk: handleFaceRecognitionVectorOk,
      },
      form,
      formFlows,
      locationSearch: location.search,
      loginMethod,
      organizationName,
      providerActions,
      sessionAccount,
      shellActions: {
        changeLanguage,
        changeLoginMethod: setLoginMethod,
        preferredLanguage,
        storeSigninUrl,
      },
    },
  );

  const shellProps: LoginShellProps = {
    application: authContext.resolvedApplication,
    customSigninHtml: authContext.customSigninHtml,
    outerBackground: authContext.outerBackground,
    formPositionStyles: authContext.formPositionStyles,
    isSplitLayout: authContext.isSplitLayout,
  };

  return {
    shellProps,
    overlaySectionProps,
    panelSectionProps,
    signinContentProps,
  };
}
