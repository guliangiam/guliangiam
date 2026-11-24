#!/usr/bin/env python3
"""Check that the query parameters the web app sends are ones the API accepts.

Why this exists
---------------
The transport layer reads query parameters by literal name
(`r.URL.Query().Get("keyword")`, `route.QueryInt(r, "pageSize", …)`) while the
web app builds them by literal name too (`query.set("keyword", …)`). Nothing
connected the two, so a rename on one side turned into a silent no-op on the
other: the list pages sent `search` and `sortBy` for months while every handler
read `keyword` and `sortField`, which meant every search box and column sort in
the console did nothing at all and reported no error.

This gate collects the accepted parameter names from the transport layer and the
sent parameter names from the web API layer, and fails when the web sends a name
the API never reads. It is intentionally one-directional: an API that accepts a
parameter the console does not use is fine.

Known false-positive sources are handled explicitly:
  * `filters` on the web side forwards caller-chosen keys, so it is skipped;
  * parameters built with a variable name (``query.set(key, …)``) are skipped.

Usage: python3 scripts/check-api-params.py
Exit code 0 = every sent parameter is accepted, 1 = drift found.

输入：web/src/ API 调用、internal/transport/http/route/ 读参
输出：PASS/FAIL + 参数名漂移列表
--check 行为：无 --check；失败即 exit 1
消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
"""

import os
import re
import sys

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
TRANSPORT_DIR = os.path.join(REPO_ROOT, "internal/transport/http")
WEB_API_DIR = os.path.join(REPO_ROOT, "web/src/shared/api")
# Rules 4-5 read values, and values are written by pages, not only by the API
# layer. Scanning WEB_API_DIR alone is exactly how the `created_at` defect got
# through: the literal lived in web/src/pages/users/UsersPage.tsx.
WEB_SRC_DIR = os.path.join(REPO_ROOT, "web/src")

# Parameters the API reads. Both the direct accessor and the typed helpers are
# covered, because the codebase uses both.
#
# The list must match the helpers that actually exist in
# `internal/transport/http/route/query.go`: `QueryBool`/`QueryString` used to be
# listed here and were never written, while `QueryOptionalString` — which *is*
# used (`products` reads `state` through it) — was missing. Phantom patterns
# read like coverage while projecting none, and a missing one means the gate has
# no idea a parameter is read at all, so either direction produces false
# confidence.
BACKEND_PATTERNS = [
    re.compile(r'\.Query\(\)\.Get\(\s*"([^"]+)"\s*\)'),
    re.compile(r'\.Query\(\)\[\s*"([^"]+)"\s*\]'),
    re.compile(r'route\.QueryInt\(\s*r\s*,\s*"([^"]+)"'),
    re.compile(r'route\.QueryOptionalBool\(\s*r\s*,\s*"([^"]+)"'),
    re.compile(r'route\.QueryOptionalString\(\s*r\s*,\s*"([^"]+)"'),
]

# A handler that aliases the query map (`q := r.URL.Query()`) then reads
# parameters as `q.Get("action")` is invisible to the patterns above, so every
# parameter it reads looks unread. In `records/chi.go` that made the gate report
# a parameter the handler does read — a false positive that hides the real
# failure mode just as effectively, since a genuine drift in an aliased handler
# would look like the same noise and be waved through.
QUERY_ALIAS = re.compile(r"([A-Za-z_]\w*)\s*(?::=|=)\s*r\.URL\.Query\(\)")

# Parameters the web app sets, by literal name.
WEB_PATTERNS = [
    re.compile(r'query\.set\(\s*"([^"]+)"'),
    re.compile(r'query\.append\(\s*"([^"]+)"'),
    re.compile(r'parts\.push\(`([A-Za-z][A-Za-z0-9_]*)='),
]

# `new URLSearchParams({ a: …, b: … })` — every literal key counts, not just the
# first one, so the whole object literal is scanned for `key:` pairs.
WEB_OBJECT_LITERAL = re.compile(r"new URLSearchParams\(\{(.*?)\}\)", re.DOTALL)
OBJECT_KEY = re.compile(r'(?:^|[,{\s])([A-Za-z][A-Za-z0-9_]*)\s*:', re.MULTILINE)

# Names that are not query parameters at all, or that the web layer forwards
# from its caller without knowing the key.
IGNORED_SENT = {
    "filters",
}

# --- Rule 2: the OpenAPI contract's parameter names must be readable ---------
#
# Rule 1 above only compares the web app against the handlers. The spec is a third
# list, and a name that appears there but nowhere in the transport layer documents
# a parameter that does nothing. That is not hypothetical: 61 operations declared
# `sortBy` while every handler read `sortField`, so a client that followed the
# document sent a parameter the server ignored — silently, because an unknown query
# parameter is not an error.
OPENAPI_FILE = os.path.join(REPO_ROOT, "openapi/openapi.yaml")
PARAM_COMPONENT = re.compile(r"^( {4})([A-Za-z][A-Za-z0-9_]*):\s*$", re.MULTILINE)
PARAM_NAME = re.compile(r"^\s+name:\s*([A-Za-z][A-Za-z0-9_]*)\s*$", re.MULTILINE)
PARAM_LOCATION = re.compile(r"^\s+in:\s*(\w+)\s*$", re.MULTILINE)

# Parameters the contract declares that no handler reads on purpose: they belong
# to a face whose handlers are generated or to a standard that fixes the name.
IGNORED_DECLARED: dict[str, str] = {}

# --- Rule 3: no pagination floor in the repositories ------------------------
#
# `max(x.PageSize, N)` reads like a cap but is a floor: pageSize=5 becomes N rows
# while the response echoes 5, and the offset (which used a different N) makes
# adjacent pages overlap. The bound belongs in platform/pagination, applied once.
INFRA_DIR = os.path.join(REPO_ROOT, "internal/infra/postgres")
# The receiver may be a struct field (`query.PageSize`) or a local variable
# (`max(pageSize, 20)`), so the dot is optional. Requiring it let one
# repository keep the floor while the gate reported success.
PAGE_SIZE_FLOOR = re.compile(r"max\(\s*[\w.]*[Pp]ageSize\s*,")

# A locally-defined function that is handed `PageSize` is a private
# re-implementation of a bound that `shared.ClampPageSize`/`shared.PageOffset`
# own. `mx(v, f)` was the instance: fifteen identical copies across the
# repositories, none of which clamped, and the `max(` rule above could not see
# them **because the name was not `max`**. Naming was the entire hiding place.
LOCAL_FUNC_DECL = re.compile(r"(?m)^func\s+(?:\([^)]*\)\s*)?([A-Za-z_]\w*)\s*\(")
# `shared/` is where the canonical bound lives, so its own helpers are exempt.
CANONICAL_DIR = os.path.join(INFRA_DIR, "shared")


def find_local_pagination_helpers():
    """Return hits where a package-local function receives a page size."""
    hits = []
    packages = {}
    for path in go_files(INFRA_DIR):
        if path.startswith(CANONICAL_DIR + os.sep):
            continue
        packages.setdefault(os.path.dirname(path), []).append(path)

    for _package, paths in sorted(packages.items()):
        local = set()
        for path in paths:
            with open(path, encoding="utf-8") as handle:
                source = strip_go_comments(handle.read())
            local.update(LOCAL_FUNC_DECL.findall(source))
        if not local:
            continue
        for path in paths:
            with open(path, encoding="utf-8") as handle:
                source = strip_go_comments(handle.read())
            relpath = os.path.relpath(path, REPO_ROOT)
            # A declaration names a page size too (`func (r R) List(…, pageSize int)`),
            # and the character before the name there is a space, not a dot, so the
            # bare-call pattern matches it. Record the declarations and skip them.
            declarations = {m.start(1) for m in LOCAL_FUNC_DECL.finditer(source)}
            for name in sorted(local):
                # A bare call only: `pkg.Name(...)` is somebody else's helper.
                call = re.compile(r"(?<![\w.])" + re.escape(name) + r"\s*\(")
                for match in call.finditer(source):
                    if match.start() in declarations:
                        continue
                    open_paren = source.index("(", match.start())
                    arguments = balanced_arguments(source, open_paren)
                    # 回调字面量体内的 pageSize 属于闭包内的局部变量（仓储先
                    # ClampPageSize 再传进事务函数），不是「传给该本地函数的实参」。
                    # 只看第一个 func 字面量之前的实参列表，避免把包内事务/执行器
                    # 误判成分页回落的第二实现。
                    callback_at = re.search(r"\bfunc\s*\(", arguments)
                    if callback_at:
                        arguments = arguments[: callback_at.start()]
                    if not re.search(r"[Pp]ageSize", arguments):
                        continue
                    line = source[: match.start()].count("\n") + 1
                    hits.append(
                        f"{relpath}:{line}: {name}(…) receives a page size — "
                        "use shared.ClampPageSize / shared.PageOffset"
                    )
    return hits


def balanced_arguments(source, open_paren):
    """Return the text between a call's parentheses."""
    depth = 0
    i = open_paren
    n = len(source)
    while i < n:
        c = source[i]
        if c in "\"'`":
            quote = c
            i += 1
            while i < n:
                if source[i] == "\\":
                    i += 2
                    continue
                if source[i] == quote:
                    i += 1
                    break
                i += 1
            continue
        if c == "(":
            depth += 1
        elif c == ")":
            depth -= 1
            if depth == 0:
                return source[open_paren + 1 : i]
        i += 1
    return ""


def go_files(root):
    for dirpath, _dirs, files in os.walk(root):
        for name in sorted(files):
            if name.endswith(".go") and not name.endswith("_test.go"):
                yield os.path.join(dirpath, name)


def ts_files(root):
    for dirpath, _dirs, files in os.walk(root):
        for name in sorted(files):
            if name.endswith(".ts") and not name.endswith(".test.ts"):
                yield os.path.join(dirpath, name)


def web_source_files(root):
    """Every non-test TypeScript module under web/src.

    Sorted so the report order is stable across runs.
    """
    for dirpath, _dirs, files in os.walk(root):
        for name in sorted(files):
            if not (name.endswith(".ts") or name.endswith(".tsx")):
                continue
            if ".test." in name or ".spec." in name:
                continue
            yield os.path.join(dirpath, name)


def collect_backend_params():
    accepted = set()
    for path in go_files(TRANSPORT_DIR):
        with open(path, encoding="utf-8") as handle:
            source = handle.read()
        for pattern in BACKEND_PATTERNS:
            accepted.update(pattern.findall(source))
        for alias in QUERY_ALIAS.findall(source):
            alias_get = re.compile(r"\b" + re.escape(alias) + r'\.Get\(\s*"([^"]+)"\s*\)')
            accepted.update(alias_get.findall(source))
    return accepted


def collect_web_params():
    sent = {}
    for path in ts_files(WEB_API_DIR):
        with open(path, encoding="utf-8") as handle:
            source = handle.read()
        names = []
        for pattern in WEB_PATTERNS:
            names.extend(pattern.findall(source))
        for literal in WEB_OBJECT_LITERAL.findall(source):
            names.extend(OBJECT_KEY.findall(literal))
        for name in names:
            if name in IGNORED_SENT:
                continue
            sent.setdefault(name, os.path.relpath(path, REPO_ROOT))
    return sent


def collect_declared_query_params():
    """Return {component name: parameter name} for query parameters the spec declares."""
    with open(OPENAPI_FILE, encoding="utf-8") as handle:
        source = handle.read()

    parameters_at = source.find("\n  parameters:\n")
    if parameters_at < 0:
        raise SystemExit("ERROR: openapi.yaml has no components.parameters section")

    # The parameters block runs until the next top-level component section.
    rest = source[parameters_at + len("\n  parameters:\n"):]
    end = rest.find("\n  " + "schemas:")
    block = rest if end < 0 else rest[:end]

    declared = {}
    for match in PARAM_COMPONENT.finditer(block):
        component = match.group(2)
        chunk = block[match.end():]
        next_component = PARAM_COMPONENT.search(chunk)
        body = chunk if next_component is None else chunk[: next_component.start()]
        location = PARAM_LOCATION.search(body)
        name = PARAM_NAME.search(body)
        if not name or not location or location.group(1) != "query":
            continue
        declared[component] = name.group(1)
    return declared


def strip_go_comments(source):
    """Blank out Go comments and string literals.

    A checker that greps raw source matches its own documentation: the comment
    explaining why `max(query.PageSize, 20)` is wrong is itself a
    `max(<x>.PageSize,` match. String literals are blanked for the same reason —
    a SQL fragment or an error message may mention the pattern without being it.
    Newlines are preserved so reported line numbers stay correct.
    """
    out = []
    i, n = 0, len(source)
    while i < n:
        c = source[i]
        if source.startswith("//", i):
            j = source.find("\n", i)
            j = n if j < 0 else j
            out.append(" " * (j - i))
            i = j
            continue
        if source.startswith("/*", i):
            j = source.find("*/", i + 2)
            j = n if j < 0 else j + 2
            out.append("".join("\n" if ch == "\n" else " " for ch in source[i:j]))
            i = j
            continue
        if c == "`":
            j = source.find("`", i + 1)
            j = n if j < 0 else j + 1
            out.append("".join("\n" if ch == "\n" else " " for ch in source[i:j]))
            i = j
            continue
        if c == '"':
            j = i + 1
            while j < n:
                if source[j] == "\\":
                    j += 2
                    continue
                if source[j] == '"':
                    j += 1
                    break
                if source[j] == "\n":
                    break
                j += 1
            out.append("".join("\n" if ch == "\n" else " " for ch in source[i:j]))
            i = j
            continue
        out.append(c)
        i += 1
    return "".join(out)


def find_page_size_floors():
    hits = []
    for path in go_files(INFRA_DIR):
        with open(path, encoding="utf-8") as handle:
            source = strip_go_comments(handle.read())
        for match in PAGE_SIZE_FLOOR.finditer(source):
            line = source[: match.start()].count("\n") + 1
            hits.append(f"{os.path.relpath(path, REPO_ROOT)}:{line}")
    return hits


# --- Rule 4: the *value* of a sort parameter must be a backend-legal key -----
#
# Rules 1-2 compare parameter names, which is why they could not see this defect:
# the web app sent `sortField` (the right name) with the value `created_at`,
# while platformsorting keys its whitelist by API camelCase (`createdAt`). An
# unknown key is not ignored — Spec.Resolve answers 422, so the whole list
# request fails, not just the ordering. The web app's own tests mock the HTTP
# layer, so a wrong value passes every unit test.
#
# The whitelist is read from `platformsorting.NewSpec(map[string]string{...})`
# literals in the repositories; the sent values are read from the web app.
SORT_SPEC_CALL = re.compile(
    r"platformsorting\.NewSpec\(\s*map\[string\]string\{(.*?)\}\s*\)", re.DOTALL
)
SORT_SPEC_KEY = re.compile(r'"([A-Za-z][A-Za-z0-9_]*)"\s*:')

# `sortField: "createdAt"` / `sortBy: "createdAt"` object properties, and
# `const DEFAULT_SORT_FIELD = "createdAt"` constants. The constant form matters:
# the fix for the defect above moved the literal behind a named constant, and a
# pattern that only matched object properties would have stopped seeing it.
WEB_SORT_LITERAL = re.compile(r'\b(?:sortField|sortBy)\s*:\s*"([^"]*)"')
WEB_SORT_CONSTANT = re.compile(r'\b[A-Z0-9_]*SORT_(?:FIELD|BY)\s*=\s*"([^"]*)"')


def strip_ts_comments(source):
    """Remove `//` and `/* */` comments, preserving string literals.

    Comments must go: a comment explaining the `created_at` defect is itself a
    match. Strings must stay: they are the values under test. Single chars are
    skipped so `"http://x"` is not mistaken for a line comment.
    """
    out = []
    i, n = 0, len(source)
    while i < n:
        c = source[i]
        if c in "\"'`":
            quote = c
            j = i + 1
            while j < n:
                if source[j] == "\\":
                    j += 2
                    continue
                if source[j] == quote:
                    j += 1
                    break
                j += 1
            out.append(source[i:j])
            i = j
            continue
        if source.startswith("//", i):
            j = source.find("\n", i)
            j = n if j < 0 else j
            i = j
            continue
        if source.startswith("/*", i):
            j = source.find("*/", i + 2)
            j = n if j < 0 else j + 2
            out.append("".join("\n" if ch == "\n" else " " for ch in source[i:j]))
            i = j
            continue
        out.append(c)
        i += 1
    return "".join(out)


def collect_sort_whitelist():
    allowed = set()
    for path in go_files(INFRA_DIR):
        with open(path, encoding="utf-8") as handle:
            source = handle.read()
        for body in SORT_SPEC_CALL.findall(source):
            allowed.update(SORT_SPEC_KEY.findall(body))
    return allowed


def find_bad_sort_values(allowed):
    hits = []
    for path in web_source_files(WEB_SRC_DIR):
        with open(path, encoding="utf-8") as handle:
            source = strip_ts_comments(handle.read())
        for pattern in (WEB_SORT_LITERAL, WEB_SORT_CONSTANT):
            for match in pattern.finditer(source):
                value = match.group(1)
                if not value or value in allowed:
                    continue
                line = source[: match.start()].count("\n") + 1
                hits.append(f"{os.path.relpath(path, REPO_ROOT)}:{line}: {value!r}")
    return hits


# --- Rule 5: a page size the web app asks for must be one the server serves ---
#
# MaxPageSize is read from its Go definition rather than duplicated here: a
# second literal would drift from the first, which is the defect this whole
# script exists to catch.
MAX_PAGE_SIZE_DECL = re.compile(r"\bMaxPageSize\s*=\s*(\d+)")
PAGINATION_GO = os.path.join(REPO_ROOT, "internal/platform/pagination/pagination.go")
WEB_PAGE_SIZE_LITERAL = re.compile(r"\bpageSize\s*:\s*[\"']?(\d+)[\"']?")


def read_max_page_size():
    with open(PAGINATION_GO, encoding="utf-8") as handle:
        source = strip_go_comments(handle.read())
    match = MAX_PAGE_SIZE_DECL.search(source)
    if not match:
        raise SystemExit(
            "ERROR: could not read MaxPageSize from internal/platform/pagination/pagination.go"
        )
    return int(match.group(1))


def find_oversized_page_sizes(max_page_size):
    hits = []
    for path in web_source_files(WEB_SRC_DIR):
        with open(path, encoding="utf-8") as handle:
            source = strip_ts_comments(handle.read())
        for match in WEB_PAGE_SIZE_LITERAL.finditer(source):
            size = int(match.group(1))
            if size <= max_page_size:
                continue
            line = source[: match.start()].count("\n") + 1
            hits.append(f"{os.path.relpath(path, REPO_ROOT)}:{line}: {size}")
    return hits


# --- Rule 6: a filter dropdown's key must be a parameter the API reads -------
#
# `filters` forwards caller-chosen keys, so rules 1-2 skip it (`IGNORED_SENT`).
# That exemption is exactly the blind spot: the *key* is chosen by the page, so a
# dropdown whose key no handler reads renders, forwards, and does nothing — the
# same class as `status=enabled` on `keys`.
#
# Read structurally, not by regex: the block is delimited by the matching `]` of
# `filters={[`, and the keys inside it are collected. A regex would also pick up
# `key:` from `columns`/`rowSelection` arrays, which are table-row identity, not
# parameters.
FILTERS_PROP = re.compile(r"\bfilters\s*=\s*\{\[")
FILTER_KEY = re.compile(r'\bkey\s*:\s*"([^"]+)"')

# Filter keys that no handler reads yet. Each is a real column the console could
# filter on, so the fix is a backend implementation rather than a control
# removal; the entries are reported on every run so they cannot be forgotten.
#
# Empty as of 第十五批: the eight dead filters found in that sweep are all
# implemented (`radius_configs.authProtocol` plus the seven below), so rule 6 now
# fails outright on any new one.
#   adapters.databaseType / coupons.discountType / permissions.effect /
#   plans.isExclusive / products.isRecharge / webhooks.method / tickets.priority
UNRESOLVED_FILTERS = {}


def braced_block(source, start):
    """Return the text up to the `]` matching an already-opened `[` at `start`."""
    depth = 1
    i = start
    n = len(source)
    while i < n:
        c = source[i]
        if c in "\"'`":
            quote = c
            i += 1
            while i < n:
                if source[i] == "\\":
                    i += 2
                    continue
                if source[i] == quote:
                    i += 1
                    break
                i += 1
            continue
        if c == "[":
            depth += 1
        elif c == "]":
            depth -= 1
            if depth == 0:
                return source[start:i]
        i += 1
    return source[start:]


def find_unaccepted_filter_keys(accepted):
    """Return (bad, unresolved) filter-key findings."""
    bad = []
    unresolved = {}
    for path in web_source_files(WEB_SRC_DIR):
        with open(path, encoding="utf-8") as handle:
            source = strip_ts_comments(handle.read())
        relpath = os.path.relpath(path, REPO_ROOT)
        for match in FILTERS_PROP.finditer(source):
            block = braced_block(source, match.end())
            for key in FILTER_KEY.findall(block):
                if key in accepted:
                    continue
                if key in UNRESOLVED_FILTERS:
                    unresolved.setdefault(key, relpath)
                    continue
                bad.append(f"{relpath}: filters key {key!r}")
    return bad, unresolved


def main():
    accepted = collect_backend_params()
    if not accepted:
        print("ERROR: no query parameters found in the transport layer", file=sys.stderr)
        return 1

    sent = collect_web_params()
    unknown = {name: path for name, path in sent.items() if name not in accepted}

    failed = False

    if unknown:
        failed = True
        print(
            f"API parameter check FAILED ({len(unknown)} parameter(s) the web app "
            "sends but no handler reads):"
        )
        for name, path in sorted(unknown.items()):
            print(f"  - {name!r} (sent by {path})")
        print(
            "  accepted names: " + ", ".join(sorted(accepted)),
            file=sys.stderr,
        )

    declared = collect_declared_query_params()
    if not declared:
        failed = True
        print(
            "API parameter check FAILED: no query parameter components found in "
            "openapi.yaml (the parser may be stale)",
            file=sys.stderr,
        )
    dangling = {
        component: name
        for component, name in declared.items()
        if name not in accepted and name not in IGNORED_DECLARED
    }
    if dangling:
        failed = True
        print(
            f"API parameter check FAILED ({len(dangling)} parameter(s) the OpenAPI "
            "contract declares but no handler reads):"
        )
        for component, name in sorted(dangling.items()):
            print(f"  - {component} declares {name!r}")
        print(
            "  A declared-but-unread parameter is documented behaviour the server "
            "does not have: a client that follows the spec sends it and gets no "
            "error and no effect. Rename the component to the name the handlers "
            "read, or stop declaring it.\n"
            "  accepted names: " + ", ".join(sorted(accepted)),
            file=sys.stderr,
        )

    floors = find_page_size_floors()
    if floors:
        failed = True
        print(
            f"API parameter check FAILED ({len(floors)} pagination floor(s) in the "
            "repositories):"
        )
        for hit in floors:
            print(f"  - {hit}")
        print(
            "  `max(x.PageSize, N)` is a floor, not a cap: pageSize=5 returns N "
            "rows while the response echoes 5, and the offset (computed from a "
            "different N) makes adjacent pages overlap. Use shared.ClampPageSize "
            "and shared.PageOffset so there is one bound (手册 4.7.1).",
            file=sys.stderr,
        )

    local_helpers = find_local_pagination_helpers()
    if local_helpers:
        failed = True
        print(
            f"API parameter check FAILED ({len(local_helpers)} package-local "
            "function(s) receiving a page size):"
        )
        for hit in local_helpers:
            print(f"  - {hit}")
        print(
            "  A page-size bound belongs in exactly one place "
            "(shared.ClampPageSize / shared.PageOffset, 手册 4.7.1). A private "
            "helper is a second implementation that no naming-based rule can see: "
            "`mx(q.PageSize, 20)` sat in fifteen repositories and the `max(` rule "
            "above never matched it.",
            file=sys.stderr,
        )

    sort_whitelist = collect_sort_whitelist()
    if not sort_whitelist:
        failed = True
        print(
            "API parameter check FAILED: no sort whitelist found "
            "(platformsorting.NewSpec in internal/infra/postgres — the parser may be stale)",
            file=sys.stderr,
        )
    bad_sort = find_bad_sort_values(sort_whitelist)
    if bad_sort:
        failed = True
        print(
            f"API parameter check FAILED ({len(bad_sort)} sort value(s) outside the "
            "backend whitelist):"
        )
        for hit in bad_sort:
            print(f"  - {hit}")
        print(
            "  The whitelist keys are API camelCase, so `created_at` is not a typo "
            "the server tolerates: platformsorting.Spec.Resolve answers 422 and the "
            "entire list request fails. Allowed: " + ", ".join(sorted(sort_whitelist)),
            file=sys.stderr,
        )

    max_page_size = read_max_page_size()
    oversized = find_oversized_page_sizes(max_page_size)
    if oversized:
        failed = True
        print(
            f"API parameter check FAILED ({len(oversized)} pageSize value(s) above "
            f"MaxPageSize={max_page_size}):"
        )
        for hit in oversized:
            print(f"  - {hit}")
        print(
            "  The middleware rejects an out-of-range pageSize with 422 rather than "
            "truncating it, so the request fails outright.",
            file=sys.stderr,
        )

    if failed:
        return 1

    bad_filters, unresolved_filters = find_unaccepted_filter_keys(accepted)
    if bad_filters:
        print(
            f"API parameter check FAILED ({len(bad_filters)} filter key(s) no "
            "handler reads):"
        )
        for hit in bad_filters:
            print(f"  - {hit}")
        print(
            "  A `filters` key is the query parameter name, so a key no handler "
            "reads makes the dropdown render, forward, and do nothing. Name it "
            "after the parameter the handler reads, or drop the control.",
            file=sys.stderr,
        )
        return 1

    print(
        f"API parameter check OK: {len(sent)} parameter(s) sent by the web app, "
        f"all accepted by one of {len(accepted)} handler parameters; "
        f"{len(declared)} declared query parameter(s) all read by a handler; "
        "no pagination floor in the repositories; "
        f"every sort value is one of {len(sort_whitelist)} whitelisted key(s); "
        f"no pageSize above {max_page_size}; "
        "every filter key is a parameter a handler reads"
    )
    for key, path in sorted(unresolved_filters.items()):
        print(f"  NOTE {UNRESOLVED_FILTERS[key]}: filter key {key!r} matches no handler parameter — used by {path}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
