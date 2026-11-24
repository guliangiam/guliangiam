import { createBrowserRouter } from "react-router-dom";
import { RouterErrorElement, withSuspense } from "./shared";
import { LazyAppShell } from "./LazyAppShell";
import { ResourceGate } from "@/app/layout/ResourceGate";
import { authRoutes } from "./auth-routes";
import { dashboardRoutes } from "./dashboard-routes";
import { identityRoutes } from "./identity-routes";
import { accessControlRoutes } from "./access-control-routes";
import { infraRoutes } from "./infra-routes";
import { securityRoutes } from "./security-routes";
import { aiRoutes } from "./ai-routes";
import { commerceRoutes } from "./commerce-routes";
import { supportRoutes } from "./support-routes";
import { systemRoutes } from "./system-routes";

/**
 * Application router — aggregates domain-specific route modules.
 *
 * Structure:
 *   Public auth routes (top-level, no AppShell)
 *   AppShell route (path: "/")
 *     ├── dashboardRoutes
 *     ├── identityRoutes       (organizations, users, groups, invitations, MFA)
 *     ├── accessControlRoutes  (applications, providers, roles, permissions, sessions, tokens)
 *     ├── infraRoutes          (resources, certs, models, adapters, enforcers, syncers, webhooks, ldaps, records)
 *     ├── securityRoutes       (captcha, notification providers, rules, face-id, radius, entries)
 *     ├── aiRoutes             (AI Gateway management + agents + MCP servers — see ai-routes.tsx)
 *     ├── commerceRoutes       (products, orders, payments, plans, pricings, subscriptions, coupons, keys)
 *     ├── supportRoutes        (tickets, forms, gateway servers/sites)
 *     └── systemRoutes         (account, system-info, settings, theme-editor, swagger, i18n, 404)
 */
export const router = createBrowserRouter([
  ...authRoutes,
  {
    path: "/",
    element: withSuspense(
      <ResourceGate>
        <LazyAppShell />
      </ResourceGate>,
    ),
    errorElement: <RouterErrorElement />,
    children: [
      ...dashboardRoutes,
      ...identityRoutes,
      ...accessControlRoutes,
      ...infraRoutes,
      ...securityRoutes,
      ...aiRoutes,
      ...commerceRoutes,
      ...supportRoutes,
      ...systemRoutes,
    ],
  },
]);
