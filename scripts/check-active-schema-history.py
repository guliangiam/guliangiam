#!/usr/bin/env python3
"""Validate that frozen active schema snapshots form the runtime migration chain.

输入：contracts/schema/history/active/V{N}（最新活动快照）、contracts/schema/*.yaml
输出：PASS/FAIL + 具体差异描述
--check 行为：无 --check；失败即 exit 1
消费方：独立脚本、CI
"""
from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
ACTIVE_HISTORY = ROOT / "contracts/schema/history/active"
SCHEMA_DIR = ROOT / "contracts" / "schema"


def check_active_history(root: Path = ROOT) -> list[str]:
    active = root / "contracts/schema/history/active"
    schema_dir = root / "contracts" / "schema"
    current_files = {path.name: path.read_bytes() for path in schema_dir.glob("*.yaml")}
    errors: list[str] = []

    if not current_files:
        return ["no current contracts/schema/*.yaml files found"]
    if not (active / "V1").is_dir():
        return ["active V1 baseline is missing; run gen_schema.py --initialize-active-baseline"]

    snapshots: list[tuple[int, Path]] = []
    for path in active.iterdir():
        if not path.is_dir():
            errors.append(f"unexpected file in active history: {path.relative_to(root)}")
            continue
        match = re.fullmatch(r"V([1-9][0-9]*)", path.name)
        if not match:
            errors.append(f"invalid active snapshot directory: {path.relative_to(root)}")
            continue
        snapshots.append((int(match.group(1)), path))
    snapshots.sort()

    versions = [version for version, _ in snapshots]
    if versions != list(range(1, len(versions) + 1)):
        errors.append(f"active snapshots are not continuous from V1: {versions}")
        return errors

    latest_version, latest_path = snapshots[-1]
    latest_files = {path.name: path.read_bytes() for path in latest_path.glob("*.yaml")}
    if latest_files != current_files:
        missing = sorted(current_files.keys() - latest_files.keys())
        extra = sorted(latest_files.keys() - current_files.keys())
        changed = sorted(
            name for name in current_files.keys() & latest_files.keys()
            if current_files[name] != latest_files[name]
        )
        errors.append(
            f"active V{latest_version} snapshot differs from contracts/schema "
            f"(missing={missing}, extra={extra}, changed={changed}); "
            "generate and freeze the next migration version"
        )

    for version, _ in snapshots[1:]:
        result = subprocess.run(
            [
                sys.executable,
                str(root / "scripts/gen_schema.py"),
                "--dialect",
                "postgresql",
                "--version",
                str(version),
                "--check",
            ],
            cwd=root,
            capture_output=True,
            text=True,
            check=False,
        )
        if result.returncode != 0:
            details = (result.stdout + result.stderr).strip()
            errors.append(f"active V{version} migration artifacts are invalid:\n{details}")

    return errors


def main() -> int:
    errors = check_active_history()
    if errors:
        for error in errors:
            print(f"FAIL: {error}", file=sys.stderr)
        return 1
    print("PASS: active schema snapshots match current declarations and migration artifacts")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
