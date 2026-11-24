#!/usr/bin/env python3
"""Rehearse a V1 backup/restore drill against an isolated drill database.

Required env:
  REHEARSE_SOURCE_DSN       source database DSN (read-only)
  REHEARSE_TARGET_DB        target database name; MUST contain "_drill"
  REHEARSE_TARGET_DSN       target database DSN (same name as TARGET_DB)
  REHEARSE_TARGET_ADMIN_DSN admin DSN (e.g. .../postgres) used to drop/create
Optional:
  REHEARSE_TARGET_OWNER     owner role for CREATE DATABASE
  REHEARSE_EVIDENCE         write a Markdown drill record to this path
  REHEARSE_KEEP=1           keep target database and dump
"""
from __future__ import annotations

import datetime as dt
import hashlib
import os
import subprocess
import tempfile
from pathlib import Path


def run(args: list[str], *, input_text: str | None = None) -> str:
    result = subprocess.run(args, input=input_text, text=True, capture_output=True)
    if result.returncode != 0:
        raise RuntimeError(f"command failed ({' '.join(args)}): {result.stderr.strip()}")
    return result.stdout


def psql(dsn: str, sql: str) -> str:
    return run(["psql", dsn, "-v", "ON_ERROR_STOP=1", "-tA", "-c", sql]).strip()


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def table_count(dsn: str) -> int:
    return int(psql(dsn, "SELECT count(*) FROM information_schema.tables WHERE table_schema='guliangiam'"))


def main() -> int:
    source = os.environ.get("REHEARSE_SOURCE_DSN", "").strip()
    target_db = os.environ.get("REHEARSE_TARGET_DB", "").strip()
    target = os.environ.get("REHEARSE_TARGET_DSN", "").strip()
    admin = os.environ.get("REHEARSE_TARGET_ADMIN_DSN", "").strip()
    owner = os.environ.get("REHEARSE_TARGET_OWNER", "").strip()
    evidence = os.environ.get("REHEARSE_EVIDENCE", "").strip()
    keep = os.environ.get("REHEARSE_KEEP", "").strip() == "1"
    for name, value in (("REHEARSE_SOURCE_DSN", source), ("REHEARSE_TARGET_DB", target_db), ("REHEARSE_TARGET_DSN", target), ("REHEARSE_TARGET_ADMIN_DSN", admin)):
        if not value:
            raise SystemExit(f"{name} is required")
    if "_drill" not in target_db:
        raise SystemExit(f"refusing to touch non-drill database: {target_db}")

    started = dt.datetime.now(dt.timezone.utc)
    dump_path = Path(tempfile.mkdtemp(prefix="v1-drill-")) / "guliangiam.dump"
    source_tables = table_count(source)
    run(["pg_dump", source, "-Fc", "-f", str(dump_path)])
    dump_hash = sha256_file(dump_path)

    owner_clause = f" OWNER {owner}" if owner else ""
    psql(admin, f'DROP DATABASE IF EXISTS "{target_db}"')
    psql(admin, f'CREATE DATABASE "{target_db}"{owner_clause}')
    try:
        run(["pg_restore", "--dbname", target, "--no-owner", "--no-privileges", str(dump_path)])
        target_tables = table_count(target)
        migration_rows = psql(target, "SELECT count(*) FROM guliangiam.schema_migrations")
        if target_tables != source_tables:
            raise RuntimeError(f"table count mismatch: source={source_tables}, target={target_tables}")
        finished = dt.datetime.now(dt.timezone.utc)
        record = f"""# V1 backup/restore drill

- Started: {started.isoformat()}
- Finished: {finished.isoformat()}
- Source DSN: {source}
- Target drill DB: {target_db}
- Dump SHA-256: {dump_hash}
- Source tables: {source_tables}
- Restored tables: {target_tables}
- schema_migrations rows: {migration_rows}
- Result: **PASS**
"""
        if evidence:
            evidence_path = Path(evidence)
            evidence_path.parent.mkdir(parents=True, exist_ok=True)
            evidence_path.write_text(record, encoding="utf-8")
            print(f"evidence written: {evidence_path}")
        print(f"PASS: restored {target_tables} tables from V1 dump")
        return 0
    finally:
        if not keep:
            psql(admin, f'DROP DATABASE IF EXISTS "{target_db}"')
            try:
                dump_path.unlink()
            except OSError:
                pass


if __name__ == "__main__":
    raise SystemExit(main())
