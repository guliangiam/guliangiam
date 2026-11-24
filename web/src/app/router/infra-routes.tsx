import { lazy } from "react";
import type { RouteObject } from "react-router-dom";
import { lazyRouteLoader, withSuspense } from "./shared";
const resourcesListLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/infra-loaders"),
  "resourcesListLoader",
);
const resourceDetailLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/infra-loaders"),
  "resourceDetailLoader",
);
const certsListLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/infra-loaders"),
  "certsListLoader",
);
const certDetailLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/infra-loaders"),
  "certDetailLoader",
);
const syncersListLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/infra-loaders"),
  "syncersListLoader",
);
const syncerDetailLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/infra-loaders"),
  "syncerDetailLoader",
);
const webhooksListLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/infra-loaders"),
  "webhooksListLoader",
);
const webhookDetailLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/infra-loaders"),
  "webhookDetailLoader",
);
const ldapsListLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/infra-loaders"),
  "ldapsListLoader",
);
const ldapDetailLoader = lazyRouteLoader(
  () => import("@/shared/api/loaders/infra-loaders"),
  "ldapDetailLoader",
);

const ResourcesPage = lazy(() => import("@/pages/resources/ResourcesPage"));
const ResourceCreatePage = lazy(() => import("@/pages/resources/ResourceCreatePage"));
const ResourceDetailPage = lazy(() => import("@/pages/resources/ResourceDetailPage"));
const CertsPage = lazy(() => import("@/pages/certs/CertsPage"));
const CertCreatePage = lazy(() => import("@/pages/certs/CertCreatePage"));
const CertDetailPage = lazy(() => import("@/pages/certs/CertDetailPage"));
const ModelsPage = lazy(() => import("@/pages/models/ModelsPage"));
const ModelDetailPage = lazy(() => import("@/pages/models/ModelDetailPage"));
const ModelCreatePage = lazy(() => import("@/pages/models/ModelCreatePage"));
const AdaptersPage = lazy(() => import("@/pages/adapters/AdaptersPage"));
const AdapterDetailPage = lazy(() => import("@/pages/adapters/AdapterDetailPage"));
const AdapterCreatePage = lazy(() => import("@/pages/adapters/AdapterCreatePage"));
const EnforcersPage = lazy(() => import("@/pages/enforcers/EnforcersPage"));
const EnforcerDetailPage = lazy(() => import("@/pages/enforcers/EnforcerDetailPage"));
const EnforcerCreatePage = lazy(() => import("@/pages/enforcers/EnforcerCreatePage"));
const SyncersPage = lazy(() => import("@/pages/syncers/SyncersPage"));
const SyncerCreatePage = lazy(() => import("@/pages/syncers/SyncerCreatePage"));
const SyncerDetailPage = lazy(() => import("@/pages/syncers/SyncerDetailPage"));
const MigrationPage = lazy(() => import("@/pages/migration/MigrationPage"));
const WebhooksPage = lazy(() => import("@/pages/webhooks/WebhooksPage"));
const WebhookCreatePage = lazy(() => import("@/pages/webhooks/WebhookCreatePage"));
const WebhookDetailPage = lazy(() => import("@/pages/webhooks/WebhookDetailPage"));
const LdapsPage = lazy(() => import("@/pages/ldaps/LdapsPage"));
const LdapCreatePage = lazy(() => import("@/pages/ldaps/LdapCreatePage"));
const LdapDetailPage = lazy(() => import("@/pages/ldaps/LdapDetailPage"));
const LdapSyncPage = lazy(() => import("@/pages/ldaps/LdapSyncPage"));
const RecordsPage = lazy(() => import("@/pages/records/RecordsPage"));
const LoginLogsPage = lazy(() => import("@/pages/audit/LoginLogsPage"));
const OperationLogsPage = lazy(() => import("@/pages/audit/OperationLogsPage"));
const ErrorLogsPage = lazy(() => import("@/pages/audit/ErrorLogsPage"));
const NotificationInboxPage = lazy(() => import("@/pages/notifications/NotificationInboxPage"));
const NotificationListPage = lazy(() => import("@/pages/notifications/NotificationListPage"));
const VerificationsPage = lazy(() => import("@/pages/verifications/VerificationsPage"));

/** Infrastructure & security routes — resources, certs, models, adapters, enforcers, syncers, webhooks, ldaps, records */
export const infraRoutes: RouteObject[] = [
  // ── Resources ──
  { path: "resources", element: withSuspense(<ResourcesPage />), loader: resourcesListLoader },
  { path: "resources/new", element: withSuspense(<ResourceCreatePage />) },
  {
    path: "resources/:id",
    element: withSuspense(<ResourceDetailPage />),
    loader: resourceDetailLoader,
  },

  // ── Certs ──
  { path: "certs", element: withSuspense(<CertsPage />), loader: certsListLoader },
  { path: "certs/new", element: withSuspense(<CertCreatePage />) },
  { path: "certs/:id", element: withSuspense(<CertDetailPage />), loader: certDetailLoader },

  // ── Models ──
  { path: "models", element: withSuspense(<ModelsPage />) },
  { path: "models/new", element: withSuspense(<ModelCreatePage />) },
  { path: "models/:id", element: withSuspense(<ModelDetailPage />) },

  // ── Adapters ──
  { path: "adapters", element: withSuspense(<AdaptersPage />) },
  { path: "adapters/new", element: withSuspense(<AdapterCreatePage />) },
  { path: "adapters/:id", element: withSuspense(<AdapterDetailPage />) },

  // ── Enforcers ──
  { path: "enforcers", element: withSuspense(<EnforcersPage />) },
  { path: "enforcers/new", element: withSuspense(<EnforcerCreatePage />) },
  { path: "enforcers/:id", element: withSuspense(<EnforcerDetailPage />) },

  // ── Syncers ──
  { path: "syncers", element: withSuspense(<SyncersPage />), loader: syncersListLoader },
  { path: "syncers/new", element: withSuspense(<SyncerCreatePage />) },
  { path: "syncers/:id", element: withSuspense(<SyncerDetailPage />), loader: syncerDetailLoader },

  // ── Migration ──
  { path: "migration", element: withSuspense(<MigrationPage />) },

  // ── Webhooks ──
  { path: "webhooks", element: withSuspense(<WebhooksPage />), loader: webhooksListLoader },
  { path: "webhooks/new", element: withSuspense(<WebhookCreatePage />) },
  {
    path: "webhooks/:id",
    element: withSuspense(<WebhookDetailPage />),
    loader: webhookDetailLoader,
  },

  // ── LDAP ──
  { path: "ldaps", element: withSuspense(<LdapsPage />), loader: ldapsListLoader },
  { path: "ldaps/new", element: withSuspense(<LdapCreatePage />) },
  { path: "ldaps/:id", element: withSuspense(<LdapDetailPage />), loader: ldapDetailLoader },
  { path: "ldaps/:id/sync", element: withSuspense(<LdapSyncPage />) },

  // ── Audit center ──
  { path: "audit/login-logs", element: withSuspense(<LoginLogsPage />) },
  { path: "audit/operation-logs", element: withSuspense(<OperationLogsPage />) },
  { path: "audit/error-logs", element: withSuspense(<ErrorLogsPage />) },

  // ── Records & Notifications ──
  { path: "records", element: withSuspense(<RecordsPage />) },
  { path: "notifications", element: withSuspense(<NotificationInboxPage />) },
  { path: "notification-deliveries", element: withSuspense(<NotificationListPage />) },
  { path: "verifications", element: withSuspense(<VerificationsPage />) },
];
