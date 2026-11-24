#!/usr/bin/env python3
"""check-redundant-index.py — 冗余索引门禁（D9 索引经济性）

输入：contracts/schema/*.yaml（声明式 Schema）
消费方：verify-docs.sh（RUN_CODE_CHECKS=1）

判据（D9）：普通（非部分、非唯一）索引的列，若被某个唯一约束/唯一索引的**最左前缀**
完全覆盖（即索引列 == 唯一列序列的前 k 列），则该普通索引冗余——唯一约束已隐式建立了
等价索引，再建一份只增加写放大与存储。

排除：
  * 带 predicate 的部分索引（语义不同，不算冗余）；
  * 显式唯一索引本身。

输出：PASS/FAIL + 违规项。
--check 行为：无 --check；失败即 exit 1。
"""
from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / "scripts"))

from schema_common import load_tables  # noqa: E402


def main() -> int:
    tables = load_tables()
    problems: list[str] = []

    for t in tables:
        name = t.get("table_name", "?")
        constraints = t.get("constraints", []) or []
        indexes = t.get("indexes", []) or []

        # 唯一约束的最左前缀集合（含自身全列）
        unique_prefixes: set[tuple[str, ...]] = set()
        for c in constraints:
            if c.get("type") == "unique" and c.get("columns"):
                cols = tuple(c["columns"])
                for k in range(1, len(cols) + 1):
                    unique_prefixes.add(cols[:k])

        for idx in indexes:
            if not isinstance(idx, dict):
                continue
            if idx.get("predicate"):
                continue  # 部分索引语义不同
            cols = idx.get("columns")
            if not cols:
                continue
            if tuple(cols) in unique_prefixes:
                # 找到覆盖它的唯一约束名，便于出口
                covering = [
                    c["name"]
                    for c in constraints
                    if c.get("type") == "unique"
                    and c.get("columns")
                    and tuple(c["columns"][: len(cols)]) == tuple(cols)
                ]
                problems.append(
                    f"{name}: 索引 {idx.get('name')} 列 {cols} 被唯一约束 {covering} 最左前缀覆盖（冗余）"
                )

    if problems:
        print(f"FAIL: 发现 {len(problems)} 处冗余索引（D9）")
        for p in problems:
            print(f"  - {p}")
        return 1

    print(f"PASS: 无冗余索引（D9；共扫描 {len(tables)} 张表）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
