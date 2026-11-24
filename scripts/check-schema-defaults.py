#!/usr/bin/env python3
"""check-schema-defaults.py — 声明式 Schema 默认值的方言可移植性门禁

背景：MySQL 严格模式禁止 TEXT/BLOB/JSON 携带 DEFAULT，因此声明式 Schema 里
逻辑类型为 TEXT 的列一旦声明默认值，该默认值在 MySQL 产物中不可表达。此前
生成器对此「静默丢弃」：同一份声明式 Schema 在 PostgreSQL 与 MySQL 上给出不同
契约，而没有任何门禁能发现。

规则：
  1. LONGTEXT 是显式的「无上界」类型，**禁止**携带默认值（硬失败）。
  2. TEXT 携带默认值属于存量债务，必须已在 scripts/text-default-baseline.txt
     登记；新增条目硬失败（只减不增）。收敛方向：有界且需要默认值的列改用
     VARCHAR(n)，真正无上界的列不得携带默认值。

用法：python3 scripts/check-schema-defaults.py
退出码：0 = 合规；1 = 新增债务或 LONGTEXT 携带默认值。

输入：contracts/schema/*.yaml、scripts/text-default-baseline.txt
输出：PASS/FAIL + 新增 LONGTEXT 默认值 / 超出基线 TEXT 默认值
--check 行为：无 --check；失败即 exit 1
消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
"""
from __future__ import annotations

import glob
import sys
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parent.parent
SCHEMA_DIR = ROOT / "contracts" / "schema"
BASELINE = ROOT / "scripts" / "text-default-baseline.txt"


def load_baseline() -> set[str]:
    if not BASELINE.exists():
        return set()
    entries = set()
    for line in BASELINE.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if line and not line.startswith("#"):
            entries.add(line)
    return entries


def scan() -> tuple[list[str], list[str]]:
    """Return (text_defaults, longtext_defaults) as table.column keys."""
    text_defaults: list[str] = []
    longtext_defaults: list[str] = []
    for fp in sorted(glob.glob(str(SCHEMA_DIR / "*.yaml"))):
        with open(fp, encoding="utf-8") as fh:
            for doc in yaml.safe_load_all(fh):
                if not doc or not doc.get("table_name"):
                    continue
                table = doc["table_name"].split(".")[-1]
                for col in doc.get("columns", []):
                    if col.get("default") is None:
                        continue
                    key = f"{table}.{col['name']}"
                    if col.get("type") == "LONGTEXT":
                        longtext_defaults.append(key)
                    elif col.get("type") == "TEXT":
                        text_defaults.append(key)
    return text_defaults, longtext_defaults


def main() -> int:
    text_defaults, longtext_defaults = scan()
    baseline = load_baseline()

    print("=== check-schema-defaults.py ===")
    failures = 0

    if longtext_defaults:
        failures += 1
        print(f"FAIL: LONGTEXT 列不得携带默认值（MySQL 无法表达），共 {len(longtext_defaults)} 处：")
        for key in longtext_defaults[:10]:
            print(f"  - {key}")

    new_debt = sorted(set(text_defaults) - baseline)
    if new_debt:
        failures += 1
        print(f"FAIL: 新增 TEXT 默认值债务 {len(new_debt)} 处（须改用 VARCHAR(n) 或去掉默认值）：")
        for key in new_debt[:15]:
            print(f"  - {key}")

    retired = sorted(baseline - set(text_defaults))
    if retired:
        print(f"NOTE: 存量已减少 {len(retired)} 处，请同步下调 scripts/text-default-baseline.txt 的计数：")
        for key in retired[:10]:
            print(f"  - {key}")

    print(f"存量：TEXT 默认值 {len(text_defaults)} 处 / 基线 {len(baseline)} 处；LONGTEXT 默认值 0 处为合规")
    if failures:
        print(f"FAIL: {failures} 项")
        return 1
    print("PASS: 无新增默认值债务，LONGTEXT 未携带默认值")
    return 0


if __name__ == "__main__":
    sys.exit(main())
