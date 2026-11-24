import { App } from "antd";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

/**
 * Shared test providers.
 *
 * Before this helper existed, 18 test files each defined their own
 * `createWrapper()`, which drifted in two ways that caused real defects:
 *  - some wrapped `<App>` (required when the component calls `App.useApp()` for
 *    `message`/`notification`/`modal`) and some did not;
 *  - some disabled mutation retries and some only disabled query retries.
 *
 * `App` is rendered with `component={false}` so it contributes no DOM node —
 * it only installs the antd context holders, keeping queries like `getByText`
 * unaffected.
 */

export function createTestQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
}

export type TestWrapperOptions = {
  /** Reuse an existing client (e.g. to assert cache state after a render). */
  queryClient?: QueryClient;
  /**
   * Wrap in antd's `<App>`. Required when the component under test (or a hook it
   * calls) uses `App.useApp()`. Defaults to true because adding it is harmless.
   */
  withApp?: boolean;
};

/** Returns a `wrapper` function suitable for `render(ui, { wrapper })`. */
export function createTestWrapper(options: TestWrapperOptions = {}) {
  const { queryClient = createTestQueryClient(), withApp = true } = options;

  return function Wrapper({ children }: { children: ReactNode }) {
    const tree = <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
    return withApp ? <App component={false}>{tree}</App> : tree;
  };
}
