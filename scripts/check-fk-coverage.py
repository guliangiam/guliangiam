#!/usr/bin/env python3
"""表间引用完整性门禁：UUID *_id 列必须有物理外键，或在裁决表登记分类。

Why this exists
---------------
声明式 Schema 有 200+ 个 *_id 列。物理外键由 fk_constraints 生成，但「哪些引用故意
不建外键」此前没有任何登记或门禁：一次删掉 fk_constraints、或新增一个 *_id 列而忘记
外键，DDL 照常生成、所有门禁全绿，而数据库从此失去引用完整性——孤儿行、跨聚合悬空
引用只会在业务读取时暴露。审计发现 6 个 UUID 引用列没有任何物理外键（含租户列
webauthn_challenges.organization_id、用户列 qr_code_sessions.user_id）。

裁决模型（唯一事实源：contracts/registries/referential-integrity.yaml）：
  * UUID 类型 + _id 后缀：必须被 fk_constraints 覆盖，或登记为
    logical_references（架构上不建 FK，须写理由）/ pending_physical_fks（目标态补 FK，
    须指向状态为 开放的开放项）。
  * 非 UUID 的文本/字节类型 + _id 后缀：外部系统标识（OAuth client_id、第三方流水号），
    不建物理 FK，由应用层校验；无需逐列登记。
  * 非 UUID/非文本类型的 _id 后缀列：命名异常（如布尔列 use_email_as_saml_name_id），
    WARN + 开放项登记，重命名需版本化迁移。

输入：contracts/schema/*.yaml、contracts/registries/referential-integrity.yaml、
      contracts/registries/open-items.yaml
输出：PASS/FAIL + 未分类引用 / 悬空分类 / 命名异常清单
--check 行为：无 --check；存在未分类或悬空分类即非零退出
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
REGISTRY = os.path.join(REPO, "contracts", "registries", "referential-integrity.yaml")
OPEN_ITEMS = os.path.join(REPO, "contracts", "registries", "open-items.yaml")

TEXT_TYPES = ("TEXT", "VARCHAR", "CHAR", "BYTEA", "CITEXT", "UUID")
UUID_TYPE_RE = re.compile(r"^UUID\b", re.IGNORECASE)
TEXT_TYPE_RE = re.compile(r"^(TEXT|VARCHAR|CHAR|BYTEA|CITEXT)\b", re.IGNORECASE)
TARGET_RE = re.compile(r"^guliangiam\.([a-z0-9_]+)\(([a-z0-9_]+)\)$")


def load_yaml(path):
    with open(path, encoding="utf-8") as handle:
        return yaml.safe_load(handle) or {}


def load_schema():
    tables: dict[str, dict[str, str]] = {}
    fk_covered: set[tuple[str, str]] = set()
    for path in sorted(glob.glob(SCHEMA_GLOB)):
        with open(path, encoding="utf-8") as handle:
            for doc in yaml.safe_load_all(handle):
                if not isinstance(doc, dict) or not doc.get("table_name"):
                    continue
                table = str(doc["table_name"]).split(".")[-1]
                columns = {
                    column.get("name"): str(column.get("type") or "")
                    for column in (doc.get("columns") or [])
                    if isinstance(column, dict) and column.get("name")
                }
                tables[table] = columns
                for fk in doc.get("fk_constraints") or []:
                    if not isinstance(fk, dict):
                        continue
                    if fk.get("column"):
                        fk_covered.add((table, str(fk["column"])))
                    for column in fk.get("columns") or []:
                        fk_covered.add((table, str(column)))
    return tables, fk_covered


def load_open_items():
    data = load_yaml(OPEN_ITEMS)
    open_ids = set()
    for item in data.get("items") or []:
        if isinstance(item, dict) and item.get("status") == "open":
            open_ids.add(str(item.get("id")))
    return open_ids


def main():
    if not os.path.isfile(REGISTRY):
        print(f"FAIL: 缺少引用完整性裁决表 {os.path.relpath(REGISTRY, REPO)}", file=sys.stderr)
        return 1
    tables, fk_covered = load_schema()
    if not tables:
        print("FAIL: contracts/schema/*.yaml 未解析出任何表", file=sys.stderr)
        return 1
    registry = load_yaml(REGISTRY)
    open_ids = load_open_items()

    problems: list[str] = []
    policy = registry.get("policy") or {}
    for key in ("uuid_identifier", "external_identifier", "naming_anomaly", "on_delete_policy"):
        if not policy.get(key):
            problems.append(f"referential-integrity.yaml policy.{key} 缺失")

    logical: dict[tuple[str, str], dict] = {}
    for entry in registry.get("logical_references") or []:
        if not isinstance(entry, dict):
            problems.append("logical_references 条目不是映射")
            continue
        key = (str(entry.get("table")), str(entry.get("column")))
        if key in logical:
            problems.append(f"logical_references 重复登记 {key[0]}.{key[1]}")
        logical[key] = entry
        if not entry.get("kind") or not entry.get("reason"):
            problems.append(f"logical_references {key[0]}.{key[1]} 缺 kind/reason")

    pending: dict[tuple[str, str], dict] = {}
    for entry in registry.get("pending_physical_fks") or []:
        if not isinstance(entry, dict):
            problems.append("pending_physical_fks 条目不是映射")
            continue
        key = (str(entry.get("table")), str(entry.get("column")))
        if key in pending:
            problems.append(f"pending_physical_fks 重复登记 {key[0]}.{key[1]}")
        pending[key] = entry
        target = TARGET_RE.match(str(entry.get("target") or ""))
        if not target:
            problems.append(f"pending_physical_fks {key[0]}.{key[1]} target 格式非法（应为 guliangiam.table(column)）")
        elif target.group(1) not in tables or target.group(2) not in tables.get(target.group(1), {}):
            problems.append(f"pending_physical_fks {key[0]}.{key[1]} target 指向不存在的表/列")
        if not entry.get("on_delete") or not entry.get("on_update"):
            problems.append(f"pending_physical_fks {key[0]}.{key[1]} 缺显式 on_delete/on_update")
        item = str(entry.get("open_item") or "")
        if item not in open_ids:
            problems.append(f"pending_physical_fks {key[0]}.{key[1]} 的 open_item {item or '<空>'} 未登记为开放项")
        if not entry.get("reason"):
            problems.append(f"pending_physical_fks {key[0]}.{key[1]} 缺 reason")

    uuid_total = uuid_fk = uuid_logical = uuid_pending = external = 0
    naming_anomalies: list[str] = []
    for table, columns in sorted(tables.items()):
        for column, column_type in sorted(columns.items()):
            if not column.endswith("_id"):
                continue
            key = (table, column)
            if key in fk_covered:
                if key in logical or key in pending:
                    problems.append(f"{table}.{column} 已有物理 FK，却仍登记在裁决表中")
                continue
            if UUID_TYPE_RE.match(column_type):
                uuid_total += 1
                if key in logical:
                    uuid_logical += 1
                elif key in pending:
                    uuid_pending += 1
                else:
                    problems.append(
                        f"{table}.{column} 是 UUID 引用列，既无 fk_constraints 也不在裁决表中"
                    )
            elif TEXT_TYPE_RE.match(column_type):
                external += 1
            else:
                naming_anomalies.append(f"{table}.{column}（{column_type}）")

    for key, entry in list(logical.items()) + list(pending.items()):
        table, column = key
        if table not in tables:
            problems.append(f"裁决表条目 {table}.{column} 指向不存在的表")
        elif column not in tables[table]:
            problems.append(f"裁决表条目 {table}.{column} 指向不存在的列")

    if problems:
        print(f"=== check-fk-coverage.py: FAIL（{len(problems)} 处）===", file=sys.stderr)
        for problem in problems:
            print(f"  - {problem}", file=sys.stderr)
        print(
            "修正方式：为 UUID 引用列补 fk_constraints，或在 referential-integrity.yaml 登记"
            " logical_references（永久逻辑引用）或 pending_physical_fks（指向开放项）。",
            file=sys.stderr,
        )
        return 1

    print(
        f"PASS: 引用完整性分类完整（UUID 引用列 {uuid_total}：物理 FK 外 {uuid_logical} 逻辑引用、"
        f"{uuid_pending} 待补 FK（OPEN-API-30）；外部标识 {external} 列豁免；"
        f"物理 FK 覆盖 {len(fk_covered)} 列）"
    )
    if uuid_pending:
        print(
            f"WARN: {uuid_pending} 个 pending_physical_fks 尚未落物理外键（exit_condition 见 OPEN-API-30）",
            file=sys.stderr,
        )
    for anomaly in naming_anomalies:
        print(f"WARN: 命名异常 _id 后缀列 {anomaly}（OPEN-API-31）", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
