#!/usr/bin/env bash
# check-migrations.sh — 版本化迁移链完整性 CI 门禁
#
# 依据：ADR-022《声明式 Schema 唯一源与版本化迁移》与《GuliangIAM 开发规范手册》3.1。
# 用法：bash scripts/check-migrations.sh [repo_root]
#
# 检查项（全部阻塞合并）：
#   1. 结构迁移链：db/postgresql/V{N}__{desc}.sql 版本号唯一、自 1 起连续、命名合规
#   2. 回滚配对与回滚顺序：每个结构迁移都有 db/rollback/{同名}.down.sql，且无孤儿回滚脚本；
#      回滚脚本的 DROP 顺序必须是完整外键图的逆拓扑序（建表顺序取反即可删是错的：生成器
#      把前向引用外键延迟到文件尾 ALTER TABLE 补上，取反会在真实库上被依赖拒绝）
#   3. 种子链：db/seeds/V{N}__{desc}.sql 版本号唯一、自 1 起连续、命名合规，目录内不得出现
#      声明层文件（数据链是人工编写的唯一事实源，无生成器、无 YAML，见 ADR-022 决策 6），
#      且每个种子脚本头部必须自述链与文件名（`@seed-chain: authored file={basename}`，
#      改名后残留旧文件名即失败）
#   4. 类别纯度：结构目录禁止数据写入语句；种子脚本禁止 DDL；每条种子 INSERT 必须带 ON CONFLICT
#   5. 种子副本一致：db/seeds/*.sql 与 internal/bootstrap/seed_sql/ 同名文件 SHA-256 一致
#   6. 嵌入覆盖：internal/bootstrap/migrate.go 的 go:embed 覆盖两个脚本目录下的每个 .sql
#   7. 账本单一写入者：guliangiam.schema_migrations 不得被声明式 Schema 声明，也不得在
#      internal/platform/db/ 之外出现写入路径
#   8. 活动 Schema 快照与迁移链连续，最新快照必须与 contracts/schema/*.yaml 字节一致
#
# 说明：结构副本（db/postgresql ↔ internal/bootstrap/schema_sql）与生成物零差异分别由
# check-db-conventions.sh 检查 7、check-generated-zero-diff.sh 覆盖，此处不重复实现。

# 输入：db/postgresql/V{N}__*.sql、db/rollback/*.down.sql、db/seeds/*.sql、internal/bootstrap/seed_sql/、internal/bootstrap/migrate.go
# 输出：PASS/FAIL + 缺失版本/未配对回滚/重复种子副本等
# --check 行为：无 --check；失败即 exit 1
# 消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
set -euo pipefail

repo_root=$(cd "${1:-$(dirname "${BASH_SOURCE[0]}")/..}" && pwd)
cd "$repo_root"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

VIOLATIONS=0
STRUCT_DIR="internal/bootstrap/schema_sql"
ROLLBACK_DIR="db/rollback"
SEED_DIR="db/seeds"
SEED_EMBED_DIR="internal/bootstrap/seed_sql"

fail() {
  echo -e "${RED}  VIOLATION: $*${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
}

pass() {
  echo -e "${GREEN}  OK: $*${NC}"
}

warn() {
  echo -e "${YELLOW}  NOTE: $*${NC}"
}

collect() { # collect <dir> <glob> → 排序后的文件列表
  find "$1" -maxdepth 1 -name "$2" -type f 2>/dev/null | sort
}

name_ok() { # name_ok <basename> → V{N}__{snake_case}.sql
  [[ "$1" =~ ^V[0-9]+__[a-z0-9_]+\.sql$ ]]
}

echo "=========================================="
echo "  GuliangIAM 迁移链完整性门禁"
echo "=========================================="
echo ""

# -------------------------------------------------------------------------- #
echo -e "${YELLOW}[1/7] 结构迁移链检查${NC}"
struct_files=()
while IFS= read -r f; do struct_files+=("$f"); done < <(collect "$STRUCT_DIR" 'V*.sql')

if [ ${#struct_files[@]} -eq 0 ]; then
  fail "未找到结构迁移：$STRUCT_DIR/V{N}__{desc}.sql"
else
  versions=""
  for f in "${struct_files[@]}"; do
    base=$(basename "$f")
    if ! name_ok "$base"; then
      fail "$f 命名不合规（应为 V{N}__{snake_case}.sql）"
      continue
    fi
    v=${base#V}
    v=${v%%__*}
    versions+="$v"$'\n'
  done

  dups=$(printf '%s' "$versions" | grep -v '^$' | sort | uniq -d || true)
  if [ -n "$dups" ]; then
    fail "结构迁移版本号重复：$(echo "$dups" | tr '\n' ' ')"
  fi

  # 自 1 起连续：缺失的版本号意味着存在无法回答的中间态，升级路径不可复现
  expect=1
  while IFS= read -r v; do
    [ -z "$v" ] && continue
    if [ "$v" -lt "$expect" ]; then
      fail "结构迁移版本号乱序或重复：$v（应为 $expect）"
    elif [ "$v" -gt "$expect" ]; then
      fail "结构迁移版本号不连续：缺少 V$expect"
    fi
    expect=$((v + 1))
  done < <(printf '%s' "$versions" | grep -v '^$' | sort -n | uniq)
  [ "$VIOLATIONS" -eq 0 ] && pass "结构迁移链 V1…V$((expect - 1)) 连续且命名合规"
fi

before=$VIOLATIONS
if ! python3 scripts/check-active-schema-history.py; then
  fail "活动 Schema 快照或对应 PostgreSQL 增量产物不完整"
fi
[ "$VIOLATIONS" -eq "$before" ] && pass "活动 Schema 快照连续，最新快照与声明式 Schema 一致"

echo ""
# -------------------------------------------------------------------------- #
echo -e "${YELLOW}[2/7] 回滚配对检查（forward-only 框架，回滚为可选 operator 生成物）${NC}"
before=$VIOLATIONS

# 新架构：结构迁移源 = internal/bootstrap/schema_sql/（单目录，无方言分层）
# 迁移框架 forward-only — rollback 是独立 operator action（由 deploy pipeline
# 执行生成的 db/rollback/*.down.sql，不在运行时或 cmd/migrate 中）。
# 因此本检查在 db/rollback/ 不存在时跳过；存在则与 STRUCT_DIR 配对。

STRUCT_FILES=()
while IFS= read -r f; do STRUCT_FILES+=("$f"); done < <(collect "$STRUCT_DIR" 'V*.sql')

if [ ! -d "$ROLLBACK_DIR" ]; then
  warn "未找到 $ROLLBACK_DIR——forward-only 迁移框架，回滚脚本由部署流水线生成（非运行时需求）"
else
  for f in "${STRUCT_FILES[@]}"; do
    base=$(basename "$f")
    down="$ROLLBACK_DIR/${base%.sql}.down.sql"
    if [ ! -f "$down" ]; then
      base_ver=$(echo "$base" | sed -n 's/^V\([0-9][0-9]*\)__.*/\1/p')
      if [ "$base_ver" = "1" ]; then
        # V1 是冻结全量基线（forward-only）：其回滚由部署流水线备份/重建承担，
        # 不物化 down 脚本；V2+ 增量迁移必须有配对回滚（ADR-022 决策 3）。
        pass "V1 冻结基线为 forward-only（回滚由备份/重建承担），跳过 down 配对"
        continue
      fi
      fail "结构迁移 $f 缺少回滚脚本 $down"
      continue
    fi
    if ! grep -qE '\bDROP\b' "$down"; then
      down_base=$(basename "$down")
      down_ver=$(echo "$down_base" | sed -n 's/^V\([0-9][0-9]*\)__.*/\1/p')
      if [ "$down_ver" = "1" ]; then
        fail "$down 不含 DROP 类语句，可能不是 $base 的逆运算"
      fi
    fi
  done

  # 反向：每个回滚脚本必须能匹配到 STRUCT_DIR 下的正向脚本
  for d in $(collect "$ROLLBACK_DIR" '*.down.sql'); do
    base=$(basename "$d")
    candidate="$STRUCT_DIR/${base%.down.sql}.sql"
    if [ ! -f "$candidate" ]; then
      fail "回滚脚本 $d 无对应结构迁移 $candidate"
    fi
  done

  # 回滚顺序必须是逆依赖序。这一项无法靠人工比对发现：声明里绝大多数外键在建表时内联，
  # 少量前向引用被生成器延迟到文件尾 ALTER TABLE 补上，于是 `tables` 的拓扑序只覆盖前者，
  # 「建表顺序取反」在真实库上会被 `cannot drop table ... because other objects depend on it` 拒绝。
  while IFS= read -r line; do
    [ -n "$line" ] && fail "$line"
  done < <(python3 - <<'PY'
import re
import sys
from pathlib import Path

sys.path.insert(0, "scripts")
from schema_common import load_tables, short_name

short_of = {short_name(t["table_name"]): t for t in load_tables()}
refs = {
    name: {short_name(fk["ref_table"]) for fk in t.get("fk_constraints") or []} - {name}
    for name, t in short_of.items()
}

for down in sorted(Path("db/rollback").glob("V*.down.sql")):
    text = down.read_text(encoding="utf-8")
    drops = re.findall(r"(?m)^DROP TABLE IF EXISTS\s+guliangiam\.([a-z0-9_]+);", text)
    pos = {name: i for i, name in enumerate(drops)}

    m = re.match(r"V(\d+)__", down.name)
    ver = int(m.group(1)) if m else 0
    is_full_rollback = (ver <= 1)

    if is_full_rollback:
        missing = sorted(set(short_of) - set(drops))
        extra = sorted(set(drops) - set(short_of))
        dupes = sorted({n for n in drops if drops.count(n) > 1})
        if missing:
            print(f"{down} 未删除以下声明表：{'、'.join(missing)}")
        if extra:
            print(f"{down} 删除了未声明的表：{'、'.join(extra)}")
        if dupes:
            print(f"{down} 重复删除同一张表：{'、'.join(dupes)}")

    for name, i in pos.items():
        for ref in sorted(refs.get(name, ())):
            if ref in pos and pos[ref] < i:
                print(
                    f"{down} 回滚顺序错误：guliangiam.{name} 早于其引用的 "
                    f"guliangiam.{ref} 被删除（应先删引用方）"
                )

    last_table = max(pos.values()) if pos else -1
    fn_at = [m.start() for m in re.finditer(r"(?m)^DROP FUNCTION IF EXISTS", text)]
    schema_at = [m.start() for m in re.finditer(r"(?m)^DROP SCHEMA IF EXISTS", text)]
    drop_line = {m.start(): m.group(0) for m in re.finditer(r"(?m)^DROP TABLE IF EXISTS", text)}
    table_at = sorted(drop_line)
    for at in fn_at:
        if table_at and at < table_at[-1]:
            print(f"{down} 回滚顺序错误：DROP FUNCTION 出现在 DROP TABLE 之前——RLS 策略依赖函数，会拒绝删除")
            break
    for at in schema_at:
        if table_at and at < table_at[-1]:
            print(f"{down} 回滚顺序错误：DROP SCHEMA 出现在 DROP TABLE 之前")
            break
    if is_full_rollback and not re.search(r"(?m)^DROP SCHEMA IF EXISTS guliangiam CASCADE;", text):
        print(
            f"{down} 末态 DROP SCHEMA 未使用 CASCADE——schema 内还有执行器自建的迁移账本表，"
            "用 RESTRICT 会以「schema is not empty」失败"
        )
PY
  )
fi
[ "$VIOLATIONS" -eq "$before" ] && pass "结构迁移与回滚脚本一一配对"
echo ""
# -------------------------------------------------------------------------- #
echo -e "${YELLOW}[3/7] 种子链完整性与编写契约检查${NC}"
before=$VIOLATIONS
seed_files=()
while IFS= read -r f; do seed_files+=("$f"); done < <(collect "$SEED_DIR" 'V*.sql')

if [ ${#seed_files[@]} -eq 0 ]; then
  warn "未找到版本化种子脚本：$SEED_DIR/V{N}__{desc}.sql"
else
  # 数据链没有声明层：任何非 .sql 文件（尤指 *.yaml 声明）都是第二份表示，即漂移面。
  # 仅忽略操作系统元数据（macOS Finder 会在目录被浏览时重建 .DS_Store）。
  for stray in $(collect "$SEED_DIR" '*' | grep -v -E '\.sql$|(^|/)\.DS_Store$' || true); do
    fail "$stray 不是版本化种子脚本——数据链为人工编写的唯一事实源，禁止声明层文件（ADR-022 决策 6）"
  done

  seed_versions=""
  for f in "${seed_files[@]}"; do
    base=$(basename "$f")
    if ! name_ok "$base"; then
      fail "$f 命名不合规（应为 V{N}__{snake_case}.sql）"
      continue
    fi
    v=${base#V}
    v=${v%%__*}
    seed_versions+="$v"$'\n'

    # 头部自述契约：改名后残留旧文件名（历史缺陷：脚本自称 db_seed_init.sql）即失败
    marker=$(grep -m1 -E '^--[[:space:]]*@seed-chain:' "$f" || true)
    if [ -z "$marker" ]; then
      fail "$f 头部缺少种子链契约行（-- @seed-chain: authored file=$base）"
      continue
    fi
    if [ "$marker" != "-- @seed-chain: authored file=$base" ]; then
      fail "$f 种子链契约行不合规：期望 '-- @seed-chain: authored file=$base'，实际 '$marker'"
    fi
  done
  dups=$(printf '%s' "$seed_versions" | grep -v '^$' | sort | uniq -d || true)
  [ -n "$dups" ] && fail "种子脚本版本号重复：$(echo "$dups" | tr '\n' ' ')"

  expect=1
  while IFS= read -r v; do
    [ -z "$v" ] && continue
    [ "$v" -ne "$expect" ] && fail "种子脚本版本号不连续：期望 V$expect，实际 V$v"
    expect=$((v + 1))
  done < <(printf '%s' "$seed_versions" | grep -v '^$' | sort -n | uniq)
fi
[ "$VIOLATIONS" -eq "$before" ] && pass "种子链连续、无声明层、契约行自述文件名"

echo ""
# -------------------------------------------------------------------------- #
echo -e "${YELLOW}[4/7] 类别纯度检查${NC}"
before=$VIOLATIONS

# 豁免名单：单行状态表（CHECK id=1 或类似约束）的初始化 INSERT。
# 这些 INSERT 是结构定义的一部分（表生命周期约束），而不是业务种子数据。
# 条件：目标表有 CHECK 单行约束 + INSERT 带 ON CONFLICT DO NOTHING 幂等。
# 当前生成的结构迁移不含单行状态表 INSERT，因此豁免名单为空；
# 若未来增量迁移需要该例外，按实际文件名重新登记。
STRUCT_INSERT_EXEMPT=(
)

for f in "${struct_files[@]:-}"; do
  [ -z "$f" ] && continue
  base=$(basename "$f")
  if [[ " ${STRUCT_INSERT_EXEMPT[*]} " =~ " ${base} " ]]; then
    exempt_hits=$(grep -nEi '^[[:space:]]*(INSERT[[:space:]]+INTO|UPDATE[[:space:]]|DELETE[[:space:]]+FROM|COPY[[:space:]])' "$f" || true)
    if [ -n "$exempt_hits" ]; then
      warn "$f 含数据写入语句——豁免：单行状态表结构初始化（CHECK 单行 + ON CONFLICT DO NOTHING）"
    fi
    continue
  fi
  hits=$(grep -nEi '^[[:space:]]*(INSERT[[:space:]]+INTO|UPDATE[[:space:]]|DELETE[[:space:]]+FROM|COPY[[:space:]])' "$f" || true)
  if [ -n "$hits" ]; then
    fail "$f 含数据写入语句——结构迁移只允许 DDL，数据必须放 $SEED_DIR"
    echo "$hits" | sed 's/^/      /'
  fi
done
for f in "${seed_files[@]:-}"; do
  [ -z "$f" ] && continue
  hits=$(grep -nEi '^[[:space:]]*(CREATE[[:space:]]+(TABLE|INDEX|UNIQUE|SCHEMA|TRIGGER|FUNCTION|POLICY)|ALTER[[:space:]]+TABLE|DROP[[:space:]]|TRUNCATE)' "$f" || true)
  if [ -n "$hits" ]; then
    fail "$f 含 DDL——种子脚本只允许数据写入，结构必须放 $STRUCT_DIR"
    echo "$hits" | sed 's/^/      /'
  fi
  # 语句级（非行级）判定：生成物把一条语句压成多行，行级匹配会漏判
  read -r inserts idempotent < <(awk '
    { line = $0; sub(/--.*$/, "", line); buf = buf " " line }
    /;[[:space:]]*$/ {
      u = toupper(buf); buf = ""
      if (u ~ /INSERT[[:space:]]+INTO/) {
        total++
        if (u ~ /ON[[:space:]]+CONFLICT/) ok++
      }
    }
    END { printf "%d %d\n", total + 0, ok + 0 }
  ' "$f")
  if [ "$inserts" -gt "$idempotent" ]; then
    fail "$f 有 $((inserts - idempotent)) 条 INSERT 缺少 ON CONFLICT——种子脚本必须幂等可重放"
  fi

  # 仲裁键安全校验（OPEN-API-34 根因修复）：
  # 固定 UUID 主键的种子 INSERT 不能用只指向非主键唯一约束的仲裁键。
  # PostgreSQL 的 ON CONFLICT (arbiter) 只在指定的那个唯一约束上检查冲突。
  # 当主键冲突但仲裁键约束上**没有**冲突（如数据被损坏导致 id 与内容不匹配），
  # ON CONFLICT 看不到冲突，主键冲突直接报 duplicate key。
  # 正确写法二选一：
  #   1. ON CONFLICT DO NOTHING（不带仲裁键——覆盖所有唯一约束含主键）
  #   2. ON CONFLICT (..., id, ...) 仲裁键必须包含主键列 id
  read -r uuid_pk_inserts unsafe_arbiters < <(python3 - "$f" <<'PY'
import re, sys

sql = open(sys.argv[1], encoding='utf-8').read()

# 去掉注释
no_comments = re.sub(r'--[^\n]*', '', sql)
no_comments = re.sub(r'/\*.*?\*/', '', no_comments, flags=re.DOTALL)

# 按 INSERT 语句分割：从 INSERT INTO 开始，到对应的分号结束
# 用非贪婪匹配 + 平衡括号来正确识别
uuid_pk_count = 0
unsafe_count = 0

# 找所有 INSERT INTO ... ; 语句
# 简化处理：用栈匹配括号 + 直到语句末尾分号
lines = no_comments.split('\n')
i = 0
while i < len(lines):
    line = lines[i]
    if not re.search(r'(?i)^\s*INSERT\s+INTO', line):
        i += 1
        continue

    # 开始收集一条 INSERT 语句
    buf = line
    depth = 0
    i += 1
    while i < len(lines):
        buf += ' ' + lines[i]
        # 统计括号深度
        depth += lines[i].count('(') - lines[i].count(')')
        if depth <= 0 and ';' in lines[i]:
            break
        i += 1

    stmt = buf.strip()
    if not stmt:
        continue

    # 检测固定 UUID 主键：VALUES 中出现 UUID 字面量 + ::uuid 后缀
    has_uuid_pk = bool(re.search(r"'[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}'::uuid", stmt, re.IGNORECASE))
    if not has_uuid_pk:
        continue

    uuid_pk_count += 1

    # 检测 ON CONFLICT 形式
    conflict_match = re.search(r'(?i)ON\s+CONFLICT\s*\(([^)]*)\)\s*DO', stmt)
    if not conflict_match:
        # 不带括号的 ON CONFLICT DO NOTHING — 安全
        continue

    arbiter = conflict_match.group(1).strip().lower()
    arbiter_cols = set(re.split(r'[,\s]+', arbiter))
    arbiter_cols.discard('')

    if 'id' not in arbiter_cols:
        unsafe_count += 1

print(f"{uuid_pk_count} {unsafe_count}")
PY
)

  if [ "$unsafe_arbiters" -gt 0 ]; then
    fail "$f 有 $unsafe_arbiters 条固定 UUID 主键的 INSERT 使用不含 id 的仲裁键——重放时主键冲突 ON CONFLICT 可能看不到（OPEN-API-34），应改用 ON CONFLICT DO NOTHING 或仲裁键包含 id"
  fi
done
[ "$VIOLATIONS" -eq "$before" ] && pass "结构/种子脚本类别纯净且种子幂等"

echo ""
# -------------------------------------------------------------------------- #
echo -e "${YELLOW}[5/7] 种子脚本双副本一致性检查${NC}"
before=$VIOLATIONS
if [ ! -d "$SEED_DIR" ]; then
  fail "种子源目录 $SEED_DIR 不存在——不能校验镜像一致性"
fi
if [ ! -d "$SEED_EMBED_DIR" ]; then
  fail "种子嵌入目录 $SEED_EMBED_DIR 不存在——migrate.go go:embed 会失败"
fi
for src in $(collect "$SEED_DIR" 'V*.sql'); do
  base=$(basename "$src")
  dst="$SEED_EMBED_DIR/$base"
  if [ ! -f "$dst" ]; then
    fail "缺少嵌入副本 $dst"
    continue
  fi
  a=$(shasum -a 256 "$src" | awk '{print $1}')
  b=$(shasum -a 256 "$dst" | awk '{print $1}')
  if [ "$a" != "$b" ]; then
    fail "$base 与 $dst 内容不一致（SHA-256 不同）"
    echo "      src:   $a"
    echo "      embed: $b"
  fi
done
for dst in $(collect "$SEED_EMBED_DIR" 'V*.sql'); do
  base=$(basename "$dst")
  # Non-seed data migrations (V2+, e.g. profile enrich) live in seed_sql/
  # but not in db/seeds/ — they skip the mirror check. Seed versions mirror
  # the seed dir one-to-one; data-enrich versions are declared directly in
  # migrate.go and have no seed counterpart.
  version=$(echo "$base" | sed 's/V\([0-9]*\)__.*/\1/')
  if [ ! -f "$SEED_DIR/$base" ]; then
    # Accept only if every seed file version strictly precedes this one —
    # prevents silent mirror drift on older seed versions while allowing
    # newer data-enrich versions (V2+) to stand alone.
    seed_max_version=0
    for seed_file in $(collect "$SEED_DIR" 'V*.sql'); do
      seed_v=$(basename "$seed_file" | sed 's/V\([0-9]*\)__.*/\1/')
      seed_max_version=$(( seed_v > seed_max_version ? seed_v : seed_max_version ))
    done
    if [ "$version" -gt "$seed_max_version" ]; then
      continue
    fi
    fail "嵌入副本 $dst 无对应源文件 $SEED_DIR/$base（孤儿副本）"
  fi
done
[ "$VIOLATIONS" -eq "$before" ] && pass "$SEED_DIR/ 与 $SEED_EMBED_DIR/ 逐字节一致"

echo ""
# -------------------------------------------------------------------------- #
echo -e "${YELLOW}[6/7] 嵌入覆盖检查${NC}"
before=$VIOLATIONS
BOOTSTRAP="internal/bootstrap/migrate.go"
if [ ! -f "$BOOTSTRAP" ]; then
  fail "缺少 $BOOTSTRAP"
else
  for src in $(collect "$STRUCT_DIR" 'V*.sql'); do
    base=$(basename "$src")
    grep -q "go:embed schema_sql/$base" "$BOOTSTRAP" || fail "$BOOTSTRAP 未嵌入 schema_sql/$base"
  done
  for src in $(collect "$SEED_DIR" 'V*.sql'); do
    base=$(basename "$src")
    grep -q "go:embed seed_sql/$base" "$BOOTSTRAP" || fail "$BOOTSTRAP 未嵌入 seed_sql/$base"
  done

  # 声明顺序必须与迁移版本链一致：声明里的版本号即执行顺序，错序会导致升级路径错乱
  prev=0
  for v in $(grep -oE 'Kind: db\.KindStructure, Version: [0-9]+' "$BOOTSTRAP" | grep -oE '[0-9]+$'); do
    [ "$v" -le "$prev" ] && fail "$BOOTSTRAP 结构迁移声明未按版本升序：$v 出现在 $prev 之后"
    prev=$v
  done
fi
[ "$VIOLATIONS" -eq "$before" ] && pass "全部迁移脚本已嵌入且声明有序"

echo ""
# -------------------------------------------------------------------------- #
echo -e "${YELLOW}[7/7] 迁移账本单一写入者检查${NC}"
before=$VIOLATIONS
declared=$(grep -rln "schema_migrations" contracts/schema/ 2>/dev/null || true)
if [ -n "$declared" ]; then
  fail "声明式 Schema 声明了迁移账本表（应由执行器独占）：$declared"
fi
# 豁免名单：仅包含常量/枚举定义，非写入路径
SCHEMA_MIGRATIONS_EXEMPT=(
  "internal/domain/regions/tier.go"   # 分类枚举映射表，纯常量定义
)
stray_raw=$(grep -rln "schema_migrations" --include='*.go' internal/ cmd/ 2>/dev/null | grep -v '^internal/platform/db/' || true)
stray=""
for f in $stray_raw; do
  exempt=false
  for e in "${SCHEMA_MIGRATIONS_EXEMPT[@]}"; do
    if [ "$f" = "$e" ]; then exempt=true; break; fi
  done
  if [ "$exempt" = false ]; then
    stray="${stray:+$stray$'\n'}$f"
  fi
done
if [ -n "$stray" ]; then
  fail "迁移账本只允许在 internal/platform/db/ 中读写，其他位置出现：$stray"
fi
embed_stray=$(grep -rln "CREATE TABLE.*schema_migrations" --include='*.sql' db/postgresql/ db/rollback/ db/generated/ internal/bootstrap/ 2>/dev/null || true)
if [ -n "$embed_stray" ]; then
  fail "迁移账本不得进入生成物或迁移脚本：$embed_stray"
fi
[ "$VIOLATIONS" -eq "$before" ] && pass "迁移账本由 internal/platform/db 独占"

echo ""
echo "=========================================="
if [ "$VIOLATIONS" -eq 0 ]; then
  echo -e "${GREEN}  PASS: 迁移链完整${NC}"
  echo "=========================================="
  exit 0
fi
echo -e "${RED}  FAIL: $VIOLATIONS 处问题${NC}"
echo "=========================================="
exit 1
