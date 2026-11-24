#!/usr/bin/env python3
"""Validate every SQL literal in internal/infra/postgres against the DDL schema.

Why this exists
---------------
The repository layer writes SQL as Go string literals. Nothing checks that the
columns and tables those literals name actually exist, so a rename in
`db/generated/postgresql/current.up.sql` silently breaks the query at
runtime — and the `//go:build integration` tests that would catch it are not run
by CI. This gate parses the DDL, then resolves every table reference, INSERT
column list, UPDATE SET target and `alias.column` reference in the SQL literals
and fails on anything the schema does not define.

Scope and limits
----------------
- Every non-test `.go` file under `internal/infra/postgres` is scanned, so
  statements written inline in `repository.go` are covered too.
- A literal that is a fragment (no `FROM`/`INTO`/`UPDATE`, e.g. a filter clause
  concatenated at the call site) cannot be resolved to a table and is skipped;
  the count is reported as a note. Such fragments are only checked once they are
  complete statements, by `scripts/check-sql-runtime.py`.
- Dynamic SQL built with fmt.Sprintf is skipped, not guessed.
- Unknown application table references fail instead of silently disabling the
  column check; CTEs and PostgreSQL builtins are excluded explicitly.
- Qualified names other than `guliangiam.` are ignored (they are extensions or
  pg_catalog built-ins).

Usage: python3 scripts/check-sql-columns.py
Exit code 0 = no drift, 1 = drift found.

输入：internal/infra/postgres/*.go SQL 字面量、db/postgresql/V1__*.sql
输出：PASS/FAIL + 未定义表/列引用
--check 行为：无 --check；失败即 exit 1
消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
"""

import os
import re
import sys
from collections import defaultdict

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DDL_FILE = os.path.join(REPO_ROOT, "db/generated/postgresql/current.up.sql")
SCAN_DIR = os.path.join(REPO_ROOT, "internal/infra/postgres")

# Columns provided by PostgreSQL itself or by the schema's compatibility shims.
BUILTIN_TABLES = {
    "information_schema",
    "pg_catalog",
    "pg_type",
    "pg_constraint",
    "pg_class",
    "pg_namespace",
    "pg_indexes",
    "pg_tables",
    "pg_attribute",
    "generate_series",
}


def parse_column_type(line):
    """Return the logical type of a column definition line, e.g. 'JSONB', 'TEXT[]'.

    Multi-word types are only produced when the leading word is one that really
    continues (``TIMESTAMP WITH TIME ZONE``, ``DOUBLE PRECISION``,
    ``CHARACTER VARYING``); otherwise the next token is a constraint such as
    NOT/NULL/DEFAULT and must not be folded into the type. The array suffix is
    read from the token immediately following the type name, so a ``[]`` inside a
    DEFAULT or CHECK clause cannot turn a scalar column into an array.
    """
    match = re.match(
        r'^"?([a-z_][a-z0-9_]*)"?\s+([A-Za-z]+)'
        r'(?:\s*\([^)]*\))?(\s+(?:WITH|VARYING|PRECISION))?'
        r'(?:\s*\([^)]*\))?(\s*\[\s*\])?',
        line,
        re.IGNORECASE,
    )
    if not match:
        return None, None

    name = match.group(1).lower()
    head = match.group(2).upper()
    tail = (match.group(3) or "").strip().upper()
    is_array = bool(match.group(4))

    if head == "TIMESTAMP":
        logical = "TIMESTAMPTZ" if tail.startswith("WITH") else "TIMESTAMP"
    elif head == "TIME":
        logical = "TIMETZ" if tail.startswith("WITH") else "TIME"
    elif head == "DOUBLE" and tail.startswith("PRECISION"):
        logical = "DOUBLE PRECISION"
    elif head in ("VARCHAR", "CHAR", "BPCHAR", "CHARACTER"):
        logical = "VARCHAR"
    elif head in ("INT", "INT4"):
        logical = "INTEGER"
    elif head == "INT8":
        logical = "BIGINT"
    elif head == "BOOL":
        logical = "BOOLEAN"
    elif head == "TIMESTAMPTZ":
        logical = "TIMESTAMPTZ"
    else:
        logical = head

    if is_array:
        logical += "[]"
    return name, logical


def extract_balanced_body(source, open_paren):
    """Return the text between a '(' and its matching ')'.

    Column definitions contain parenthesised types and expressions
    (`CHECK (x IN ('a','b'))`, `DECIMAL(19,4)`, `DEFAULT ('{}'::jsonb)`), so the
    body cannot be found by looking for the first `)`. Counting depth — while
    skipping string literals, dollar-quoted bodies, and comments — is the only
    way to find the real terminator regardless of how the file is laid out.

    Returns None when the parentheses never balance, which means the caller
    should fail loudly rather than analyse half a table.
    """
    depth = 0
    index = open_paren
    length = len(source)
    while index < length:
        char = source[index]

        # Line comment: skip to end of line.
        if char == "-" and source.startswith("--", index):
            newline = source.find("\n", index)
            index = length if newline == -1 else newline + 1
            continue

        # Block comment (nesting is allowed in Postgres).
        if char == "/" and source.startswith("/*", index):
            depth_comment = 1
            index += 2
            while index < length and depth_comment:
                if source.startswith("/*", index):
                    depth_comment += 1
                    index += 2
                elif source.startswith("*/", index):
                    depth_comment -= 1
                    index += 2
                else:
                    index += 1
            continue

        # Single-quoted string, with '' as the escape.
        if char == "'":
            index += 1
            while index < length:
                if source[index] == "'":
                    if index + 1 < length and source[index + 1] == "'":
                        index += 2
                        continue
                    index += 1
                    break
                index += 1
            continue

        # Dollar-quoted body ($$ ... $$ or $tag$ ... $tag$).
        if char == "$":
            tag_match = re.match(r"\$[A-Za-z_]*\$", source[index:])
            if tag_match:
                tag = tag_match.group(0)
                close = source.find(tag, index + len(tag))
                index = length if close == -1 else close + len(tag)
                continue

        if char == "(":
            depth += 1
        elif char == ")":
            depth -= 1
            if depth == 0:
                return source[open_paren + 1 : index]
        index += 1
    return None


def split_top_level_commas(body):
    """Split a CREATE TABLE body into definition segments at depth-0 commas.

    Splitting on lines would depend on the layout: when a formatter joins several
    columns onto one line, a line-based reader silently keeps only the first and
    then reports the rest as "column does not exist". Commas at parenthesis
    depth 0 are the definition boundary in every layout, so that is what is used.
    """
    segments = []
    current = []
    depth = 0
    index = 0
    length = len(body)
    while index < length:
        char = body[index]

        if char == "-" and body.startswith("--", index):
            newline = body.find("\n", index)
            index = length if newline == -1 else newline + 1
            continue

        if char == "/" and body.startswith("/*", index):
            depth_comment = 1
            index += 2
            while index < length and depth_comment:
                if body.startswith("/*", index):
                    depth_comment += 1
                    index += 2
                elif body.startswith("*/", index):
                    depth_comment -= 1
                    index += 2
                else:
                    index += 1
            continue

        if char == "'":
            current.append(char)
            index += 1
            while index < length:
                current.append(body[index])
                if body[index] == "'":
                    if index + 1 < length and body[index + 1] == "'":
                        current.append("'")
                        index += 2
                        continue
                    index += 1
                    break
                index += 1
            continue

        if char == "(":
            depth += 1
        elif char == ")":
            depth -= 1

        if char == "," and depth == 0:
            segments.append("".join(current))
            current = []
            index += 1
            continue

        current.append(char)
        index += 1

    if "".join(current).strip():
        segments.append("".join(current))
    return segments


def parse_ddl(path):
    """Return {table_name: {column_name: logical_type}} from the CREATE TABLE blocks.

    Layout-independent on purpose. The file is edited by hand and has been
    reformatted by external tools more than once; a parser that depends on where
    the closing parenthesis or the line breaks sit turns a cosmetic change into
    either a silent no-op (0 tables parsed) or a burst of false "column does not
    exist" errors.
    """
    with open(path, encoding="utf-8") as handle:
        ddl = handle.read()

    tables = {}
    header_re = re.compile(
        r"CREATE TABLE IF NOT EXISTS\s+(?:guliangiam\.)?([a-z_][a-z0-9_]*)\s*\(",
        re.IGNORECASE,
    )

    for match in header_re.finditer(ddl):
        name = match.group(1).lower()
        body = extract_balanced_body(ddl, match.end() - 1)
        if body is None:
            raise SystemExit(
                "ERROR: unbalanced parentheses in the CREATE TABLE block for %r.\n"
                "       The DDL is malformed; fix the file rather than the checker."
                % name
            )

        columns = {}
        for segment in split_top_level_commas(body):
            # Collapse to a single line so a definition split across lines still
            # parses as one column.
            definition = " ".join(segment.split())
            if not definition:
                continue
            # Skip table-level constraints.
            if re.match(
                r"^(CONSTRAINT|PRIMARY\s+KEY|UNIQUE|FOREIGN\s+KEY|CHECK|EXCLUDE)\b",
                definition,
                re.IGNORECASE,
            ):
                continue
            col, col_type = parse_column_type(definition)
            if col:
                columns[col] = col_type
        tables[name] = columns

    # A partial parse is worse than no parse: it silently drops columns from the
    # comparison, so every statement touching them is reported as a violation.
    # Refuse to continue unless every declared table was read.
    declared = len(header_re.findall(ddl))
    if len(tables) != declared:
        raise SystemExit(
            "ERROR: the DDL declares %d tables but only %d could be parsed.\n"
            "       Refusing to compare against an incomplete schema."
            % (declared, len(tables))
        )
    if not tables:
        raise SystemExit(
            "ERROR: no tables parsed from the DDL. The file was not found or no "
            "longer contains CREATE TABLE statements."
        )

    # ALTER TABLE ... ADD COLUMN also contributes columns.
    alter_re = re.compile(
        r"ALTER TABLE\s+(?:guliangiam\.)?([a-z_][a-z0-9_]*)\s+ADD COLUMN(?:\s+IF NOT EXISTS)?\s+"
        r'"?([a-z_][a-z0-9_]*)"?\s+([A-Za-z]+)',
        re.IGNORECASE,
    )
    for match in alter_re.finditer(ddl):
        tables.setdefault(match.group(1).lower(), {})[match.group(2).lower()] = match.group(
            3
        ).upper()
    return tables


def extract_string_literals(path):
    """Yield (line_number, literal) for every raw string in a Go file.

    Go raw string literals are delimited by backticks and routinely span many
    lines (every SQL constant in this repository does), so a per-line regex is
    not enough: the scanner has to walk the file and track the opening line.
    """
    with open(path, encoding="utf-8") as handle:
        text = handle.read()

    out = []
    i = 0
    line = 1
    while i < len(text):
        ch = text[i]
        if ch == "\n":
            line += 1
            i += 1
            continue
        if ch == "`":
            start_line = line
            end = text.find("`", i + 1)
            if end == -1:
                break
            literal = text[i + 1:end]
            out.append((start_line, literal))
            line += literal.count("\n")
            i = end + 1
            continue
        if ch == "/" and i + 1 < len(text) and text[i + 1] == "/":
            newline = text.find("\n", i)
            i = len(text) if newline == -1 else newline
            continue
        if ch == '"':
            # Interpreted string: skip it so backticks inside are not mistaken
            # for a literal delimiter.
            i += 1
            while i < len(text):
                if text[i] == "\\":
                    i += 2
                    continue
                if text[i] == '"':
                    break
                if text[i] == "\n":
                    line += 1
                i += 1
            i += 1
            continue
        i += 1
    return out


SQL_KEYWORDS = {
    "select",
    "from",
    "where",
    "insert",
    "into",
    "values",
    "update",
    "set",
    "delete",
    "join",
    "left",
    "right",
    "inner",
    "outer",
    "on",
    "and",
    "or",
    "not",
    "null",
    "as",
    "order",
    "by",
    "group",
    "limit",
    "offset",
    "returning",
    "conflict",
    "do",
    "nothing",
    "distinct",
    "case",
    "when",
    "then",
    "else",
    "end",
    "is",
    "in",
    "exists",
    "coalesce",
    "count",
    "now",
    "true",
    "false",
    "default",
    "using",
    "with",
    "union",
    "all",
}


def strip_sql_comments(sql):
    sql = re.sub(r"--[^\n]*", " ", sql)
    return sql


def split_top_level(text):
    """Split a VALUES list on commas that are not inside parentheses or quotes."""
    parts = []
    depth = 0
    current = []
    quote = None
    i = 0
    while i < len(text):
        ch = text[i]
        if quote:
            current.append(ch)
            if ch == quote:
                quote = None
            i += 1
            continue
        if ch in "'\"":
            quote = ch
            current.append(ch)
        elif ch == "(":
            depth += 1
            current.append(ch)
        elif ch == ")":
            depth -= 1
            current.append(ch)
        elif ch == "," and depth == 0:
            parts.append("".join(current))
            current = []
        else:
            current.append(ch)
        i += 1
    if current:
        parts.append("".join(current))
    return parts


def find_matching_paren(text, open_index):
    """Return the index of the ')' matching the '(' at open_index, or -1.

    A plain non-greedy `\\((.*?)\\)` stops at the first ')' it sees, which is wrong
    as soon as a value contains a function call (e.g. `CASE ... ($5 * INTERVAL
    '1 minute')`) — that produced bogus "N columns but M values" reports.
    """
    if open_index >= len(text) or text[open_index] != "(":
        return -1
    depth = 0
    quote = None
    i = open_index
    while i < len(text):
        ch = text[i]
        if quote:
            if ch == quote:
                quote = None
            i += 1
            continue
        if ch in "'\"":
            quote = ch
        elif ch == "(":
            depth += 1
        elif ch == ")":
            depth -= 1
            if depth == 0:
                return i
        i += 1
    return -1


INSERT_RE = re.compile(
    r"INSERT\s+INTO\s+(?:\"?guliangiam\"?\.)?\"?([a-z_][a-z0-9_]*)\"?\s*\(",
    re.IGNORECASE,
)


def iter_inserts(sql):
    """Yield (table, columns, values) for every INSERT ... VALUES in a SQL literal."""
    for match in INSERT_RE.finditer(sql):
        table = match.group(1).lower()
        cols_end = find_matching_paren(sql, match.end() - 1)
        if cols_end == -1:
            continue
        cols_text = sql[match.end():cols_end]

        values_kw = re.compile(r"\bVALUES\b", re.IGNORECASE).search(sql, cols_end)
        if not values_kw:
            continue
        values_open = sql.find("(", values_kw.end())
        if values_open == -1:
            continue
        values_end = find_matching_paren(sql, values_open)
        if values_end == -1:
            continue

        cols = [c.strip().strip('"').lower() for c in cols_text.split(",")]
        values = [v.strip() for v in split_top_level(sql[values_open + 1:values_end])]
        yield table, cols, values


# Logical column type -> the casts that are acceptable on its placeholder. An
# empty set means "no cast needed, but any of these are tolerated".
CAST_RULES = {
    "JSONB": {"jsonb"},
    "JSON": {"json", "jsonb"},
    "TEXT[]": {"text[]", "varchar[]"},
    "VARCHAR[]": {"varchar[]", "text[]"},
    "UUID[]": {"uuid[]"},
    "JSONB[]": {"jsonb[]"},
    "INTEGER[]": {"int[]", "integer[]"},
    "UUID": {"uuid"},
    "TIMESTAMPTZ": {"timestamptz"},
    "TIMESTAMP": {"timestamp", "timestamptz"},
    "INET": {"inet"},
    "BYTEA": {"bytea", "blob"},
    "NUMERIC": {"numeric", "decimal"},
    "BOOLEAN": {"boolean", "bool"},
    "BIGINT": {"bigint", "int8"},
    "INTEGER": {"int", "integer", "int4"},
    "VARCHAR": {"varchar", "text", "char"},
}


def cast_is_compatible(column_type, cast):
    """Return True when casting a parameter to `cast` can satisfy `column_type`."""
    column_type = column_type.upper()
    expected = CAST_RULES.get(column_type)
    if expected is None:
        # Anything not listed is treated as text-compatible.
        return cast in {"text", "varchar"}
    return cast in expected


def find_table_aliases(sql, tables):
    """Return aliases, known tables, and unknown guliangiam/unqualified tables.

    CTE names, PostgreSQL builtins, and explicitly non-guliangiam schemas are not
    application tables. Every other unresolved table must be reported instead of
    silently disabling all column checks for that statement.
    """
    aliases = {}
    found = []
    unknown = set()
    cte_names = {
        match.group(1).lower()
        for match in re.finditer(
            r"(?:\bWITH|,)\s*([a-z_][a-z0-9_]*)\s+AS\s*\(",
            sql,
            re.IGNORECASE,
        )
    }
    pattern = re.compile(
        r"\b(FROM|JOIN|INTO|UPDATE)\s+"
        r"(?:\"?([a-z_][a-z0-9_]*)\"?\.)?\"?([a-z_][a-z0-9_]*)\"?"
        r"(?:\s+(?:AS\s+)?([a-z_][a-z0-9_]*))?",
        re.IGNORECASE,
    )
    for match in pattern.finditer(sql):
        keyword = match.group(1).upper()
        prefix = sql[:match.start()]
        # FOR UPDATE SKIP LOCKED / FOR UPDATE OF ... are locking clauses, not
        # UPDATE statements. EXTRACT(HOUR FROM created_at) is a function, not a
        # table reference either.
        if keyword == "UPDATE" and re.search(r"\bFOR\s*$", prefix, re.IGNORECASE):
            continue
        if keyword == "FROM" and re.search(
            r"\b(?:EXTRACT|TRIM|SUBSTRING)\s*\([^()]*$", prefix, re.IGNORECASE
        ):
            continue
        schema = (match.group(2) or "").lower()
        table = match.group(3).lower()
        if schema and schema != "guliangiam":
            continue
        if table in BUILTIN_TABLES or table in cte_names or table in SQL_KEYWORDS:
            continue
        if table not in tables:
            unknown.add(table)
            continue
        found.append(table)
        aliases[table] = table
        alias = match.group(4)
        if alias:
            alias = alias.lower()
            if alias not in SQL_KEYWORDS:
                aliases[alias] = table
    return aliases, found, sorted(unknown)


def check_file(path, tables, violations, notes):
    rel = os.path.relpath(path, REPO_ROOT)
    for line_no, literal in extract_string_literals(path):
        if not re.search(r"\b(SELECT|INSERT|UPDATE|DELETE)\b", literal, re.IGNORECASE):
            continue
        if "fmt.Sprintf" in literal or "%s" in literal or "%d" in literal:
            continue

        sql = strip_sql_comments(literal)
        aliases, table_list, unknown_tables = find_table_aliases(sql, tables)
        for table in unknown_tables:
            violations.append(f"{rel}:{line_no}: table {table!r} does not exist in the DDL")
        if not table_list:
            continue

        known = list(dict.fromkeys(table_list))

        # 1. INSERT column list, plus placeholder position and cast sanity.
        for table, cols, vals in iter_inserts(sql):
            if table not in tables:
                continue

            for col in cols:
                if col and col.isidentifier() and col not in tables[table]:
                    violations.append(
                        f"{rel}:{line_no}: INSERT into {table}: column {col!r} does not exist"
                    )

            if len(cols) != len(vals):
                violations.append(
                    f"{rel}:{line_no}: INSERT into {table}: {len(cols)} columns but "
                    f"{len(vals)} values"
                )
                continue

            for index, (col, value) in enumerate(zip(cols, vals), start=1):
                placeholder = re.match(r"\$(\d+)", value)
                if placeholder and int(placeholder.group(1)) > len(vals):
                    violations.append(
                        f"{rel}:{line_no}: INSERT into {table}: position {index} "
                        f"references ${placeholder.group(1)} but only {len(vals)} "
                        "parameters exist"
                    )

                # The cast can only be judged when the value is a bare parameter
                # (optionally cast). A value like `CASE ... THEN $4::timestamptz
                # + ($5::int * INTERVAL '1 minute') ...` contains several casts
                # and none of them describes the column.
                simple = re.fullmatch(r"\$\d+::([a-z_]+(?:\s*\[\s*\])?)", value, re.IGNORECASE)
                if not simple or col not in tables[table]:
                    continue
                want = tables[table][col]
                got = re.sub(r"\s+", "", simple.group(1).lower())
                if not cast_is_compatible(want, got):
                    violations.append(
                        f"{rel}:{line_no}: INSERT into {table}: column {col!r} is "
                        f"{want} but the placeholder casts to {got}"
                    )

        # 2. UPDATE ... SET targets.
        for match in re.finditer(
            r"UPDATE\s+(?:\"?guliangiam\"?\.)?\"?([a-z_][a-z0-9_]*)\"?\s+SET\s+(.*?)"
            r"(?:\bWHERE\b|\bRETURNING\b|$)",
            sql,
            re.IGNORECASE | re.DOTALL,
        ):
            table = match.group(1).lower()
            if table not in tables:
                continue
            for assign in match.group(2).split(","):
                if "=" not in assign:
                    continue
                col = assign.split("=", 1)[0].strip().strip('"').lower()
                col = col.split(".")[-1]
                if col and col.isidentifier() and col not in tables[table]:
                    violations.append(
                        f"{rel}:{line_no}: UPDATE {table}: column {col!r} does not exist"
                    )

        # 3. alias.column references.
        for match in re.finditer(r"\b([a-z_][a-z0-9_]*)\.([a-z_][a-z0-9_]*)\b", sql):
            alias, col = match.group(1).lower(), match.group(2).lower()
            table = aliases.get(alias)
            if table is None:
                continue  # unresolved alias, e.g. a CTE or a sub-select
            columns = tables.get(table)
            if columns is None:
                continue
            if col not in columns:
                violations.append(
                    f"{rel}:{line_no}: {alias}.{col} does not exist on {table}"
                )

        # 4. Every $N referenced must exist among the statement's parameters.
        # (An exact position check is not possible: literals such as `'idle'` and
        # `1` sit between placeholders, and a parameter like `now` is legitimately
        # reused several times.)
        for match in re.finditer(r"\$(\d+)", sql):
            index = int(match.group(1))
            if index == 0:
                violations.append(f"{rel}:{line_no}: $0 is not a valid parameter")



def normalize_sql_fragment(text):
    """Collapse whitespace so two equivalent clauses compare equal."""
    return re.sub(r"\s+", " ", text).strip()


def where_clause_of(sql):
    """Return the normalized WHERE clause body of a statement, or None."""
    match = re.search(
        r"(?:^|\n)\s*WHERE\b(.*?)(?:\n\s*ORDER BY\b|\n\s*LIMIT\b|\n\s*OFFSET\b|$)",
        sql,
        re.DOTALL | re.IGNORECASE,
    )
    if not match:
        return None
    return normalize_sql_fragment(match.group(1))


def check_list_count_consistency(violations, notes):
    """Fail when a list query and its paired COUNT query filter differently.

    Every paginated endpoint answers "which rows" and "how many rows" with two
    statements built from the same filter. If the two WHERE clauses drift apart
    the page content and `page.total` disagree, which silently corrupts
    pagination: the client either paginates past the real end or stops early,
    and neither is reported as an error. The pair is matched by name
    (`queryListX`/`querySearchX` <-> `queryCountX`), so this only covers the
    convention the repository layer actually follows.
    """
    checked = 0
    for root, _dirs, files in os.walk(SCAN_DIR):
        for name in sorted(files):
            if name != "sql.go":
                continue
            path = os.path.join(root, name)
            with open(path, encoding="utf-8") as handle:
                source = handle.read()

            constants = dict(
                re.findall(r"\b(\w+)\s*=\s*`([^`]*)`", source)
            )
            lists = {
                key: value
                for key, value in constants.items()
                if re.match(r"query(List|Search)", key)
            }
            counts = {
                key: value for key, value in constants.items() if "Count" in key
            }

            relative = os.path.relpath(path, REPO_ROOT)
            for list_key, list_sql in sorted(lists.items()):
                count_key = re.sub(r"query(List|Search)", "queryCount", list_key)
                if count_key not in counts:
                    continue
                checked += 1
                list_where = where_clause_of(list_sql)
                count_where = where_clause_of(counts[count_key])
                if list_where == count_where:
                    continue
                violations.append(
                    f"{relative}: {list_key} and {count_key} filter differently, so "
                    f"page.total disagrees with the page contents\n"
                    f"      list : {list_where}\n"
                    f"      count: {count_where}"
                )

    notes.append(
        f"list/COUNT filter consistency: {checked} paired queries compared"
    )
    return checked


def main():
    tables = parse_ddl(DDL_FILE)
    if not tables:
        print("ERROR: no tables parsed from the DDL", file=sys.stderr)
        return 1

    violations = []
    notes = []

    for root, _dirs, files in os.walk(SCAN_DIR):
        for name in sorted(files):
            if not name.endswith(".go") or name.endswith("_test.go"):
                continue
            path = os.path.join(root, name)
            check_file(path, tables, violations, notes)

    # Every non-test .go file under infra/postgres is already scanned above. The
    # gap that remains is narrower than "outside sql.go": a literal that is a
    # fragment (built by concatenation, so it has no FROM clause) cannot be
    # resolved to a table and is skipped. Report how many such fragments exist so
    # the gap stays visible; they are covered by scripts/check-sql-runtime.py
    # once they are complete statements.
    fragments = defaultdict(int)
    for root, _dirs, files in os.walk(SCAN_DIR):
        for name in files:
            if not (name.endswith(".go") and not name.endswith("_test.go")):
                continue
            path = os.path.join(root, name)
            for _line, literal in extract_string_literals(path):
                if not re.search(r"\b(SELECT|INSERT|UPDATE|DELETE)\b", literal, re.IGNORECASE):
                    continue
                if re.search(r"\bFROM\b|\bINTO\b|\bUPDATE\b", literal, re.IGNORECASE):
                    continue
                fragments[os.path.relpath(path, REPO_ROOT)] += 1
    if fragments:
        notes.append(
            "SQL fragments without a table reference are not column-checked "
            "(concatenated at the call site): "
            + ", ".join(f"{k} ({v})" for k, v in sorted(fragments.items()))
        )

    checked_pairs = check_list_count_consistency(violations, notes)

    unique = sorted(set(violations))
    for note in notes:
        print(f"NOTE: {note}")
    if unique:
        print(f"SQL column check FAILED ({len(unique)} violation(s)):")
        for item in unique:
            print(f"  - {item}")
        return 1
    print(
        f"SQL column check OK: {len(tables)} tables, "
        "every table/column reference in internal/infra/postgres/**/*.go resolves"
        f", {checked_pairs} list/COUNT pairs agree"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
