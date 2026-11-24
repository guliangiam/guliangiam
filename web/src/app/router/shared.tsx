import { Suspense, Component } from "react";
import type { ReactNode, ErrorInfo } from "react";
import { useRouteError, isRouteErrorResponse } from "react-router-dom";
import Button from "antd/es/button";
import Result from "antd/es/result";
import Skeleton from "antd/es/skeleton";
import { t } from "i18next";
import { isHttpError } from "@/shared/api/http";

// ── Error Boundary for lazy-loaded routes ──
type ErrorBoundaryProps = { children: ReactNode };
type ErrorBoundaryState = { hasError: boolean; error?: Error };

export class RouteErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { hasError: false };

  static getDerivedStateFromError(error: Error) {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("Route load error:", error, info);
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="gl-empty-state">
          <Result
            status="error"
            title={t("common:pageLoadFailed")}
            subTitle={this.state.error?.message || t("common:unexpectedError")}
            extra={
              <Button type="primary" onClick={() => window.location.reload()}>
                {t("common:reload")}
              </Button>
            }
          />
        </div>
      );
    }
    return this.props.children;
  }
}

export function RouterErrorElement() {
  const error = useRouteError();

  const is404 =
    (isRouteErrorResponse(error) && error.status === 404) ||
    (isHttpError(error) && error.status === 404);

  const title = is404 ? t("common:notFound") : t("common:unexpectedError");
  const subTitle = is404
    ? t("common:notFoundDesc")
    : isRouteErrorResponse(error)
      ? `${error.status} ${error.statusText}`
      : error instanceof Error
        ? error.message
        : undefined;

  return (
    <div className="gl-empty-state">
      <Result
        status={is404 ? "404" : "error"}
        title={title}
        subTitle={subTitle}
        extra={
          <Button type="primary" onClick={() => (window.location.href = "/")}>
            {t("common:backToHome")}
          </Button>
        }
      />
    </div>
  );
}

// oxlint-disable-next-line react/only-export-components -- Suspense 包装器与错误边界同属路由外壳，拆分需改动 src/app/router 的导入方
export function withSuspense(node: ReactNode) {
  return (
    <RouteErrorBoundary>
      <Suspense
        fallback={
          <div className="gl-empty-state">
            <Skeleton active paragraph={{ rows: 6 }} title />
          </div>
        }
      >
        {node}
      </Suspense>
    </RouteErrorBoundary>
  );
}

import type { LoaderFunctionArgs } from "react-router-dom";

// Route loaders are only needed after navigating to an authenticated route.
// Importing loader modules statically would drag their API clients (and zod
// schemas) into the entry chunk even on /login; this wrapper loads the module
// on first navigation instead (OPEN-PERF-01).
// oxlint-disable-next-line react/only-export-components -- 懒加载 loader 工厂与路由外壳同模块，拆分需改动 src/app/router 的导入方
export function lazyRouteLoader(
  moduleLoader: () => Promise<Record<string, unknown>>,
  exportName: string,
) {
  return async (args: LoaderFunctionArgs) => {
    const module = await moduleLoader();
    const loader = module[exportName];
    if (typeof loader !== "function") {
      throw new Error(`loader ${exportName} is not a function`);
    }
    return (loader as (args: LoaderFunctionArgs) => unknown)(args);
  };
}
