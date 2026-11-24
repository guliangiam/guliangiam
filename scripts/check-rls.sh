#!/usr/bin/env bash
# check-rls.sh — Row Level Security 强制隔离 CI 门禁
#
# 依据：ADR-016、架构 7.4、安全就绪度 S12
# 用法：bash scripts/check-rls.sh [repo_root]
#
# 检查项：
#   1. contracts/schema/00_infra.yaml 中 rls.force 必须为 true
#   2. 所有含 organization_id 列的租户表必须声明 rls: true（或 rls_predicate）
#   3. 生成的 SQL 中租户表必须包含 ENABLE + FORCE ROW LEVEL SECURITY
#   4. 应用角色 guliangiam_app 已创建且被授予必要权限（非 owner）

# 输入：contracts/schema/00_infra.yaml rls.force、contracts/schema/*.yaml 租户表 rls、db/postgresql/*.sql
# 输出：PASS/FAIL + 未启用 RLS 的租户表
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
SCHEMA_DIR="contracts/schema"
SQL_DIR="db/postgresql"

fail() { echo -e "${RED}  VIOLATION: $*${NC}"; VIOLATIONS=$((VIOLATIONS + 1)); }
pass() { echo -e "${GREEN}  OK: $*${NC}"; }

echo "=========================================="
echo "  GuliangIAM RLS 强制隔离门禁"
echo "=========================================="

# ── [1/4] rls.force 全局开关 ─────────────────────────────────────
echo ""
echo -e "${YELLOW}[1/4] 全局 rls.force 开关检查${NC}"
# 用 YAML 解析器而非行正则：rls 既可能是标量 true，也可能是映射 {force: true}，
# 行正则只认一种写法，另一种写法会被误报为「未声明 RLS」。
force_val=$(python3 - "$SCHEMA_DIR/00_infra.yaml" <<'PY'
import sys
import yaml

try:
    with open(sys.argv[1], encoding="utf-8") as handle:
        infra = yaml.safe_load(handle) or {}
    print("true" if (infra.get("rls") or {}).get("force") is True else "false")
except Exception:
    print("parse_error")
PY
)
if [ "$force_val" != "true" ]; then
  fail "00_infra.yaml 中 rls.force 不为 true（当前值: ${force_val:-<未设置>}）"
else
  pass "rls.force = true"
fi

# ── [2/4] 租户表 rls 声明一致性 ──────────────────────────────────
echo ""
echo -e "${YELLOW}[2/4] 租户表 rls 声明检查${NC}"
# 逐文档 YAML 解析：每个 --- 分隔的文档检查「含 organization_id 的租户表必须启用 RLS」。
# 判定按 YAML 语义（rls 真值 = 标量 true 或任意非空映射）而非行文本形态，
# 因此 rls: true 与 rls: {force: true} 两种合法写法都能被正确识别。
before=$VIOLATIONS
if ! rls_missing=$(python3 - "$SCHEMA_DIR" <<'PY'
import glob
import sys

import yaml

problems = []
for path in sorted(glob.glob(sys.argv[1] + "/*.yaml")):
    with open(path, encoding="utf-8") as handle:
        for doc in yaml.safe_load_all(handle):
            if not isinstance(doc, dict):
                continue
            table = str(doc.get("table_name") or "")
            if not table:
                continue
            columns = {
                column.get("name")
                for column in (doc.get("columns") or [])
                if isinstance(column, dict) and column.get("name")
            }
            if "organization_id" not in columns:
                continue
            if doc.get("rls") or doc.get("rls_predicate") or doc.get("rls_force"):
                continue
            problems.append(f"      {path}: 表 {table} 含 organization_id 但未声明 rls")

for problem in problems:
    print(problem)
sys.exit(1 if problems else 0)
PY
); then
  fail "存在缺少 rls 声明的租户表"
fi
[ "$VIOLATIONS" -eq "$before" ] && pass "所有租户表均声明 rls"

# ── [3/4] 生成 SQL 中 FORCE RLS ──────────────────────────────────
echo ""
echo -e "${YELLOW}[3/4] 生成 SQL FORCE RLS 检查${NC}"
before=$VIOLATIONS
sql_files=("$SQL_DIR"/*.sql)
if [ ${#sql_files[@]} -gt 0 ]; then
  force_count=$(grep -rh "FORCE ROW LEVEL SECURITY" "$SQL_DIR"/*.sql 2>/dev/null | wc -l | tr -d ' ')
  enable_count=$(grep -rh "ENABLE ROW LEVEL SECURITY" "$SQL_DIR"/*.sql 2>/dev/null | wc -l | tr -d ' ')
  if [ "$force_count" -eq 0 ]; then
    fail "生成的 SQL 中未找到 FORCE ROW LEVEL SECURITY 语句"
  else
    pass "生成 SQL 含 $force_count 条 FORCE RLS（$enable_count 条 ENABLE RLS）"
  fi
  # 反向：检查是否有租户表定义了 RLS 策略但缺少 FORCE
  no_force=$(grep -L "FORCE ROW LEVEL SECURITY" "$SQL_DIR"/*.sql 2>/dev/null | xargs grep -l "ENABLE ROW LEVEL SECURITY" 2>/dev/null || true)
  if [ -n "$no_force" ]; then
    fail "以下 SQL 文件含 ENABLE RLS 但缺少 FORCE RLS：$(echo "$no_force" | tr '\n' ' ')"
  fi
else
  fail "未找到生成的 SQL 文件（请先运行 gen_schema.py）"
fi
[ "$VIOLATIONS" -eq "$before" ] && pass "所有 RLS 表均启用 FORCE"

# ── [4/4] 应用角色 guliangiam_app ────────────────────────────────
echo ""
echo -e "${YELLOW}[4/4] 应用角色 guliangiam_app 检查${NC}"
before=$VIOLATIONS
role_sql=$(grep -rh "guliangiam_app" "$SQL_DIR"/*.sql 2>/dev/null | head -5 || true)
if [ -z "$role_sql" ]; then
  fail "生成的 SQL 中未找到 guliangiam_app 角色定义/授权"
else
  pass "guliangiam_app 角色存在于生成 SQL 中"
fi
# 检查 SQL 中是否有对 guliangiam_app 的 GRANT（证明非 owner 角色被授权使用 schema）
grant_count=$(grep -rh "GRANT.*guliangiam_app" "$SQL_DIR"/*.sql 2>/dev/null | wc -l | tr -d ' ')
if [ "$grant_count" -eq 0 ]; then
  fail "SQL 中未找到对 guliangiam_app 的 GRANT 语句（应用角色无法访问 schema）"
else
  pass "SQL 含 $grant_count 条对 guliangiam_app 的 GRANT 语句"
fi

echo ""
echo "=========================================="
if [ "$VIOLATIONS" -gt 0 ]; then
  echo -e "${RED}失败：发现 $VIOLATIONS 项 RLS 违规${NC}"
  echo "=========================================="
  exit 1
fi
echo -e "${GREEN}通过：RLS 强制隔离检查全部通过${NC}"
echo "=========================================="
