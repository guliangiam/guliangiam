#!/usr/bin/env bash
# sync-seed-mirror.sh — 把 db/seeds 的数据链源文件同步到 go:embed 副本。
#
# 唯一事实源：db/seeds/*.sql（人工编写，ADR-022 决策 6）。
# 镜像目录：internal/bootstrap/seed_sql/（go:embed 消费，禁止手改）。
#
# 用法：bash scripts/sync-seed-mirror.sh
# 出口：复制后逐文件 cmp；任一不一致 exit 1。
set -euo pipefail

repo_root=$(cd "${1:-$(dirname "${BASH_SOURCE[0]}")/..}" && pwd)
cd "$repo_root"

src_dir="db/seeds"
dst_dir="internal/bootstrap/seed_sql"

[ -d "$src_dir" ] || { echo "missing $src_dir" >&2; exit 1; }
mkdir -p "$dst_dir"

for src in "$src_dir"/V*.sql; do
  [ -e "$src" ] || { echo "no seed files matched in $src_dir" >&2; exit 1; }
  name=$(basename "$src")
  cp "$src" "$dst_dir/$name"
  echo "synced $name"
done

# Max seed version in db/seeds (source of the authored data chain).
src_max_version=0
for src in "$src_dir"/V*.sql; do
  v=$(basename "$src" | sed 's/V\([0-9]*\)__.*/\1/')
  [ "$v" -gt "$src_max_version" ] && src_max_version=$v
done

for dst in "$dst_dir"/V*.sql; do
  name=$(basename "$dst")
  if [ ! -f "$src_dir/$name" ]; then
    # Data-enrich files (e.g. V2__admin_profile_enrich.sql) are declared
    # directly in migrate.go and have no db/seeds counterpart — accepted only
    # when their version strictly exceeds every seed version, same policy as
    # check-migrations.sh [5/7]. Anything older is orphan mirror drift.
    v=$(basename "$dst" | sed 's/V\([0-9]*\)__.*/\1/')
    if [ "$v" -gt "$src_max_version" ]; then
      echo "skipped $name (data-enrich, no seed source)"
      continue
    fi
    echo "VIOLATION: $dst has no source in $src_dir" >&2
    exit 1
  fi
  if ! cmp -s "$src_dir/$name" "$dst"; then
    echo "VIOLATION: $name differs between $src_dir and $dst_dir" >&2
    exit 1
  fi
done

echo "OK: seed mirror is byte-identical"
