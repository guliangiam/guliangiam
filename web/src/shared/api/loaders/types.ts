/**
 * LoaderContext — React Router loader 共享上下文。
 *
 * loader 在路由匹配阶段执行，此时 Zustand store 可能还没初始化（AppShell
 * 还没 mount），所以不能直接读 useSessionStore。改为从 cookie / localStorage
 * 读取最小必要信息，保证 SSR 和首次渲染都能拿到 orgId。
 */

export type LoaderContext = {
  /** 当前组织 ID（从 cookie 读，未登录则 null） */
  orgId: string | null;
  /** 当前用户 ID（从 cookie 读，未登录则 null） */
  userId: string | null;
};

/**
 * 从 Cookie 头解析 orgId。
 *
 * 后端在登录后设置 `gl_org_id` cookie（HttpOnly + SameSite=Lax），
 * loader 作为 fetch 调用的一部分会自动带上，读取即可。
 */
export function getOrgIdFromRequest(request: Request): string | null {
  const cookieHeader = request.headers.get("cookie");
  if (!cookieHeader) return null;
  const match = cookieHeader.match(/(?:^|;\s*)gl_org_id=([^;]*)/);
  return match ? decodeURIComponent(match[1]) : null;
}

/** 从 Cookie 头解析 userId */
export function getUserIdFromRequest(request: Request): string | null {
  const cookieHeader = request.headers.get("cookie");
  if (!cookieHeader) return null;
  const match = cookieHeader.match(/(?:^|;\s*)gl_user_id=([^;]*)/);
  return match ? decodeURIComponent(match[1]) : null;
}

/** 为当前 request 构建 LoaderContext */
export function buildLoaderContext(request: Request): LoaderContext {
  return {
    orgId: getOrgIdFromRequest(request),
    userId: getUserIdFromRequest(request),
  };
}
