#!/usr/bin/env python3
"""check-tenant-isolation.py — 租户隔离分类门禁（静态，无需 DSN）。

为什么需要本门禁：`contracts/registries/tenant-isolation.yaml` 此前只是文档，
没有任何消费者；新增表、删掉 RLS、把租户表误标为 global 都不会被发现。

断言：
  1. contracts/schema/*.yaml 中每张表必须且仅属于一类：
     direct（含 organization_id）、indirect（无 organization_id 但 RLS 谓词经父表间接隔离）、
     self_scoped（organizations）、platform_scoped（平台专用）、global（白名单）。
  2. direct / indirect / self_scoped / platform_scoped 必须 rls: true。
  3. indirect 必须有 rls_predicate，且谓词引用 organization_id 与至少一张父表。
  4. global 只允许 yaml 登记项；global 不得含 organization_id、不得 rls: true。
  5. 登记在 yaml 的 global/platform/self 表必须真实存在于 schema（防止陈旧白名单）。
  6. topology.required_columns 必须全部存在。

用法：python3 scripts/check-tenant-isolation.py
输出：PASS/FAIL + 未分类/缺失 RLS/白名单冲突明细
消费方：scripts/verify-docs.sh（CI 的 RUN_CODE_CHECKS=1）
"""

from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import yaml  # noqa: E402

from schema_common import load_tables, short_name  # noqa: E402

ROOT = Path(__file__).resolve().parent.parent
REGISTRY = ROOT / "contracts" / "registries" / "tenant-isolation.yaml"


def load_registry() -> dict:
    with open(REGISTRY, "r", encoding="utf-8") as fh:
        return yaml.safe_load(fh)


def main() -> int:
    registry = load_registry()
    classification = registry.get("classification", {})
    topology = registry.get("topology", {})
    identity = registry.get("identity", {})

    self_scoped = set(classification.get("self_scoped", []))
    platform_scoped = set(classification.get("platform_scoped", []))
    global_entries = classification.get("global", [])
    global_tables = {entry["table"] if isinstance(entry, dict) else entry for entry in global_entries}
    global_reasons = {
        (entry["table"] if isinstance(entry, dict) else entry): (entry.get("reason", "") if isinstance(entry, dict) else "")
        for entry in global_entries
    }

    tables = load_tables()
    seen = {short_name(t["table_name"]) for t in tables}
    errors: list[str] = []
    counts = {"direct": 0, "indirect": 0, "self_scoped": 0, "platform_scoped": 0, "global": 0}

    for table in tables:
        name = short_name(table["table_name"])
        columns = {col["name"] for col in table.get("columns", [])}
        rls = table.get("rls") is True
        predicate = (table.get("rls_predicate") or "").strip()

        if name in self_scoped:
            kind = "self_scoped"
        elif name in platform_scoped:
            kind = "platform_scoped"
        elif name in global_tables:
            kind = "global"
        elif "organization_id" in columns:
            kind = "direct"
        else:
            kind = "indirect"

        counts[kind] += 1

        if kind == "global":
            if "organization_id" in columns:
                errors.append(f"{name}: 登记为 global 但含 organization_id 列（租户表不能进全局白名单）")
            if rls:
                errors.append(f"{name}: 登记为 global 但 rls: true（全局表不得启用租户策略）")
            if not global_reasons.get(name):
                errors.append(f"{name}: global 白名单缺少 reason")
            continue

        if not rls:
            errors.append(f"{name}: {kind} 表缺少 rls: true")

        if kind == "indirect":
            if not predicate:
                errors.append(f"{name}: indirect 表缺少 rls_predicate（无法证明间接租户隔离）")
            else:
                if "organization_id" not in predicate:
                    errors.append(f"{name}: indirect 谓词未引用 organization_id")
                if "exists" not in predicate.lower() and " in (" not in predicate.lower():
                    errors.append(f"{name}: indirect 谓词未经父表 EXISTS/IN 子查询")
        elif kind == "direct" and predicate and "organization_id" not in predicate:
            # 缺省谓词由生成器补 organization_id = current_organization_id()；显式覆盖必须保留租户列。
            errors.append(f"{name}: direct 表显式谓词未引用 organization_id")

    for name in sorted(self_scoped | platform_scoped | global_tables):
        if name not in seen:
            errors.append(f"{name}: 登记在 tenant-isolation.yaml 但 schema 中不存在（陈旧白名单）")

    identity_table = identity.get("table")
    if identity_table and identity_table not in seen:
        errors.append(f"{identity_table}: identity 表不存在")

    topology_table = topology.get("table")
    if topology_table and topology_table not in seen:
        errors.append(f"{topology_table}: topology 表不存在")
    if topology_table:
        topology_columns = next(
            ({col["name"] for col in t.get("columns", [])} for t in tables if short_name(t["table_name"]) == topology_table),
            set(),
        )
        for required in topology.get("required_columns", []):
            if required not in topology_columns:
                errors.append(f"{topology_table}: 缺少 topology.required_columns 登记列 {required}")

    total = len(tables)
    print(
        f"tenant-isolation: tables={total} direct={counts['direct']} indirect={counts['indirect']} "
        f"self_scoped={counts['self_scoped']} platform_scoped={counts['platform_scoped']} global={counts['global']}"
    )
    if errors:
        for item in errors:
            print(f"FAIL {item}")
        print(f"FAIL tenant-isolation: {len(errors)} 处问题")
        return 1
    print("PASS tenant-isolation")
    return 0


if __name__ == "__main__":
    sys.exit(main())
