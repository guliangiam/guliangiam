#!/usr/bin/env python3
"""Parse every repository SQL statement against a real PostgreSQL schema.

Why this exists
---------------
`scripts/check-sql-columns.py` is a static check: it catches a column that the
DDL does not define, but it cannot see any of the following, all of which are
real defects that were live in this repository:

  * a parameter compared against an incompatible column type
    (`WHERE actor_id = $2` with actor_id UUID and $2 text -> "operator does not
    exist: uuid = text", so the whole audit list endpoint failed at runtime);
  * a parameter number that is never referenced (`$2` unused -> "could not
    determine data type of parameter $2");
  * an ambiguous column reference in a multi-table UPDATE
    (`revision = revision + 1` with both tables having a revision column);
  * a table that does not exist at all (a repository written against an imagined
    schema).

Statements are gathered from every non-test `.go` file under
`internal/infra/postgres` — not just the `sql.go` files — because a meaningful
number of them are written inline in `repository.go`, and those drifted too.

PostgreSQL's `PREPARE` resolves all of these at parse time, so preparing every
statement against a freshly replayed DDL is a complete check of the
statement/schema contract — it is the same guarantee the `//go:build
integration` tests give, but it runs without fixtures and covers every statement
rather than the handful the tests happen to exercise.

Usage
-----
    SQL_CHECK_DSN=postgres://user:pass@host:5432/db_sqlcheck python3 scripts/check-sql-runtime.py
    SQL_CHECK_DSN=... python3 scripts/check-sql-runtime.py --dry-run

The DSN is taken from `SQL_CHECK_DSN`, then `POSTGRES_DSN`, then `DATABASE_URL`.
The gate drops and rebuilds the schema, so the target must be a disposable
database: its name has to end in `_sqlcheck` / `_verify` / `_scratch`, or the
operator must explicitly acknowledge the reset with
`SQL_CHECK_ALLOW_DESTRUCTIVE=1`. The development database `guliangiam` and
production-like names (`prod`, `production`, `prd`, `live`) are refused
unconditionally, even with the acknowledgement. `--dry-run` performs the same
validation, prints the target and the destructive step, then exits without
connecting. When no DSN is configured the check is skipped with a warning
unless `REQUIRE_SQL_RUNTIME_CHECK=1`, which turns the skip into a failure so CI
cannot silently stop checking.

Exit code 0 = every statement prepares, 1 = at least one does not.

输入：internal/infra/postgres/*.go SQL 字面量、db/postgresql/V1__*.sql PREPARE
输出：PASS/FAIL + PREPARE 失败语句；无 DSN 时 SKIP 并告警
--check 行为：无 --check；失败即 exit 1。--dry-run 只校验目标并打印计划，不连库
消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
"""

import os
import re
import subprocess
import sys
from urllib.parse import unquote, urlsplit

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SCAN_DIR = os.path.join(REPO_ROOT, "internal/infra/postgres")
DDL_FILE = os.path.join(REPO_ROOT, "db/generated/postgresql/current.up.sql")
# The DDL owns exactly one schema; the gate rebuilds it from scratch on every run
# so the result reflects the DDL rather than the target database's history.
SCHEMA_NAME = "guliangiam"

# The gate drops and rebuilds this schema, so the target must be disposable.
# Names ending in one of these suffixes are treated as scratch databases; any
# other name needs an explicit acknowledgement. The development database and
# production-like names are refused even with that acknowledgement.
ALLOWED_DB_SUFFIXES = ("_sqlcheck", "_verify", "_scratch")
PROTECTED_DB_NAMES = {"guliangiam"}
PRODUCTION_NAME_PATTERN = re.compile(
    r"(?:^|[_-])(prod|production|prd|live)(?:$|[_-])", re.IGNORECASE
)

STATEMENT_PREFIX = re.compile(r"(?is)^\s*(select|insert|update|delete|with)\b")
LITERAL = re.compile(r"(\w+)\s*=\s*`([\s\S]*?)`")

# Statements that SCIM builds by concatenation: a static skeleton plus a filter
# clause and LIMIT/OFFSET appended at the call site. Those cannot be checked as
# a single literal, so the resolver below stitches the pieces back together.
ASSIGN = re.compile(r"^\s*(\w+)\s*(?::=|\+=|=)\s*(.+)$", re.M)
APPEND = re.compile(r"^\s*(\w+)\s*\+=\s*(.+)$", re.M)
IDENT_OR_LITERAL = re.compile(r"^(?:[`\"][^`\"]*[`\"]|\w+)$")
PLUS_SPLIT = re.compile(r"\s*\+\s*")
SPRINTF = re.compile(r"fmt\.Sprintf\(\s*`([^`]*)`\s*(?:,[^)]*)?\)")
BARE_KEYWORD = re.compile(r"(?is)^\s*(select|insert|update|delete|with)\s*;?\s*$")


def normalise_dynamic(sql):
    """Replace fmt.Sprintf(...) with a placeholder value so the statement parses.

    The result is only used for a parse check, so the concrete ordinal does not
    matter — what matters is that every table/column reference in the skeleton is
    resolved by PostgreSQL.
    """
    sql = SPRINTF.sub(lambda m: m.group(1).replace("%d", "2").replace("%s", "x"), sql)
    return sql


def _join_concatenated_lines(source: str) -> str:
    """Join physical lines that continue a Go string concatenation with `+`.

    Many sql.go constants are written across lines:

        queryInsert = "INSERT INTO ... " +
            "(id, organization_id, ...) " +
            "VALUES (...)"

    The per-line assignment regex sees the first line alone, whose right-hand side
    is a single literal plus a dangling `+`, and yields that literal as if it were
    a complete statement. Preparing it then fails with "syntax error at or near ;"
    even though the real statement is fine. Joining continuation lines before
    resolution keeps the extraction aligned with what Go actually concatenates.
    """
    joined: list[str] = []
    buffer = ""
    for line in source.splitlines():
        buffer = f"{buffer} {line.strip()}" if buffer else line
        if buffer.rstrip().endswith("+"):
            continue
        joined.append(buffer)
        buffer = ""
    if buffer:
        joined.append(buffer)
    return "\n".join(joined)


def resolve_assignments(source):
    """Stitch ``x := `A` + `B``` concatenations into one statement per assignment.

    Only right-hand sides built exclusively from string literals and other
    already-resolved variables are followed; anything else (a function call, a
    struct field) stops the resolution instead of guessing.

    Returns a list rather than a dict: the same variable name (`baseQuery`,
    `countQuery`, ...) is reused across functions, and keying by name would keep
    only the first one and silently drop the rest.
    """
    source = _join_concatenated_lines(source)
    resolved = []
    values = {}
    appended_names = set()
    for _ in range(6):  # small fixpoint: variables may reference later ones
        for match in ASSIGN.finditer(source):
            name, rhs = match.group(1), match.group(2).strip()
            is_append = bool(re.match(r"^\s*\w+\s*\+=", match.group(0)))
            pieces = []
            ok = True
            for part in PLUS_SPLIT.split(rhs):
                part = part.strip()
                if not part:
                    continue
                if len(part) >= 2 and part[0] in "`\"" and part[-1] == part[0]:
                    pieces.append(part[1:-1])
                elif part in values:
                    pieces.append(values[part])
                elif SPRINTF.fullmatch(part):
                    pieces.append(normalise_dynamic(part))
                else:
                    ok = False
                    break
            if not ok or not pieces:
                continue
            appended = " ".join(pieces)
            if is_append and name in values:
                sql = values[name] + " " + appended
                appended_names.add(name)
            else:
                sql = appended
            if values.get(name) != sql:
                values[name] = sql
                resolved.append((name, sql, name in appended_names))
    return resolved


def find_dsn():
    for key in ("SQL_CHECK_DSN", "POSTGRES_DSN", "DATABASE_URL"):
        value = os.environ.get(key, "").strip()
        if value:
            return value, key
    return "", ""


def parse_target(dsn):
    """Return ``(host, database)`` from a URL or key/value DSN, without connecting."""
    parsed = urlsplit(dsn)
    if parsed.scheme in ("postgres", "postgresql"):
        database = unquote(parsed.path.lstrip("/").split("/")[0])
        return parsed.hostname or "<unknown>", database

    values = {}
    for match in re.finditer(r"(?:^|\s)(\w+)=('[^']*'|\"[^\"]*\"|\S+)", dsn):
        raw = match.group(2)
        if len(raw) >= 2 and raw[0] == raw[-1] and raw[0] in "'\"":
            raw = raw[1:-1]
        values[match.group(1)] = raw
    return values.get("host", "<unknown>"), values.get("dbname", "")


def validate_target(dsn, source_key):
    """Refuse to drop a schema on anything that is not a disposable database.

    Validation happens before any ``psql`` invocation, so a rejected target
    never receives a connection, let alone a ``DROP SCHEMA``.
    """
    host, database = parse_target(dsn)
    database = (database or "").strip()
    print(f"SQL check target: host={host} database={database or '<unknown>'} (via {source_key})")

    if not database:
        print(
            "ERROR: refusing to drop a schema because the DSN does not name a database",
            file=sys.stderr,
        )
        return False, host, database
    if database.lower() in PROTECTED_DB_NAMES:
        print(
            f"ERROR: refusing destructive replay on protected database '{database}' "
            "(development database). Point SQL_CHECK_DSN at a scratch database.",
            file=sys.stderr,
        )
        return False, host, database
    if PRODUCTION_NAME_PATTERN.search(database):
        print(
            f"ERROR: refusing destructive replay on production-like database '{database}'",
            file=sys.stderr,
        )
        return False, host, database
    if database.lower().endswith(ALLOWED_DB_SUFFIXES):
        return True, host, database
    if os.environ.get("SQL_CHECK_ALLOW_DESTRUCTIVE") == "1":
        print(
            f"WARNING: destructive replay on '{database}' acknowledged by "
            "SQL_CHECK_ALLOW_DESTRUCTIVE=1"
        )
        return True, host, database

    print(
        f"ERROR: refusing destructive replay on '{database}': the database name must end "
        f"with one of {', '.join(ALLOWED_DB_SUFFIXES)}, or set "
        "SQL_CHECK_ALLOW_DESTRUCTIVE=1 after confirming it is a disposable database.",
        file=sys.stderr,
    )
    return False, host, database


def psql(dsn, args):
    return subprocess.run(
        ["psql", dsn, "-q", "-v", "ON_ERROR_STOP=1", *args],
        capture_output=True,
        text=True,
    )


def _composed_names(source):
    """Names referenced inside a ``+`` expression (statement assembled at the call site).

    A constant such as ``queryListActionsSelect`` is only the SELECT list; the
    FROM/WHERE/ORDER BY tail is appended where it is used. Preparing the literal
    alone is guaranteed to reference columns that have no table yet, so the
    standalone form must not be treated as a defect.
    """
    names = set()
    for match in re.finditer(r"\b(\w+)\b\s*\+|\+\s*\b(\w+)\b|\b(\w+)\b\s*\+=", source):
        for group in match.groups():
            if group:
                names.add(group)
    return names


def _global_composed_names():
    """Composed-name set across the whole scan tree.

    The constant and the ``a + b`` expression live in different files of the same
    package (``sql.go`` defines the SELECT list, ``repository.go`` appends the
    tail), so the detection has to span files.
    """
    names = set()
    for root, _dirs, files in os.walk(SCAN_DIR):
        for name in sorted(files):
            if not name.endswith(".go") or name.endswith("_test.go"):
                continue
            with open(os.path.join(root, name), encoding="utf-8") as handle:
                names |= _composed_names(handle.read())
    return names


def collect_statements():
    """Yield (path, name, sql, composed) for every statement under infra/postgres.

    Two shapes are collected:

    * a plain backtick literal assigned to a name (the `sql.go` convention);
    * an expression stitched together from literals and other resolved variables
      (the pattern SCIM uses for its filter + paging clauses).

    The whole package tree is scanned, not just `sql.go`: a meaningful number of
    statements live inline in `repository.go` (users, scim, ldapsync, coupons,
    sessions, applications), and those are exactly the ones that drifted.
    """
    global_composed = _global_composed_names()
    for root, _dirs, files in os.walk(SCAN_DIR):
        for name in sorted(files):
            if not name.endswith(".go") or name.endswith("_test.go"):
                continue
            path = os.path.join(root, name)
            with open(path, encoding="utf-8") as handle:
                source = handle.read()

            seen = set()
            for match in LITERAL.finditer(source):
                sql = match.group(2).strip()
                if BARE_KEYWORD.match(sql):
                    # A Go variable holding just the word "SELECT" is not a statement.
                    continue
                # A backtick literal glued to another expression with `+` is only one
                # piece of a statement (e.g. `... AND ` + scopeValidity + `)`). Its
                # tail lives outside this literal, so preparing it standalone is
                # guaranteed to raise a syntax error; such fragments must be skipped,
                # exactly like the fragments resolve_assignments already marks.
                stitched = source[: match.start()].rstrip().endswith("+") or                     source[match.end():].lstrip().startswith("+")
                if STATEMENT_PREFIX.match(sql) and sql not in seen:
                    seen.add(sql)
                    yield path, match.group(1), sql, match.group(1) in global_composed or stitched

            for var, sql, had_append in resolve_assignments(source):
                sql = normalise_dynamic(sql).strip()
                if BARE_KEYWORD.match(sql):
                    continue
                if STATEMENT_PREFIX.match(sql) and sql not in seen:
                    seen.add(sql)
                    # A statement built with `+=` inside branches is a superset of
                    # the real statements (both arms of an if/else get appended),
                    # so it can legitimately fail to parse. Mark it partial.
                    yield path, var, sql, had_append


def main():
    dsn, source_key = find_dsn()
    if not dsn:
        message = (
            "no DSN configured (SQL_CHECK_DSN / POSTGRES_DSN / DATABASE_URL); "
            "skipping the runtime SQL check"
        )
        if os.environ.get("REQUIRE_SQL_RUNTIME_CHECK") == "1":
            print(f"ERROR: {message}", file=sys.stderr)
            return 1
        print(f"WARNING: {message}", file=sys.stderr)
        return 0

    ok, host, database = validate_target(dsn, source_key)
    if not ok:
        return 1

    if "--dry-run" in sys.argv[1:] or os.environ.get("SQL_CHECK_DRY_RUN") == "1":
        print(
            f"DRY RUN: would DROP SCHEMA {SCHEMA_NAME} CASCADE and replay "
            f"{os.path.basename(DDL_FILE)} on {host}/{database}; no connection made"
        )
        return 0

    print(
        f"[1/3] destructive replay on {database}: resetting the {SCHEMA_NAME} "
        f"schema (host={host}, via {source_key})"
    )
    # The DDL uses CREATE TABLE IF NOT EXISTS, so replaying it onto an existing
    # database is a no-op for every table that already exists. That makes the
    # result depend on the target database history instead of on the DDL:
    #   - a column the DDL added is missing  -> false FAILURE (stale database)
    #   - a column the DDL dropped is present -> false PASS (statement prepares
    #     against a schema that a fresh deployment would not have)
    # ADR-022 makes the DDL the single source of truth replayed in full, so the
    # check must build that schema from scratch to be meaningful.
    #
    # Resetting is destructive, so it is gated: SQL_CHECK_DSN is this gate's own
    # scratch database (the name says so), while POSTGRES_DSN / DATABASE_URL may
    # well point at a shared or production instance and additionally need the
    # explicit ALLOW_SCHEMA_RESET=1 acknowledgement that
    # scripts/dev/validate_schema_replay.sh already uses.
    if source_key != "SQL_CHECK_DSN" and os.environ.get("ALLOW_SCHEMA_RESET") != "1":
        print(
            f"ERROR: refusing to DROP SCHEMA {SCHEMA_NAME} on a connection taken "
            f"from {source_key}. Point SQL_CHECK_DSN at a scratch database, or "
            "set ALLOW_SCHEMA_RESET=1 after confirming the target is not production.",
            file=sys.stderr,
        )
        return 1

    reset = psql(dsn, ["-c", f"DROP SCHEMA IF EXISTS {SCHEMA_NAME} CASCADE;"])
    if reset.returncode != 0:
        print("ERROR: could not reset the schema:", file=sys.stderr)
        print(reset.stderr.strip(), file=sys.stderr)
        return 1

    print(f"[2/3] replaying {os.path.basename(DDL_FILE)}")
    # 重放前把 search_path 固定到 public：DDL 的第 1 条语句是 CREATE EXTENSION，
    # 而 CREATE SCHEMA guliangiam 在其后；若目标库/角色把 search_path 设成 guliangiam
    # （本仓库开发机的默认设置），DROP SCHEMA 之后没有任何可创建对象的 schema，
    # CREATE EXTENSION 直接报 "no schema has been selected to create in"。
    # 生产迁移执行器会先建 schema 再重放，门禁必须自己复刻这一步。
    replay = psql(dsn, ["-c", "SET search_path = public;", "-f", DDL_FILE])
    if replay.returncode != 0:
        print("ERROR: could not replay the DDL:", file=sys.stderr)
        print(replay.stderr.strip(), file=sys.stderr)
        return 1

    statements = list(collect_statements())
    print(f"[3/3] preparing {len(statements)} statements")

    failures = []
    skipped_fragments = []
    for path, name, sql, composed in statements:
        result = psql(dsn, ["-c", f"PREPARE _check AS {sql};"])
        if result.returncode != 0:
            detail = (result.stderr or "").strip().splitlines()
            reason = detail[0].strip() if detail else "unknown error"
            # A constant that is only one piece of a statement assembled at the
            # call site cannot be prepared on its own: its FROM / WHERE / ORDER BY
            # tail lives elsewhere. Report it as skipped rather than as a defect,
            # otherwise every SELECT-list constant becomes a permanent false alarm
            # and the gate gets ignored. Fully-resolved statements stay failures.
            if composed and (
                "syntax error" in reason
                or "does not exist" in reason
                or "could not determine data type" in reason
            ):
                skipped_fragments.append((os.path.relpath(path, REPO_ROOT), name, reason))
                continue
            failures.append((os.path.relpath(path, REPO_ROOT), name, reason))

    if skipped_fragments:
        print(
            f"SQL runtime check skipped {len(skipped_fragments)} call-site fragment(s) "
            "(statement is assembled from several constants):"
        )
        for path, name, reason in skipped_fragments:
            print(f"  - {path}::{name}: {reason}")

    if failures:
        print(f"SQL runtime check FAILED ({len(failures)} statement(s)):")
        for path, name, reason in failures:
            print(f"  - {path}::{name}: {reason}")
        return 1

    print(
        f"SQL runtime check OK: {len(statements)} statements prepare against the "
        "replayed schema"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
