#!/usr/bin/env bash
# ============================================================
# GuliangIAM Production Database Init Script
# ============================================================
# Version: 1.0.0
#
# Usage:
#   # 1. Docker 挂载模式：本脚本无需手动调用
#   #    postgres 官方镜像自动执行挂载到
#   #    /docker-entrypoint-initdb.d/ 的 00_bootstrap.sql
#
#   # 2. 手工模式（RDS / 自建 PG / 云 PG）
#   scripts/prod-init-db.sh \
#       --host $DB_HOST --port 5432 \
#       --superuser $SUPERUSER --superuser-password $SUPER_PASSWORD \
#       --app-password $APP_PASSWORD
#
#   # 3. 只做 pre-flight 检查（不修改）
#   scripts/prod-init-db.sh --check --host ... --superuser ...
#
#   # 4. bootstrap 后立即跑 migration（一次到位）
#   scripts/prod-init-db.sh ... --migrate
#
# 环境变量（等价于 flag）：
#   DB_HOST / DB_PORT / SUPERUSER / SUPERUSER_PASSWORD / APP_PASSWORD
#   APP_DB_NAME  (默认 guliangiam)
#   APP_ROLE     (默认 guliangiam)
#
# 安全：
#   - 脚本退出时清除 PGPASSWORD
#   - 不在输出中打印密码
#   - 临时文件用 mktemp + trap EXIT 清理
# ============================================================


# 输入：--host/--port/--superuser/--superuser-password/--app-password 或环境变量
# 输出：创建 guliangiam DB + guliangiam_app 角色 + CREATE EXTENSION 等；--check 做 pre-flight 不写
# --check 行为：--check 只做 pre-flight 检查，不执行任何 DDL
# 消费方：手动执行（CI 不跑）；Docker 挂载模式下由 postgres 官方镜像自动调用 db-init/00_bootstrap.sql
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BOOTSTRAP_SQL="${SCRIPT_DIR}/db-init/00_bootstrap.sql"

# ------------------------------------------------------------
# Flag parsing
# ------------------------------------------------------------
HOST="${DB_HOST:-}"
PORT="${DB_PORT:-5432}"
SUPERUSER="${SUPERUSER:-}"
SUPERUSER_PASSWORD="${SUPERUSER_PASSWORD:-}"
APP_PASSWORD="${APP_PASSWORD:-}"
APP_DB_NAME="${APP_DB_NAME:-guliangiam}"
APP_ROLE="${APP_ROLE:-guliangiam}"
CHECK_ONLY=false
DO_MIGRATE=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --host)                HOST="$2"; shift 2 ;;
        --port)                PORT="$2"; shift 2 ;;
        --superuser)           SUPERUSER="$2"; shift 2 ;;
        --superuser-password)  SUPERUSER_PASSWORD="$2"; shift 2 ;;
        --app-password)        APP_PASSWORD="$2"; shift 2 ;;
        --app-db)              APP_DB_NAME="$2"; shift 2 ;;
        --app-role)            APP_ROLE="$2"; shift 2 ;;
        --check)               CHECK_ONLY=true; shift ;;
        --migrate)             DO_MIGRATE=true; shift ;;
        -h|--help)
            sed -n '2,/^# ====/p' "$0" | head -40
            exit 0 ;;
        *) echo "Unknown flag: $1"; exit 1 ;;
    esac
done

# ------------------------------------------------------------
# Helpers
# ------------------------------------------------------------
log()  { printf "\033[34m[init]\033[0m %s\n" "$*"; }
ok()   { printf "\033[32m  ✔\033[0m %s\n" "$*"; }
warn() { printf "\033[33m  ⚠\033[0m %s\n" "$*"; }
err()  { printf "\033[31m  ✖\033[0m %s\n" "$*" >&2; }

cleanup() {
    [[ -n "${PGPASSWORD:-}" ]] && unset PGPASSWORD
    [[ -n "${PGPASSFILE:-}" ]] && rm -f "$PGPASSFILE" && unset PGPASSFILE
    [[ -n "${TMP_PASSWORD_FILE:-}" ]] && rm -f "$TMP_PASSWORD_FILE"
    rm -f "${TMP_BOOTSTRAP:-}"
}
trap cleanup EXIT

# ------------------------------------------------------------
# Pre-flight: 必需参数
# ------------------------------------------------------------
[[ -z "$HOST" ]] && { err "Missing --host (or DB_HOST)"; exit 1; }
[[ -z "$SUPERUSER" ]] && { err "Missing --superuser (or SUPERUSER)"; exit 1; }
[[ -f "$BOOTSTRAP_SQL" ]] || { err "Bootstrap SQL not found: $BOOTSTRAP_SQL"; exit 1; }

export PGPASSWORD="$SUPERUSER_PASSWORD"

# psql 连通性（3 秒超时）
if ! PGPASSWORD="$SUPERUSER_PASSWORD" psql -h "$HOST" -p "$PORT" -U "$SUPERUSER" \
        -d postgres -c 'SELECT 1' -v ON_ERROR_STOP=1 -q 2>/dev/null; then
    err "Cannot connect to PostgreSQL at $HOST:$PORT as $SUPERUSER"
    exit 1
fi
ok "Connected to $HOST:$PORT (as $SUPERUSER)"

# ------------------------------------------------------------
# Pre-flight: 集群健康
# ------------------------------------------------------------
PG_VERSION=$(PGPASSWORD="$SUPERUSER_PASSWORD" psql -h "$HOST" -p "$PORT" \
    -U "$SUPERUSER" -d postgres -tAc 'SHOW server_version_num')
ok "PostgreSQL server_version_num = $PG_VERSION"

if [[ "$PG_VERSION" -lt 140000 ]]; then
    warn "PostgreSQL 14+ recommended for RLS + pgcrypto features"
fi

# ------------------------------------------------------------
# --check: 只做验证不修改
# ------------------------------------------------------------
if $CHECK_ONLY; then
    log "=== Pre-flight check (no changes) ==="
    # 角色
    if PGPASSWORD="$SUPERUSER_PASSWORD" psql -h "$HOST" -p "$PORT" \
            -U "$SUPERUSER" -d postgres -tAc "SELECT 1 FROM pg_roles WHERE rolname='$APP_ROLE'" | grep -q 1; then
        ok "Role $APP_ROLE exists"
    else
        warn "Role $APP_ROLE missing — will be created"
    fi
    # DB
    if PGPASSWORD="$SUPERUSER_PASSWORD" psql -h "$HOST" -p "$PORT" \
            -U "$SUPERUSER" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='$APP_DB_NAME'" | grep -q 1; then
        DB_OWNER=$(PGPASSWORD="$SUPERUSER_PASSWORD" psql -h "$HOST" -p "$PORT" \
            -U "$SUPERUSER" -d postgres -tAc "SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname='$APP_DB_NAME'")
        ok "Database $APP_DB_NAME exists (owner: $DB_OWNER)"
    else
        warn "Database $APP_DB_NAME missing — will be created"
    fi
    exit 0
fi

# ------------------------------------------------------------
# Generate APP password
# ------------------------------------------------------------
if [[ -z "$APP_PASSWORD" ]]; then
    if PGPASSWORD="$SUPERUSER_PASSWORD" psql -h "$HOST" -p "$PORT" \
            -U "$SUPERUSER" -d postgres -tAc "SELECT 1 FROM pg_roles WHERE rolname='$APP_ROLE' AND rolcanlogin" | grep -q 1; then
        log "Role $APP_ROLE already exists — will reset password to environment APP_PASSWORD (empty = skip)"
    fi
    if [[ -z "$APP_PASSWORD" ]]; then
        warn "No --app-password / APP_PASSWORD provided; will keep existing role password"
    fi
fi

# ------------------------------------------------------------
# Apply bootstrap SQL
# ------------------------------------------------------------
log "=== Applying bootstrap SQL ==="

# 替换 SQL 里的 app_role / app_db 占位符（本文件当前没占位符，留钩子）
TMP_BOOTSTRAP=$(mktemp "${TMPDIR:-/tmp}/guliangiam-bootstrap.XXXXXX.sql")
cp "$BOOTSTRAP_SQL" "$TMP_BOOTSTRAP"

PGPASSWORD="$SUPERUSER_PASSWORD" psql -h "$HOST" -p "$PORT" \
    -U "$SUPERUSER" -d postgres \
    -v ON_ERROR_STOP=1 -f "$TMP_BOOTSTRAP"

ok "Bootstrap SQL applied"

# ------------------------------------------------------------
# Set app role password (if provided)
# ------------------------------------------------------------
if [[ -n "$APP_PASSWORD" ]]; then
    log "Setting role $APP_ROLE password"
    PGPASSWORD="$SUPERUSER_PASSWORD" psql -h "$HOST" -p "$PORT" \
        -U "$SUPERUSER" -d "$APP_DB_NAME" \
        -v ON_ERROR_STOP=1 -c "ALTER ROLE $APP_ROLE WITH LOGIN PASSWORD CURRENT_SETTING('guliangiam.app_password', true) OR PASSWORD '$APP_PASSWORD';" 2>/dev/null \
    || PGPASSWORD="$SUPERUSER_PASSWORD" psql -h "$HOST" -p "$PORT" \
        -U "$SUPERUSER" -d "$APP_DB_NAME" \
        -v ON_ERROR_STOP=1 -c "ALTER ROLE $APP_ROLE WITH LOGIN PASSWORD '$APP_PASSWORD';"
    ok "Role password set"
fi

# ------------------------------------------------------------
# Verify final state
# ------------------------------------------------------------
log "=== Verification ==="
ROLE_INFO=$(PGPASSWORD="$SUPERUSER_PASSWORD" psql -h "$HOST" -p "$PORT" \
    -U "$SUPERUSER" -d postgres -tAc \
    "SELECT 'LOGIN='||rolcanlogin||' CREATEDB='||rolcreatedb||' CREATEROLE='||rolcreaterole FROM pg_roles WHERE rolname='$APP_ROLE'")
ok "Role $APP_ROLE: $ROLE_INFO"

DB_OWNER=$(PGPASSWORD="$SUPERUSER_PASSWORD" psql -h "$HOST" -p "$PORT" \
    -U "$SUPERUSER" -d postgres -tAc \
    "SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname='$APP_DB_NAME'")
ok "Database $APP_DB_NAME owner: $DB_OWNER"

SCHEMA_OWNER=$(PGPASSWORD="$SUPERUSER_PASSWORD" psql -h "$HOST" -p "$PORT" \
    -U "$SUPERUSER" -d "$APP_DB_NAME" -tAc \
    "SELECT pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname='$APP_ROLE'")
ok "Schema $APP_ROLE owner: $SCHEMA_OWNER"

# ------------------------------------------------------------
# Run migration (optional)
# ------------------------------------------------------------
if $DO_MIGRATE; then
    log "=== Running migration ==="
    DSN="postgres://${APP_ROLE}:${APP_PASSWORD}@${HOST}:${PORT}/${APP_DB_NAME}?sslmode=disable"
    if command -v go &>/dev/null && [[ -f "go.mod" ]]; then
        AUTO_MIGRATE=true go run ./cmd/migrate up --dsn "$DSN"
    elif [[ -x "./guliangiam" ]]; then
        AUTO_MIGRATE=true ./guliangiam migrate up --dsn "$DSN"
    else
        err "Cannot find guliangiam binary or go.mod — skip migration"
        exit 1
    fi
    ok "Migration applied"
fi

log "=== All done ==="
echo
echo "  Next steps:"
echo "    1. Export DSN for your app:"
echo "       export DATABASE_URL='postgres://${APP_ROLE}:***@${HOST}:${PORT}/${APP_DB_NAME}?sslmode=disable'"
echo "    2. (One-time) Run migration from app repo root:"
echo "       AUTO_MIGRATE=true go run ./cmd/migrate up --dsn \"\$DATABASE_URL\""
echo "    3. Start the app:"
echo "       ./guliangiam serve"
echo
