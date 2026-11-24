#!/usr/bin/env python3
"""AC ↔ 追溯矩阵双向登记完备性门禁

输入：docs/acceptance-criteria.md、docs/traceability-matrix.md
输出：PASS/FAIL + 未登记/未定义 AC 列表
--check 行为：无 --check 参数；有缺陷即 exit 1
消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI"""

import re, sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

def extract_ac_ids(text):
    """提取 AC-* 编号（支持数字前缀与多段族，如 AC-B2B-001、AC-DATA-001）"""
    return set(re.findall(r'\b(AC-[A-Z0-9]+(?:-[A-Z0-9]+)*-\d+)\b', text))

def main():
    ac_file = ROOT / "docs/acceptance-criteria.md"
    trace_file = ROOT / "docs/traceability-matrix.md"

    missing = [str(f.relative_to(ROOT)) for f in [ac_file, trace_file] if not f.exists()]
    if missing:
        print(
            "ERROR: 必需输入缺失: " + ", ".join(missing)
            + "——输入缺失说明仓库结构被破坏，本门禁未执行任何校验，不能当作通过",
            file=sys.stderr,
        )
        sys.exit(1)
    
    ac_text = ac_file.read_text()
    trace_text = trace_file.read_text()
    
    ac_in_file = extract_ac_ids(ac_text)
    ac_in_trace = extract_ac_ids(trace_text)
    
    missing_from_trace = ac_in_file - ac_in_trace
    missing_from_ac = ac_in_trace - ac_in_file
    
    print(f"=== check-ac-traceability.py ===")
    print(f"AC 文件: {len(ac_in_file)} 个 AC 编号")
    print(f"追溯矩阵: {len(ac_in_trace)} 个 AC 编号")
    
    errors = 0
    if missing_from_trace:
        print(f"FAIL: {len(missing_from_trace)} 个 AC 未在追溯矩阵中登记: {sorted(missing_from_trace)}")
        errors += 1
    if missing_from_ac:
        print(f"WARN: {len(missing_from_ac)} 个追溯矩阵 AC 未在 AC 文件中定义: {sorted(missing_from_ac)}")
    
    if errors == 0:
        print("PASS: 双向追溯完备")
        sys.exit(0)
    sys.exit(1)

if __name__ == "__main__":
    sys.exit(main())
