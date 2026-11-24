#!/usr/bin/env python3
"""docs 根目录布局门禁：根目录只允许 4 份正式入口文档。

过程报告、审计快照与一次性诊断不得写入 docs/ 根目录；过程证据写入
`.trae/documents/`（不参与 docs 门禁与追溯矩阵）。
消费方：scripts/verify-docs.sh；登记：docs/README.md §2.4。
"""

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DOCS = ROOT / "docs"
ALLOWED = {
    "README.md",
    "GuliangIAM-开发规范.md",
    "acceptance-criteria.md",
    "traceability-matrix.md",
}


def main() -> int:
    # 根目录只允许四份 Markdown 入口；非 Markdown（含 .DS_Store 等隐藏元数据）
    # 同样视为违规，避免垃圾文件绕过「只允许 4 份入口」的红线。
    files = sorted(p for p in DOCS.iterdir() if p.is_file())
    extra = [
        p.name
        for p in files
        if p.name not in ALLOWED or p.suffix.lower() != ".md"
    ]
    print("=== check-docs-root.py ===")
    print(f"根目录正式入口：{len(files) - len(extra)} 份，根目录违规文件：{len(extra)} 份")
    if extra:
        for name in extra:
            print(
                f"FAIL: docs/{name} 不得位于 docs 根目录；"
                "过程证据写入 .trae/documents/（docs/README.md §2.2），"
                "根目录只允许 4 份 Markdown 入口文件",
                file=sys.stderr,
            )
        return 1
    print("PASS: docs 根目录仅保留 4 份正式入口文件")
    return 0


if __name__ == "__main__":
    sys.exit(main())
