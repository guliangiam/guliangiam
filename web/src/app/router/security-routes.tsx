import { lazy } from "react";
import { Navigate, type RouteObject } from "react-router-dom";
import { withSuspense } from "./shared";

const CaptchaProviderDetailPage = lazy(
  () => import("@/pages/captcha-providers/CaptchaProviderDetailPage"),
);
const NotificationProviderCreatePage = lazy(
  () => import("@/pages/notification-providers/NotificationProviderCreatePage"),
);
const NotificationProviderDetailPage = lazy(
  () => import("@/pages/notification-providers/NotificationProviderDetailPage"),
);
const RulesPage = lazy(() => import("@/pages/rules/RulesPage"));
const RuleCreatePage = lazy(() => import("@/pages/rules/RuleCreatePage"));
const RuleDetailPage = lazy(() => import("@/pages/rules/RuleDetailPage"));
const FaceIdProviderCreatePage = lazy(
  () => import("@/pages/face-id-providers/FaceIdProviderCreatePage"),
);
const FaceIdProviderDetailPage = lazy(
  () => import("@/pages/face-id-providers/FaceIdProviderDetailPage"),
);
const RadiusConfigsPage = lazy(() => import("@/pages/radius-configs/RadiusConfigsPage"));
const RadiusConfigCreatePage = lazy(() => import("@/pages/radius-configs/RadiusConfigCreatePage"));
const RadiusConfigDetailPage = lazy(() => import("@/pages/radius-configs/RadiusConfigDetailPage"));
const RadiusAccountingsPage = lazy(
  () => import("@/pages/radius-accountings/RadiusAccountingsPage"),
);
const EntriesPage = lazy(() => import("@/pages/entries/EntriesPage"));
const EntryCreatePage = lazy(() => import("@/pages/entries/EntryCreatePage"));
const EntryDetailPage = lazy(() => import("@/pages/entries/EntryDetailPage"));
const EntryTranscriptPage = lazy(() => import("@/pages/entries/EntryTranscriptPage"));
const RiskPage = lazy(() => import("@/pages/risk/RiskPage"));
const IgaPage = lazy(() => import("@/pages/iga/IgaPage"));
const IDVPage = lazy(() => import("@/pages/idv/IDVPage"));
const WorkloadPage = lazy(() => import("@/pages/workload/WorkloadPage"));

/** Security extension routes — captcha, notification providers, rules, face-id, radius, entries */
export const securityRoutes: RouteObject[] = [
  // ── Captcha Providers ──（列表入口收敛到统一 Provider Center 类别筛选，深链保留重定向）
  { path: "captcha-providers", element: <Navigate to="/providers?category=Captcha" replace /> },
  // 旧专用创建页已并入统一表单（分流提交到专用端点），深链保留重定向。
  {
    path: "captcha-providers/new",
    element: <Navigate to="/providers/new?category=Captcha" replace />,
  },
  { path: "captcha-providers/:id", element: withSuspense(<CaptchaProviderDetailPage />) },
  // 独立验证码测试页已融合进验证码提供商配置页，深链保留重定向。
  { path: "captcha/test", element: <Navigate to="/providers?category=Captcha" replace /> },

  // ── Notification Providers ──
  {
    path: "notification-providers",
    element: <Navigate to="/providers?category=Notification" replace />,
  },
  { path: "notification-providers/new", element: withSuspense(<NotificationProviderCreatePage />) },
  { path: "notification-providers/:id", element: withSuspense(<NotificationProviderDetailPage />) },

  // ── Rules ──
  { path: "rules", element: withSuspense(<RulesPage />) },
  { path: "rules/new", element: withSuspense(<RuleCreatePage />) },
  { path: "rules/:id", element: withSuspense(<RuleDetailPage />) },

  // ── Face ID Providers ──
  { path: "face-id-providers", element: <Navigate to="/providers?category=FaceId" replace /> },
  { path: "face-id-providers/new", element: withSuspense(<FaceIdProviderCreatePage />) },
  { path: "face-id-providers/:id", element: withSuspense(<FaceIdProviderDetailPage />) },

  // ── Radius ──
  { path: "radius-configs", element: withSuspense(<RadiusConfigsPage />) },
  { path: "radius-configs/new", element: withSuspense(<RadiusConfigCreatePage />) },
  { path: "radius-configs/:id", element: withSuspense(<RadiusConfigDetailPage />) },
  { path: "radius-accountings", element: withSuspense(<RadiusAccountingsPage />) },

  // ── Entries ──
  { path: "entries", element: withSuspense(<EntriesPage />) },
  { path: "entries/new", element: withSuspense(<EntryCreatePage />) },
  { path: "entries/:id", element: withSuspense(<EntryDetailPage />) },
  { path: "entries/:id/transcript", element: withSuspense(<EntryTranscriptPage />) },

  // ── Adaptive Risk ──
  { path: "risk", element: withSuspense(<RiskPage />) },

  // ── IGA (Identity Governance & Administration) ──
  { path: "iga", element: withSuspense(<IgaPage />) },

  // ── IDV (Identity Verification) ──
  { path: "idv", element: withSuspense(<IDVPage />) },

  // ── Workload Identity Federation ──
  { path: "identity-pools", element: withSuspense(<WorkloadPage />) },
];
