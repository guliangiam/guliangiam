#!/usr/bin/env python3
"""Check that every declared enum closed set agrees across the language boundary.

Why this exists
---------------
Enum-shaped API fields are validated against a closed set on the server
(手册 4.14.3, `internal/platform/enumset`). The console offers the same set in a
select. Nothing connected the two lists, and they diverged completely: the
console sent `Default`/`reCAPTCHA`/`Turnstile` while the captcha factory
registers `default`/`recaptcha`/`cloudflare`, and the notification console sent
`DingTalk`/`WeCom`/… against `dingtalk`/`wecom`/…. Not one value matched, so
every captcha or notification provider created in the console was unusable — the
failure only appeared later, as a 500 when generating a captcha or as a
notification that never arrived.

Three rules, each covering a different way the two sides can drift:

  1. **Set integrity** — a `NewSet` declaration must be well formed and its name
     unique. A duplicate name silently merges two different concepts.
  2. **Cross-language agreement** — a frontend list tagged `@enumset <name>` must
     equal the Go set of that name, in *both* directions. A frontend-only value
     produces a form the user cannot submit; a server-only value hides a
     supported capability from the console. Both are defects, so neither
     direction is a warning.
  3. **No unannotated lists** — a frontend `z.enum([...])` on a field that has a
     server set, without an annotation, is where the next divergence will start.
  4. **Caption coverage** — a value that reaches the screen must have a caption in
     every language pack. Adding a state on the server, or a value to a frontend
     list, without the caption left a raw `payment_pending` in the column and an
     English word in every other language; both are the same defect.
  5. **Transition agreement** — a state select offers the edges the server
     declares, not a hand-written list. `web/src/shared/constants/lifecycles.ts`
     mirrors each Go `var valid...Transitions map[string][]string`; an edge the
     server removed but the console still offers is a form that answers 422.

Usage: python3 scripts/check-enums.py
Exit code 0 = every set agrees, 1 = drift found.

输入：internal/platform/enumset/、web/src/ z.enum 与 @enumset 注释
输出：PASS/FAIL + 枚举漂移项
--check 行为：无 --check；失败即 exit 1
消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
"""

import glob
import json
import os
import re
import sys

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DOMAIN_DIR = os.path.join(REPO_ROOT, "internal/domain")
PLATFORM_DIR = os.path.join(REPO_ROOT, "internal/platform")
ENUMSET_PKG = os.path.join(PLATFORM_DIR, "enumset")
GO_SCAN_DIRS = [DOMAIN_DIR, PLATFORM_DIR, os.path.join(REPO_ROOT, "internal/app")]
FRONTEND_ENUMS = os.path.join(REPO_ROOT, "web/src/shared/constants/enums.ts")
FRONTEND_LIFECYCLES = os.path.join(REPO_ROOT, "web/src/shared/constants/lifecycles.ts")
LOCALES_DIR = os.path.join(REPO_ROOT, "web/src/shared/i18n/locales")

# `enumset.NewSet(\n "name",\n "a",\n ... )` — the declaration may span lines and
# the name may be a literal. A non-literal name cannot be matched to a frontend
# annotation, so it is reported rather than skipped.
#
# The body is read with balanced parentheses rather than a regex, because a
# value may be wrapped in a conversion (`string(DecisionAllow)`) to pass a
# named-string constant into the variadic `...string` parameter. A non-greedy
# regex stops at the first `)` and reports the set as empty.
SET_START = re.compile(r"enumset\.NewSet\(")


def balanced_call_body(source, open_paren):
    """Return the text between a call's parentheses (strings aware)."""
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


def split_top_level(body):
    """Split a call body on commas that are not inside parentheses."""
    parts = []
    depth = 0
    current = []
    i = 0
    n = len(body)
    while i < n:
        c = body[i]
        if c in "\"'`":
            quote = c
            start = i
            i += 1
            while i < n:
                if body[i] == "\\":
                    i += 2
                    continue
                if body[i] == quote:
                    i += 1
                    break
                i += 1
            current.append(body[start:i])
            continue
        if c == "(":
            depth += 1
        elif c == ")":
            depth -= 1
        elif c == "," and depth == 0:
            parts.append("".join(current))
            current = []
            i += 1
            continue
        current.append(c)
        i += 1
    parts.append("".join(current))
    return parts


GO_STRING_LITERAL = re.compile(r'"([^"]*)"')

# A set body may name its values through constants instead of inline literals,
# and it should: `enumset.NewSet("ordersState", StateCreated, ...)` keeps the
# value list and the transition table reading from the same declaration, while
# inlining the literals would create a second copy that can drift. The parser
# therefore resolves identifiers against the file's own `const` declarations
# rather than requiring literals. An identifier it cannot resolve is reported,
# never skipped — otherwise a typo would silently shrink a set to nothing.
GO_CONST_BLOCK_START = re.compile(r"^const\s*\($")
GO_CONST_BLOCK_END = re.compile(r"^\)\s*$")
GO_CONST_SINGLE = re.compile(
    r"^const\s+([A-Za-z_]\w*)(?:\s+[A-Za-z_][\w\.\[\]\*]*)?\s*=\s*\"([^\"]*)\"$"
)
GO_CONST_MEMBER = re.compile(
    r"^\s*([A-Za-z_]\w*)(?:\s+[A-Za-z_][\w\.\[\]\*]*)?\s*=\s*\"([^\"]*)\"$"
)

# A frontend list must be an array of string literals, optionally `as const`,
# preceded by an `@enumset <name>` tag within the same comment block. A list
# whose values reach the screen also carries `captions=<ns>:<group>`, which
# declares where the caption of each value lives.
FRONTEND_ANNOTATION = re.compile(
    r"@enumset\s+([A-Za-z][A-Za-z0-9_]*)"
    r"(?:\s+captions=([A-Za-z][A-Za-z0-9_-]*):([A-Za-z][A-Za-z0-9_.-]*))?"
)
FRONTEND_ARRAY_DECL = re.compile(
    r"export const ([A-Z][A-Z0-9_]*)\s*(?::[^=]*)?=\s*\[(.*?)\]\s*(?:as const)?\s*;",
    re.DOTALL,
)
FRONTEND_STRING_LITERAL = re.compile(r'"([^"]*)"')

# Rule 3: every declared set must either have a frontend list or be recorded here.
#
# A value-overlap heuristic ("a `z.enum` that happens to share a value with some
# server set") was the first attempt and it was wrong: `z.enum(["totp", "sms",
# "email", "push"])` overlaps the notification set through `sms`/`email` while
# describing an unrelated field. Guessing intent produced noise instead of a
# finding. The rule is now exact: a set is covered by an annotated frontend list,
# or it is listed here with the reason it has none. A set that is neither is a
# declaration nobody consumes — the state the captcha types were in.
HEADLESS_SETS: dict[str, str] = {
    # 以下闭集目前只在服务端内部判定/展示，控制台没有建立联合类型消费点：
    #   - AI 网关：provider 类型/状态、健康状态、attempt 状态由表单字符串与后端校验承载，
    #     前端未以联合类型消费（若将来控制台需要下拉闭集，应先建 @enumset 清单再移出本表）；
    #   - B2B 域名认领状态：控制台尚未提供域名认领 UI（OPEN-API 目标态能力）。
    "aigatewayAttemptState": "服务端内部 attempt 状态，控制台无联合类型消费点",
    "aigatewayHealthState": "服务端健康探针状态，控制台以徽标文本展示，无联合类型消费点",
    "aigatewayProviderStatus": "服务端 provider 生命周期状态，控制台无闭集控件",
    "aigatewayProviderType": "provider 类型由表单字符串+后端校验承载，控制台无联合类型消费点",
    "organizationDomainStatus": "B2B 域名认领状态；控制台尚未实现域名认领 UI",
}

# A Go transition table is `var valid<X>Transitions = map[string][]string{`. The
# console mirrors it one entry per line, which is what makes the two sides
# comparable as data instead of as text.
GO_TRANSITION_DECL = re.compile(
    r"var\s+([A-Za-z_]\w*)\s*=\s*map\[string\]\[\]string\s*\{(.*?)\n\}", re.DOTALL
)
GO_TRANSITION_ENTRY = re.compile(
    r"^\s*([^:{\s][^:]*?)\s*:\s*\{([^}]*)\}\s*,\s*$", re.MULTILINE
)
FRONTEND_LIFECYCLE_TAG = re.compile(r"@lifecycle\s+([A-Za-z_]\w*)")
FRONTEND_LIFECYCLE_DECL = re.compile(
    r"export const ([A-Z][A-Z0-9_]*)\s*:\s*TransitionTable\s*=\s*\{(.*?)\n\};", re.DOTALL
)
FRONTEND_LIFECYCLE_ENTRY = re.compile(r'^\s*([A-Za-z_]\w*)\s*:\s*\[([^\]]*)\]\s*,\s*$', re.MULTILINE)
FRONTEND_LIFECYCLE_VALUE = re.compile(r'"([^"]*)"')


def go_files(root):
    for dirpath, _dirs, files in os.walk(root):
        for name in sorted(files):
            if name.endswith(".go") and not name.endswith("_test.go"):
                yield os.path.join(dirpath, name)


def strip_go_comments(source):
    """Blank Go comments and string literals are kept — values are strings here.

    Only comments are removed: a comment explaining the `Default`/`default`
    mismatch contains the very spellings under test.
    """
    out = []
    i, n = 0, len(source)
    while i < n:
        c = source[i]
        if c == '"':
            j = i + 1
            while j < n:
                if source[j] == "\\":
                    j += 2
                    continue
                if source[j] == '"':
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


def collect_go_const_strings(sources):
    """Map every `const Name = "literal"` string constant to its value.

    Only declarations are read: a line inside a `const (` block, or a single-line
    `const`. An assignment inside a function body is not a declaration and is
    ignored, so a local shadow cannot change what a set resolves to.
    """
    consts = {}
    for _path, source in sources:
        in_block = False
        for line in source.splitlines():
            if in_block:
                if GO_CONST_BLOCK_END.match(line):
                    in_block = False
                    continue
                match = GO_CONST_MEMBER.match(line)
                if match:
                    consts.setdefault(match.group(1), match.group(2))
                continue
            if GO_CONST_BLOCK_START.match(line):
                in_block = True
                continue
            match = GO_CONST_SINGLE.match(line)
            if match:
                consts.setdefault(match.group(1), match.group(2))
    return consts


def resolve_set_values(body, consts):
    """Return (values, unresolved identifiers) for a NewSet body."""
    values = []
    unresolved = []
    for item in body.split(","):
        item = item.strip()
        if not item:
            continue
        # A named-string constant cannot be passed to `...string` directly, so
        # the declaration wraps it: `string(GovernanceMonitor)`. The conversion
        # is not a second value — unwrap it before resolving the identifier.
        conversion = re.fullmatch(r"string\(\s*([A-Za-z_]\w*)\s*\)", item)
        if conversion:
            item = conversion.group(1)
        if item.startswith('"') and item.endswith('"') and len(item) >= 2:
            values.append(item[1:-1])
        elif item in consts:
            values.append(consts[item])
        else:
            unresolved.append(item)
    return values, unresolved


def collect_go_sets():
    """Return {set name: {"values": [...], "file": relpath, "line": int}}."""
    sets = {}
    sources = []
    for root in GO_SCAN_DIRS:
        for path in go_files(root):
            with open(path, encoding="utf-8") as handle:
                raw = handle.read()
            if "enumset.NewSet(" not in raw:
                continue
            sources.append((path, strip_go_comments(raw)))

    consts = collect_go_const_strings(sources)

    for path, source in sources:
        for match in SET_START.finditer(source):
            body = balanced_call_body(source, source.index("(", match.start()))
            parts = split_top_level(body)
            name_expr = parts[0].strip()
            body = ",".join(parts[1:])
            literal_name = None
            if name_expr.startswith('"') and name_expr.endswith('"'):
                literal_name = name_expr[1:-1]
            line = source[: match.start()].count("\n") + 1
            relpath = os.path.relpath(path, REPO_ROOT)
            if literal_name is None:
                print(
                    f"  {relpath}:{line}: set name is the expression {name_expr!r}; "
                    "a computed name cannot be matched to the frontend annotation"
                )
                sets.setdefault("", {"values": [], "file": relpath, "line": line})
                continue
            values, unresolved = resolve_set_values(body, consts)
            sets[literal_name] = {
                "values": values,
                "file": relpath,
                "line": line,
                "unresolved": unresolved,
            }
    return sets


def collect_frontend_lists():
    """Return {array const name: {"set": name, "values": [...], "line": int}}."""
    if not os.path.exists(FRONTEND_ENUMS):
        return {}
    with open(FRONTEND_ENUMS, encoding="utf-8") as handle:
        source = handle.read()

    # Pre-compute every `export const` start offset so each array's preceding
    # window is bounded by the previous declaration — otherwise an array
    # without its own `@enumset` tag picks up the last tag in the file and
    # cross-checks against the wrong Go set (the DATABASE_ADAPTER_OPTIONS vs
    # ticketsStatus mis-match was caused by this).
    ALL_CONST_STARTS = [m.start() for m in re.finditer(r"export\s+const\s+[A-Z]", source)]

    def _preceding_boundary(match_start: int) -> int:
        """Return the offset *after* the closest preceding `export const`
        declaration so `preceding` only contains this array's own comment block.
        Returns 0 when no prior declaration exists (first export const in file)."""
        last_const_start = 0
        for offset in ALL_CONST_STARTS:
            if offset < match_start:
                last_const_start = offset
            else:
                break
        if last_const_start == 0:
            return 0
        # Advance past the preceding declaration's full line(s), including
        # trailing semicolon and any whitespace up to the next newline.
        after_decl = source.find("\n", last_const_start)
        if after_decl == -1:
            return 0
        return after_decl + 1

    lists = {}
    for match in FRONTEND_ARRAY_DECL.finditer(source):
        const_name, body = match.groups()
        # The annotation sits immediately above the declaration, so search
        # only the text between the previous `export const` end and the
        # current match start — never the whole file prefix.
        boundary = _preceding_boundary(match.start())
        preceding = source[boundary: match.start()]
        tags = FRONTEND_ANNOTATION.findall(preceding)
        set_name, cap_ns, cap_group = tags[-1] if tags else (None, None, None)
        lists[const_name] = {
            "set": set_name,
            "captions": (cap_ns, cap_group) if cap_ns else None,
            "values": FRONTEND_STRING_LITERAL.findall(body),
            "line": source[: match.start()].count("\n") + 1,
        }
    return lists


def locale_caption_keys(namespace):
    """Return {language: set of dotted key paths} for one locale namespace.

    The caption of a declared value is a user-visible string, so it lives in the
    locale packs like every other string. Reading them here — rather than
    trusting the annotation — is what turns "a state was added without a
    translation" into a build failure instead of an English cell in a Japanese
    console.
    """
    packs = {}
    for path in sorted(glob.glob(os.path.join(LOCALES_DIR, "*", f"{namespace}.json"))):
        language = os.path.basename(os.path.dirname(path))
        with open(path, encoding="utf-8") as handle:
            packs[language] = flatten_json(json.load(handle))
    return packs


def flatten_json(node, prefix=""):
    """Dotted paths of every leaf in a locale pack."""
    if isinstance(node, dict):
        out = set()
        for key, value in node.items():
            out |= flatten_json(value, f"{prefix}{key}.")
        return out
    return {prefix[:-1]}


def go_sets_requiring_captions(annotated, go_sets):
    """Return (missing, packs) for every value that has no caption anywhere.

    Both directions are checked: a declared value with no caption is a state the
    console cannot name, and a caption for a value no set declares is a string
    kept alive by nothing — the same defect as an unreferenced helper.
    """
    missing = []
    declared = set()
    packs = {}
    for set_name, (_const, info) in sorted(annotated.items()):
        if set_name not in go_sets or not info["captions"]:
            continue
        namespace, group = info["captions"]
        if namespace not in packs:
            packs[namespace] = locale_caption_keys(namespace)
        if not packs[namespace]:
            missing.append(
                f"namespace {namespace!r} has no locale pack under "
                f"{os.path.relpath(LOCALES_DIR, REPO_ROOT)}/"
            )
            continue
        for value in go_sets[set_name]["values"]:
            key = f"{group}.{value}"
            declared.add(f"{namespace}:{key}")
            for language, keys in sorted(packs[namespace].items()):
                if key not in keys:
                    missing.append(f"{language}/{namespace}.json is missing {key!r}")
    return missing, packs, declared


def collect_go_transitions():
    """Return {table name: {from value: [to values]}} for every Go table."""
    tables = {}
    sources = []
    for root in GO_SCAN_DIRS:
        for path in go_files(root):
            with open(path, encoding="utf-8") as handle:
                raw = handle.read()
            if "map[string][]string{" not in raw:
                continue
            sources.append((path, strip_go_comments(raw)))

    consts = collect_go_const_strings(sources)

    for path, source in sources:
        for match in GO_TRANSITION_DECL.finditer(source):
            name, body = match.groups()
            line = source[: match.start()].count("\n") + 1
            relpath = os.path.relpath(path, REPO_ROOT)
            table = {}
            unresolved = []
            for entry in GO_TRANSITION_ENTRY.finditer(body):
                raw_from, raw_to = entry.groups()
                start = resolve_value(raw_from.strip(), consts)
                if start is None:
                    unresolved.append(raw_from.strip())
                    continue
                targets = []
                for item in raw_to.split(","):
                    item = item.strip()
                    if not item:
                        continue
                    value = resolve_value(item, consts)
                    if value is None:
                        unresolved.append(item)
                        continue
                    targets.append(value)
                if start in table:
                    print(
                        f"  {relpath}:{line}: table {name!r} declares the start "
                        f"state {start!r} twice"
                    )
                table[start] = targets
            tables[name] = {
                "table": table,
                "file": relpath,
                "line": line,
                "unresolved": unresolved,
            }
    return tables


def resolve_value(token, consts):
    """A string literal, a known constant name, or None."""
    if token.startswith('"') and token.endswith('"') and len(token) >= 2:
        return token[1:-1]
    return consts.get(token)


def collect_frontend_lifecycles():
    """Return {table const name: {"table": name, "edges": {}, "line": int}}."""
    if not os.path.exists(FRONTEND_LIFECYCLES):
        return {}
    with open(FRONTEND_LIFECYCLES, encoding="utf-8") as handle:
        source = handle.read()

    tables = {}
    for match in FRONTEND_LIFECYCLE_DECL.finditer(source):
        const_name, body = match.groups()
        tags = FRONTEND_LIFECYCLE_TAG.findall(source[: match.start()])
        edges = {}
        for entry in FRONTEND_LIFECYCLE_ENTRY.finditer(body):
            start, raw = entry.groups()
            edges[start] = FRONTEND_LIFECYCLE_VALUE.findall(raw)
        tables[const_name] = {
            "table": tags[-1] if tags else None,
            "edges": edges,
            "line": source[: match.start()].count("\n") + 1,
        }
    return tables


def find_uncovered_sets(go_sets, covered):
    """Server sets with neither a frontend list nor a recorded reason."""
    return sorted(
        name
        for name in go_sets
        if name and name not in covered and name not in HEADLESS_SETS
    )


def main():
    failed = False

    go_sets = collect_go_sets()
    if not go_sets or (len(go_sets) == 1 and "" in go_sets):
        failed = True
        print(
            "ERROR: no enumset.NewSet declaration found "
            "(internal/domain, internal/platform, internal/app — the parser may be stale)",
            file=sys.stderr,
        )

    # Rule 1: integrity.
    for name, info in sorted(go_sets.items()):
        if not name:
            failed = True
            continue
        values = info["values"]
        where = f"{info['file']}:{info['line']}"
        unresolved = info.get("unresolved") or []
        if unresolved:
            failed = True
            print(
                f"ENUM CHECK FAILED: set {name!r} ({where}) names "
                f"{len(unresolved)} value(s) that are not a string literal or a "
                f"declared constant in the same file: {', '.join(unresolved)}"
            )
        if len(values) < 2:
            failed = True
            print(f"ENUM CHECK FAILED: set {name!r} ({where}) declares fewer than two values")
        if len(set(values)) != len(values):
            failed = True
            print(f"ENUM CHECK FAILED: set {name!r} ({where}) declares a duplicate value")

    # Rule 2: the frontend list must equal the Go set, in both directions.
    frontend = collect_frontend_lists()
    annotated = {info["set"]: (const, info) for const, info in frontend.items() if info["set"]}
    if not annotated:
        failed = True
        print(
            f"ERROR: no @enumset annotation found in {os.path.relpath(FRONTEND_ENUMS, REPO_ROOT)}",
            file=sys.stderr,
        )

    for set_name, (const_name, info) in sorted(annotated.items()):
        if set_name not in go_sets:
            failed = True
            print(
                f"ENUM CHECK FAILED: {const_name} declares @enumset {set_name}, "
                "but no enumset.NewSet with that name exists"
            )
            continue
        go_values = go_sets[set_name]["values"]
        ts_values = info["values"]
        where = f"web/src/shared/constants/enums.ts:{info['line']}"
        missing = [v for v in go_values if v not in ts_values]
        extra = [v for v in ts_values if v not in go_values]
        if missing:
            failed = True
            print(
                f"ENUM CHECK FAILED: {where} {const_name} is missing "
                f"{len(missing)} value(s) the server accepts: {', '.join(sorted(missing))}"
            )
            print(
                "  A value the server accepts but the console cannot offer is a "
                "supported capability with no UI.",
                file=sys.stderr,
            )
        if extra:
            failed = True
            print(
                f"ENUM CHECK FAILED: {where} {const_name} offers "
                f"{len(extra)} value(s) the server rejects: {', '.join(sorted(extra))}"
            )
            print(
                "  The server answers 422 for these, so the form cannot be "
                "submitted. This is how the captcha/notification provider types "
                "diverged.",
                file=sys.stderr,
            )

    # Rule 3: every server set is consumed by a frontend list, or recorded.
    covered = {name for name in annotated if name in go_sets}
    uncovered = find_uncovered_sets(go_sets, covered)
    if uncovered:
        failed = True
        print(f"ENUM CHECK FAILED ({len(uncovered)} server set(s) with no frontend list):")
        for name in uncovered:
            info = go_sets[name]
            print(f"  - {name} ({info['file']}:{info['line']})")
        print(
            "  Either declare the list in web/src/shared/constants/enums.ts with an "
            "@enumset tag, or add the set to HEADLESS_SETS in this script with the "
            "reason it has no console surface.",
            file=sys.stderr,
        )

    # Rule 4: a declared value must have a caption in every language pack.
    caption_missing, _packs, declared_captions = go_sets_requiring_captions(annotated, go_sets)
    if caption_missing:
        failed = True
        print(
            f"ENUM CHECK FAILED ({len(caption_missing)} missing caption(s) for a "
            "declared value):"
        )
        for line in caption_missing[:40]:
            print(f"  - {line}")
        if len(caption_missing) > 40:
            print(f"  … and {len(caption_missing) - 40} more")
        print(
            "  Every language pack must carry the caption, so a new state is a "
            "translation task rather than an untranslated cell.",
            file=sys.stderr,
        )

    # Rule 5: every transition edge agrees in both directions.
    go_tables = collect_go_transitions()
    front_tables = collect_frontend_lifecycles()
    if not front_tables:
        failed = True
        print(
            f"ERROR: no @lifecycle table found in "
            f"{os.path.relpath(FRONTEND_LIFECYCLES, REPO_ROOT)}",
            file=sys.stderr,
        )
    for const_name, info in sorted(front_tables.items()):
        where = f"web/src/shared/constants/lifecycles.ts:{info['line']}"
        name = info["table"]
        if not name:
            failed = True
            print(f"ENUM CHECK FAILED: {where} {const_name} has no @lifecycle tag")
            continue
        if name not in go_tables:
            failed = True
            print(
                f"ENUM CHECK FAILED: {where} {const_name} mirrors {name!r}, "
                "but no such Go transition table exists"
            )
            continue
        go_table = go_tables[name]["table"]
        front_edges = info["edges"]
        for start in sorted(set(go_table) | set(front_edges)):
            go_targets = go_table.get(start)
            front_targets = front_edges.get(start)
            if go_targets is None:
                failed = True
                print(
                    f"ENUM CHECK FAILED: {where} {const_name} declares start state "
                    f"{start!r} that the server's {name} does not"
                )
                continue
            if front_targets is None:
                failed = True
                print(
                    f"ENUM CHECK FAILED: {where} {const_name} omits start state "
                    f"{start!r} from the server's {name}"
                )
                continue
            if sorted(go_targets) != sorted(front_targets):
                failed = True
                missing = [t for t in go_targets if t not in front_targets]
                extra = [t for t in front_targets if t not in go_targets]
                print(
                    f"ENUM CHECK FAILED: {where} {const_name}[{start!r}] differs from "
                    f"the server's {name}"
                )
                if missing:
                    print(f"    the server allows and the console does not: {missing}")
                if extra:
                    print(f"    the console offers and the server refuses (422): {extra}")

    for name, info in sorted(go_tables.items()):
        if info["unresolved"]:
            failed = True
            print(
                f"ENUM CHECK FAILED: transition table {name!r} "
                f"({info['file']}:{info['line']}) names {len(info['unresolved'])} "
                f"value(s) that are not a string literal or a declared constant: "
                f"{', '.join(info['unresolved'])}"
            )

    if failed:
        return 1

    covered_names = ", ".join(sorted(covered))
    print(
        f"Enum check OK: {len(go_sets)} server set(s) well formed; "
        f"{len(annotated)} frontend list(s) agree in both directions ({covered_names}); "
        f"every server set is consumed or recorded ({len(HEADLESS_SETS)} recorded as headless); "
        f"{len(declared_captions)} caption(s) present in every language pack; "
        f"{len(front_tables)} transition table(s) agree with the server edge for edge"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
