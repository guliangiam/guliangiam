#!/usr/bin/env python3
"""Provider kind routing gate.

Why this exists
---------------
`identityKinds` used to be a flat catalog: 30 of its values had no consumer at
all, so creating a provider row for them changed nothing. L1 moves the closed
set to a routed model: every kind maps to one subsystem, and a kind stays
`routable: false` until its consumer lands.

This gate keeps these surfaces equal:
  1. `contracts/registries/provider-kinds.yaml` — the single mapping source;
  2. `identityKinds` in `internal/app/providers/kind.go` — the API closed set;
  3. the pending runtime routes in `internal/app/providers/kind_routes.go`;
  4. the messaging kind -> channel table in
     `internal/domain/providers/messaging.go` (B1 consumer, routable=true);
  5. `delegatedIdentityKinds` in `internal/app/providers/delegated.go` —
     kinds whose runtime consumer is a dedicated subsystem table, mirrored by
     the registry's `managed_via` / `consumer_table` fields.

It fails when a kind is missing/duplicated, when a pending kind loses its batch,
when the runtime map and the registry disagree, when the registry and the domain
messaging table disagree about which kinds messaging routes, or when the
delegated kind -> management endpoint map and the registry disagree (missing,
extra or mismatched ownership).
"""

import re
import sys
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parent.parent
KIND_GO = ROOT / "internal/app/providers/kind.go"
ROUTES_GO = ROOT / "internal/app/providers/kind_routes.go"
DOMAIN_MESSAGING_GO = ROOT / "internal/domain/providers/messaging.go"
DELEGATED_GO = ROOT / "internal/app/providers/delegated.go"
GOTH_GO = ROOT / "internal/infra/idp/goth.go"
REGISTRY = ROOT / "contracts/registries/provider-kinds.yaml"

SUBSYSTEMS = {"messaging", "storage", "captcha", "faceid", "scan", "logs", "legacy"}
BATCHES = {"B1", "B2", "B3", "B4", "B5", "B6", "C6", "P5", "P6", "P7", "prior"}

# managed_via -> the dedicated table the runtime reads. A managed kind must
# carry both fields and the table must match the endpoint that owns it.
MANAGED_BY_TABLES = {
    "/api/v1/ldaps": "ldaps",
    "/api/v1/captcha-providers": "captcha_providers",
    "/api/v1/face-id-providers": "face_id_providers",
}


def fail(message: str) -> None:
    print(f"FAIL: {message}")
    sys.exit(1)


def parse_identity_kinds() -> list[str]:
    content = KIND_GO.read_text(encoding="utf-8")
    block = re.search(r"var identityKinds = \[\]string\{(.*?)\n\}", content, re.S)
    if not block:
        fail("cannot locate identityKinds in kind.go")
    constants = load_constants()
    body = re.sub(r"//[^\n]*", "", block.group(1))
    tokens = [token.strip() for token in body.split(",") if token.strip()]
    return [resolve(token, constants) for token in tokens]


CONST_RE = re.compile(r"^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*\"([^\"]+)\"", re.M)


def load_constants() -> dict[str, str]:
    constants: dict[str, str] = {}
    for path in (KIND_GO, ROUTES_GO, DELEGATED_GO):
        for name, value in CONST_RE.findall(path.read_text(encoding="utf-8")):
            constants[name] = value
    return constants


def resolve(token: str, constants: dict[str, str]) -> str:
    token = token.strip().strip('"')
    return constants.get(token, token)


def parse_pending_routes() -> dict[str, dict]:
    content = ROUTES_GO.read_text(encoding="utf-8")
    constants = load_constants()
    pattern = re.compile(
        r"\{Kind:\s*([^,]+),\s*Subsystem:\s*([^,]+),\s*Batch:\s*\"([^\"]+)\",\s*Routable:\s*(true|false)\}"
    )
    routes = {}
    for kind_token, subsystem_token, batch, routable in pattern.findall(content):
        kind = resolve(kind_token, constants)
        subsystem = resolve(subsystem_token, constants)
        if kind in routes:
            fail(f"duplicate runtime route: {kind}")
        routes[kind] = {
            "subsystem": subsystem,
            "batch": batch,
            "routable": routable == "true",
        }
    if not routes:
        fail("no pending routes parsed from kind_routes.go")
    return routes


def parse_delegated_kinds() -> dict[str, str]:
    if not DELEGATED_GO.exists():
        fail("missing internal/app/providers/delegated.go")
    content = DELEGATED_GO.read_text(encoding="utf-8")
    block = re.search(
        r"var delegatedIdentityKinds = map\[domain\.Kind\]string\{(.*?)\n\}",
        content,
        re.S,
    )
    if not block:
        fail("cannot locate delegatedIdentityKinds in delegated.go")
    constants = load_constants()
    body = re.sub(r"//[^\n]*", "", block.group(1))
    mapping: dict[str, str] = {}
    pattern = re.compile(r'"([^"]+)"\s*:\s*([A-Za-z_][A-Za-z0-9_]*|"[^"]*")')
    for kind, value in pattern.findall(body):
        if kind in mapping:
            fail(f"delegatedIdentityKinds contains duplicate kind: {kind}")
        mapping[kind] = resolve(value, constants)
    if not mapping:
        fail("delegatedIdentityKinds is empty")
    return mapping


def parse_domain_messaging_kinds() -> set[str]:
    content = DOMAIN_MESSAGING_GO.read_text(encoding="utf-8")
    block = re.search(r"var messagingKindRoutes = \[\]MessagingKindRoute\{(.*?)\n\}", content, re.S)
    if not block:
        fail("cannot locate messagingKindRoutes in domain/providers/messaging.go")
    return set(re.findall(r'\{Kind:\s*"([^"]+)"', block.group(1)))


def parse_string_list(path: Path, anchor: str) -> set[str]:
    content = path.read_text(encoding="utf-8")
    block = re.search(anchor + r"(.*?)\n\}", content, re.S)
    if not block:
        fail(f"cannot locate {anchor} in {path.relative_to(ROOT)}")
    return set(re.findall(r'"([^"]+)"', block.group(1)))


def main() -> None:
    data = yaml.safe_load(REGISTRY.read_text(encoding="utf-8")) or {}
    entries = data.get("kinds") or []
    if not entries:
        fail("provider-kinds.yaml has no kinds entries")

    identity_kinds = parse_identity_kinds()
    if len(set(identity_kinds)) != len(identity_kinds):
        fail("identityKinds contains duplicates")

    by_kind: dict[str, dict] = {}
    for entry in entries:
        kind = entry.get("kind")
        if not kind:
            fail("provider-kinds entry is missing kind")
        if kind in by_kind:
            fail(f"duplicate registry entry: {kind}")
        for field in ("subsystem", "batch", "routable"):
            if field not in entry:
                fail(f"{kind}: missing {field}")
        if entry["subsystem"] not in SUBSYSTEMS:
            fail(f"{kind}: unknown subsystem {entry['subsystem']}")
        if entry["batch"] not in BATCHES:
            fail(f"{kind}: unknown batch {entry['batch']}")

        routable = bool(entry["routable"])
        if not routable and entry["batch"] == "prior":
            fail(f"{kind}: routable=false must name a future batch")
        if routable and entry["batch"] == "prior" and entry["subsystem"] != "legacy":
            fail(f"{kind}: routable prior kinds must use subsystem=legacy")

        by_kind[kind] = entry

    missing = sorted(set(identity_kinds) - set(by_kind))
    extra = sorted(set(by_kind) - set(identity_kinds))
    if missing:
        fail(f"registry missing identity kinds: {missing}")
    if extra:
        fail(f"registry contains unknown kinds: {extra}")

    # Delegated ownership: the Go map and the registry managed_via/consumer_table
    # fields must be a one-to-one mirror. Omission on both sides is the only
    # allowed "not managed" representation.
    delegated = parse_delegated_kinds()
    registry_managed = {
        kind: entry for kind, entry in by_kind.items() if entry.get("managed_via")
    }
    missing_managed = sorted(set(delegated) - set(registry_managed))
    extra_managed = sorted(set(registry_managed) - set(delegated))
    if missing_managed:
        fail(f"registry missing managed_via for delegated kinds: {missing_managed}")
    if extra_managed:
        fail(f"registry managed_via has no delegatedIdentityKinds entry: {extra_managed}")
    for kind, managed_via in sorted(delegated.items()):
        if managed_via not in MANAGED_BY_TABLES:
            fail(f"{kind}: unknown managed_via endpoint {managed_via}")
        entry = by_kind[kind]
        if entry.get("managed_via") != managed_via:
            fail(
                f"{kind}: managed_via mismatch "
                f"(go={managed_via}, registry={entry.get('managed_via')})"
            )
        expected_table = MANAGED_BY_TABLES[managed_via]
        if entry.get("consumer_table") != expected_table:
            fail(
                f"{kind}: consumer_table mismatch "
                f"(managed_via={managed_via} expects {expected_table}, "
                f"registry={entry.get('consumer_table')})"
            )
    # Unified-provider IDV kinds have no delegated provider table: the
    # providers row is owned by /api/v1/providers (so they must stay out of
    # delegatedIdentityKinds), while their session/result rows live in
    # identity_verifications. consumer_table without managed_via is therefore
    # expected and truthful for exactly these kinds.
    consumer_table_without_managed = {
        "jumio": "identity_verifications",
        "aliyun_idv": "identity_verifications",
        "tencent_idv": "identity_verifications",
        "onfido": "identity_verifications",
        "veriff": "identity_verifications",
        "persona": "identity_verifications",
    }
    for kind, entry in by_kind.items():
        if entry.get("consumer_table") and not entry.get("managed_via"):
            if consumer_table_without_managed.get(kind) != entry.get("consumer_table"):
                fail(f"{kind}: consumer_table requires managed_via")

    pending = {kind: entry for kind, entry in by_kind.items() if not entry["routable"]}
    runtime = parse_pending_routes()
    if set(pending) != set(runtime):
        fail(
            "pending registry != runtime routes: "
            f"registry-only={sorted(set(pending) - set(runtime))}, "
            f"runtime-only={sorted(set(runtime) - set(pending))}"
        )
    for kind, entry in pending.items():
        if entry["subsystem"] != runtime[kind]["subsystem"] or entry["batch"] != runtime[kind]["batch"]:
            fail(f"{kind}: registry and runtime route disagree")

    messaging_entries = {
        kind: entry for kind, entry in by_kind.items() if entry["subsystem"] == "messaging"
    }
    domain_messaging = parse_domain_messaging_kinds()
    if set(messaging_entries) != domain_messaging:
        fail(
            "messaging registry != domain channel table: "
            f"registry-only={sorted(set(messaging_entries) - domain_messaging)}, "
            f"domain-only={sorted(domain_messaging - set(messaging_entries))}"
        )
    not_routable = sorted(kind for kind, entry in messaging_entries.items() if not entry["routable"])
    if not_routable:
        fail(f"messaging kinds must be routable after B1 wiring: {not_routable}")

    # Goth catalog: app list == idp switch list == registry goth_types, and no
    # goth type may also own a dedicated closed-set kind (that would split the
    # route between kind=goth and the kind's own consumer).
    goth_app = parse_string_list(KIND_GO, r"var gothProviderTypes = \[\]string\{")
    goth_idp = parse_string_list(GOTH_GO, r"func SupportedGothTypes\(\) \[\]string \{\n\treturn \[\]string\{")
    goth_registry = set(data.get("goth_types") or [])
    if not goth_registry:
        fail("provider-kinds.yaml has no goth_types entries")
    if goth_app != goth_idp:
        fail(f"goth catalog app != idp: app-only={sorted(goth_app - goth_idp)}, idp-only={sorted(goth_idp - goth_app)}")
    if goth_app != goth_registry:
        fail(
            "goth catalog != registry: "
            f"code-only={sorted(goth_app - goth_registry)}, registry-only={sorted(goth_registry - goth_app)}"
        )
    overlap = sorted(goth_registry & set(identity_kinds))
    if overlap:
        fail(f"goth types must route via kind=goth, not a dedicated kind: {overlap}")

    print(
        f"PASS: provider kind routing complete "
        f"({len(identity_kinds)} identity kinds, {len(delegated)} delegated kinds, "
        f"{len(pending)} pending across "
        f"{len({e['subsystem'] for e in pending.values()})} subsystems, "
        f"{len(domain_messaging)} routable messaging kinds, "
        f"{len(goth_registry)} goth types)"
    )


if __name__ == "__main__":
    main()
