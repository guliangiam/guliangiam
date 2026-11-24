#!/usr/bin/env python3
"""paths.ts 一致性门禁 — API 路径字面量检查

目标：所有 API URL path 必须从 shared/api/paths.ts 获取常量，
     禁止在生产代码里硬编码 `/api/v1/...` 字符串。
     理由：paths.ts 是唯一事实源 — 变更 endpoint 时改一处即可，
     避免多处同步遗漏或拼写错误。

Scope: web/src/ 下所有 .ts / .tsx
排除:  shared/api/paths.ts (常量定义自身)
       *.test.ts / *.test.tsx (测试允许 fixture URL)
       generated/ / e2e/
       app/router/ (React Router lazy() 动态 import 路径不是 API 调用)

注释（//、/* */、/** */）内的 `/api/v1/...` 不参与匹配：
说明性文档常引用旧端点，它们不是可执行的请求路径。

Usage:
  python3 scripts/check-paths-consistency.py

输入：web/src/ *.ts / *.tsx
输出：PASS/FAIL + 硬编码路径位置
Exit code：零硬编码 → 0；有硬编码 → 1
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
WEB_ROOT = ROOT / "web" / "src"

# 匹配字符串/模板字符串字面量中的 /api/v1/... 字面量。
# 同时覆盖：
#   - 单引号/双引号: "/api/v1/users"
#   - 反引号模板:     `/api/v1/users/${id}`  (完整路径片段)
# 匹配前由 _mask_comments 把注释内容替换为空格，因此该正则只作用于代码区。
# 模板字符串只抓固定前缀部分（到 ${} 或 } 为止）。
API_PATH_RE = re.compile(
    r"""
    ['"`]                          # 开头引号
    (/api/v1/                      # API 路径前缀
      [^'"`\s]*)                   # 后续片段（停在闭合引号/反引号内的动态插值）
    ['"`]                          # 结尾引号
    """,
    re.VERBOSE,
)

EXCLUDE_DIRS = {
    "generated",
    "e2e",
    "app",  # React Router import 路径，不是 API 调用
}
EXCLUDE_FILES = {
    "paths.ts",
}

# 正则字面量起始判定：上一个有效记号是这些关键字或标点时，'/' 视为正则开头。
_REGEX_PRECEDING_KEYWORDS = frozenset({
    "await", "case", "delete", "do", "else", "in", "instanceof", "new",
    "of", "return", "throw", "typeof", "void", "yield",
})
_REGEX_PRECEDING_CHARS = frozenset("([{,;:=!&|?+-*%<>~^")


def should_exclude(rel_parts: list[str], filename: str) -> bool:
    if filename in EXCLUDE_FILES:
        return True
    for part in rel_parts:
        if part in EXCLUDE_DIRS:
            return True
    if filename.endswith(".test.ts") or filename.endswith(".test.tsx"):
        return True
    return False


def _regex_can_start(chars: list[str], index: int) -> bool:
    """判断 index 处的 '/' 是否更像正则字面量开头（否则按除法处理）。"""
    j = index - 1
    while j >= 0 and chars[j].isspace():
        j -= 1
    if j < 0:
        return True
    ch = chars[j]
    if ch.isalnum() or ch in "_$":
        k = j
        while k >= 0 and (chars[k].isalnum() or chars[k] in "_$"):
            k -= 1
        return "".join(chars[k + 1 : j + 1]) in _REGEX_PRECEDING_KEYWORDS
    if ch in "+-":
        # i++ / 2、i-- / 2 是除法；同号连写时 '/' 不是正则开头
        k = j - 1
        while k >= 0 and chars[k].isspace():
            k -= 1
        if k >= 0 and chars[k] == ch:
            return False
    return ch in _REGEX_PRECEDING_CHARS


def _mask_comments(text: str) -> str:
    """把注释内容替换为空格（保留换行），字符串/模板/正则原样保留。

    够用的 JS/TS 词法状态机，零第三方依赖。已知边界（本门禁可接受）：
      - '/' 是除法还是正则开头用前导记号启发式判定；')'、']'、'}' 之后按除法处理
      - 单/双引号字符串遇裸换行即中止（JS 本身非法），避免 JSX 撇号跨行吞代码
      - 模板字符串的 ${...} 按代码继续扫描；跨行模板仍按行匹配（与旧行为一致）
    """
    chars = list(text)
    n = len(text)
    # 栈帧：[kind, ...]；code 帧第二项为模板插值的花括号深度。
    stack: list[list] = [["code", 0]]
    prev_sig = ""
    i = 0
    while i < n:
        ch = text[i]
        frame = stack[-1]
        kind = frame[0]

        if kind == "line_comment":
            if ch == "\n":
                stack.pop()
            else:
                chars[i] = " "
            i += 1
            continue

        if kind == "block_comment":
            if text.startswith("*/", i):
                chars[i] = chars[i + 1] = " "
                stack.pop()
                i += 2
                continue
            if ch != "\n":
                chars[i] = " "
            i += 1
            continue

        if kind == "string":
            quote = frame[1]
            if ch == "\\" and i + 1 < n:
                i += 2
                continue
            if ch == quote:
                stack.pop()
                prev_sig = quote
                i += 1
                continue
            if ch == "\n":
                # 单/双引号字符串不允许裸换行；中止以免 JSX 撇号吞掉后续代码
                stack.pop()
                prev_sig = quote
                i += 1
                continue
            i += 1
            continue

        if kind == "template":
            if ch == "\\" and i + 1 < n:
                i += 2
                continue
            if ch == "`":
                stack.pop()
                prev_sig = "`"
                i += 1
                continue
            if text.startswith("${", i):
                stack.append(["code", 1])
                prev_sig = "{"
                i += 2
                continue
            i += 1
            continue

        if kind == "regex":
            if ch == "\\" and i + 1 < n:
                i += 2
                continue
            if ch == "[":
                frame[1] = True
                i += 1
                continue
            if ch == "]":
                frame[1] = False
                i += 1
                continue
            if ch == "/" and not frame[1]:
                stack.pop()
                prev_sig = "/"
                i += 1
                continue
            if ch == "\n":
                # 未闭合（多为把除法误判为正则）；放弃正则帧，按代码继续
                stack.pop()
                continue
            i += 1
            continue

        # ---- code ----
        if ch == "/" and i + 1 < n and text[i + 1] == "/":
            chars[i] = chars[i + 1] = " "
            stack.append(["line_comment"])
            i += 2
            continue
        if ch == "/" and i + 1 < n and text[i + 1] == "*":
            chars[i] = chars[i + 1] = " "
            stack.append(["block_comment"])
            i += 2
            continue
        if ch in "'\"":
            if prev_sig and (prev_sig.isalnum() or prev_sig in "_$"):
                # JSX 文本里的撇号（Don't）不是字符串起始
                prev_sig = ch
                i += 1
                continue
            stack.append(["string", ch])
            prev_sig = ch
            i += 1
            continue
        if ch == "`":
            stack.append(["template"])
            prev_sig = ch
            i += 1
            continue
        if ch == "/" and _regex_can_start(chars, i):
            stack.append(["regex", False])
            prev_sig = ch
            i += 1
            continue
        if ch == "{" and frame[1] > 0:
            frame[1] += 1
        elif ch == "}" and frame[1] > 0:
            frame[1] -= 1
            if frame[1] == 0:
                stack.pop()
                prev_sig = ch
                i += 1
                continue
        if not ch.isspace():
            prev_sig = ch
        i += 1

    return "".join(chars)


def find_violations(text: str) -> list[tuple[int, str, str]]:
    """返回 (行号, 硬编码路径, 原始行文本)，注释内容不参与匹配。"""
    masked = _mask_comments(text)
    original_lines = text.splitlines()
    violations: list[tuple[int, str, str]] = []
    for lineno, line in enumerate(masked.splitlines(), 1):
        for m in API_PATH_RE.finditer(line):
            violations.append((lineno, m.group(1), original_lines[lineno - 1].strip()))
    return violations


def scan() -> list[tuple[Path, int, str, str]]:
    """返回 (文件, 行号, 硬编码路径, 原行文本) 列表。"""
    violations: list[tuple[Path, int, str, str]] = []
    for ts_file in sorted(WEB_ROOT.rglob("*")):
        if ts_file.suffix not in (".ts", ".tsx"):
            continue
        rel_parts = list(ts_file.relative_to(WEB_ROOT).parts)
        if should_exclude(rel_parts, ts_file.name):
            continue
        try:
            text = ts_file.read_text()
        except Exception:
            continue
        for lineno, path_literal, line in find_violations(text):
            violations.append((ts_file, lineno, path_literal, line))
    return violations


def main() -> int:
    print("=" * 55)
    print("  paths.ts 一致性门禁 — API 路径硬编码检查")
    print("=" * 55)
    print()

    if not WEB_ROOT.is_dir():
        print(
            f"ERROR: 必需输入 {WEB_ROOT.relative_to(ROOT)} 不存在——输入缺失说明仓库结构被破坏，"
            "本门禁未扫描任何前端文件，不能当作通过",
            file=sys.stderr,
        )
        return 1

    violations = scan()

    if not violations:
        print("PASS: web/src/ 生产代码零硬编码 /api/v1/ 字面量")
        print("  → 全部 API 路径从 shared/api/paths.ts 常量获取")
        return 0

    print(f"FAIL: 发现 {len(violations)} 处硬编码 /api/v1/（应改用 paths.ts 常量）\n")
    for fpath, lineno, path_literal, _ in violations:
        rel = fpath.relative_to(ROOT)
        print(f"  {rel}:{lineno}  →  {path_literal}")

    print()
    print("修复指引:")
    print("  1. 在 paths.ts 里找对应常量（如 APPLICATIONS_BY_ID(id)）")
    print("  2. import { APPLICATION_BY_ID } from '@/shared/api/paths'")
    print("  3. 替换硬编码字符串为常量调用")
    print("  4. 如果常量不存在 → 先加到 paths.ts（新增常量 + 测试覆盖）")
    return 1


if __name__ == "__main__":
    sys.exit(main())
