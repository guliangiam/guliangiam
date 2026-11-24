import { lazy } from "react";
import { Navigate } from "react-router-dom";
import type { RouteObject } from "react-router-dom";
import { withSuspense } from "./shared";

// ── AI Gateway 管理面页面（后端 admin_chi.go 已暴露的 7 个实体 + usage）──
const AIGatewayProvidersPage = lazy(
  () => import("@/pages/ai-gateway/providers/AIGatewayProvidersPage"),
);
const AIGatewayProviderCreatePage = lazy(
  () => import("@/pages/ai-gateway/providers/AIGatewayProviderCreatePage"),
);
const AIGatewayProviderDetailPage = lazy(
  () => import("@/pages/ai-gateway/providers/AIGatewayProviderDetailPage"),
);
const AIGatewayProviderTestPage = lazy(
  () => import("@/pages/ai-gateway/providers/AIGatewayProviderTestPage"),
);
const AIGatewayModelsPage = lazy(() => import("@/pages/ai-gateway/models/AIGatewayModelsPage"));
const AIGatewayAliasesPage = lazy(() => import("@/pages/ai-gateway/aliases/AIGatewayAliasesPage"));
const AIGatewayRoutesPage = lazy(() => import("@/pages/ai-gateway/routes/AIGatewayRoutesPage"));
const AIGatewayAPIKeysPage = lazy(() => import("@/pages/ai-gateway/api-keys/AIGatewayAPIKeysPage"));
const AIGatewayQuotasPage = lazy(() => import("@/pages/ai-gateway/quotas/AIGatewayQuotasPage"));
const AIGatewayPricesPage = lazy(() => import("@/pages/ai-gateway/prices/AIGatewayPricesPage"));
const AIGatewayUsagePage = lazy(() => import("@/pages/ai-gateway/usage/AIGatewayUsagePage"));
const AIGatewayUsageStatsPage = lazy(
  () => import("@/pages/ai-gateway/usage-stats/AIGatewayUsageStatsPage"),
);
const AIGatewayHealthPage = lazy(() => import("@/pages/ai-gateway/health/AIGatewayHealthPage"));

// ── 从 infraRoutes 迁入的 Agents 域 ──
const AgentsPage = lazy(() => import("@/pages/agents/AgentsPage"));
const AgentCreatePage = lazy(() => import("@/pages/agents/AgentCreatePage"));
const AgentDetailPage = lazy(() => import("@/pages/agents/AgentDetailPage"));

// ── 从 securityRoutes 迁入的 MCP Servers 域 ──
const McpServersPage = lazy(() => import("@/pages/mcp-servers/McpServersPage"));
const McpServerCreatePage = lazy(() => import("@/pages/mcp-servers/McpServerCreatePage"));
const McpServerDetailPage = lazy(() => import("@/pages/mcp-servers/McpServerDetailPage"));
const McpStorePage = lazy(() => import("@/pages/mcp-servers/McpStorePage"));

/**
 * AI Gateway 管理面 + Agents + MCP Servers 路由分组。
 *
 * 路由聚合位置：router.tsx → children: [...aiRoutes]
 * 统一菜单树分组名：AI 网关（icon: RobotOutlined）
 *
 * 含三种子域：
 *   1. aigateway 新增域（providers/models/aliases/routes/api-keys/quotas/prices/usage/health）
 *      —— 后端 admin_chi.go 已暴露完整 CRUD（/api/v1/ai/{entity}）
 *   2. agents 域（原 infraRoutes 迁入）
 *   3. mcpservers 域（原 securityRoutes 迁入）
 */
export const aiRoutes: RouteObject[] = [
  {
    path: "/ai",
    children: [
      { index: true, element: <Navigate to="/ai/providers" replace /> },
      // ── aigateway 新增 ──
      { path: "providers", element: withSuspense(<AIGatewayProvidersPage />) },
      { path: "providers/new", element: withSuspense(<AIGatewayProviderCreatePage />) },
      { path: "providers/:id", element: withSuspense(<AIGatewayProviderDetailPage />) },
      { path: "providers/:id/test", element: withSuspense(<AIGatewayProviderTestPage />) },
      { path: "models", element: withSuspense(<AIGatewayModelsPage />) },
      { path: "aliases", element: withSuspense(<AIGatewayAliasesPage />) },
      { path: "routes", element: withSuspense(<AIGatewayRoutesPage />) },
      { path: "api-keys", element: withSuspense(<AIGatewayAPIKeysPage />) },
      { path: "quotas", element: withSuspense(<AIGatewayQuotasPage />) },
      { path: "prices", element: withSuspense(<AIGatewayPricesPage />) },
      { path: "usage", element: withSuspense(<AIGatewayUsagePage />) },
      { path: "usage/stats", element: withSuspense(<AIGatewayUsageStatsPage />) },
      { path: "health", element: withSuspense(<AIGatewayHealthPage />) },
      // ── Agents 域（原 infraRoutes 迁入）──
      { path: "agents", element: withSuspense(<AgentsPage />) },
      { path: "agents/new", element: withSuspense(<AgentCreatePage />) },
      { path: "agents/:id", element: withSuspense(<AgentDetailPage />) },
      // ── MCP Servers 域（原 securityRoutes 迁入）──
      { path: "mcp-servers", element: withSuspense(<McpServersPage />) },
      { path: "mcp-servers/new", element: withSuspense(<McpServerCreatePage />) },
      { path: "mcp-servers/:id", element: withSuspense(<McpServerDetailPage />) },
      { path: "mcp-servers/store", element: withSuspense(<McpStorePage />) },
    ],
  },
];
