#!/usr/bin/env bash
# ────────────────────────────────────────────────────────────────────────────────
# check-schema-consistency.sh
#   唯一事实源门禁：contracts/schema/*.yaml → DDL 生成物必须零差异。
#   ADR-022 出口条件：YAML 声明式 schema 与运行时 DDL 是逐字节同源。
#
#   检查项：
#     1. python3 scripts/gen_schema.py --dialect postgresql --check
#        内部会读取所有 contracts/schema/*.yaml，重新生成全部产物并与工作区
#        逐字节 diff：db/generated/postgresql/current.up.sql（当前态渲染）、
#        db/postgresql/V1__*.sql（冻结基线），以及 V2+ 增量迁移与回滚。
#        任何一个文件有差异 = 退出码 1 + 打印哪个文件不同。
#
#   用法：
#     bash scripts/check-schema-consistency.sh
#
#   失败时修复流程：
#     python3 scripts/gen_schema.py --dialect postgresql --all
#     bash scripts/check-schema-consistency.sh   # 再跑一次确认
#     git add db/generated/ db/postgresql/ db/rollback/
# ────────────────────────────────────────────────────────────────────────────────

# 输入：contracts/schema/*.yaml、db/generated/postgresql/current.up.sql、db/postgresql/V*.sql、db/rollback/
# 输出：PASS/FAIL + 差异文件
# --check 行为：无 --check；内部调用 gen_schema.py --check
# 消费方：独立脚本、CI
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

echo "=========================================="
echo "  Schema 一致性门禁 (ADR-022)"
echo "=========================================="
echo ""

GEN_SCHEMA="${SCRIPT_DIR}/gen_schema.py"

if [ ! -f "${GEN_SCHEMA}" ]; then
    echo -e "${RED}FAIL: ${GEN_SCHEMA} 不存在${NC}"
    exit 1
fi

if ! command -v python3 &>/dev/null; then
    echo -e "${RED}FAIL: python3 未安装${NC}"
    exit 1
fi

# 运行 --check，捕获输出和退出码
CHECK_OUTPUT=$(python3 "${GEN_SCHEMA}" --dialect postgresql --check 2>&1)
CHECK_RC=$?

if [ ${CHECK_RC} -eq 0 ]; then
    echo -e "${GREEN}${BOLD}PASS${NC} Schema 一致性门禁"
    echo "${CHECK_OUTPUT}"
    echo ""
    echo "→ 所有 DDL 生成物与 contracts/schema/*.yaml 零差异"
    exit 0
else
    echo -e "${RED}${BOLD}FAIL${NC} Schema 一致性门禁"
    echo "${CHECK_OUTPUT}"
    echo ""
    echo -e "${YELLOW}修复流程：${NC}"
    echo "  python3 scripts/gen_schema.py --dialect postgresql --all"
    echo "  bash scripts/check-schema-consistency.sh  # 再跑一次确认"
    exit 1
fi
