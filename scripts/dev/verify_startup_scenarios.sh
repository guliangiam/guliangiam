#!/usr/bin/env bash
# verify_startup_scenarios.sh — 服务启动的三种数据库组合验证（开发/CI 专用）
#
# 依据：本地启动流程改造（服务启动默认不重放、不导入；-migrate/-seed 显式开启）。
#
# 用法：
#   DATABASE_URL="postgres://operator@127.0.0.1:5432/guliangiam_startup_verify?sslmode=disable" \
#   APP_DATABASE_URL="postgres://guliangiam@127.0.0.1:5432/guliangiam_startup_verify?sslmode=disable" \
#   MIGRATION_DATABASE_URL="postgres://guliangiam_migrator@127.0.0.1:5432/guliangiam_startup_verify?sslmode=disable" \
#   APP_DATABASE_ROLE="guliangiam" \
#   MIGRATION_DATABASE_ROLE="guliangiam_migrator" \
#     ALLOW_SCHEMA_RESET=1 bash scripts/dev/verify_startup_scenarios.sh
# DATABASE_URL 仅用于隔离库重置与检查；APP_DATABASE_URL 用于不迁移的服务启动；
# MIGRATION_DATABASE_URL 用于 -migrate/-seed 服务启动。后二者都必须使用非 superuser、
# 非 BYPASSRLS 角色；迁移角色还需在隔离库中创建 schema 对象的权限。
# APP_DATABASE_ROLE 与 MIGRATION_DATABASE_ROLE 只在隔离库内获得测试所需权限。
#
# 可选：
#   STARTUP_VERIFY_PORT=8041   起始端口（四个场景各用一个，默认 8041..8044）
#
# 被测行为：
#   场景 1（跳过重放 + 跳过导入）
#     1a 空库：必须明确 fail-fast（schema is empty and AUTO_MIGRATE is not enabled），
#              且不得创建任何表（证明没有偷偷重放）；
#     1b 已初始化库：正常起服，日志声明 skipped，账本与数据行不变。
#   场景 2（仅重放，-migrate）：只写入结构链账本，数据链为 0，无管理员。
#     此时完整性校验必然不通过（数据链缺版本、种子表为空、无 admin 凭据），
#     默认严格会拒绝启动；本场景显式 BOOTSTRAP_INTEGRITY_STRICT=false 验证
#     「关闭时继续启动并打 WARN 声明校验被显式跳过」。
#   场景 3（重放 + 导入，-migrate -seed）：结构链 + 数据链 + 管理员哨兵凭据。
#
# 隔离要求：脚本会 DROP SCHEMA guliangiam CASCADE，必须显式 ALLOW_SCHEMA_RESET=1；
# 严禁指向生产库或共享开发库。服务进程在临时目录中启动，不读取仓库根目录的
# .env/.env.local，所有依赖均显式注入，保证结果与开发者本地配置无关。
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$repo_root"

: "${DATABASE_URL:?请设置 DATABASE_URL，例如 postgres://user@127.0.0.1:5432/guliangiam_startup_verify?sslmode=disable}"
 : "${APP_DATABASE_URL:?请设置非 superuser 的 APP_DATABASE_URL}"
 : "${MIGRATION_DATABASE_URL:?请设置非 superuser 的 MIGRATION_DATABASE_URL}"
 : "${MIGRATION_DATABASE_ROLE:?请设置隔离库迁移角色 MIGRATION_DATABASE_ROLE}"
app_database_url=$APP_DATABASE_URL
migration_database_url=$MIGRATION_DATABASE_URL
app_database_role=${APP_DATABASE_ROLE:-}
migration_database_role=$MIGRATION_DATABASE_ROLE
if [ "${ALLOW_SCHEMA_RESET:-}" != "1" ]; then
  echo "该脚本会 DROP SCHEMA guliangiam CASCADE。确认目标是非生产库后，设置 ALLOW_SCHEMA_RESET=1 重跑。" >&2
  exit 2
fi
for required_cmd in psql curl go python3; do
  command -v "$required_cmd" >/dev/null 2>&1 || {
    echo "缺少命令：$required_cmd" >&2
    exit 2
  }
done

base_port=${STARTUP_VERIFY_PORT:-8041}
workdir=$(mktemp -d "${TMPDIR:-/tmp}/guliangiam-startup-verify.XXXXXX")
server_bin="$workdir/guliangiam-server"
migrate_bin="$workdir/guliangiam-migrate"
current_pid=""
server_pid=""
# 场景级额外环境变量（空格分隔，不含空格的值）：默认清空，只有需要显式关闭
# 启动期完整性校验的场景才设置。不能依赖调用者 shell 里的同名变量。
startup_extra_env=""

cleanup() {
  if [ -n "$current_pid" ] && kill -0 "$current_pid" 2>/dev/null; then
    kill "$current_pid" 2>/dev/null || true
    wait "$current_pid" 2>/dev/null || true
  fi
  rm -rf "$workdir"
}
trap cleanup EXIT

psql_quiet() { psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -q "$@"; }
# 断言读取走 super_admin 会话：租户表已 FORCE RLS，默认会话 count 恒为 0，
# 会把「种子已导入管理员」误判为失败。本脚本只校验启动行为，不校验 RLS 本身。
q() { PGOPTIONS='-c app.current_role=super_admin' psql "$DATABASE_URL" -tAc "$1"; }

ok() { echo "    OK: $1"; }
fail() {
  echo "    FAIL: $1" >&2
  exit 1
}
assert_eq() { # assert_eq <label> <want> <got>
  if [ "$3" != "$2" ]; then
    fail "$1（期望 $2，实际 $3）"
  fi
  ok "$1 = $3"
}
# 服务日志是 slog JSON（字段形如 "migrate":false），不是 key=value 文本；
# 断言必须匹配 JSON 形态，否则日志格式升级后脚本会把「行为正确」误判为失败。
assert_log() { # assert_log <log> <needle>
  local log="$1" needle="$2"
  if ! grep -qF -- "$needle" "$log"; then
    echo "    FAIL: 日志缺少「$needle」" >&2
    grep -E "startup database|database bootstrap|server listening|create server" "$log" | sed 's/^/      /' >&2 || true
    exit 1
  fi
  ok "日志包含：$needle"
}
reset_schema() {
  PGOPTIONS='-c client_min_messages=warning' psql_quiet -f scripts/dev/reset_schema.sql >/dev/null
  [[ "$migration_database_role" =~ ^[a-z_][a-z0-9_]*$ ]] || fail "MIGRATION_DATABASE_ROLE 必须是小写 SQL 标识符"
  local database_name
  database_name=$(psql_quiet -tAc "SELECT current_database()" | tr -d '[:space:]')
  [[ "$database_name" =~ ^[a-zA-Z_][a-zA-Z0-9_]*$ ]] || fail "DATABASE_URL 的数据库名不符合隔离测试约定"
  psql_quiet -c "GRANT CREATE ON DATABASE \"$database_name\" TO \"$migration_database_role\""
  psql_quiet -c "GRANT USAGE, CREATE ON SCHEMA guliangiam TO \"$migration_database_role\""
  if [ -n "$app_database_role" ]; then
    [[ "$app_database_role" =~ ^[a-z_][a-z0-9_]*$ ]] || fail "APP_DATABASE_ROLE 必须是小写 SQL 标识符"
    psql_quiet -c "GRANT USAGE, CREATE ON SCHEMA guliangiam TO \"$app_database_role\""
  fi
  ok "重置 schema（空库）"
}
grant_app_data_access() {
  if [ -n "$app_database_role" ]; then
    psql_quiet -c "GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA guliangiam TO \"$app_database_role\""
    psql_quiet -c "GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA guliangiam TO \"$app_database_role\""
    psql_quiet -c "GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA guliangiam TO \"$app_database_role\""
  fi
}

# start_server <log> <port> [server flags...]
# 结果写入全局 server_pid / current_pid：背景进程必须是主 shell 的子进程，
# 否则 `wait` 无法取得退出码（命令替换会把它变成子 shell 的子进程）。
start_server() {
  local log="$1" port="$2" startup_database_url="$app_database_url" flag
  shift 2
  for flag in "$@"; do
    case "$flag" in
      -migrate|-seed) startup_database_url=$migration_database_url ;;
    esac
  done
  (
    cd "$workdir" || exit 1
    # 清空可能干扰裁决的环境变量；服务运行在临时目录，不会加载仓库根目录的 .env。
    # BOOTSTRAP_INTEGRITY_STRICT 默认清空（= 严格），只有场景级开关才显式注入。
    # shellcheck disable=SC2086 # 故意的单词拆分：值本身就是 KEY=VALUE 列表，且不含空格。
    exec env -u AUTO_MIGRATE -u AUTO_SEED -u DB_DSN -u DATABASE_URL -u MYSQL_DSN -u MYSQL_URL \
      -u HTTP_PORT -u ALLOW_CHECKSUM_DRIFT -u SCHEMA_CONTENT_HASH -u BOOTSTRAP_ADMIN_PASSWORD \
      -u BOOTSTRAP_INTEGRITY_STRICT \
      ${startup_extra_env} \
      APP_ENV=development \
      POSTGRES_DSN="$startup_database_url" \
      HTTP_ADDR=":$port" \
      ISSUER_URL="http://127.0.0.1:$port" \
      ENCRYPTION_KEY_CURRENT="$fixture_encryption_key" \
      SESSION_HMAC_KEY="$fixture_hmac_key" \
      BOOTSTRAP_ADMIN_PASSWORD='' \
      "$server_bin" "$@"
  ) >"$log" 2>&1 &
  server_pid=$!
  current_pid=$server_pid
}

wait_for_http() { # wait_for_http <port> <pid> <log>
  local port="$1" pid="$2" log="$3" code waited=0
  while :; do
    if ! kill -0 "$pid" 2>/dev/null; then
      echo "    FAIL: 服务提前退出（日志见下）" >&2
      tail -n 20 "$log" | sed 's/^/      /' >&2 || true
      return 1
    fi
    code=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$port/healthz" 2>/dev/null || true)
    if [ -n "$code" ] && [ "$code" != "000" ]; then
      ok "服务就绪（healthz=$code）"
      return 0
    fi
    waited=$((waited + 1))
    if [ "$waited" -ge 60 ]; then
      echo "    FAIL: 等待服务就绪超时" >&2
      tail -n 20 "$log" | sed 's/^/      /' >&2 || true
      return 1
    fi
    sleep 1
  done
}

stop_server() { # stop_server <pid>
  local pid="$1"
  kill "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
  current_pid=""
}

expect_startup_failure() { # expect_startup_failure <pid> <log> <message fragment>
  local pid="$1" log="$2" fragment="$3" waited=0 status=0
  while kill -0 "$pid" 2>/dev/null; do
    waited=$((waited + 1))
    if [ "$waited" -ge 30 ]; then
      kill "$pid" 2>/dev/null || true
      fail "进程仍在运行，未按预期拒绝启动"
    fi
    sleep 1
  done
  wait "$pid" || status=$?
  current_pid=""
  if [ "$status" -eq 0 ]; then
    fail "进程意外成功退出（期望非零）"
  fi
  ok "启动被拒绝（exit=$status）"
  assert_log "$log" "$fragment"
}

echo "=========================================="
echo "  GuliangIAM 启动场景验证"
echo "=========================================="
echo ""

struct_chain_len=$(find internal/bootstrap/schema_sql -maxdepth 1 -name 'V*.sql' | wc -l | tr -d ' ')
# seed_chain_len 取自 Go embed 源 internal/bootstrap/seed_sql/（含种子 V1 + data-enrich V2），
# db/seeds/ 仅放种子镜像（sync-seed-mirror.sh 会跳过 data-enrich）。
seed_chain_len=$(find internal/bootstrap/seed_sql -maxdepth 1 -name 'V*.sql' | wc -l | tr -d ' ')
[ "$struct_chain_len" -gt 0 ] || fail "未找到结构迁移脚本 internal/bootstrap/schema_sql/V*.sql"
[ "$seed_chain_len" -gt 0 ] || fail "未找到数据迁移脚本 internal/bootstrap/seed_sql/V*.sql"

echo "--> 构建验证用二进制"
go build -o "$server_bin" . || fail "构建服务二进制失败"
go build -o "$migrate_bin" ./cmd/migrate || fail "构建迁移命令失败"
ok "服务与迁移命令已构建（临时目录）"

# 固定夹具：仅满足配置校验，不是任何环境的真实凭据。
fixture_encryption_key=$(python3 -c 'import base64; print(base64.b64encode(b"0"*32).decode())')
fixture_hmac_key=$(python3 -c 'import secrets; print(secrets.token_hex(32))')

sentinel_hash=$(grep -o '"\$argon2id[^"]*"' internal/bootstrap/migrate.go | head -n 1 | tr -d '"')
[ -n "$sentinel_hash" ] || fail "未能从 internal/bootstrap/migrate.go 解析 SeedBootstrapAdminHash"

echo ""
echo "=== 场景 1/3：跳过重放 + 跳过导入 ==="
echo "--> 1a 空库（必须拒绝启动且不建表）"
reset_schema
assert_eq "空库 guliangiam 表数" "0" "$(q "SELECT count(*) FROM information_schema.tables WHERE table_schema='guliangiam'")"
log_1a="$workdir/1a-skip-empty.log"
start_server "$log_1a" "$base_port"
expect_startup_failure "$current_pid" "$log_1a" "schema is empty and AUTO_MIGRATE is not enabled"
assert_log "$log_1a" '"migrate":false,"migrate_source":"default","seed":false,"seed_source":"default"'
assert_log "$log_1a" "database bootstrap skipped"
assert_eq "裸启动后 guliangiam 表数（未偷偷重放）" "0" "$(q "SELECT count(*) FROM information_schema.tables WHERE table_schema='guliangiam'")"

echo ""
echo "--> 1b 已初始化库（正常起服且账本不变）"
reset_schema
init_log="$workdir/1b-init.log"
if ! env -u AUTO_MIGRATE -u AUTO_SEED -u DB_DSN -u DATABASE_URL -u MYSQL_DSN -u MYSQL_URL \
  APP_ENV=development BOOTSTRAP_ADMIN_PASSWORD='' \
  "$migrate_bin" up --seed -dsn "$DATABASE_URL" >"$init_log" 2>&1; then
  tail -n 20 "$init_log" | sed 's/^/      /' >&2 || true
  fail "初始化库失败（migrate up --seed）"
fi
ok "初始化完成（migrate up --seed）"
grant_app_data_access
assert_eq "初始化后结构链账本" "$struct_chain_len" \
  "$(q "SELECT count(*) FROM guliangiam.schema_migrations WHERE kind='structure'")"
assert_eq "初始化后数据链账本" "$seed_chain_len" \
  "$(q "SELECT count(*) FROM guliangiam.schema_migrations WHERE kind='data'")"
ledger_before=$(q "SELECT coalesce(max(applied_at)::text, '') FROM guliangiam.schema_migrations")
orgs_before=$(q "SELECT count(*) FROM guliangiam.organizations")

log_1b="$workdir/1b-skip-ready.log"
start_server "$log_1b" "$((base_port + 1))"
wait_for_http "$((base_port + 1))" "$current_pid" "$log_1b"
assert_log "$log_1b" '"migrate":false,"migrate_source":"default","seed":false,"seed_source":"default"'
assert_log "$log_1b" "database bootstrap skipped"
assert_log "$log_1b" "server listening"
stop_server "$current_pid"
assert_eq "裸启动未改写账本 applied_at" "$ledger_before" \
  "$(q "SELECT coalesce(max(applied_at)::text, '') FROM guliangiam.schema_migrations")"
assert_eq "裸启动未改写业务数据" "$orgs_before" "$(q "SELECT count(*) FROM guliangiam.organizations")"

echo ""
echo "=== 场景 2/3：仅重放（-migrate）==="
echo "（数据链/种子表/管理员均缺失，完整性校验默认会拒绝启动；此处显式关闭校验）"
reset_schema
log_2="$workdir/2-migrate-only.log"
startup_extra_env="BOOTSTRAP_INTEGRITY_STRICT=false"
start_server "$log_2" "$((base_port + 2))" -migrate
startup_extra_env=""
wait_for_http "$((base_port + 2))" "$current_pid" "$log_2"
assert_log "$log_2" '"migrate":true,"migrate_source":"flag","seed":false,"seed_source":"default"'
assert_log "$log_2" "database bootstrap finished"
assert_log "$log_2" '"applied_now":'"$struct_chain_len"
assert_log "$log_2" "bootstrap integrity check explicitly skipped"
assert_log "$log_2" '"switch":"BOOTSTRAP_INTEGRITY_STRICT=false"'
assert_eq "仅重放后结构链账本" "$struct_chain_len" \
  "$(q "SELECT count(*) FROM guliangiam.schema_migrations WHERE kind='structure'")"
assert_eq "仅重放后数据链账本（应为 0）" "0" \
  "$(q "SELECT count(*) FROM guliangiam.schema_migrations WHERE kind='data'")"
assert_eq "仅重放后管理员数（应为 0）" "0" "$(q "SELECT count(*) FROM guliangiam.users WHERE is_admin = true")"
stop_server "$current_pid"

echo ""
echo "=== 场景 3/3：重放 + 导入（-migrate -seed）==="
reset_schema
log_3="$workdir/3-migrate-seed.log"
start_server "$log_3" "$((base_port + 3))" -migrate -seed
wait_for_http "$((base_port + 3))" "$current_pid" "$log_3"
assert_log "$log_3" '"migrate":true,"migrate_source":"flag","seed":true,"seed_source":"flag"'
assert_log "$log_3" "database bootstrap finished"
assert_log "$log_3" '"applied_now":'"$((struct_chain_len + seed_chain_len))"
assert_eq "重放+导入后结构链账本" "$struct_chain_len" \
  "$(q "SELECT count(*) FROM guliangiam.schema_migrations WHERE kind='structure'")"
assert_eq "重放+导入后数据链账本" "$seed_chain_len" \
  "$(q "SELECT count(*) FROM guliangiam.schema_migrations WHERE kind='data'")"
assert_eq "重放+导入后管理员数" "1" "$(q "SELECT count(*) FROM guliangiam.users WHERE is_admin = true")"
assert_eq "内置管理员账号" "admin" \
  "$(q "SELECT username FROM guliangiam.users ORDER BY username LIMIT 1")"
credential_hash=$(q "SELECT password_hash FROM guliangiam.user_credentials LIMIT 1")
if [ "$credential_hash" != "$sentinel_hash" ]; then
  fail "管理员凭据与开发哨兵不匹配"
fi
ok "管理员凭据匹配开发哨兵（值不输出）"
assert_eq "强制首登改密标记" "true" \
  "$(q "SELECT need_update_password::text FROM guliangiam.user_credentials LIMIT 1")"
stop_server "$current_pid"

echo ""
echo "=========================================="
echo "通过：三种启动场景全部通过"
echo "=========================================="
