import { lazy } from "react";
import type { RouteObject } from "react-router-dom";
import { withSuspense } from "./shared";

const LoginPage = lazy(() => import("@/pages/auth/LoginPage"));
const SignupPage = lazy(() => import("@/pages/auth/SignupPage"));
const ForgotPasswordPage = lazy(() => import("@/pages/auth/ForgotPasswordPage"));
const InvitationRegisterPage = lazy(() => import("@/pages/auth/InvitationRegisterPage"));
const ConsentPage = lazy(() => import("@/pages/auth/ConsentPage"));
const PromptPage = lazy(() => import("@/pages/auth/PromptPage"));
const SamlCallbackPage = lazy(() => import("@/pages/auth/SamlCallbackPage"));
const CasLogoutPage = lazy(() => import("@/pages/auth/CasLogoutPage"));
const TelegramLoginPage = lazy(() => import("@/pages/auth/TelegramLoginPage"));
const SocialCallbackPage = lazy(() => import("@/pages/auth/SocialCallbackPage"));
const AuthResultPage = lazy(() => import("@/pages/auth/AuthResultPage"));
const DeviceVerifyPage = lazy(() => import("@/pages/device/DeviceVerifyPage"));
const PricingSelectPage = lazy(() => import("@/pages/pricings/PricingSelectPage"));

/** Public authentication routes (not under AppShell) */
export const authRoutes: RouteObject[] = [
  { path: "/login", element: withSuspense(<LoginPage />) },
  { path: "/signup", element: withSuspense(<SignupPage />) },
  { path: "/signup/:organizationName", element: withSuspense(<SignupPage />) },
  { path: "/signup/oauth/authorize", element: withSuspense(<SignupPage />) },
  { path: "/forgot-password", element: withSuspense(<ForgotPasswordPage />) },
  { path: "/register/invitation", element: withSuspense(<InvitationRegisterPage />) },
  { path: "/login/:organizationName", element: withSuspense(<LoginPage />) },
  { path: "/login/oauth/authorize", element: withSuspense(<LoginPage />) },
  { path: "/consent/:applicationName", element: withSuspense(<ConsentPage />) },
  { path: "/prompt/:applicationName", element: withSuspense(<PromptPage />) },
  { path: "/login/saml/authorize/:owner/:applicationName", element: withSuspense(<LoginPage />) },
  { path: "/cas/:owner/:casApplicationName/login", element: withSuspense(<LoginPage />) },
  { path: "/cas/:owner/:casApplicationName/logout", element: withSuspense(<CasLogoutPage />) },
  { path: "/callback/saml", element: withSuspense(<SamlCallbackPage />) },
  { path: "/telegram-login", element: withSuspense(<TelegramLoginPage />) },
  { path: "/device", element: withSuspense(<DeviceVerifyPage />) },
  { path: "/auth/result", element: withSuspense(<AuthResultPage />) },
  { path: "/auth/callback/:providerType", element: withSuspense(<SocialCallbackPage />) },
  {
    path: "/select-plan/:orgId/:pricingName",
    element: withSuspense(<PricingSelectPage />),
  },
];
