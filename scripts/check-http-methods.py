#!/usr/bin/env python3
"""Check that every request the console issues matches a route the server registers.

Why this exists
---------------
The console and the transport layer each spell out an HTTP method and a path
independently:

    router.Get("/api/v1/mcp-servers/online", ...)      // transport
    iamPostRaw("/api/v1/mcp-servers/online")           // console

Nothing compared them. A method mismatch is not a soft failure — the router has
no handler for that verb on that path, so the request 404s (or, for a chi sub
router, 405s) and the page stays empty forever. That is exactly what happened to
the MCP server store: the console POSTed while the handler registered GET, and
the page could never load.

`check-api-params.py` checks the *parameter names* on a path and
`check-openapi-contract.sh` checks the generated spec, but neither looks at the
verb a client actually sends. This gate covers that gap.

How it works
------------
Two sides are collected and intersected:

  * **Issued** — every request helper call under `web/src`. There are two
    families:

      - verb-in-the-name: `iamGet`/`iamCreate`/`iamPatch`/`iamUpdate`/
        `iamDelete`/`iamPostRaw`/`iamPost`;
      - verb-in-the-options: `request`/`requestEnvelope`/`fetchImpl`/`fetch`
        (read `init.method`, defaulting to GET, which is `fetch`'s own default),
        `navigator.sendBeacon` (always POST), and `requestDocumentJson`.

    The second family used to be excluded, and that is exactly where eight
    console capabilities broke unnoticed: `shared/lib/auth-*.ts` aimed
    `/api/send-verification-code`, `/api/get-captcha`, `/api/get-qrcode`,
    `/api/get-webhook-event`, `/api/faceid-signin-begin`,
    `/api/webauthn/signin/begin|finish` and `/api/get-saml-login` at a server
    that never registered them.

    The path is resolved through `web/src/shared/api/paths.ts` — the module the
    console uses as its single source of API paths:

      - string/template literals (including nested `${…}` helpers);
      - imported declarations: `USERS`, `USER_BY_ID(id)`;
      - file-local aliases: `const BASE = AI_GATEWAY_BASE`;
      - local query wrappers: `withOrganization(…)`, `withQuery(…)`.

    A call that still cannot be resolved statically is *skipped*, but not
    silently: it must appear in `SKIP_ALLOWLIST` with a reason (generic HTTP
    plumbing and wrappers that receive the path at runtime), and any unlisted
    skip fails the gate. A coverage floor also fails the run when fewer calls
    resolve than are skipped — the exact shape of the old "1 issued / 557
    skipped, exit 0" defect.

  * **Registered** — every `router.Get/Post/Put/Patch/Delete("...")` under
    `internal/transport/http`, plus the `Pattern:`/`Method:` pairs in the
    neighbouring `routes.go` (the two are already held equal to each other by
    `internal/bootstrap/route_contract_test.go`, so either source is
    representative; using the registration site keeps this gate about the
    runtime).

A path is compared after normalising `{id}`/`${...}` to a single placeholder, so
`/api/v1/mcp-servers/${id}` matches `/api/v1/mcp-servers/{id}`.

Helper family vs path family
----------------------------
A second, independent way a request can fail before routing is consulted: the
helper cannot parse the response it will receive.

  * requestDocumentJson parses bare documents — the only API helper the
    frontend emits. The v1 `/api/v1/*` surface answers with bare documents,
    so this is the correct choice. Aiming it at the legacy `/api/<something>`
    surface would be a mistake (no such routes remain registered).

Usage: python3 scripts/check-http-methods.py [--self-test]
Exit code 0 = every issued request matches a registered route, 1 = drift found.
`--self-test` runs synthetic parser / verb-mismatch / coverage-floor samples and
exits 1 if any case stops behaving as specified.

输入：internal/transport/http/route/ 方法声明、web/src/ api 调用（iamGet/iamPostRaw/...）、web/src/shared/api/paths.ts 路径声明
输出：PASS/FAIL + 方法/路径不匹配的端点
--check 行为：无 --check；失败即 exit 1；--self-test 为自检
消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
"""

import os
import re
import sys
from collections import defaultdict

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
WEB_SRC = os.path.join(REPO_ROOT, "web/src")
TRANSPORT = os.path.join(REPO_ROOT, "internal/transport/http")

# Helper name -> implied verb. A helper missing from this map means its verb
# cannot be inferred from the name, so it is read from the call's options object
# (and defaults to GET, which is `fetch`'s own default).
HELPER_VERBS = {
    "iamGet": "GET",
    "iamGetIfChanged": "GET",
    "iamList": "GET",
    "iamListWithPagination": "GET",
    "iamCreate": "POST",
    "iamPatch": "PATCH",
    "iamUpdate": "PUT",
    "iamDelete": "DELETE",
    "iamPostRaw": "POST",
    "iamPost": "POST",
}

# Helpers that carry no verb in their name: the verb lives in `init.method`.
#
# These were previously outside this gate, and that is precisely where the
# defects hid: `shared/lib/auth-*` called `/api/send-verification-code`,
# `/api/get-captcha`, `/api/webauthn/signin/begin` and friends through
# `request`/`requestEnvelope`/`fetchImpl`. Those routes do not exist in this
# server, so eight console capabilities failed while the gate stayed green.
OPTION_VERB_HELPERS = (
    "request",
    "requestEnvelope",
    "fetchImpl",
    "fetch",
    "requestDocumentJsonIfChanged",
    "requestText",
    "requestBlob",
)

# `navigator.sendBeacon` is always POST by definition.
BEACON_HELPER = "sendBeacon"

ALL_HELPERS = tuple(HELPER_VERBS) + OPTION_VERB_HELPERS + (BEACON_HELPER, "requestDocumentJson")

# Matches the helper name plus its opening paren; the arguments are then read by
# scanning to the balanced closing paren, because a naive `\{.*?\}` capture stops
# at the first nested brace (`body: JSON.stringify({...})`) and silently loses the
# `method:` that follows it.
#
# The lookbehind rejects a `.` so an unrelated `thing.request(...)` is not read as
# the console's `request` helper. `sendBeacon` is handled separately: it is only
# ever `navigator.sendBeacon`, so a `.` prefix is expected rather than suspicious.
HELPER_CALL = re.compile(
    r"(?<![\w.])(("
    + "|".join(name for name in ALL_HELPERS if name != BEACON_HELPER)
    + r"))\s*(?:<[^>]*>)?\s*\("
)

BEACON_CALL = re.compile(r"\bsendBeacon\s*\(")

# A path literal is either an ordinary string or a template literal, and must be
# absolute so it addresses this server.

OPTION_METHOD = re.compile(r"\bmethod\s*:\s*[\"']([A-Za-z]+)[\"']")

# requestDocumentJson parses bare documents from the v1 API surface.
# Legacy envelope helpers (request/requestEnvelope/requestJson) were removed in
# the Casdoor protocol cleanup — they are no longer emitted by the frontend.
DOCUMENT_HELPERS = frozenset({"requestDocumentJson"})

V1_PATH = re.compile(r"^/api/v1/")
# Casdoor 风格旧面 `/api/*`（非 `/api/v1/*`）：已无对应路由注册，文档类助手若仍指向
# 该前缀即为死代码。此常量曾在重构中丢失而使用处保留，导致门禁 `LEGACY_PATH`
# NameError 崩溃、契约检查从未执行（known-red 把崩溃记成 1 条 WARN）。
LEGACY_PATH = re.compile(r"^/api/(?!v1/)")



def skip_braced(source, i):
    """Advance past a `{...}` block starting at `i`, honouring nesting and strings."""
    depth = 0
    n = len(source)
    while i < n:
        c = source[i]
        if c in "\"'`":
            found = read_string(source, i)
            if found is None:
                return n
            i = found[1]
            continue
        if c == "{":
            depth += 1
        elif c == "}":
            depth -= 1
            if depth == 0:
                return i + 1
        i += 1
    return n


def braced_spans(source):
    """Return the (start, end) span of every balanced `${...}` in a path literal."""
    spans = []
    i = 0
    n = len(source)
    while i < n:
        if source.startswith("${", i):
            end = skip_braced(source, i + 1)
            spans.append((i, end))
            i = end
            continue
        i += 1
    return spans


def read_string(source, i):
    """Read the string/template literal starting at `i`; return (text, end) or None.
    Template literals can nest: ``/api/v1/adapters/${id}/policies${qs ? `?${qs}` : ""}``
    embeds a second template inside the first. A `[^`]*` style scan stops at that
    inner backtick, truncating the path mid-expression and inventing a mismatch
    for a route that exists, so the `${…}` body is skipped by brace balancing.
    """
    quote = source[i]
    start = i
    i += 1
    n = len(source)
    while i < n:
        c = source[i]
        if c == "\\":
            i += 2
            continue
        if quote == "`" and c == "$" and i + 1 < n and source[i + 1] == "{":
            i = skip_braced(source, i + 1)
            continue
        if c == quote:
            return source[start : i + 1], i + 1
        i += 1
    return None


def call_arguments(source, open_paren):
    """Return the text between a call's parentheses, honouring nesting and strings."""
    depth = 0
    i = open_paren
    n = len(source)
    while i < n:
        c = source[i]
        if c in "\"'`":
            found = read_string(source, i)
            i = found[1] if found else n
            continue
        if c == "(":
            depth += 1
        elif c == ")":
            depth -= 1
            if depth == 0:
                return source[open_paren + 1 : i]
        i += 1
    return ""


PATHS_FILE = os.path.join(WEB_SRC, "shared/api/paths.ts")

PATH_DECL = re.compile(r"\bexport\s+const\s+([A-Za-z_$][\w$]*)\s*(?::[^=]*)?=\s*")
LOCAL_DECL = re.compile(r"(?<![\w.])const\s+([A-Za-z_$][\w$]*)\s*(?::[^=]*)?=\s*")
IDENTIFIER = re.compile(r"^[A-Za-z_$][\w$]*$")
CALL_HEAD = re.compile(r"^([A-Za-z_$][\w$]*)\s*\(")
LEADING_IDENTIFIER = re.compile(r"[A-Za-z_$][\w$]*")

# Local wrappers that only append a query string to a path. Resolving their
# first argument keeps real call sites visible instead of losing them to the
# wrapper's variable name.
PATH_WRAPPERS = {"withOrganization", "withQuery", "withOrgQuery"}

# Call sites whose path is a runtime value by design. Every entry must name a
# file, helper and first-argument token, with the reason; a skip that is not
# listed here fails the gate, so coverage cannot decay silently again.
SKIP_ALLOWLIST = {
    ("web/src/shared/api/http.ts", "requestDocumentJson", "path"): "generic client receives the path",
    ("web/src/shared/api/http.ts", "requestDocumentJsonIfChanged", "path"): "generic client receives the path",
    ("web/src/shared/api/http.ts", "requestText", "path"): "generic client receives the path",
    ("web/src/shared/api/http.ts", "requestBlob", "path"): "generic client receives the path",
    ("web/src/shared/api/http.ts", "fetch", "path"): "generic client receives the path",
    ("web/src/shared/api/iam/client.ts", "iamGet", "path"): "helper definition receives the path",
    ("web/src/shared/api/iam/client.ts", "iamGetIfChanged", "path"): "helper definition receives the path",
    ("web/src/shared/api/iam/client.ts", "iamList", "path"): "helper definition receives the path",
    ("web/src/shared/api/iam/client.ts", "iamListWithPagination", "path"): "helper definition receives the path",
    ("web/src/shared/api/iam/client.ts", "iamCreate", "path"): "helper definition receives the path",
    ("web/src/shared/api/iam/client.ts", "iamPost", "path"): "helper definition receives the path",
    ("web/src/shared/api/iam/client.ts", "iamPostRaw", "path"): "helper definition receives the path",
    ("web/src/shared/api/iam/client.ts", "iamPatch", "path"): "helper definition receives the path",
    ("web/src/shared/api/iam/client.ts", "iamUpdate", "path"): "helper definition receives the path",
    ("web/src/shared/api/iam/client.ts", "iamDelete", "path"): "helper definition receives the path",
    ("web/src/shared/api/iam/client.ts", "requestDocumentJson", "path"): "helper definition receives the path",
    ("web/src/shared/api/iam/client.ts", "requestDocumentJsonIfChanged", "path"): "helper definition receives the path",
    ("web/src/shared/api/iam/identity-pools.ts", "requestDocumentJson", "path"): "requestRequired wrapper receives the path",
    ("web/src/shared/api/iam/organization.ts", "requestDocumentJson", "path"): "loop over both known identifier paths",
    ("web/src/shared/ui/ProviderFields/SamlProviderFields.tsx", "requestText", "url"): "metadata URL is a component prop",
    ("web/src/shared/lib/auth-prompt-data.ts", "requestDocumentJson", "url"): "option loader receives the URL from its caller",
}


def parse_path_declarations(source):
    """Read `export const` path declarations into {name: template-body}.

    There are two shapes in `paths.ts`:

        export const USERS = "/api/v1/users";
        export const USER_BY_ID = (id: string) => `/api/v1/users/${encodeURIComponent(id)}`;

    A plain literal is stored as-is. For an arrow function the parameter list,
    return type and `=>` are skipped and only the returned literal is kept. The
    body keeps its `${…}` interpolations so a declaration that references a
    sibling helper (`${USER_BY_ID(id)}/revoke`) can be resolved later.

    This module is the single source of API paths (see its header), so parsing
    it — rather than accepting only inline literals — is what turns the gate
    from "1 issued / 557 skipped" into real coverage.
    """
    declarations = {}
    for match in PATH_DECL.finditer(source):
        name = match.group(1)
        i = match.end()
        n = len(source)
        while i < n and source[i] in " \t\n\r":
            i += 1
        if i < n and source[i] in "\"'`":
            found = read_string(source, i)
            if found is not None:
                declarations[name] = found[0][1:-1]
            continue
        if i >= n or source[i] != "(":
            continue
        depth = 0
        j = i
        while j < n:
            c = source[j]
            if c in "\"'`":
                found = read_string(source, j)
                j = found[1] if found is not None else n
                continue
            if c == "(":
                depth += 1
            elif c == ")":
                depth -= 1
                if depth == 0:
                    j += 1
                    break
            j += 1
        arrow = source.find("=>", j)
        if arrow < 0:
            continue
        k = arrow + 2
        while k < n and source[k] in " \t\n\r":
            k += 1
        if k < n and source[k] in "\"'`":
            found = read_string(source, k)
            if found is not None:
                declarations[name] = found[0][1:-1]
    return declarations


def resolve_interpolation(expression, declarations, depth):
    """Resolve one `${…}` body to literal text, or None if it is a runtime value.

    A reference to a path declaration keeps the declaration's text (recursing
    through nested helper calls). Every other expression is a runtime value and
    stays unresolved; the caller keeps it as `${…}` so `normalise` can apply its
    path-segment vs trailing-query rules.
    """
    expression = expression.strip()
    if IDENTIFIER.match(expression) and expression in declarations:
        return declarations[expression]
    head = CALL_HEAD.match(expression)
    if head and head.group(1) in declarations and depth <= 8:
        resolved = resolve_template(declarations[head.group(1)], declarations, depth + 1)
        if resolved is not None:
            return resolved
    return None


def resolve_template(template, declarations, depth=0):
    """Substitute every resolvable `${…}` in a path template.

    Unresolved interpolations are kept verbatim: `normalise` turns a path
    segment into `{}` and drops a trailing query builder, so a resolved
    `/api/v1/users/count${query}` still compares equal to the registered
    `/api/v1/users/count`.
    """
    if depth > 8:
        return None
    spans = braced_spans(template)
    out = []
    cursor = 0
    for start, end in spans:
        out.append(template[cursor:start])
        expression = template[start + 2 : end - 1]
        resolved = resolve_interpolation(expression, declarations, depth)
        out.append(resolved if resolved is not None else "${" + expression + "}")
        cursor = end
    out.append(template[cursor:])
    return "".join(out)


def parse_local_path_constants(source, declarations):
    """Collect file-local aliases that hold a static path.

    `aigateway.ts` does `const BASE = AI_GATEWAY_BASE;` and builds every URL
    from it; `b2b.ts` builds a local `const url` from a path template.
    Without these aliases those real calls stay invisible. Only a literal or a
    reference to an already-known declaration is accepted — a local assigned
    from a function call stays unresolved.
    """
    local = {}
    for match in LOCAL_DECL.finditer(source):
        name = match.group(1)
        i = match.end()
        n = len(source)
        while i < n and source[i] in " \t\n\r":
            i += 1
        if i >= n:
            continue
        if source[i] in "\"'`":
            found = read_string(source, i)
            if found is None:
                continue
            j = found[1]
            while j < n and source[j] in " \t\n\r":
                j += 1
            if j < n and source[j] == ";":
                resolved = resolve_template(found[0][1:-1], {**declarations, **local})
                if resolved is not None:
                    local[name] = resolved
            continue
        identifier = LEADING_IDENTIFIER.match(source, i)
        if identifier is None:
            continue
        j = identifier.end()
        while j < n and source[j] in " \t\n\r":
            j += 1
        if j < n and source[j] == ";" and identifier.group(0) in {**declarations, **local}:
            local[name] = {**declarations, **local}[identifier.group(0)]
    return local


def first_argument_expression(arguments):
    """Return the first argument as one balanced expression, or None.

    A nested call must not be cut at its internal commas (`USER_BY_ID(a, b)`),
    so the scan tracks bracket depth and string literals.
    """
    start = 0
    while start < len(arguments) and arguments[start] in " \t\n\r":
        start += 1
    if start >= len(arguments):
        return None
    i = start
    n = len(arguments)
    depth = 0
    while i < n:
        c = arguments[i]
        if c in "\"'`":
            found = read_string(arguments, i)
            if found is None:
                return None
            i = found[1]
            continue
        if c in "([{":
            depth += 1
        elif c in ")]}":
            depth -= 1
        elif c == "," and depth == 0:
            break
        i += 1
    expression = arguments[start:i].strip()
    return expression or None


def skip_token(arguments):
    """Identify a dynamic first argument by its leading name.

    A template that starts with an interpolation (`${path}${separator}${qs}`)
    is keyed by that identifier, so the allowlist names the dynamic variable
    rather than a fragile literal prefix.
    """
    expression = (first_argument_expression(arguments) or "").strip()
    if expression.startswith("`"):
        match = re.search(r"\$\{\s*([A-Za-z_$][\w$]*)", expression)
        return match.group(1) if match else "template"
    match = LEADING_IDENTIFIER.match(expression)
    return match.group(0) if match else expression[:30]


def first_path_argument(arguments, declarations):
    """Return the first argument resolved to a path, or None.

    The first argument may be a string/template literal, an exported constant
    (`USERS`), an exported helper call (`USER_BY_ID(id)`) or a local query
    wrapper (`withQuery(METERING_AGGREGATES_REFRESH, …)`). A local variable, a
    concatenation or an unknown helper cannot be resolved statically and yields
    None, which the caller reports as skipped.
    """
    expression = first_argument_expression(arguments)
    if expression is None:
        return None
    if expression[0] in "\"'`":
        found = read_string(expression, 0)
        if found is None or found[1] != len(expression):
            return None
        template = found[0][1:-1]
        return resolve_template(template, declarations)
    head = CALL_HEAD.match(expression)
    if head and head.group(1) in PATH_WRAPPERS:
        inner = call_arguments(expression, expression.find("(", head.end() - 1))
        return first_path_argument(inner, declarations)
    return resolve_interpolation(expression, declarations, 0)

NOTES = r'''
Why the declared table is the registered side
--------------------------------------------
The registration sites (`chi.go`) are the wrong source here for three reasons
that are all real in this repo: the receiver is named `router` in most files and
`r` in others; several modules chain `router.With(mw.X).Post(...)`, which a
`router\.(Get|Post)` pattern misses; and the authentication module registers
**relative** paths (`/login`) on a sub-router, so the full path only exists once
the mount prefix is resolved.

`routes.go` carries the full `Method` + `Pattern` for every route, and
`internal/bootstrap/route_contract_test.go::TestRouteContractMatchesRuntimeRouter`
already holds the declared set equal to the runtime router (511 ⇔ 511). So reading
the declarations keeps this gate about the real routing table while staying
robust to how a handler happens to be registered.
'''

# `{ Method: "POST", Pattern: "/api/v1/..." }` — the two fields sit in the same
# struct literal, so the pair is matched as a unit rather than by proximity.
ROUTE_DECL = re.compile(
    r"Method:\s*\"([A-Za-z]+)\"[^}]*?Pattern:\s*\"([^\"]+)\"", re.DOTALL
)
ROUTE_DECL_REVERSED = re.compile(
    r"Pattern:\s*\"([^\"]+)\"[^}]*?Method:\s*\"([A-Za-z]+)\"", re.DOTALL
)
# Requests that match no route and are NOT yet resolved. Each entry names the
# decision it is waiting on, and the OK output lists them so they cannot be
# forgotten. Keeping them out of the failure path is deliberate: a gate that
# stays red on a known, product-blocked item stops being read, and then it cannot
# report the next real breakage either.
#
# Q9 — `transaction.ts`'s `placeExistingOrder(orderId)` posts to
# `/api/v1/orders/{id}:place`. The backend has no such route: `PlaceOrder`
# generates a NEW id and stores `State: "Created"`, so it is a create, and the
# order state machine has no "place an existing order" transition (`pending` is
# already the placed state). `order.ts`'s `placeOrder(payload)` already POSTs the
# correct `/api/v1/orders:place`. The detail page's "confirm place order" button
# therefore 404s. Removing it is user-visible, so it needs a product decision.
#
# Q11–Q13 (resolved 2026-10) — legacy Casdoor endpoints `/api/faceid-signin-begin`,
# `/api/get-qrcode`, `/api/get-webhook-event`, `/api/get-saml-login` are no longer
# emitted by the frontend after the auth shim layer cleanup. The `UNRESOLVED`
# entries for these routes have been removed.
UNRESOLVED = {
    ("POST", "/api/v1/orders/{}:place"): "Q9",
}

# Paths the console requests that intentionally have no *declared* route:
#
#   * `/.well-known/openid-configuration` is served by the identity provider
#     under test, not by this server.
#   * `/openapi/openapi.json` is the generated spec itself. It is registered
#     directly on the router in `internal/bootstrap/server.go`, outside the
#     `routes.go` declaration system this gate reads (a spec cannot describe its
#     own serving route through itself).
ISSUED_EXEMPT_PREFIXES = (
    "/.well-known/openid-configuration",
    "/openapi/",
)


def web_files():
    for dirpath, _dirs, files in os.walk(WEB_SRC):
        for name in sorted(files):
            if not (name.endswith(".ts") or name.endswith(".tsx")):
                continue
            if ".test." in name or ".spec." in name:
                continue
            yield os.path.join(dirpath, name)


def go_files(root):
    for dirpath, _dirs, files in os.walk(root):
        for name in sorted(files):
            if name.endswith(".go") and not name.endswith("_test.go"):
                yield os.path.join(dirpath, name)


def strip_ts_comments(source):
    """Drop comments, keep string literals.

    Comments must go: a comment explaining the `online` verb mismatch quotes the
    very call under test. Strings must stay: the paths are string literals.
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
            i = n if j < 0 else j
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


def normalise(path):
    """Reduce a path to a comparable shape.

    Router parameters (`{id}`) and template interpolations (`${id}`) both become
    `{}`, so `/api/v1/mcp-servers/${id}` and `/api/v1/mcp-servers/{id}` are the
    same route. A query string never selects a route, so it is dropped.

    A **trailing** interpolation is two different things depending on what
    precedes it:

      * `/api/v1/adapters/${id}` — the `/` makes it a path segment, so it stays
        as `{}`;
      * `/api/v1/mcp-servers/online${query}` — it is glued to the preceding
        literal and builds an optional query suffix (`online` or
        `online?x=y`), so the path is `online`.

    The query string is dropped **after** interpolations are resolved: a
    template like `/api/v1/adapters/${id}/policies${qs ? "?" + qs : ""}` carries
    its `?` *inside* the interpolation, so splitting on `?` first truncated the
    path mid-expression and turned a real route into a phantom mismatch.

    The interpolations are located by brace balancing, not by a regex: the body
    may itself contain a nested template (``${qs ? `?${qs}` : ""}``), whose
    closing brace a `[^}]*` style pattern would stop at.
    """
    spans = braced_spans(path)

    # A trailing interpolation glued to a non-`/` predecessor builds an optional
    # query suffix (`online` or `online?x=y`), so the path stops before it.
    if spans and spans[-1][1] == len(path) and not path[: spans[-1][0]].endswith("/"):
        path = path[: spans[-1][0]]
        spans = spans[:-1]

    rebuilt = []
    cursor = 0
    for start, end in spans:
        rebuilt.append(path[cursor:start])
        rebuilt.append("{}")
        cursor = end
    rebuilt.append(path[cursor:])
    path = "".join(rebuilt)

    path = re.sub(r"\{[^}]*\}", "{}", path)
    # A query string never selects a route, so drop it now that no literal `?`
    # is hiding inside an interpolation any more.
    path = path.split("?")[0]
    return path.rstrip("/") or "/"


def collect_issued(declarations):
    """Return (issued, skipped, unlisted, helper_misuse, resolved_calls).

    `issued` maps (verb, normalised path) -> location. `resolved_calls` counts
    every call site whose path was resolved (the issued dict collapses duplicate
    verb+path pairs). `skipped` lists allowlisted dynamic call sites with the
    reason; `unlisted` lists dynamic call sites that are not registered, and any
    entry there fails the gate. `helper_misuse` records calls whose *helper
    family* cannot parse the *path family* it is aimed at — a mismatch that
    fails before any route is even consulted.
    """
    issued = {}
    skipped = []
    unlisted = []
    helper_misuse = []
    resolved_calls = 0
    for path in web_files():
        with open(path, encoding="utf-8") as handle:
            source = strip_ts_comments(handle.read())
        relpath = os.path.relpath(path, REPO_ROOT)
        scope = dict(declarations)
        scope.update(parse_local_path_constants(source, declarations))

        calls = [(match, match.group(1)) for match in HELPER_CALL.finditer(source)]
        calls += [(match, BEACON_HELPER) for match in BEACON_CALL.finditer(source)]

        for match, helper in calls:
            arguments = call_arguments(source, match.end() - 1)
            literal = first_path_argument(arguments, scope)
            if literal is None or not literal.startswith("/"):
                line = source[: match.start()].count("\n") + 1
                first = arguments.strip().split("\n")[0][:80]
                key = (relpath, helper, skip_token(arguments))
                label = f"{relpath}:{line}: {helper}({first}…)"
                reason = SKIP_ALLOWLIST.get(key)
                if reason is None:
                    unlisted.append(label)
                else:
                    skipped.append(f"{label} [{reason}]")
                continue
            resolved_calls += 1

            if helper in HELPER_VERBS:
                verb = HELPER_VERBS[helper]
            elif helper == BEACON_HELPER:
                verb = "POST"
            else:
                method = OPTION_METHOD.search(arguments)
                # `fetch` defaults to GET and none of these helpers ever sets a
                # different verb implicitly, so an absent `method:` is GET
                # rather than a guess.
                verb = method.group(1).upper() if method else "GET"

            if helper in DOCUMENT_HELPERS and LEGACY_PATH.match(literal):
                helper_misuse.append(
                    f"{relpath}: {helper}({literal}) expects a bare document, but "
                    "legacy /api/* surface used a Casdoor-style envelope — no "
                    "such routes remain registered"
                )

            key = (verb, normalise(literal))
            issued.setdefault(key, f"{relpath}: {helper}({literal}, …)")
    return issued, skipped, unlisted, helper_misuse, resolved_calls


def collect_registered():
    """Return {verb: {normalised path: location}} from the route declarations."""
    registered = defaultdict(dict)
    for path in go_files(TRANSPORT):
        if os.path.basename(path) != "routes.go":
            continue
        with open(path, encoding="utf-8") as handle:
            source = handle.read()
        relpath = os.path.relpath(path, REPO_ROOT)
        for match in ROUTE_DECL.finditer(source):
            verb, pattern = match.group(1).upper(), match.group(2)
            line = source[: match.start()].count("\n") + 1
            registered[verb][normalise(pattern)] = f"{relpath}:{line}"
        for match in ROUTE_DECL_REVERSED.finditer(source):
            pattern, verb = match.group(1), match.group(2).upper()
            line = source[: match.start()].count("\n") + 1
            registered[verb].setdefault(normalise(pattern), f"{relpath}:{line}")
    return registered


def load_path_declarations():
    with open(PATHS_FILE, encoding="utf-8") as handle:
        return parse_path_declarations(strip_ts_comments(handle.read()))


def evaluate(issued, skipped, unlisted, helper_misuse, registered, resolved_calls):
    if not issued:
        print("ERROR: no console request found (the parser may be stale)", file=sys.stderr)
        return 1
    if not registered:
        print("ERROR: no router registration found (the parser may be stale)", file=sys.stderr)
        return 1

    failed = bool(helper_misuse)
    if helper_misuse:
        print(f"HTTP method check FAILED ({len(helper_misuse)} helper/path mismatch(es)):")
        for item in helper_misuse:
            print(f"  - {item}")
        print(
            "  requestDocumentJson only understands bare documents from /api/v1/*. "
            "Legacy Casdoor envelope helpers were removed — do not target /api/* routes.",
            file=sys.stderr,
        )

    if unlisted:
        failed = True
        print(f"HTTP method check FAILED ({len(unlisted)} unregistered dynamic call site(s)):")
        for item in unlisted:
            print(f"  - {item}")
        print(
            "  A call whose path cannot be resolved statically must either use a "
            "web/src/shared/api/paths.ts declaration or be added to SKIP_ALLOWLIST "
            "with a reason, so skipped coverage is registered rather than silent.",
            file=sys.stderr,
        )

    misses = []
    unresolved = []
    for (verb, path), origin in sorted(issued.items()):
        if any(path.startswith(prefix) for prefix in ISSUED_EXEMPT_PREFIXES):
            continue
        if path in registered.get(verb, {}):
            continue
        if (verb, path) in UNRESOLVED:
            unresolved.append((verb, path, UNRESOLVED[(verb, path)], origin))
            continue
        # Report the closest thing found, so the message is actionable.
        other_verbs = sorted(v for v in registered if path in registered[v])
        misses.append((verb, path, origin, other_verbs))

    if misses:
        failed = True
        print(f"HTTP method check FAILED ({len(misses)} request(s) matching no route):")
        for verb, path, origin, other_verbs in misses:
            print(f"  - {verb} {path}")
            print(f"      issued by {origin}")
            if other_verbs:
                print(
                    f"      the path is registered for {'/'.join(other_verbs)} — "
                    "the verb is the mismatch"
                )
            else:
                print("      the path is not registered for any verb")
        print(
            "  A verb that matches no route is not a soft failure: the request "
            "404s and the page stays empty. Align the console helper with the "
            "handler, or add the handler if the capability is missing.",
            file=sys.stderr,
        )

    # Coverage floor: a parser that silently stops resolving `paths.ts` once
    # produced "1 issued / 557 skipped" and still exited 0. When more calls are
    # unreadable than readable, the green result says nothing about the console.
    total_skipped = len(skipped) + len(unlisted)
    if resolved_calls < total_skipped:
        failed = True
        print(
            f"HTTP method check FAILED: only {resolved_calls} of "
            f"{resolved_calls + total_skipped} call(s) resolved to a static path; "
            f"{total_skipped} could not be checked. Fix the resolver or move the "
            "paths back into web/src/shared/api/paths.ts."
        )
        for item in (skipped + unlisted)[:20]:
            print(f"  - {item}")
        if total_skipped > 20:
            print(f"  … and {total_skipped - 20} more skipped call(s)")

    if failed:
        return 1

    total_routes = sum(len(paths) for paths in registered.values())
    print(
        f"HTTP method check OK: {resolved_calls} request(s) issued by the web app "
        f"({len(issued)} distinct verb+path pair(s)), each matching one of "
        f"{total_routes} registered route(s); {total_skipped} call(s) skipped "
        "(path not statically resolvable)"
    )
    for item in skipped:
        print(f"  SKIP: {item}")
    for verb, path, decision, origin in unresolved:
        print(f"  NOTE {decision}: {verb} {path} matches no route — issued by {origin}")
    return 0


def self_test():
    """Prove the parser and the failure paths with synthetic inputs.

    Run: python3 scripts/check-http-methods.py --self-test
    """
    failures = []

    def check(name, condition):
        if not condition:
            failures.append(name)

    declarations = parse_path_declarations(
        'export const USERS = "/api/v1/users";\n'
        "export const USER_BY_ID = (id: string) =>\n"
        "  `/api/v1/users/${encodeURIComponent(id)}`;\n"
        "export const USER_REVOKE = (id: string) =>\n"
        "  `${USER_BY_ID(id)}/revoke`;\n"
    )
    check("plain constant parsed", declarations.get("USERS") == "/api/v1/users")
    check(
        "nested helper resolved",
        normalise(resolve_interpolation("USER_REVOKE(x)", declarations, 0))
        == "/api/v1/users/{}/revoke",
    )
    check(
        "template with query resolved",
        normalise(resolve_template("${USERS}?${query.toString()}", declarations))
        == "/api/v1/users",
    )
    check(
        "identifier first argument",
        first_path_argument('USERS, { method: "POST" }', declarations) == "/api/v1/users",
    )
    check(
        "helper-call first argument",
        normalise(first_path_argument('USER_BY_ID(userId), { method: "DELETE" }', declarations))
        == "/api/v1/users/{}",
    )
    check(
        "unknown identifier is skipped",
        first_path_argument("localPath, {}", declarations) is None,
    )
    check(
        "nested template normalised",
        normalise(resolve_template("${USER_REVOKE(id)}?x=1", declarations))
        == "/api/v1/users/{}/revoke",
    )

    registered = {"POST": {"/api/v1/users": "routes.go:1"}}
    import contextlib
    import io

    def run(issued, skipped, resolved_calls, unlisted=None):
        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(
            io.StringIO()
        ):
            return evaluate(
                issued, skipped, unlisted or [], [], registered, resolved_calls
            )

    check(
        "verb mismatch exits 1",
        run({("GET", "/api/v1/users"): "client.ts:1"}, [], 1) == 1,
    )
    check(
        "matching verb exits 0",
        run({("POST", "/api/v1/users"): "client.ts:1"}, [], 1) == 0,
    )
    check(
        "coverage collapse exits 1",
        run(
            {("POST", "/api/v1/users"): "client.ts:1"},
            ["client.ts:2: request(…)", "client.ts:3: request(…)"],
            1,
        )
        == 1,
    )
    check(
        "unlisted dynamic call exits 1",
        run(
            {("POST", "/api/v1/users"): "client.ts:1"},
            [],
            1,
            unlisted=["client.ts:4: request(localPath…)"],
        )
        == 1,
    )

    check(
        "local alias resolved",
        parse_local_path_constants(
            'const BASE = AI_GATEWAY_BASE;\n',
            {"AI_GATEWAY_BASE": "/api/v1/ai"},
        ).get("BASE")
        == "/api/v1/ai",
    )
    check(
        "query wrapper resolved",
        normalise(
            first_path_argument(
                'withQuery(METERING_AGGREGATES_REFRESH, { days }), {}',
                {"METERING_AGGREGATES_REFRESH": "/api/v1/metering/aggregates/refresh"},
            )
        )
        == "/api/v1/metering/aggregates/refresh",
    )

    if failures:
        print(f"SELF-TEST FAILED: {len(failures)} case(s): {failures}", file=sys.stderr)
        return 1
    print("SELF-TEST OK: parsing, verb-mismatch and coverage-floor cases behave as expected")
    return 0


def main():
    declarations = load_path_declarations()
    if not declarations:
        print(
            "ERROR: no path declaration parsed from web/src/shared/api/paths.ts "
            "(the parser may be stale)",
            file=sys.stderr,
        )
        return 1
    issued, skipped, unlisted, helper_misuse, resolved_calls = collect_issued(declarations)
    registered = collect_registered()
    return evaluate(issued, skipped, unlisted, helper_misuse, registered, resolved_calls)


if __name__ == "__main__":
    if "--self-test" in sys.argv[1:]:
        sys.exit(self_test())
    sys.exit(main())
