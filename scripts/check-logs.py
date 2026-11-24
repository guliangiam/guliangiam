#!/usr/bin/env python3
"""Static log red-line scanner (E3-b).

Rules (architecture decision for 规范 12.4/12.10):

1. Log attribute keys must be snake_case: no camelCase (uppercase) key literal.
2. Sensitive keys are forbidden: password / token / secret / idCard /
   authorization / cookie / credential / API key / private key / keytab.
3. Full request/response body keys are forbidden (body / request_body /
   raw_body / payload). Size/byte/length companions are allowed.
4. Exact-key exemptions: ``SENSITIVE_KEY_EXEMPT`` lists keys whose logged
   value is a boolean, counter, enum or file path — never credential material
   itself. The allowlist matches the whole key literal (no substring match) and
   only suppresses rule 2; rules 1 and 3 still apply. Adding an entry requires a
   reviewed value expression and a one-line justification comment.

The scanner only inspects production Go files (``*_test.go`` is skipped unless
``--include-tests`` is passed). It is deliberately a script, not a CI gate: the
current tree still has legacy camelCase audit keys, so a whole-repo run reports
that pre-existing backlog instead of blocking unrelated changes.

Usage:
    python3 scripts/check-logs.py
    python3 scripts/check-logs.py --paths internal/platform/logging/logging.go \
        internal/platform/middleware/access_log.go
    python3 scripts/check-logs.py --include-tests
"""

import argparse
import os
import re
import sys

DEFAULT_PATHS = ("internal", "cmd")
SKIP_DIRS = {".git", "vendor", "node_modules", "testdata", "__pycache__"}

LOG_CALL_RE = re.compile(
    r"\b(?:(?:slog|logger|log)\."
    r"(?:Debug|Info|Warn|Error|Log|LogAttrs)(?:Context)?|SlogAudit)\s*\("
)
ATTR_CALL_RE = re.compile(
    r"\bslog\.(?:String|Int|Int64|Uint64|Float64|Bool|Time|Duration|Any|Group)\s*\("
)
WITH_CALL_RE = re.compile(r"\b(?:logger|log|slog)\.With\s*\(")

SENSITIVE_ROOTS = (
    "password",
    "token",
    "secret",
    "idcard",
    "authorization",
    "cookie",
    "credential",
    "apikey",
    "privatekey",
    "keytab",
    "sessionsecret",
    "bindpassword",
    "recoverycode",
)
FULL_BODY_KEYS = {
    "body",
    "requestbody",
    "responsebody",
    "rawbody",
    "fullbody",
    "payload",
    "requestpayload",
}
# Exact key exemptions for the sensitive-root rule (rule 4). Matching is on the
# full key literal, never a substring: camelCase or renamed variants still fail.
SENSITIVE_KEY_EXEMPT = {
    "admin_credential",  # value: bool (admin password hash present), not the hash
    "access_secret_rotated",  # value: bool (rotation requested), not the secret
    "keytab_path",  # value: keytab file path string, not keytab contents
    "password_changed",  # value: bool (password supplied), not the password
}
SIZE_SUFFIXES = ("size", "bytes", "length", "count")


def strip_go_comments(src):
    """Replace // and /* */ comments with spaces, preserving newlines."""
    out = []
    i = 0
    in_string = None
    escape = False
    while i < len(src):
        ch = src[i]
        nxt = src[i + 1] if i + 1 < len(src) else ""
        if in_string:
            out.append(ch)
            if in_string == "`":
                if ch == "`":
                    in_string = None
            else:
                if escape:
                    escape = False
                elif ch == "\\":
                    escape = True
                elif ch == in_string:
                    in_string = None
            i += 1
            continue
        if ch in ('"', "'", "`"):
            in_string = ch
            out.append(ch)
            i += 1
            continue
        if ch == "/" and nxt == "/":
            while i < len(src) and src[i] != "\n":
                out.append(" ")
                i += 1
            continue
        if ch == "/" and nxt == "*":
            out.append(" ")
            out.append(" ")
            i += 2
            while i < len(src):
                if src[i] == "*" and i + 1 < len(src) and src[i + 1] == "/":
                    out.append(" ")
                    out.append(" ")
                    i += 2
                    break
                out.append("\n" if src[i] == "\n" else " ")
                i += 1
            continue
        out.append(ch)
        i += 1
    return "".join(out)


def read_call_args(src, open_index):
    """Return (top-level args, closing index) for the call at open_index."""
    depth = 0
    args = []
    current = []
    i = open_index
    in_string = None
    escape = False
    while i < len(src):
        ch = src[i]
        if in_string:
            current.append(ch)
            if in_string == "`":
                if ch == "`":
                    in_string = None
            else:
                if escape:
                    escape = False
                elif ch == "\\":
                    escape = True
                elif ch == in_string:
                    in_string = None
            i += 1
            continue
        if ch in ('"', "'", "`"):
            in_string = ch
            current.append(ch)
            i += 1
            continue
        if ch in "([{":
            depth += 1
            if depth > 1:
                current.append(ch)
            i += 1
            continue
        if ch in ")]}":
            depth -= 1
            if depth <= 0:
                args.append("".join(current))
                return args, i
            current.append(ch)
            i += 1
            continue
        if ch == "," and depth == 1:
            args.append("".join(current))
            current = []
            i += 1
            continue
        current.append(ch)
        i += 1
    return args, len(src) - 1


def string_literal(arg):
    arg = arg.strip()
    if len(arg) < 2:
        return None
    if arg[0] == "`" and arg[-1] == "`":
        return arg[1:-1]
    if arg[0] == '"' and arg[-1] == '"':
        inner = arg[1:-1]
        # Reject concatenations such as "a" + "b".
        escaped = False
        for ch in inner:
            if escaped:
                escaped = False
            elif ch == "\\":
                escaped = True
            elif ch == '"':
                return None
        return inner
    return None


def normalize_key(key):
    return re.sub(r"[^a-z0-9]", "", key.lower())


def classify_key(key):
    reasons = []
    if re.search(r"[A-Z]", key):
        reasons.append("snake_case")
    canonical = normalize_key(key)
    if key not in SENSITIVE_KEY_EXEMPT and any(root in canonical for root in SENSITIVE_ROOTS):
        reasons.append("sensitive")
    if canonical in FULL_BODY_KEYS and not canonical.endswith(SIZE_SUFFIXES):
        reasons.append("full_body")
    return reasons


def arg_start(name, args):
    if name in ("Log", "LogAttrs"):
        return 3
    if name == "SlogAudit":
        return 2
    if name.endswith("Context"):
        return 2
    return 1


def scan_log_calls(src):
    found = []
    for match in LOG_CALL_RE.finditer(src):
        open_index = src.find("(", match.start(), match.end())
        if open_index < 0:
            continue
        args, _ = read_call_args(src, open_index)
        name = match.group(0).strip().rstrip("(").split(".")[-1]
        start = arg_start(name, args)
        for i in range(start, len(args) - 1, 2):
            key = string_literal(args[i])
            if key is not None:
                found.append((match.start(), key))
    return found


def scan_attr_calls(src):
    found = []
    for match in ATTR_CALL_RE.finditer(src):
        open_index = src.find("(", match.start(), match.end())
        if open_index < 0:
            continue
        args, _ = read_call_args(src, open_index)
        if not args:
            continue
        key = string_literal(args[0])
        if key is not None:
            found.append((match.start(), key))
    return found


def scan_with_calls(src):
    found = []
    for match in WITH_CALL_RE.finditer(src):
        open_index = src.find("(", match.start(), match.end())
        if open_index < 0:
            continue
        args, _ = read_call_args(src, open_index)
        for i in range(0, len(args) - 1, 2):
            key = string_literal(args[i])
            if key is not None:
                found.append((match.start(), key))
    return found


def go_files(paths, include_tests):
    files = []
    for path in paths:
        if os.path.isfile(path):
            candidates = [path]
        else:
            candidates = []
            for root, dirs, names in os.walk(path):
                dirs[:] = sorted(d for d in dirs if d not in SKIP_DIRS)
                for name in sorted(names):
                    if not name.endswith(".go"):
                        continue
                    candidates.append(os.path.join(root, name))
        for candidate in candidates:
            if not include_tests and candidate.endswith("_test.go"):
                continue
            files.append(candidate)
    return sorted(set(files))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--paths",
        nargs="*",
        default=list(DEFAULT_PATHS),
        help="files or directories to scan (default: internal cmd)",
    )
    parser.add_argument(
        "--include-tests",
        action="store_true",
        help="also scan *_test.go (test fixtures may use invalid keys on purpose)",
    )
    args = parser.parse_args()

    violations = []
    checked_calls = 0
    files = go_files(args.paths, args.include_tests)
    for path in files:
        try:
            with open(path, "r", encoding="utf-8") as handle:
                raw = handle.read()
        except OSError as err:
            print("%s: cannot read: %s" % (path, err), file=sys.stderr)
            return 2
        src = strip_go_comments(raw)
        candidates = scan_log_calls(src) + scan_attr_calls(src) + scan_with_calls(src)
        checked_calls += len(candidates)
        seen = set()
        for offset, key in candidates:
            line = src.count("\n", 0, offset) + 1
            reasons = classify_key(key)
            if not reasons:
                continue
            marker = (line, key, tuple(reasons))
            if marker in seen:
                continue
            seen.add(marker)
            violations.append((path, line, key, reasons))

    for path, line, key, reasons in violations:
        print("%s:%d: key=%r rule=%s" % (path, line, key, ",".join(reasons)))

    print(
        "checked %d file(s), %d key literal(s), %d violation(s)"
        % (len(files), checked_calls, len(violations))
    )
    return 1 if violations else 0


if __name__ == "__main__":
    sys.exit(main())
