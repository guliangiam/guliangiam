#!/usr/bin/env python3
"""Check what the browser is forced to download before the app can render.

Why this exists
---------------
`vite.config.ts` groups vendor libraries into named chunks so that a deploy only
invalidates the chunk that actually changed. That mechanism has one failure mode
that no type check, lint run or unit test can see: **a `manualChunks` rule whose
group is only reachable through a lazy route can drag the whole group back into
the entry's static module graph.** The route still lazy-loads, the page still
works, and the only symptom is that every visitor downloads a library they will
almost never open.

That is not hypothetical. The Scalar API-reference group (`@scalar/*`, reached
only by the lazy `/swagger` route) was in the first screen:

    index.html  ->  modulepreload /assets/scalar-vendor-*.js   (1025 KB gzip)
    first-screen total 1719 KB gzip -> 694 KB gzip after removing the rule

A size budget alone would have caught that as "we went over"; it would not have
said which chunk is illegitimate, and it cannot be tightened far enough to be
useful while a single legitimate chunk (antd) is 474 KB of the total. So this
gate asserts two different things:

  1. **No undeclared vendor on the first screen.** Every `*-vendor-*.js`
     referenced by `web/dist/index.html` must appear in `FIRST_SCREEN_VENDORS`
     with a stated reason. A hoisted lazy group fails here with a message that
     names the chunk, which is the actionable half of the failure above.

  2. **A ratchet on the byte total.** `FIRST_SCREEN_BUDGET_KB` is deliberately
     set to the measured value, not to an aspiration. Growing the first screen
     is allowed but must be an explicit, reviewed edit of this constant, which is
     the point: silent growth is the thing being prevented.

The numbers come from `web/dist`, so the frontend must be built first.

Usage: cd web && pnpm run build && python3 scripts/check-bundle-budget.py
Exit code 0 = first screen within budget, 1 = an undeclared vendor or over budget.

输入：web/dist/ 构建产物、web/vite.config.ts manualChunks 声明
输出：PASS/FAIL + 越界 chunk 及体积/理由缺失
--check 行为：无 --check；失败即 exit 1
消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
"""

import gzip
import os
import re
import sys

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DIST = os.path.join(REPO_ROOT, "web/dist")
INDEX_HTML = os.path.join(DIST, "index.html")

# Every `*-vendor` chunk the entry is allowed to pull in, with the reason it is on
# the critical path. Adding a line here is the cost of adding a vendor chunk to
# the first screen; removing a rule from `vite.config.ts` is usually the right
# answer instead.
FIRST_SCREEN_VENDORS = {
    "antd-vendor": "AppShell, ResourceListCard and the shared form/table primitives render on every route",
    "data-vendor": "TanStack Query and zustand are constructed by the app root before any route resolves",
    "i18n-vendor": "i18next must be initialised to render any label, including the shell chrome",
    "react-vendor": "React and React Router are the renderer",
    "zod-vendor": "http.ts validates every response envelope before a page sees it",
    "emotion-vendor": "antd v6 CSS-in-JS runtime (0.5 KB) is required by the first antd component rendered",
}

# Ratchet: the measured first-screen gzip total, rounded up to the next 10 KB.
# Raising it is a deliberate act; the gate exists so that raising it is noticed.
#
# 700 -> 710 (2026-09-30): measured 700.6 KB. The previous 700 came from a 692.5 KB
# measurement. The +8.1 KB is app code, not vendor: `antd-vendor` moved 474.5 -> 474.2 KB,
# while the entry chunk grew with the CrudListPage/ResourceListCard unification and the
# 14-locale i18n sync. Nothing was hoisted onto the first screen from a lazy route (all
# 154 routes are still `lazy()`; non-default locales are still dynamically imported), so
# this is the "the app legitimately got bigger" case, which the ratchet is meant to make
# visible rather than block. The 300 KB target below is unchanged and still unmet.
#
# 710 -> 730 (2026-10-19): measured 719.2 KB after V4.14-V4.23. Growth is app code, not
# hoisting: AI gateway console menu/routes, Web3 passkey panel strings (36 keys x en/zh-CN
# static; other 12 locales stay lazy), 125-table generated contract types, and the admin
# workbench surface all legitimately ship on the first screen. `antd-vendor` alone is
# 489 KB gzip (~68% of the total) and is the framework floor for an Ant Design console;
# splitting it needs a component-loading architecture change, tracked as OPEN-PERF-01 in
# `contracts/registries/open-items.yaml`. The 300 KB target below remains unchanged and unmet.
#
# 730 -> 720 (2026-10-20, V4.63): 入口静态图的 3 个 antd barrel 导入（MenuSearch/
# GlobalErrorBoundary/SessionTimeoutDialog）改为深导入后实测 717.6 KB；ratchet 收紧到 720。
#
# 720 -> 570 (2026-10-20, V4.65): 入口静态图 barrel 清零后，删除 antd/recharts 的
# manualChunks 分组，rolldown 按组件自然分割；实测首屏 561.0 KB（50 个请求，
# HTTP/2 多路复用）。请求数从 15 升到 50 是字节下降的 trade-off，记录于
# docs/design/frontend/performance.md。
#
# 570 -> 460 (2026-10-20, V4.66): AppShell 改为 lazy（仅认证路由需要），
# 登录首屏不再下载 Layout/Menu/Drawer/Dropdown/Select 等壳层；实测 449.5 KB、30 请求。
#
# 460 -> 430 (2026-10-20, V4.67): 删除 @ant-design/icons 的 manualChunks 分组，
# 首屏只保留初始路由实际使用的图标；实测 421.4 KB，请求数仍 30。
#
# 430 -> 420 (2026-10-20, V4.69): 路由 loader 改为 lazy（首次导航时动态导入），
# 初始化不再静态引入 loader 模块及其 API/zod 依赖；实测 418.1 KB。
#
# 420 -> 380 (2026-10-20, V4.70): 应用/Provider 详情 loader 的 API 客户端改为
# 动态 import，初始化不再静态引入 application/provider API（连带 zod 与表单组件
# 依赖）；实测 375.4 KB、26 请求。
#
# 380 -> 381 (2026-10-29, V4.122): /system-info 契约修复补上 heapSys/stackInuse/
# os·arch 三个统计项与 14 个语言包的两个标签（stackInuse/platform），首屏实测
# 380.0 KB 越过 380 KB 的字节阈值；删除已无消费方的 totalAlloc/gcCycles 标签后
# 仍差不到 1 KB，按 ratchet 规则显式上调 1 KB 并记录原因。
FIRST_SCREEN_BUDGET_KB = 381

# From docs/design/frontend/performance.md (首屏字节预算: enforced 381 KB ratchet,
# target ≤ 300KB cancelled 2026-10-20 per OPEN-PERF-01 review — framework floor
# 191.3 KB (react 93.3 + emotion 67.5 + i18n 17.6 + data 12.4 + runtime 0.5)
# makes 300 KB physically unreachable on React 19 + antd v6 emotion stack).
FIRST_SCREEN_TARGET_KB = 300  # cancelled by review; retained for reference only

ASSET_REF = re.compile(r'(?:src|href)="(/assets/[^"]+)"')
# Vite appends `-<hash>` before the extension; stripping it is what turns
# `antd-vendor-C1GroLw2.js` into the `vite.config.ts` group name `antd-vendor`.
HASH_SUFFIX = re.compile(r"-[A-Za-z0-9_-]{8}$")


def read(path):
    with open(path, encoding="utf-8") as handle:
        return handle.read()


def chunk_name(asset):
    """`/assets/antd-vendor-C1GroLw2.js` -> `antd-vendor`."""
    base = os.path.basename(asset)
    stem = base.rsplit(".", 1)[0]
    return HASH_SUFFIX.sub("", stem)


def first_screen_assets():
    """The assets a browser fetches before any lazy route is entered.

    `index.html` is the whole definition of the first screen: `<script src>` for
    the entry chunk and one `modulepreload` per statically imported chunk. Lazy
    chunks are reachable only through the `__vitePreload` dependency map inside
    the entry chunk, so they never appear here -- which is exactly why their
    absence is worth asserting.
    """
    seen = []
    for asset in ASSET_REF.findall(read(INDEX_HTML)):
        if asset not in seen:
            seen.append(asset)
    return seen


def gzipped_size(path):
    with open(path, "rb") as handle:
        data = handle.read()
    # mtime=0 keeps the result reproducible across checkouts; the gate compares
    # sizes, so the header timestamp would otherwise be pure noise.
    return len(gzip.compress(data, 9, mtime=0))


def main():
    if not os.path.isfile(INDEX_HTML):
        print(f"FAIL: {os.path.relpath(INDEX_HTML, REPO_ROOT)} not found.")
        print("       Build the frontend first: cd web && pnpm run build")
        return 1

    rows = []
    total = 0
    undeclared = {}
    for asset in first_screen_assets():
        path = os.path.join(DIST, asset.lstrip("/"))
        if not os.path.isfile(path):
            print(f"FAIL: {asset} is referenced by index.html but missing from dist.")
            return 1
        name = chunk_name(asset)
        size = gzipped_size(path)
        total += size
        rows.append((size, asset, name))
        if name.endswith("-vendor") and name not in FIRST_SCREEN_VENDORS:
            undeclared.setdefault(name, []).append((asset, size))

    rows.sort(reverse=True)
    for size, asset, _ in rows:
        print(f"{size / 1024:8.1f} KB  {asset}")

    total_kb = total / 1024
    print(f"{'-' * 40}\n{total_kb:8.1f} KB  first screen (gzip, {len(rows)} requests)")

    failed = False

    if undeclared:
        failed = True
        print()
        print("FAIL: vendor chunk on the first screen but not declared in FIRST_SCREEN_VENDORS:")
        for name, assets in sorted(undeclared.items()):
            weight = sum(size for _, size in assets)
            listed = ", ".join(asset for asset, _ in assets)
            print(f"  - {name} ({weight / 1024:.1f} KB: {listed})")
        print()
        print("  Fix by picking one of:")
        print("    a) the vendor is genuinely needed to render the shell -> add it to")
        print("       FIRST_SCREEN_VENDORS with the reason;")
        print("    b) it is only reached through a lazy route -> delete the manualChunks")
        print("       rule that names it and let natural code splitting keep it lazy")
        print("       (this is the shape the Scalar group had).")

    if total_kb > FIRST_SCREEN_BUDGET_KB:
        failed = True
        print()
        print(
            f"FAIL: first screen is {total_kb:.1f} KB gzip, "
            f"budget {FIRST_SCREEN_BUDGET_KB} KB."
        )
        print("  Raise FIRST_SCREEN_BUDGET_KB only together with the reason the extra")
        print("  weight is unavoidable, so the ratchet records the decision.")

    if failed:
        return 1

    print()
    if total_kb > FIRST_SCREEN_TARGET_KB:
        print(
            f"NOTE: above the {FIRST_SCREEN_TARGET_KB} KB target of "
            "docs/design/frontend/performance.md."
        )
    print(
        f"PASS: first screen {total_kb:.1f} KB gzip; "
        f"{len([r for r in rows if r[2].endswith('-vendor')])} declared vendor chunks, "
        "no undeclared vendor, within budget."
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
