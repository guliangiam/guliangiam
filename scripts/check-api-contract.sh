#!/usr/bin/env bash
# API 契约门禁

# 输入：openapi/openapi.yaml、internal/transport/http/route/、docs/generated/surfaces-registry.md
# 输出：PASS/FAIL + 未闭合项
# --check 行为：无 --check；失败即 exit 1
# 消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
set -euo pipefail
ROOT="${1:-$(pwd)}"

echo "=== check-api-contract.sh ==="

# 1. 注册表存在
REG="$ROOT/docs/generated/surfaces-registry.md"
if [ ! -f "$REG" ]; then
    echo "FAIL: surfaces-registry.md 不存在"
    exit 1
fi

# 2. SurfaceFor 分类器存在
if ! grep -rq "SurfaceFor" "$ROOT/internal/transport/http/route/" 2>/dev/null; then
    echo "FAIL: SurfaceFor 分类器未在 route 包中定义"
    exit 1
fi
echo "PASS: SurfaceFor 分类器存在"

# 3. OpenAPI 与路由声明一致
# 生成器/产物缺失必须硬失败。此前条件写成「文件存在才校验」，生成器迁到 scripts/ 后条件
# 恒假，本步骤静默跳过，门禁整体仍报 PASS——「校验没跑」被当成「校验通过」。
if [ ! -f "$ROOT/scripts/gen_openapi.py" ]; then
    echo "FAIL: 缺少 OpenAPI 生成器 scripts/gen_openapi.py"
    exit 1
fi
if [ ! -f "$ROOT/openapi/openapi.yaml" ]; then
    echo "FAIL: 缺少 OpenAPI 产物 openapi/openapi.yaml"
    exit 1
fi
( cd "$ROOT" && python3 scripts/gen_openapi.py --check ) 2>&1 || {
    echo "FAIL: OpenAPI 生成结果与已提交产物不一致"
    exit 1
}
echo "PASS: OpenAPI 契约一致"

echo "PASS: 全部通过"
exit 0
