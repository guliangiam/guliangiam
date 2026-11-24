#!/usr/bin/env python3
"""前端权限 key ↔ permissions.yaml 注册表一致性门禁。

Why this exists
---------------
`PermissionGuard` 在 key 不在 `/auth/permissions` 返回集合里时直接隐藏控件，
既不抛错也不回 403：页面少了一个按钮，构建、lint 与组件测试全绿。C1
（98aeb415）与 B5（baf37e8a）都点名缺一道静态门禁，本脚本把「前端写的
action/resource 必须能在 contracts/registries/permissions.yaml 中找到」变成
可执行检查。

扫描面（web/src/**，排除测试与 Storybook 文件）
-----------------------------------------------
  1. `<PermissionGuard action="..." resource="...">` 的全部用法；
  2. `bulkDeletePermission: { permission: "...", resource: "..." }`；
  3. `submitPermission={{ permission: "...", resource: "..." }}`；
  4. `hasAction(...)` / `hasResource(...)` 调用；
  5. `ADMIN_FULL_PERMISSIONS` 手工镜像集合（与注册表做已登记前缀的一致性比对）。

校验规则
--------
  - action 形如 `<frontend_prefix>:<action>`，必须命中注册表的
    `frontend_prefix`（缺省时按 resourceToActionKey 的既有派生：forms→form /
    entries→entry / syncers→syncer）+ `actions` 组合。允许 C1 的 HTTP 动词别名：
    get/head→read、post/put/patch→write、delete→delete。
  - resource 必须命中注册表的 `casbin_resource` 或其派生集合；对
    `casbin_resource: "aigateway:providers"` 这类冒号资源，派生集合同时包含
    `aigateway:providers` 与前端使用的 `aigateway-providers`。
  - 动态构造（变量、模板字符串、条件表达式、属性访问）无法静态判定：不失败，
    但必须逐条进入「不可判定清单」，不得静默放过。
  - 存量缺口登记在 `scripts/permission-key-baseline.txt`（kind|key|file:line|理由）。
    基线只减不增：新增未登记 key 失败；基线项被登记或不再使用时视为过期并失败。

Usage:
  python3 scripts/check-permission-keys.py

输入：contracts/registries/permissions.yaml、web/src/**/*.{ts,tsx}、
      scripts/permission-key-baseline.txt
输出：PASS/FAIL + 扫描统计 + 违规列表 + 不可判定清单 + 基线过期项
Exit code：零违规/零过期 → 0；否则 1
"""

from __future__ import annotations

import re
import sys
from dataclasses import dataclass
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
WEB_ROOT = ROOT / "web" / "src"
REGISTRY_PATH = ROOT / "contracts" / "registries" / "permissions.yaml"
ALLOWLIST_PATH = ROOT / "scripts" / "permission-key-baseline.txt"
MIRROR_PATH = WEB_ROOT / "shared" / "hooks" / "usePermissions.ts"

# 测试、Storybook 与生成物不是运行期权限声明面。Storybook 示例里的
# `view-dashboard` / `edit-settings` 只是演示文案，不代表产品 guard。
_EXCLUDED_FILE_RE = re.compile(r"\.(?:test|spec|stories)\.(?:ts|tsx)$")
_EXCLUDED_DIRS = {"generated", "e2e", "testdata"}

# 注册表未显式声明 frontend_prefix 时，按 internal/app/auth/permissions_query.go
# 的 resourceToActionKey 既有派生。注册表头部注释已写明这三条映射。
_DERIVED_FRONTEND_PREFIX = {
    "forms": "form",
    "entries": "entry",
    "syncers": "syncer",
}

# C1 的查询侧归一：HTTP 动词 → CRUD 词表（internal/app/auth/permissions_query.go
# crudActionAlias）。属性值大小写不敏感。
_CRUD_ALIAS = {
    "get": "read",
    "head": "read",
    "post": "write",
    "put": "write",
    "patch": "write",
    "delete": "delete",
}

_ALLOWLIST_KINDS = frozenset({"action", "resource", "mirror"})


class RegistryParseError(ValueError):
    """注册表 / 基线格式无法解析；门禁应失败而不是静默降级。"""


@dataclass(frozen=True)
class RegistryEntry:
    domain: str
    resource: str
    actions: tuple[str, ...]
    frontend_prefix: str | None
    casbin_resource: str | None


@dataclass(frozen=True)
class Registry:
    entries: tuple[RegistryEntry, ...]
    action_keys: frozenset[str]
    resources: frozenset[str]
    prefixes: frozenset[str]


@dataclass(frozen=True)
class Usage:
    path: str
    line: int
    kind: str
    action: str | None
    resource: str | None
    missing_resource: bool = False


@dataclass(frozen=True)
class Undecidable:
    path: str
    line: int
    kind: str
    expression: str


@dataclass(frozen=True)
class Violation:
    path: str
    line: int
    kind: str
    key: str
    detail: str


@dataclass(frozen=True)
class AllowlistEntry:
    kind: str
    key: str
    location: str
    reason: str


@dataclass(frozen=True)
class MirrorSet:
    actions: tuple[tuple[str, int], ...]
    resources: tuple[tuple[str, int], ...]


@dataclass(frozen=True)
class ValidationResult:
    violations: list[Violation]
    allowed_instances: int
    used_allowlist: frozenset[tuple[str, str]]
    undecidable: list[Undecidable]


# ── 注册表解析（stdlib-only，覆盖本文件的 `domain → resource → key` 形状）─────


def _strip_yaml_comment(line: str) -> str:
    quote = ""
    for index, char in enumerate(line):
        if quote:
            if char == quote:
                quote = ""
        elif char in "\"'":
            quote = char
        elif char == "#":
            return line[:index]
    return line


def _split_inline_list(value: str) -> list[str]:
    items: list[str] = []
    current: list[str] = []
    quote = ""
    for char in value:
        if quote:
            current.append(char)
            if char == quote:
                quote = ""
            continue
        if char in "\"'":
            quote = char
            current.append(char)
            continue
        if char == ",":
            items.append("".join(current).strip())
            current = []
            continue
        current.append(char)
    tail = "".join(current).strip()
    if tail:
        items.append(tail)
    return [_unquote(item) for item in items if item]


def _unquote(value: str) -> str:
    value = value.strip()
    if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
        return value[1:-1]
    return value


def canonical_action(action: str) -> str:
    action = action.strip()
    return _CRUD_ALIAS.get(action.lower(), action)


def split_action_key(key: str) -> tuple[str, str] | None:
    key = key.strip()
    if ":" not in key:
        return None
    prefix, action = key.rsplit(":", 1)
    prefix, action = prefix.strip(), action.strip()
    if not prefix or not action:
        return None
    return prefix, action


def action_is_registered(key: str, registry: Registry) -> bool:
    parts = split_action_key(key)
    if parts is None:
        return False
    prefix, action = parts
    return f"{prefix}:{action}" in registry.action_keys or (
        f"{prefix}:{canonical_action(action)}" in registry.action_keys
    )


def resource_is_registered(resource: str, registry: Registry) -> bool:
    return resource.strip() in registry.resources


def _derived_resource(casbin_resource: str) -> str:
    # 冒号/下划线资源名的前端派生：aigateway:providers → aigateway-providers，
    # aigateway:api_keys → aigateway-api-keys（见 frontendResourceAlias）。
    return casbin_resource.replace(":", "-").replace("_", "-")


def parse_registry(text: str) -> Registry:
    raw_entries: list[dict[str, object]] = []
    domain = ""
    current: dict[str, object] | None = None
    for lineno, raw in enumerate(text.splitlines(), 1):
        line = _strip_yaml_comment(raw).rstrip()
        if not line.strip():
            continue
        indent = len(line) - len(line.lstrip(" "))
        stripped = line.strip()
        if indent == 0 and stripped.endswith(":"):
            domain = stripped[:-1].strip()
            current = None
            continue
        if indent == 2 and stripped.endswith(":"):
            current = {
                "domain": domain,
                "resource": stripped[:-1].strip(),
                "actions": [],
                "frontend_prefix": None,
                "casbin_resource": None,
            }
            raw_entries.append(current)
            continue
        if indent != 4 or current is None:
            continue
        key, separator, value = stripped.partition(":")
        if separator != ":":
            raise RegistryParseError(f"permissions.yaml:{lineno}: 无法解析行 {raw!r}")
        key, value = key.strip(), value.strip()
        if key == "actions":
            if not (value.startswith("[") and value.endswith("]")):
                raise RegistryParseError(
                    f"permissions.yaml:{lineno}: actions 必须是内联列表，实际 {value!r}"
                )
            current["actions"] = _split_inline_list(value[1:-1])
        elif key == "frontend_prefix":
            current["frontend_prefix"] = _unquote(value)
        elif key == "casbin_resource":
            current["casbin_resource"] = _unquote(value)
        # scenario 等说明字段不参与校验。

    built: list[RegistryEntry] = []
    action_keys: set[str] = set()
    resources: set[str] = set()
    prefixes: set[str] = set()
    for entry in raw_entries:
        entry_domain = str(entry["domain"])
        entry_resource = str(entry["resource"])
        actions = tuple(str(action) for action in entry["actions"])
        if not actions:
            raise RegistryParseError(
                f"permissions.yaml: {entry_domain}.{entry_resource} 缺少 actions"
            )
        explicit_prefix = entry["frontend_prefix"]
        prefix = str(explicit_prefix) if explicit_prefix else _DERIVED_FRONTEND_PREFIX.get(
            entry_resource, entry_resource
        )
        casbin_resource = str(entry["casbin_resource"]) if entry["casbin_resource"] else None
        built.append(
            RegistryEntry(
                domain=entry_domain,
                resource=entry_resource,
                actions=actions,
                frontend_prefix=str(explicit_prefix) if explicit_prefix else None,
                casbin_resource=casbin_resource,
            )
        )
        prefixes.add(prefix)
        for action in actions:
            action_keys.add(f"{prefix}:{action}")
            action_keys.add(f"{prefix}:{canonical_action(action)}")
        if casbin_resource:
            resources.add(casbin_resource)
            if ":" in casbin_resource:
                resources.add(_derived_resource(casbin_resource))
        else:
            resources.add(entry_resource)
    return Registry(
        entries=tuple(built),
        action_keys=frozenset(action_keys),
        resources=frozenset(resources),
        prefixes=frozenset(prefixes),
    )


# ── JS/TSX 词法辅助（只屏蔽注释，保留字符串内容）────────────────────────────


def mask_js_comments(text: str) -> str:
    """把 // 与 /* */ 注释内容替换为空格（保留换行）。

    已知边界：模板字符串 ${...} 内部不再细分注释；正则字面量中的 `//`
    可能被误判为行注释。两处都不影响 PermissionGuard 的属性值解析。
    """
    chars = list(text)
    index = 0
    length = len(text)
    state = "code"
    quote = ""
    while index < length:
        char = text[index]
        if state == "code":
            if char == "/" and index + 1 < length and text[index + 1] == "/":
                chars[index] = chars[index + 1] = " "
                index += 2
                while index < length and text[index] != "\n":
                    chars[index] = " "
                    index += 1
                continue
            if char == "/" and index + 1 < length and text[index + 1] == "*":
                chars[index] = chars[index + 1] = " "
                index += 2
                while index < length:
                    if text.startswith("*/", index):
                        chars[index] = chars[index + 1] = " "
                        index += 2
                        break
                    if text[index] != "\n":
                        chars[index] = " "
                    index += 1
                continue
            if char in "\"'":
                state = "string"
                quote = char
            elif char == "`":
                state = "template"
            index += 1
            continue
        if state == "string":
            if char == "\\":
                index += 2
                continue
            if char == quote:
                state = "code"
            elif char == "\n":
                # JSX 文本里的撇号（Don't）不应吞掉后续代码。
                state = "code"
            index += 1
            continue
        # template
        if char == "\\":
            index += 2
            continue
        if char == "`":
            state = "code"
        index += 1
    return "".join(chars)


def _balanced_end(text: str, start: int, open_char: str) -> int | None:
    """返回与 start 处 open_char 配对的闭合字符之后的下标。"""
    close_char = "}" if open_char == "{" else ")" if open_char == "(" else "]"
    depth = 0
    index = start
    quote = ""
    while index < len(text):
        char = text[index]
        if quote:
            if char == "\\":
                index += 2
                continue
            if char == quote:
                quote = ""
            index += 1
            continue
        if char in "\"'`":
            quote = char
            index += 1
            continue
        if char == open_char:
            depth += 1
        elif char == close_char:
            depth -= 1
            if depth == 0:
                return index + 1
        index += 1
    return None


def _jsx_tag_end(text: str, start: int) -> int | None:
    """返回 JSX 开始标签结尾 `>` 之后的下标（跳过 {} 内的 `>`）。"""
    depth = 0
    index = start
    quote = ""
    while index < len(text):
        char = text[index]
        if quote:
            if char == "\\":
                index += 2
                continue
            if char == quote:
                quote = ""
            index += 1
            continue
        if char in "\"'`":
            quote = char
            index += 1
            continue
        if char == "{":
            depth += 1
        elif char == "}":
            depth = max(0, depth - 1)
        elif char == ">" and depth == 0:
            return index + 1
        index += 1
    return None


def _string_literal(expr: str) -> str | None:
    expr = expr.strip()
    if len(expr) < 2 or expr[0] not in "\"'" or expr[-1] != expr[0]:
        return None
    body = expr[1:-1]
    index = 0
    while index < len(body):
        if body[index] == "\\":
            index += 2
            continue
        if body[index] == expr[0]:
            return None
        index += 1
    return re.sub(r"\\(.)", r"\1", body)


def _plain_template_literal(expr: str) -> str | None:
    expr = expr.strip()
    if len(expr) < 2 or expr[0] != "`" or expr[-1] != "`":
        return None
    body = expr[1:-1]
    index = 0
    while index < len(body):
        if body[index] == "\\":
            index += 2
            continue
        if body.startswith("${", index):
            return None
        if body[index] == "`":
            return None
        index += 1
    return re.sub(r"\\(.)", r"\1", body)


def _strip_outer_parens(expr: str) -> str:
    expr = expr.strip()
    while expr.startswith("("):
        end = _balanced_end(expr, 0, "(")
        if end != len(expr):
            break
        expr = expr[1:-1].strip()
    return expr


def classify_expression(expr: str) -> tuple[str, str]:
    """返回 (literal|dynamic, 值或表达式)。"""
    expr = _strip_outer_parens(expr)
    for parser in (_string_literal, _plain_template_literal):
        value = parser(expr)
        if value is not None:
            return "literal", value
    return "dynamic", re.sub(r"\s+", " ", expr)[:120]


def parse_jsx_attrs(body: str) -> dict[str, tuple[str, str] | None]:
    """解析开始标签内部的顶层属性；嵌套在 {} / fallback JSX 里的同名属性忽略。"""
    attrs: dict[str, tuple[str, str] | None] = {}
    index = 0
    while index < len(body):
        char = body[index]
        if char.isspace() or char == "/":
            index += 1
            continue
        if char == "{":  # {...spread}
            end = _balanced_end(body, index, "{")
            index = end if end is not None else index + 1
            continue
        match = re.match(r"[A-Za-z_:][\w:.-]*", body[index:])
        if not match:
            index += 1
            continue
        name = match.group(0)
        index += match.end()
        while index < len(body) and body[index].isspace():
            index += 1
        if index >= len(body) or body[index] != "=":
            attrs[name] = None
            continue
        index += 1
        while index < len(body) and body[index].isspace():
            index += 1
        if index >= len(body):
            attrs[name] = None
            continue
        if body[index] in "\"'":
            quote = body[index]
            end = index + 1
            while end < len(body):
                if body[end] == "\\":
                    end += 2
                    continue
                if body[end] == quote:
                    break
                end += 1
            value = body[index + 1 : end]
            attrs[name] = ("literal", re.sub(r"\\(.)", r"\1", value))
            index = end + 1
            continue
        if body[index] == "{":
            end = _balanced_end(body, index, "{")
            if end is None:
                attrs[name] = ("dynamic", body[index:])
                break
            attrs[name] = classify_expression(body[index + 1 : end - 1])
            index = end
            continue
        token = re.match(r"[^\s/>]+", body[index:])
        if token:
            attrs[name] = ("dynamic", token.group(0))
            index += token.end()
        else:
            attrs[name] = None
            index += 1
    return attrs


# ── 扫描 ────────────────────────────────────────────────────────────────────


def _line_of(text: str, offset: int) -> int:
    return text.count("\n", 0, offset) + 1


def _attr_literal(
    undecidable: list[Undecidable],
    path: str,
    line: int,
    kind: str,
    attr: tuple[str, str] | None,
    field: str,
) -> str | None:
    if attr is None:
        return None
    nature, value = attr
    if nature == "dynamic":
        undecidable.append(Undecidable(path, line, f"{kind}.{field}", value))
        return None
    return value


def scan_permission_guards(
    text: str, path: str, masked: str
) -> tuple[list[Usage], list[Undecidable]]:
    usages: list[Usage] = []
    undecidable: list[Undecidable] = []
    for match in re.finditer(r"<\s*PermissionGuard\b", masked):
        end = _jsx_tag_end(masked, match.end())
        if end is None:
            continue
        line = _line_of(text, match.start())
        raw_attrs = masked[match.end() : end - 1]
        if re.match(r"\s*\{\s*\.\.\.", raw_attrs):
            undecidable.append(
                Undecidable(path, line, "PermissionGuard.spread", raw_attrs.strip()[:120])
            )
        attrs = parse_jsx_attrs(raw_attrs)
        action = _attr_literal(
            undecidable, path, line, "PermissionGuard", attrs.get("action"), "action"
        )
        resource = _attr_literal(
            undecidable, path, line, "PermissionGuard", attrs.get("resource"), "resource"
        )
        # 动态 guard 也计入扫描面（其 action/resource 进入不可判定清单）。
        usages.append(Usage(path, line, "PermissionGuard", action, resource))
    return usages, undecidable


def _read_value_at(body: str, start: int) -> tuple[str, str, int] | None:
    """从 start 读取一个 JS 值，返回 (nature, value, 结束下标)。"""
    index = start
    while index < len(body) and body[index].isspace():
        index += 1
    if index >= len(body):
        return None
    if body[index] == "{":
        end = _balanced_end(body, index, "{")
        if end is None:
            return ("dynamic", "<未闭合对象>", len(body))
        return classify_expression(body[index + 1 : end - 1]) + (end,)
    if body[index] in "\"'`":
        quote = body[index]
        end = index + 1
        while end < len(body):
            if body[end] == "\\":
                end += 2
                continue
            if body[end] == quote:
                break
            end += 1
        return classify_expression(body[index : end + 1]) + (end + 1,)
    token = re.match(r"[^\s,}]+", body[index:])
    if token:
        return ("dynamic", token.group(0), index + token.end())
    return None


def _extract_object_fields(body: str) -> dict[str, tuple[str, str]]:
    """提取对象字面量里的 permission/resource 字段。

    只能在字符串外、且键名后紧跟 `:` 时命中，避免把值
    `"permission:delete"` 内部误当成新字段。
    """
    fields: dict[str, tuple[str, str]] = {}
    index = 0
    quote = ""
    while index < len(body):
        char = body[index]
        if quote:
            if char == "\\":
                index += 2
                continue
            if char == quote:
                quote = ""
            index += 1
            continue
        if char in "\"'`":
            quote = char
            index += 1
            continue
        if char == "{" or char == "}" or char == ",":
            index += 1
            continue
        match = re.match(r"(permission|resource)\b", body[index:])
        if not match:
            index += 1
            continue
        previous = body[index - 1] if index > 0 else ""
        if previous in "._$" or previous.isalnum():
            index += match.end()
            continue
        key = match.group(1)
        cursor = index + match.end()
        while cursor < len(body) and body[cursor].isspace():
            cursor += 1
        if cursor >= len(body) or body[cursor] != ":":
            index += match.end()
            continue
        parsed = _read_value_at(body, cursor + 1)
        if parsed is not None:
            nature, value, end = parsed
            fields[key] = (nature, value)
            index = end
        else:
            index = cursor + 1
    return fields


def scan_shared_slots(
    text: str, path: str, masked: str
) -> tuple[list[Usage], list[Undecidable]]:
    usages: list[Usage] = []
    undecidable: list[Undecidable] = []
    for match in re.finditer(r"\b(bulkDeletePermission|submitPermission)\b", masked):
        index = match.end()
        while index < len(masked) and masked[index].isspace():
            index += 1
        if index >= len(masked) or masked[index] not in "=:":
            continue  # `config.bulkDeletePermission` 引用、类型属性等不是声明
        index += 1
        while index < len(masked) and masked[index].isspace():
            index += 1
        if index >= len(masked) or masked[index] != "{":
            # 变量/属性访问形式（如 `submitPermission={submitPermission}`）无法
            # 静态判定；类型声明（`SubmitPermission`）首字母大写，按类型跳过。
            token = re.match(r"[^\s,;}]+", masked[index:]) if index < len(masked) else None
            if token and not token.group(0)[:1].isupper():
                undecidable.append(
                    Undecidable(
                        path,
                        _line_of(text, match.start()),
                        match.group(1),
                        token.group(0)[:120],
                    )
                )
            continue
        end = _balanced_end(masked, index, "{")
        if end is None:
            undecidable.append(
                Undecidable(path, _line_of(text, match.start()), match.group(1), "<未闭合对象>")
            )
            continue
        body = masked[index + 1 : end - 1]
        fields = _extract_object_fields(body)
        if not fields:
            continue  # 纯类型声明（permission: string / resource: string）无字面量
        line = _line_of(text, match.start())
        action = None
        resource = None
        missing_resource = False
        for field_name, (nature, value) in fields.items():
            if nature == "dynamic":
                # 类型声明 `permission: string` 不含引号，按类型而非动态使用跳过。
                if value in {"string", "string;"}:
                    continue
                undecidable.append(
                    Undecidable(path, line, f"{match.group(1)}.{field_name}", value)
                )
                continue
            if field_name == "permission":
                action = value
            else:
                resource = value
        if (action or resource) and not resource:
            missing_resource = True
        if action or resource or missing_resource:
            usages.append(
                Usage(path, line, match.group(1), action, resource, missing_resource)
            )
    return usages, undecidable


def scan_permission_helpers(
    text: str, path: str, masked: str
) -> tuple[list[Usage], list[Undecidable]]:
    usages: list[Usage] = []
    undecidable: list[Undecidable] = []
    for match in re.finditer(r"\b(hasAction|hasResource)\s*\(", masked):
        end = _balanced_end(masked, match.end() - 1, "(")
        if end is None:
            continue
        nature, value = classify_expression(masked[match.end() : end - 1])
        line = _line_of(text, match.start())
        if nature == "dynamic":
            undecidable.append(Undecidable(path, line, match.group(1), value))
            continue
        if match.group(1) == "hasAction":
            usages.append(Usage(path, line, "hasAction", value, None))
        else:
            usages.append(Usage(path, line, "hasResource", None, value))
    return usages, undecidable


def scan_source(
    text: str, path: str
) -> tuple[list[Usage], list[Undecidable]]:
    masked = mask_js_comments(text)
    guards = scan_permission_guards(text, path, masked)
    slots = scan_shared_slots(text, path, masked)
    helpers = scan_permission_helpers(text, path, masked)
    return (
        guards[0] + slots[0] + helpers[0],
        guards[1] + slots[1] + helpers[1],
    )


def iter_web_sources() -> list[Path]:
    sources: list[Path] = []
    for path in sorted(WEB_ROOT.rglob("*")):
        if path.suffix not in (".ts", ".tsx") or not path.is_file():
            continue
        relative_parts = path.relative_to(WEB_ROOT).parts
        if any(part in _EXCLUDED_DIRS for part in relative_parts[:-1]):
            continue
        if _EXCLUDED_FILE_RE.search(path.name):
            continue
        sources.append(path)
    return sources


# ── ADMIN_FULL_PERMISSIONS 镜像 ──────────────────────────────────────────────


def _array_values_with_lines(
    text: str, masked: str, absolute_start: int, open_index: int
) -> list[tuple[str, int]]:
    end = _balanced_end(masked, open_index, "[")
    if end is None:
        raise RegistryParseError(f"{MIRROR_PATH}:{_line_of(text, open_index)}: 数组未闭合")
    values: list[tuple[str, int]] = []
    for match in re.finditer(r"\"([^\"]+)\"|'([^']+)'", masked[open_index + 1 : end - 1]):
        value = match.group(1) or match.group(2)
        values.append((value, _line_of(text, absolute_start + open_index + 1 + match.start())))
    return values


def parse_mirror(text: str) -> MirrorSet:
    masked = mask_js_comments(text)
    declaration = re.search(r"ADMIN_FULL_PERMISSIONS[^=]*=\s*\{", masked)
    if not declaration:
        raise RegistryParseError(f"{MIRROR_PATH}: 找不到 ADMIN_FULL_PERMISSIONS 对象")
    object_start = masked.index("{", declaration.start())
    object_end = _balanced_end(masked, object_start, "{")
    if object_end is None:
        raise RegistryParseError(f"{MIRROR_PATH}: ADMIN_FULL_PERMISSIONS 对象未闭合")
    body = masked[object_start + 1 : object_end - 1]
    arrays: dict[str, list[tuple[str, int]]] = {}
    for field in ("actions", "resources"):
        field_match = re.search(rf"\b{field}\s*:\s*\[", body)
        if not field_match:
            raise RegistryParseError(f"{MIRROR_PATH}: ADMIN_FULL_PERMISSIONS 缺少 {field} 数组")
        open_index = object_start + 1 + field_match.end() - 1
        arrays[field] = _array_values_with_lines(text, masked, 0, open_index)
    if not arrays["actions"] or not arrays["resources"]:
        raise RegistryParseError(f"{MIRROR_PATH}: ADMIN_FULL_PERMISSIONS 数组为空")
    return MirrorSet(tuple(arrays["actions"]), tuple(arrays["resources"]))


def mirror_drifts(mirror: MirrorSet, registry: Registry) -> list[Violation]:
    drifts: list[Violation] = []
    for key, line in mirror.actions:
        parts = split_action_key(key)
        if parts is None:
            if key in registry.action_keys:
                continue
            drifts.append(
                Violation(
                    str(MIRROR_PATH.relative_to(ROOT)),
                    line,
                    "mirror",
                    key,
                    "镜像 key 不是 prefix:action 形状",
                )
            )
            continue
        prefix, _ = parts
        if prefix not in registry.prefixes:
            continue  # 未登记前缀属 G-2 注册表覆盖缺口，页面 key 侧单独控管
        if not action_is_registered(key, registry):
            drifts.append(
                Violation(
                    str(MIRROR_PATH.relative_to(ROOT)),
                    line,
                    "mirror",
                    key,
                    "admin 镜像包含该前缀下未登记的 action，PermissionGuard 兜底会与注册表不一致",
                )
            )
    # 资源侧不按 entry 名做镜像比对：注册表既有 entry（providers/models）与
    # AI 网关 casbin_resource（aigateway:providers）是不同命名空间，按 entry 名
    # 比对会把 IAM 的 providers/models 误报成漂移。共享槽资源由页面扫描侧校验。
    return drifts


def mirror_coverage_violations(usages: list[Usage], mirror: MirrorSet) -> list[Violation]:
    """页面使用的 key 必须同时存在于管理员兜底镜像。

    `/auth/permissions` 失败时，`usePermissions` 对管理员回退到
    ADMIN_FULL_PERMISSIONS；镜像少一个 key，管理员在降级模式下同样会看到控件
    静默消失。按唯一 key 报告首个出现位置，避免同一 key 刷屏。
    """
    mirror_actions = {key for key, _line in mirror.actions}
    mirror_canonical: set[str] = set()
    for key in mirror_actions:
        parts = split_action_key(key)
        if parts is not None:
            mirror_canonical.add(f"{parts[0]}:{canonical_action(parts[1])}")
    mirror_resources = {key for key, _line in mirror.resources}
    missing: dict[str, Usage] = {}
    for usage in usages:
        if usage.action:
            parts = split_action_key(usage.action)
            canonical = (
                f"{parts[0]}:{canonical_action(parts[1])}" if parts is not None else usage.action
            )
            if usage.action not in mirror_actions and canonical not in mirror_canonical:
                missing.setdefault(usage.action, usage)
        if usage.resource and usage.resource not in mirror_resources:
            missing.setdefault(usage.resource, usage)
    return [
        Violation(
            usage.path,
            usage.line,
            "mirror",
            key,
            "页面使用了该 key，但 ADMIN_FULL_PERMISSIONS 兜底镜像缺失；"
            "权限接口失败时管理员会看到控件被隐藏",
        )
        for key, usage in sorted(missing.items())
    ]


# ── 基线 / allowlist ─────────────────────────────────────────────────────────


def parse_allowlist(text: str) -> list[AllowlistEntry]:
    entries: list[AllowlistEntry] = []
    seen: set[tuple[str, str]] = set()
    for lineno, raw in enumerate(text.splitlines(), 1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        parts = [part.strip() for part in line.split("|", 3)]
        if len(parts) != 4:
            raise RegistryParseError(
                f"permission-key-baseline.txt:{lineno}: 格式必须为 kind|key|file:line|reason"
            )
        kind, key, location, reason = parts
        if kind not in _ALLOWLIST_KINDS:
            raise RegistryParseError(
                f"permission-key-baseline.txt:{lineno}: 未知 kind {kind!r}"
            )
        if not key or not reason or not re.match(r".+:\d+$", location):
            raise RegistryParseError(
                f"permission-key-baseline.txt:{lineno}: key/reason/file:line 不完整"
            )
        if (kind, key) in seen:
            raise RegistryParseError(
                f"permission-key-baseline.txt:{lineno}: 重复条目 {kind}|{key}"
            )
        seen.add((kind, key))
        entries.append(AllowlistEntry(kind, key, location, reason))
    return entries


def _allowlist_action_key(key: str) -> str:
    parts = split_action_key(key)
    if parts is None:
        return key
    prefix, action = parts
    return f"{prefix}:{canonical_action(action)}"


def validate_usages(
    usages: list[Usage],
    registry: Registry,
    allowlist: list[AllowlistEntry],
) -> ValidationResult:
    action_allow = {
        _allowlist_action_key(entry.key): entry
        for entry in allowlist
        if entry.kind == "action"
    }
    resource_allow = {
        entry.key: entry for entry in allowlist if entry.kind == "resource"
    }
    used: set[tuple[str, str]] = set()
    violations: list[Violation] = []
    allowed_instances = 0

    for usage in usages:
        if usage.action:
            parts = split_action_key(usage.action)
            if parts is None:
                if usage.action in action_allow:
                    used.add(("action", _allowlist_action_key(usage.action)))
                    allowed_instances += 1
                else:
                    violations.append(
                        Violation(
                            usage.path,
                            usage.line,
                            "action",
                            usage.action,
                            "action 必须是 <frontend_prefix>:<read|write|delete>",
                        )
                    )
            elif action_is_registered(usage.action, registry):
                pass
            else:
                canonical = _allowlist_action_key(usage.action)
                if canonical in action_allow:
                    used.add(("action", canonical))
                    allowed_instances += 1
                else:
                    violations.append(
                        Violation(
                            usage.path,
                            usage.line,
                            "action",
                            usage.action,
                            "注册表无此前缀+action 组合（含 CRUD alias）",
                        )
                    )
        if usage.resource:
            if resource_is_registered(usage.resource, registry):
                pass
            elif usage.resource in resource_allow:
                used.add(("resource", usage.resource))
                allowed_instances += 1
            else:
                violations.append(
                    Violation(
                        usage.path,
                        usage.line,
                        "resource",
                        usage.resource,
                        "注册表 casbin_resource/派生集合无此资源",
                    )
                )
        if usage.missing_resource and usage.kind in {"bulkDeletePermission", "submitPermission"}:
            violations.append(
                Violation(
                    usage.path,
                    usage.line,
                    "resource",
                    "<missing>",
                    f"{usage.kind} 共享槽必须同时声明 resource",
                )
            )
    return ValidationResult(
        violations=violations,
        allowed_instances=allowed_instances,
        used_allowlist=frozenset(used),
        undecidable=[],
    )


def stale_allowlist_entries(
    allowlist: list[AllowlistEntry], used: frozenset[tuple[str, str]]
) -> list[AllowlistEntry]:
    return [
        entry for entry in allowlist if (entry.kind, _allowlist_action_key(entry.key) if entry.kind == "action" else entry.key) not in used
    ]


# ── main ────────────────────────────────────────────────────────────────────


def _print_undecidable(items: list[Undecidable]) -> None:
    print(f"\n不可判定清单（{len(items)} 处，动态构造不失败但必须人工复核）：")
    for item in items:
        print(f"  {item.path}:{item.line}  [{item.kind}]  {item.expression}")


def main() -> int:
    print("=" * 64)
    print("  前端权限 key ↔ permissions.yaml 注册表一致性门禁")
    print("=" * 64)
    print()

    if not WEB_ROOT.is_dir():
        print(f"ERROR: {WEB_ROOT} 不存在，跳过前端门禁")
        return 0
    if not REGISTRY_PATH.is_file():
        print(f"FAIL: 缺少注册表 {REGISTRY_PATH.relative_to(ROOT)}")
        print("FAIL_COUNT=1")
        return 1
    if not ALLOWLIST_PATH.is_file():
        print(f"FAIL: 缺少存量基线 {ALLOWLIST_PATH.relative_to(ROOT)}")
        print("FAIL_COUNT=1")
        return 1

    try:
        registry = parse_registry(REGISTRY_PATH.read_text(encoding="utf-8"))
        allowlist = parse_allowlist(ALLOWLIST_PATH.read_text(encoding="utf-8"))
        mirror = parse_mirror(MIRROR_PATH.read_text(encoding="utf-8"))
    except (OSError, RegistryParseError) as error:
        print(f"FAIL: {error}")
        print("FAIL_COUNT=1")
        return 1

    print(
        f"registry: {len(registry.entries)} 个 resource / "
        f"{len(registry.action_keys)} 个 action key / {len(registry.resources)} 个 resource 值"
    )

    all_usages: list[Usage] = []
    all_undecidable: list[Undecidable] = []
    scanned_files = 0
    for path in iter_web_sources():
        scanned_files += 1
        relative = str(path.relative_to(ROOT))
        usages, undecidable = scan_source(path.read_text(encoding="utf-8"), relative)
        all_usages.extend(usages)
        all_undecidable.extend(undecidable)

    kind_counts: dict[str, int] = {}
    for usage in all_usages:
        kind_counts[usage.kind] = kind_counts.get(usage.kind, 0) + 1
    static_count = sum(1 for usage in all_usages if usage.action or usage.resource)
    touched_files = len({usage.path for usage in all_usages})
    print(
        f"scan: {scanned_files} 个文件 / {len(all_usages)} 处权限声明面"
        f"（{', '.join(f'{k}={v}' for k, v in sorted(kind_counts.items()))}；"
        f"静态 {static_count} 处；命中 {touched_files} 个文件）"
    )
    print(
        f"mirror: ADMIN_FULL_PERMISSIONS {len(mirror.actions)} 个 action / "
        f"{len(mirror.resources)} 个 resource"
    )

    validation = validate_usages(all_usages, registry, allowlist)
    undecidable = all_undecidable + validation.undecidable
    mirror_violations: list[Violation] = []
    seen_mirror_keys: set[str] = set()
    for violation in mirror_drifts(mirror, registry) + mirror_coverage_violations(
        all_usages, mirror
    ):
        if violation.key in seen_mirror_keys:
            continue
        seen_mirror_keys.add(violation.key)
        mirror_violations.append(violation)

    used = set(validation.used_allowlist)
    mirror_allow = {entry.key: entry for entry in allowlist if entry.kind == "mirror"}
    allowed_mirror = 0
    for violation in mirror_violations:
        if violation.key in mirror_allow:
            used.add(("mirror", violation.key))
            allowed_mirror += 1
        else:
            validation.violations.append(violation)

    used_mirror = sum(1 for item in used if item[0] == "mirror")
    allowed_total = validation.allowed_instances + allowed_mirror
    stale = stale_allowlist_entries(allowlist, frozenset(used))

    unregistered_mirror_prefixes = sorted(
        {split_action_key(key)[0] for key, _ in mirror.actions if split_action_key(key)}
        - set(registry.prefixes)
    )

    if validation.violations:
        print(f"\nFAIL: {len(validation.violations)} 处未登记的权限 key（且不在基线内）")
        for violation in validation.violations:
            print(
                f"  {violation.path}:{violation.line}  [{violation.kind}] "
                f"{violation.key}  →  {violation.detail}"
            )
    if stale:
        print(f"\nFAIL: {len(stale)} 条存量基线已过期（key 已登记或不再使用，必须删除）")
        for entry in stale:
            print(f"  [{entry.kind}] {entry.key}  {entry.location}  ← {entry.reason}")
    if undecidable:
        _print_undecidable(undecidable)
    if unregistered_mirror_prefixes:
        print(
            f"\nmirror 未登记前缀（{len(unregistered_mirror_prefixes)} 个，属 G-2 注册表覆盖缺口，"
            "页面 key 侧由基线控管）："
        )
        print("  " + ", ".join(unregistered_mirror_prefixes))

    if validation.violations or stale:
        print(f"\nFAIL_COUNT={len(validation.violations) + len(stale)}")
        print("\n修复指引：")
        print("  1. 已登记域：把页面 key 改成注册表内 `<frontend_prefix>:read|write|delete`")
        print("  2. 新权限点：先改 contracts/registries/permissions.yaml（并重生成渲染层）")
        print("  3. 仅限评审确认的存量缺口：在 scripts/permission-key-baseline.txt 登记")
        print("     `kind|key|file:line|理由`；该文件只减不增")
        return 1

    print(
        f"\nPASS: {len(all_usages)} 处权限声明全部可解析；"
        f"未登记 key 0 处；基线覆盖 {allowed_total} 处"
        f"（action/resource {validation.allowed_instances} + mirror {allowed_mirror}）；"
        f"基线条目 {len(allowlist)} 条（mirror 命中 {used_mirror} 条）；"
        f"不可判定 {len(undecidable)} 处（见上方清单）"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
