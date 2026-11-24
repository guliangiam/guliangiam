#!/usr/bin/env bash
# Verify the MySQL mirror against a real MySQL 8.4+ database.
#
# Required:
#   MYSQL_DSN   go-sql-driver DSN, e.g. root:root@tcp(127.0.0.1:3306)/guliangiam_mysql_test?parseTime=true
#
# The database must first replay db/mysql/V1__guliangiam_schema_init.sql.
# This script is evidence tooling for OPEN-API-37; it does not claim full MySQL
# runtime support (AI Gateway/payments/audit/SSF wiring is still PostgreSQL-only).
set -euo pipefail

: "${MYSQL_DSN:?MYSQL_DSN is required (point it at a migrated MySQL 8.4+ database)}"

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"

echo "== MySQL identity repositories =="
go test -count=1 ./internal/infra/mysql/users/

echo "== MySQL organization repositories =="
go test -count=1 ./internal/infra/mysql/organizations/

echo "== MySQL Casbin policy store =="
go test -count=1 ./internal/infra/mysql/casbin/

echo "== MySQL authorization audit hash chain =="
go test -count=1 ./internal/infra/mysql/authorization_audit/

echo "== MySQL AI Gateway repositories =="
go test -count=1 ./internal/infra/mysql/aigateway/

echo "== MySQL payments channel config =="
go test -count=1 ./internal/infra/mysql/payments/

echo "== MySQL SSF receiver registry =="
go test -count=1 ./internal/infra/mysql/ssf/

echo "== AI Gateway MySQL completion =="
go test -count=1 -run TestGatewayService_Chat_MySQLUsage ./internal/app/aigateway/

echo "== SSF MySQL delivery =="
go test -count=1 -run TestHandlerDeliversThroughMySQLReceiverRegistry ./internal/outbound/ssf/

echo "== MySQL composition root =="
go test -count=1 -run TestMySQLRuntimeStarts ./internal/bootstrap/

echo "PASS: MySQL identity/organization/policy-store integration suites"
