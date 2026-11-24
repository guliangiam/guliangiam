#!/usr/bin/env bash
# align_schema_db.sh — 把既有开发库对齐到声明式 Schema（开发/CI 专用；只新增补齐，绝不删除）
#
# 依据：docs/GuliangIAM-开发设计.md「声明式 Schema 为唯一源」与《GuliangIAM 开发规范手册》
#       第 19 章变更流程（`contracts/schema/*.yaml` → 生成 `db/postgresql/V{N}__*.sql`；产物全量幂等、
#       不含破坏性语句）。
#
# 背景：迁移执行器只执行「账本未记录」的版本。V1 被改写后，账本已记录过 V1 的旧库不会重放，
#       于是新建的表/列/约束/索引在旧库中缺失，并且启动会被校验和漂移拦下（见
#       internal/platform/db/migrate.go 的 "an applied migration must never be edited"）。
#       本脚本按「库 vs 声明式 Schema 产物」的差异，生成**仅含新增类语句**的对齐事务并（可选）执行。
#
# 用法：
#   DATABASE_URL=postgres://… bash scripts/dev/align_schema_db.sh            # 只报告差异 + 打印将执行的 SQL（有漂移退出码 1）
#   DATABASE_URL=postgres://… bash scripts/dev/align_schema_db.sh --apply    # 执行对齐；并把账本校验和修到当前镜像
#   DATABASE_URL=… bash scripts/dev/align_schema_db.sh --apply --no-ledger   # 执行对齐但不改账本
#
# 安全边界：
#   1. 只生成 CREATE TABLE IF NOT EXISTS / ALTER TABLE … ADD COLUMN IF NOT EXISTS /
#      ALTER TABLE … ADD CONSTRAINT / CREATE [UNIQUE] INDEX 以及 RLS 语句，**不含**任何
#      DROP / TRUNCATE / 改列类型 —— 不存在数据丢失路径。
#   2. 全部语句在单个事务内执行（psql ON_ERROR_STOP=1）；任一步失败整体回滚，库保持原样。
#      （新增 NOT NULL 列若在已有数据上违规、或新 CHECK/UNIQUE 与既有数据冲突，都会在此失败并回滚。）
#   3. 不做「删除」对齐：库中多出的对象只在报告中列出，绝不自动删除。
#
# 退出码：0 = 已对齐（或对齐成功）；1 = 检测到漂移；2 = 用法/环境错误；3 = 对齐执行失败。

# 输入：
# 输出：
# --check 行为：
# 消费方：
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"

usage() { sed -n '2,26p' "$0"; }

DB_URL="${DATABASE_URL:-${POSTGRES_DSN:-}}"
if [ -z "$DB_URL" ]; then
  echo "请设置 DATABASE_URL（或 POSTGRES_DSN），例如 postgres://postgres:postgres@localhost:5432/guliangiam?sslmode=disable" >&2
  exit 2
fi

apply=0
repair_ledger=1
for arg in "$@"; do
  case "$arg" in
    --apply) apply=1 ;;
    --no-ledger) repair_ledger=0 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "未知参数：$arg（用 -h 查看用法）" >&2; exit 2 ;;
  esac
done

PRODUCT=db/postgresql/V1__guliangiam_schema_init.sql
[ -f "$PRODUCT" ] || { echo "缺少生成产物：$PRODUCT（先运行 python3 scripts/gen_schema.py）" >&2; exit 2; }

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
live_dump="$tmpdir/live.tsv"
sql_out="$tmpdir/align.sql"

PSQL=(psql "$DB_URL" -v ON_ERROR_STOP=1 -At)

echo "=========================================="
echo "  库 vs Schema 对齐（$(printf '%s' "$DB_URL" | sed 's/?.*$//')）"
echo "=========================================="

# 采集库内对象：T=表 C=列 I=索引 K=约束 R=启用 RLS 的表。单次查询，TSV 输出。
collect_live() {
  "${PSQL[@]}" -F $'\t' -c "
SELECT 'T', tablename, '' FROM pg_tables WHERE schemaname='guliangiam'
UNION ALL SELECT 'C', table_name, column_name FROM information_schema.columns WHERE table_schema='guliangiam'
UNION ALL SELECT 'I', indexname, '' FROM pg_indexes WHERE schemaname='guliangiam'
UNION ALL SELECT 'K', conname, '' FROM pg_constraint c JOIN pg_namespace n ON n.oid=c.connamespace
  WHERE n.nspname='guliangiam' AND c.contype <> 'n'
UNION ALL SELECT 'R', c.relname, '' FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
  WHERE n.nspname='guliangiam' AND c.relrowsecurity AND c.relkind='r'
" > "$live_dump"
}

run_diff() {
  # 退出码：0=已对齐，3=存在漂移（由 python 判定）
  python3 - "$PRODUCT" "$live_dump" "$sql_out" <<'PYALIGN'
import re
import sys

product_path, live_path, out_path = sys.argv[1], sys.argv[2], sys.argv[3]
product = open(product_path, encoding='utf-8').read()

sys.path.insert(0, 'scripts')
from schema_common import load_tables, parse_ddl, short_name


def split_statements(text):
    """按顶层 ';' 切分 SQL，字符串/美元引用/注释安全（与 validate_schema_replay.sh 同源手法）。"""
    buf, stmts = [], []
    i, n = 0, len(text)
    while i < n:
        if text.startswith('--', i):
            j = text.find('\n', i)
            i = n if j < 0 else j + 1
            continue
        if text.startswith('/*', i):
            j = text.find('*/', i + 2)
            i = n if j < 0 else j + 2
            continue
        c = text[i]
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
        d = re.match(r'\$[A-Za-z_]*\$', text[i:])
        if d:
            tag = d.group(0)
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
    return [s.strip() for s in stmts if s.strip()]


QUAL = r'"?guliangiam"?\s*\.\s*'
# 产物中 CREATE TABLE / ALTER TABLE 带 schema 限定，而索引名是不带限定的裸名，故统一用可选限定
QUAL_OPT = r'(?:' + QUAL + r')?'
stmts = split_statements(product)

# --- 产物侧（= 声明式 Schema）：以生成产物为唯一事实源 ---
declared_tables = set()
for s in stmts:
    m = re.match(r'CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?' + QUAL_OPT + r'"?([a-z_]\w*)"?\s*\(', s, re.I)
    if m:
        declared_tables.add(m.group(1))

idx_stmt = {}
for s in stmts:
    m = re.match(r'CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?' + QUAL_OPT + r'"?([a-z_]\w*)"?', s, re.I)
    if m:
        idx_stmt[m.group(1)] = s

# 约束：定位每条 `CONSTRAINT <name>` 在 CREATE TABLE 体内的完整片段（括号/逗号感知）
con_stmt = {}
for s in stmts:
    owner = re.match(r'CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?' + QUAL_OPT + r'"?([a-z_]\w*)"?\s*\(', s, re.I)
    if not owner:
        continue
    table = owner.group(1)
    for m in re.finditer(r'\bCONSTRAINT\s+"?([A-Za-z_]\w*)"?\s', s):
        start, depth, i = m.start(), 0, m.end()
        while i < len(s):
            ch = s[i]
            if ch == '(':
                depth += 1
            elif ch == ')':
                if depth == 0:
                    break
                depth -= 1
            elif ch == ',' and depth == 0:
                break
            i += 1
        seg = re.sub(r'\s+', ' ', s[start:i]).strip()
        con_stmt[m.group(1)] = ('inline', table, seg)

# 受控 DO 块（延迟外键等）：整块自身幂等（每个约束用 `conname = '…'` 守卫），
# 因此按其守卫的约束名登记；只要其中任一名缺失，就整块重放（重放安全）。
dobl_blocks = []
for s in stmts:
    if 'ADD CONSTRAINT' not in s:
        continue
    guarded = set(re.findall(r"conname\s*=\s*'([A-Za-z_]\w*)'", s))
    if guarded:
        dobl_blocks.append((s, guarded))
        for name in guarded:
            con_stmt[name] = ('dobl', len(dobl_blocks) - 1)

# 表 -> 归属该表的语句组（CREATE / INDEX ON / ALTER TABLE / RLS 策略 DO 块）
def owner_tables(s):
    names = set()
    m = re.match(r'CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?' + QUAL_OPT + r'"?([a-z_]\w*)"?', s, re.I)
    if m:
        names.add(m.group(1))
    for m in re.finditer(r'\bON\s+' + QUAL + r'"?([a-z_]\w*)"?', s, re.I):
        names.add(m.group(1))
    for m in re.finditer(r'\bALTER\s+TABLE\s+(?:ONLY\s+)?' + QUAL_OPT + r'"?([a-z_]\w*)"?', s, re.I):
        names.add(m.group(1))
    for m in re.finditer(r"tablename\s*=\s*'([a-z_]\w*)'", s, re.I):
        names.add(m.group(1))
    return names


groups = {}
for s in stmts:
    for t in owner_tables(s):
        groups.setdefault(t, []).append(s)

ddl = {short_name(k): v for k, v in parse_ddl(product).items()}
col_defs = {t: v.get('columns', {}) for t, v in ddl.items()}

exp_cols, exp_rls = {}, set()
for t in load_tables():
    name = short_name(t['table_name'])
    exp_cols[name] = {(c['name'] if isinstance(c, dict) else c) for c in (t.get('columns') or [])}
    if t.get('rls'):
        exp_rls.add(name)

# --- 库侧 ---
live_tables, live_cols, live_idx, live_con, live_rls = set(), {}, set(), set(), set()
with open(live_path, encoding='utf-8') as fh:
    for line in fh:
        parts = line.rstrip('\n').split('\t')
        if not parts or not parts[0]:
            continue
        kind = parts[0]
        if kind == 'T':
            live_tables.add(parts[1])
        elif kind == 'C':
            live_cols.setdefault(parts[1], set()).add(parts[2])
        elif kind == 'I':
            live_idx.add(parts[1])
        elif kind == 'K':
            live_con.add(parts[1])
        elif kind == 'R':
            live_rls.add(parts[1])

# 账本表是执行器基础设施，不属于声明式 Schema
live_tables.discard('schema_migrations')
live_cols.pop('schema_migrations', None)

# --- 差异 ---
missing_tables = sorted(declared_tables - live_tables)
missing_cols = {
    t: sorted(exp_cols.get(t, set()) - live_cols.get(t, set()))
    for t in exp_cols
    if t not in missing_tables and exp_cols.get(t, set()) - live_cols.get(t, set())
}
missing_con = sorted(set(con_stmt) - live_con)
missing_idx = sorted(set(idx_stmt) - live_idx)
missing_rls = sorted(t for t in exp_rls - live_rls if t not in missing_tables)

# 库中多出的对象：排除账本表相关、与已声明约束同名的回填索引、以及 PG 自动命名（*_pkey/*_key/*_fkey）。
# 唯一约束在库中会以同名索引出现，故索引集合需并上 con_stmt。
def is_extra(name):
    if 'schema_migrations' in name:
        return False
    return not re.search(r'(_pkey|_key|_fkey)$', name)


extra_tables = sorted(t for t in live_tables - declared_tables)
extra_idx = sorted(n for n in live_idx - set(idx_stmt) - set(con_stmt) if is_extra(n))
extra_con = sorted(n for n in live_con - set(con_stmt) if is_extra(n))

drift = bool(missing_tables or missing_cols or missing_con or missing_idx or missing_rls)


def report(title, items):
    if items:
        print('  [漂移] %s (%d)：%s' % (title, len(items), ', '.join(items)))


report('缺失表', missing_tables)
for t, cols in sorted(missing_cols.items()):
    print('  [漂移] 缺失列 %s.%s (%d)' % (t, '/'.join(cols), len(cols)))
report('缺失约束', missing_con)
report('缺失索引', missing_idx)
report('缺失 RLS', missing_rls)

if extra_tables or extra_idx or extra_con:
    print('  [提示] 库中多出、Schema 未定义（仅列出，绝不自动删除）：')
    if extra_tables:
        print('         多出表：%s' % ', '.join(extra_tables))
    if extra_idx:
        print('         多出索引：%s' % ', '.join(extra_idx))
    if extra_con:
        print('         多出约束：%s' % ', '.join(extra_con))

# --- 生成对齐 SQL（仅新增类语句）---
# 统一经 add() 收集：① 补齐语句结尾分号（split_statements 已剥离）；② 对同一语句去重。
# 缺失表的 CREATE TABLE 组已内含其内联约束/索引/RLS，重建时不得再单独 ADD 一遍。
out = []
seen = set()


def add(stmt):
    if stmt not in seen:
        seen.add(stmt)
        out.append(stmt)


for t in missing_tables:
    for s in groups.get(t, []):
        add(s + ';')
for t, cols in sorted(missing_cols.items()):
    for c in cols:
        add('ALTER TABLE guliangiam.%s ADD COLUMN IF NOT EXISTS %s %s;'
            % (t, c, col_defs[t][c]))
blocks_needed = set()
for n in missing_con:
    meta = con_stmt[n]
    if meta[0] == 'dobl':
        blocks_needed.add(meta[1])
    else:
        _, table, seg = meta
        if table in missing_tables:
            continue  # 内联约束已随 CREATE TABLE 一并创建
        add('ALTER TABLE guliangiam.%s ADD %s;' % (table, seg))
for i in sorted(blocks_needed):
    add(dobl_blocks[i][0] + ';')
for n in missing_idx:
    add(idx_stmt[n] + ';')
for t in missing_rls:
    for s in groups.get(t, []):
        if re.search(r'ROW\s+LEVEL\s+SECURITY|CREATE\s+POLICY|pg_policies', s, re.I):
            add(s + ';')

with open(out_path, 'w', encoding='utf-8') as fh:
    fh.write('\n'.join(out) + ('\n' if out else ''))

print('  %s' % ('存在结构漂移' if drift else '库与 Schema 完全对齐'))
sys.exit(3 if drift else 0)
PYALIGN
}

collect_live
set +e
run_diff
diff_rc=$?
set -e

if [ "$diff_rc" = "0" ] && [ "$apply" = "0" ]; then
  echo ""
  echo "对齐检查通过：库结构与声明式 Schema 一致。"
  exit 0
fi

if [ "$diff_rc" != "0" ] && [ "$diff_rc" != "3" ]; then
  echo "差异分析失败（退出码 $diff_rc）" >&2
  exit 3
fi

# 账本校验和修复：把 structure/data 账本行修到当前镜像产物哈希（仅当账本表存在）
ledger_sql="$tmpdir/ledger.sql"
: > "$ledger_sql"
if [ "$repair_ledger" = "1" ]; then
  has_ledger=$("${PSQL[@]}" -c "SELECT COALESCE(to_regclass('guliangiam.schema_migrations')::text,'')")
  if [ -n "$has_ledger" ]; then
    for pair in "structure:internal/bootstrap/schema_sql" "data:internal/bootstrap/seed_sql"; do
      kind=${pair%%:*}; dir=${pair#*:}
      for f in "$dir"/V*.sql; do
        [ -e "$f" ] || continue
        ver=$(basename "$f" | sed -n 's/^V\([0-9][0-9]*\)__.*/\1/p')
        [ -n "$ver" ] || continue
        sum=$(shasum -a 256 "$f" | awk '{print $1}')
        printf "UPDATE guliangiam.schema_migrations SET checksum='%s' WHERE kind='%s' AND version=%s;\n" \
          "$sum" "$kind" "$ver" >> "$ledger_sql"
      done
    done
  else
    echo "  [提示] 未发现账本表 guliangiam.schema_migrations，跳过校验和修复。"
  fi
fi

if [ "$apply" = "0" ]; then
  echo ""
  echo "== 将执行的对齐语句（未执行；加 --apply 执行）=="
  cat "$sql_out"
  echo ""
  echo "检测到漂移。用 --apply 执行对齐。" >&2
  exit 1
fi

combined="$tmpdir/combined.sql"
{
  echo "BEGIN;"
  # FORCE ROW LEVEL SECURITY 下，非超级用户会话看不到被租户策略隔离的行；外键校验会因此
  # 误判「父行不存在」而使整笔对齐失败（典型：enforcers 有行但被 RLS 挡住）。以 super_admin
  # 身份运行对齐事务即可让校验读到全量行；set_config(..., true) 为事务局部，随提交/回滚失效。
  echo "SELECT set_config('app.current_role','super_admin', true);"
  cat "$sql_out"
  cat "$ledger_sql"
  echo "COMMIT;"
} > "$combined"

echo ""
echo "== 执行对齐事务 =="
if ! "${PSQL[@]}" -f "$combined"; then
  echo "对齐执行失败：事务已回滚，库保持原样。" >&2
  exit 3
fi
echo "    对齐事务已提交。"

echo ""
echo "== 复核 =="
collect_live
set +e
run_diff
verify_rc=$?
set -e
if [ "$verify_rc" != "0" ]; then
  echo "复核失败：仍存在漂移。" >&2
  exit 3
fi

echo ""
echo "=========================================="
echo "通过：库已对齐到声明式 Schema"
echo "=========================================="
