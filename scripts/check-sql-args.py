#!/usr/bin/env python3
"""Check that every SQL call passes exactly as many arguments as its statement binds.

Why this exists
---------------
`pgx` rejects an argument-count mismatch at run time:

    ERROR: wrong number of parameters for prepared statement "m"
    DETAIL: Expected 3 parameters but got 2.

Nothing static caught it. `check-sql-columns.py` compares *columns*, and
`check-sql-runtime.py` PREPAREs every statement — but PREPARE never sees the call
site, so a statement that binds three parameters and is called with two prepares
perfectly and fails on the first request. `//go:build integration` tests are not
run by default, so the defect reaches production.

That is not hypothetical. Adding the `is_enabled` predicate to `adapters` and
`models` updated the SQL in both the list and the COUNT statement but left the
COUNT call site passing two arguments instead of three. Both list endpoints then
failed on **every** request — the count query runs unconditionally after the row
scan — while all fifteen gates stayed green.

Scope: statements it can prove
------------------------------
Only statements whose right-hand side is a **single backtick literal** are
compared. Everything else — statements assembled by concatenation, or carrying a
named `{ORDER_BY}` slot — is counted as unresolved and skipped.

That is deliberately conservative. Resolving concatenation needs the whole
expression grammar, and a half-working resolver is worse than none: an earlier
attempt spliced SQL fragments into unrelated statements, because every multi-line
SQL literal contains lines that look like assignments (`       policy=$17,`), and
produced 174 false findings. A gate that noisy stops being read, and then it
cannot report the next real breakage either. The single-literal form still covers
the defect class above: both COUNT statements that broke are single literals.

A statement binding no parameters must be called with no arguments — pgx rejects
surplus arguments too.

Usage: python3 scripts/check-sql-args.py
Exit code 0 = every checked call site matches its statement, 1 = mismatch found.

输入：internal/infra/postgres/*.go 中 SQL 字面量 + 调用端
输出：PASS/FAIL + 语句/调用端参数不匹配
--check 行为：无 --check；失败即 exit 1
消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
"""

import os
import re
import sys

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
INFRA_DIR = os.path.join(REPO_ROOT, "internal/infra/postgres")

ASSIGNMENT = re.compile(r"(?m)^[ \t]*(\w+)[ \t]*=[ \t]*")
CALL = re.compile(r"\bdb\.(Query|QueryRow|Exec)\(\s*ctx\s*,\s*(\w+)")
BIND = re.compile(r"\$(\d+)")
# Slots substituted at the call site (`strings.Replace(query, "{ORDER_BY}", …)`).
NAMED_SLOT = re.compile(r"\{[A-Z_]+\}")


def literal_mask(source):
    """Mark, at matching offsets, which characters sit inside a string literal.

    Assignment detection must run on code, not on text: every multi-line SQL
    literal contains lines that look like assignments (`       policy=$17,`), and
    treating those as package constants is what corrupted the first resolver.

    Literal content is masked with `x` rather than a space for two reasons: a
    space placeholder would be swallowed by the `[ \\t]*` that follows `=` in the
    assignment pattern (pushing the match inside the literal), and a visible
    character keeps SQL lines — whose own `=` are masked away — from matching.
    The **delimiters themselves are kept**, so the assignment pattern stops on the
    opening backtick and the expression reader can start from it. Newlines are
    preserved so line numbers stay correct.
    """
    out = []
    i = 0
    n = len(source)
    while i < n:
        c = source[i]
        if c in "\"'`":
            quote = c
            out.append(quote)
            i += 1
            while i < n:
                if source[i] == "\\" and quote != "`":
                    out.append("x")
                    i += 1
                    if i < n:
                        out.append("x")
                        i += 1
                    continue
                if source[i] == quote:
                    out.append(quote)
                    i += 1
                    break
                out.append("\n" if source[i] == "\n" else "x")
                i += 1
            continue
        out.append(c)
        i += 1
    return "".join(out)


def single_literal(source, start):
    """Return a backtick-only right-hand side, or None when it is an expression.

    `start` is the offset just past the `=`. Only whitespace and the closing
    backtick may follow the literal on its own line, so anything joined with `+`
    — or produced by a call — is refused rather than guessed at.
    """
    i = start
    n = len(source)
    while i < n and source[i] in " \t":
        i += 1
    if i >= n or source[i] != "`":
        return None
    end = source.find("`", i + 1)
    if end == -1:
        return None
    tail = source[end + 1 :].split("\n", 1)[0]
    if tail.strip() != "":
        return None
    return source[i + 1 : end]


def collect_statements(package_files):
    """Return ({name: sql}, {name: reason}) split into provable and skipped."""
    simple = {}
    skipped = {}
    for path in package_files:
        with open(path, encoding="utf-8") as handle:
            source = handle.read()
        masked = literal_mask(source)
        for match in ASSIGNMENT.finditer(masked):
            name = match.group(1)
            if name in simple or name in skipped:
                continue
            body = single_literal(source, match.end())
            if body is None:
                skipped[name] = "assembled (concatenation or call)"
                continue
            simple[name] = body
    return simple, skipped


def call_arguments(source, open_paren):
    """Return the text between a call's parentheses, honouring nesting and strings."""
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


def strip_go_comments(text):
    """去掉 Go 行注释/块注释（跳过字符串/字符/反引号字面量内部），避免把尾注释当成实参。"""
    out = []
    i = 0
    n = len(text)
    while i < n:
        c = text[i]
        if c in (chr(39), chr(34), chr(96)):
            quote = c
            out.append(c)
            i += 1
            while i < n:
                out.append(text[i])
                if text[i] == chr(92) and quote != chr(96):
                    i += 1
                    if i < n:
                        out.append(text[i])
                elif text[i] == quote:
                    i += 1
                    break
                i += 1
            continue
        if c == "/" and i + 1 < n and text[i + 1] == "/":
            i += 2
            while i < n and text[i] != chr(10):
                i += 1
            continue
        if c == "/" and i + 1 < n and text[i + 1] == "*":
            i += 2
            while i + 1 < n and not (text[i] == "*" and text[i + 1] == "/"):
                i += 1
            i = min(i + 2, n)
            continue
        out.append(c)
        i += 1
    return "".join(out)


def split_arguments(source):
    """Split an argument list on top-level commas."""
    source = strip_go_comments(source)
    out = []
    depth = 0
    current = ""
    i = 0
    n = len(source)
    while i < n:
        c = source[i]
        if c in "\"'`":
            quote = c
            current += c
            i += 1
            while i < n:
                if source[i] == "\\":
                    current += source[i : i + 2]
                    i += 2
                    continue
                current += source[i]
                if source[i] == quote:
                    i += 1
                    break
                i += 1
            continue
        if c in "([{":
            depth += 1
        elif c in ")]}":
            depth -= 1
        elif c == "," and depth == 0:
            out.append(current.strip())
            current = ""
            i += 1
            continue
        current += c
        i += 1
    if current.strip():
        out.append(current.strip())
    return out


def go_files(root):
    for dirpath, _dirs, files in os.walk(root):
        for name in sorted(files):
            if name.endswith(".go") and not name.endswith("_test.go"):
                yield os.path.join(dirpath, name)


def main():
    checked = 0
    mismatches = []
    unresolved = 0
    seen = set()

    for path in go_files(INFRA_DIR):
        package_dir = os.path.dirname(path)
        if package_dir in seen:
            continue
        seen.add(package_dir)

        package_files = [
            os.path.join(package_dir, name)
            for name in sorted(os.listdir(package_dir))
            if name.endswith(".go") and not name.endswith("_test.go")
        ]
        simple, skipped = collect_statements(package_files)

        for candidate in package_files:
            with open(candidate, encoding="utf-8") as handle:
                source = handle.read()
            relpath = os.path.relpath(candidate, REPO_ROOT)
            for match in CALL.finditer(source):
                statement = match.group(2)
                if statement in skipped:
                    unresolved += 1
                    continue
                if statement not in simple:
                    continue

                sql = simple[statement]
                if NAMED_SLOT.search(sql):
                    unresolved += 1
                    continue

                open_paren = source.index("(", match.start())
                parts = split_arguments(call_arguments(source, open_paren))
                args = parts[2:] if len(parts) >= 2 else []

                bindings = [int(value) for value in BIND.findall(sql)]
                expected = max(bindings) if bindings else 0
                checked += 1

                if len(args) != expected:
                    line = source[: match.start()].count("\n") + 1
                    mismatches.append((relpath, line, statement, len(args), expected))

    if not checked:
        print(
            "ERROR: no SQL call site checked (the parser may be stale)",
            file=sys.stderr,
        )
        return 1

    if mismatches:
        print(
            f"SQL argument check FAILED ({len(mismatches)} call(s) passing the "
            "wrong number of arguments):"
        )
        for relpath, line, statement, got, wanted in sorted(mismatches):
            print(f"  - {relpath}:{line}  {statement}")
            print(f"      passes {got} argument(s), statement binds {wanted}")
        print(
            "  pgx rejects this at run time with `wrong number of parameters`. "
            "The statement prepares fine (PREPARE never sees the call site), so "
            "neither the runtime gate nor the column gate can report it; only the "
            "affected endpoint's first request does.",
            file=sys.stderr,
        )
        return 1

    print(
        f"SQL argument check OK: {checked} call site(s) pass exactly the number "
        f"of arguments their statement binds; {unresolved} call(s) refer to a "
        "statement this gate cannot resolve statically"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
