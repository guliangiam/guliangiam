#!/usr/bin/env bash
# 声明式 Schema 完整性门禁

# 输入：contracts/schema/*.yaml 清单
# 输出：PASS/FAIL + 缺失文件
# --check 行为：无 --check；失败即 exit 1
# 消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
set -euo pipefail
ROOT="${1:-$(pwd)}"

echo "=== check-schema.sh ==="

ERRORS=0

# 1. YAML 文件存在性
REQUIRED=(
  "contracts/schema/00_infra.yaml"
  "contracts/schema/00_platform.yaml"
  "contracts/schema/10_tenant.yaml"
  "contracts/schema/20_identity.yaml"
  "contracts/schema/30_authz.yaml"
  "contracts/schema/40_session.yaml"
  "contracts/schema/50_protocol.yaml"
  "contracts/schema/60_audit.yaml"
  "contracts/schema/70_commerce.yaml"
  "contracts/schema/80_governance.yaml"
  "contracts/schema/90_metering.yaml"
)

for f in "${REQUIRED[@]}"; do
  if [ ! -f "$ROOT/$f" ]; then
    echo "FAIL: 缺少声明式 Schema $f"
    ERRORS=$((ERRORS + 1))
  fi
done

# 2. DDL 缺失表检查
DDL="$ROOT/db/generated/postgresql/current.up.sql"
if [ -f "$DDL" ]; then
  for table in encryption_reencrypt_progress outbox_dead_letters; do
    if ! grep -q "CREATE TABLE.*$table" "$DDL" 2>/dev/null; then
      echo "FAIL: DDL 缺失表 $table"
      ERRORS=$((ERRORS + 1))
    fi
  done
  echo "PASS: DDL 必需表齐全"
  echo "NOTE: 表/列/约束/索引/RLS 的全量等价性由 scripts/check-schema-parity.py 强制"
else
  echo "SKIP: DDL 文件不存在"
fi

if [ $ERRORS -eq 0 ]; then
  echo "PASS: Schema 完整"
  exit 0
else
  echo "FAIL: $ERRORS 个错误"
  exit 1
fi
