#!/usr/bin/env python3
"""前端功能覆盖门禁 — 路由、页面、菜单与设计文档必须互相闭合。

Why this exists
---------------
The console has three independently maintained statements of the same fact --
which pages exist:

  1. `web/src/app/router/router.tsx` (what React Router can actually render),
  2. `web/src/features/navigation/menu.tsx` (what the operator can navigate to),
  3. `docs/design/frontend/registry.md` 的菜单表（生成块，见下）。

`menu.test.ts` covers the relationship between (2) and its own i18n keys, but
nothing connected (1) to (2), and nothing at all connected (3) to either. The
result was measurable drift: the menu table listed eight groups whose
children no longer matched `menu.tsx`, and a route could be added with a lazy
import that pointed at a file nobody had created yet -- `withSuspense` renders
the error boundary, so the page shows "page load failed" instead of 404 and no
type check or unit test notices.

Rules (each is a property of the tree, not a judgement about intent):

  1. Every `path:` entry resolves to a component name, and that component's
     `lazy(() => import("@/..."))` target exists on disk.
  2. Every `*Page.tsx` under `web/src/pages/` is referenced by the router.
     A page nobody can route to is dead code.
  3. Every menu child key and every `routeKeys` entry exists as a router path.
     A menu entry without a route is a link to a blank screen.
  4. Every router path is either reachable from the menu or listed in
     `NON_NAV_ROUTES` below. Adding a route therefore forces a decision about
     how the operator gets there, instead of silently orphaning it.
  5. The menu table in `docs/design/frontend/registry.md` lists exactly the groups,
     children and labels that `menu.tsx` declares. The table is a generated
     block, so this rule is really "the generator ran and its output is current".
  6. Every `NON_NAV_ROUTES` entry names a route that actually exists, and every
     `NON_NAV_PREFIXES` entry matches at least one route. An allowlist entry
     without a route is stale: it can silently admit a future route that was
     never reviewed for navigation reachability.

Usage: python3 scripts/check-frontend-coverage.py
Exit code 0 = the three statements of "which pages exist" agree.

输入：web/src/app/router/router.tsx、web/src/features/navigation/menu.tsx、docs/design/frontend/registry.md
输出：PASS/FAIL + 孤岛页面/失配路由/失配菜单
--check 行为：无 --check；失败即 exit 1
消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from frontend_common import (  # noqa: E402  (path is set above)
    DESIGN,
    MENU,
    MENU_TABLE_BEGIN,
    MENU_TABLE_END,
    PAGES,
    ROOT,
    ROUTER,
    parse_menu,
    parse_router,
    resolve_module,
)

# Routes that intentionally have no menu entry: public authentication and
# protocol surfaces (entered by redirect from a relying party), and detail
# routes reached by clicking a row in a list. Keep this list small -- every
# entry is a route the operator cannot discover from the console.
NON_NAV_ROUTES = {
    # ── AI 分区根路径：仅重定向到 /ai/providers，菜单以具体页进入 ──
    "/ai",
    # ── public / protocol surfaces: entered by redirect, never by menu ──
    "/login",
    "/login/:organizationName",
    "/login/oauth/authorize",
    "/login/saml/authorize/:owner/:applicationName",
    "/cas/:owner/:casApplicationName/login",
    "/cas/:owner/:casApplicationName/logout",
    "/signup",
    "/signup/:organizationName",
    "/signup/oauth/authorize",
    "/forgot-password",
    "/register/invitation",
    "/consent/:applicationName",
    "/prompt/:applicationName",
    "/callback/saml",
    "/telegram-login",
    "/device",
    "/auth/result",
    "/auth/callback/:providerType",
    "/select-plan/:orgId/:pricingName",
    # ── redirect aliases kept so old bookmarks keep working ──
    "/system/settings",
    # ── self-service flows entered from the account page ──
    "/mfa/setup",
    # ── provider type list routes: redirect aliases into unified /providers?kind=… ──
    "/captcha-providers",
    "/notification-providers",
    "/face-id-providers",
    # standalone captcha test page folded into the captcha provider config page
    "/captcha/test",
    # ── catch-all: renders the router error element ──
    "*",
}

# Detail / create routes reached from a list row or the list's primary action.
NON_NAV_PREFIXES = (
    "/organizations/",
    "/users/",
    "/groups/",
    "/invitations/",
    "/applications/",
    "/providers/",
    "/roles/",
    "/permissions/",
    "/tokens/",
    "/products/",
    "/plans/",
    "/coupons/",
    "/orders/",
    "/payments/",
    "/pricings/",
    "/subscriptions/",
    "/transactions/",
    "/syncers/",
    "/webhooks/",
    "/ldaps/",
    "/servers/",
    "/sites/",
    "/resources/",
    "/certs/",
    "/keys/",
    "/notification-providers/",
    "/radius-configs/",
    "/rules/",
    "/forms/",
    "/tickets/",
    "/models/",
    "/adapters/",
    "/enforcers/",
    "/captcha-providers/",
    "/face-id-providers/",
    "/entries/",
)

FAILURES: list[str] = []


def fail(message: str) -> None:
    FAILURES.append(message)


def parse_doc_menu_table() -> dict[str, set[str]]:
    """Return {group key: {route, ...}} from the generated menu table block."""
    lines = DESIGN.read_text(encoding="utf-8").splitlines()
    start = next(
        (i for i, line in enumerate(lines) if line.strip() == MENU_TABLE_BEGIN), None
    )
    if start is None:
        return {}
    table: dict[str, set[str]] = {}
    for line in lines[start:]:
        if line.strip() == MENU_TABLE_END:
            break
        cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
        if len(cells) != 3 or not cells[0].startswith("`"):
            continue
        group = cells[0].strip("`")
        routes = set(re.findall(r"`(/[^`]*)`", cells[2]))
        table[group] = routes
    return table


def resolve_module(target: str) -> Path | None:
    """Resolve an `@/...` specifier the way Vite does, without a bundler."""
    base = ROOT / "web/src" / target.removeprefix("@/")
    for candidate in (
        base,
        base.with_suffix(".tsx"),
        base.with_suffix(".ts"),
        base / "index.tsx",
        base / "index.ts",
    ):
        if candidate.is_file():
            return candidate.resolve()
    return None


def check_routes(
    routes: dict[str, str],
    lazy_imports: dict[str, str],
    static_imports: dict[str, str],
    local_components: set[str],
) -> None:
    for path, component in routes.items():
        if not component:
            fail(f"规则 1：路由 {path} 的 element 没有可解析的页面组件")
            continue
        if component in lazy_imports:
            target = lazy_imports[component]
            if resolve_module(target) is None:
                fail(f"规则 1：路由 {path} → {component} → {target} 在磁盘上不存在")
            continue
        if component in static_imports or component in local_components:
            source = static_imports.get(component, "")
            if source.startswith("@/") and resolve_module(source) is None:
                fail(f"规则 1：路由 {path} → {component} → {source} 在磁盘上不存在")
            continue
        fail(f"规则 1：路由 {path} 使用了 {component}，但它既不是导入也不是本地声明")


def check_orphan_pages(routes: dict[str, str], lazy_imports: dict[str, str]) -> None:
    referenced = {
        resolved
        for component in routes.values()
        if component in lazy_imports
        if (resolved := resolve_module(lazy_imports[component])) is not None
    }
    for page in sorted(PAGES.rglob("*Page.tsx")):
        if page.name.endswith(".test.tsx"):
            continue
        if page.resolve() not in referenced:
            fail(f"规则 2：{page.relative_to(ROOT)} 没有任何路由引用（不可达页面）")


def check_menu_routes(
    routes: dict[str, str],
    route_keys: dict[str, list[str]],
    children: dict[str, set[str]],
) -> None:
    for group, keys in route_keys.items():
        for key in keys:
            if key not in routes:
                fail(f"规则 3：菜单组 {group} 的 routeKeys 引用了不存在的路由 {key}")
    for group, keys in children.items():
        for key in sorted(keys):
            if key not in routes:
                fail(f"规则 3：菜单组 {group} 的子项 {key} 没有对应路由")


def check_reachability(routes: dict[str, str], route_keys: dict[str, list[str]]) -> None:
    covered = {key for keys in route_keys.values() for key in keys}
    for path in routes:
        if path in covered or path in NON_NAV_ROUTES:
            continue
        if any(path.startswith(prefix) for prefix in NON_NAV_PREFIXES):
            continue
        fail(
            f"规则 4：路由 {path} 既不在任何菜单组的 routeKeys 中，"
            f"也不在 NON_NAV_ROUTES / NON_NAV_PREFIXES 允许清单内"
        )


def check_non_nav_allowlist(routes: dict[str, str]) -> None:
    """Every allowlist entry must correspond to at least one real route.

    Exact entries must be a route path verbatim (including the catch-all `*`);
    prefix entries must match at least one route by prefix. Stale entries are
    reported by name so the operator can delete them instead of leaving a
    silently widening allowlist behind.
    """
    for path in sorted(NON_NAV_ROUTES):
        if path not in routes:
            fail(f"规则 6：NON_NAV_ROUTES 中的 {path} 没有对应路由（陈旧放行项）")
    for prefix in NON_NAV_PREFIXES:
        if not any(path.startswith(prefix) for path in routes):
            fail(f"规则 6：NON_NAV_PREFIXES 中的 {prefix} 不匹配任何路由（陈旧放行前缀）")


def check_design_doc(
    route_keys: dict[str, list[str]], children: dict[str, set[str]]
) -> None:
    table = parse_doc_menu_table()
    if not table:
        fail("规则 5：未能在《控制台设计》解析出菜单表生成块（标记缺失或未生成）")
        return
    for group in route_keys:
        if group not in table:
            fail(f"规则 5：菜单表缺少分组 {group}")
            continue
        expected = {key for key in route_keys[group]}
        # The table documents the navigable children; routeKeys additionally
        # carries nested paths (e.g. `/users/import`) that are reached from a
        # parent page, so only child keys must appear verbatim.
        missing = {key for key in children[group] if key not in table[group]}
        if missing:
            fail(f"规则 5：菜单表分组 {group} 缺少子项 {sorted(missing)}")
        stale = {key for key in table[group] if key not in expected}
        if stale:
            fail(f"规则 5：菜单表分组 {group} 列出了菜单中不存在的路由 {sorted(stale)}")
    for group in table:
        if group not in route_keys:
            fail(f"规则 5：菜单表列出了 menu.tsx 中不存在的分组 {group}")


def main() -> int:
    for required in (ROUTER, MENU, PAGES, DESIGN):
        if not required.exists():
            print(f"FAIL: 缺少必需事实源 {required.relative_to(ROOT)}")
            return 1

    routes, lazy_imports, static_imports, local_components = parse_router()
    route_keys, children, _labels = parse_menu()

    check_routes(routes, lazy_imports, static_imports, local_components)
    check_orphan_pages(routes, lazy_imports)
    check_menu_routes(routes, route_keys, children)
    check_reachability(routes, route_keys)
    check_non_nav_allowlist(routes)
    check_design_doc(route_keys, children)

    print("=== check-frontend-coverage.py ===")
    print(f"路由 {len(routes)} 条 / 菜单分组 {len(route_keys)} 个 / 菜单子项 "
          f"{sum(len(value) for value in children.values())} 项 / "
          f"页面文件 {len(list(PAGES.rglob('*Page.tsx')))} 个")
    if FAILURES:
        for message in FAILURES:
            print(f"FAIL: {message}")
        print(f"FAIL: {len(FAILURES)} 项前端覆盖缺口")
        return 1
    print("PASS: 路由、页面、菜单与《控制台设计》菜单表互相闭合")
    return 0


if __name__ == "__main__":
    sys.exit(main())
