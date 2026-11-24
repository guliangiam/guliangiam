#!/usr/bin/env python3
"""check-short-lived-state.py — 短期状态矩阵登记完整性门禁

唯一事实源：contracts/registries/short-lived-state.yaml
消费方：verify-docs.sh（RUN_CODE_CHECKS=1）

检查项：
  1. 注册表含 version 与 states 列表，且 states 非空；
  2. 每条状态登记 id / storage / multi_region / fail_mode / cleanup —— 缺一即失败；
  3. storage 取值 ∈ {redis, postgres, redis+postgres, redis_pubsub+postgres}；
  4. multi_region 取值 ∈ {per_region_independent, control_plane_outbox_fanout, global}；
  5. 幂等缓存（storage=postgres 且 id=idempotency_cache）必须带 ttl。

输出：PASS/FAIL + 违规项。
--check 行为：无 --check；失败即 exit 1。
"""
from __future__ import annotations

import sys
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parent.parent
REGISTRY = ROOT / "contracts" / "registries" / "short-lived-state.yaml"

REQUIRED_FIELDS = ("id", "storage", "multi_region", "fail_mode", "cleanup")
# postgres+mysql：同一状态在两种运行期方言各有实现（如 dpop_jti 的 dpop_replay 表，
# PG/MySQL 仓储同时存在），不是「降级」也不是「双语种混合存储」。
ALLOWED_STORAGE = {"redis", "postgres", "redis+postgres", "redis_pubsub+postgres", "postgres+mysql"}
ALLOWED_MULTI_REGION = {"per_region_independent", "control_plane_outbox_fanout", "global"}


def main() -> int:
    if not REGISTRY.exists():
        print(f"FAIL: 注册表缺失 {REGISTRY.relative_to(ROOT)}")
        return 1

    data = yaml.safe_load(REGISTRY.read_text(encoding="utf-8")) or {}
    problems: list[str] = []

    if "version" not in data:
        problems.append("缺 version 字段")

    states = data.get("states")
    if not isinstance(states, list) or not states:
        print("FAIL: short-lived-state.yaml 的 states 列表为空或缺失")
        return 1

    seen_ids: set[str] = set()
    for idx, entry in enumerate(states):
        if not isinstance(entry, dict):
            problems.append(f"states[{idx}] 不是映射")
            continue
        sid = entry.get("id", f"states[{idx}]")
        for field in REQUIRED_FIELDS:
            if not entry.get(field):
                problems.append(f"{sid}: 缺必填字段 {field}")
        storage = entry.get("storage")
        if storage and storage not in ALLOWED_STORAGE:
            problems.append(f"{sid}: storage 取值非法 {storage!r}")
        mr = entry.get("multi_region")
        if mr and mr not in ALLOWED_MULTI_REGION:
            problems.append(f"{sid}: multi_region 取值非法 {mr!r}")
        if sid in seen_ids:
            problems.append(f"{sid}: 重复 id")
        seen_ids.add(sid)
        if sid == "idempotency_cache" and not entry.get("ttl"):
            problems.append(f"{sid}: 幂等缓存必须登记 ttl")

    if problems:
        print(f"FAIL: 短期状态矩阵登记不完整（{len(problems)} 项）")
        for p in problems:
            print(f"  - {p}")
        return 1

    print(f"PASS: 短期状态矩阵登记完整（{len(states)} 条，含多区域策略与失败姿态）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
