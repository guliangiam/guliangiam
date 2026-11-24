#!/usr/bin/env python3
"""V1 freeze manifest + release readiness checks (OPEN-API-57).

Usage:
  python3 scripts/release/check_release_readiness.py --write-manifest
  python3 scripts/release/check_release_readiness.py
  python3 scripts/release/check_release_readiness.py --strict-manual
"""
from __future__ import annotations

import argparse
import hashlib
import os
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
MANIFEST = ROOT / "deploy/release/v1-checksums.sha256"
V1_FILES = [
    "db/postgresql/V1__guliangiam_schema_init.sql",
    "internal/bootstrap/schema_sql/V1__guliangiam_schema_init.sql",
    "db/mysql/V1__guliangiam_schema_init.sql",
    "db/gaussdb/V1__guliangiam_schema_init.sql",
    "db/kingbasees/V1__guliangiam_schema_init.sql",
]
SNAPSHOT_DIR = ROOT / "contracts/schema/history/active/V1"
CONFIG_GLOBS = [".env", ".env.*", "deploy/**/*", "docker-compose*.yml", "docker-compose*.yaml", "charts/**/*", "k8s/**/*"]
DRIFT_RE = re.compile(r"ALLOW_CHECKSUM_DRIFT\s*[:=]\s*['\"]?(true|1|yes|on)", re.IGNORECASE)


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def snapshot_digest() -> str:
    digest = hashlib.sha256()
    for path in sorted(p for p in SNAPSHOT_DIR.rglob("*") if p.is_file()):
        rel = path.relative_to(ROOT).as_posix()
        digest.update(rel.encode("utf-8"))
        digest.update(b"\0")
        digest.update(sha256(path).encode("ascii"))
        digest.update(b"\n")
    return digest.hexdigest()


def manifest_lines() -> list[str]:
    lines = [f"{sha256(ROOT / rel)}  {rel}" for rel in V1_FILES]
    lines.append(f"{snapshot_digest()}  contracts/schema/history/active/V1/")
    return lines


def write_manifest() -> int:
    MANIFEST.parent.mkdir(parents=True, exist_ok=True)
    MANIFEST.write_text("\n".join(manifest_lines()) + "\n", encoding="utf-8")
    print(f"wrote {MANIFEST.relative_to(ROOT)}")
    return 0


def verify_manifest() -> list[str]:
    if not MANIFEST.exists():
        return [f"missing manifest: {MANIFEST.relative_to(ROOT)}"]
    expected = {line.split("  ", 1)[1]: line.split("  ", 1)[0] for line in MANIFEST.read_text(encoding="utf-8").splitlines() if line.strip()}
    actual = {rel: sha256(ROOT / rel) for rel in V1_FILES}
    actual["contracts/schema/history/active/V1/"] = snapshot_digest()
    problems = []
    for rel, digest in actual.items():
        if rel not in expected:
            problems.append(f"manifest missing entry: {rel}")
        elif expected[rel] != digest:
            problems.append(f"checksum drift: {rel} (manifest {expected[rel]}, actual {digest})")
    for rel in expected:
        if rel not in actual:
            problems.append(f"manifest has stale entry: {rel}")
    return problems


def scan_drift_assignments() -> list[str]:
    hits = []
    for pattern in CONFIG_GLOBS:
        for path in ROOT.glob(pattern):
            if not path.is_file() or ".git" in path.parts:
                continue
            try:
                text = path.read_text(encoding="utf-8", errors="ignore")
            except OSError:
                continue
            for lineno, line in enumerate(text.splitlines(), 1):
                if DRIFT_RE.search(line):
                    hits.append(f"{path.relative_to(ROOT)}:{lineno}: {line.strip()}")
    return hits


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--write-manifest", action="store_true")
    parser.add_argument("--strict-manual", action="store_true", help="require baseline snapshot and rollback evidence paths")
    args = parser.parse_args()
    if args.write_manifest:
        return write_manifest()

    problems = verify_manifest()
    drift = scan_drift_assignments()
    problems.extend(f"forbidden deployment drift toggle: {hit}" for hit in drift)

    if args.strict_manual:
        for env_name in ("RELEASE_BASELINE_SNAPSHOT", "RELEASE_ROLLBACK_EVIDENCE"):
            value = os.environ.get(env_name, "").strip()
            if not value:
                problems.append(f"manual release artifact missing: set {env_name}")
            elif not Path(value).is_file():
                problems.append(f"manual release artifact not found: {env_name}={value}")

    if problems:
        print("FAIL: release readiness")
        for problem in problems:
            print(f"  - {problem}")
        return 1
    if not args.strict_manual:
        print("WARN: manual baseline snapshot / rollback evidence not checked (use --strict-manual)")
    print("PASS: V1 checksum manifest and deployment drift scan")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
