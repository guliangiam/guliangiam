#!/usr/bin/env bash
# validate_schema_replay.sh — 数据库 schema 全量重放验证（开发/CI 专用）
#
# 依据：《GuliangIAM 开发规范手册》第 3 章「数据库设计规范（多数据库兼容版 V3）」3.1 / 3.8 检查清单
# 用法：
#   DATABASE_URL="postgres://user:pass@host:5432/db?sslmode=disable" \
#   BOOTSTRAP_ADMIN_PASSWORD="provide-a-strong-one-time-secret" \
#     bash scripts/dev/validate_schema_replay.sh
#
# 前置：DATABASE_URL 的连接角色必须为 superuser 或具备 BYPASSRLS；
#       运行期 RLS 角色会把已写入的种子行读成 0，脚本会在开始前直接拒绝（exit 2）。
#
# 验证内容（对应验收条件 AC-DATA-004）：
#   1. scripts/dev/reset_schema.sql 重置 schema
#   2. db/postgresql/V1 基线连续执行两次（V2+ 增量存在时同样按序重放）
#   3. db/seeds/V1 基线全量种子连续执行两次（种子幂等）
#   4. 结构断言：金额精度、币种宽度、声明时间类型、软删除部分唯一索引、核心外键、
#      id 无库函数默认值、租户上下文函数与 RLS 覆盖
#   5. UPSERT 仲裁目标可解析性
#   6. 迁移账本与回滚链：结构/数据双链独立、重放不新增账本行、校验和漂移被拒、
#      数据库版本超前于镜像被拒；V2+ down 回到冻结 V1，再用 reset_schema 验证全新供给
#
# 任一 psql 调用返回非零（含 ON_ERROR_STOP 捕获的 SQL 错误）即判定失败。
# 该脚本会清空目标库的 guliangiam schema，严禁指向生产库。

# 输入：
# 输出：
# --check 行为：
# 消费方：
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"

: "${DATABASE_URL:?请设置 DATABASE_URL，例如 postgres://postgres:postgres@localhost:5432/guliangiam?sslmode=disable}"
: "${BOOTSTRAP_ADMIN_PASSWORD:?请设置符合策略的 BOOTSTRAP_ADMIN_PASSWORD，用于首次初始化内置管理员}"

if [ "${ALLOW_SCHEMA_RESET:-}" != "1" ]; then
  echo "该脚本会 DROP SCHEMA guliangiam CASCADE。确认目标是非生产库后，设置 ALLOW_SCHEMA_RESET=1 重跑。" >&2
  exit 2
fi

# 角色前置校验：断言使用「行数」判定种子是否就绪（如 organizations count），
# 但平台表启用 RLS 后，非 superuser 且无 BYPASSRLS 的连接会把已写入的行读成 0，
# 产生假红并让结论不可用于验收。这里前置硬失败，而不是打印告警后继续产出
# 可被误读的结果。
role_name=$(psql "$DATABASE_URL" -tAc "SELECT current_user")
role_super=$(psql "$DATABASE_URL" -tAc "SELECT rolsuper FROM pg_roles WHERE rolname = current_user")
role_bypass=$(psql "$DATABASE_URL" -tAc "SELECT rolbypassrls FROM pg_roles WHERE rolname = current_user")
if [ "$role_super" != "t" ] && [ "$role_bypass" != "t" ]; then
  echo "FAIL: DATABASE_URL 连接角色 '$role_name' 既非 superuser 也无 BYPASSRLS。" >&2
  echo "      本脚本以行数判定种子就绪与账本状态，RLS 会把不可见行过滤成 0（假红）。" >&2
  echo "      请改用具备 superuser 或 BYPASSRLS 的维护 DSN，例如：" >&2
  echo "        DATABASE_URL=postgres://postgres@localhost:5432/<隔离库>?sslmode=disable" >&2
  exit 2
fi
echo "角色校验通过：role=$role_name (rolsuper=$role_super, rolbypassrls=$role_bypass)"

# 本脚本验证「已应用迁移被编辑必须拒绝」；开发环境的隐式校验和漂移放行会掩盖该行为，
# 因此固定关闭漂移放行，使断言与 APP_ENV 无关。
export ALLOW_CHECKSUM_DRIFT=false

PSQL=(psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -q)

echo "=========================================="
echo "  GuliangIAM schema 全量重放验证"
echo "=========================================="
echo ""

step() {
  echo "--> $1"
  "${PSQL[@]}" -f "$2" >/dev/null
  echo "    OK"
}

mapfile -t schema_chain < <(find db/postgresql -maxdepth 1 -name 'V*.sql' | sort -V)
step "重置 schema (scripts/dev/reset_schema.sql)" scripts/dev/reset_schema.sql
step "V1 首次执行 (冻结基线 DDL)" "${schema_chain[0]}"
step "V1 二次执行 (幂等重放)" "${schema_chain[0]}"
for f in "${schema_chain[@]:1}"; do
  step "增量迁移 ${f##*/}" "$f"
done
mapfile -t seed_chain < <(find db/seeds -maxdepth 1 -name 'V*.sql' | sort -V)
for pass in 1 2; do
  for f in "${seed_chain[@]}"; do
    step "Seed ${f##*/} 第 ${pass} 次执行" "$f"
  done
done

echo ""
echo "--> 结构断言"
assert_zero() {
  local label="$1" query="$2" got
  got=$(psql "$DATABASE_URL" -tAc "$query")
  if [ "$got" != "0" ]; then
    echo "    FAIL: $label（期望 0，实际 $got）" >&2
    exit 1
  fi
  echo "    OK: $label"
}

assert_zero "不存在非 DECIMAL(19,4) 的定点数列" \
  "SELECT count(*) FROM information_schema.columns WHERE table_schema='guliangiam' AND data_type='numeric' AND NOT (numeric_precision=19 AND numeric_scale=4)"

assert_zero "不存在非 VARCHAR(3) 的币种列" \
  "SELECT count(*) FROM information_schema.columns WHERE table_schema='guliangiam' AND column_name LIKE '%currency%' AND data_type='character varying' AND character_maximum_length<>3"

timestamp_expectations=$(python3 - <<'PY'
from pathlib import Path
import yaml

rows = []
for path in Path("contracts/schema").glob("*.yaml"):
    for doc in yaml.safe_load_all(path.read_text()):
        if not doc:
            continue
        table = doc.get("table_name", "").split(".")[-1]
        for column in doc.get("columns", []) or []:
            logical = str(column.get("type", "")).upper()
            if logical not in ("TIMESTAMP", "TIMESTAMPTZ"):
                continue
            expected = "timestamp without time zone" if logical == "TIMESTAMP" else "timestamp with time zone"
            rows.append(f"('{table}','{column['name']}','{expected}')")
print(",".join(rows))
PY
)
if [ -z "$timestamp_expectations" ]; then
  echo "    FAIL: 未从声明式 Schema 解析到时间列类型断言" >&2
  exit 1
fi
assert_zero "时间列类型与声明式 Schema 一致" \
  "SELECT count(*) FROM (VALUES ${timestamp_expectations}) AS expected(table_name,column_name,expected_type) LEFT JOIN information_schema.columns actual ON actual.table_schema='guliangiam' AND actual.table_name=expected.table_name AND actual.column_name=expected.column_name WHERE actual.data_type IS DISTINCT FROM expected.expected_type"

assert_zero "不存在无主键的表" \
  "SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='guliangiam' AND c.relkind='r' AND NOT EXISTS (SELECT 1 FROM pg_constraint x WHERE x.conrelid=c.oid AND x.contype='p')"

assert_zero "id 主键不存在库函数默认值（id 由应用/种子显式提供，ADR-022 §7）" \
  "SELECT count(*) FROM information_schema.columns WHERE table_schema='guliangiam' AND column_name='id' AND udt_name='uuid' AND column_default IS NOT NULL"

assert_zero "租户上下文函数齐备（ADR-016 / ADR-022 §4）" \
  "SELECT 5 - count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='guliangiam' AND p.proname IN ('set_tenant_context','current_organization_id','is_super_admin','resolve_org_id','sync_users_is_deleted')"

assert_zero "带 organization_id 的表全部启用 RLS（ADR-016）" \
  "SELECT count(*) FROM information_schema.columns col JOIN pg_class c ON c.relname=col.table_name AND c.relkind='r' JOIN pg_namespace n ON n.oid=c.relnamespace AND n.nspname=col.table_schema WHERE col.table_schema='guliangiam' AND col.column_name='organization_id' AND NOT c.relrowsecurity"

assert_zero "tenant_isolation 策略与 RLS 表一一对应（ADR-016）" \
  "SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='guliangiam' AND c.relkind='r' AND c.relrowsecurity AND NOT EXISTS (SELECT 1 FROM pg_policies pol WHERE pol.schemaname='guliangiam' AND pol.tablename=c.relname AND pol.policyname='tenant_isolation')"

assert_zero "软删除部分唯一索引齐备" \
  "SELECT 3 - count(*) FROM pg_indexes WHERE schemaname='guliangiam' AND indexname IN ('uq_organizations_slug_active','uq_users_org_username_active','uq_sites_host_active')"

assert_zero "前向引用 FK 已补全" \
  "SELECT 2 - count(*) FROM pg_constraint WHERE conname IN ('fk_users_signup_application_id','fk_app_provider_bindings_provider_id')"

echo ""
echo "--> UPSERT 仲裁目标可解析性断言"
# 扫描 Go 仓储与全部种子里的 «INSERT INTO guliangiam.<表> ... ON CONFLICT (<列>) [WHERE <谓词>]»，
# 校验每个仲裁目标在库中确有列集合一致的唯一约束/唯一索引（且谓词有无一致）。
# 该类缺陷只在真实执行 INSERT 时才暴露，故在重放阶段前置拦截。
#
# 解析必须**与排版无关**：早先的 awk 版本逐行扫描，谓词有无取决于 `WHERE` 是否与
# `ON CONFLICT (...)` 同行 —— 外部格式化器把 `) ON CONFLICT (slug)` 的换行搬走之后，
# 两个部分唯一索引（organizations.slug、users(organization_id, username)）被判定为
# 「无谓词」，断言随即失败并**指向 schema**，而实际是脚本缺陷（与 check-sql-columns.py
# 同类：门禁依赖格式化布局 = 每次编辑都在赌）。
#
# 现改为：先把每个文件还原成**逻辑语句**（剥注释、抽出 Go 反引号里的 SQL、按顶层 `;`
# 切分），再在语句内定位表名与 `ON CONFLICT`，谓词按「`ON CONFLICT (...)` 之后紧跟的
# 首个 token 是否为 WHERE」判定。
#
# 模式仍必须同时接受 `guliangiam.x` 与 `"guliangiam".x`：两种写法在仓储中并存，
# 只认前者会让带引号的 INSERT 解析不到表名 —— 而表名为空的记录会被拼进断言 SQL，
# 结果是一条语法错误的 psql 报错（曾把 `= $pred` 变成 `= ON CONFLICT (...)`），
# 让人误以为 schema 有问题。故此处一并接受引号，并在表名为空时直接失败。
mapfile -t arbiter_files < <(find db/seeds -maxdepth 1 -name 'V*.sql' | sort -V)
while IFS= read -r f; do arbiter_files+=("$f"); done < <(find internal/infra/postgres -name '*.go' | sort)

scan_out=$(mktemp)
if ! python3 - "${arbiter_files[@]}" > "$scan_out" <<'PYSCAN'
import re
import sys

INSERT_RE = re.compile(r'INSERT\s+INTO\s+"?guliangiam"?\s*\.\s*([a-z_][a-z0-9_]*)', re.I)
CONFLICT_RE = re.compile(r'ON\s+CONFLICT\s*\(([^)]*)\)', re.I)


def sql_chunks_from_go(src):
    r"""Return each backtick raw string of a .go file as its own SQL fragment.

    Chunks are kept **separate**, not concatenated: a Go file holds many query
    constants, and joining them would let a later `ON CONFLICT` be attributed to
    an earlier `INSERT` — which is how `provider_configs`' arbiter was once read
    as belonging to `providers`. A query assembled as `` `a` + helper() + `b` ``
    still works for the part that carries the INSERT: the ON CONFLICT of an
    upsert always sits inside the same literal as its INSERT.

    Anything outside the backticks (Go comments, `"..."` strings, code) is
    dropped, so an `ON CONFLICT` mentioned in a comment cannot be mistaken for a
    query. `//` inside a raw string is protected because we only slice on
    backticks here.
    """
    chunks = []
    i, n = 0, len(src)
    while i < n:
        c = src[i]
        if c == '`':
            j = src.find('`', i + 1)
            if j < 0:
                chunks.append(src[i + 1:])
                break
            chunks.append(src[i + 1:j])
            i = j + 1
            continue
        if src.startswith('//', i):
            j = src.find('\n', i)
            i = n if j < 0 else j + 1
            continue
        if src.startswith('/*', i):
            j = src.find('*/', i + 2)
            i = n if j < 0 else j + 2
            continue
        if c == '"':
            i += 1
            while i < n:
                if src[i] == '\\':
                    i += 2
                    continue
                if src[i] == '"':
                    i += 1
                    break
                i += 1
            continue
        i += 1
    return chunks


def strip_sql_comments_and_split(text):
    """Drop SQL comments (string-aware) and split on top-level `;`.

    String literals are kept intact so a `;` inside a string cannot split a
    statement, and `--` inside a string is not treated as a comment.
    """
    buf = []
    stmts = []
    i, n = 0, len(text)
    while i < n:
        c = text[i]
        if text.startswith('--', i):
            j = text.find('\n', i)
            i = n if j < 0 else j + 1
            continue
        if text.startswith('/*', i):
            j = text.find('*/', i + 2)
            i = n if j < 0 else j + 2
            continue
        if c == "'":
            j = i + 1
            while j < n:
                if text[j] == "'":
                    if j + 1 < n and text[j + 1] == "'":
                        j += 2
                        continue
                    break
                j += 1
            buf.append(text[i:j + 1])
            i = j + 1
            continue
        if c == '"':
            j = text.find('"', i + 1)
            j = n - 1 if j < 0 else j
            buf.append(text[i:j + 1])
            i = j + 1
            continue
        dollar = re.match(r'\$[A-Za-z_]*\$', text[i:])
        if dollar:
            tag = dollar.group(0)
            j = text.find(tag, i + len(tag))
            end = n if j < 0 else j + len(tag)
            buf.append(text[i:end])
            i = end
            continue
        if c == ';':
            stmts.append(''.join(buf))
            buf = []
            i += 1
            continue
        buf.append(c)
        i += 1
    stmts.append(''.join(buf))
    return stmts


def scan(paths):
    rows = []
    orphans = []
    for path in paths:
        with open(path, encoding='utf-8') as handle:
            source = handle.read()
        fragments = sql_chunks_from_go(source) if path.endswith('.go') else [source]
        for fragment in fragments:
            for stmt in strip_sql_comments_and_split(fragment):
                conflict = CONFLICT_RE.search(stmt)
                if not conflict:
                    continue
                inserted = INSERT_RE.search(stmt)
                if not inserted:
                    # A conflict clause we cannot attribute to a table means the
                    # pattern missed a spelling. Reporting it is required:
                    # treating "did not parse" as "passed" is how a gate goes
                    # blind.
                    orphans.append(path)
                    continue
                table = inserted.group(1)
                columns = [c.strip().strip('"') for c in conflict.group(1).split(',')]
                columns = ','.join(sorted(c for c in columns if c))
                rest = stmt[conflict.end():]
                predicate = 'true' if re.match(r'\s*WHERE\b', rest, re.I) else 'false'
                rows.append((table, columns, predicate, re.sub(r'\s+', ' ', conflict.group(0)).strip()))
    return rows, orphans


paths = sys.argv[1:]
rows, orphans = scan(paths)
if orphans:
    print('    FAIL: 以下文件含无法定位表名的 ON CONFLICT，脚本模式需更新：', file=sys.stderr)
    for orphan in sorted(set(orphans)):
        print('      ' + orphan, file=sys.stderr)
    sys.exit(3)
if not rows:
    print('    FAIL: 未解析到任何带表名的 ON CONFLICT 仲裁目标（脚本可能失效）', file=sys.stderr)
    sys.exit(4)
for table, columns, predicate, raw in sorted(set(rows)):
    print('\t'.join((table, columns, predicate, raw)))
PYSCAN
then
  rm -f "$scan_out"
  echo "    FAIL: ON CONFLICT 仲裁目标扫描失败（原因见上）" >&2
  exit 1
fi
arbiters=$(cat "$scan_out")
rm -f "$scan_out"

union=""
count=0
while IFS=$'\t' read -r tbl cols pred raw; do
  [ -z "$tbl" ] && continue
  count=$((count + 1))
  clause="SELECT '$raw ($tbl: $cols)' AS arbiter WHERE NOT EXISTS (
      SELECT 1
      FROM pg_index i
      JOIN pg_class c ON c.oid = i.indrelid
      JOIN pg_namespace n ON n.oid = c.relnamespace
      WHERE n.nspname = 'guliangiam'
        AND c.relname = '$tbl'
        AND i.indisunique
        AND ((i.indpred IS NOT NULL) = $pred)
        AND (SELECT string_agg(a.attname, ',' ORDER BY a.attname)
             FROM pg_attribute a
             WHERE a.attrelid = c.oid AND a.attnum = ANY(i.indkey)) = '$cols'
    )"
  if [ -z "$union" ]; then union="$clause"; else union="$union UNION ALL $clause"; fi
done <<< "$arbiters"

bad=$(psql "$DATABASE_URL" -tAc "$union")
if [ -n "$bad" ]; then
  echo "    FAIL: 以下 ON CONFLICT 仲裁目标在库中无匹配的唯一约束/索引：" >&2
  echo "$bad" | sed 's/^/      /' >&2
  exit 1
fi
echo "    OK: $count 个 ON CONFLICT 仲裁目标均有匹配的唯一约束/索引"

echo ""
echo "--> 迁移账本与回滚链（ADR-022 结构门禁 2 / 出口条件 9）"
# 前面几段用 psql 直接重放 SQL，验证 DDL 本身可执行；本段换用真实执行器 (cmd/migrate)，
# 验证只有执行器才承担的语义：双链独立、重放不新增账本行、校验和漂移被拒、
# 数据库版本超前于镜像被拒；V2+ down 反向回到冻结 V1，V1 由 reset_schema 清理。
# 这类语义无法由任何静态门禁证明 —— 生成物零差异只说明「字节稳定」，不说明「语义正确」。
struct_chain_len=$(find db/postgresql -maxdepth 1 -name 'V*.sql' | wc -l | tr -d ' ')
# 数据链版本数以执行器实际注册为准：db/seeds 是种子唯一源，但镜像目录还包含
# data-enrich 版本（如 V2__admin_profile_enrich，go:embed 仅存在于 seed_sql/），
# 执行器会把两者都写入 kind='data' 账本，因此期望值统计镜像目录。
seed_chain_len=$(find internal/bootstrap/seed_sql -maxdepth 1 -name 'V*.sql' | wc -l | tr -d ' ')
mapfile -t rollback_chain < <(find db/rollback -maxdepth 1 -name 'V*.down.sql' | sort -rV)

q() { psql "$DATABASE_URL" -tAc "$1"; }
ledger_rows() { q "SELECT count(*) FROM guliangiam.schema_migrations WHERE kind='$1'"; }
expect_eq() { # expect_eq <label> <want> <got>
  if [ "$3" != "$2" ]; then
    echo "    FAIL: $1（期望 $2，实际 $3）" >&2
    exit 1
  fi
  echo "    OK: $1 = $3"
}
expect_migrate_fails() { # expect_migrate_fails <label> <message fragment>
  local out
  if out=$(AUTO_SEED=false go run ./cmd/migrate up -dsn "$DATABASE_URL" 2>&1); then
    echo "    FAIL: $1（执行器未拒绝）" >&2
    echo "$out" | sed 's/^/      /' >&2
    exit 1
  fi
  if ! printf '%s' "$out" | grep -qF "$2"; then
    echo "    FAIL: $1（拒绝原因不是预期的那一条）" >&2
    printf '%s\n' "$out" | sed 's/^/      /' >&2
    exit 1
  fi
  echo "    OK: $1"
}

"${PSQL[@]}" -f scripts/dev/reset_schema.sql >/dev/null
# 开发环境 AutoSeed 默认开启；本检查验证「结构化迁移不写业务数据」，
# 因此显式关闭 AUTO_SEED，避免把默认值当作被测行为。
if ! AUTO_SEED=false go run ./cmd/migrate up -dsn "$DATABASE_URL" >/dev/null; then
  echo "    FAIL: migrate up 执行失败" >&2
  exit 1
fi
echo "    OK: migrate up 执行成功"
expect_eq "账本记录的结构版本数" "$struct_chain_len" "$(ledger_rows structure)"
expect_eq "数据链默认关闭（未播种）" "0" "$(q 'SELECT count(*) FROM guliangiam.organizations')"

applied_at_before=$(q "SELECT applied_at FROM guliangiam.schema_migrations WHERE kind='structure' AND version=1")
if ! AUTO_SEED=false go run ./cmd/migrate up -dsn "$DATABASE_URL" >/dev/null; then
  echo "    FAIL: migrate up 二次执行失败（须可重放）" >&2
  exit 1
fi
expect_eq "二次执行未新增账本行" "$struct_chain_len" "$(ledger_rows structure)"
expect_eq "二次执行未改写已应用行" "$applied_at_before" \
  "$(q "SELECT applied_at FROM guliangiam.schema_migrations WHERE kind='structure' AND version=1")"

good_checksum=$(q "SELECT checksum FROM guliangiam.schema_migrations WHERE kind='structure' AND version=1")
q "UPDATE guliangiam.schema_migrations SET checksum='deadbeef' WHERE kind='structure' AND version=1" >/dev/null
expect_migrate_fails "已应用迁移被编辑时拒绝启动" "an applied migration must never be edited"
q "UPDATE guliangiam.schema_migrations SET checksum='$good_checksum' WHERE kind='structure' AND version=1" >/dev/null
q "INSERT INTO guliangiam.schema_migrations (kind, version, name, checksum, applied_at, execution_ms)
   VALUES ('structure', $((struct_chain_len + 1)), 'from_the_future', 'x', now(), 0)
   ON CONFLICT (kind, version) DO NOTHING" >/dev/null
expect_migrate_fails "数据库版本超前于镜像时拒绝启动" "this image does not contain"
q "DELETE FROM guliangiam.schema_migrations WHERE kind='structure' AND version=$((struct_chain_len + 1))" >/dev/null

expect_eq "回滚脚本数量（V2 起，冻结 V1 无 down）" "$((struct_chain_len - 1))" "${#rollback_chain[@]}"
for rollback_script in "${rollback_chain[@]}"; do
  if ! "${PSQL[@]}" -f "$rollback_script" >/dev/null 2>&1; then
    echo "    FAIL: 回滚脚本执行失败：$rollback_script" >&2
    exit 1
  fi
done
expect_eq "V2+ down 后保留冻结 V1 schema" "1" \
  "$(q "SELECT count(*) FROM pg_namespace WHERE nspname='guliangiam'")"
expect_eq "V2+ down 后保留冻结 V1 organizations 表" "1" \
  "$(q "SELECT count(*) FROM pg_tables WHERE schemaname='guliangiam' AND tablename='organizations'")"
# forward-only V1 由专用 reset 清理；随后用执行器和完整种子基线重新供给。
"${PSQL[@]}" -f scripts/dev/reset_schema.sql >/dev/null
if ! go run ./cmd/migrate up --seed -dsn "$DATABASE_URL" >/dev/null; then
  echo "    FAIL: 重置后重新供给失败" >&2
  exit 1
fi
expect_eq "重新供给后结构链版本数" "$struct_chain_len" "$(ledger_rows structure)"
expect_eq "重新供给后数据链版本数" "$seed_chain_len" "$(ledger_rows data)"
expect_eq "重新供给后种子数据齐备" "1" "$(q 'SELECT count(*) FROM guliangiam.organizations')"

echo ""
echo "=========================================="
echo "通过：schema 全量重放验证全部通过"
echo "=========================================="
