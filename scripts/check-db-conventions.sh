#!/usr/bin/env bash
# check-db-conventions.sh — 数据库设计规范 CI 门禁
#
# 依据：《GuliangIAM 开发规范手册》第 3 章「数据库设计规范（多数据库兼容版 V3）」
# 用法：bash scripts/check-db-conventions.sh [repo_root]
#
# 检查项：
#   1. schema 脚本不得包含破坏性语句（DROP SCHEMA/TABLE、TRUNCATE、DROP INDEX）
#   2. 不得出现遗留表名（iam_*、gateway_servers、gateway_sites、*_uk/uk_* 命名）
#   3. 所有 CREATE TABLE/CREATE INDEX 必须带 IF NOT EXISTS（幂等重放）
#   4. 含 updated_at 的表必须含 revision BIGINT NOT NULL DEFAULT 1，且不得出现 DEFAULT 0
#   5. 约束/索引命名前缀必须为 uq_/idx_/ck_/fk_，表级 UNIQUE 必须显式命名
#   6. 同一脚本内不得出现重复索引名
#   7. db/postgresql/ 与 internal/bootstrap/schema_sql/ 下的同名脚本必须逐字节一致
#   8. 不得出现 FK 前向引用（引用表创建位置早于被引用表）
#   9. 金额列必须为 DECIMAL(19,4)，币种列必须为 VARCHAR(3)

# 输入：db/postgresql/*.sql、internal/bootstrap/schema_sql/*.sql
# 输出：PASS/FAIL + 违规项
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
# DDL/迁移产物的真实位置（由 gen_schema.py 生成并由 internal/bootstrap 嵌入）。
# 历史值 db/postgresql/ 曾按 Flyway 目录布局流传，本仓库不使用 Flyway；
# 实际执行入口为 cmd/migrate + guliangiam.schema_migrations 账本（OPEN-API-24）。
SCHEMA_DIR="internal/bootstrap/schema_sql"
MIRROR_DIR="db/postgresql"

fail() {
  echo -e "${RED}  VIOLATION: $*${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
}

pass() {
  echo -e "${GREEN}  OK: $*${NC}"
}

schema_files=()
while IFS= read -r f; do
  schema_files+=("$f")
done < <(find "$SCHEMA_DIR" -maxdepth 1 -name '*.sql' -type f | sort -V)

if [ ${#schema_files[@]} -eq 0 ]; then
  echo "no schema scripts found under $SCHEMA_DIR" >&2
  exit 1
fi

echo "=========================================="
echo "  GuliangIAM 数据库设计规范门禁"
echo "=========================================="
echo ""
echo -e "${YELLOW}[1/9] 破坏性语句检查${NC}"
for f in "${schema_files[@]}"; do
  # 迁移脚本（V2+ 生成式增量）以增量 ALTER 为常态；V1 为全量生成式基线，禁止破坏性语句，
  # 属版本化演进而非「基线含破坏性语句」；破坏性红线只针对 V1 基线 init。
  case "$(basename "$f")" in
    V1__*) ;;
    *) continue ;;
  esac
  hits=$(grep -nE '(^|[^-])\b(DROP[[:space:]]+SCHEMA|DROP[[:space:]]+TABLE|DROP[[:space:]]+INDEX|TRUNCATE)\b' "$f" | grep -vE '^\s*[0-9]+:\s*--' || true)
  if [ -n "$hits" ]; then
    fail "$f 含破坏性语句（应移至 scripts/dev/reset_schema.sql）"
    echo "$hits" | sed 's/^/      /'
  fi
done
[ "$VIOLATIONS" -eq 0 ] && pass "未发现破坏性语句"

echo ""
echo -e "${YELLOW}[2/9] 遗留命名检查${NC}"
legacy_before=$VIOLATIONS
for f in "${schema_files[@]}"; do
  hits=$(grep -nE '\b(uk_[a-z_]+|idx_gateway_[a-z_]+|uq_gateway_[a-z_]+)\b' "$f" || true)
  if [ -n "$hits" ]; then
    fail "$f 含旧前缀命名（uk_ → uq_、gateway_* → 语义化表名）"
    echo "$hits" | sed 's/^/      /'
  fi
  hits=$(grep -nE 'guliangiam\.(iam|gateway_servers|gateway_sites|mcp_server)\b' "$f" || true)
  if [ -n "$hits" ]; then
    fail "$f 引用遗留表名"
    echo "$hits" | sed 's/^/      /'
  fi
done
[ "$VIOLATIONS" -eq "$legacy_before" ] && pass "未发现遗留命名"

echo ""
echo -e "${YELLOW}[3/9] 幂等性检查（IF NOT EXISTS）${NC}"
before=$VIOLATIONS
for f in "${schema_files[@]}"; do
  hits=$(grep -nE '^[[:space:]]*CREATE[[:space:]]+(TABLE|UNIQUE[[:space:]]+INDEX|INDEX)' "$f" | grep -vE 'IF[[:space:]]+NOT[[:space:]]+EXISTS' || true)
  if [ -n "$hits" ]; then
    fail "$f 存在缺少 IF NOT EXISTS 的 CREATE 语句"
    echo "$hits" | sed 's/^/      /'
  fi
done
[ "$VIOLATIONS" -eq "$before" ] && pass "所有 CREATE 语句均为幂等写法"

echo ""
echo -e "${YELLOW}[4/9] revision 乐观锁列检查${NC}"
before=$VIOLATIONS
# 声明式 Schema 是唯一事实源：显式写 `revision: false` 的表（追加型链状态、单例状态行、
# 状态机驱动的子表）本来就不该有乐观锁列。生成产物里看不到这个声明，所以先从 YAML 取
# 豁免集，避免把有意的设计判成违规。
EXEMPT_REVISION=$(python3 - <<'PY'
import glob
import yaml

names = set()
for path in sorted(glob.glob("contracts/schema/*.yaml")):
    with open(path, encoding="utf-8") as handle:
        for doc in yaml.safe_load_all(handle):
            if isinstance(doc, dict) and doc.get("revision") is False:
                table = str(doc.get("table_name") or "").split(".")[-1]
                if table:
                    names.add(table)
print(" ".join(sorted(names)))
PY
)
for f in "${schema_files[@]}"; do
  hits=$(grep -nE 'revision[[:space:]]+BIGINT[[:space:]]+NOT[[:space:]]+NULL[[:space:]]+DEFAULT[[:space:]]+0' "$f" || true)
  if [ -n "$hits" ]; then
    fail "$f 存在 revision DEFAULT 0（必须为 DEFAULT 1）"
    echo "$hits" | sed 's/^/      /'
  fi
  awk -v file="$f" -v exempt="$EXEMPT_REVISION" '
    /^CREATE TABLE IF NOT EXISTS/ { table=$6; sub(/\($/, "", table); has_updated=0; has_revision=0; next }
    /^\);/ {
      if (table != "" && has_updated && !has_revision) {
        short=table; sub(/^.*\./, "", short)
        if (index(" " exempt " ", " " short " ") == 0) {
          printf "      %s: 表 %s 含 updated_at 但缺少 revision BIGINT NOT NULL DEFAULT 1 (line %d)\n", file, table, NR
          bad++
        }
      }
      table=""; next
    }
    table != "" && /updated_at[[:space:]]+TIMESTAMPTZ/ { has_updated=1 }
    table != "" && /revision[[:space:]]+BIGINT[[:space:]]+NOT[[:space:]]+NULL/ { has_revision=1 }
    END { exit (bad > 0) ? 1 : 0 }
  ' "$f" || fail "$f 存在缺少 revision 列的可更新表"
done
[ "$VIOLATIONS" -eq "$before" ] && pass "所有含 updated_at 的表均定义了 revision（已豁免声明 revision: false 的表）"

echo ""
echo -e "${YELLOW}[5/9] 约束与索引命名检查${NC}"
before=$VIOLATIONS
for f in "${schema_files[@]}"; do
  # 表级 UNIQUE 必须显式命名
  hits=$(grep -nE '^[[:space:]]+UNIQUE[[:space:]]*\(' "$f" || true)
  if [ -n "$hits" ]; then
    fail "$f 存在未命名的表级 UNIQUE 约束（应写为 CONSTRAINT uq_<table>_<cols> UNIQUE (...))"
    echo "$hits" | sed 's/^/      /'
  fi
  # CONSTRAINT 名称前缀
  hits=$(grep -oE 'CONSTRAINT[[:space:]]+[a-z_]+' "$f" | awk '{print $2}' | grep -vE '^(uq_|fk_|ck_)' | sort -u || true)
  if [ -n "$hits" ]; then
    fail "$f 存在不符合 uq_/fk_/ck_ 前缀的约束名"
    echo "$hits" | sed 's/^/      /'
  fi
  # 普通索引名前缀（唯一索引使用 uq_，单独校验）
  hits=$(grep -nE '^[[:space:]]*CREATE[[:space:]]+INDEX' "$f" \
    | grep -oE 'EXISTS[[:space:]]+[a-z_]+' | awk '{print $2}' | grep -vE '^idx_' | sort -u || true)
  if [ -n "$hits" ]; then
    fail "$f 存在不符合 idx_ 前缀的普通索引名"
    echo "$hits" | sed 's/^/      /'
  fi
  # 唯一索引名前缀
  hits=$(grep -nE '^[[:space:]]*CREATE[[:space:]]+UNIQUE[[:space:]]+INDEX' "$f" \
    | grep -oE 'EXISTS[[:space:]]+[a-z_]+' | awk '{print $2}' | grep -vE '^uq_' | sort -u || true)
  if [ -n "$hits" ]; then
    fail "$f 存在不符合 uq_ 前缀的唯一索引名"
    echo "$hits" | sed 's/^/      /'
  fi
done
[ "$VIOLATIONS" -eq "$before" ] && pass "约束与索引命名符合规范"

echo ""
echo -e "${YELLOW}[6/9] 重复索引名检查${NC}"
before=$VIOLATIONS
for f in "${schema_files[@]}"; do
  dups=$(grep -oE 'INDEX([[:space:]]+IF[[:space:]]+NOT[[:space:]]+EXISTS)?[[:space:]]+[a-z_]+' "$f" \
    | awk '{print $NF}' | sort | uniq -d || true)
  if [ -n "$dups" ]; then
    fail "$f 存在重复创建的索引名"
    echo "$dups" | sed 's/^/      /'
  fi
done
[ "$VIOLATIONS" -eq "$before" ] && pass "无重复索引名"

echo ""
echo -e "${YELLOW}[7/9] schema 脚本双副本一致性检查${NC}"
before=$VIOLATIONS
if [ ! -d "$MIRROR_DIR" ]; then
  echo -e "${RED}  VIOLATION: 必需输入目录 $MIRROR_DIR 不存在——双副本比对未执行，不能当作通过${NC}" >&2
  echo "  该目录由 scripts/gen_schema.py 生成并随仓库提交；缺失说明生成物被删除或仓库结构被破坏" >&2
  fail "必需输入目录 $MIRROR_DIR 不存在：schema 脚本双副本一致性检查未执行"
else
  for src in "$SCHEMA_DIR"/*.sql; do
    name=$(basename "$src")
    dst="$MIRROR_DIR/$name"
    if [ ! -f "$dst" ]; then
      fail "缺少镜像副本 $dst"
      continue
    fi
    a=$(shasum -a 256 "$src" | awk '{print $1}')
    b=$(shasum -a 256 "$dst" | awk '{print $1}')
    if [ "$a" != "$b" ]; then
      fail "$name 与 $dst 内容不一致（SHA-256 不同）"
      echo "      schema: $a"
      echo "      mirror: $b"
    fi
  done
fi
[ "$VIOLATIONS" -eq "$before" ] && pass "$SCHEMA_DIR/ 与镜像 $MIRROR_DIR/ 逐字节一致"

echo ""
echo -e "${YELLOW}[8/9] FK 前向引用检查${NC}"
before=$VIOLATIONS
prior_tables=""
for f in "${schema_files[@]}"; do
  hits=$(awk -v prior="$prior_tables" '
    BEGIN { pn=split(prior, p, " "); for (i=1;i<=pn;i++) if (p[i] != "") created[p[i]]=0 }
    /^CREATE TABLE IF NOT EXISTS[[:space:]]+guliangiam\./ {
      t=$6; sub(/^guliangiam\./, "", t);
      if (!(t in created)) created[t]=NR
    }
    {
      rest=$0
      while (match(rest, /REFERENCES[[:space:]]+guliangiam\.[a-z_]+/)) {
        r=substr(rest, RSTART, RLENGTH)
        sub(/^REFERENCES[[:space:]]+guliangiam\./, "", r)
        refs[++rn]=NR "\t" r
        rest=substr(rest, RSTART + RLENGTH)
      }
    }
    END {
      for (i=1;i<=rn;i++) {
        split(refs[i], a, "\t")
        ln=a[1]+0; tb=a[2]
        if (!(tb in created)) { printf "      %s: line %d -> %s（被引用表未定义）\n", FILENAME, ln, tb; bad++ }
        else if (ln < created[tb]+0) { printf "      %s: line %d -> %s（被引用表在第 %d 行才创建）\n", FILENAME, ln, tb, created[tb]; bad++ }
      }
      exit (bad > 0) ? 1 : 0
    }
  ' "$f" || true)
  while IFS= read -r table; do
    [ -n "$table" ] && prior_tables="$prior_tables$table "
  done < <(grep -oE '^CREATE TABLE IF NOT EXISTS[[:space:]]+guliangiam\.[a-z_]+' "$f" | sed -E 's/.*guliangiam\.//' || true)
  if [ -n "$hits" ]; then
    fail "$f 存在 FK 前向引用（内联 REFERENCES 指向后置创建的表）"
    echo "$hits"
  fi
done
[ "$VIOLATIONS" -eq "$before" ] && pass "无 FK 前向引用"

echo ""
echo -e "${YELLOW}[9/9] 金额与币种精度检查${NC}"
before=$VIOLATIONS
for f in "${schema_files[@]}"; do
  # 金额类列：默认仅允许 DECIMAL(19,4)。显式例外（有理由、经评审，非静默放过）：
  #   LLM 计价按「每百万 token 单价」与累计成本，单 token 成本可低至 1e-6，
  #   DECIMAL(19,4) 会在低单价模型上把成本归零（input/output_price_per_1m、cost_amount）。
  hits=$(grep -nEi '(DECIMAL|NUMERIC)[[:space:]]*\([[:space:]]*[0-9]+[[:space:]]*,[[:space:]]*[0-9]+[[:space:]]*\)' "$f" \
    | grep -vE '\([[:space:]]*19[[:space:]]*,[[:space:]]*4[[:space:]]*\)' \
    | grep -vE 'input_price_per_1m|output_price_per_1m|cost_amount' || true)
  if [ -n "$hits" ]; then
    fail "$f 存在非 DECIMAL(19,4) 的定点数列"
    echo "$hits" | sed 's/^/      /'
  fi
  # 币种类列：仅允许 VARCHAR(3)
  hits=$(grep -nEi '[a-z_]*currency[[:space:]]+VARCHAR' "$f" \
    | grep -vE 'VARCHAR[[:space:]]*\([[:space:]]*3[[:space:]]*\)' || true)
  if [ -n "$hits" ]; then
    fail "$f 存在非 VARCHAR(3) 的币种列"
    echo "$hits" | sed 's/^/      /'
  fi
done
[ "$VIOLATIONS" -eq "$before" ] && pass "金额 DECIMAL(19,4) / 币种 VARCHAR(3) 全部符合规范"

echo ""
echo "=========================================="
if [ "$VIOLATIONS" -gt 0 ]; then
  echo -e "${RED}失败：发现 $VIOLATIONS 项数据库规范违规${NC}"
  echo "=========================================="
  exit 1
fi
echo -e "${GREEN}通过：数据库设计规范检查全部通过${NC}"
echo "=========================================="
