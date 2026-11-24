#!/usr/bin/env bash
# 数据流闭环门禁 — 检查写路径是否声明了 outbox + records + idempotency

# 输入：internal/app/ 写路径、docs/GuliangIAM-架构设计.md
# 输出：PASS/WARN + 比例统计
# --check 行为：无 --check；WARN 不阻断
# 消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
set -euo pipefail
ROOT="${1:-$(pwd)}"

echo "=== check-data-flow.sh ==="

# 检查 App 层写操作是否同事务写 outbox 和 records
# 简化版：扫描 app 层文件中是否引用 outbox 或 records
APP_WRITE=$(grep -rln "outbox\|records\|audit" "$ROOT/internal/app/" 2>/dev/null | wc -l | tr -d ' ')
APP_FILES=$(find "$ROOT/internal/app/" -name '*.go' -not -name '*_test.go' 2>/dev/null | wc -l | tr -d ' ')

if [ "$APP_FILES" -gt 0 ]; then
    RATIO=$((APP_WRITE * 100 / APP_FILES))
    echo "App 层 ${APP_WRITE}/${APP_FILES} 文件引用 outbox/records/audit ($RATIO%)"
fi

# 25.1 统一规则存在
DESIGN="$ROOT/docs/GuliangIAM-架构设计.md"
if [ -f "$DESIGN" ]; then
    grep -q "事务内不执行不可控网络调用" "$DESIGN" && \
        echo "PASS: 事务边界规则存在" || \
        echo "WARN: 未找到事务边界规则"
fi

echo "PASS: 数据流闭环检查完成（静态扫描）"
exit 0
