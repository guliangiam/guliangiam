#!/usr/bin/env bash
# check-target-schema.sh — 目标域 Schema 完整性 CI 门禁
#
# 依据：Step 4 目标域 Schema 落地（ReBAC / Workload Identity / Metering / IGA / Risk / Actions）
# 用法：bash scripts/check-target-schema.sh [repo_root]
#
# 检查项：
#   1. 16 张目标域表在 contracts/schema/*.yaml 中均有定义
#   2. 生成的 SQL 中均包含对应 CREATE TABLE
#   3. YAML 与生成 SQL 零漂移（gen_schema.py --check）

# 输入：contracts/schema/*.yaml、db/postgresql/*.sql
# 输出：PASS/FAIL + 缺失的目标域表
# --check 行为：无 --check；失败即 exit 1
# 消费方：独立脚本、CI
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

# Step 4 落地的 16 张目标域表（表名，不含 schema 前缀）
TARGET_TABLES=(
  "relation_namespaces"
  "relation_tuples"
  "identity_pools"
  "federation_trusts"
  "service_accounts"
  "metering_events"
  "metering_aggregates"
  "access_campaigns"
  "sod_rules"
  "access_lifecycle"
  "risk_signals"
  "risk_decisions"
  "session_governance"
  "identity_actions"
  "action_versions"
  "execution_logs"
)

fail() { echo -e "${RED}  VIOLATION: $*${NC}"; VIOLATIONS=$((VIOLATIONS + 1)); }
pass() { echo -e "${GREEN}  OK: $*${NC}"; }

echo "=========================================="
echo "  GuliangIAM 目标域 Schema 门禁"
echo "=========================================="

# ── [1/3] YAML 中目标域表定义 ───────────────────────────────────
echo ""
echo -e "${YELLOW}[1/3] YAML 目标域表定义检查${NC}"
before=$VIOLATIONS
for t in "${TARGET_TABLES[@]}"; do
  if ! grep -rq "guliangiam\.$t\b" "$SCHEMA_DIR"/*.yaml; then
    fail "YAML 中未找到目标域表: guliangiam.$t"
  fi
done
[ "$VIOLATIONS" -eq "$before" ] && pass "全部 ${#TARGET_TABLES[@]} 张目标域表在 YAML 中有定义"

# ── [2/3] 生成 SQL 中目标域表 ────────────────────────────────────
echo ""
echo -e "${YELLOW}[2/3] 生成 SQL 目标域表检查${NC}"
before=$VIOLATIONS
for t in "${TARGET_TABLES[@]}"; do
  if ! grep -rq "CREATE TABLE IF NOT EXISTS guliangiam\.$t\b" "$SQL_DIR"/*.sql; then
    fail "生成 SQL 中未找到目标域表: guliangiam.$t"
  fi
done
[ "$VIOLATIONS" -eq "$before" ] && pass "全部 ${#TARGET_TABLES[@]} 张目标域表在生成 SQL 中有定义"

# ── [3/3] YAML 与生成 SQL 零漂移 ─────────────────────────────────
echo ""
echo -e "${YELLOW}[3/3] YAML 与生成 SQL 零漂移检查${NC}"
before=$VIOLATIONS
if command -v python3 >/dev/null 2>&1; then
  if ! python3 scripts/gen_schema.py --check >/dev/null 2>&1; then
    fail "gen_schema.py --check 检测到 YAML 与生成 SQL 漂移（请重新运行 gen_schema.py）"
  else
    pass "YAML 与生成 SQL 零漂移（gen_schema.py --check 通过）"
  fi
else
  echo -e "${RED}  未执行：python3 不可用——[3/3] YAML 与生成 SQL 零漂移检查没有运行，这不是通过${NC}" >&2
  echo "  修复：安装 python3 后重跑本门禁（macOS: brew install python3；Debian/Ubuntu: apt-get install -y python3）" >&2
  fail "python3 不可用：[3/3] YAML 与生成 SQL 零漂移检查未执行，不能当作通过"
fi

echo ""
echo "=========================================="
if [ "$VIOLATIONS" -gt 0 ]; then
  echo -e "${RED}失败：发现 $VIOLATIONS 项目标域 Schema 违规${NC}"
  echo "=========================================="
  exit 1
fi
echo -e "${GREEN}通过：目标域 Schema 检查全部通过${NC}"
echo "=========================================="
