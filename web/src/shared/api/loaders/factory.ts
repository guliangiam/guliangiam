/**
 * Loader 工厂 — React Router 7 Data Router 基础设施。
 *
 * 设计目标：让每个域路由的 loader 只有 3-5 行。业务逻辑复用现有
 * API client（`src/shared/api/iam/*.ts`），参数从 request.params +
 * LoaderContext 提取，错误处理统一在这里完成。
 *
 * 为什么需要工厂而不是每个 loader 自己写：
 *   1. 每个域路由都要 handle 401→redirect 到登录页、404→errorElement，
 *      手写一遍等于每个 loader 都要记得同样的 catch 结构。
 *   2. 列表页和详情页有稳定的参数提取模式：
 *      - 列表页：request.url.searchParams 取 keyword/page/pageSize/sortBy
 *      - 详情页：params.id
 *      工厂把这些固定逻辑封装，业务只需要提供 fetch 函数。
 *
 * 与 TanStack Query 的关系：loader 做首屏预加载 + SSR 数据注入，
 * TanStack Query 做客户端 CRUD mutation + 二次 fetch。两者互补。
 */

import { redirect, type LoaderFunctionArgs } from "react-router-dom";
import { isHttpError } from "@/shared/api/http";
import { buildLoaderContext } from "./types";
import type { LoaderContext } from "./types";

/** Loader 工厂的 fetch 函数签名 */
export type LoaderFetch<T> = (ctx: LoaderContext) => Promise<T>;

/** 列表 loader 额外参数（从 URL search params 提取） */
export type ListLoaderParams = {
  keyword?: string;
  page?: number;
  pageSize?: number;
  sortBy?: string;
  sortOrder?: "asc" | "desc";
};

/** 列表 loader 的 fetch 函数签名 */
export type ListLoaderFetch<T> = (ctx: LoaderContext, params: ListLoaderParams) => Promise<T>;

/**
 * wrapLoader — 统一错误处理包装器。
 *
 * 行为：
 *   - HttpError 401 → redirect("/login?from=当前路径")，让 AppShell 处理
 *   - HttpError 404 → 重新 throw，React Router 的 errorElement 接住
 *   - HttpError 403/其他 → 重新 throw，errorElement 接住
 *   - 其他 Error → 重新 throw
 *
 * SSR 友好：所有分支都返回 Promise，blocking loader 可以 await。
 */
export function wrapLoader<T>(
  requestFn: (args: LoaderFunctionArgs, ctx: LoaderContext) => Promise<T>,
): (args: LoaderFunctionArgs) => Promise<T> {
  return async (args: LoaderFunctionArgs): Promise<T> => {
    const ctx = buildLoaderContext(args.request);
    try {
      return await requestFn(args, ctx);
    } catch (err) {
      if (isHttpError(err) && err.status === 401) {
        // 401: 重定向到登录页，保留 from 参数让登录后回到当前页
        const fromPath = new URL(args.request.url).pathname;
        throw redirect(`/login?from=${encodeURIComponent(fromPath)}`);
      }
      // 其他错误让 React Router 抛到 errorElement
      throw err;
    }
  };
}

/**
 * createDetailLoader — 详情页 loader 工厂。
 *
 * 从 params[0] 取 id，从 cookies 取 orgId，调用 fetch 函数。
 *
 * @example
 *   export const loader = createDetailLoader({
 *     fetch: (ctx, id) => getApplication(id),
 *   });
 *   // 路由: { path: "/applications/:id", loader }
 */
export function createDetailLoader<T>(config: {
  fetch: (ctx: LoaderContext, id: string) => Promise<T>;
  idParam?: string;
}) {
  const idParam = config.idParam ?? "id";
  return wrapLoader<T>(async (args, ctx) => {
    const id = args.params[idParam];
    if (!id) {
      throw new Error(`Missing route param: ${idParam}`);
    }
    return config.fetch(ctx, id);
  });
}

/**
 * createListLoader — 列表页 loader 工厂。
 *
 * 从 URL search params 提取 keyword / page / pageSize / sortBy / sortOrder，
 * 从 cookies 取 orgId，调用 fetch 函数。
 *
 * 注意：列表 loader 默认只拉第一页。客户端后续翻页由 TanStack Query
 * 负责。这样 loader 只做首屏数据注入，避免阻塞 SSR。
 *
 * @example
 *   export const loader = createListLoader({
 *     fetch: (ctx, params) => listApplications({
 *       organizationId: ctx.orgId,
 *       keyword: params.keyword,
 *       page: params.page ?? 1,
 *       pageSize: params.pageSize ?? 10,
 *     }),
 *   });
 */
export function createListLoader<T>(config: {
  fetch: ListLoaderFetch<T>;
  defaultPageSize?: number;
  /** When true (default), network errors return empty data instead of throwing —
   *  keeps navigation usable even when the backend is unreachable. */
  failOpen?: boolean;
}) {
  const failOpen = config.failOpen ?? true;
  return wrapLoader<T>(async (args, ctx) => {
    const url = new URL(args.request.url);
    const params: ListLoaderParams = {
      keyword: url.searchParams.get("keyword") ?? undefined,
      page: parseInt(url.searchParams.get("page")),
      pageSize: parseInt(url.searchParams.get("pageSize")) ?? config.defaultPageSize ?? 10,
      sortBy: url.searchParams.get("sortBy") ?? undefined,
      sortOrder: (url.searchParams.get("sortOrder") as "asc" | "desc" | null) ?? undefined,
    };
    try {
      return await config.fetch(ctx, params);
    } catch (err) {
      if (failOpen && !(isHttpError(err) && err.status === 401)) {
        // Log and return empty — let the page render its Empty state instead of
        // blocking navigation with React Router's errorElement.
        console.warn("[loader] list fetch failed, returning empty data:", err);
        const empty = { items: [], total: 0 } as unknown as T;
        return empty;
      }
      throw err;
    }
  });
}

/** URL param → number 解析工具（undefined 而非 NaN 表示"没传"） */
function parseInt(raw: string | null): number | undefined {
  if (raw === null || raw === "") return undefined;
  const n = Number.parseInt(raw, 10);
  return Number.isFinite(n) ? n : undefined;
}

/**
 * 使用 loader data 的 Hook（类型安全封装）。
 *
 * 绕过 React Router 直接导出 useLoaderData，而是提供带泛型的别名，
 * 让 Page 组件写 `const { items } = useListLoaderData<Application>()` 时
 * 获得正确的推断，而不是 `unknown`。
 */
export {
  useLoaderData as useListLoaderData,
  useLoaderData as useDetailLoaderData,
} from "react-router-dom";
