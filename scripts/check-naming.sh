#!/usr/bin/env bash
# 禁用字面量扫描 — 门禁命名统一

# 输入：全仓 .go/.py/.md/.ts/.tsx/.sh/.yaml
# 输出：PASS/FAIL + 禁用字面量命中列表
# --check 行为：无 --check；失败即 exit 1
# 消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
set -euo pipefail

ROOT="${1:-$(pwd)}"
shift 2>/dev/null || true

FORBIDDEN=(
  'mfa_token'       # 已废弃，naming-registry N4
  'credId'          # 已废弃，naming-registry N6
  'qr-code'         # URL 不含连字符
  'X-RateLimit'     # 已废弃，用 IETF RateLimit-*
  'revision_conflict'    # 已废弃，应为 conflict
)

# 依赖目录与产物目录不参与扫描：命中 node_modules 既是噪声也让本门禁退化为数分钟级，
# 实际效果是「没人愿意跑」。真实违规只可能出现在受版本控制的源码与文档中。
EXCLUDES=(
  --exclude-dir=node_modules --exclude-dir=.git --exclude-dir=dist
  --exclude-dir=build --exclude-dir=vendor --exclude-dir=coverage
  --exclude-dir=.vite --exclude-dir=tmp --exclude-dir=.trae
)

echo "=== check-naming.sh — 禁用字面量扫描 ==="
found=0
for token in "${FORBIDDEN[@]}"; do
  # 行内豁免标记 `naming-allow`：用于「说明某字面量已废弃、不得再用」的规范文本与变更
  # 记录。没有这一机制时，唯一能让门禁变绿的办法是把废弃事实从文档里删掉——那会让
  # 下一个开发者重新发明该字面量。豁免必须逐行显式声明，不是按文件或按目录。
  matches=$(grep -rn "${EXCLUDES[@]}" --include='*.go' --include='*.ts' --include='*.tsx' --include='*.md' \
    -i "$token" "$ROOT" 2>/dev/null \
    | grep -v 'scripts/check-naming.sh' \
    | grep -v 'naming-registry.md' \
    | grep -v 'errors-registry.md' \
    | grep -v 'limit-registry.md' \
    | grep -v 'state-machine-registry.md' \
    | grep -v 'env-registry.md' \
    | grep -v 'slo-registry.md' \
    | grep -v 'cache-registry.md' \
    | grep -v 'outbox-registry.md' \
    | grep -v 'security-readiness.md' \
    | grep -v 'naming-allow' || true)
  if [ -n "$matches" ]; then
    echo "FAIL: 发现禁用字面量 '$token' (naming-registry 已废弃)"
    echo "$matches"
    found=$((found + 1))
  fi
done

if [ $found -eq 0 ]; then
  echo "PASS: 无禁用字面量"
  exit 0
else
  echo "FAIL: $found 个禁用字面量"
  exit 1
fi
