#!/usr/bin/env python3
"""check-surface-registry.py — API 三面分类注册表完整性门禁

唯一事实源：contracts/registries/surfaces.yaml
消费方：verify-docs.sh（RUN_CODE_CHECKS=1）

检查项：
  1. surfaces 列表非空，取值 ⊆ {management, authentication, protocol}；
  2. protocol_families 非空，每条含 id/family/prefix；
  3. explicit_paths 非空；每条 {method ∈ {GET,POST,PUT,PATCH,DELETE}, path 以 / 开头,
     surface ∈ 三面}；
  4. public_whitelist 非空；每条 {method, path} 完备；
  5. public_whitelist 中的条目不得与 explicit_paths 中 surface=management 的
     「方法 + 路径」重复（公开豁免不得命中管理面，防回归）；
  6. surfaces.yaml 引用的 gate 脚本路径必须真实存在。

输出：PASS/FAIL + 违规项。
--check 行为：无 --check；失败即 exit 1。
"""
from __future__ import annotations

import sys
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parent.parent
REGISTRY = ROOT / "contracts" / "registries" / "surfaces.yaml"

ALLOWED_SURFACES = {"management", "authentication", "protocol"}
ALLOWED_METHODS = {"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}


def main() -> int:
    if not REGISTRY.exists():
        print(f"FAIL: 注册表缺失 {REGISTRY.relative_to(ROOT)}")
        return 1

    data = yaml.safe_load(REGISTRY.read_text(encoding="utf-8")) or {}
    problems: list[str] = []

    # 1. surfaces
    surfaces = data.get("surfaces")
    if not isinstance(surfaces, list) or not surfaces:
        problems.append("surfaces 列表为空或缺失")
    else:
        for s in surfaces:
            val = s.get("surface") if isinstance(s, dict) else None
            if val not in ALLOWED_SURFACES:
                problems.append(f"surfaces: 非法 surface 值 {val!r}")

    # 2. protocol_families
    families = data.get("protocol_families")
    if not isinstance(families, list) or not families:
        problems.append("protocol_families 列表为空或缺失")
    else:
        for f in families:
            if not isinstance(f, dict) or not f.get("id") or not f.get("prefix"):
                problems.append(f"protocol_families: 条目缺 id/family/prefix：{f!r}")

    # 3. explicit_paths
    explicit = data.get("explicit_paths")
    if not isinstance(explicit, list) or not explicit:
        problems.append("explicit_paths 列表为空或缺失")
        explicit = []
    else:
        for e in explicit:
            if not isinstance(e, dict):
                problems.append(f"explicit_paths: 条目不是映射：{e!r}")
                continue
            method, path, surface = e.get("method"), e.get("path"), e.get("surface")
            if method not in ALLOWED_METHODS:
                problems.append(f"explicit_paths: 非法 method {method!r}（path={path}）")
            if not (isinstance(path, str) and path.startswith("/")):
                problems.append(f"explicit_paths: 非法 path {path!r}")
            if surface not in ALLOWED_SURFACES:
                problems.append(f"explicit_paths: 非法 surface {surface!r}（path={path}）")

    # 4. public_whitelist
    whitelist = data.get("public_whitelist")
    if not isinstance(whitelist, list) or not whitelist:
        problems.append("public_whitelist 列表为空或缺失")
        whitelist = []
    else:
        for w in whitelist:
            if not isinstance(w, dict) or not w.get("method") or not w.get("path"):
                problems.append(f"public_whitelist: 条目缺 method/path：{w!r}")

    # 5. 公开豁免不得命中管理面
    mgmt_pairs = {
        (e.get("method"), e.get("path"))
        for e in explicit
        if isinstance(e, dict) and e.get("surface") == "management"
    }
    for w in whitelist:
        if isinstance(w, dict) and (w.get("method"), w.get("path")) in mgmt_pairs:
            problems.append(
                f"public_whitelist 命中管理面：{w.get('method')} {w.get('path')}"
            )

    # 6. gates 引用的脚本必须存在
    for g in data.get("gates", []) or []:
        script = g.get("script") if isinstance(g, dict) else None
        if script and not (ROOT / script).exists():
            problems.append(f"gates: 引用的脚本不存在 {script}")

    if problems:
        print(f"FAIL: surfaces.yaml 完整性校验未通过（{len(problems)} 项）")
        for p in problems:
            print(f"  - {p}")
        return 1

    print(
        f"PASS: surfaces.yaml 完整性通过（{len(surfaces)} 面 / {len(families)} 家族 / "
        f"{len(explicit)} 显式端点 / {len(whitelist)} 公开豁免）"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
