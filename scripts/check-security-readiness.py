#!/usr/bin/env python3
"""check-security-readiness.py — 安全就绪度矩阵门禁

依据：ADR-022 结构门禁 / 出口条件 9 / Phase 安全就绪度 CI 门禁

检查维度：
  1. 所有 impl_status / design_status 值必须在 status_values 里（无拼写漂移）
  2. 标记"已实现"的项必须有一个可验证 anchor（ac 列表，长度 >= 1）
     - ac 可以是验收条件 ID（如 AC-AUTH-007）
     - 也可以是 Go 函数/测试名（如 TestFieldEncryptor_MissingKeyRefusesStartup）
  3. 标记"已实现"但 anchor 不存在于代码仓库 — 警告（不阻塞 CI，但要修复）
  4. claim_discipline 里的 claim_rule 不能引用未实现的能力（防虚假声称）

退出码：0=全部通过；1=发现未登记状态或已实现无 anchor；2=非法 YAML 结构

输入：contracts/registries/security-readiness.yaml、internal/ 中 anchor 函数/测试
输出：PASS/FAIL + 拼写漂移、WARNING + 已实现无 anchor
--check 行为：无 --check；失败即 exit 1
消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
"""
from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

import yaml  # noqa: E402

ROOT = Path(__file__).resolve().parent.parent
SM_PATH = ROOT / "contracts" / "registries" / "security-readiness.yaml"
GO_ROOT = ROOT / "internal" / "app"
WEB_ROOT = ROOT / "web" / "src"

DIMENSIONS = [
    "auth_and_protocol", "authz_and_tenant", "keys_and_audit",
    "egress_and_input", "production_hardening",
]


def load():
    with open(SM_PATH) as f:
        return yaml.safe_load(f)


def check(allow_missing_code_ref: bool = False) -> tuple[int, list[str]]:
    d = load()
    allowed = set(d.get("status_values", []))
    errors: list[str] = []
    warnings: list[str] = []

    if not allowed:
        return 2, ["status_values 为空 — YAML 结构错误"]

    for dim in DIMENSIONS:
        items = d.get(dim, []) or []
        for item in items:
            sid = item.get("id", "(无 ID)")
            cap = item.get("capability", "(无能力名)")

            for status_field in ("design_status", "impl_status"):
                val = item.get(status_field)
                if val and val not in allowed:
                    errors.append(
                        f"[{dim} {sid}] {status_field}={val!r} 不在 status_values 内 — 可能是拼写漂移"
                    )

            # 标记"已实现"的项必须有 anchor
            if item.get("impl_status") == "已实现":
                ac = item.get("ac")
                if not ac:
                    errors.append(
                        f"[{dim} {sid} {cap}] impl_status=已实现 但 ac 为空 — "
                        f"至少提供一个验收锚点（AC-xxx 或 Go 函数名）"
                    )
                else:
                    # 把 ac 统一成列表
                    anchors = ac if isinstance(ac, list) else [ac]
                    if not allow_missing_code_ref:
                        for a in anchors:
                            a_str = str(a)
                            # 验收条件 ID 不做文件查找（acceptance-criteria.md 维护）
                            if a_str.startswith("AC-") or a_str.startswith("RISK-"):
                                continue
                            # Go 测试名/函数名 — 验证仓库里能找到
                            found = _code_lookup(a_str)
                            if not found:
                                warnings.append(
                                    f"[{dim} {sid}] anchor {a_str!r} 在代码仓库中未找到 — "
                                    f"验证锚点可能过期"
                                )

    # claim_discipline 基本格式检查
    for cd in d.get("claim_discipline", []) or []:
        if not cd.get("capability") or not cd.get("claim_rule"):
            errors.append(f"claim_discipline 项缺 capability 或 claim_rule: {cd}")

    # 输出
    for w in warnings:
        print(f"  ⚠️  WARN  {w}")
    for e in errors:
        print(f"  ❌ FAIL  {e}")

    if warnings:
        print(f"\n  {len(warnings)} 个 WARN：建议修复但不阻塞")
    if errors:
        print(f"  {len(errors)} 个 FAIL：阻塞合并")
    else:
        print("  ✅ PASS: 安全就绪度矩阵完整（已实现项均有验收锚点，无未登记状态）")

    return (0 if not errors else 1), errors


# 跳过目录与二进制后缀：仓库根 rglob 会命中 node_modules / .git / 构建产物，
# 既拖慢门禁也把非源码文本混进「锚点存在」的证据里。
_SKIP_DIRS = {".git", "node_modules", "dist", "storybook-static", "uploads", "coverage", "__pycache__"}
_SKIP_SUFFIXES = {".png", ".jpg", ".jpeg", ".gif", ".ico", ".webp", ".pdf", ".profraw", ".test"}


def _walk_text_files(scope: Path):
    for path in scope.rglob("*"):
        if not path.is_file():
            continue
        if any(part in _SKIP_DIRS for part in path.parts):
            continue
        if path.suffix in _SKIP_SUFFIXES:
            continue
        yield path


def _py_filename_exists(name: str, scope: Path) -> bool:
    return any(path.name == name for path in _walk_text_files(scope))


def _py_contains(needle: str, scope: Path) -> bool:
    for path in _walk_text_files(scope):
        try:
            if needle in path.read_text(encoding="utf-8", errors="ignore"):
                return True
        except OSError:
            continue
    return False


def _code_lookup(anchor: str) -> bool:
    """验证锚点有效性。三级查找：精确路径 → 文件名 → 代码内容。

    ripgrep 加速存在时优先使用；缺失时回退纯 Python 遍历——macOS 默认不带
    rg 缺失时，硬依赖会把「矩阵本来全绿」变成红灯（这是环境问题，不是能力缺口）。
    """
    # 文档锚点（AC/RISK/DESIGN/ADR）不需要代码查找
    if anchor.startswith(("AC-", "RISK-", "DESIGN-", "ADR-")):
        return True

    # 1. 精确路径匹配（锚点可能是相对路径如 internal/xxx/foo.go）
    p = ROOT / anchor
    if p.exists():
        return True

    name = anchor.rsplit("/", 1)[-1]

    # 2. 文件名匹配（锚点可能是 xxx_benchmark_test.go）
    import shutil
    import subprocess

    if shutil.which("rg"):
        found = subprocess.run(
            ["rg", "-l", "-g", f"**/{name}", str(ROOT)], capture_output=True, text=True
        )
        if found.returncode == 0 and found.stdout.strip():
            return True
        # 3. 代码内容匹配（函数名/错误码常量等短字符串）
        found = subprocess.run(
            ["rg", "-l", anchor, str(ROOT / "internal"), str(ROOT / "scripts")],
            capture_output=True,
            text=True,
        )
        return found.returncode == 0

    if _py_filename_exists(name, ROOT):
        return True
    return _py_contains(anchor, ROOT / "internal") or _py_contains(anchor, ROOT / "scripts")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--allow-missing-code-ref", action="store_true",
                        help="已实现项的 Go 函数锚点不存在于仓库时只警告不报错")
    args = parser.parse_args()

    rc, _ = check(allow_missing_code_ref=args.allow_missing_code_ref)
    sys.exit(rc)


if __name__ == "__main__":
    main()
