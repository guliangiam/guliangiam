#!/usr/bin/env python3
"""check-write-invalidation.py — 写路径成功后未失效查询缓存的静态门禁

Why this exists
---------------
D3b1–D3b9 清扫出一类真实缺陷：列表页数据来自 TanStack Query 缓存，写路径
（创建/删除）成功后只 `navigate()` 回列表，却没有失效对应 queryKey，导致：

  · 删除后返回列表仍显示已删除行（最长 30 分钟，静态层）；
  · 创建后返回列表看不到新行（最长 30 分钟/5 分钟）；
  · 失效键写成兄弟键（`["users","auth"]`）而不是列表前缀（`["users","paginated",…]`）。

tsc、oxlint 与 mock 了 HTTP 的组件测试都看不见：请求成功、组件无错，只是缓存未更新。

判定规则
  R1. 扫描 `web/src/pages/**/*.{ts,tsx}`（跳过 `*.test.*`），提取
      `const deleteMutation = useMutation({ ... })` 块（按花括号配对）。
      `onSuccess` 到下一个 `onError`（或块尾）的片段必须包含以下任一失效信号：
      `invalidateQueries` / `invalidateDomains` / `refetch(` / `setQueryData` /
      `removeQueries` / `.clear(`；或调用**同文件**的命名 helper（只展开一层），
      helper 体内含上述任一信号。
      二跳及以上不展开；跨文件 helper 不解析，仍按“未失效”处理（遗留项）。
  R2. 扫描 `web/src/pages/**/*Create*.tsx` 与 `use*CreatePageController*.{ts,tsx}`。
      仅当文件同时含 `onSuccess` 与 `navigate(` 时检查；文件必须含上述任一失效信号。
      重定向型创建页（`<Navigate>`）没有 `onSuccess`/`navigate(`，自然豁免；
      委托控制器的页面（如 `UserCreatePage`）由 `useUserCreatePageController` 命中。

Usage: python3 scripts/check-write-invalidation.py [--pages DIR]
  `--pages` 只用于自检/临时样例；默认扫描仓库 `web/src/pages`。
Exit code 0 = PASS；1 = 存在写路径未失效查询缓存；2 = 参数错误。

消费方：scripts/verify-docs.sh（RUN_CODE_CHECKS=1）、CI。
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
PAGES = REPO / "web" / "src" / "pages"

INVALIDATORS = (
    "invalidateQueries",
    "invalidateDomains",
    "refetch(",
    "setQueryData",
    "removeQueries",
    ".clear(",
)

DELETE_RE = re.compile(r"const\s+deleteMutation\s*=\s*useMutation\(\{")
ON_SUCCESS_RE = re.compile(r"onSuccess")
ON_ERROR_RE = re.compile(r"onError")
# 同文件命名 helper：函数声明或 `const/let/var name = (...) =>`。
FUNCTION_DEF_RE = re.compile(
    r"(?m)^[ \t]*(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*\([^)]*\)\s*{"
)
ARROW_DEF_RE = re.compile(
    r"(?m)^[ \t]*(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s*)?"
    r"(?:\([^)]*\)|\w+)\s*(?::[^=;]+)?=>"
)
CALL_RE = re.compile(r"\b(\w+)\s*\(")


def iter_source_files(root: Path | None = None) -> list[Path]:
    root = root or PAGES
    files = []
    for path in sorted(root.rglob("*")):
        if path.suffix not in {".ts", ".tsx"} or ".test." in path.name:
            continue
        files.append(path)
    return files


def matching_brace(text: str, start: int) -> int:
    """Return the index of the `}` matching the `{` at start."""
    depth = 0
    for index in range(start, len(text)):
        char = text[index]
        if char == "{":
            depth += 1
        elif char == "}":
            depth -= 1
            if depth == 0:
                return index
    return len(text)


def line_number(text: str, index: int) -> int:
    return text.count("\n", 0, index) + 1


def has_invalidator(text: str) -> bool:
    return any(token in text for token in INVALIDATORS)


def extract_helpers(text: str) -> dict[str, str]:
    """Collect same-file named helper bodies: function declarations and arrow functions.

    Only the body text is returned, so the caller can check for a literal
    invalidation signal without expanding any further calls (one hop).
    """
    helpers: dict[str, str] = {}
    for match in FUNCTION_DEF_RE.finditer(text):
        open_brace = text.find("{", match.end() - 1)
        if open_brace == -1:
            continue
        helpers.setdefault(match.group(1), text[open_brace : matching_brace(text, open_brace) + 1])

    for match in ARROW_DEF_RE.finditer(text):
        body_start = match.end()
        remainder = text[body_start:]
        stripped = remainder.lstrip()
        if stripped.startswith("{"):
            open_brace = text.find("{", body_start)
            body = text[open_brace : matching_brace(text, open_brace) + 1]
        else:
            # Expression-bodied arrow: stop at the end of the statement.
            semicolon = text.find(";", body_start)
            newline = text.find("\n", body_start)
            end = min(value for value in (semicolon, newline, len(text)) if value != -1)
            body = text[body_start:end]
        helpers.setdefault(match.group(1), body)
    return helpers


def segment_has_invalidation(segment: str, helpers: dict[str, str]) -> bool:
    """True when the segment invalidates directly or via a one-hop same-file helper.

    A helper is only accepted when its own body contains a literal signal; calls
    inside that helper are never followed, so a two-hop chain stays red.
    """
    if has_invalidator(segment):
        return True
    for match in CALL_RE.finditer(segment):
        helper = helpers.get(match.group(1))
        if helper is not None and has_invalidator(helper):
            return True
    return False


def display_path(path: Path, root: Path) -> Path:
    try:
        return path.relative_to(REPO)
    except ValueError:
        return path.relative_to(root)


def delete_violations(root: Path | None = None) -> list[tuple[Path, int, str]]:
    root = root or PAGES
    violations = []
    for path in iter_source_files(root):
        text = path.read_text()
        helpers = extract_helpers(text)
        for match in DELETE_RE.finditer(text):
            open_brace = text.find("{", match.end() - 1)
            if open_brace == -1:
                continue
            block = text[open_brace : matching_brace(text, open_brace) + 1]
            on_success = ON_SUCCESS_RE.search(block)
            if not on_success:
                continue
            on_error = ON_ERROR_RE.search(block, on_success.end())
            segment = block[on_success.end() : on_error.start() if on_error else len(block)]
            if not segment_has_invalidation(segment, helpers):
                violations.append(
                    (display_path(path, root), line_number(text, match.start()), "删除 onSuccess 缺少缓存失效")
                )
    return violations


def create_violations(root: Path | None = None) -> list[tuple[Path, int, str]]:
    root = root or PAGES
    violations = []
    for path in iter_source_files(root):
        name = path.name
        if "Create" not in name:
            continue
        text = path.read_text()
        if "onSuccess" not in text or "navigate(" not in text:
            continue
        if not has_invalidator(text):
            violations.append(
                (
                    display_path(path, root),
                    line_number(text, text.find("onSuccess")),
                    "创建 onSuccess 缺少缓存失效",
                )
            )
    return violations


def main(argv: list[str] | None = None) -> int:
    argv = sys.argv[1:] if argv is None else argv
    pages = PAGES
    if argv:
        if len(argv) != 2 or argv[0] != "--pages":
            print("usage: check-write-invalidation.py [--pages DIR]")
            return 2
        pages = Path(argv[1]).resolve()
        if not pages.is_dir():
            print(f"ERROR: pages directory not found: {pages}")
            return 2

    if pages == PAGES:
        print("== check-write-invalidation.py ==")
    else:
        print(f"== check-write-invalidation.py (pages={pages}) ==")

    violations = delete_violations(pages) + create_violations(pages)
    if violations:
        for path, line, message in violations:
            print(f"FAIL: {path}:{line} {message}")
        print(f"FAIL: {len(violations)} 处写路径未失效查询缓存")
        return 1
    print("PASS: 写路径失效门禁通过（删除/创建 onSuccess 均含缓存失效信号）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
