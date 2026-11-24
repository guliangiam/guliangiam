import { lazy } from "react";

// AppShell is only needed after an authenticated route is entered; keeping it
// static pulled the whole shell (Layout/Menu/Drawer/Dropdown/Select…) into the
// first screen even when the initial route is /login (OPEN-PERF-01).
export const LazyAppShell = lazy(() =>
  import("@/app/layout/AppShell").then((module) => ({ default: module.AppShell })),
);
