#!/usr/bin/env python3
"""路由资源派生一致性门禁（warn 阶段）。

Why this exists
---------------
`internal/transport/http/api/v1/auth/auth_middleware.go` 用 **HTTP 方法 + URL
路径** 在运行期推导 Casbin 的 resource/action（`resourceFromPath` +
`casbinActionCandidates`），默认不读取路由声明的 `Permission`。唯一例外是
`internal/bootstrap/chi.go` 显式登记的权限点覆盖（`auth.PermissionConfig`）：
命中覆盖时运行期改用 `splitDeclaredPermission` 的**首个冒号**切分结果，且只裁决
该动作（`actions = []string{action}`）。除覆盖命中的路由外，声明 `Permission`
只用于 OpenAPI 文档与治理登记，因此二者可以长期静默漂移：

  * 声明资源 ≠ 派生资源（例如 `/api/v1/users/{id}/orders` 声明 `orders:read`，
    运行期只看首段 `users`）——声明永远不会命中，授权粒度实际退化为宿主模块；
  * 声明动作 ∉ `casbinActionCandidates(method)`（例如 GET 声明 `:delete`，
    或 `aigateway:providers` 这种只有资源没有动作的写法）。

这类漂移不会让任何单测或 typecheck 变红，只会在越权审计里以「声明看似收紧、
实际按路径首段 + 方法别名放行」的形态出现。本门禁把「路径 → 运行期资源」与
「声明资源/动作」做成可重复的静态判定，并按 chi.go 的覆盖表放过真正生效的声明。

规则
----
R1  声明资源必须等于 `resourceFromPath(pattern)` 的结果。
R2  声明动作必须属于 `casbinActionCandidates(method)`（未写动作视为失败）。
OVERRIDE  chi.go 覆盖表命中且声明串完全一致的路由：运行期确实按声明授权，
      因此按 Go 的 `splitDeclaredPermission`（**首个冒号**）拆资源/动作。R1 仍
      要求资源前缀等于派生资源；R2 不适用（动作由覆盖自身钉选，运行期只裁决它）。
      覆盖表里没有这条声明时，退回默认口径照常判红——补登记才是唯一出路。
NESTED  **嵌套高危，永不可豁免**：pattern 里首段之后出现 `{var}` 且声明资源
      ≠ 派生资源 → 直接判红，warn 阶段也生效。它覆盖
      `/users/{id}/orders`、`/users/{id}/payments` 这类宿主直通路径。
KNOWN-RED  既有失配登记在 `contracts/registries/known-red.yaml` 的
      `route_derivation_known_red` 列表：命中且未过期 → 计入 PASS（打印
      `KNOWN-RED n/22` 摘要）；已过期 → 判红；不在基线内 → 立即判红。
      它只抑制「已经登记过的历史失配」，不改变 NESTED 的不可豁免语义，也不会
      放过任何新增失配。
EXEMPTION  R1/R2 可登记豁免，但 `expires_on` 过期、字段缺失或试图豁免
      NESTED 一律判红。NESTED 类只允许走 known-red 的「到期再红」，不允许豁免。

与运行时同源
------------
脚本无法直接 import Go，因此 `resource_from_path` / `casbin_action_candidates`
/ `split_declared_permission` 是按
`internal/transport/http/api/v1/auth/auth_rbac_models.go` 逐分支复刻的：

  * `resourceFromPath`（Go 同文件 L131-L172）：AI 前缀、faces→chat、
    global 取第二段、默认首段去 `:`、AI 实体去 `:` 且 `-`→`_`；
  * `casbinActionCandidates`（Go 同文件 L97-L112）：GET/HEAD→read、
    POST/PUT/PATCH→write、DELETE→delete，verb 优先、alias 兜底；
  * `splitDeclaredPermission`（Go 同文件 L121-L127）：**首个**冒号切分，
    因此 `users:password:reset` = 资源 `users` + 动作 `password:reset`。

覆盖表本身是数据不是算法：`load_permission_overrides` 直接读
`internal/bootstrap/chi.go` 里的 `auth.PermissionConfig` 字面量，Go 侧新增或
删除覆盖条目会自动反映到判定里，无需回写常量。

为防止 Go 侧特判分支变化而脚本未同步，脚本对上述函数做**规范化源码指纹**
（去注释、压空白后 SHA-256）并在每次运行时比对。Go 函数改动但脚本未更新时
直接判红，并打印重新生成指纹的命令：

    python3 scripts/check-route-resource-derivation.py --print-fingerprints

warn 阶段口径
-------------
  * 默认：只有**本次变更集**（`git diff` 新增行）命中的路由、以及 NESTED 规则
    判红；其余全量只报告（WARN）不阻断。
  * `--strict`：全部 R1/R2 失配阻断（供后续收敛阶段切换）。
  * 变更集基准：`--base-ref` > `ROUTE_DERIVATION_BASE` > `origin/$GITHUB_BASE_REF`
    > `origin/main` > `HEAD`。无法解析基准时降级为「全量报告 + NESTED 阻断」。

豁免注册表
-----------
`contracts/registries/route-resource-exemptions.yaml`：每个条目必须包含
`pattern`、`reason`、`owner`、`expires_on`（YYYY-MM-DD），`method` 可选
（缺省 `*`）。过期即红；NESTED 命中豁免条目时按「不可豁免」判红。

排除范围（与运行时一致）
------------------------
  * `Permission: public` 与不声明 Permission 的路由：不经 Casbin；
  * `/api/v1/auth/*`：`isAuthRequiringSession` / 公开登录端点自行处理会话，
    运行时跳过 Casbin（`auth_middleware.go` Gate 2a/2b）；
  * `Surface: SurfaceProtocol`：协议面（SCIM/CAS 等）使用协议自身认证；
  * 非 `/api/v1/` 路径：`resourceFromPath` 对它们返回空串。

用法
----
    python3 scripts/check-route-resource-derivation.py                # warn 阶段
    python3 scripts/check-route-resource-derivation.py --strict       # 未来收紧
    python3 scripts/check-route-resource-derivation.py --base-ref HEAD~1
    python3 scripts/check-route-resource-derivation.py --self-test    # 门禁自测
    python3 scripts/check-route-resource-derivation.py --print-fingerprints

退出码：0 = 无阻断项（可含 WARN）；1 = 有阻断项；2 = 参数/环境错误。

输入：internal/transport/http/api/v1/**/routes.go、auth_rbac_models.go、
      contracts/registries/route-resource-exemptions.yaml、
      contracts/registries/known-red.yaml、git 变更集
输出：PASS/FAIL + 逐条失配 + 分类计数 + `FAIL_COUNT=N`
      + `KNOWN-RED n/22` / `WARN n` 汇总
消费方：scripts/verify-docs.sh（RUN_CODE_CHECKS=1）、CI gates 作业
"""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import os
import re
import subprocess
import sys
from dataclasses import dataclass, field

try:
    import yaml
except ImportError:  # pragma: no cover - 缺 PyYAML 时显式失败，不静默降级
    yaml = None

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
ROUTES_DIR = os.path.join(REPO, "internal", "transport", "http", "api", "v1")
GO_MODELS = os.path.join(
    ROUTES_DIR, "auth", "auth_rbac_models.go"
)
GO_WIRE = os.path.join(REPO, "internal", "bootstrap", "chi.go")
EXEMPTIONS_FILE = os.path.join(
    REPO, "contracts", "registries", "route-resource-exemptions.yaml"
)
KNOWN_RED_FILE = os.path.join(REPO, "contracts", "registries", "known-red.yaml")
KNOWN_RED_KEY = "route_derivation_known_red"
KNOWN_RED_GATE = "check-route-resource-derivation"
KNOWN_RED_CODES = {"R1", "R2", "NESTED"}
KNOWN_RED_REQUIRED = (
    "gate",
    "pattern",
    "reason",
    "owner",
    "target_batch",
    "expires_on",
    "evidence_ref",
)
ROUTE_DIFF_PATH = "internal/transport/http/api/v1"

# ── 运行时规则复刻 ─────────────────────────────────────────────────────────
# 与 auth_rbac_models.go 的 casbinActionCandidates 对应（L97-L112）。
ACTION_ALIASES = {
    "GET": ("get", "read"),
    "HEAD": ("head", "read"),
    "POST": ("post", "write"),
    "PUT": ("put", "write"),
    "PATCH": ("patch", "write"),
    "DELETE": ("delete", "delete"),
}

# 声明动作词表：只有末尾是这些词时才把 `res:act` 拆成资源 + 动作。
# `aigateway:providers`、`scim.users`、`mcpservers.proxy` 这类没有动作的
# 声明保持整串作为资源，交由 R1 精确比对、R2 报「未声明动作」。
DECLARED_ACTIONS = {
    "read",
    "write",
    "delete",
    "admin",
    "get",
    "head",
    "post",
    "put",
    "patch",
}

# 规范化源码指纹：与脚本内复刻逻辑绑定的 Go 函数快照。
# 更新 Go 函数后运行 --print-fingerprints，把新值写回这里，并同步改复刻逻辑。
RESOURCE_FINGERPRINT = "45c53eeebc1e8712fc4ef182130bfbc9f3a0867828130c925eefbadc83244fae"
ACTION_FINGERPRINT = "07b96551e9cb04dd26b4c7957a12c7fdbda6b0ce0519e05e62961548a3920a09"
SPLIT_FINGERPRINT = "7f37465b7836be312b0894e619b1b326d3deb72fa6b3c5b764508301f953a200"

# 结构标记：即使指纹被误更新，缺少分支标记仍判红，提示复刻逻辑已失真。
RESOURCE_MARKERS = (
    'parts[0] == "ai"',
    'parts[1] == "faces"',
    '"aigateway:chat"',
    '"aigateway:" + entity',
    'parts[0] == "global"',
    'strings.ReplaceAll(entity, "-", "_")',
)
ACTION_MARKERS = (
    'case "GET", "HEAD":',
    'alias = "read"',
    'case "POST", "PUT", "PATCH":',
    'alias = "write"',
    'case "DELETE":',
    'alias = "delete"',
)
SPLIT_MARKERS = (
    'strings.Index(trimmed, ":")',
    'trimmed[:idx]',
    'trimmed[idx+1:]',
)

# chi.go 覆盖表：运行时 auth_middleware.go 只在 matchPermissionOverride
# 命中时改用声明权限点，因此门禁的「声明是否真正生效」必须以这份字面量为准。
PERMISSION_OVERRIDE_RE = re.compile(r"auth\.PermissionConfig\s*\{")

FIELD_RE = {
    "method": re.compile(r'\bMethod:\s*"([^"]+)"'),
    "pattern": re.compile(r'\bPattern:\s*"([^"]+)"'),
    "permission": re.compile(r'\bPermission:\s*"([^"]+)"'),
    "surface": re.compile(r"\bSurface:\s*([A-Za-z0-9_.]+)"),
}


@dataclass
class Route:
    method: str
    pattern: str
    permission: str
    surface: str
    file: str
    lines: dict[str, int] = field(default_factory=dict)
    changed: bool = False

    @property
    def key(self) -> tuple[str, str]:
        return (self.method, self.pattern)


@dataclass
class Finding:
    code: str
    message: str
    blocking: bool
    route: Route | None = None


@dataclass
class Evaluation:
    findings: list[Finding]
    known_red_total: int = 0
    known_red_matched: int = 0
    known_red_expired: int = 0


# ── Go 源码指纹 ───────────────────────────────────────────────────────────


def go_function_source(name: str) -> str | None:
    """提取 Go 函数体并做规范化（去行注释、压空白）。"""
    if not os.path.isfile(GO_MODELS):
        return None
    with open(GO_MODELS, encoding="utf-8") as handle:
        text = handle.read()
    anchor = text.find(f"func {name}(")
    if anchor < 0:
        return None
    start = text.find("{", anchor)
    if start < 0:
        return None
    depth = 0
    end = -1
    for idx in range(start, len(text)):
        char = text[idx]
        if char == "{":
            depth += 1
        elif char == "}":
            depth -= 1
            if depth == 0:
                end = idx + 1
                break
    if end < 0:
        return None
    body = text[start:end]
    body = re.sub(r"//[^\n]*", "", body)
    body = re.sub(r"\s+", " ", body).strip()
    return body


def fingerprint(body: str) -> str:
    return hashlib.sha256(body.encode("utf-8")).hexdigest()


# ── 运行时规则复刻 ───────────────────────────────────────────────────────


def derive_resource(path: str) -> str:
    """复刻 auth_rbac_models.go 的 resourceFromPath（L131-L172）。"""
    trimmed = path[len("/api/v1/"):] if path.startswith("/api/v1/") else path
    parts = trimmed.split("/")
    if not parts:
        return path
    if parts[0] == "ai" and len(parts) > 1:
        if parts[1] == "faces":
            return "aigateway:chat"
        entity = parts[1]
        if ":" in entity:
            entity = entity.split(":", 1)[0]
        entity = entity.replace("-", "_")
        return "aigateway:" + entity
    if parts[0] == "global" and len(parts) > 1:
        resource = parts[1]
        if ":" in resource:
            resource = resource.split(":", 1)[0]
        return resource
    resource = parts[0]
    if ":" in resource:
        resource = resource.split(":", 1)[0]
    return resource


def action_candidates(method: str) -> list[str]:
    """复刻 auth_rbac_models.go 的 casbinActionCandidates（L97-L112）。"""
    verb = method.strip().lower()
    alias = ""
    upper = method.strip().upper()
    if upper in ("GET", "HEAD"):
        alias = "read"
    elif upper in ("POST", "PUT", "PATCH"):
        alias = "write"
    elif upper == "DELETE":
        alias = "delete"
    if not alias or alias == verb:
        return [verb]
    return [verb, alias]


def split_declared_permission(permission: str) -> tuple[str, str]:
    """复刻 auth_rbac_models.go 的 splitDeclaredPermission（L121-L127）：首个冒号切分。"""
    trimmed = permission.strip()
    idx = trimmed.find(":")
    if idx >= 0:
        return trimmed[:idx], trimmed[idx + 1:]
    return trimmed, ""


@dataclass
class PermissionOverride:
    """复刻 chi.go 里登记的一条 auth.PermissionConfig 覆盖项。"""

    method: str
    pattern: str
    permission: str


def _extract_string_fields(block: str) -> dict[str, str]:
    """从 Go struct 字面量片段里取 Method/Pattern/Permission 的字符串值。"""
    out: dict[str, str] = {}
    for key, regex in (
        ("method", re.compile(r'\bMethod:\s*"((?:[^"\\]|\\.)*)"')),
        ("pattern", re.compile(r'\bPattern:\s*"((?:[^"\\]|\\.)*)"')),
        ("permission", re.compile(r'\bPermission:\s*"((?:[^"\\]|\\.)*)"')),
    ):
        match = regex.search(block)
        if match:
            out[key] = match.group(1)
    return out


def load_permission_overrides(path: str = GO_WIRE) -> list[PermissionOverride]:
    """读取 chi.go 的权限点覆盖表：运行期只在这张表命中时改用声明。

    覆盖表是数据不是算法——Go 侧增删条目会自动反映到这里，无需回写常量。
    """
    if not os.path.isfile(path):
        return []
    with open(path, encoding="utf-8") as handle:
        text = handle.read()
    out: list[PermissionOverride] = []
    for match in PERMISSION_OVERRIDE_RE.finditer(text):
        start = text.find("{", match.start())
        if start < 0:
            continue
        depth = 0
        end = -1
        in_string = False
        escaped = False
        for idx in range(start, len(text)):
            char = text[idx]
            if in_string:
                if escaped:
                    escaped = False
                elif char == "\\":
                    escaped = True
                elif char == '"':
                    in_string = False
                continue
            if char == '"':
                in_string = True
            elif char == "{":
                depth += 1
            elif char == "}":
                depth -= 1
                if depth == 0:
                    end = idx + 1
                    break
        if end < 0:
            continue
        fields = _extract_string_fields(text[start:end])
        permission = fields.get("permission", "")
        if not permission:
            continue
        out.append(
            PermissionOverride(
                method=fields.get("method", ""),
                pattern=fields.get("pattern", ""),
                permission=permission,
            )
        )
    return out


def override_matches(override: PermissionOverride, method: str, pattern: str) -> bool:
    """复刻 matchPermissionOverride 的匹配语义（method 大小写不敏感，`{param}` 段作通配符）。

    运行期比的是真实请求路径，这里比的是路由声明，所以两侧的参数段都按单段通配处理。
    """
    if override.permission == "":
        return False
    if override.method and override.method.upper() != method.upper():
        return False
    left = override.pattern.split("/")
    right = pattern.split("/")
    if len(left) != len(right):
        return False
    for a, b in zip(left, right):
        if a == b:
            continue
        if a == "*" or _is_param(a) or _is_param(b):
            continue
        return False
    return True


def _is_param(segment: str) -> bool:
    return segment.startswith("{") and segment.endswith("}")


def find_honored_override(
    route: Route, overrides: list[PermissionOverride]
) -> PermissionOverride | None:
    """运行期真正生效的覆盖项：命中路径与方法，且声明串与路由完全一致。

    只有这种情况下运行期才会用 splitDeclaredPermission 拆声明权限点；覆盖表缺这条
    声明时不能假定声明生效，否则门禁会从「检出失配」退化成「放过失配」。
    """
    for override in overrides:
        if not override_matches(override, route.method, route.pattern):
            continue
        if override.permission == route.permission.strip():
            return override
    return None


def parse_declared(
    permission: str, honored: PermissionOverride | None = None
) -> tuple[str, str | None]:
    """拆出声明资源与声明动作。

    honored 非空表示运行期走覆盖表，按 Go 的首个冒号拆（`users:password:reset` →
    资源 `users` + 动作 `password:reset`）；否则用默认口径：无动作词尾时整串视为资源。
    """
    value = permission.strip()
    if honored is not None:
        resource, action = split_declared_permission(value)
        return resource, (action or None)
    if value == "public":
        return value, None
    cut = max(value.rfind(":"), value.rfind("."))
    if cut > 0 and value[cut + 1:] in DECLARED_ACTIONS:
        return value[:cut], value[cut + 1:]
    return value, None


# ── 路由清单解析 ─────────────────────────────────────────────────────────


def parse_routes_file(path: str) -> list[Route]:
    """解析 routes.go 的 route.Definition 字面量。

    Go 侧有两种书写形态：多行 struct 字面量与单行 `{Method: ..., ...}`。
    这里按「记录起始 → 字段行 → 右花括号」跟踪；不依赖 gofmt 缩进。
    """
    with open(path, encoding="utf-8") as handle:
        lines = handle.read().splitlines()
    out: list[Route] = []
    current: dict[str, object] | None = None

    def flush(record: dict[str, object] | None) -> None:
        if record and "pattern" in record:
            out.append(
                Route(
                    method=str(record.get("method", "")),
                    pattern=str(record["pattern"]),
                    permission=str(record.get("permission", "")),
                    surface=str(record.get("surface", "")),
                    file=os.path.relpath(path, REPO),
                    lines=dict(record.get("lines", {})),
                )
            )

    for lineno, raw in enumerate(lines, 1):
        stripped = raw.strip()
        if stripped.startswith("//"):
            continue
        if "{Method:" in raw:
            record: dict[str, object] = {"lines": {}}
            for name, regex in FIELD_RE.items():
                match = regex.search(raw)
                if match:
                    record[name] = match.group(1)
                    record["lines"][name] = lineno
            flush(record)
            current = None
            continue
        if stripped == "{":
            current = {"lines": {}}
            continue
        if current is not None:
            for name, regex in FIELD_RE.items():
                match = regex.search(raw)
                if match:
                    current[name] = match.group(1)
                    current["lines"][name] = lineno  # type: ignore[index]
            if stripped in ("},", "}"):
                flush(current)
                current = None
    return out


def load_routes() -> list[Route]:
    routes: list[Route] = []
    for dirpath, _, filenames in os.walk(ROUTES_DIR):
        for filename in filenames:
            if filename == "routes.go":
                routes.extend(parse_routes_file(os.path.join(dirpath, filename)))
    return routes


def in_scope(route: Route) -> bool:
    if not route.permission or route.permission == "public":
        return False
    if not route.pattern.startswith("/api/v1/"):
        return False
    if route.pattern.startswith("/api/v1/auth/"):
        return False
    if "Protocol" in route.surface:
        return False
    return True


def is_nested(pattern: str) -> bool:
    segments = [seg for seg in pattern.split("/") if seg]
    return len(segments) >= 3 and any(
        seg.startswith("{") for seg in segments[:-1]
    )


# ── git 变更集 ────────────────────────────────────────────────────────────


def git(*args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", *args], cwd=REPO, text=True, capture_output=True, check=False
    )


def resolve_base(explicit: str | None) -> str | None:
    if explicit:
        probe = git("rev-parse", "--verify", f"{explicit}^{{commit}}")
        if probe.returncode != 0:
            print(f"FAIL: --base-ref 无法解析: {explicit}", file=sys.stderr)
            sys.exit(2)
        return explicit
    candidates: list[str] = []
    env_base = os.environ.get("ROUTE_DERIVATION_BASE", "").strip()
    if env_base:
        candidates.append(env_base)
    github_base = os.environ.get("GITHUB_BASE_REF", "").strip()
    if github_base:
        candidates.append(f"origin/{github_base}")
    candidates.extend(["origin/main", "HEAD"])
    for candidate in candidates:
        probe = git("rev-parse", "--verify", f"{candidate}^{{commit}}")
        if probe.returncode == 0:
            merged = git("merge-base", candidate, "HEAD")
            if merged.returncode == 0 and merged.stdout.strip():
                return merged.stdout.strip()
            return candidate
    return None


HUNK_RE = re.compile(r"^@@ -[0-9]+(?:,[0-9]+)? +\+([0-9]+)(?:,([0-9]+))? @@")


def added_line_ranges(base: str) -> dict[str, list[tuple[int, int]]]:
    """返回 文件 → 新增行区间（闭区间），用于判定路由是否属于本次变更集。"""
    diff = git("diff", "--unified=0", "--no-color", base, "--", ROUTE_DIFF_PATH)
    if diff.returncode != 0:
        print(f"WARN: git diff 失败（{diff.stderr.strip()}），本次按全量报告模式运行")
        return {}
    ranges: dict[str, list[tuple[int, int]]] = {}
    current: str | None = None
    for line in diff.stdout.splitlines():
        if line.startswith("+++ b/"):
            current = line[len("+++ b/"):]
            continue
        if line.startswith("+++ ") or line.startswith("--- "):
            if line.startswith("+++ /dev/null"):
                current = None
            continue
        match = HUNK_RE.match(line)
        if match and current:
            start = int(match.group(1))
            count = int(match.group(2)) if match.group(2) is not None else 1
            if count > 0:
                ranges.setdefault(current, []).append((start, start + count - 1))
    return ranges


def mark_changed(routes: list[Route], ranges: dict[str, list[tuple[int, int]]]) -> int:
    changed = 0
    for route in routes:
        spans = ranges.get(route.file, [])
        route.changed = any(
            start <= line <= end
            for line in route.lines.values()
            for start, end in spans
        )
        if route.changed:
            changed += 1
    return changed


# ── 豁免注册表 ────────────────────────────────────────────────────────────


def load_exemptions() -> tuple[list[dict], list[Finding]]:
    findings: list[Finding] = []
    if not os.path.isfile(EXEMPTIONS_FILE):
        return [], findings
    if yaml is None:
        findings.append(
            Finding("EXEMPTION_INVALID", "缺少 PyYAML，无法解析豁免注册表", True)
        )
        return [], findings
    with open(EXEMPTIONS_FILE, encoding="utf-8") as handle:
        data = yaml.safe_load(handle) or {}
    entries = data.get("exemptions", []) or []
    if not isinstance(entries, list):
        findings.append(
            Finding("EXEMPTION_INVALID", "exemptions 必须是列表", True)
        )
        return [], findings
    return entries, findings


def parse_expiry(value: object) -> dt.date | None:
    try:
        return dt.date.fromisoformat(str(value))
    except (ValueError, TypeError):
        return None


# ── known-red 基线（带期限，非豁免）──────────────────────────────────────
#
# 既有失配统一登记在 contracts/registries/known-red.yaml 的
# `route_derivation_known_red` 列表（独立顶层键，不进入 `rules` 的
# baseline_failures 语义，也不会改变 docs/generated/known-red-registry.md）。
# 语义严格区分三种情况：
#   * 命中且未过期 → 不计阻断（计入 PASS），打印 KNOWN-RED 摘要；
#   * 命中但已过期 → 判红（KNOWN_RED_EXPIRED）；
#   * 不在基线内 → 立即判红（原有 R1/R2/NESTED），证明检出能力未被削弱。
# 条目字段非法（缺 gate/pattern/reason/owner/target_batch/expires_on/evidence_ref，
# 或 code 不在 {R1,R2,NESTED,*}) → 判红（KNOWN_RED_INVALID），避免基线静默失效。


def load_known_red() -> tuple[object, list[Finding]]:
    """读取 known-red.yaml 的 `route_derivation_known_red` 列表。"""
    findings: list[Finding] = []
    if not os.path.isfile(KNOWN_RED_FILE):
        return [], findings
    if yaml is None:
        findings.append(
            Finding("KNOWN_RED_INVALID", "缺少 PyYAML，无法解析 known-red 基线", True)
        )
        return [], findings
    with open(KNOWN_RED_FILE, encoding="utf-8") as handle:
        data = yaml.safe_load(handle) or {}
    if not isinstance(data, dict):
        findings.append(
            Finding("KNOWN_RED_INVALID", "known-red.yaml 顶层必须是映射", True)
        )
        return [], findings
    return data.get(KNOWN_RED_KEY, []) or [], findings


def validate_known_red(
    entries: object, today: dt.date
) -> tuple[list[dict], list[Finding]]:
    """校验 known-red 条目结构；返回合法条目（带内部 _expiry）。"""
    findings: list[Finding] = []
    if not isinstance(entries, list):
        findings.append(
            Finding("KNOWN_RED_INVALID", f"{KNOWN_RED_KEY} 必须是列表", True)
        )
        return [], findings
    valid: list[dict] = []
    for entry in entries:
        if not isinstance(entry, dict):
            findings.append(
                Finding("KNOWN_RED_INVALID", f"known-red 条目必须是映射：{entry!r}", True)
            )
            continue
        missing = [
            name for name in KNOWN_RED_REQUIRED if not str(entry.get(name, "")).strip()
        ]
        expiry = parse_expiry(entry.get("expires_on"))
        code = str(entry.get("code", "*") or "*")
        problems: list[str] = []
        if missing:
            problems.append(f"缺少 {missing}")
        if expiry is None:
            problems.append("expires_on 非法（需 YYYY-MM-DD）")
        if entry.get("gate") != KNOWN_RED_GATE:
            problems.append(f"gate={entry.get('gate')!r} 非 {KNOWN_RED_GATE}")
        if code not in KNOWN_RED_CODES and code != "*":
            problems.append(f"code={code!r} 不在 {sorted(KNOWN_RED_CODES)} 或 *")
        if problems:
            findings.append(
                Finding(
                    "KNOWN_RED_INVALID",
                    f"known-red 条目非法（{'；'.join(problems)}）：{entry!r}",
                    True,
                )
            )
            continue
        valid.append({**entry, "_expiry": expiry})
    return valid, findings


def known_red_matches(entry: dict, route: Route, code: str) -> bool:
    method = str(entry.get("method", "*") or "*")
    if method not in ("*", route.method):
        return False
    if entry.get("pattern") != route.pattern:
        return False
    entry_code = str(entry.get("code", "*") or "*")
    return entry_code in ("*", code)


def resolve_known_red(
    known_red: object | None, today: dt.date
) -> tuple[list[dict], list[Finding]]:
    findings: list[Finding] = []
    if known_red is None:
        loaded, load_findings = load_known_red()
        findings.extend(load_findings)
        known_red = loaded
    valid, validate_findings = validate_known_red(known_red, today)
    findings.extend(validate_findings)
    return valid, findings


# ── 判定 ─────────────────────────────────────────────────────────────────


def evaluate(
    routes: list[Route],
    exemptions: list[dict] | None,
    today: dt.date,
    strict: bool,
    known_red: object | None = None,
    overrides: list[PermissionOverride] | None = None,
) -> Evaluation:
    findings: list[Finding] = []
    if overrides is None:
        overrides = load_permission_overrides()
    if exemptions is None:
        exemptions, entry_findings = load_exemptions()
        findings.extend(entry_findings)

    by_key: dict[tuple[str, str], list[dict]] = {}
    for entry in exemptions:
        if not isinstance(entry, dict):
            findings.append(
                Finding(
                    "EXEMPTION_INVALID",
                    f"豁免条目必须是映射（pattern/reason/owner/expires_on）：{entry!r}",
                    True,
                )
            )
            continue
        pattern = entry.get("pattern")
        method = entry.get("method", "*") or "*"
        missing = [
            name
            for name in ("pattern", "reason", "owner", "expires_on")
            if not str(entry.get(name, "")).strip()
        ]
        expiry = parse_expiry(entry.get("expires_on"))
        if missing or not isinstance(pattern, str) or expiry is None:
            findings.append(
                Finding(
                    "EXEMPTION_INVALID",
                    f"豁免条目字段非法（缺少 {missing or '合法 expires_on'}）：{entry!r}",
                    True,
                )
            )
            continue
        if expiry < today:
            findings.append(
                Finding(
                    "EXEMPTION_EXPIRED",
                    f"豁免已过期（{pattern} {method}，expires_on={expiry.isoformat()}）",
                    True,
                )
            )
            continue
        by_key.setdefault((method, pattern), []).append(entry)

    for route in routes:
        derived = derive_resource(route.pattern)
        honored = find_honored_override(route, overrides)
        declared_resource, declared_action = parse_declared(route.permission, honored)
        nested_hit = is_nested(route.pattern) and declared_resource != derived

        # 豁免查询：method 精确匹配或 `*`。
        exempt = bool(
            by_key.get((route.method, route.pattern))
            or by_key.get(("*", route.pattern))
        )
        if nested_hit and exempt:
            findings.append(
                Finding(
                    "EXEMPTION_INVALID",
                    f"嵌套高危路由不可豁免：{route.method} {route.pattern}",
                    True,
                    route,
                )
            )

        r1_ok = declared_resource == derived
        # 覆盖命中时运行期把动作钉死为覆盖自己的词尾（actions = []string{action}），
        # 方法别名不再参与裁决，所以 R2 不适用；否则 users:password:reset 会因
        # ∉ casbinActionCandidates(POST) 恒红，而那正是决策 #3 要的行为。
        r2_ok = honored is not None or (
            declared_action is not None
            and declared_action in action_candidates(route.method)
        )

        for code, ok, detail in (
            (
                "R1",
                r1_ok or nested_hit,  # 嵌套失配由 NESTED 单独报告，避免重复
                f"声明资源 {declared_resource!r} ≠ 运行期资源 {derived!r}",
            ),
            (
                "R2",
                r2_ok,
                f"声明动作 {declared_action!r} ∉ casbinActionCandidates({route.method})",
            ),
        ):
            if ok:
                continue
            blocking = (strict or route.changed) and not (exempt and not nested_hit)
            findings.append(
                Finding(
                    code,
                    f"{route.method} {route.pattern}（{route.file}:{route.lines.get('pattern', 0)}）：{detail}",
                    blocking,
                    route,
                )
            )

        if nested_hit:
            findings.append(
                Finding(
                    "NESTED",
                    (
                        f"{route.method} {route.pattern}"
                        f"（{route.file}:{route.lines.get('pattern', 0)}）："
                        f"嵌套路径声明资源 {declared_resource!r} ≠ 运行期资源 {derived!r}，"
                        "运行期按路径首段 + 方法别名放行（未在 chi.go 覆盖表钉选），不可豁免"
                    ),
                    True,
                    route,
                )
            )

    # 未命中的豁免条目：仅提示，避免注册表变成永久债务。
    used: set[tuple[str, str]] = set()
    for route in routes:
        used.add((route.method, route.pattern))
        used.add(("*", route.pattern))
    for (method, pattern) in by_key:
        if (method, pattern) in used:
            continue
        findings.append(
            Finding(
                "EXEMPTION_UNUSED",
                f"豁免条目未命中任何当前路由：{method} {pattern}",
                False,
            )
        )

    # ── known-red 基线抑制 ────────────────────────────────────────────────
    known, known_findings = resolve_known_red(known_red, today)
    findings.extend(known_findings)
    matched: set[int] = set()
    expired: set[int] = set()
    expired_emitted: set[tuple[int, str, str]] = set()
    kept: list[Finding] = []
    for finding in findings:
        entry_index = None
        if (
            finding.route is not None
            and finding.code in KNOWN_RED_CODES
            and known
        ):
            for index, entry in enumerate(known):
                if known_red_matches(entry, finding.route, finding.code):
                    entry_index = index
                    break
        if entry_index is None:
            kept.append(finding)
            continue
        matched.add(entry_index)
        entry = known[entry_index]
        if entry["_expiry"] >= today:
            # 未过期：阻断项计入 PASS（不打印逐条失配、也不计入 WARN）；
            # 原本就非阻断的提示保留。
            if finding.blocking:
                continue
            kept.append(finding)
            continue
        # 已过期：无论原 finding 是否阻断，到期即判红（每个条目每条路由一次）。
        expired.add(entry_index)
        marker_key = (entry_index, finding.route.method, finding.route.pattern)
        if marker_key not in expired_emitted:
            expired_emitted.add(marker_key)
            kept.append(
                Finding(
                    "KNOWN_RED_EXPIRED",
                    (
                        f"{finding.route.method} {finding.route.pattern}"
                        f"（{finding.route.file}:"
                        f"{finding.route.lines.get('pattern', 0)}）："
                        f"known-red 已过期（expires_on={entry['_expiry'].isoformat()}，"
                        f"target_batch={entry.get('target_batch')}），"
                        "到期即恢复阻断；请修复或重新排期"
                    ),
                    True,
                    finding.route,
                )
            )

    # 未命中任何当前失配的 known-red 条目：已过期 → 判红；未过期 → 提示清理。
    for index, entry in enumerate(known):
        if index in matched:
            continue
        if entry["_expiry"] < today:
            expired.add(index)
            kept.append(
                Finding(
                    "KNOWN_RED_EXPIRED",
                    (
                        f"known-red 条目已过期且不再命中任何失配"
                        f"（{entry.get('method', '*')} {entry.get('pattern')}，"
                        f"expires_on={entry['_expiry'].isoformat()}）："
                        "到期即恢复阻断；请修复或重新排期"
                    ),
                    True,
                )
            )
            continue
        kept.append(
            Finding(
                "KNOWN_RED_STALE",
                (
                    f"known-red 条目未命中当前失配（{entry.get('method', '*')} "
                    f"{entry.get('pattern')}，target_batch={entry.get('target_batch')}）："
                    "对应失配已修复或路由已删除，请从基线移除"
                ),
                False,
            )
        )

    return Evaluation(
        findings=kept,
        known_red_total=len(known),
        known_red_matched=len(matched - expired),
        known_red_expired=len(expired),
    )


# ── 自测 ─────────────────────────────────────────────────────────────────


def self_test(today: dt.date) -> int:
    """造 3 个反例 + 正例，验证判定与豁免语义，不依赖仓库当前路由。"""

    def build(
        method: str, pattern: str, permission: str, changed: bool = True
    ) -> Route:
        return Route(
            method=method,
            pattern=pattern,
            permission=permission,
            surface="route.SurfaceManagement",
            file="self-test/routes.go",
            lines={"pattern": 1, "method": 1, "permission": 1},
            changed=changed,
        )

    cases: list[tuple[str, list[Route], list[dict], list[dict] | None, int]] = []

    # 反例 1：嵌套失配（users/{id}/orders 声明 orders:read）→ NESTED 阻断。
    cases.append(
        (
            "反例1 嵌套失配（不在基线内）",
            [build("GET", "/api/v1/users/{id}/orders", "orders:read")],
            [],
            [],
            1,
        )
    )

    # 反例 2：动作不可达（GET 声明 :delete）→ R2 阻断。
    cases.append(
        (
            "反例2 动作不可达",
            [build("GET", "/api/v1/widgets", "widgets:delete")],
            [],
            [],
            1,
        )
    )

    # 反例 3：豁免过期 → EXEMPTION_EXPIRED 阻断。
    expired = {
        "pattern": "/api/v1/widgets",
        "method": "GET",
        "reason": "self-test",
        "owner": "self-test",
        "expires_on": (today - dt.timedelta(days=1)).isoformat(),
    }
    cases.append(
        (
            "反例3 豁免过期",
            [build("GET", "/api/v1/widgets", "gadgets:read", changed=False)],
            [expired],
            [],
            1,
        )
    )

    # 正例：ai 特判、faces、global、普通首段均不应产生任何 finding。
    positives = [
        build("GET", "/api/v1/ai/providers", "aigateway:providers:read"),
        build("GET", "/api/v1/ai/faces/openai/chat", "aigateway:chat:read"),
        build("GET", "/api/v1/global/certs", "certs:read"),
        build("GET", "/api/v1/orders", "orders:read"),
        build("POST", "/api/v1/organizations/{orgId}/domains", "organizations:write"),
    ]
    cases.append(("正例 ai/global/普通/一致嵌套", positives, [], [], 0))

    # 正例 2：合法未过期豁免把 R1/R2 失配降为非阻断。
    future = dict(expired, expires_on=(today + dt.timedelta(days=30)).isoformat())
    exempted = [build("GET", "/api/v1/widgets", "gadgets:read")]
    cases.append(("正例 未过期豁免", exempted, [future], [], 0))

    def known_entry(
        pattern: str, code: str, expires_on: str, method: str = "GET"
    ) -> dict:
        return {
            "gate": KNOWN_RED_GATE,
            "code": code,
            "method": method,
            "pattern": pattern,
            "file": "self-test/routes.go",
            "line": 1,
            "reason": "self-test known-red",
            "owner": "self-test",
            "target_batch": "Y",
            "expires_on": expires_on,
            "evidence_ref": "self-test",
        }

    valid_expiry = (today + dt.timedelta(days=30)).isoformat()

    # 反例 5：known-red 只覆盖旧路由；新增嵌套失配不在基线内 → 必须立即红。
    # 这是「检出能力不被基线掩盖」的核心证明。
    cases.append(
        (
            "反例5 基线外新增嵌套失配",
            [
                build("GET", "/api/v1/users/{id}/orders", "orders:read"),
                build("GET", "/api/v1/users/{id}/invoices", "invoices:read"),
            ],
            [],
            [known_entry("/api/v1/users/{id}/orders", "NESTED", valid_expiry)],
            1,
        )
    )

    # 正例 3：命中未过期 known-red → 计入 PASS，不产生阻断。
    suppressed_routes = [build("GET", "/api/v1/users/{id}/orders", "orders:read")]
    suppressed_known = [
        known_entry("/api/v1/users/{id}/orders", "NESTED", valid_expiry)
    ]
    cases.append(("正例 未过期 known-red 抑制", suppressed_routes, [], suppressed_known, 0))

    # 反例 6：known-red 到期（expires_on=昨天）→ 必须恢复阻断。
    expired_known = [
        known_entry(
            "/api/v1/users/{id}/orders",
            "NESTED",
            (today - dt.timedelta(days=1)).isoformat(),
        )
    ]
    cases.append(
        ("反例6 known-red 到期", suppressed_routes, [], expired_known, 1)
    )

    # 反例 7：known-red 字段缺失（缺 evidence_ref）→ KNOWN_RED_INVALID 阻断。
    broken_known = [
        {k: v for k, v in known_entry(
            "/api/v1/users/{id}/orders", "NESTED", valid_expiry
        ).items() if k != "evidence_ref"}
    ]
    cases.append(
        ("反例7 known-red 条目非法", suppressed_routes, [], broken_known, 2)
    )

    failed = False

    def run(
        name: str,
        routes: list[Route],
        exemptions: list[dict],
        known_red: list[dict],
        expected_blocking: int,
        overrides: list[PermissionOverride],
    ) -> None:
        nonlocal failed
        result = evaluate(
            routes,
            exemptions,
            today,
            strict=False,
            known_red=known_red,
            overrides=overrides,
        )
        blocking = [f for f in result.findings if f.blocking]
        ok = len(blocking) == expected_blocking
        codes = sorted({f.code for f in blocking})
        status = "ok" if ok else "FAILED"
        print(
            f"  [{status}] {name}: 阻断 {len(blocking)}（期望 {expected_blocking}），"
            f"codes={codes or 'none'}"
        )
        if not ok:
            failed = True
            for finding in result.findings:
                print(f"        {finding.code} blocking={finding.blocking} {finding.message}")

    # 覆盖表在自测里固定注入：结果不能依赖 chi.go 今天的登记内容。
    password_reset_override = [
        PermissionOverride(
            "POST", "/api/v1/users/{id}/set-password", "users:password:reset"
        )
    ]
    for name, routes, exemptions, known_red, expected_blocking in cases:
        run(name, routes, exemptions, known_red, expected_blocking, password_reset_override)

    # 正例 4：三段式权限点已登记在覆盖表 → 运行期按声明授权，必须零阻断。
    # 这是决策 #3（口令重置独立权限点）在门禁侧的唯一证明。
    run(
        "正例 覆盖表钉选三段式权限点",
        [build("POST", "/api/v1/users/{id}/set-password", "users:password:reset")],
        [],
        [],
        0,
        password_reset_override,
    )

    # 反例 8：同一声明但未登记覆盖表 → 运行期走默认派生，必须判红（不可静默放行）。
    run(
        "反例8 覆盖表缺失同一条声明",
        [build("POST", "/api/v1/users/{id}/set-password", "users:password:reset", changed=False)],
        [],
        [],
        1,
        [],
    )

    # 反例 9：覆盖表登记了不同的权限点 → 声明并未生效，仍按默认口径判红。
    run(
        "反例9 覆盖表与声明不一致",
        [build("POST", "/api/v1/users/{id}/set-password", "users:password:reset", changed=False)],
        [],
        [],
        1,
        [
            PermissionOverride(
                "POST", "/api/v1/users/{id}/set-password", "users:write"
            )
        ],
    )

    # 同源自检：真实 Go 函数必须命中登记指纹；故意错配必须判红。
    resource_body = go_function_source("resourceFromPath")
    action_body = go_function_source("casbinActionCandidates")
    split_body = go_function_source("splitDeclaredPermission")
    if resource_body is None or action_body is None or split_body is None:
        print("  [FAILED] 同源自检: 无法提取 Go 函数，指纹机制失效")
        failed = True
    else:
        synced = [
            finding
            for finding in fingerprint_findings(
                resource_body,
                action_body,
                split_body,
                RESOURCE_FINGERPRINT,
                ACTION_FINGERPRINT,
                SPLIT_FINGERPRINT,
            )
            if finding.blocking
        ]
        drifted = [
            finding
            for finding in fingerprint_findings(
                resource_body,
                action_body,
                split_body,
                "0" * 64,
                "0" * 64,
                "0" * 64,
            )
            if finding.blocking
        ]
        ok = not synced and bool(drifted)
        print(
            f"  [{'ok' if ok else 'FAILED'}] 反例4 指纹漂移 / 正例 当前指纹同步: "
            f"同步阻断 {len(synced)}（期望 0），错配阻断 {len(drifted)}（期望 ≥1）"
        )
        if not ok:
            failed = True
            for finding in synced + drifted:
                print(f"        {finding.code} {finding.message}")

    if failed:
        print("SELF-TEST FAIL")
        return 1
    print("SELF-TEST PASS")
    return 0


# ── 主流程 ───────────────────────────────────────────────────────────────


def fingerprint_findings(
    resource_body: str,
    action_body: str,
    split_body: str,
    expected_resource: str,
    expected_action: str,
    expected_split: str,
) -> list[Finding]:
    """比对 Go 函数规范化源码与脚本登记值，并校验结构标记。"""
    findings: list[Finding] = []
    resource_fp = fingerprint(resource_body)
    action_fp = fingerprint(action_body)
    split_fp = fingerprint(split_body)
    if resource_fp != expected_resource:
        findings.append(
            Finding(
                "SYNC",
                (
                    "resourceFromPath 源码已变化，脚本复刻未同步："
                    f"实测指纹 {resource_fp}，脚本登记 {expected_resource}。"
                    "更新 scripts/check-route-resource-derivation.py 的 derive_resource "
                    "后运行 --print-fingerprints 回写常量"
                ),
                True,
            )
        )
    if action_fp != expected_action:
        findings.append(
            Finding(
                "SYNC",
                (
                    "casbinActionCandidates 源码已变化，脚本复刻未同步："
                    f"实测指纹 {action_fp}，脚本登记 {expected_action}。"
                    "同步 action_candidates 后运行 --print-fingerprints 回写常量"
                ),
                True,
            )
        )
    if split_fp != expected_split:
        findings.append(
            Finding(
                "SYNC",
                (
                    "splitDeclaredPermission 源码已变化，脚本复刻未同步："
                    f"实测指纹 {split_fp}，脚本登记 {expected_split}。"
                    "同步 split_declared_permission 后运行 --print-fingerprints 回写常量"
                ),
                True,
            )
        )
    for marker in RESOURCE_MARKERS:
        if marker not in resource_body:
            findings.append(
                Finding("SYNC", f"resourceFromPath 缺失结构标记 {marker!r}", True)
            )
    for marker in ACTION_MARKERS:
        if marker not in action_body:
            findings.append(
                Finding("SYNC", f"casbinActionCandidates 缺失结构标记 {marker!r}", True)
            )
    for marker in SPLIT_MARKERS:
        if marker not in split_body:
            findings.append(
                Finding(
                    "SYNC", f"splitDeclaredPermission 缺失结构标记 {marker!r}", True
                )
            )
    return findings


def check_fingerprints() -> list[Finding]:
    findings: list[Finding] = []
    resource_body = go_function_source("resourceFromPath")
    action_body = go_function_source("casbinActionCandidates")
    split_body = go_function_source("splitDeclaredPermission")
    if resource_body is None or action_body is None or split_body is None:
        findings.append(
            Finding(
                "SYNC",
                f"无法在 {os.path.relpath(GO_MODELS, REPO)} 定位 resourceFromPath / "
                "casbinActionCandidates / splitDeclaredPermission，复刻逻辑失去同源源码",
                True,
            )
        )
        return findings
    findings.extend(
        fingerprint_findings(
            resource_body,
            action_body,
            split_body,
            RESOURCE_FINGERPRINT,
            ACTION_FINGERPRINT,
            SPLIT_FINGERPRINT,
        )
    )
    return findings


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description="路由资源派生一致性门禁")
    parser.add_argument("--strict", action="store_true", help="全部失配阻断")
    parser.add_argument("--base-ref", default=None, help="变更集基准（默认自动解析）")
    parser.add_argument("--self-test", action="store_true", help="运行门禁自测")
    parser.add_argument(
        "--print-fingerprints", action="store_true", help="打印 Go 函数规范化指纹"
    )
    parser.add_argument(
        "--today", default=None, help="覆盖判定日期（YYYY-MM-DD，供自测/复现）"
    )
    args = parser.parse_args(argv)

    today = dt.date.today()
    if args.today:
        try:
            today = dt.date.fromisoformat(args.today)
        except ValueError:
            print(f"FAIL: --today 非法日期: {args.today}", file=sys.stderr)
            return 2

    if args.print_fingerprints:
        bodies = {
            "RESOURCE": go_function_source("resourceFromPath"),
            "ACTION": go_function_source("casbinActionCandidates"),
            "SPLIT": go_function_source("splitDeclaredPermission"),
        }
        missing = [name for name, body in bodies.items() if body is None]
        if missing:
            print(
                f"FAIL: 无法提取 Go 函数 {missing}，检查 auth_rbac_models.go",
                file=sys.stderr,
            )
            return 2
        print(f"RESOURCE_FINGERPRINT = {fingerprint(bodies['RESOURCE'])}")
        print(f"ACTION_FINGERPRINT = {fingerprint(bodies['ACTION'])}")
        print(f"SPLIT_FINGERPRINT = {fingerprint(bodies['SPLIT'])}")
        return 0

    if args.self_test:
        return self_test(today)

    routes = load_routes()
    scoped = [route for route in routes if in_scope(route)]
    base = resolve_base(args.base_ref)
    ranges = added_line_ranges(base) if base else {}
    changed = mark_changed(scoped, ranges)
    overrides = load_permission_overrides()

    evaluation = evaluate(scoped, None, today, args.strict, overrides=overrides)
    findings = check_fingerprints() + evaluation.findings

    blocking = [f for f in findings if f.blocking]
    warnings = [f for f in findings if not f.blocking]

    print(
        f"路由清单：解析 {len(routes)} 条，纳入判定 {len(scoped)} 条，"
        f"排除 {len(routes) - len(scoped)} 条（public/无声明/auth 绕过/协议面/非 /api/v1）"
    )
    print(
        f"权限点覆盖表：chi.go 登记 {len(overrides)} 条"
        "（命中且声明一致的路由按声明的首个冒号拆资源/动作，其余走默认派生）"
    )
    if base:
        print(
            f"变更集基准：{base}（新增行命中的路由 {changed} 条；"
            f"warn 阶段仅这些路由的 R1/R2 阻断）"
        )
    else:
        print("变更集基准：无法解析（仅 NESTED 与豁免问题阻断，其余全量报告）")

    for finding in blocking:
        print(f"FAIL [{finding.code}] {finding.message}")
    for finding in warnings:
        print(f"WARN [{finding.code}] {finding.message}")

    print(
        f"KNOWN-RED {evaluation.known_red_matched}/{evaluation.known_red_total}"
        f"（已过期 {evaluation.known_red_expired} 条；"
        f"target_batch=Y，基线见 contracts/registries/known-red.yaml）"
    )
    print(f"WARN {len(warnings)}")

    by_code: dict[str, int] = {}
    for finding in blocking:
        by_code[finding.code] = by_code.get(finding.code, 0) + 1
    summary = ", ".join(f"{code}={count}" for code, count in sorted(by_code.items()))
    if blocking:
        print(
            f"FAIL: 阻断 {len(blocking)} 项（{summary}）；"
            f"仅报告 {len(warnings)} 项。豁免注册表："
            "contracts/registries/route-resource-exemptions.yaml（NESTED 不可豁免）"
        )
        print(f"FAIL_COUNT={len(blocking)}")
        return 1

    print(
        f"PASS: 无阻断项（仅报告 {len(warnings)} 项；"
        f"纳入判定 {len(scoped)} 条，变更集命中 {changed} 条；"
        f"known-red {evaluation.known_red_matched}/{evaluation.known_red_total} 计入 PASS）"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
