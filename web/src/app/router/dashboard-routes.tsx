import { lazy } from "react";
import type { RouteObject } from "react-router-dom";
import { withSuspense } from "./shared";

const DashboardPage = lazy(() => import("@/pages/dashboard/DashboardPage"));
const OidcDiscoveryPage = lazy(() => import("@/pages/auth/OidcDiscoveryPage"));
const ShortcutsPage = lazy(() => import("@/pages/shortcuts/ShortcutsPage"));
const AppsPage = lazy(() => import("@/pages/apps/AppsPage"));

/** Dashboard and quick-access routes */
export const dashboardRoutes: RouteObject[] = [
  { index: true, element: withSuspense(<DashboardPage />) },
  { path: "oidc-discovery", element: withSuspense(<OidcDiscoveryPage />) },
  { path: "shortcuts", element: withSuspense(<ShortcutsPage />) },
  { path: "apps", element: withSuspense(<AppsPage />) },
];
