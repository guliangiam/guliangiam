#!/usr/bin/env bash
# ============================================================================
# check-rollback-independence.sh — 回滚脚本独立性验证（CI 门禁）
#
# 依据：《GuliangIAM 架构全面审视与优化方案》Phase 1.5
#       ADR-022 结构门禁 2 / 出口条件 9
#
# 前提：目标库必须是干净的测试库。脚本会调用 reset_schema.sql + DROP SCHEMA CASCADE。
#       禁止指向生产库。
#
# 验证维度：
#   1. V1 全量回滚（db/rollback/V1__*.down.sql）
#      Apply V1 up → Run V1 down → 断言 guliangiam schema 已消失
#
#   2. 版本化增量回滚（db/rollback/V{N}__*.down.sql，N ≥ 2）
#      Apply up to V(N-1) → Apply VN up → Run VN down → 断言回到 V(N-1) 状态
#      （正向：V1 → V2 上 → V2 下 == V1；V1 → V2 → V3 上 → V3 下 == V2）
#
# 设计要点：
#   - 每个 rollback 脚本**独立运行**：脚本之间互不依赖，失败时能精确定位
#     是哪个 down.sql 有问题（reset + 重放正向 + 执行 down + 断言）。
#   - 版本化回滚必须**先 apply 到目标版本**再 down，不能假设当前状态。
#     这保证了 down.sql 不依赖任何"先被 down 过"的脏状态。
#   - 多方言：当前只验证 PostgreSQL（CI 主方言）。MySQL/GaussDB/KingbaseES
#     的 V1 全量回滚由 gen_schema.py --check 保证零差异，在此不做执行验证。
#
# 用法：
#   DATABASE_URL="postgres://user:pass@host:5432/db?sslmode=disable" \
#     ALLOW_SCHEMA_RESET=1 bash scripts/dev/check-rollback-independence.sh
#
# 退出码：0=全部通过，非零=某步失败（会打印失败脚本名和 psql 错误）
# ============================================================================

# 输入：
# 输出：
# --check 行为：
# 消费方：
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"

: "${DATABASE_URL:?请设置 DATABASE_URL，例如 postgres://postgres:postgres@localhost:5432/guliangiam?sslmode=disable}"

if [ "${ALLOW_SCHEMA_RESET:-}" != "1" ]; then
  echo "该脚本会 DROP SCHEMA guliangiam CASCADE。确认目标是非生产库后，设置 ALLOW_SCHEMA_RESET=1 重跑。" >&2
  exit 2
fi

PSQL=(psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -q)
PASS=0
FAIL=0
FAILURES=()

# ── 辅助函数 ──────────────────────────────────────────────────────────────

reset_db() {
  "${PSQL[@]}" -f scripts/dev/reset_schema.sql >/dev/null
}

q() { psql "$DATABASE_URL" -tAc "$1"; }

assert_label() {
  # assert_label <label> <expected> <actual>
  if [ "$3" != "$2" ]; then
    echo "    FAIL: $1（期望 $2，实际 $3）" >&2
    return 1
  fi
  echo "    OK: $1"
}

run_test() {
  # run_test <描述> <脚本路径> <验证函数>
  local label="$1" script="$2" verify_fn="$3"
  echo ""
  echo "--> $label"
  echo "    脚本: $script"

  reset_db

  # 捕获执行错误，不立即退出（让后续用例继续运行）
  if ! "${PSQL[@]}" -f "$script" 2>/tmp/rollback_err.$$; then
    echo "    FAIL: 执行错误" >&2
    sed 's/^/      /' /tmp/rollback_err.$$ >&2
    rm -f /tmp/rollback_err.$$
    FAIL=$((FAIL + 1))
    FAILURES+=("$label")
    return 1
  fi
  rm -f /tmp/rollback_err.$$

  if ! "$verify_fn"; then
    FAIL=$((FAIL + 1))
    FAILURES+=("$label")
    return 1
  fi

  echo "    PASS"
  PASS=$((PASS + 1))
}

# ── 验证函数 ──────────────────────────────────────────────────────────────

# V1 全量回滚后：guliangiam schema 必须消失
verify_v1_full_rollback() {
  local schemas
  schemas=$(q "SELECT count(*) FROM pg_namespace WHERE nspname='guliangiam'")
  assert_label "guliangiam schema 已消失" "0" "$schemas"
}

# 版本化回滚：先 apply 正脚本到目标版本，再执行 down，然后断言回到 V(N-1)
# 用法：prepare_versioned_rollback <target_version>
prepare_versioned_rollback() {
  local target="$1"
  reset_db
  # Apply V1 init DDL
  "${PSQL[@]}" -f db/postgresql/V1__guliangiam_schema_init.sql >/dev/null
  # Apply V2 .. V(target-1) 正脚本
  local v
  for ((v = 2; v < target; v++)); do
    local up="db/postgresql/V${v}__"*.sql
    # glob 可能匹配多个文件（虽然当前每版本一个）
    for f in $up; do
      [ -f "$f" ] && "${PSQL[@]}" -f "$f" >/dev/null
    done
  done
}

# 版本化回滚验证：执行 V{N} 正脚本 + V{N} down 脚本后，表数量应等于 V{N-1} 基线
# 基线记录：在 prepare_versioned_rollback 之后、apply V{N} 之前，快照表数量
verify_versioned_rollback() {
  local target="$1" down_script="$2"

  # 1. 准备：apply 到 V{N-1}
  prepare_versioned_rollback "$target"

  # 2. 快照 V{N-1} 的表数量
  local tables_before
  tables_before=$(q "SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='guliangiam' AND c.relkind='r'")
  echo "    V$((target - 1)) 基线表数: $tables_before"

  # 3. Apply V{N} 正脚本
  local up="db/postgresql/V${target}__"*.sql
  local up_applied=0
  for f in $up; do
    if [ -f "$f" ]; then
      "${PSQL[@]}" -f "$f" >/dev/null
      up_applied=$((up_applied + 1))
    fi
  done
  if [ "$up_applied" -eq 0 ]; then
    echo "    SKIP: db/postgresql/V${target}__*.sql 不存在（没有正脚本就没有 down）"
    return 0
  fi

  # 4. 执行 V{N} down 脚本
  if ! "${PSQL[@]}" -f "$down_script" 2>/tmp/rollback_err.$$; then
    echo "    FAIL: down.sql 执行错误" >&2
    sed 's/^/      /' /tmp/rollback_err.$$ >&2
    rm -f /tmp/rollback_err.$$
    return 1
  fi
  rm -f /tmp/rollback_err.$$

  # 5. 断言：down 后表数量 == V{N-1} 基线
  local tables_after
  tables_after=$(q "SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='guliangiam' AND c.relkind='r'")
  assert_label "回滚后表数回到 V$((target - 1)) 基线" "$tables_before" "$tables_after"
}

# ── 主流程 ────────────────────────────────────────────────────────────────

echo "=========================================="
echo "  GuliangIAM 回滚脚本独立性验证"
echo "=========================================="
echo ""

# ── Part A: V1 全量回滚（PostgreSQL）──
run_test "V1 全量回滚 — PostgreSQL" \
  "db/rollback/V1__guliangiam_schema_init.down.sql" \
  verify_v1_full_rollback || true

# ── Part B: 版本化增量回滚 ──
# 每个 down 脚本的命名约定：V{N}__{描述}.down.sql
# 正脚本在 db/postgresql/V{N}__{描述}.sql
#
# 遍历 db/rollback/ 中所有 V{2,3,...}__*.down.sql（排除 V1 和方言前缀）

for down_script in db/rollback/V[0-9]*__*.down.sql; do
  [ -f "$down_script" ] || continue

  # 解析版本号：文件名首段 V{N}
  base=$(basename "$down_script")
  ver=$(echo "$base" | sed -n 's/^V\([0-9]*\)__.*/\1/p')
  [ -z "$ver" ] && continue
  [ "$ver" -eq 1 ] && continue  # V1 已在 Part A 单独处理

  echo ""
  echo "--> V${ver} 版本化回滚"
  echo "    down: $down_script"
  echo "    场景: apply up to V$((ver - 1)) → apply V${ver} up → run V${ver} down → verify"

  if verify_versioned_rollback "$ver" "$down_script"; then
    PASS=$((PASS + 1))
    echo "    PASS"
  else
    FAIL=$((FAIL + 1))
    FAILURES+=("V${ver} ${base}")
  fi
done

# ── 汇总 ──
echo ""
echo "=========================================="
echo "  结果: PASS=$PASS  FAIL=$FAIL"
echo "=========================================="

if [ "$FAIL" -gt 0 ]; then
  echo ""
  echo "失败用例:" >&2
  for f in "${FAILURES[@]}"; do
    echo "  ✗ $f" >&2
  done
  exit 1
fi

echo "全部通过 ✅"
