#!/usr/bin/env python3
"""文档链接门禁：正式文档中的相对链接必须可解析（docs/README.md §1 规则 4）。

Why this exists
---------------
check-doc-anchors.py 校验「《文档名》§N.M」式的章节锚点，但不校验 Markdown
相对链接。事实源目录搬迁（结构声明与注册表目录、OpenAPI 生成器均迁到新位置）之后，
指向旧位置的链接会静默 404：读者点击后落到不存在的位置，而所有既有门禁仍然全绿。
本脚本把「链接可解析」变成可执行判定，覆盖两类失效：

  1. 相对路径断链（文件/目录被移动、改名、删除）；
  2. 链接逃逸仓库根（../.. 越过仓库边界，落到无法随仓库分发的路径）。

判定范围：docs/**/*.md 与仓库根 README.md / CONTRIBUTING.md / SECURITY.md。
跳过：围栏代码块、行内代码 span、HTML 注释、外部 URL、纯锚点与协议相对 URL
（//host/...）；图片链接一并校验。

输入：上述 Markdown 文件
输出：PASS/FAIL + 断链列表（文件:行 → 目标 → 解析结果）
--check 行为：无 --check；存在断链即以非零码退出
消费方：scripts/verify-docs.sh
"""

from __future__ import annotations

import os
import re
import sys
from urllib.parse import unquote

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

ROOT_DOCS = ("README.md", "CONTRIBUTING.md", "SECURITY.md")

LINK_RE = re.compile(r"!?\[[^\]]*\]\(\s*(<[^>]*>|[^)\s]+)(?:\s+[\"'][^\"']*[\"'])?\s*\)")
REF_LINK_RE = re.compile(r"^ {0,3}\[[^\]]+\]:\s*(<[^>]*>|\S+)")
INLINE_CODE_RE = re.compile(r"`+[^`\n]*`+")
HTML_COMMENT_RE = re.compile(r"<!--.*?-->", re.DOTALL)
SCHEME_RE = re.compile(r"^[A-Za-z][A-Za-z0-9+.-]*:")

SKIP_SCHEMES = {"http", "https", "mailto", "tel", "data", "javascript", "ftp", "ssh"}


def split_comments_preserving_lines(text):
    """把 HTML 注释替换为等长空白（保留换行，行号不漂移）。"""

    def blank(match):
        return "".join("\n" if ch == "\n" else " " for ch in match.group(0))

    return HTML_COMMENT_RE.sub(blank, text)


def strip_noise(text):
    """返回 [(行号, 去噪后的行)]：去围栏代码块、行内代码、HTML 注释。"""
    text = split_comments_preserving_lines(text)
    lines = text.split("\n")
    kept = []
    in_fence = False
    fence_marker = ""
    for number, line in enumerate(lines, 1):
        stripped = line.lstrip()
        if stripped.startswith("```") or stripped.startswith("~~~"):
            marker = stripped[:3]
            if not in_fence:
                in_fence, fence_marker = True, marker
            elif marker == fence_marker:
                in_fence = False
            continue
        if in_fence:
            continue
        kept.append((number, INLINE_CODE_RE.sub("", line)))
    return kept


def resolve_target(source_path, raw_target):
    """解析链接目标，返回 (仓库相对路径或 None, 失败原因)。"""
    target = raw_target.strip()
    if target.startswith("<") and target.endswith(">"):
        target = target[1:-1].strip()
    if not target:
        return None, "empty target"
    if target.startswith("#"):
        return None, ""
    if target.startswith("//"):
        return None, ""
    scheme = SCHEME_RE.match(target)
    if scheme:
        if scheme.group(0)[:-1].lower() in SKIP_SCHEMES:
            return None, ""
        return None, ""
    target = target.split("#", 1)[0].split("?", 1)[0]
    if not target:
        return None, ""
    target = unquote(target)
    if target.startswith("/"):
        base = REPO
        target = target.lstrip("/")
    else:
        base = os.path.dirname(source_path)
    resolved = os.path.normpath(os.path.join(base, target))
    try:
        rel = os.path.relpath(resolved, REPO)
    except ValueError:
        return None, "无法相对化路径"
    if rel.startswith(".."):
        return None, "链接逃逸仓库根"
    return resolved, ""


def iter_doc_files():
    paths = []
    docs_root = os.path.join(REPO, "docs")
    for dirpath, dirnames, filenames in os.walk(docs_root):
        dirnames[:] = sorted(d for d in dirnames if d not in {"node_modules", ".git"})
        for name in sorted(filenames):
            if name.endswith(".md"):
                paths.append(os.path.join(dirpath, name))
    for name in ROOT_DOCS:
        candidate = os.path.join(REPO, name)
        if os.path.isfile(candidate):
            paths.append(candidate)
    return paths


def main():
    problems = []
    checked = 0
    for path in iter_doc_files():
        with open(path, encoding="utf-8") as handle:
            raw = handle.read()
        relpath = os.path.relpath(path, REPO)
        for number, line in strip_noise(raw):
            for match in LINK_RE.finditer(line):
                checked += 1
                resolved, reason = resolve_target(path, match.group(1))
                if resolved is None:
                    if reason:
                        problems.append(f"{relpath}:{number}: {reason} -> {match.group(1)}")
                    continue
                if not os.path.exists(resolved):
                    shown = os.path.relpath(resolved, REPO)
                    problems.append(
                        f"{relpath}:{number}: 断链 -> {match.group(1)}（解析为 {shown}）"
                    )
            ref = REF_LINK_RE.match(line)
            if ref:
                checked += 1
                resolved, reason = resolve_target(path, ref.group(1))
                if resolved is None:
                    if reason:
                        problems.append(f"{relpath}:{number}: {reason} -> {ref.group(1)}")
                elif not os.path.exists(resolved):
                    shown = os.path.relpath(resolved, REPO)
                    problems.append(
                        f"{relpath}:{number}: 引用式断链 -> {ref.group(1)}（解析为 {shown}）"
                    )

    if problems:
        print(f"=== check-doc-links.py: FAIL（{len(problems)} 处）===", file=sys.stderr)
        for problem in problems:
            print(f"  {problem}", file=sys.stderr)
        print(
            "修正方式：把链接改为仓库内真实路径；事实源路径以 docs/README.md §3 为准。",
            file=sys.stderr,
        )
        return 1

    print(f"PASS: 文档链接可解析（{checked} 个相对链接，0 断链）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
