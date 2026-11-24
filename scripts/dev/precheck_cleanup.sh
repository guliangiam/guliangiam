#!/usr/bin/env bash
# precheck_cleanup.sh — 生产库「Schema 多出对象清理」前的只读预检（零写操作）
#
# 依据：docs/GuliangIAM-开发设计.md「声明式 Schema 为唯一源」；三阶段清理流程：
#   align_schema_db.sh（补齐）→ precheck_cleanup.sh（预检）
#   → cleanup_extra_objects.sh --apply（执行）。预检是执行前强制闸口。
#
# 安全分级矩阵：
#   CHECK 约束             → 绿（DROP 不动数据）
#   普通索引               → 绿（仅影响查询速度）
#   UNIQUE 约束（表空/有新约束兜底）→ 绿；否则 → 黄
#   FK 约束（一端空）      → 绿；否则 → 黄（孤儿行）
#   表（任何情况）          → 红（禁止自动）
#
# 退出码：0 全绿/可执行；1 有红黄需审批；2 环境错；3 DB 异常

# 输入：
# 输出：
# --check 行为：
# 消费方：
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"

DB_URL="${DATABASE_URL:-${POSTGRES_DSN:-}}"
[ -z "$DB_URL" ] && { echo "请设置 DATABASE_URL（或 POSTGRES_DSN）" >&2; exit 2; }
command -v python3 >/dev/null 2>&1 || { echo "缺少 python3" >&2; exit 2; }
command -v psql >/dev/null 2>&1 || { echo "缺少 psql" >&2; exit 2; }

PRODUCT=db/postgresql/V1__guliangiam_schema_init.sql
[ -f "$PRODUCT" ] || { echo "缺少 $PRODUCT（先 gen_schema.py）" >&2; exit 2; }

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

echo "## 预检报告 — Schema 多出对象清理"
echo ""
echo "- 库：$(printf '%s' "$DB_URL" | sed 's/?.*$//')"
echo "- 时间：$(date '+%Y-%m-%d %H:%M:%S')"

# 核心：纯 python 完成所有对比、分级、硬编码扫描、Markdown 报告生成
# psql 只读调用也通过 python subprocess 发起，彻底避免 shell 多层嵌套的语法陷阱。
python3 - "$PRODUCT" "$DB_URL" <<'PYREPORT'
import os, re, sys, subprocess, json
from pathlib import Path

product_path = sys.argv[1]
db_url = sys.argv[2]

product = open(product_path, encoding='utf-8').read()
sys.path.insert(0, str(Path.cwd() / 'scripts'))
from schema_common import short_name

# --- 工具 ---
def psql(sql):
    r = subprocess.run(
        ['psql', db_url, '-v', 'ON_ERROR_STOP=1', '-At', '-q', '-F\t', '-c', sql],
        capture_output=True, text=True
    )
    if r.returncode != 0:
        raise RuntimeError(f"psql failed: {r.stderr.strip()[:200]}")
    return [l.split('\t') for l in r.stdout.rstrip('\n').split('\n') if l]

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

# --- 声明侧 ---
stmts = split_statements(product)
declared_t, declared_i, declared_k = set(), set(), set()
QUAL_OPT = r'(?:(?:"?guliangiam"?\s*\.\s*)?|)'
for s in stmts:
    m = re.match(r'CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?' + QUAL_OPT + r'"?([a-z_]\w*)"?', s, re.I)
    if m: declared_t.add(m.group(1))
for s in stmts:
    m = re.match(r'CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?' + QUAL_OPT + r'"?([a-z_]\w*)"?', s, re.I)
    if m: declared_i.add(m.group(1))
for s in stmts:
    m = re.match(r'CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?' + QUAL_OPT + r'"?([a-z_]\w*)"?\s*\(', s, re.I)
    if m:
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
            declared_k.add(mm.group(1))
for s in stmts:
    if 'ADD CONSTRAINT' not in s: continue
    for name in re.findall(r"conname\s*=\s*'([A-Za-z_]\w*)'", s):
        declared_k.add(name)

# --- 库侧 ---
live_sql = """
SELECT 'T', tablename, '' FROM pg_tables WHERE schemaname='guliangiam'
UNION ALL SELECT 'I', indexname, '' FROM pg_indexes WHERE schemaname='guliangiam'
UNION ALL SELECT 'K', conname, '' FROM pg_constraint c JOIN pg_namespace n ON n.oid=c.connamespace
  WHERE n.nspname='guliangiam' AND c.contype <> 'n'
"""
live_rows = psql(live_sql)
live_t, live_i, live_k = set(), set(), set()
for r in live_rows:
    if not r or not r[0]: continue
    if r[0] == 'T': live_t.add(r[1])
    elif r[0] == 'I': live_i.add(r[1])
    elif r[0] == 'K': live_k.add(r[1])
live_t.discard('schema_migrations')

def is_extra(name):
    if 'schema_migrations' in name: return False
    return not re.search(r'(_pkey|_key|_fkey)$', name)

extra_t = sorted(t for t in live_t - declared_t)
extra_i = sorted(n for n in live_i - declared_i - declared_k if is_extra(n))
extra_k = sorted(n for n in live_k - declared_k if is_extra(n))

# --- 逐项元数据 ---
rows_md, act_md = {}, {}
if extra_t:
    for t in extra_t:
        row = psql(f"SELECT COALESCE(n_live_tup,0), (SELECT count(*) FROM pg_stat_activity WHERE state='active' AND query ILIKE '%{t}%' AND pid<>pg_backend_pid()) FROM pg_stat_user_tables WHERE schemaname='guliangiam' AND relname='{t}'")
        if row: rows_md[t], act_md[t] = row[0][0], row[0][1]

# 约束/索引元数据
items = []  # (level, kind, name, ctype, tbl, rows, active, fallback, lock_risk, reason)

# 表
for name in extra_t:
    r = rows_md.get(name, '?'); a = act_md.get(name, '?')
    items.append(('red', '表', name, 'TABLE', name, r, a, '—', '—', '整表删除 = 丢数据，禁止自动'))

# 约束
kmeta = {}
if extra_k:
    rows = psql(f"""
        SELECT c.conname, c.contype, t.relname, pg_get_constraintdef(c.oid)
        FROM pg_constraint c JOIN pg_class t ON t.oid=c.conrelid JOIN pg_namespace n ON n.oid=t.relnamespace
        WHERE n.nspname='guliangiam' AND c.conname IN ({','.join("'"+x+"'" for x in extra_k)})
    """)
    for conname, ctype, tbl, cdef in rows:
        kmeta[conname] = (ctype, tbl, cdef)

# 索引 → 表关联
idx_tbl = {}
if extra_i:
    rows = psql(f"SELECT indexname, tablename, indisunique FROM pg_indexes i JOIN pg_index ix ON ix.indexrelid=i.indexname::regclass WHERE schemaname='guliangiam' AND indexname IN ({','.join("'"+x+"'" for x in extra_i)})")
    for idx, tbl, uniq in rows:
        idx_tbl[idx] = (tbl, uniq == 't')

# 代码硬编码扫描
code_hits = []
skip_dirs = {'vendor', 'docs', '.git', 'db/postgresql', 'db/mysql', 'scripts'}
for name in extra_i + extra_k + extra_t:
    for root, dirs, files in os.walk('.'):
        dirs[:] = [d for d in dirs if d not in skip_dirs]
        for fn in files:
            if fn.endswith('.go') or fn.endswith('.sql'):
                fp = os.path.join(root, fn)
                try:
                    with open(fp, encoding='utf-8', errors='ignore') as fh:
                        content = fh.read()
                    if name in content:
                        code_hits.append((name, fp))
                except: pass

# --- 安全分级 ---
def grade(kind, name, tbl, extra_info=None):
    rows = psql(f"SELECT COALESCE(n_live_tup,0) FROM pg_stat_user_tables WHERE schemaname='guliangiam' AND relname='{tbl}'")
    row_cnt = int(rows[0][0]) if rows else 0
    actv = psql(f"SELECT count(*) FROM pg_stat_activity WHERE state='active' AND query ILIKE '%{tbl}%' AND pid<>pg_backend_pid()")
    actv_cnt = int(actv[0][0]) if actv else 0

    if kind == 'INDEX':
        is_uniq = extra_info or False
        if is_uniq and row_cnt > 0:
            return ('yellow', f'DROP UNIQUE 索引 → 允许重复行；表非空({row_cnt})', '中' if actv_cnt > 0 else '低', row_cnt, actv_cnt)
        return ('green', '普通索引 DROP 不动数据，仅影响查询速度', '低', row_cnt, actv_cnt)

    if kind == 'CONSTRAINT':
        ctype, cdef = extra_info or ('c', '')
        if ctype == 'c':
            return ('green', 'CHECK 约束纯语义守卫；DROP 不触碰数据', '低', row_cnt, actv_cnt)
        if ctype == 'u':
            # 兜底：旧 _state → 新 _status
            base = re.sub(r'_state$', '_status', name)
            fb = psql(f"SELECT conname FROM pg_constraint WHERE conrelid='guliangiam.{tbl}'::regclass AND conname='{base}' AND conname<>'{name}'")
            if row_cnt == 0:
                return ('green', 'UNIQUE 表空，DROP 安全', '低', row_cnt, actv_cnt)
            if fb:
                return ('green', f'同表已存在新命名 UNIQUE {base} 兜底', '低', row_cnt, actv_cnt)
            return ('yellow', f'DROP UNIQUE 后允许重复行；表非空({row_cnt})且无兜底', '中', row_cnt, actv_cnt)
        if ctype == 'f':
            parent = re.search(r'REFERENCES\s+([a-z_][a-z0-9_]*)', cdef)
            parent = parent.group(1) if parent else ''
            pr = psql(f"SELECT COALESCE(n_live_tup,0) FROM pg_stat_user_tables WHERE schemaname='guliangiam' AND relname='{parent or tbl}'")
            parent_rows = int(pr[0][0]) if pr else 0
            if row_cnt == 0 or parent_rows == 0:
                return ('green', f'FK 一端为空({tbl}={row_cnt},{parent}={parent_rows})，DROP 安全', '低', row_cnt, actv_cnt)
            return ('yellow', f'DROP FK 后可能产生孤儿行；{tbl}={row_cnt},{parent}={parent_rows}', '中', row_cnt, actv_cnt)
        return ('yellow', f'未知约束类型 {ctype}，请人工判定', '中', row_cnt, actv_cnt)

    return ('red', '整表删除 = 丢数据，禁止自动', '—', row_cnt, actv_cnt)

# 收集全部分级后 item
buckets = {'green': [], 'yellow': [], 'red': []}

for name in extra_t:
    lvl, reason, lock, rc, ac = grade('TABLE', name, name)
    buckets[lvl].append(('TABLE', name, name, 'TABLE', rc, ac, '—', lock, reason))

for conname in extra_k:
    if conname not in kmeta: continue
    ctype, tbl, cdef = kmeta[conname]
    lvl, reason, lock, rc, ac = grade('CONSTRAINT', conname, tbl, (ctype, cdef))
    ctype_label = {'c':'CHECK','u':'UNIQUE','f':'FK','p':'PRIMARY KEY'}.get(ctype, ctype)
    fb = '—'
    if ctype == 'u':
        base = re.sub(r'_state$', '_status', conname)
        fb_rows = psql(f"SELECT conname FROM pg_constraint WHERE conrelid='guliangiam.{tbl}'::regclass AND conname='{base}' AND conname<>'{conname}'")
        if fb_rows: fb = base
    buckets[lvl].append(('CONSTRAINT', conname, tbl, ctype_label, rc, ac, fb, lock, reason))

for idx in extra_i:
    if idx not in idx_tbl: continue
    tbl, is_uniq = idx_tbl[idx]
    lvl, reason, lock, rc, ac = grade('INDEX', idx, tbl, is_uniq)
    label = 'INDEX(UNIQUE)' if is_uniq else 'INDEX'
    fb = '—'
    if is_uniq:
        base = re.sub(r'_state$', '_status', idx)
        fb_rows = psql(f"SELECT conname FROM pg_constraint WHERE conrelid='guliangiam.{tbl}'::regclass AND conname='{base}'")
        if fb_rows: fb = base
    buckets[lvl].append(('INDEX', idx, tbl, label, rc, ac, fb, lock, reason))

# --- 输出 Markdown ---
total_g = len(buckets['green'])
total_y = len(buckets['yellow'])
total_r = len(buckets['red'])

print(f"- 表: {len(extra_t)} | 索引: {len(extra_i)} | 约束: {len(extra_k)} | 合计: {len(extra_t)+len(extra_i)+len(extra_k)}")
print()

if not extra_t and not extra_i and not extra_k:
    print("### 🟢 库已完全对齐，无需清理。")
    sys.exit(0)

# 总览
print("### 总览")
print()
print("| 类型 | 数量 |")
print("|---|---|")
print(f"| 表（Schema 未定义） | {len(extra_t)} |")
print(f"| 索引 | {len(extra_i)} |")
print(f"| 约束 | {len(extra_k)} |")
print(f"| **合计** | {len(extra_t)+len(extra_i)+len(extra_k)} |")
print()

# 代码硬编码
print("### 代码硬编码引用扫描")
print()
unique_hits = {}
for name, fp in code_hits:
    # 只扫 .go 源码 + SQL（跳过 docs/ 是靠 os.walk 已经排除了）
    unique_hits.setdefault(name, set()).add(fp)

if not unique_hits:
    print("✅ **无硬编码引用**。旧命名仅出现在 DDL 产物中，应用层不依赖。")
else:
    print(f"⚠️  发现 {len(unique_hits)} 处硬编码引用：")
    print()
    for name in sorted(unique_hits):
        for fp in sorted(unique_hits[name]):
            print(f"- `{name}` ← `{fp}`")
    print()
    print("  **风险**：若应用代码有 `switch err.Error()` 匹配约束字符串或 `ADD CONSTRAINT` 硬编码旧名，")
    print("  DROP 后对应分支失效。建议在清理前 grep 确认是否涉及错误处理路径。")
print()

# 分级表
def bucket(title, items, icon):
    print(f"### {icon} {title}（{len(items)} 项）")
    print()
    if not items:
        print("*—*")
        print()
        return
    print("| # | 类型 | 名称 | 所属表 | 行数 | 活跃查询 | 兜底 | 锁风险 | 备注 |")
    print("|---|------|------|--------|------|----------|------|--------|------|")
    for i, (kind, name, tbl, ctype, rc, ac, fb, lock, reason) in enumerate(items, 1):
        print(f"| {i} | {ctype} | `{name}` | `{tbl}` | {rc} | {ac} | {fb} | {lock} | {reason} |")
    print()

bucket("绿区", buckets['green'], "🟢")
bucket("黄区", buckets['yellow'], "🟡")
bucket("红区", buckets['red'], "🔴")

# 锁全景
print("### 锁影响全景")
print()
print("所有 DROP CONSTRAINT / DROP INDEX 均需 ACCESS EXCLUSIVE 锁。下表列出涉及表的活跃度：")
print()
print("| 表 | n_live_tup | 活跃查询 | 最后 vacuum | 热点 |")
print("|---|-----------|----------|-------------|------|")

all_tables = set()
for b in buckets.values():
    for _, _, tbl, _, _, _, _, _, _ in b:
        all_tables.add(tbl)

sql_m = f"""SELECT relname, COALESCE(n_live_tup,0),
  (SELECT count(*) FROM pg_stat_activity WHERE state='active' AND query ILIKE '%' || relname || '%' AND pid<>pg_backend_pid()),
  COALESCE(TO_CHAR(last_vacuum, 'YYYY-MM-DD HH24:MI:SS'), TO_CHAR(last_autovacuum, 'YYYY-MM-DD HH24:MI:SS'), 'never')
FROM pg_stat_user_tables WHERE schemaname='guliangiam' AND relname IN ({','.join("'"+t+"'" for t in all_tables)})"""
for relname, rc, ac, lv in psql(sql_m):
    hot = "是" if ac != '0' else "否"
    print(f"| `{relname}` | {rc} | {ac} | {lv} | {hot} |")
print()

# 行动建议
print("### 行动建议")
print()

if total_r > 0:
    print("🚨 **红区存在 — 禁止自动清理**。红区涉及整表 DROP 或高风险对象，必须 DBA 手工审批后单独处理。")
    print()
    print("建议：")
    print("  1. 先人工确认红区对象是否真的废弃（可能是脚本侧误判）")
    print("  2. 若确需清理，走正式变更窗口 + 先备份 + 先 align_schema_db.sh 补齐")
    print("  3. cleanup_extra_objects.sh 默认拒绝 DROP 表，需脚本增强 ALLOW_DROP_TABLE=1 额外守卫")
    print()

if total_y > 0:
    print("🟡 **黄区存在 — 需审批 + 低峰窗口执行**。黄区对象 DROP 会让数据库丢失一部分约束/唯一索引能力，建议：")
    print()
    print("  1. 在低峰时段（凌晨 2-5 点）执行")
    print("  2. 先确认新约束/新索引已就位（`align_schema_db.sh --apply` 已跑过）")
    print("  3. 执行前立即 `pg_dump -t 'guliangiam.*'` 备份一次")
    print("  4. 执行后立即验证关键查询无性能回退（EXPLAIN ANALYZE 对比）")
    print("  5. 若为 FK 且两端非空，考虑先 `DELETE FROM …` 清理孤儿行再 DROP")
    print()

if total_g > 0:
    print("🟢 **绿区可安全执行**。可跳过审批，直接用 cleanup_extra_objects.sh 在生产窗口内任意时段执行：")
    print()
    print("```bash")
    print("# 1. 先补齐声明式 Schema 新增的对象（若尚未执行）")
    print("ALLOW_CHECKSUM_DRIFT=true DATABASE_URL=… bash scripts/dev/align_schema_db.sh --apply")
    print()
    print("# 2. 再执行清理（三重守卫：仅空表 × 仅约束/索引 × ALLOW_SCHEMA_CLEANUP=1）")
    print("ALLOW_SCHEMA_CLEANUP=1 DATABASE_URL=… bash scripts/dev/cleanup_extra_objects.sh --apply")
    print("```")
    print()

print("---")
print("本预检脚本为只读，未执行任何写操作。")

# 退出码分级
if total_r > 0: sys.exit(1)
if total_y > 0: sys.exit(1)
sys.exit(0)
PYREPORT
PYRC=$?

echo ""
echo "--"
echo "退出码：$PYRC（0=全绿可执行；1=有红黄需审批；2=环境错；3=DB 异常）"
exit $PYRC
