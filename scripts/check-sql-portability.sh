#!/usr/bin/env bash
# check-sql-portability.sh — SQL 可移植性门禁（多数据库兼容版）
#
# 依据：《GuliangIAM 开发规范手册》第 3 章「数据库设计规范（多数据库兼容版 V3）」
#       3.3.1 受限 SQL 子集 / 3.3.2 逻辑类型映射，以及 ADR-023（多数据库方言抽象）
# 用法：bash scripts/check-sql-portability.sh [repo_root]
#
# 检查项：
#   1. 方言隔离：Go 代码中不得出现方言专有 SQL 构造（白名单见下，白名单必须显式登记）
#   2. 逻辑类型：DDL 中不得出现不可移植/不合规的物理类型（MONEY/FLOAT/SERIAL/...）
#   3. 报告 DDL 中的方言专有构造统计（非阻塞；衡量 DB-041 / DB-048 整改体量）
#   4. 报告方言抽象层实现现状（非阻塞；DB-039）
#
# 说明：`db/postgresql/` 是 PostgreSQL 方言产物，出现 JSONB/TEXT[]/INET/BYTEA/部分索引
#       属预期行为，只做统计不判失败；真正的约束是「方言构造不得泄漏到业务代码」。

# 输入：internal/**/*.go 中 SQL 构造
# 输出：PASS/FAIL + 方言构造泄漏位置；并输出存量统计
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
SCHEMA_DIR="db/postgresql"
DIALECT_DIR="internal/infra/dialect"
TARGET_REPO_DIR="internal/infra/postgres"

fail() {
  echo -e "${RED}  VIOLATION: $*${NC}"
  VIOLATIONS=$((VIOLATIONS + 1))
}

pass() {
  echo -e "${GREEN}  OK: $*${NC}"
}

warn() {
  echo -e "${YELLOW}  NOTE: $*${NC}"
}

echo "=========================================="
echo " SQL 可移植性门禁（多数据库兼容版 V3）"
echo " 依据：开发规范手册 3.3.1 / 3.3.2 + ADR-023"
echo "=========================================="

# 方言专有 SQL 构造（Go 代码中禁止直接书写）
# 注意：均为 ERE，使用 \b 词边界，避免 JSONB64 / json_schema 之类误报。
DIALECT_PATTERN='\bON CONFLICT\b|\bRETURNING\b|\bJSONB\b|\bSKIP LOCKED\b|\bILIKE\b|\bDISTINCT ON\b|FILTER[[:space:]]*\([[:space:]]*WHERE|\bgen_random_uuid\b|\bpgcrypto\b|\bON DUPLICATE KEY UPDATE\b|\bMERGE INTO\b|\bBIGSERIAL\b|\bSMALLSERIAL\b|::(uuid|text|jsonb|json|timestamptz|timestamp|inet|cidr|numeric|decimal|boolean|int8|bytea)\b|set_config[[:space:]]*\(|\bSET LOCAL\b'

# 白名单：file path -> 理由。新增条目必须写明理由与整改编号。
declare -A ALLOWLIST=(
  # 迁移执行器的账本写入是唯一「允许内联方言 DDL」的位置：账本表由执行器自建，不属于
  # 声明式 Schema（ADR-022 决策 2 / 附录 A.2.2），因此无法由生成器产出、也无 Dialect
  # 抽象可经（生成器只实现 PostgreSQL）。此处 ON CONFLICT 的语义是「账本行已存在即忽略」，
  # 即迁移的幂等性本身，不可替换为方言中立的写法。
  [internal/platform/db/migrate.go]="迁移账本由执行器自建且不在声明式 Schema 内，其幂等写入无 Dialect 抽象可经（ADR-022 附录 A.2.2）"
  # 租户上下文 GUC 的写入位置。RLS 与 `app.*` 自定义 GUC 都是 PostgreSQL 族
  # （PostgreSQL / GaussDB / KingBaseES）独有机制：MySQL 方言根本没有对应语义，
  # 因此不存在可承载它的 Dialect 实现，也无法改写为方言中立写法。且 PostgreSQL
  # 对自定义 GUC **不接受** `SET LOCAL app.x = ...` 语法，只能经 set_config() 调用
  # （见该文件头部陷阱说明），故 `set_config(` 在此是唯一正确写法（ADR-016 / ADR-022 §4）。
  [internal/platform/db/rls.go]="RLS 与 app.* 自定义 GUC 为 PostgreSQL 族独有，MySQL 无对应语义，无 Dialect 可经；PG 不允许 SET LOCAL 写自定义 GUC（ADR-016）"
  # pgx 方言启动层：连接池的会话级 GUC 清理与 super_admin 上下文注入只能经
  # set_config() 执行（同 rls.go）；该文件是 PostgreSQL 方言实现，不承载业务 SQL。
  [internal/platform/db/postgres.go]="pgx 方言启动层（会话 GUC 清理/super_admin 上下文），属 PostgreSQL 专属实现，非业务 SQL"
  # Casbin enforcer 的 UUID 解析与 super_admin GUC 绕过：仅 PostgreSQL 基线使用
  # （MySQL 基线不部署 Casbin enforcer，鉴权降级路径见 ADR-023）。
  [internal/infra/casbin/manager.go]="Casbin enforcer 的 PG 专属初始化（pgx + super_admin GUC 绕过 RLS），MySQL 基线不使用"
  # MySQL 方言仓储实现：ON DUPLICATE KEY UPDATE 是 MySQL 原生 upsert，文件位于
  # infra/mysql/，正是方言隔离的落点（ADR-023 第二基线）。
  [internal/infra/mysql/aigateway/sql.go]="MySQL 方言仓储实现，ON DUPLICATE KEY UPDATE 为 MySQL 原生 upsert（ADR-023 第二基线）"
  [internal/infra/mysql/applications/client_certificates.go]="MySQL 方言仓储实现，ON DUPLICATE KEY UPDATE 为 MySQL 原生 upsert（ADR-023 第二基线）"
  [internal/infra/mysql/applications/client_jwks.go]="MySQL 方言仓储实现，ON DUPLICATE KEY UPDATE 为 MySQL 原生 upsert（ADR-023 第二基线）"
  [internal/infra/mysql/dpop/sql.go]="MySQL 方言仓储实现，ON DUPLICATE KEY UPDATE 为 MySQL 原生 upsert（ADR-023 第二基线）"
  [internal/infra/mysql/iga/sql.go]="MySQL 方言仓储实现，ON DUPLICATE KEY UPDATE 为 MySQL 原生 upsert（ADR-023 第二基线）"
  [internal/infra/mysql/usernotifications/repository.go]="MySQL 方言仓储实现，通知偏好/渠道绑定的 ON DUPLICATE KEY UPDATE 为 MySQL 原生 upsert（ADR-023 第二基线）"
  # users 聚合写：主表 core + 5 张 user_* 子表的幂等 upsert，同上（ADR-023 第二基线）。
  [internal/infra/mysql/users/sql.go]="MySQL 方言仓储实现，ON DUPLICATE KEY UPDATE 为 MySQL 原生 upsert（ADR-023 第二基线）"
  # 迁移执行器与迁移后完整性校验：两者签名即绑定 *pgxpool.Pool，且 MySQL 基线不跑
  # 迁移链（双基线 DDL 由脚本维护，见 runtime_factory.go），因此这里的 set_config /
  # ON CONFLICT 只可能运行在 PostgreSQL 族上，无方言中立写法可替换。
  [internal/bootstrap/migrate.go]="迁移执行器仅 PostgreSQL 族运行（MySQL 基线由 DDL 脚本维护，不跑迁移链）；super_admin GUC 与账本 upsert 无方言中立写法"
  [internal/bootstrap/integrity.go]="迁移后完整性校验签名绑定 pgxpool，仅 PostgreSQL 族运行；读取 RLS 表需 super_admin GUC"
  [internal/bootstrap/bootstrap_admin_scope_check.go]="启动期 RLS 诊断签名绑定 *pgxpool.Pool，仅 PostgreSQL 族运行；读取 platform_admins 需 super_admin GUC，无方言中立写法（ADR-016）"
  # Casbin 适配层：PG 侧 v5 JSON 的时间/生命周期过滤使用 timestamptz 转换；
  # MySQL 侧由 internal/infra/mysql/casbin 提供等价实现（JIT 在 Go 内求值）。
  [internal/infra/casbin/adapter.go]="Casbin PG 适配层的时间窗口过滤需 timestamptz 转换；MySQL 方言由 internal/infra/mysql/casbin 实现（ADR-023）"
  [internal/infra/mysql/casbin/sql.go]="MySQL 方言策略存储实现，ON DUPLICATE KEY UPDATE 为 MySQL 原生 upsert（ADR-023 第二基线）"
)
allowlisted() {
  local f="$1"
  if [ -n "${ALLOWLIST[$f]:-}" ]; then
    echo "${ALLOWLIST[$f]}"
    return 0
  fi
  return 1
}

echo
echo "[1/4] 方言隔离检查（Go 代码不得出现方言专有 SQL 构造）"
go_sources=()
while IFS= read -r f; do
  go_sources+=("$f")
done < <(find internal -name '*.go' -type f \
  -not -path "$DIALECT_DIR/*" -not -path "$TARGET_REPO_DIR/*" -not -name '*_test.go' | sort)

if [ ${#go_sources[@]} -eq 0 ]; then
  fail "在 internal/ 下未找到任何 Go 源文件（--exclude 条件可能写错）"
else
  # 先剥离行尾注释，避免「代码 + 注释里提到 JSONB」被误判为方言泄漏。
  stripped=$(mktemp)
  for f in "${go_sources[@]}"; do
    sed 's|//.*||' "$f" | sed 's/"[^"]*"//g' | awk -v F="$f" '{ printf "%s:%d:%s\n", F, NR, $0 }' >> "$stripped"
  done
  hits=$(grep -E "$DIALECT_PATTERN" "$stripped" 2>/dev/null || true)
  rm -f "$stripped"
  if [ -z "$hits" ]; then
    pass "未发现方言专有构造（已排除 $DIALECT_DIR/ 与 $TARGET_REPO_DIR/，已剥离注释）"
  else
    seen_allowed=""
    while IFS= read -r line; do
      file="${line%%:*}"
      if reason=$(allowlisted "$file"); then
        case " $seen_allowed " in
          *" $file "*) ;;
          *) warn "白名单 $file — $reason"; seen_allowed="$seen_allowed $file" ;;
        esac
      else
        fail "$line"
      fi
    done <<< "$hits"
  fi
fi

echo
echo "[2/4] 逻辑类型检查（DDL 不得使用不可移植/不合规的物理类型）"
forbidden='\bMONEY\b|\bFLOAT4\b|\bFLOAT8\b|\bREAL\b|DOUBLE PRECISION|TIMESTAMP WITHOUT TIME ZONE|\bSERIAL\b|\bBIGSERIAL\b|\bSMALLSERIAL\b|\bOID\b|\bXML\b|\bNUMERIC\b|CHARACTER VARYING|\bJSON\b'
if [ ! -d "$SCHEMA_DIR" ]; then
  fail "$SCHEMA_DIR 不存在（Schema 目录被移动或删除）"
else
  # 注释（-- 行注释与 COMMENT ON 说明文字）里的类型词不是物理列类型，先排除再判。
  bad=$(grep -nE "$forbidden" "$SCHEMA_DIR"/*.sql 2>/dev/null     | grep -vE ':[0-9]+:[[:space:]]*--'     | grep -vE ':[0-9]+:[[:space:]]*COMMENT ON' || true)
  if [ -z "$bad" ]; then
    pass "未发现不可移植/不合规物理类型（金额 DECIMAL、币种 CHAR、主键 UUID）"
  else
    while IFS= read -r line; do
      [ -n "$line" ] && fail "$line"
    done <<< "$bad"
  fi
fi

echo
echo "[3/4] DDL 方言专有构造统计（非阻塞；DB-041 / DB-048 整改体量）"
report_patterns=(
  "ON CONFLICT"
  "RETURNING"
  "JSONB"
  "TEXT\[\]|UUID\[\]"
  "\bINET\b|\bCIDR\b"
  "\bBYTEA\b"
  "DO \\\$\\\$"
  "plpgsql"
  "guliangiam\.uuidv7"
  "pgcrypto"
  "SKIP LOCKED"
  "CREATE INDEX$|CONCURRENTLY"
  "PARTITION BY|PARTITION OF"
  "ROW LEVEL SECURITY|CREATE POLICY"
  "EXTENSION IF NOT EXISTS"
)
for p in "${report_patterns[@]}"; do
  n=$(grep -cE "$p" "$SCHEMA_DIR"/*.sql 2>/dev/null | awk -F: '{s+=$NF} END{print s+0}' || true)
  printf "  %-42s %s\n" "$p" "${n:-0}"
done
echo "  以上构造仅在 PostgreSQL 方言产物中出现属预期；若出现在 $SCHEMA_DIR 之外即为缺陷。"

echo
echo "[4/4] 方言抽象层现状（非阻塞；DB-039 / DB-040）"
if [ -d "$DIALECT_DIR" ]; then
  impls=$(ls -1 "$DIALECT_DIR" 2>/dev/null | grep -vE '_test\.go$' | tr '\n' ' ' || true)
  warn "$DIALECT_DIR 存在，文件：${impls:-（空）}"
  refs=$(grep -rl 'infra/dialect' internal --include='*.go' 2>/dev/null | grep -v "^$DIALECT_DIR/" | wc -l | tr -d ' ' || true)
  if [ "${refs:-0}" = "0" ]; then
    warn "目标方言仓储 $TARGET_REPO_DIR/ 直接写 PG SQL 属合法（当前进程运行时即 import 该仓储）。未来扩展国产库/多租户方言时，新仓储将强制走 shared.Dialect 生成。"
  else
    pass "有 $refs 个包引用 Dialect"
  fi
else
  warn "$DIALECT_DIR 不存在（DB-039：[Target]）"
fi
drivers=$(grep -cE 'jackc/pgx|go-sql-driver/mysql|kingbase|dm-go|godror' go.mod 2>/dev/null | head -1 || true)
warn "go.mod 中数据库驱动条目数：${drivers:-0}（当前仅 pgx/v5，DB-040）"

echo
echo "=========================================="
if [ "$VIOLATIONS" -eq 0 ]; then
  echo -e "${GREEN}SQL 可移植性检查通过（0 项违规）${NC}"
  echo "（非阻塞项见上方 NOTE，均为 [Target] 整改范围）"
  exit 0
else
  echo -e "${RED}SQL 可移植性检查失败：$VIOLATIONS 项违规${NC}"
  exit 1
fi
