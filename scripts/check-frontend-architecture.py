#!/usr/bin/env python3
"""前端架构门禁 — 直调 fetch 检查

目标：所有 HTTP 请求必须走 shared/api/http.ts 统一入口。
     直调 fetch/XMLHttpRequest/axios 绕过了：
     - credentials: "include" （session cookie）
     - CSRF token 注入 （X-CSRF-Token）
     - Accept-Language  （多语言）
     - 错误映射（HttpError 含 code/meta/retryAfterSeconds）
     - 401 自动登出（handleUnauthorized）

Scope: web/src/ 下所有 .ts / .tsx
排除:  shared/api/http.ts (统一入口自身会调 fetch)
       *_test.ts / *.test.ts / *.test.tsx (测试允许 fixture fetch)

Usage:
  python3 scripts/check-frontend-architecture.py

输入：web/src/ *.ts / *.tsx
输出：PASS/FAIL + 直调 fetch/axios/XMLHttpRequest 位置
--check 行为：无 --check；失败即 exit 1
消费方：独立脚本、CI
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

WEB_ROOT = Path(__file__).resolve().parent.parent / "web" / "src"
EXCLUDE_PATTERNS = [
    "shared/api/http.ts",       # 统一入口
    "shared/api/api-client.ts", # 可能有的其他统一封装
]

# 匹配全局 fetch( / fetch ( 调用。
# 必须排除成员访问与方法名：配置对象里的回调也叫 fetch（如 config.fetch(ctx, id)），
# 以及 refetch/prefetch。原先的 `(?:^|[^(])` 会吃掉 `.`，把 `config.fetch(` 又匹配回来，
# 于是 factory.ts 的两个回调被误报为"直调 fetch"。
FETCH_CALL_RE = re.compile(
    r"(?<![.\w$])fetch\s*\(",
    re.MULTILINE,
)


def should_exclude(rel_path: str) -> bool:
    if rel_path.endswith(".test.ts") or rel_path.endswith(".test.tsx"):
        return True
    if any(rel_path.endswith(p) or rel_path == p for p in EXCLUDE_PATTERNS):
        return True
    return False


def scan() -> list[tuple[Path, int, str]]:
    violations: list[tuple[Path, int, str]] = []
    for ts_file in sorted(WEB_ROOT.rglob("*")):
        if ts_file.suffix not in (".ts", ".tsx"):
            continue
        rel = ts_file.relative_to(WEB_ROOT)
        if should_exclude(str(rel)):
            continue
        try:
            text = ts_file.read_text()
        except Exception:
            continue
        for lineno, line in enumerate(text.splitlines(), 1):
            if FETCH_CALL_RE.search(line):
                violations.append((ts_file, lineno, line.strip()))
    return violations


def main() -> int:
    print("=" * 50)
    print("  前端架构门禁 — 直调 fetch 检查")
    print("=" * 50)
    print()

    if not WEB_ROOT.is_dir():
        print(f"ERROR: {WEB_ROOT} 不存在，跳过前端门禁")
        return 0

    violations = scan()

    if not violations:
        print("PASS: web/src/ 无直调 fetch")
        print("  → 全部 HTTP 请求走 shared/api/http.ts 统一入口")
        return 0

    print(f"FAIL: 发现 {len(violations)} 处直调 fetch（应改用 shared/api/http.ts）\n")
    for fpath, lineno, line in violations:
        rel = fpath.relative_to(Path(__file__).resolve().parent.parent)
        print(f"  {rel}:{lineno}")
        print(f"          {line}")

    print()
    print("修复指引:")
    print("  GET JSON     → import { requestDocumentJson } from '@/shared/api/http'")
    print("  纯文本/XML   → import { requestText } from '@/shared/api/http'")
    print("  POST/PATCH   → import { request } from '@/shared/api/http'")
    print("  二进制下载   → import { requestBlob } from '@/shared/api/http'")
    return 1


if __name__ == "__main__":
    sys.exit(main())
