import { lazy } from "react";
import type { RouteObject } from "react-router-dom";
import { Navigate } from "react-router-dom";
import { withSuspense, RouterErrorElement } from "./shared";

const AccountPage = lazy(() => import("@/pages/account/AccountPage"));
const SystemInfoPage = lazy(() => import("@/pages/system-info/SystemInfoPage"));
const SwaggerPage = lazy(() => import("@/pages/swagger/SwaggerPage"));
const ThemeEditorPage = lazy(() => import("@/pages/theme-editor/ThemeEditorPage"));
const OrgSettingsPage = lazy(() => import("@/pages/settings/SettingsPage"));
const I18nLanguagesPage = lazy(() => import("@/pages/i18n/I18nLanguagesPage"));
const I18nTranslationsPage = lazy(() => import("@/pages/i18n/I18nTranslationsPage"));

/** User & system routes — account, system-info, settings, theme-editor, swagger, i18n, 404 catch-all */
export const systemRoutes: RouteObject[] = [
  { path: "account", element: withSuspense(<AccountPage />) },
  { path: "system-info", element: withSuspense(<SystemInfoPage />) },
  { path: "system/settings", element: <Navigate to="/system-info" replace /> },
  { path: "settings", element: withSuspense(<OrgSettingsPage />) },
  { path: "theme-editor", element: withSuspense(<ThemeEditorPage />) },
  { path: "swagger", element: withSuspense(<SwaggerPage />) },
  { path: "i18n/languages", element: withSuspense(<I18nLanguagesPage />) },
  { path: "i18n/translations", element: withSuspense(<I18nTranslationsPage />) },
  { path: "*", element: <RouterErrorElement /> },
];
