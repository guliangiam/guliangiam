#!/usr/bin/env python3
"""Ensure the app shell and entry modules deep-import antd components.

Why this exists
---------------
The 1.5 MB antd bundle is grouped into a single vendor chunk. When a
statically reachable module (AppShell, providers, router helpers, main.tsx)
imports the antd barrel, the bundler cannot prove that only a few components
are needed and the whole antd graph - including Form, Table, DatePicker and
Upload used only by lazy CRUD pages - becomes part of the entry static module
graph. The first-screen budget gate then reports ~470 KB gzip of antd.

Deep imports (antd/es/button) keep each component module tree-shakable. Lazy
pages may keep using the barrel: their chunks are not on the first screen.
This gate covers the app shell/entry layer, where a barrel import is always a
first-screen regression.

Usage: python3 scripts/check-app-barrel-imports.py
Exit code 0 = no barrel import in app shell/entry; 1 = violations.
"""

from pathlib import Path
import sys

REPO_ROOT = Path(__file__).resolve().parent.parent
TARGET_DIRS = [REPO_ROOT / "web" / "src" / "app"]
TARGET_FILES = [REPO_ROOT / "web" / "src" / "main.tsx"]
BARREL_PATTERNS = ('from "antd"', "from 'antd'", 'import "antd"', "import 'antd'")


def candidate_files():
    files = []
    for target in TARGET_DIRS:
        if target.is_dir():
            files.extend(sorted(target.rglob("*.ts")))
            files.extend(sorted(target.rglob("*.tsx")))
    for target in TARGET_FILES:
        if target.is_file():
            files.append(target)
    return [f for f in files if not f.name.endswith((".test.ts", ".test.tsx", ".stories.tsx"))]


def main():
    violations = []
    scanned = 0
    for file in candidate_files():
        scanned += 1
        for lineno, line in enumerate(file.read_text(encoding="utf-8").splitlines(), 1):
            if "import type" in line:
                continue
            if any(pattern in line for pattern in BARREL_PATTERNS):
                rel = file.relative_to(REPO_ROOT)
                violations.append(f"{rel}:{lineno}: import from antd barrel (use antd/es/<component>)")
    if violations:
        print("FAIL: app shell/entry imports antd barrel (first-screen regression):")
        for violation in violations:
            print("  - " + violation)
        print("Use deep imports, e.g. import Button from antd/es/button")
        return 1
    print(f"PASS: {scanned} app shell/entry files deep-import antd components")
    return 0


if __name__ == "__main__":
    sys.exit(main())
