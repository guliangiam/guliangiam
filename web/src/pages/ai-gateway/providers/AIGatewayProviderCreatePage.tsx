// =============================================================================
// AI Gateway — Provider Create
// =============================================================================
//
// Provider 完整创建表单已集成在 AIGatewayProvidersPage.tsx 列表页内（行内 Modal）。
// 本路由做 immediate redirect，避免维护两份表单逻辑。
// =============================================================================

import { Navigate } from "react-router-dom";

export default function AIGatewayProviderCreatePage() {
  return <Navigate to="/ai/providers" replace />;
}
