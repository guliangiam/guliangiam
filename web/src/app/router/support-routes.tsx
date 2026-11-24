import { lazy } from "react";
import type { RouteObject } from "react-router-dom";
import { withSuspense } from "./shared";

const TicketsPage = lazy(() => import("@/pages/tickets/TicketsPage"));
const TicketCreatePage = lazy(() => import("@/pages/tickets/TicketCreatePage"));
const TicketDetailPage = lazy(() => import("@/pages/tickets/TicketDetailPage"));
const FormsPage = lazy(() => import("@/pages/forms/FormsPage"));
const FormCreatePage = lazy(() => import("@/pages/forms/FormCreatePage"));
const FormDetailPage = lazy(() => import("@/pages/forms/FormDetailPage"));
const ServersPage = lazy(() => import("@/pages/gateway/ServersPage"));
const ServerCreatePage = lazy(() => import("@/pages/gateway/ServerCreatePage"));
const ServerDetailPage = lazy(() => import("@/pages/gateway/ServerDetailPage"));
const SitesPage = lazy(() => import("@/pages/gateway/SitesPage"));
const SiteCreatePage = lazy(() => import("@/pages/gateway/SiteCreatePage"));
const SiteDetailPage = lazy(() => import("@/pages/gateway/SiteDetailPage"));

/** Support & gateway routes — tickets, forms, servers, sites */
export const supportRoutes: RouteObject[] = [
  // ── Tickets ──
  { path: "tickets", element: withSuspense(<TicketsPage />) },
  { path: "tickets/new", element: withSuspense(<TicketCreatePage />) },
  { path: "tickets/:id", element: withSuspense(<TicketDetailPage />) },

  // ── Forms ──
  { path: "forms", element: withSuspense(<FormsPage />) },
  { path: "forms/new", element: withSuspense(<FormCreatePage />) },
  { path: "forms/:id", element: withSuspense(<FormDetailPage />) },

  // ── Gateway Servers ──
  { path: "servers", element: withSuspense(<ServersPage />) },
  { path: "servers/new", element: withSuspense(<ServerCreatePage />) },
  { path: "servers/:organizationName/:serverName", element: withSuspense(<ServerDetailPage />) },

  // ── Gateway Sites ──
  { path: "sites", element: withSuspense(<SitesPage />) },
  { path: "sites/new", element: withSuspense(<SiteCreatePage />) },
  { path: "sites/:organizationName/:siteName", element: withSuspense(<SiteDetailPage />) },
];
