import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { Form } from "antd";

import {
  LoginSigninForm,
  type SigninFormAuthState,
  type SigninFormFlowState,
  type SigninFormProviderState,
  type SigninFormOrganizationState,
  type SigninFormUiState,
} from "./LoginSigninForm";
import { SigninItemConfig } from "@/shared/lib/auth-signin-items";
import type { ProviderSigninOption } from "@/shared/lib/auth-provider-signin";
import type { DirectAuthProvider } from "@/shared/lib/provider-auth";
import type { ApplicationItem } from "@/shared/types/domain";
import { createTestWrapper } from "../../test-utils/render";

// ─────────────────────────────────────────────
// Mocks
// ─────────────────────────────────────────────

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: "en" },
  }),
  Trans: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("@/shared/lib/dom-purify", () => ({
  sanitizeHtml: (html: string) => html,
  sanitizeCss: (css: string) => css,
}));

vi.mock("@/shared/lib/auth-signin-items", () => ({
  getUsernamePlaceholder: (method: string) => (method === "email" ? "Email" : "Username"),
  isCustomSigninItem: (item: SigninItemConfig) => item.name === "Custom",
  isSigninItemShown: (item: SigninItemConfig) => item.visible !== false,
}));

vi.mock("./LoginAuxiliarySigninItems", () => ({
  SigninMethodsItem: ({ itemKey }: { itemKey: string }) => (
    <div data-testid="signin-methods" data-key={itemKey}>
      Signin Methods
    </div>
  ),
  SelectOrganizationSigninItem: ({ itemKey }: { itemKey: string }) => (
    <div data-testid="select-org" data-key={itemKey}>
      Select Organization
    </div>
  ),
  UsernameSigninItem: ({ itemKey }: { itemKey: string }) => (
    <div data-testid="username-item" data-key={itemKey}>
      Username
    </div>
  ),
  PasswordSigninItem: ({ itemKey }: { itemKey: string }) => (
    <div data-testid="password-item" data-key={itemKey}>
      Password
    </div>
  ),
  VerificationCodeSigninItem: ({ itemKey }: { itemKey: string }) => (
    <div data-testid="verification-code" data-key={itemKey}>
      Verification Code
    </div>
  ),
  AgreementSigninItem: ({ itemKey }: { itemKey: string }) => (
    <div data-testid="agreement-item" data-key={itemKey}>
      Agreement
    </div>
  ),
  CaptchaSigninItem: ({ itemKey }: { itemKey: string }) => (
    <div data-testid="captcha-item" data-key={itemKey}>
      Captcha
    </div>
  ),
  LoginButtonSigninItem: ({ itemKey }: { itemKey: string }) => (
    <div data-testid="login-button" data-key={itemKey}>
      Login Button
    </div>
  ),
  ForgotPasswordSigninItem: ({ itemKey }: { itemKey: string }) => (
    <div data-testid="forgot-password" data-key={itemKey}>
      Forgot Password
    </div>
  ),
  SignupLinkSigninItem: ({ itemKey }: { itemKey: string }) => (
    <div data-testid="signup-link" data-key={itemKey}>
      Signup Link
    </div>
  ),
  ProvidersSigninItem: ({ itemKey }: { itemKey: string }) => (
    <div data-testid="providers-item" data-key={itemKey}>
      Providers
    </div>
  ),
  LoginMethodSupportAlert: () => <div data-testid="method-support-alert">Alert</div>,
}));

vi.mock("./GoogleOneTapVirtualButton", () => ({
  default: () => <div data-testid="google-one-tap">Google One Tap</div>,
}));

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

const createWrapper = createTestWrapper;

const defaultAuthState: SigninFormAuthState = {
  agreementRequired: false,
  availableCountryCodes: ["+86", "+1"],
  currentLoginMethodSupported: true,
  forgetPasswordUrl: null,
  googleOneTapProvider: null,
  hasInlineSigninForm: true,
  hasProviderHintSupport: false,
  hasSupportedSigninMethod: true,
  isAuthApplicationError: false,
  isAuthApplicationLoading: false,
  isOauthAuthorize: false,
  isSamlLogin: false,
  isSupportedOauthResponseType: true,
  providerSigninOptions: [],
  resolvedApplication: {
    id: "app-1",
    name: "default",
    displayName: "Default App",
    enableSignUp: true,
  } as ApplicationItem,
  samlRequest: undefined,
  showProviders: false,
  showSelectOrganization: false,
  signinItems: [
    { name: "Username", visible: true },
    { name: "Password", visible: true },
    { name: "Login button", visible: true },
  ],
  signinMethodTabs: [{ key: "password", label: "Password" }],
  signupUrl: null,
  termsOfUseEnabled: false,
};

const defaultFlowState: SigninFormFlowState = {
  captchaReloadSeed: 0,
  captchaRequired: false,
  captchaState: null,
  reloadCaptcha: vi.fn<(...args: unknown[]) => unknown>(),
  sendCodePending: false,
  qrSigninState: { status: "idle", code: null, expiresIn: null, error: null },
  onReloadQrSignin: vi.fn<(...args: unknown[]) => unknown>(),
  onSendCode: vi.fn<(...args: unknown[]) => unknown>(),
};

const defaultProviderState: SigninFormProviderState = {
  canRenderGoogleOneTap: false,
  handleProviderClick: vi.fn<(...args: unknown[]) => unknown>(),
};

const defaultOrganizationState: SigninFormOrganizationState = {
  fallbackOrganizationInputDisabled: false,
  onSelectOrganization: vi.fn<(...args: unknown[]) => unknown>(),
  organizationOptions: [],
  organizationOptionsError: undefined,
  organizationOptionsIsError: false,
  organizationOptionsLoading: false,
};

const defaultUiState: SigninFormUiState = {
  itemSigninContextKey: "login",
  loadButtonLoading: false,
  locationSearch: "",
  loginMethod: "password",
  onChangeLoginMethod: vi.fn<(...args: unknown[]) => unknown>(),
  onFinish: vi.fn<(...args: unknown[]) => unknown>(),
  onOpenTerms: vi.fn<(...args: unknown[]) => unknown>(),
  onStoreSigninUrl: vi.fn<(...args: unknown[]) => unknown>(),
};

const MockCaptchaWidget = () => <div data-testid="captcha-widget">Captcha</div>;

function TestFormWrapper({
  authState = defaultAuthState,
  flowState = defaultFlowState,
  providerState = defaultProviderState,
  organizationState = defaultOrganizationState,
  uiState = defaultUiState,
}: {
  authState?: SigninFormAuthState;
  flowState?: SigninFormFlowState;
  providerState?: SigninFormProviderState;
  organizationState?: SigninFormOrganizationState;
  uiState?: SigninFormUiState;
}) {
  const [form] = Form.useForm();

  return (
    <LoginSigninForm
      CaptchaWidget={MockCaptchaWidget}
      authState={authState}
      flowState={flowState}
      providerState={providerState}
      organizationState={organizationState}
      uiState={uiState}
      form={form}
    />
  );
}

// ─────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────

describe("LoginSigninForm", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("renders form structure", () => {
    it("renders the form element", () => {
      render(<TestFormWrapper />, { wrapper: createWrapper() });
      expect(screen.getByTestId("method-support-alert")).toBeInTheDocument();
    });

    it("renders login method support alert", () => {
      render(<TestFormWrapper />, { wrapper: createWrapper() });
      expect(screen.getByTestId("method-support-alert")).toBeInTheDocument();
    });

    it("renders hidden application field", () => {
      render(<TestFormWrapper />, { wrapper: createWrapper() });
      expect(screen.getByText("application")).toBeInTheDocument();
    });

    it("renders hidden organization field when not showing select", () => {
      render(<TestFormWrapper />, { wrapper: createWrapper() });
      expect(screen.getByText("organization")).toBeInTheDocument();
    });
  });

  describe("renders signin items", () => {
    it("renders Username signin item", () => {
      render(<TestFormWrapper />, { wrapper: createWrapper() });
      expect(screen.getByTestId("username-item")).toBeInTheDocument();
    });

    it("renders Password signin item", () => {
      render(<TestFormWrapper />, { wrapper: createWrapper() });
      expect(screen.getByTestId("password-item")).toBeInTheDocument();
    });

    it("renders Login button signin item", () => {
      render(<TestFormWrapper />, { wrapper: createWrapper() });
      expect(screen.getByTestId("login-button")).toBeInTheDocument();
    });

    it("renders Signin methods item", () => {
      const authStateWithMethods: SigninFormAuthState = {
        ...defaultAuthState,
        signinItems: [
          { name: "Signin methods", visible: true },
          { name: "Username", visible: true },
          { name: "Password", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateWithMethods} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("signin-methods")).toBeInTheDocument();
    });

    it("renders Verification code signin item", () => {
      const authStateWithCode: SigninFormAuthState = {
        ...defaultAuthState,
        signinItems: [
          { name: "Username", visible: true },
          { name: "Verification code", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateWithCode} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("verification-code")).toBeInTheDocument();
    });

    it("renders Agreement signin item", () => {
      const authStateWithAgreement: SigninFormAuthState = {
        ...defaultAuthState,
        signinItems: [
          { name: "Username", visible: true },
          { name: "Password", visible: true },
          { name: "Agreement", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateWithAgreement} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("agreement-item")).toBeInTheDocument();
    });

    it("renders Captcha signin item", () => {
      const authStateWithCaptcha: SigninFormAuthState = {
        ...defaultAuthState,
        signinItems: [
          { name: "Username", visible: true },
          { name: "Password", visible: true },
          { name: "Captcha", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateWithCaptcha} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("captcha-item")).toBeInTheDocument();
    });

    it("renders Forgot password signin item", () => {
      const authStateWithForgot: SigninFormAuthState = {
        ...defaultAuthState,
        signinItems: [
          { name: "Username", visible: true },
          { name: "Password", visible: true },
          { name: "Forgot password?", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateWithForgot} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("forgot-password")).toBeInTheDocument();
    });

    it("renders Signup link signin item", () => {
      const authStateWithSignup: SigninFormAuthState = {
        ...defaultAuthState,
        signinItems: [
          { name: "Username", visible: true },
          { name: "Password", visible: true },
          { name: "Signup link", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateWithSignup} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("signup-link")).toBeInTheDocument();
    });

    it("renders Providers signin item", () => {
      const authStateWithProviders: SigninFormAuthState = {
        ...defaultAuthState,
        signinItems: [
          { name: "Username", visible: true },
          { name: "Password", visible: true },
          { name: "Providers", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateWithProviders} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("providers-item")).toBeInTheDocument();
    });

    it("renders Select organization signin item", () => {
      const authStateWithOrg: SigninFormAuthState = {
        ...defaultAuthState,
        signinItems: [
          { name: "Select organization", visible: true },
          { name: "Username", visible: true },
          { name: "Password", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateWithOrg} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("select-org")).toBeInTheDocument();
    });
  });

  describe("organization field visibility", () => {
    it("shows organization select when showSelectOrganization is true", () => {
      const authStateWithOrgSelect: SigninFormAuthState = {
        ...defaultAuthState,
        showSelectOrganization: true,
      };
      render(<TestFormWrapper authState={authStateWithOrgSelect} />, { wrapper: createWrapper() });
      // When showSelectOrganization is true, the hidden org input should not be rendered
      expect(screen.queryByText("organization")).not.toBeInTheDocument();
    });

    it("hides organization select when showSelectOrganization is false", () => {
      render(<TestFormWrapper />, { wrapper: createWrapper() });
      expect(screen.getByText("organization")).toBeInTheDocument();
    });
  });

  describe("Google One Tap", () => {
    it("renders Google One Tap when canRenderGoogleOneTap is true and providers item exists", () => {
      const authStateWithGoogle: SigninFormAuthState = {
        ...defaultAuthState,
        googleOneTapProvider: { id: "google", type: "OIDC" } as DirectAuthProvider,
        signinItems: [
          { name: "Providers", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      const providerStateWithGoogle: SigninFormProviderState = {
        ...defaultProviderState,
        canRenderGoogleOneTap: true,
      };
      render(
        <TestFormWrapper authState={authStateWithGoogle} providerState={providerStateWithGoogle} />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByTestId("google-one-tap")).toBeInTheDocument();
    });

    it("does not render Google One Tap when canRenderGoogleOneTap is false", () => {
      const authStateWithGoogle: SigninFormAuthState = {
        ...defaultAuthState,
        googleOneTapProvider: { id: "google", type: "OIDC" } as DirectAuthProvider,
        signinItems: [
          { name: "Providers", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateWithGoogle} />, { wrapper: createWrapper() });
      expect(screen.queryByTestId("google-one-tap")).not.toBeInTheDocument();
    });
  });

  describe("custom signin items", () => {
    it("renders custom signin item with custom CSS", () => {
      const authStateWithCustom: SigninFormAuthState = {
        ...defaultAuthState,
        signinItems: [
          { name: "Custom", visible: true, customCss: "<div>Custom Content</div>" },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateWithCustom} />, { wrapper: createWrapper() });
      expect(screen.getByText("Custom Content")).toBeInTheDocument();
    });

    it("does not render custom signin item without custom CSS", () => {
      const authStateWithCustom: SigninFormAuthState = {
        ...defaultAuthState,
        signinItems: [
          { name: "Custom", visible: true, customCss: "" },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateWithCustom} />, { wrapper: createWrapper() });
      expect(screen.queryByTestId("login-custom-signin-item")).not.toBeInTheDocument();
    });
  });

  describe("hidden signin items", () => {
    it("does not render hidden signin items", () => {
      const authStateWithHidden: SigninFormAuthState = {
        ...defaultAuthState,
        signinItems: [
          { name: "Username", visible: false },
          { name: "Password", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateWithHidden} />, { wrapper: createWrapper() });
      expect(screen.queryByTestId("username-item")).not.toBeInTheDocument();
    });
  });

  describe("no resolved application", () => {
    it("renders default signin items when resolvedApplication is null (fail-open)", () => {
      const authStateNoApp: SigninFormAuthState = {
        ...defaultAuthState,
        resolvedApplication: null,
      };
      render(<TestFormWrapper authState={authStateNoApp} />, { wrapper: createWrapper() });
      // 品牌回退失败不得吞掉表单：默认登录项必须照常渲染。
      expect(screen.getByTestId("username-item")).toBeInTheDocument();
      expect(screen.getByTestId("password-item")).toBeInTheDocument();
      expect(screen.getByTestId("login-button")).toBeInTheDocument();
    });
  });

  describe("form submission", () => {
    it("form has onFinish handler", () => {
      const mockOnFinish = vi.fn<(...args: unknown[]) => unknown>();
      const uiStateWithFinish: SigninFormUiState = {
        ...defaultUiState,
        onFinish: mockOnFinish,
      };
      render(<TestFormWrapper uiState={uiStateWithFinish} />, { wrapper: createWrapper() });
      // The form element should be present
      const form = document.querySelector("form");
      expect(form).toBeInTheDocument();
    });
  });

  describe("edge cases", () => {
    it("renders with empty signin items", () => {
      const authStateEmpty: SigninFormAuthState = {
        ...defaultAuthState,
        signinItems: [],
      };
      render(<TestFormWrapper authState={authStateEmpty} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("method-support-alert")).toBeInTheDocument();
    });

    it("renders with multiple signin method tabs", () => {
      const authStateMultiMethod: SigninFormAuthState = {
        ...defaultAuthState,
        signinMethodTabs: [
          { key: "password", label: "Password" },
          { key: "verificationCode", label: "Verification Code" },
        ],
      };
      render(<TestFormWrapper authState={authStateMultiMethod} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("method-support-alert")).toBeInTheDocument();
    });

    it("renders with captcha required state", () => {
      const flowStateWithCaptcha: SigninFormFlowState = {
        ...defaultFlowState,
        captchaRequired: true,
        captchaState: {
          type: "default",
          subType: null,
          clientId: "test-key",
          clientSecret: null,
          clientId2: null,
          clientSecret2: null,
        },
      };
      const authStateWithCaptcha: SigninFormAuthState = {
        ...defaultAuthState,
        signinItems: [
          { name: "Username", visible: true },
          { name: "Password", visible: true },
          { name: "Captcha", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      render(
        <TestFormWrapper authState={authStateWithCaptcha} flowState={flowStateWithCaptcha} />,
        { wrapper: createWrapper() },
      );
      expect(screen.getByTestId("captcha-item")).toBeInTheDocument();
    });

    it("renders with loading button state", () => {
      const uiStateLoading: SigninFormUiState = {
        ...defaultUiState,
        loadButtonLoading: true,
      };
      render(<TestFormWrapper uiState={uiStateLoading} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("login-button")).toBeInTheDocument();
    });

    it("renders with auth application error state", () => {
      const authStateError: SigninFormAuthState = {
        ...defaultAuthState,
        isAuthApplicationError: true,
      };
      render(<TestFormWrapper authState={authStateError} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("login-button")).toBeInTheDocument();
    });

    it("renders with auth application loading state", () => {
      const authStateLoading: SigninFormAuthState = {
        ...defaultAuthState,
        isAuthApplicationLoading: true,
      };
      render(<TestFormWrapper authState={authStateLoading} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("login-button")).toBeInTheDocument();
    });

    it("renders with SAML login state", () => {
      const authStateSaml: SigninFormAuthState = {
        ...defaultAuthState,
        isSamlLogin: true,
      };
      render(<TestFormWrapper authState={authStateSaml} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("login-button")).toBeInTheDocument();
    });

    it("renders with OAuth authorize state", () => {
      const authStateOauth: SigninFormAuthState = {
        ...defaultAuthState,
        isOauthAuthorize: true,
      };
      render(<TestFormWrapper authState={authStateOauth} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("login-button")).toBeInTheDocument();
    });

    it("renders with unsupported OAuth response type", () => {
      const authStateUnsupported: SigninFormAuthState = {
        ...defaultAuthState,
        isSupportedOauthResponseType: false,
      };
      render(<TestFormWrapper authState={authStateUnsupported} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("login-button")).toBeInTheDocument();
    });

    it("renders with agreement required", () => {
      const authStateAgreement: SigninFormAuthState = {
        ...defaultAuthState,
        agreementRequired: true,
        signinItems: [
          { name: "Username", visible: true },
          { name: "Password", visible: true },
          { name: "Agreement", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateAgreement} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("agreement-item")).toBeInTheDocument();
    });

    it("renders with terms of use enabled", () => {
      const authStateTerms: SigninFormAuthState = {
        ...defaultAuthState,
        termsOfUseEnabled: true,
        signinItems: [
          { name: "Username", visible: true },
          { name: "Password", visible: true },
          { name: "Agreement", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateTerms} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("agreement-item")).toBeInTheDocument();
    });

    it("renders with provider hint support", () => {
      const authStateHint: SigninFormAuthState = {
        ...defaultAuthState,
        hasProviderHintSupport: true,
        signinItems: [
          { name: "Username", visible: true },
          { name: "Password", visible: true },
          { name: "Providers", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateHint} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("providers-item")).toBeInTheDocument();
    });

    it("renders with inline signin form mode", () => {
      const authStateInline: SigninFormAuthState = {
        ...defaultAuthState,
        hasInlineSigninForm: true,
      };
      render(<TestFormWrapper authState={authStateInline} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("username-item")).toBeInTheDocument();
    });

    it("renders with non-inline signin form mode", () => {
      const authStateNonInline: SigninFormAuthState = {
        ...defaultAuthState,
        hasInlineSigninForm: false,
      };
      render(<TestFormWrapper authState={authStateNonInline} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("username-item")).toBeInTheDocument();
    });

    it("renders with an active QR signin session", () => {
      const flowStateWithQr: SigninFormFlowState = {
        ...defaultFlowState,
        qrSigninState: { status: "pending", code: "qr-payload", expiresIn: 300, error: null },
      };
      render(<TestFormWrapper flowState={flowStateWithQr} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("username-item")).toBeInTheDocument();
    });

    it("renders with an expired QR signin session", () => {
      const flowStateExpired: SigninFormFlowState = {
        ...defaultFlowState,
        qrSigninState: { status: "expired", code: null, expiresIn: null, error: "QR expired" },
      };
      render(<TestFormWrapper flowState={flowStateExpired} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("username-item")).toBeInTheDocument();
    });

    it("renders with send code pending state", () => {
      const flowStatePending: SigninFormFlowState = {
        ...defaultFlowState,
        sendCodePending: true,
      };
      const authStateWithCode: SigninFormAuthState = {
        ...defaultAuthState,
        signinItems: [
          { name: "Username", visible: true },
          { name: "Verification code", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateWithCode} flowState={flowStatePending} />, {
        wrapper: createWrapper(),
      });
      expect(screen.getByTestId("verification-code")).toBeInTheDocument();
    });

    it("renders with organization options loading", () => {
      const orgStateLoading: SigninFormOrganizationState = {
        ...defaultOrganizationState,
        organizationOptionsLoading: true,
      };
      const authStateWithOrg: SigninFormAuthState = {
        ...defaultAuthState,
        showSelectOrganization: true,
        signinItems: [
          { name: "Select organization", visible: true },
          { name: "Username", visible: true },
          { name: "Password", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateWithOrg} organizationState={orgStateLoading} />, {
        wrapper: createWrapper(),
      });
      expect(screen.getByTestId("select-org")).toBeInTheDocument();
    });

    it("renders with organization options error", () => {
      const orgStateError: SigninFormOrganizationState = {
        ...defaultOrganizationState,
        organizationOptionsIsError: true,
        organizationOptionsError: "Failed to load organizations",
      };
      const authStateWithOrg: SigninFormAuthState = {
        ...defaultAuthState,
        showSelectOrganization: true,
        signinItems: [
          { name: "Select organization", visible: true },
          { name: "Username", visible: true },
          { name: "Password", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateWithOrg} organizationState={orgStateError} />, {
        wrapper: createWrapper(),
      });
      expect(screen.getByTestId("select-org")).toBeInTheDocument();
    });

    it("renders with available country codes", () => {
      const authStateWithCodes: SigninFormAuthState = {
        ...defaultAuthState,
        availableCountryCodes: ["+86", "+1", "+44", "+81"],
      };
      render(<TestFormWrapper authState={authStateWithCodes} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("username-item")).toBeInTheDocument();
    });

    it("renders with signup URL", () => {
      const authStateWithSignup: SigninFormAuthState = {
        ...defaultAuthState,
        signupUrl: "/signup",
        signinItems: [
          { name: "Username", visible: true },
          { name: "Password", visible: true },
          { name: "Signup link", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateWithSignup} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("signup-link")).toBeInTheDocument();
    });

    it("renders with forgot password URL", () => {
      const authStateWithForgot: SigninFormAuthState = {
        ...defaultAuthState,
        forgetPasswordUrl: "/forgot-password",
        signinItems: [
          { name: "Username", visible: true },
          { name: "Password", visible: true },
          { name: "Forgot password?", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateWithForgot} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("forgot-password")).toBeInTheDocument();
    });

    it("renders with SAML request", () => {
      const authStateWithSaml: SigninFormAuthState = {
        ...defaultAuthState,
        isSamlLogin: true,
        samlRequest: "base64-encoded-saml-request",
      };
      render(<TestFormWrapper authState={authStateWithSaml} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("login-button")).toBeInTheDocument();
    });

    it("renders with provider signin options", () => {
      const authStateWithProviders: SigninFormAuthState = {
        ...defaultAuthState,
        providerSigninOptions: [
          { name: "google", displayName: "Google", type: "OIDC" },
          { name: "github", displayName: "GitHub", type: "OIDC" },
        ] as ProviderSigninOption[],
        showProviders: true,
        signinItems: [
          { name: "Providers", visible: true },
          { name: "Login button", visible: true },
        ],
      };
      render(<TestFormWrapper authState={authStateWithProviders} />, { wrapper: createWrapper() });
      expect(screen.getByTestId("providers-item")).toBeInTheDocument();
    });
  });
});
