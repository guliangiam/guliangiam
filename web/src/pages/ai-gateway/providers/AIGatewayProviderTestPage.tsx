// =============================================================================
// AI Gateway — Provider Test（占位）
// =============================================================================
//
// 完整的连通性测试按钮 + 结果展示已集成到 AIGatewayProviderDetailPage
// 详情页工具栏（POST /api/v1/ai/providers/{id}:test）。
// 本路由做 immediate redirect，避免维护两份连通性测试 UI。
// =============================================================================

import { Navigate, useParams } from "react-router-dom";

export default function AIGatewayProviderTestPage() {
  const { id } = useParams<{ id: string }>();
  return <Navigate to={`/ai/providers/${id ?? ""}`} replace />;
}
