#!/usr/bin/env python3
"""Reject semantic validation failures that answer 400 instead of 422.

Why this exists
---------------
规范 4.14.1 divides the two codes by *what the caller can do*:

  * 400 `invalid_argument` — the request could not be parsed or a required
    field was omitted. Retrying the same body cannot succeed.
  * 422 `unprocessable_entity` — the request parsed and bound, but a rule
    rejected it: bad RFC3339/UUID, an enum value outside the closed set, a
    cross-field contradiction, an illegal state transition, a value out of
    range. The caller succeeds by changing the field.

The application layer used `NewInvalidArgument` for both, so every "status must
be active or disabled" or "payment is in state X" arrived as a 400. A client
cannot tell "your JSON is broken" (do not retry) from "change `status`" (retry
after the change), and the console cannot attach the message to a field.

`internal/platform/enumset` already answers 422 with
`{field, value, allowed, set}`; the range/format/cross-field rules use
`NewWithMeta(CodeUnprocessableEntity, …)`. What was missing is a gate: a new
semantic check written next to an old missing-field check is invisible until a
user hits it.

Rules
-----
  R1. A call to `apperrors.NewInvalidArgument` / `NewInvalidArgumentf`, or
      `apperrors.Newf(CodeInvalidArgument, …)`, whose argument expression
      carries a semantic marker (`must …`, `unsupported …`, `cannot advance`,
      `is in state`, `cannot transition`) is a semantic validation site.
      `must not be blank` is excluded: a blank value in a PATCH body is the
      caller omitting a field (400, 手册 4.14.1).
  R2. Every semantic site must use `CodeUnprocessableEntity`
      (`NewWithMeta(CodeUnprocessableEntity, …)` / `NewUnprocessableEntity*`),
      or be listed in `PENDING` with the batch that will convert it.
  R3. `PENDING` must match the source exactly: an entry whose site no longer
      exists is a FAIL. A backlog that names code which is gone is worse than
      no backlog — it reports progress that never happened.

Scope: `internal/app/**` and `internal/domain/**`. In the application layer
`auth/` (protocol-face error codes are fixed by RFC 6749/6750 and the OAuth
error writer), `scim/` (RFC 7644 error body) and `system/` (CLI argument
validation, not an HTTP surface) stay excluded. The domain layer was added in
E1d4; its packages named auth/scim are ordinary domain rule checks and are
scanned.

Usage: python3 scripts/check-error-semantics.py
Exit code 0 = every semantic site answers 422 or is registered in PENDING,
1 = an unregistered site or a stale entry was found.

输入：internal/app/ 与 internal/domain/ 的非测试 Go 文件
输出：PASS/FAIL + 未登记语义站点 / 失效 PENDING 条目 + pending 计数
--check 行为：无 --check；失败即 exit 1
消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
"""

import os
import re
import sys

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SCAN_DIRS = [
    os.path.join(REPO_ROOT, "internal/app"),
    os.path.join(REPO_ROOT, "internal/domain"),
]

# Directories whose error codes are not governed by 4.14.1. Each needs a
# reason, because "excluded" without a reason is how a blind spot starts.
EXCLUDED = {
    "auth": "协议面错误码由 RFC 6749/6750 与 WriteOAuthError 决定",
    "scim": "SCIM 错误体由 RFC 7644 决定（WriteSCIMError）",
    "system": "CLI 参数校验，不经过 HTTP 错误映射",
}

CALL_START = re.compile(r"apperrors\.NewInvalidArgument(?:f)?\(")

# The same defect also hides in `NewWithMeta(CodeInvalidArgument, …)`: a check
# that attaches `{field: …}` metadata but still answers 400. Matching only the
# constructor call would report a clean run while these sites stayed 400.
WITH_META_START = re.compile(r"NewWithMeta\(\s*(?:apperrors\.)?CodeInvalidArgument\s*,")

# `apperrors.Newf(CodeInvalidArgument, …)` builds the same 400 with a format
# string. Without this pattern region.go-style range checks are invisible.
NEWF_WITH_CODE = re.compile(r"apperrors\.Newf\(\s*(?:apperrors\.)?CodeInvalidArgument\s*,")

# Liveness probe used once the 400 backlog reaches zero: the same marker set
# must still find semantic sites that now answer 422. Zero matches on both
# sides means the marker list, not the code, went stale.
WITH_META_UNPROCESSABLE = re.compile(r"NewWithMeta\(\s*(?:apperrors\.)?CodeUnprocessableEntity\s*,")

# A semantic marker inside the call expression. Matching the whole expression
# (not just the first literal) also catches dynamically built messages such as
# `field + " must be a UUID"`.
#
# The last five cover the state/precondition and cross-field shapes that the
# first set missed: `payment is in state X, cannot refund`, `refund amount
# exceeds available`, `currency does not match the catalog currency`, `product
# is not available for purchase`, `insufficient product quantity`. They are the
# same class as the `must` rules — the caller succeeds by changing one field.
SEMANTIC_MARKERS = [
    re.compile(r"\bmust\b"),
    re.compile(r"unsupported "),
    re.compile(r"cannot advance"),
    re.compile(r"is in state"),
    re.compile(r"cannot transition"),
    re.compile(r"cannot "),
    re.compile(r"exceeds "),
    re.compile(r"does not match"),
    re.compile(r"not available"),
    re.compile(r"insufficient "),
]

# Closed-set values are the other semantic shape: "invalid ticket status: %s",
# "unknown sovereignty mode: %s", a length ceiling. These markers stay precise
# on purpose: a generic `invalid `/`unknown ` would also match credential
# failures ("refresh token is invalid or expired", 401), unparseable bodies
# ("invalid json", 400) and lookups ("unknown organization", 404), none of
# which belong in 422.
DOMAIN_CLOSED_SET_MARKERS = [
    re.compile(r"invalid [^:]+: %s"),
    re.compile(r"invalid session state:"),
    re.compile(r"too long: %d >"),
    re.compile(r"unknown .*(mode|scope): %s"),
    re.compile(r"illegal [^:]*transition:"),
]

# The one "must" that is a missing-field case, not a semantic one. It is
# removed before the marker scan so a message can still use the phrase.
MISSING_PHRASE = "must not be blank"

STRING_LITERAL = re.compile(r'"([^"\\]*(?:\\.[^"\\]*)*)"')


def call_arguments(source, open_paren):
    """Return the text between the call's parentheses (strings aware)."""
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


def message_argument(arguments, index):
    """Return the argument that carries the message text.

    Only that argument is scanned for markers and used as the PENDING key: a
    metadata map contains string literals too (`{"field": "language"}`), and
    folding them into the message would make the key change whenever the
    metadata does.
    """
    parts = split_top_level(arguments)
    return parts[index] if index < len(parts) else arguments


def normalized_message(arguments):
    """Message key for a site: its string literals, whitespace normalised.

    The literal set is what a reviewer edits, and it survives line-number
    drift, which a `path:line` key does not.
    """
    literals = [m.group(1) for m in STRING_LITERAL.finditer(arguments)]
    message = " ".join(literals)
    return re.sub(r"\s+", " ", message).strip()


def is_semantic(arguments):
    text = arguments.replace(MISSING_PHRASE, "")
    return any(marker.search(text) for marker in SEMANTIC_MARKERS) or any(
        marker.search(text) for marker in DOMAIN_CLOSED_SET_MARKERS
    )


# Unregistered semantic sites, keyed by "relative path|normalised message".
#
# Each value names the batch that will convert it. The entries are removed in
# the same commit that converts the site, so the count only ever goes down
# (R3 fails on a stale entry). Scope decisions live in the module docstring.
#
# The same message repeated in one file is one entry: it is one rule that
# happens to sit at several call sites, and converting one without the others
# is not a state worth distinguishing. The site count in the PASS line is the
# real number.
#
# Batch map:
#   E1a — 已完成（risk）
#   E1b — 已完成（payments / orders / entries）
#   E1c — 已完成（providers）
#   E1d — the rest, then extend the scan to internal/domain
# E1d3 converted the last application-layer site, so the backlog is empty.
# The scan stays in place: a new `NewInvalidArgument` carrying a semantic marker
# fails immediately instead of silently re-growing an unreported 400 surface.
PENDING: dict[str, str] = {}


def go_files(root):
    for dirpath, _dirs, files in os.walk(root):
        for name in sorted(files):
            if name.endswith(".go") and not name.endswith("_test.go"):
                yield os.path.join(dirpath, name)


def scanned_files():
    """Yield (relpath, path) for every Go file in scope.

    EXCLUDED applies to the application layer only: a domain package named
    auth/scim still carries ordinary domain rule checks.
    """
    for root in SCAN_DIRS:
        for path in go_files(root):
            rel = os.path.relpath(path, REPO_ROOT)
            parts = rel.split(os.sep)
            if len(parts) > 2 and parts[1] == "app" and parts[2] in EXCLUDED:
                continue
            yield rel, path


def site_patterns():
    return ((CALL_START, 0), (WITH_META_START, 1), (NEWF_WITH_CODE, 1))


def scan():
    """Return [(relpath, line, message)] for every semantic site."""
    sites = []
    for rel, path in scanned_files():
        with open(path, encoding="utf-8") as handle:
            source = handle.read()
        for pattern, message_index in site_patterns():
            for match in pattern.finditer(source):
                arguments = call_arguments(source, source.index("(", match.start()))
                message = message_argument(arguments, message_index)
                if not is_semantic(message):
                    continue
                line = source[: match.start()].count("\n") + 1
                sites.append((rel, line, normalized_message(message)))
    return sites


# =============================================================================
def semantic_unprocessable_exists():
    """True when a semantic marker still matches an existing 422 site.

    Once every 400 site is converted the raw site list is empty by design, so
    the old "zero sites means stale scanner" guard would fire forever. This
    probe keeps that guard meaningful: it only passes when the marker set still
    describes real 422 code.
    """
    for rel, path in scanned_files():
        with open(path, encoding="utf-8") as handle:
            source = handle.read()
        for match in WITH_META_UNPROCESSABLE.finditer(source):
            arguments = call_arguments(source, source.index("(", match.start()))
            if is_semantic(message_argument(arguments, 1)):
                return True
    return False


def main():
    sites = scan()
    if not sites and not PENDING:
        if not semantic_unprocessable_exists():
            print(
                "ERROR: no semantic validation site found in internal/app — the "
                "scanner is stale (the markers no longer match the codebase)",
                file=sys.stderr,
            )
            return 1
        print(
            "Error-semantics check OK: no semantic invalid_argument site remains "
            "in internal/{app,domain}; pending 0"
        )
        return 0
    if not sites:
        print(
            "ERROR: no semantic validation site found in internal/app — the "
            "scanner is stale (the markers no longer match the codebase)",
            file=sys.stderr,
        )
        return 1

    seen = set()
    violations = []
    pending_hits = {}
    for rel, line, message in sites:
        key = rel + "|" + message
        seen.add(key)
        if key in PENDING:
            pending_hits.setdefault(PENDING[key], 0)
            pending_hits[PENDING[key]] += 1
            continue
        violations.append((rel, line, message))

    stale = sorted(key for key in PENDING if key not in seen)

    if violations:
        print(
            f"Error-semantics check FAILED ({len(violations)} semantic "
            "invalid_argument site(s) answering 400):"
        )
        for rel, line, message in violations:
            print(f"  - {rel}:{line}: {message!r}")
        print(
            "  A request that parsed and bound but violated a rule is 422 "
            "unprocessable_entity (手册 4.14.1): the caller succeeds by changing "
            "one field. Use apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "
            "…, meta) with {field, value, allowed|max|min|format|expected}, or "
            "enumset.Set.Validate* for a closed set (it adds {field,value,allowed,set}). "
            "Only a missing field or an unparseable body stays 400.",
            file=sys.stderr,
        )

    if stale:
        print(
            f"Error-semantics check FAILED ({len(stale)} stale PENDING entry/entries):"
        )
        for key in stale:
            print(f"  - {key} ({PENDING[key]})")
        print(
            "  The site no longer exists or its message changed. Remove the entry "
            "in the same commit that converts or rewrites the site — a backlog that "
            "names code which is gone reports progress that never happened.",
            file=sys.stderr,
        )

    if violations or stale:
        return 1

    pending_total = sum(pending_hits.values())
    batches = ", ".join(f"{batch} {count}" for batch, count in sorted(pending_hits.items()))
    detail = f"pending {pending_total}" + (f" ({batches})" if batches else "")
    print(
        "Error-semantics check OK: no unregistered semantic validation site "
        f"answers 400; {detail}"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
