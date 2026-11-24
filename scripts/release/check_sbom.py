#!/usr/bin/env python3
"""Validate the SPDX JSON SBOM that the release pipeline archives.

AC-SEC-006 requires the SBOM product to be present, parseable and structurally
complete; a missing/corrupt artifact must fail the release job.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import pathlib
import sys


def validate(path: pathlib.Path) -> list[str]:
    problems: list[str] = []
    try:
        document = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        return [f"cannot read SBOM JSON: {exc}"]
    if not isinstance(document, dict):
        return ["SBOM root must be a JSON object"]
    if not str(document.get("spdxVersion", "")).startswith("SPDX-2."):
        problems.append("spdxVersion must be SPDX-2.x")
    for key in ("SPDXID", "name"):
        if not str(document.get(key, "")).strip():
            problems.append(f"missing {key}")
    packages = document.get("packages")
    if not isinstance(packages, list) or not packages:
        problems.append("packages must be a non-empty array")
    else:
        for index, package in enumerate(packages):
            if not isinstance(package, dict):
                problems.append(f"packages[{index}] must be an object")
                continue
            for key in ("name", "SPDXID"):
                if not str(package.get(key, "")).strip():
                    problems.append(f"packages[{index}] missing {key}")
    checksum = path.with_suffix(path.suffix + ".sha256")
    if checksum.exists():
        expected = checksum.read_text(encoding="utf-8").split()[0].strip()
        actual = hashlib.sha256(path.read_bytes()).hexdigest()
        if expected != actual:
            problems.append("sha256 checksum mismatch")
    return problems


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("sbom", type=pathlib.Path)
    args = parser.parse_args()
    problems = validate(args.sbom)
    if problems:
        for problem in problems:
            print(f"FAIL: {problem}", file=sys.stderr)
        return 1
    print(f"PASS: SBOM structurally valid: {args.sbom}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
