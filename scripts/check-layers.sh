#!/usr/bin/env bash
# check-layers.sh - 分层架构违规检查脚本
# 规则：app 层不得直接 import infra 层；transport 层不得直接 import domain 层（应通过 app 层）

# 输入：internal/**/*.go
# 输出：PASS/WARN + 违规 import 列表（informational，不阻断 verify-docs）
# --check 行为：无 --check；informational 不阻断
# 消费方：verify-docs.sh(RUN_CODE_CHECKS=1, 仅 informational)、CI
set -euo pipefail

ROOT="${1:-.}"
VIOLATIONS=0

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=========================================="
echo "  GuliangIAM 分层架构违规检查"
echo "=========================================="
echo ""

# Rule 1: app/ 不得直接 import infra/ (except app/syncers which has known legacy)
echo -e "${YELLOW}[Rule 1] app/ → infra/ 直接依赖检查${NC}"
while IFS= read -r file; do
    rel=$(echo "$file" | sed "s|^$ROOT/||")
    # Check for infra imports
    infra_imports=$(grep -E '"github\.com/guliangiam/guliangiam/internal/infra/' "$file" 2>/dev/null || true)
    if [ -n "$infra_imports" ]; then
        echo -e "${RED}  VIOLATION: $rel${NC}"
        echo "$infra_imports" | sed 's/^/    /'
        VIOLATIONS=$((VIOLATIONS + 1))
    fi
done < <(find "$ROOT/internal/app" -name '*.go' -not -name '*_test.go' -type f 2>/dev/null)

echo ""

# Rule 2: transport/http/ 不得直接 import domain/ 或 infra/（应通过 app/ 层）
echo -e "${YELLOW}[Rule 2] transport/ → domain|infra 直接依赖检查${NC}"
while IFS= read -r file; do
    rel=$(echo "$file" | sed "s|^$ROOT/||")
    bad_imports=$(grep -E '"github\.com/guliangiam/guliangiam/internal/(domain|infra)/' "$file" 2>/dev/null || true)
    if [ -n "$bad_imports" ]; then
        echo -e "${RED}  VIOLATION: $rel${NC}"
        echo "$bad_imports" | sed 's/^/    /'
        VIOLATIONS=$((VIOLATIONS + 1))
    fi
done < <(find "$ROOT/internal/transport" -name '*.go' -not -name '*_test.go' -type f 2>/dev/null)

echo ""

# Rule 3: domain/ 不得 import infra/ or app/
echo -e "${YELLOW}[Rule 3] domain/ → infra|app 反向依赖检查${NC}"
while IFS= read -r file; do
    rel=$(echo "$file" | sed "s|^$ROOT/||")
    bad_imports=$(grep -E '"github\.com/guliangiam/guliangiam/internal/(infra|app)/' "$file" 2>/dev/null || true)
    if [ -n "$bad_imports" ]; then
        echo -e "${RED}  VIOLATION: $rel${NC}"
        echo "$bad_imports" | sed 's/^/    /'
        VIOLATIONS=$((VIOLATIONS + 1))
    fi
done < <(find "$ROOT/internal/domain" -name '*.go' -not -name '*_test.go' -type f 2>/dev/null)

echo ""

# Rule 4: Check for fmt.Errorf without %w wrapping an existing error.
# Only flags cases where an error variable is being wrapped but %w is missing.
# New errors created from non-error values (strings, ints, bools) legitimately
# don't need %w — these are NOT flagged. sqlguard.go creates fresh validation
# errors (not wrapping downstream errors), so it's excluded explicitly.
echo -e "${YELLOW}[Rule 4] fmt.Errorf 包装已有错误时需 %w 检查${NC}"
FMT_COUNT=0
while IFS= read -r line; do
    if [ -n "$line" ]; then
        FMT_COUNT=$((FMT_COUNT + 1))
        if [ "$FMT_COUNT" -le 10 ]; then
            echo -e "${RED}  $line${NC}"
        fi
    fi
done < <(find "$ROOT/internal" -name '*.go' -not -name '*_test.go' -not -path '*/sqlguard/*' -type f -print0 2>/dev/null \
    | xargs -0 grep -n 'fmt\.Errorf' 2>/dev/null \
    | grep -v '%w' \
    | grep -E '\b(err|Err|[A-Z][a-zA-Z]*Err|[a-z_]+err)\b' || true)
if [ "$FMT_COUNT" -gt 10 ]; then
    echo -e "${RED}  ... and $((FMT_COUNT - 10)) more${NC}"
fi
if [ "$FMT_COUNT" -gt 0 ]; then
    echo -e "${RED}  Total: $FMT_COUNT violations${NC}"
    VIOLATIONS=$((VIOLATIONS + FMT_COUNT))
fi

echo ""

# Rule 5: Check for stdlib log.* calls (should use slog)
# Scope: production code only (see Rule 4 rationale).
echo -e "${YELLOW}[Rule 5] stdlib log.* 调用检查 (应使用 slog，排除 _test.go)${NC}"
LOG_COUNT=0
while IFS= read -r line; do
    if [ -n "$line" ]; then
        LOG_COUNT=$((LOG_COUNT + 1))
        echo -e "${RED}  $line${NC}"
    fi
done < <(find "$ROOT/internal" -name '*.go' -not -name '*_test.go' -type f -print0 2>/dev/null \
    | xargs -0 grep -n 'log\.\(Println\|Printf\|Fatal\|Fatalf\|Print\)\b' 2>/dev/null || true)
if [ "$LOG_COUNT" -gt 0 ]; then
    echo -e "${RED}  Total: $LOG_COUNT violations${NC}"
    VIOLATIONS=$((VIOLATIONS + LOG_COUNT))
fi

echo ""

# Rule 6: Check for TODO(phase3) markers
# Scope: production code only (see Rule 4 rationale).
echo -e "${YELLOW}[Rule 6] 待解耦 TODO(phase3) 标记 (排除 _test.go)${NC}"
TODO_COUNT=0
while IFS= read -r line; do
    if [ -n "$line" ]; then
        TODO_COUNT=$((TODO_COUNT + 1))
        echo -e "${YELLOW}  $line${NC}"
    fi
done < <(find "$ROOT/internal" -name '*.go' -not -name '*_test.go' -type f -print0 2>/dev/null \
    | xargs -0 grep -n 'TODO(phase3)' 2>/dev/null || true)
echo "  Found: $TODO_COUNT markers"

echo ""

# ---------------------------------------------------------------------------
# Rule 7: platform/ 不得 import app/infra/transport
# 注：platform→domain 是 Clean Architecture 合法方向（domain 层端口接口由上层实现），
#     因此 (app|infra|transport) 三选一才是真正的越界。
#     layers.md §1.1 Platform "May Import domain for port interfaces" 已明示。
# 当前阶段：informational（不计 exit code）。
# ---------------------------------------------------------------------------
echo -e "${YELLOW}[Rule 7] platform/ → app|infra|transport 越层依赖检查 (informational)${NC}"
PLATFORM_VIOLATIONS=0
while IFS= read -r file; do
    rel=$(echo "$file" | sed "s|^$ROOT/||")
    bad=$(grep -E '"github\.com/guliangiam/guliangiam/internal/(app|infra|transport)/' "$file" 2>/dev/null || true)
    if [ -n "$bad" ]; then
        echo -e "${RED}  VIOLATION: $rel${NC}"
        echo "$bad" | sed 's/^/    /'
        PLATFORM_VIOLATIONS=$((PLATFORM_VIOLATIONS + 1))
    fi
done < <(find "$ROOT/internal/platform" -name '*.go' -not -name '*_test.go' -type f 2>/dev/null)
echo "  Counted violations: $PLATFORM_VIOLATIONS (informational)"

echo ""

# ---------------------------------------------------------------------------
# Rule 8: infra/ 不得反向 import app/（应通过 domain 端口接口）
# 已知合理例外：infra/*/uow/uow.go 实现 app 层 UnitOfWork 接口
# —— Clean Architecture infra-implements-app-interface 模式，但接口归属
#    应下沉到 domain 层，目前临时接受。dashboard 是真 violation。
# 当前阶段：informational（不计 exit code），待 P4 dashboard 归属下沉完成后切换 blocking。
# ---------------------------------------------------------------------------
echo -e "${YELLOW}[Rule 8] infra/ → app/ 反向依赖检查 (uow 合法例外已排除, informational)${NC}"
INFRA_APP_VIOLATIONS=0
while IFS= read -r file; do
    rel=$(echo "$file" | sed "s|^$ROOT/||")
    # 排除 uow（interface implementation 合法例外）
    case "$rel" in
        */uow/uow.go) continue ;;
    esac
    bad=$(grep -E '"github\.com/guliangiam/guliangiam/internal/app/' "$file" 2>/dev/null || true)
    if [ -n "$bad" ]; then
        echo -e "${RED}  VIOLATION: $rel${NC}"
        echo "$bad" | sed 's/^/    /'
        INFRA_APP_VIOLATIONS=$((INFRA_APP_VIOLATIONS + 1))
        # 注意：当前不计入 VIOLATIONS 累加，等 P4 收敛完成后去掉注释
        # VIOLATIONS=$((VIOLATIONS + 1))
    fi
done < <(find "$ROOT/internal/infra" -name '*.go' -not -name '*_test.go' -type f 2>/dev/null)
if [ "$INFRA_APP_VIOLATIONS" -eq 0 ]; then
    echo -e "${GREEN}  OK (dashboard DTO 应下沉 domain/dashboard，待后续轮次)${NC}"
fi
echo "  Counted violations: $INFRA_APP_VIOLATIONS (informational)"

# Rule 9: 禁止 superAdminCtx 回潮（GAP-032，baseline 0）：跨租户唯一合法路径是
# ScopeAuthorizerMiddleware → 仓储 TenantFilterSystem；任何出现即 blocking。
echo -e "${YELLOW}[Rule 9] superAdminCtx 回潮检查 (blocking, baseline 0)${NC}"
SUPERADMIN_HITS=$(grep -rn 'superAdminCtx' "$ROOT/internal" --include='*.go' 2>/dev/null || true)
SUPERADMIN_COUNT=0
if [ -n "$SUPERADMIN_HITS" ]; then
    echo "$SUPERADMIN_HITS" | sed 's/^/  /'
    SUPERADMIN_COUNT=$(echo "$SUPERADMIN_HITS" | wc -l | tr -d ' ')
    VIOLATIONS=$((VIOLATIONS + SUPERADMIN_COUNT))
fi
echo "  Counted violations: $SUPERADMIN_COUNT"

# Rule 10: SQL 放置（AGENTS.md 不变量 2，BE-12）：app/transport/domain 非测试代码
# 不得含 SQL 语句字符串字面量（blocking, baseline 0）。SQL 只在 internal/infra/**；
# platform 维护作业（db/outbox/crypto）豁免边界见 docs/architecture/layers.md §1。
# 只匹配字符串字面量（双引号/反引号）开头的语句起始关键词；注释中的 SQL 不命中。
echo -e "${YELLOW}[Rule 10] app/transport/domain SQL 语句字面量检查 (blocking, baseline 0)${NC}"
SQL_LITERAL_HITS=$(grep -RInE '("|`)[[:space:]]*(SELECT[[:space:]]|INSERT[[:space:]]+INTO|UPDATE[[:space:]]+[a-zA-Z_".]|DELETE[[:space:]]+FROM|CREATE[[:space:]]+(TABLE|INDEX|SCHEMA|UNIQUE)|ALTER[[:space:]]+TABLE|DROP[[:space:]]+(TABLE|INDEX|SCHEMA)|WITH[[:space:]]+[a-zA-Z_]+[[:space:]]+AS[[:space:]]*\()' \
    "$ROOT/internal/app" "$ROOT/internal/transport" "$ROOT/internal/domain" \
    --include='*.go' --exclude='*_test.go' 2>/dev/null || true)
SQL_LITERAL_COUNT=0
if [ -n "$SQL_LITERAL_HITS" ]; then
    echo "$SQL_LITERAL_HITS" | sed 's/^/  /'
    SQL_LITERAL_COUNT=$(echo "$SQL_LITERAL_HITS" | wc -l | tr -d ' ')
    VIOLATIONS=$((VIOLATIONS + SQL_LITERAL_COUNT))
fi
echo "  Counted violations: $SQL_LITERAL_COUNT"

echo ""
echo "=========================================="
if [ "$VIOLATIONS" -eq 0 ]; then
    echo -e "${GREEN}  ALL CHECKS PASSED${NC}"
else
    echo -e "${RED}  TOTAL VIOLATIONS: $VIOLATIONS${NC}"
fi
echo "=========================================="

exit $VIOLATIONS
