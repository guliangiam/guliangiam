#!/usr/bin/env python3
"""Reject drift between the encrypted-column registry and the repositories.

Why this exists
---------------
`internal/platform/crypto/reencrypt_job.go` and
`internal/platform/crypto/sealed_payloads.go` declare which columns are
encrypted at rest. Three failure modes are invisible to compile and unit
tests:

  - A repository stops using `NewEncryptedFieldMapper()` while its column stays
    in the registry (rotation then "re-encrypts" plaintext, or a new write path
    stores plaintext again).
  - A registry entry names a table/column that no longer exists in the
    declarative Schema, so rotation silently scans nothing.
  - A `_sealed` column is written with EncryptField (v1, no AAD) but is absent
    from the sealed-payload registry, so key rotation never reaches it and
    `-verify` never reports plaintext/unrecognized payloads.

The provider credential columns add a fourth invariant: every column listed in
`ProviderCredentialColumns` (the `migrate encrypt-provider-secrets` input) must
also be in `DefaultEncryptedColumns`, otherwise a later key rotation skips the
very columns the backfill just encrypted.

The sealed-payload columns are a separate registry on purpose: they are v1
(no AAD) and must never be rotated through the AAD-bound v2 path. Every schema
column ending in `_sealed` must therefore appear in exactly one of the three
registries, and sealed entries must not also appear in the v2 registries.

`LegacySealedColumns` (A-6.3B) maps a legacy plaintext column to its `_sealed`
target: each target must also be in `DefaultEncryptedColumns`, both sides must
resolve in the Schema, targets must be unique, and a `_sealed` column whose base
exists but has no mapping is reported as an orphan target.

Usage: python3 scripts/check-encrypted-columns.py
Exit code 0 = registry, schema and mappers agree; 1 = drift found.

输入：internal/platform/crypto/{reencrypt_job,sealed_payloads}.go、contracts/schema/*.yaml、
      internal/infra/{postgres,mysql} 下全部 EncryptedFieldMapper 包
输出：PASS/FAIL + 漂移项列表
--check 行为：无 --check；失败即 exit 1
消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
"""

from __future__ import annotations

import glob
import re
import sys
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parent.parent
REGISTRY = ROOT / "internal" / "platform" / "crypto" / "reencrypt_job.go"
SEALED_REGISTRY = ROOT / "internal" / "platform" / "crypto" / "sealed_payloads.go"
LEGACY_REGISTRY = ROOT / "internal" / "platform" / "crypto" / "legacy_sealed_columns.go"
SCHEMA_DIR = ROOT / "contracts" / "schema"

# Registry variable name -> slice element type. The sealed payloads are a
# distinct type so a v1 value can never be fed to the AAD-bound rotation path.
REGISTRY_VARS = (
    ("DefaultEncryptedColumns", "EncryptedColumn"),
    ("ProviderCredentialColumns", "EncryptedColumn"),
    ("SealedPayloadColumns", "SealedPayloadColumn"),
)

# Every package that applies envelope encryption through
# shared.NewEncryptedFieldMapper (or the mysql shared alias).
MAPPER_DIRS = [
    "internal/infra/mysql/adapters",
    "internal/infra/mysql/agents",
    "internal/infra/mysql/captchaproviders",
    "internal/infra/mysql/certs",
    "internal/infra/mysql/faceidproviders",
    "internal/infra/mysql/idv",
    "internal/infra/mysql/keys",
    "internal/infra/mysql/ldaps",
    "internal/infra/mysql/mcpservers",
    "internal/infra/mysql/notificationproviders",
    "internal/infra/mysql/providers",
    "internal/infra/mysql/radiusconfigs",
    "internal/infra/mysql/shared",
    "internal/infra/mysql/syncers",
    "internal/infra/mysql/users",
    "internal/infra/mysql/webhooks",
    "internal/infra/postgres/adapters",
    "internal/infra/postgres/agents",
    "internal/infra/postgres/captchaproviders",
    "internal/infra/postgres/certs",
    "internal/infra/postgres/faceidproviders",
    "internal/infra/postgres/idv",
    "internal/infra/postgres/keys",
    "internal/infra/postgres/ldaps",
    "internal/infra/postgres/mcpservers",
    "internal/infra/postgres/notificationproviders",
    "internal/infra/postgres/organizations",
    "internal/infra/postgres/providers",
    "internal/infra/postgres/radiusconfigs",
    "internal/infra/postgres/shared",
    "internal/infra/postgres/syncers",
    "internal/infra/postgres/users",
    "internal/infra/postgres/webhooks",
]

# The shared packages only re-expose the process-wide encryptor; they own no
# table and need no registry entry.
SHARED_MAPPER_PACKAGES = {"shared"}

# Mapper package name -> encrypted table it persists, for packages whose name
# does not match the table (organizations stores its secrets in
# guliangiam.organization_auth_config; users stores its MFA secret in the
# user_mfa_config sub-table, not in guliangiam.users).
MAPPER_PACKAGE_TABLES = {
    "idv": "identity_verifications",
    "organizations": "organization_auth_config",
    "users": "user_mfa_config",
}

CONST = re.compile(r'^const\s+(\w+)\s*=\s*"([^"]+)"', re.MULTILINE)
ENTRY = re.compile(
    r'\{Table:\s*(?:"([^"]+)"|(\w+)),\s*Column:\s*"([^"]+)",\s*PKColumn:\s*(?:"([^"]+)"|(\w+))\s*\}'
)
# LegacySealedColumn has a different shape (source -> target), so it cannot
# reuse ENTRY; separate parsers also keep the v2/v1 registries from merging.
LEGACY_ENTRY = re.compile(
    r'\{Table:\s*(?:"([^"]+)"|(\w+)),\s*SourceColumn:\s*"([^"]+)",\s*TargetColumn:\s*"([^"]+)",'
    r'\s*PKColumn:\s*(?:"([^"]+)"|(\w+))\s*\}'
)


def fail(message: str) -> int:
    print(f"FAIL: {message}", file=sys.stderr)
    return 1


def parse_registry(source: str) -> dict[str, list[dict[str, str]]]:
    constants = {name: value for name, value in CONST.findall(source)}
    registries: dict[str, list[dict[str, str]]] = {}
    for name, element_type in REGISTRY_VARS:
        block = re.search(rf"var\s+{name}\s*=\s*\[\]{element_type}\s*\{{(.*?)\n\}}", source, re.S)
        if not block:
            raise ValueError(f"registry variable {name} not found in the crypto registry")
        entries = []
        raw_entries = block.group(1).count("{Table:")
        for quoted_table, identifier_table, column, quoted_pk, identifier_pk in ENTRY.findall(block.group(1)):
            table = quoted_table or constants.get(identifier_table)
            if not table:
                raise ValueError(f"{name}: cannot resolve table constant {identifier_table!r}")
            pk = quoted_pk or constants.get(identifier_pk)
            if not pk:
                raise ValueError(f"{name}: cannot resolve PK constant {identifier_pk!r}")
            entries.append({"table": table, "column": column, "pk": pk})
        if raw_entries != len(entries):
            raise ValueError(f"{name}: parsed {len(entries)} of {raw_entries} entries; registry format drifted")
        registries[name] = entries
    return registries


def parse_legacy_registry(source: str) -> list[dict[str, str]]:
    """Parse LegacySealedColumns (source plaintext -> target sealed column)."""
    constants = {name: value for name, value in CONST.findall(source)}
    block = re.search(r"var\s+LegacySealedColumns\s*=\s*\[\]LegacySealedColumn\s*\{(.*?)\n\}", source, re.S)
    if not block:
        raise ValueError("registry variable LegacySealedColumns not found in the crypto registry")
    entries = []
    for quoted_table, identifier_table, source_column, target_column, quoted_pk, identifier_pk in LEGACY_ENTRY.findall(
        block.group(1)
    ):
        table = quoted_table or constants.get(identifier_table)
        pk = quoted_pk or constants.get(identifier_pk)
        if not table or not pk:
            raise ValueError("LegacySealedColumns: cannot resolve a table/PK constant")
        entries.append({"table": table, "source": source_column, "target": target_column, "pk": pk})
    if block.group(1).count("{Table:") != len(entries):
        raise ValueError("LegacySealedColumns: parsed entry count differs from the declared entries")
    return entries


def load_schema() -> dict[str, set[str]]:
    tables: dict[str, set[str]] = {}
    for path in sorted(glob.glob(str(SCHEMA_DIR / "*.yaml"))):
        with open(path, encoding="utf-8") as handle:
            for doc in yaml.safe_load_all(handle):
                if not doc or not doc.get("table_name"):
                    continue
                short = doc["table_name"].split(".")[-1]
                tables[short] = {col["name"] for col in doc.get("columns") or []}
    return tables


def normalize_table_name(name: str) -> str:
    """Compare mapper package names with table names ignoring separators."""
    return name.replace("_", "").lower()


def main() -> int:
    try:
        source = REGISTRY.read_text(encoding="utf-8") + "\n" + SEALED_REGISTRY.read_text(encoding="utf-8")
        registry = parse_registry(source)
        legacy_entries = parse_legacy_registry(
            source + "\n" + LEGACY_REGISTRY.read_text(encoding="utf-8")
        )
    except (OSError, ValueError) as exc:
        return fail(str(exc))

    default_entries = registry["DefaultEncryptedColumns"]
    backfill_entries = registry["ProviderCredentialColumns"]
    sealed_entries = registry["SealedPayloadColumns"]
    if not backfill_entries:
        return fail("ProviderCredentialColumns is empty; the backfill would touch nothing")
    if not sealed_entries:
        return fail("SealedPayloadColumns is empty; sealed payload rotation would touch nothing")
    if not legacy_entries:
        return fail("LegacySealedColumns is empty; the PII sealed-column backfill would touch nothing")

    schema = load_schema()
    if not schema:
        return fail("no tables loaded from contracts/schema/*.yaml")

    violations: list[str] = []

    default_keys = {f"{item['table']}.{item['column']}" for item in default_entries}
    for item in backfill_entries:
        key = f"{item['table']}.{item['column']}"
        if key not in default_keys:
            violations.append(f"{key} is backfilled but missing from DefaultEncryptedColumns (key rotation would skip it)")

    sealed_keys = {f"{item['table']}.{item['column']}" for item in sealed_entries}
    for key in sorted(sealed_keys):
        if key in default_keys:
            violations.append(
                f"{key} is a sealed payload but also in DefaultEncryptedColumns; "
                "rotation would upgrade it to v2 and DecryptField would reject it"
            )
        if any(key == f"{item['table']}.{item['column']}" for item in backfill_entries):
            violations.append(
                f"{key} is a sealed payload but also in ProviderCredentialColumns; "
                "the provider backfill would seal it as v2"
            )

    # A-6.3B: legacy plaintext -> `_sealed` target. Every target must be a
    # registered v2 column, both columns must resolve in the schema, and a
    # target may appear only once so two sources cannot race for it.
    legacy_targets: dict[str, str] = {}
    for item in legacy_entries:
        short, full = item["table"].split(".")[-1], item["table"]
        source_key = f"{full}.{item['source']}"
        target_key = f"{full}.{item['target']}"
        if not item["source"]:
            violations.append(f"{target_key} has no source column (orphan target)")
            continue
        if short not in schema:
            violations.append(f"LegacySealedColumns: table {full} does not exist in contracts/schema/*.yaml")
            continue
        for kind, column in (("source", item["source"]), ("target", item["target"])):
            if column not in schema[short]:
                violations.append(
                    f"LegacySealedColumns: {kind} column {full}.{column} does not exist in contracts/schema/*.yaml"
                )
        if item["pk"] not in schema[short]:
            violations.append(f"LegacySealedColumns: PK {full}.{item['pk']} does not exist in contracts/schema/*.yaml")
        if target_key not in default_keys:
            violations.append(f"{target_key} is a PII backfill target but missing from DefaultEncryptedColumns")
        if target_key in sealed_keys:
            violations.append(f"{target_key} is a PII backfill target but also a v1 sealed payload")
        if target_key in legacy_targets:
            violations.append(f"{target_key} has multiple sources ({legacy_targets[target_key]} and {source_key})")
        else:
            legacy_targets[target_key] = source_key

    # Orphan target: a registered `_sealed` column whose same-table base column
    # exists but has no mapping, so no command would ever fill it.
    for table, columns in sorted(schema.items()):
        for column in sorted(columns):
            if not column.endswith("_sealed"):
                continue
            target_key = f"guliangiam.{table}.{column}"
            base = column[: -len("_sealed")]
            if target_key in default_keys and base in columns and target_key not in legacy_targets:
                violations.append(f"{target_key} has legacy column {table}.{base} but no LegacySealedColumns mapping")

    for name, entries in registry.items():
        for item in entries:
            table = item["table"].split(".")[-1]
            if table not in schema:
                violations.append(f"{name}: table {item['table']} does not exist in contracts/schema/*.yaml")
            elif item["column"] not in schema[table]:
                violations.append(f"{name}: column {item['table']}.{item['column']} does not exist in contracts/schema/*.yaml")
            elif item["pk"] not in schema[table]:
                violations.append(f"{name}: PK column {item['table']}.{item['pk']} does not exist in contracts/schema/*.yaml")

    # Every `_sealed` column in the declarative schema must be registered in one
    # of the three lists; a new v1 sealed config otherwise escapes rotation and
    # `-verify` forever (the GM R10 gap).
    registered_keys = {
        f"{item['table'].split('.')[-1]}.{item['column']}"
        for entries in registry.values()
        for item in entries
    }
    for table, columns in sorted(schema.items()):
        for column in sorted(columns):
            if column.endswith("_sealed") and f"{table}.{column}" not in registered_keys:
                violations.append(
                    f"{table}.{column} ends with _sealed but is missing from the default/provider/sealed-payload registries"
                )

    default_tables = {item["table"].split(".")[-1] for item in default_entries}

    mapper_hits = 0
    for rel_dir in MAPPER_DIRS:
        dir_path = ROOT / rel_dir
        files = sorted(path for path in dir_path.glob("*.go") if not path.name.endswith("_test.go"))
        if not files:
            violations.append(f"{rel_dir}: no Go sources found")
            continue
        if not any("NewEncryptedFieldMapper" in path.read_text(encoding="utf-8") for path in files):
            violations.append(f"{rel_dir}: no source references NewEncryptedFieldMapper")
            continue
        mapper_hits += 1
        package = Path(rel_dir).name
        if package in SHARED_MAPPER_PACKAGES:
            continue
        expected_table = MAPPER_PACKAGE_TABLES.get(package, package)
        if not any(
            normalize_table_name(table) == normalize_table_name(expected_table)
            for table in default_tables
        ):
            violations.append(
                f"{rel_dir}: mapper package has no column in DefaultEncryptedColumns "
                "(key rotation would skip it)"
            )

    if violations:
        print(f"Encrypted-column registry FAILED ({len(violations)} issue(s)):")
        for violation in violations:
            print(f"  - {violation}")
        return 1

    print(
        "Encrypted-column registry OK: "
        f"{len(default_entries)} default entries, "
        f"{len(backfill_entries)} provider backfill entries, "
        f"{len(sealed_entries)} sealed payload entries, "
        f"{len(legacy_entries)} PII sealed-column mappings, "
        f"{mapper_hits} mapper package(s) wired, schema columns resolve"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
