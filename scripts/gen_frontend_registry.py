#!/usr/bin/env python3
"""生成《控制台设计》的菜单表与页面功能注册表（唯一事实源 → 文档生成物）。

Why this exists
---------------
「控制台有哪些页面、每个页面属于哪种交互模式、从哪个菜单进入」这三件事必须只有
一份事实。此前它们分别散落在 `router.tsx`、`menu.tsx` 和文档正文里，三者各自
演化，于是文档列着 8 个分组而子项早已对不上。这里把三者合成两张**生成表**：

    菜单表       —— 分组 key / 分组标签 / 子项路由
    页面注册表   —— 路由 / 页面文件 / 组件 / 交互模式 / 菜单归属 / 后端接口

文档正文不再手写这两张表；`--check` 模式在 CI 中要求「文档里的表 == 重新生成的表」，
与 `gen_schema.py --check`、`scripts/gen_openapi.py` 同一套路：产物必须等于生成结果，
手工编辑产物会被门禁拒绝。

生成块由 `<!-- ... :BEGIN/END -->` 标记界定（标记契约定义在 `frontend_common.py`），
因此文档章节重编号不会让解析失准。

Usage:
  python3 scripts/gen_frontend_registry.py           # 写入文档
  python3 scripts/gen_frontend_registry.py --check   # 只校验，不写

输入：web/src/app/router/router.tsx、web/src/features/navigation/menu.tsx、web/src/pages/*Page.tsx
输出：docs/design/frontend/registry.md 中的两张生成表（<!-- :BEGIN/END --> 标记）；--check 模式零差异门禁
--check 行为：--check 模式比较文档当前表与重新生成结果的差异，无差异时 PASS
消费方：check-generated-zero-diff.sh、CI
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from frontend_common import (  # noqa: E402  (path is set above)
    DESIGN,
    MENU_TABLE_BEGIN,
    MENU_TABLE_END,
    PAGES,
    REGISTRY_BEGIN,
    REGISTRY_END,
    ROOT,
    parse_menu,
    parse_router,
    resolve_module,
)

BEGIN = REGISTRY_BEGIN
END = REGISTRY_END

# 交互模式由组件名与所属目录决定，不做人工标注——人工标注会再次漂移。
# 模式数量刻意收敛：每个模式在《控制台设计》「交互契约」章节有一份契约，模式过多等于没有契约。
MODE_BY_SUFFIX = (
    ("CreatePage", "新建向导"),
    ("DetailPage", "详情与就地编辑"),
    ("EditPage", "详情与就地编辑"),
    ("ListPage", "列表工作台"),
    ("SettingsPage", "设置表单"),
    ("SyncPage", "作业台"),
    ("TestPage", "作业台"),
    ("VerifyPage", "作业台"),
    ("StorePage", "门户与商店"),
    ("TreePage", "列表工作台"),
)

MODE_BY_COMPONENT = {
    "DashboardPage": "仪表盘",
    "AccountPage": "个人中心",
    "AppsPage": "门户与商店",
    "ThemeEditorPage": "可视化编辑器",
    "ShortcutsPage": "导航索引",
    "MigrationPage": "作业台",
    "RouterErrorElement": "错误面",
    "ErrorPage": "错误面",
    "NotFoundPage": "错误面",
    "OidcDiscoveryPage": "协议面",
}

# 协议面：由依赖方重定向进入，不渲染控制台外壳（见《控制台设计》交互契约）。
PROTOCOL_PREFIXES = (
    "/login",
    "/cas/",
    "/signup",
    "/forgot-password",
    "/register/invitation",
    "/consent/",
    "/prompt/",
    "/callback/saml",
    "/telegram-login",
    "/device",
    "/auth/",
)

# 非导航路由的存在理由：既不在菜单内，也不属于协议面/明细面/自服务面。
NON_NAV_LABELS = {
    "self-service": "非导航（由账户页进入）",
    "detail": "非导航（由列表行或主操作进入）",
    "fallback": "非导航（兜底错误面）",
}

SELF_SERVICE_PREFIXES = ("/mfa/",)


def interaction_mode(path: str, component: str) -> str:
    if component in MODE_BY_COMPONENT:
        return MODE_BY_COMPONENT[component]
    if path == "*" or path.startswith("/error") or path.startswith("/404"):
        return "错误面"
    if path.startswith(PROTOCOL_PREFIXES) or path == "/select-plan/:orgId/:pricingName":
        return "协议面"
    for suffix, mode in MODE_BY_SUFFIX:
        if component.endswith(suffix):
            return mode
    return "列表工作台"


def menu_owner(path: str, route_keys: dict[str, list[str]], children: dict[str, set[str]]) -> str:
    for group, keys in route_keys.items():
        if path in children[group]:
            return f"`{group}` / `{path}`"
        if path in keys:
            return f"`{group}` / 嵌套"
    if path == "*":
        return NON_NAV_LABELS["fallback"]
    if path.startswith(SELF_SERVICE_PREFIXES):
        return NON_NAV_LABELS["self-service"]
    return NON_NAV_LABELS["detail"]


def page_file(routes: dict[str, str], lazy_imports: dict[str, str], path: str) -> str:
    component = routes[path]
    target = lazy_imports.get(component)
    if not target:
        return "（路由文件内本地声明）"
    resolved = resolve_module(target)
    if resolved is None:
        return f"`{target}`（缺失）"
    return str(resolved.relative_to(ROOT))


# ── 页面 → 后端接口映射 ──
# 控制台发起请求有三种写法：`iam*` 语义包装器（方法由包装器名决定）、`request*`
# 底层包装器（方法取自 options 的 `method`，缺省为 GET），以及
# `request*(withOrgQuery(<path 表达式>, organizationId), …)`——详情级请求为显式
# 携带目标组织而套的一层路径装饰。三者都能静态提取，因此「前端功能需求 → 后端契约」
# 这条链有机器证据，而不靠人工维护表格。
API_DIR = ROOT / "web/src/shared/api"
BACKTICK = chr(96)  # template/string delimiter, kept as chr() so this file stays quote-agnostic
DECLARATION = re.compile(r"export\s+(?:async\s+)?(?:const|function)\s+(\w+)")
REQUEST_CALL = re.compile(
    # 允许一层嵌套泛型：requestDocumentJson<Paginated<ProviderResponse>>(…)
    r"\b(iam\w*|request\w*)\s*(?:<(?:[^<>]|<[^<>]*>)*>)?\s*\(\s*[`\"]([^`\"]+)[`\"]",
    re.DOTALL,
)
ORG_QUERY_CALL = re.compile(
    # 路径被 withOrgQuery(base, organizationId) 包裹：base 是路径表达式而非字面量，
    # 因此 REQUEST_CALL 匹配不到。约定只把 withOrgQuery 作为请求的首个实参。
    r"\b(iam\w*|request\w*)\s*(?:<(?:[^<>]|<[^<>]*>)*>)?\s*\(\s*withOrgQuery\s*\(",
    re.DOTALL,
)
METHOD_IN_OPTIONS = re.compile(r"method\s*:\s*[\"'](\w+)[\"']")
METHOD_BY_WRAPPER = (
    ("Delete", "DELETE"),
    ("Patch", "PATCH"),
    ("Update", "PUT"),
    ("Put", "PUT"),
    ("Create", "POST"),
    ("Post", "POST"),
)


def infer_method(wrapper: str, chunk: str) -> str:
    for fragment, method in METHOD_BY_WRAPPER:
        if fragment in wrapper:
            return method
    explicit = METHOD_IN_OPTIONS.search(chunk)
    if explicit:
        return explicit.group(1).upper()
    return "GET"


def first_argument(text: str, start: int) -> str | None:
    """Return the raw source of the first argument of a call starting at start.

    Quotes and nested brackets are respected, so USER_SESSION_BY_ID(userId, sessionId)
    is returned whole instead of being cut at its inner comma.
    """
    depth = 0
    quote: str | None = None
    index = start
    while index < len(text):
        char = text[index]
        if quote is not None:
            if char == chr(92):
                index += 2
                continue
            if char == quote:
                quote = None
        elif char in ("'", '"', BACKTICK):
            quote = char
        elif char in ("(", "[", "{"):
            depth += 1
        elif char in (")", "]", "}"):
            if depth == 0:
                return text[start:index].strip()
            depth -= 1
        elif char == "," and depth == 0:
            return text[start:index].strip()
        index += 1
    return None


def render_path_expression(raw: str) -> str:
    """Render an argument expression in the notation the registry table already uses.

    Literal and template literals keep their content; identifier/call expressions are
    wrapped as the path-template placeholder form so the cell still reads as a path
    rather than a function call.
    """
    raw = raw.strip()
    if len(raw) >= 2 and raw[0] == BACKTICK and raw[-1] == BACKTICK:
        return raw[1:-1]
    if len(raw) >= 2 and raw[0] in ("'", '"') and raw[-1] == raw[0]:
        return raw[1:-1]
    return "$" + "{" + raw + "}"


def api_endpoint_index() -> dict[str, str]:
    """Return {exported function name: "METHOD /path"} for the console API client."""
    index: dict[str, str] = {}
    for source_file in sorted(API_DIR.rglob("*.ts")):
        if source_file.name.endswith(".test.ts"):
            continue
        text = source_file.read_text(encoding="utf-8")
        declarations = list(DECLARATION.finditer(text))
        for position, declaration in enumerate(declarations):
            stop = (
                declarations[position + 1].start()
                if position + 1 < len(declarations)
                else len(text)
            )
            chunk = text[declaration.start() : stop]
            call = REQUEST_CALL.search(chunk)
            if call:
                method = infer_method(call.group(1), chunk)
                index.setdefault(declaration.group(1), f"{method} {call.group(2)}")
                continue
            # Detail-level requests wrap the path in withOrgQuery(base, organizationId):
            # the base is an expression, not a literal, so REQUEST_CALL cannot resolve
            # it and the page would silently lose that backend endpoint from the table.
            wrapped = ORG_QUERY_CALL.search(chunk)
            if not wrapped:
                continue
            base = first_argument(chunk, wrapped.end())
            if not base:
                continue
            method = infer_method(wrapped.group(1), chunk)
            index.setdefault(
                declaration.group(1),
                f"{method} {render_path_expression(base)}",
            )
    return index


def resolve_relative(source_file: Path, target: str) -> Path | None:
    """Resolve a `./x` / `../x` specifier the way Vite does."""
    base = (source_file.parent / target).resolve()
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


def collect_imports(source_file: Path) -> tuple[list[str], list[Path]]:
    """Return (api function names, local modules to follow) for one source file."""
    source = source_file.read_text(encoding="utf-8")
    names: list[str] = []
    follow: list[Path] = []
    for names_block, target in re.findall(
        r'import\s*\{([^}]*)\}\s*from\s*"([^"]+)"', source, re.DOTALL
    ):
        identifiers = [
            raw.strip().split(" as ")[0].strip() for raw in names_block.split(",")
        ]
        identifiers = [name for name in identifiers if name]
        if target.startswith("@/shared/api/"):
            names.extend(identifiers)
            continue
        # 页面常常通过 hooks / 同目录控制器间接调用 API；只跟随项目内模块，限制深度，
        # 否则这张表会把「页面调了哪些接口」变成「只有直接 import 的页面才准」。
        if target.startswith("./") or target.startswith("../"):
            resolved = resolve_relative(source_file, target)
        elif target.startswith("@/"):
            resolved = resolve_module(target)
        else:
            continue
        if resolved is not None and resolved.suffix in (".ts", ".tsx"):
            follow.append(resolved)
    return names, follow


def page_endpoints(page: Path | None, index: dict[str, str]) -> str:
    """Return the endpoints a page reaches, following hooks one level down."""
    if page is None or not page.is_file():
        return "—"
    endpoints: list[str] = []
    seen: set[Path] = set()
    queue: list[tuple[Path, int]] = [(page, 0)]
    direct_http = False
    while queue:
        current, depth = queue.pop(0)
        if current in seen or depth > 3:
            continue
        seen.add(current)
        if "@/shared/api/http" in current.read_text(encoding="utf-8"):
            direct_http = True
        names, follow = collect_imports(current)
        for name in names:
            endpoint = index.get(name)
            if endpoint and endpoint not in endpoints:
                endpoints.append(endpoint)
        for module in follow:
            if module.suffix in (".ts", ".tsx"):
                queue.append((module, depth + 1))
    if not endpoints:
        return "（直接使用 http 客户端）" if direct_http else "—"
    shown = "、".join(f"`{item}`" for item in endpoints[:8])
    return f"{shown} 等 {len(endpoints)} 个" if len(endpoints) > 8 else shown


def build_rows() -> list[tuple[str, str, str, str, str, str]]:
    routes, lazy_imports, _static, _local = parse_router()
    route_keys, children, _labels = parse_menu()
    index = api_endpoint_index()

    ordered = sorted(
        routes,
        key=lambda item: (
            0 if item == "/" else 1 if not item.startswith("/") else 2,
            item,
        ),
    )
    rows = []
    for path in ordered:
        component = routes[path]
        target = lazy_imports.get(component)
        resolved = resolve_module(target) if target else None
        rows.append(
            (
                path,
                page_file(routes, lazy_imports, path),
                component,
                interaction_mode(path, component),
                menu_owner(path, route_keys, children),
                page_endpoints(resolved, index),
            )
        )
    return rows


def render() -> str:
    rows = build_rows()
    modes: dict[str, int] = {}
    for row in rows:
        modes[row[3]] = modes.get(row[3], 0) + 1

    lines = [
        BEGIN,
        "",
        "下表由 `scripts/gen_frontend_registry.py` 从路由表、菜单、磁盘文件与 API 客户端生成，"
        "禁止手工编辑；`scripts/check-frontend-coverage.py` 在 CI 中要求"
        "「文档表 == 重新生成的结果」，因此新增页面必须同时出现在这里。",
        "",
        f"路由 **{len(rows)}** 条，交互模式 **{len(modes)}** 类，"
        f"页面文件 **{len(list(PAGES.rglob('*Page.tsx')))}** 个。",
        "",
        "| # | 路由 | 页面文件 | 组件 | 交互模式 | 菜单归属 | 后端接口 |",
        "|---|---|---|---|---|---|---|",
    ]
    for index, (path, file, component, mode, owner, endpoints) in enumerate(rows, start=1):
        lines.append(
            f"| {index} | `{path}` | `{file}` | `{component}` | {mode} | {owner} | {endpoints} |"
        )

    lines.append("")
    lines.append("交互模式分布：")
    lines.append("")
    for mode, count in sorted(modes.items(), key=lambda item: (-item[1], item[0])):
        lines.append(f"- {mode}：{count} 条路由")
    lines.extend(["", END, ""])
    return "\n".join(lines)


def render_menu() -> str:
    """菜单表：分组 key / 分组标签（i18n 键）/ 子项路由。

    这张表此前是手写的，于是 `menu.tsx` 加了子项而表没跟上（8 个分组对不上）。
    改为生成后，规则 5「文档表 == menu.tsx」不再依赖人工维护。
    """
    route_keys, children, labels = parse_menu()

    lines = [
        MENU_TABLE_BEGIN,
        "",
        "下表由 `scripts/gen_frontend_registry.py` 从 `web/src/features/navigation/menu.tsx`"
        " 生成，禁止手工编辑；`scripts/check-frontend-coverage.py` 在 CI 中要求"
        "「文档表 == `menu.tsx` 声明」，因此新增菜单项必须同时出现在这里。",
        "",
        f"菜单分组 **{len(route_keys)}** 个，导航子项 **{sum(len(value) for value in children.values())}** 项。",
        "",
        "| 分组 `key` | 分组标签 | 子项路由 |",
        "|---|---|---|",
    ]
    for group in sorted(route_keys, key=lambda key: (key,)):
        group_labels = sorted(labels.get(group, ()))
        label = f"`{group_labels[0]}`" if group_labels else "—"
        entries = "、".join(f"`{item}`" for item in sorted(children.get(group, ())))
        lines.append(f"| `{group}` | {label} | {entries or '—'} |")
    lines.extend(["", MENU_TABLE_END, ""])
    return "\n".join(lines)


def replace_block(document: str, begin: str, end: str, block: str) -> str:
    start = document.index(begin)
    stop = document.index(end, start) + len(end)
    return document[:start] + block.rstrip("\n") + document[stop:]


def diff_block(document: str, begin: str, end: str, block: str) -> int:
    start = document.index(begin)
    stop = document.index(end, start)
    current = document[start:stop].splitlines()
    expected = block.splitlines()
    for index in range(max(len(current), len(expected))):
        got = current[index] if index < len(current) else "<missing>"
        want = expected[index] if index < len(expected) else "<missing>"
        if got != want:
            print(f"  首个差异在第 {index + 1} 行：文档 `{got}` / 生成 `{want}`")
            break
    return 1


def main() -> int:
    check_only = "--check" in sys.argv[1:]
    document = DESIGN.read_text(encoding="utf-8")
    for begin, end, label in (
        (MENU_TABLE_BEGIN, MENU_TABLE_END, "菜单表"),
        (REGISTRY_BEGIN, REGISTRY_END, "页面功能注册表"),
    ):
        if begin not in document or end not in document:
            print(f"FAIL: {DESIGN.name} 缺少{label}的标记区间 {begin} / {end}")
            return 1

    menu_block = render_menu()
    registry_block = render()
    updated = replace_block(document, MENU_TABLE_BEGIN, MENU_TABLE_END, menu_block)
    updated = replace_block(updated, REGISTRY_BEGIN, REGISTRY_END, registry_block)

    if check_only:
        if updated != document:
            print(f"FAIL: {DESIGN.name} 的生成表与生成结果不一致，请运行 "
                  "python3 scripts/gen_frontend_registry.py 并提交")
            if menu_block not in document:
                print("  菜单表：")
                diff_block(document, MENU_TABLE_BEGIN, MENU_TABLE_END, menu_block)
            if registry_block not in document:
                print("  页面功能注册表：")
                diff_block(document, REGISTRY_BEGIN, REGISTRY_END, registry_block)
            return 1
        print("PASS: 菜单表与页面功能注册表均与路由表、菜单、磁盘文件一致")
        return 0

    DESIGN.write_text(updated, encoding="utf-8")
    print(f"OK: 已写入 {DESIGN.name}（菜单 {len(parse_menu()[0])} 组，路由 {len(build_rows())} 条）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
