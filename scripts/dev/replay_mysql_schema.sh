#!/usr/bin/env bash
# Replay the generated MySQL V1 schema into a throwaway database and verify the
# table count matches the generated DDL. Evidence tooling for OPEN-API-37.
#
# Guard rails: the target database name must contain "_test" and
# ALLOW_MYSQL_SCHEMA_RESET=1 must be set (this script drops the database).
#
# Required env:
#   MYSQL_HOST, MYSQL_PORT, MYSQL_USER, MYSQL_DATABASE
# Optional:
#   MYSQL_PASSWORD, MYSQL_SOCKET (overrides host/port), MYSQL_ARGS
set -euo pipefail

: "${MYSQL_DATABASE:?MYSQL_DATABASE is required}"
: "${MYSQL_USER:?MYSQL_USER is required}"
: "${ALLOW_MYSQL_SCHEMA_RESET:?ALLOW_MYSQL_SCHEMA_RESET=1 is required}"

case "$MYSQL_DATABASE" in
  *_test*) ;;
  *) echo "refusing to drop non-test database: $MYSQL_DATABASE" >&2; exit 1 ;;
esac

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ddl="$root/db/mysql/V1__guliangiam_schema_init.sql"
expected="$(grep -c '^CREATE TABLE' "$ddl")"

client_args=()
if [[ -n "${MYSQL_SOCKET:-}" ]]; then
  client_args+=(--socket="$MYSQL_SOCKET")
else
  client_args+=(--host="${MYSQL_HOST:-127.0.0.1}" --port="${MYSQL_PORT:-3306}")
fi
client_args+=(--user="$MYSQL_USER")
if [[ -n "${MYSQL_PASSWORD:-}" ]]; then
  client_args+=(--password="$MYSQL_PASSWORD")
fi
# shellcheck disable=SC2206
client_args+=(${MYSQL_ARGS:-})

mysql "${client_args[@]}" -e "DROP DATABASE IF EXISTS \`$MYSQL_DATABASE\`; CREATE DATABASE \`$MYSQL_DATABASE\` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"
mysql "${client_args[@]}" "$MYSQL_DATABASE" < "$ddl"

actual="$(mysql "${client_args[@]}" -N -B -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = '$MYSQL_DATABASE'")"
if [[ "$actual" != "$expected" ]]; then
  echo "FAIL: MySQL replayed $actual tables, generated DDL declares $expected" >&2
  exit 1
fi
echo "PASS: MySQL V1 replayed ($actual tables) into $MYSQL_DATABASE"
