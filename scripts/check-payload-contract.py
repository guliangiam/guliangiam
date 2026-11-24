#!/usr/bin/env python3
"""Static payload-key check: frontend request types ↔ Go request DTOs.

`route.DecodeJSON` uses DisallowUnknownFields, so a key the Go DTO does not
declare is a 400. `tsc` only checks page literals against the frontend payload
type, not the Go struct. This gate compares the two declarations for the
explicitly registered pairs below; page payloads are covered transitively by
`pnpm typecheck`. Frontend-only keys fail; backend-only keys are reported.

Usage: python3 scripts/check-payload-contract.py
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

PAIRS = [
    ("web/src/shared/api/iam/syncer.ts", "IamCreateSyncerPayload", "internal/transport/http/api/v1/syncers/handler.go", "CreateSyncerRequest"),
    ("web/src/shared/api/iam/syncer.ts", "IamPatchSyncerPayload", "internal/transport/http/api/v1/syncers/handler.go", "PatchSyncerRequest"),
    ("web/src/shared/api/iam/rule.ts", "IamCreateRulePayload", "internal/transport/http/api/v1/rules/request.go", "CreateRuleRequest"),
    ("web/src/shared/api/iam/rule.ts", "IamPatchRulePayload", "internal/transport/http/api/v1/rules/request.go", "PatchRuleRequest"),
    ("web/src/shared/api/iam/agent.ts", "IamCreateAgentPayload", "internal/transport/http/api/v1/agents/handler.go", "CreateAgentRequest"),
    ("web/src/shared/api/iam/agent.ts", "IamPatchAgentPayload", "internal/transport/http/api/v1/agents/handler.go", "PatchAgentRequest"),
    ("web/src/shared/api/iam/order.ts", "IamCreateOrderPayload", "internal/transport/http/api/v1/orders/handler.go", "CreateOrderRequest"),
    ("web/src/shared/api/iam/order.ts", "IamPatchOrderPayload", "internal/transport/http/api/v1/orders/handler.go", "PatchOrderRequest"),
]
INLINE_PAIRS = [
    ("web/src/shared/api/iam/product.ts", "IamBuyProductPayload", "internal/transport/http/api/v1/products/chi.go"),
]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def balanced_body(text: str, match: re.Match[str]) -> str:
    index, depth = match.end(), 1
    while depth:
        char = text[index]
        depth += 1 if char == "{" else -1 if char == "}" else 0
        index += 1
    return text[match.end() : index - 1]


def ts_object_keys(text: str, alias: str) -> set[str] | None:
    match = re.search(r"export\s+type\s+" + re.escape(alias) + r"\s*=\s*\{", text)
    if not match:
        return None
    keys: set[str] = set()
    depth = 0
    for line in balanced_body(text, match).splitlines():
        line = re.sub(r"//.*", "", line)
        if depth == 0:
            prop = re.match(r"\s*([A-Za-z_$][\w$]*)\s*\??\s*:", line)
            if prop:
                keys.add(prop.group(1))
        depth += line.count("{") - line.count("}")
    return keys


def go_struct_body(text: str, struct: str) -> str | None:
    match = re.search(r"type\s+" + re.escape(struct) + r"\s+struct\s*\{", text)
    return balanced_body(text, match) if match else None


def anonymous_struct_body(text: str) -> str | None:
    matches = list(re.finditer(r"var\s+req\s+struct\s*\{", text))
    return balanced_body(text, matches[0]) if len(matches) == 1 else None


def go_tags(body: str) -> set[str]:
    return set(re.findall(r'json:"([^",]+)', body))


def compare(label: str, ts: set[str], go: set[str]) -> list[str]:
    extra, missing = sorted(ts - go), sorted(go - ts)
    print(f"CMP  {label}\n     frontend-only: {extra or 'none'}\n     backend-only : {missing or 'none'}")
    return extra


def main() -> int:
    failures: list[str] = []
    for ts_file, alias, go_file, struct in PAIRS:
        ts, body = ts_object_keys(read(ts_file), alias), go_struct_body(read(go_file), struct)
        if ts is None or body is None:
            failures.append(f"unresolved pair: {alias} ↔ {struct}")
            continue
        failures.extend(
            f"{alias} sends {key!r}, absent from {struct}"
            for key in compare(f"{alias} ↔ {struct}", ts, go_tags(body))
        )

    for ts_file, alias, go_file in INLINE_PAIRS:
        ts, body = ts_object_keys(read(ts_file), alias), anonymous_struct_body(read(go_file))
        if ts is None or body is None:
            failures.append(f"unresolved inline pair: {alias} ↔ {go_file}")
            continue
        failures.extend(
            f"{alias} sends {key!r}, absent from {go_file}"
            for key in compare(f"{alias} ↔ anonymous request body", ts, go_tags(body))
        )

    if failures:
        print("FAIL: frontend payload declares keys the backend rejects", file=sys.stderr)
        for failure in failures:
            print(f"  - {failure}", file=sys.stderr)
        return 1
    print("PASS: every frontend payload key is a declared Go json tag")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
