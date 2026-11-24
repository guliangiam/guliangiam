#!/usr/bin/env bash
# cleanup_extra_objects.sh — 删除开发库中「库 vs 声明式 Schema 差异」里 Schema 未定义的多出对象
#
# 依据：docs/GuliangIAM-开发设计.md「声明式 Schema 为唯一源 — 产物是唯一事实源」。
# 背景：迁移执行器只执行「账本未记录」的版本，旧命名（例如 V1 改写前的 ck_*_state /
#       uq_identitypools_org_name）会在库中残留。align_schema_db.sh 只做补齐、绝不删除，
#       本脚本负责**安全清理**「对齐脚本报告的多出对象」。
#
# 用法：
#   DATABASE_URL=postgres://… bash scripts/dev/cleanup_extra_objects.sh           # 仅报告（默认 dry-run）
#   DATABASE_URL=postgres://… ALLOW_SCHEMA_CLEANUP=1 \
#     bash scripts/dev/cleanup_extra_objects.sh --apply                            # 执行 DROP
#
# 安全边界（三重守卫，缺一即拒绝）：
#   1. 仅处理 align_schema_db.sh 报告的「多出约束 / 多出索引」；
#      多出表一律拒绝 DROP（必须先人工确认）。
#   2. 多出对象所在表的 pg_stat_user_tables.n_live_tup 必须为 0；
#      含数据的表，其任何多出对象都不会被触碰。
#   3. --apply 必须显式设置 ALLOW_SCHEMA_CLEANUP=1 环境变量（与 reset_schema.sql
#      的 ALLOW_SCHEMA_RESET 同风格）。
#
# 退出码：0=无多出对象或 dry-run 成功；1=存在多出对象；2=用法/环境/守卫拒绝；
#         3=DROP 执行失败或复核仍有多出对象。

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
for arg in "$@"; do
  case "$arg" in
    --apply) apply=1 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "未知参数：$arg（用 -h 查看用法）" >&2; exit 2 ;;
  esac
done

PRODUCT=db/postgresql/V1__guliangiam_schema_init.sql
[ -f "$PRODUCT" ] || { echo "缺少生成产物：$PRODUCT（先运行 python3 scripts/gen_schema.py）" >&2; exit 2; }

if [ "$apply" = "1" ] && [ "${ALLOW_SCHEMA_CLEANUP:-}" != "1" ]; then
  echo "该脚本会执行 DROP CONSTRAINT / DROP INDEX。确认目标是非生产库且所在表均为空后，" >&2
  echo "设置 ALLOW_SCHEMA_CLEANUP=1 重跑（默认仅报告不执行）。" >&2
  exit 2
fi

PSQL=(psql "$DB_URL" -v ON_ERROR_STOP=1 -At -q)

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
live_dump="$tmpdir/live.tsv"
clean_sql="$tmpdir/cleanup.sql"

echo "=========================================="
echo "  库 vs Schema 多出对象清理（$(printf '%s' "$DB_URL" | sed 's/?.*$//')）"
echo "=========================================="

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

collect_live

# 生产清理 SQL：python 端判定「多出对象 + 表空」条件，shell 端执行
python3 - "$PRODUCT" "$live_dump" "$clean_sql" <<'PYCLEAN'
import re, sys
product_path, live_path, out_path = sys.argv[1], sys.argv[2], sys.argv[3]
product = open(product_path, encoding='utf-8').read()

sys.path.insert(0, 'scripts')
from schema_common import parse_ddl, load_tables, short_name


def split_statements(text):
    buf, stmts = [], []
    i, n = 0, len(text)
    while i < n:
        if text.startswith('--', i):
            j = text.find('\n', i); i = n if j < 0 else j + 1; continue
        if text.startswith('/*', i):
            j = text.find('*/', i + 2); i = n if j < 0 else j + 2; continue
        c = text[i]
        if c == "'":
            j = i + 1
            while j < n:
                if text[j] == "'":
                    if j + 1 < n and text[j + 1] == "'": j += 2; continue
                    break
                j += 1
            buf.append(text[i:j + 1]); i = j + 1; continue
        if c == '"':
            j = text.find('"', i + 1); j = n - 1 if j < 0 else j
            buf.append(text[i:j + 1]); i = j + 1; continue
        d = re.match(r'\$[A-Za-z_]*\$', text[i:])
        if d:
            tag = d.group(0)
            j = text.find(tag, i + len(tag)); end = n if j < 0 else j + len(tag)
            buf.append(text[i:end]); i = end; continue
        if c == ';':
            stmts.append(''.join(buf)); buf = []; i += 1; continue
        buf.append(c); i += 1
    stmts.append(''.join(buf))
    return [s.strip() for s in stmts if s.strip()]


QUAL_OPT = r'(?:(?:"?guliangiam"?\s*\.\s*)?|)'
stmts = split_statements(product)
declared_tables, declared_idx, declared_con = set(), set(), set()

for s in stmts:
    m = re.match(r'CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?' + QUAL_OPT + r'"?([a-z_]\w*)"?', s, re.I)
    if m: declared_tables.add(m.group(1))

for s in stmts:
    m = re.match(r'CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?' + QUAL_OPT + r'"?([a-z_]\w*)"?', s, re.I)
    if m: declared_idx.add(m.group(1))

for s in stmts:
    m = re.match(r'CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?' + QUAL_OPT + r'"?([a-z_]\w*)"?\s*\(', s, re.I)
    if not m: continue
    table = m.group(1)
    for mm in re.finditer(r'\bCONSTRAINT\s+"?([A-Za-z_]\w*)"?\s', s):
        start, depth, i = mm.start(), 0, mm.end()
        while i < len(s):
            ch = s[i]
            if ch == '(': depth += 1
            elif ch == ')':
                if depth == 0: break
                depth -= 1
            elif ch == ',' and depth == 0: break
            i += 1
        declared_con.add(mm.group(1))
for s in stmts:
    if 'ADD CONSTRAINT' not in s: continue
    for name in re.findall(r"conname\s*=\s*'([A-Za-z_]\w*)'", s):
        declared_con.add(name)

# 库侧
live_tables, live_idx, live_con = set(), set(), set()
with open(live_path, encoding='utf-8') as fh:
    for line in fh:
        parts = line.rstrip('\n').split('\t')
        if not parts or not parts[0]: continue
        if parts[0] == 'T': live_tables.add(parts[1])
        elif parts[0] == 'I': live_idx.add(parts[1])
        elif parts[0] == 'K': live_con.add(parts[1])
live_tables.discard('schema_migrations')

# 多出对象
def is_extra(name):
    if 'schema_migrations' in name: return False
    return not re.search(r'(_pkey|_key|_fkey)$', name)

extra_tables = sorted(t for t in live_tables - declared_tables)
extra_idx = sorted(n for n in live_idx - declared_idx - declared_con if is_extra(n))
extra_con = sorted(n for n in live_con - declared_con if is_extra(n))

print('--- 多出对象检测 ---')
if extra_tables:
    print('  多出表（脚本拒绝 DROP，需人工确认）：%s' % ', '.join(extra_tables))
if extra_idx:
    print('  多出索引：%s' % ', '.join(extra_idx))
if extra_con:
    print('  多出约束：%s' % ', '.join(extra_con))
if not extra_idx and not extra_con and not extra_tables:
    print('  无 Schema 未定义的多出对象。库与 Schema 完全对齐。')

# 输出 out_path 内容：DROP 语句（仅空表上）；若输出为空则无安全清理目标。
# 索引清理需关联表名 — 从 pg_indexes 抓；约束同理 pg_constraint → pg_class。
out = []
for idx in extra_idx:
    # 唯一约束会自动以同名索引实现；若 extra_con 中存在同名约束，
    # DROP CONSTRAINT CASCADE 会连带删除该索引，不要先单独 DROP INDEX。
    if idx in extra_con:
        continue
    out.append(('INDEX', idx, None, 'DROP INDEX IF EXISTS guliangiam."%s";' % idx))
for con in extra_con:
    # 需查表名 — 留给 shell 端单独处理（更简单）
    out.append(('CON', con, None, None))

# 写出：首行是 JSON 元信息；后续是 DROP 候选（待 shell 端补全表关联后再执行）
import json
with open(out_path, 'w', encoding='utf-8') as fh:
    fh.write('# meta: %s\n' % json.dumps({'extra_idx': extra_idx, 'extra_con': extra_con}))
    for kind, name, _, _ in out:
        fh.write('%s\t%s\n' % (kind, name))
PYCLEAN

echo ""

# --- shell 端：对每个候选，查表名 + 检查表是否空 + 生成 DROP ---
candidates="$tmpdir/candidates.tsv"
tail -n +2 "$clean_sql" > "$candidates" || true

if [ ! -s "$candidates" ]; then
  echo "无安全可清理对象。"
  exit 0
fi

plan="$tmpdir/plan.sql"
> "$plan"

total_ok=0; total_blocked=0

while IFS=$'\t' read -r kind name; do
  [ -z "$name" ] && continue
  if [ "$kind" = "INDEX" ]; then
    # 从 pg_indexes 取索引关联的表
    tbl=$("${PSQL[@]}" -c "SELECT tablename FROM pg_indexes WHERE schemaname='guliangiam' AND indexname='${name}' LIMIT 1;")
    if [ -z "$tbl" ]; then
      echo "  [跳过] 索引 %s：未找到关联表" "$name"
      total_blocked=$((total_blocked + 1)); continue
    fi
    rows=$("${PSQL[@]}" -c "SELECT n_live_tup FROM pg_stat_user_tables WHERE schemaname='guliangiam' AND relname='${tbl}';")
    rows=${rows:-0}
    if [ "$rows" != "0" ]; then
      echo "  [跳过] 索引 %s：表 %s 有 %s 行" "$name" "$tbl" "$rows"
      total_blocked=$((total_blocked + 1)); continue
    fi
    printf "  [OK]   DROP INDEX IF EXISTS guliangiam.%s  （表 %s 空）\n" "$name" "$tbl"
    printf 'DROP INDEX IF EXISTS guliangiam."%s";\n' "$name" >> "$plan"
    total_ok=$((total_ok + 1))
  elif [ "$kind" = "CON" ]; then
    tbl=$("${PSQL[@]}" -c "SELECT c.relname FROM pg_constraint x JOIN pg_class c ON c.oid=x.conrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='guliangiam' AND x.conname='${name}' LIMIT 1;")
    if [ -z "$tbl" ]; then
      echo "  [跳过] 约束 %s：未找到关联表" "$name"
      total_blocked=$((total_blocked + 1)); continue
    fi
    rows=$("${PSQL[@]}" -c "SELECT n_live_tup FROM pg_stat_user_tables WHERE schemaname='guliangiam' AND relname='${tbl}';")
    rows=${rows:-0}
    if [ "$rows" != "0" ]; then
      echo "  [跳过] 约束 %s：表 %s 有 %s 行" "$name" "$tbl" "$rows"
      total_blocked=$((total_blocked + 1)); continue
    fi
    printf "  [OK]   ALTER TABLE guliangiam.%s DROP CONSTRAINT IF EXISTS %s CASCADE  （空）\n" "$tbl" "$name"
    printf 'ALTER TABLE guliangiam."%s" DROP CONSTRAINT IF EXISTS "%s" CASCADE;\n' "$tbl" "$name" >> "$plan"
    total_ok=$((total_ok + 1))
  fi
done < "$candidates"

echo ""
printf '%s\n' "-- 清理计划：${total_ok} 个安全 DROP，${total_blocked} 个跳过"

if [ "$total_ok" = "0" ]; then
  echo "无安全可清理对象，退出。"
  exit 0
fi

if [ "$apply" = "0" ]; then
  echo ""
  echo "== 将执行的 DROP 语句（未执行；加 --apply 并 ALLOW_SCHEMA_CLEANUP=1 执行）=="
  cat "$plan"
  echo ""
  echo "存在 Schema 未定义的多出对象。用 --apply 执行清理。" >&2
  exit 1
fi

combined="$tmpdir/combined.sql"
{
  echo "BEGIN;"
  echo "SELECT set_config('app.current_role','super_admin', true);"
  cat "$plan"
  echo "COMMIT;"
} > "$combined"

echo ""
echo "== 执行清理事务 =="
if ! "${PSQL[@]}" -f "$combined"; then
  echo "清理执行失败：事务已回滚，库保持原样。" >&2
  exit 3
fi
echo "    清理事务已提交。"

echo ""
echo "== 复核 =="
collect_live
# 复核用 align_schema_db.sh 的 dry-run 检查 —— 退出码 0 表示已完全对齐
set +e
diff_out=$(bash scripts/dev/precheck_cleanup.sh 2>&1)
diff_rc=$?
set -e
if [ "$diff_rc" != "0" ]; then
  echo "复核后仍有多出对象或漂移：" >&2
  echo "$diff_out" >&2
  exit 3
fi
echo "$diff_out"

echo ""
echo "=========================================="
echo "通过：清理完成，库已对齐到声明式 Schema"
echo "=========================================="
