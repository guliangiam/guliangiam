#!/usr/bin/env python3
"""数据生命周期覆盖门禁：命中生命周期类别的表必须被 lifecycle.yaml 覆盖。

Why this exists
---------------
`contracts/registries/lifecycle.yaml` 声明了保留/归档/清理策略，但此前没有任何门禁检查
**哪些表需要生命周期条目**：Schema 有 100+ 张表，注册表只有 10 个条目，新增一张带
`deleted_at`/`expires_at` 的表或追加型日志表时，删除/归档策略可以永远缺席，而所有
既有门禁仍然全绿。数据全生命周期完整性因此只停留在「有注册表」而不是「覆盖所有该覆盖的表」。

判定规则（类别与检测方式由 lifecycle.yaml 的 `coverage.required_classes` 定义，脚本不写死）：
  1. 解析 `contracts/schema/*.yaml`（多文档流）得到全部表与列；
  2. 对每张表按 required_classes 的 detect 规则判定它命中的生命周期类别；
  3. 命中类别的表必须被 `tables` 的**显式条目**或某策略组的 `applies_to` 覆盖；
  4. `tables` 中的非策略条目、以及 `applies_to` 成员必须是真实表（幽灵条目即失败）；
  5. 未命中类别的表由 `coverage.default_policy` 明确覆盖（业务主数据/配置数据）。

输入：contracts/schema/*.yaml、contracts/registries/lifecycle.yaml
输出：PASS/FAIL + 未覆盖表清单（含命中的类别）
--check 行为：无 --check；存在缺口即以非零码退出
消费方：scripts/verify-docs.sh
"""

from __future__ import annotations

import glob
import os
import re
import sys

import yaml

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SCHEMA_GLOB = os.path.join(REPO, "contracts", "schema", "*.yaml")
LIFECYCLE = os.path.join(REPO, "contracts", "registries", "lifecycle.yaml")

# 与 lifecycle.yaml coverage.required_classes 的 detect 文本一一对应；detect 文本是
# 人类可读契约，脚本侧用可执行规则实现同一语义（两者漂移由本文件的注释与注册表评审兜底）。
APPEND_ONLY_RE = re.compile(
    r"(log|logs|event|events|record|records|anchor|anchors|history|attempt|attempts|replay|audit|progress)$"
)


def load_schema_tables() -> dict[str, set[str]]:
    """返回 {短表名: 列名集合}，声明式 Schema 同表名以最后一份为准（不允许重复）。"""
    tables: dict[str, set[str]] = {}
    for path in sorted(glob.glob(SCHEMA_GLOB)):
        with open(path, encoding="utf-8") as handle:
            for doc in yaml.safe_load_all(handle):
                if not isinstance(doc, dict):
                    continue
                full_name = doc.get("table_name")
                if not full_name:
                    continue
                short = str(full_name).split(".")[-1]
                columns = {
                    column.get("name")
                    for column in (doc.get("columns") or [])
                    if isinstance(column, dict) and column.get("name")
                }
                if short in tables:
                    sys.exit(f"ERROR: contracts/schema 出现重复表声明 {short}")
                tables[short] = columns
    return tables


def detect_classes(tables: dict[str, set[str]]) -> dict[str, set[str]]:
    required: dict[str, set[str]] = {}
    for table, columns in tables.items():
        classes: set[str] = set()
        if "deleted_at" in columns:
            classes.add("soft_delete")
        if any(column == "expires_at" or column.endswith("_expires_at") for column in columns):
            classes.add("expires_at")
        if APPEND_ONLY_RE.search(table):
            classes.add("append_only")
        if classes:
            required[table] = classes
    return required


def main() -> int:
    for path in (LIFECYCLE,):
        if not os.path.isfile(path):
            print(f"FAIL: 缺少生命周期注册表 {os.path.relpath(path, REPO)}", file=sys.stderr)
            return 1

    tables = load_schema_tables()
    if not tables:
        print("FAIL: contracts/schema/*.yaml 未解析出任何表", file=sys.stderr)
        return 1

    with open(LIFECYCLE, encoding="utf-8") as handle:
        lifecycle = yaml.safe_load(handle) or {}
    entries = lifecycle.get("tables") or {}
    coverage = lifecycle.get("coverage") or {}
    policy_entries = set(coverage.get("policy_entries") or [])
    required_classes = [item.get("id") for item in coverage.get("required_classes") or []]
    default_policy = coverage.get("default_policy") or {}

    problems: list[str] = []
    if not required_classes:
        problems.append("lifecycle.yaml coverage.required_classes 为空")
    if not default_policy.get("description"):
        problems.append("lifecycle.yaml coverage.default_policy 缺少 description")

    covered: dict[str, set[str]] = {}
    for name, spec in entries.items():
        if not isinstance(spec, dict):
            problems.append(f"lifecycle.yaml 条目 {name} 不是映射")
            continue
        targets = [name] + list(spec.get("applies_to") or [])
        if name in policy_entries:
            targets = list(spec.get("applies_to") or [])
        for target in targets:
            covered.setdefault(target, set()).add(name)

    # 策略组（名字不是真实表，但有 applies_to）是合法条目；只有既不是真实表、也没有
    # applies_to、又不在 policy_entries 白名单里的名字才是幽灵条目。
    ghosts = sorted(
        name
        for name, spec in entries.items()
        if name not in tables
        and name not in policy_entries
        and not (isinstance(spec, dict) and spec.get("applies_to"))
    )
    if ghosts:
        problems.append(f"lifecycle.yaml 条目不是声明式 Schema 的真实表（幽灵条目）: {ghosts}")

    missing_targets = sorted(
        {
            target
            for spec in entries.values()
            if isinstance(spec, dict)
            for target in (spec.get("applies_to") or [])
            if target not in tables
        }
    )
    if missing_targets:
        problems.append(f"lifecycle.yaml applies_to 指向不存在的表: {missing_targets}")

    required = detect_classes(tables)
    uncovered = sorted(table for table in required if table not in covered)
    if uncovered:
        detail = ", ".join(f"{table}{sorted(required[table])}" for table in uncovered)
        problems.append(f"{len(uncovered)} 张命中生命周期类别的表未被覆盖: {detail}")

    detected_classes = set().union(*required.values()) if required else set()
    undeclared = sorted(detected_classes - set(required_classes))
    if undeclared:
        problems.append(f"检测到未在 coverage.required_classes 声明的类别: {undeclared}")

    if problems:
        print(f"=== check-lifecycle-coverage.py: FAIL（{len(problems)} 类问题）===", file=sys.stderr)
        for problem in problems:
            print(f"  - {problem}", file=sys.stderr)
        print(
            "修正方式：在 contracts/registries/lifecycle.yaml 的 tables 增加显式条目或策略组 "
            "applies_to，并在开发规范 19.4 同步登记；幽灵条目必须删除或改名。",
            file=sys.stderr,
        )
        return 1

    covered_required = len(required)
    defaulted = len(tables) - covered_required
    print(
        f"PASS: 生命周期覆盖完整（Schema {len(tables)} 表 / 命中类别 {covered_required} 表全部覆盖 / "
        f"其余 {defaulted} 表按 default_policy 处理 / 策略组 {len(policy_entries)} 个）"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
