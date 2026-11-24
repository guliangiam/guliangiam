#!/usr/bin/env python3
"""文档红线门禁：批次/结项叙事与实施进度词。

目标态文档只写「必须满足什么」，不写「在哪个批次做到哪一步」。本门禁把这两类
叙事从 `check-doc-facts.sh` 中拆出，改为逐行、带上下文判定的检查，避免
「实现状态」「规划中含」「跳过本轮」这类正常术语被词面误伤。
"""
from __future__ import annotations

import os
import re
import sys

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DOCS = os.path.join(REPO, "docs")

# 批次/结项叙事：历史只允许写入 CHANGELOG.md；ADR 也扫描（ADR 不得写批次）。
BATCH_RULES: list[tuple[str, re.Pattern[str]]] = [
    ("batch-第N批", re.compile(r"第[一二三四五六七八九十百零0-9]+批")),
    ("batch-T编号", re.compile(r"(?<![0-9A-Za-z])T[0-9]{2,}(?![0-9A-Za-z])")),
    ("batch-D4M", re.compile(r"D4M-")),
    ("batch-F2P", re.compile(r"F2-P")),
    ("batch-结项", re.compile(r"收敛记录|已归零（20|已收敛（20|已关闭（20|回归修复")),
]

# 实施进度词：ADR 按自身治理规范保留状态词（docs/README.md §1 例外），不扫描。
PROGRESS_RULES: list[tuple[str, str]] = [
    ("progress-当前实现", "当前实现"),
    ("progress-现状", "现状"),
    ("progress-已实现", "已实现"),
    ("progress-已落地", "已落地"),
    ("progress-待接入", "待接入"),
    ("progress-待落地", "待落地"),
    ("progress-规划中", "规划中"),
    ("progress-未实现", "未实现"),
    ("progress-未落地", "未落地"),
    ("progress-本轮", "本轮"),
    ("progress-本批", "本批"),
    ("progress-TODO", "TODO"),
    ("progress-待定", "待定"),
    ("progress-参见后续", "参见后续"),
]

# 逐行上下文豁免：定义、否定式治理口径、术语与技术语义。
# 每项都对应「what / why / trade_off」，见交付报告的决策记录。
CONTEXT_EXEMPTIONS: list[str] = [
    "不登记现状",
    "不含现状",
    "不登记现状三态",
    "禁止现状偏差登记",
    "不登记实现状态",
    "不以「已实现/未实现」区分",
    "规划中含",
    "跳过本轮",
    "改为待接入表述",
    "TODO(phase3)",
    "不能证明",
]

SKIP_DIRS = {"generated"}


def iter_markdown() -> list[str]:
    paths: list[str] = []
    for dirpath, dirnames, filenames in os.walk(DOCS):
        dirnames[:] = sorted(d for d in dirnames if d not in SKIP_DIRS)
        for name in sorted(filenames):
            if name.endswith(".md"):
                paths.append(os.path.join(dirpath, name))
    return paths


def is_progress_scanned(rel: str) -> bool:
    return not rel.startswith("docs/adr/") and rel != "docs/README.md"


def exempted(line: str, word: str) -> bool:
    if any(token in line for token in CONTEXT_EXEMPTIONS):
        return True
    # `实现状态` 是注册表字段名，不含「现状」语义。
    if word == "现状" and "实现状态" in line:
        return True
    return False


def main() -> int:
    hits: list[str] = []
    scanned = 0
    for abspath in iter_markdown():
        rel = os.path.relpath(abspath, REPO)
        try:
            lines = open(abspath, encoding="utf-8").read().split("\n")
        except UnicodeDecodeError:
            continue
        scanned += 1
        for lineno, line in enumerate(lines, 1):
            if "批准" not in line:
                for rule, pattern in BATCH_RULES:
                    if pattern.search(line):
                        hits.append(f"FAIL: {rule} {rel}:{lineno}: {pattern.pattern} | {line.strip()}")
            if is_progress_scanned(rel):
                for rule, word in PROGRESS_RULES:
                    if word in line and not exempted(line, word):
                        hits.append(f"FAIL: {rule} {rel}:{lineno}: {word} | {line.strip()}")

    print("=== check-doc-redlines.py ===")
    print(f"扫描 {scanned} 份 Markdown（docs/generated 排除，ADR 仅扫批次标记）。")
    for hit in hits:
        print(hit, file=sys.stderr)
    print(f"FAIL_COUNT={len(hits)}")
    if hits:
        return 1
    print("PASS: 文档红线检查通过（0 命中）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
