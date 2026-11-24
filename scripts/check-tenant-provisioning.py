#!/usr/bin/env python3
"""check-tenant-provisioning.py — 创建/退役组织的租户接线门禁（静态，无需 DSN）。

为什么需要本门禁：`CreateOrganization` 曾经只写 organizations + 配置子表，
没有租户身份/拓扑。任何人都可能把它改回去，而结构层门禁发现不了运行期回退。

断言（create_organization.go）：
  1. 存在 WithinTransaction；
  2. gates.provisioning_order 的符号按声明顺序出现：
     Organizations().Create → Tenants().CreateIdentity → Tenants().CreatePrimaryTopology → embedAuditEvent；
  3. 上述符号位置必须晚于 WithinTransaction（接线落在事务内）。

断言（delete_organization.go）：
  4. Tenants().DecommissionIdentity 与 Tenants().DecommissionTopologies 均在
     Organizations().Delete 之前，且 embedAuditEvent 在最后；
  5. 文件不得出现物理删除（DELETE FROM）。

用法：python3 scripts/check-tenant-provisioning.py
消费方：scripts/verify-docs.sh（CI 的 RUN_CODE_CHECKS=1）
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import yaml  # noqa: E402

ROOT = Path(__file__).resolve().parent.parent
REGISTRY = ROOT / "contracts" / "registries" / "tenant-isolation.yaml"
ORG_DIR = ROOT / "internal" / "app" / "organizations"
CREATE_FILE = ORG_DIR / "create_organization.go"
DELETE_FILE = ORG_DIR / "delete_organization.go"


def read(path: Path) -> str:
    if not path.exists():
        return ""
    return path.read_text(encoding="utf-8")


def main() -> int:
    with open(REGISTRY, "r", encoding="utf-8") as fh:
        registry = yaml.safe_load(fh)
    order = (registry.get("gates") or {}).get("provisioning_order") or [
        "Organizations().Create",
        "Tenants().CreateIdentity",
        "Tenants().CreatePrimaryTopology",
        "embedAuditEvent",
    ]

    errors: list[str] = []
    create = read(CREATE_FILE)
    delete = read(DELETE_FILE)

    if not create:
        errors.append(f"{CREATE_FILE.relative_to(ROOT)}: 文件不存在")
    if not delete:
        errors.append(f"{DELETE_FILE.relative_to(ROOT)}: 文件不存在")

    def function_body(text: str, name: str) -> str:
        start = text.find(f"func (s *Service) {name}(")
        if start < 0:
            start = text.find(f"func {name}(")
        if start < 0:
            return ""
        nxt = text.find("\nfunc ", start + 1)
        return text[start:] if nxt < 0 else text[start:nxt]

    create_body = function_body(create, "CreateOrganization")
    helper_body = function_body(create, "provisionTenant")

    tx_pos = create_body.find("WithinTransaction")
    if tx_pos < 0:
        errors.append("create_organization.go: CreateOrganization 缺少 WithinTransaction")

    # 事务内必须调用 provisionTenant；helper 内部必须按序写身份与拓扑。
    tx_order = ["Organizations().Create", "provisionTenant", "embedAuditEvent"]
    last_pos = -1
    for symbol in tx_order:
        pos = create_body.find(symbol)
        if pos < 0:
            errors.append(f"create_organization.go: CreateOrganization 缺少接线符号 {symbol}")
            continue
        if tx_pos >= 0 and pos < tx_pos:
            errors.append(f"create_organization.go: {symbol} 出现在 WithinTransaction 之前（未落在事务内）")
        if pos < last_pos:
            errors.append(f"create_organization.go: {symbol} 顺序回退（应保持 {tx_order} 顺序）")
        last_pos = max(last_pos, pos)

    helper_order = [s for s in order if s.startswith("Tenants().")]
    if not helper_body:
        errors.append("create_organization.go: 缺少 provisionTenant（无法证明身份/拓扑写入）")
    else:
        last_pos = -1
        for symbol in helper_order:
            pos = helper_body.find(symbol)
            if pos < 0:
                errors.append(f"create_organization.go: provisionTenant 缺少接线符号 {symbol}")
                continue
            if pos < last_pos:
                errors.append(f"create_organization.go: provisionTenant {symbol} 顺序回退（应保持 {helper_order} 顺序）")
            last_pos = max(last_pos, pos)
        if "Tenants().CreateIdentity" not in helper_body or "Tenants().CreatePrimaryTopology" not in helper_body:
            errors.append("create_organization.go: provisionTenant 未同时写入身份与主拓扑（疑似回退为只建组织）")

    delete_order = [
        "Tenants().DecommissionIdentity",
        "Tenants().DecommissionTopologies",
        "Organizations().Delete",
        "embedAuditEvent",
    ]
    last_pos = -1
    for symbol in delete_order:
        pos = delete.find(symbol)
        if pos < 0:
            errors.append(f"delete_organization.go: 缺少接线符号 {symbol}")
            continue
        if pos < last_pos:
            errors.append(f"delete_organization.go: {symbol} 顺序回退（应保持 {delete_order} 顺序）")
        last_pos = max(last_pos, pos)

    if re.search(r"\bDELETE\s+FROM\b", delete, re.IGNORECASE):
        errors.append("delete_organization.go: 出现物理删除 DELETE FROM（退役只允许软删 + 状态流转）")

    if errors:
        for item in errors:
            print(f"FAIL {item}")
        print(f"FAIL tenant-provisioning: {len(errors)} 处问题")
        return 1

    print("PASS tenant-provisioning: create/delete 接线与顺序符合 tenant-isolation.yaml")
    return 0


if __name__ == "__main__":
    sys.exit(main())
