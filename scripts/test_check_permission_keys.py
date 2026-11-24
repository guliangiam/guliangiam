"""check-permission-keys.py 的单元测试 — 注册表派生、CRUD alias、动态构造与共享槽。

输入：check-permission-keys 模块 + 构造的 TSX/registry 片段
输出：unittest 通过/失败
--check 行为：无 --check；单元测试
消费方：独立测试文件；`python3 scripts/test_check_permission_keys.py`
"""

from __future__ import annotations

import importlib.util
import unittest
from pathlib import Path

_MODULE_PATH = Path(__file__).resolve().with_name("check-permission-keys.py")
_SPEC = importlib.util.spec_from_file_location("check_permission_keys", _MODULE_PATH)
assert _SPEC is not None and _SPEC.loader is not None
check_permissions = importlib.util.module_from_spec(_SPEC)
_SPEC.loader.exec_module(check_permissions)

SAMPLE_REGISTRY = """
iam:
  adapters:
    actions: [read, write, delete]
    casbin_resource: "adapters"
    frontend_prefix: "adapter"
  forms:
    actions: [read, write]
  entries:
    actions: [read, write]
  syncers:
    actions: [read, write, admin, run]
payments:
  pay-recon:
    actions: [write]
    casbin_resource: "pay-recon"
    frontend_prefix: "pay-recon"
aigateway:
  api_keys:
    actions: [read, write, delete]
    casbin_resource: "aigateway:api_keys"
    frontend_prefix: "aigateway-api-key"
"""


def scan(text: str) -> tuple[list, list]:
    return check_permissions.scan_source(text, "sample.tsx")


def validate(text: str, registry=None, allowlist=None):
    usages, _undecidable = scan(text)
    return check_permissions.validate_usages(
        usages,
        registry if registry is not None else check_permissions.parse_registry(SAMPLE_REGISTRY),
        allowlist or [],
    )


def allow(kind: str, key: str, location: str = "sample.tsx:1", reason: str = "测试存量"):
    return check_permissions.AllowlistEntry(kind, key, location, reason)


class RegistryParseTests(unittest.TestCase):
    def setUp(self) -> None:
        self.registry = check_permissions.parse_registry(SAMPLE_REGISTRY)

    def test_derives_frontend_prefix_for_forms(self) -> None:
        self.assertIn("form:read", self.registry.action_keys)
        self.assertTrue(
            check_permissions.action_is_registered("form:read", self.registry)
        )

    def test_derives_frontend_resource_from_colon_casbin_resource(self) -> None:
        self.assertIn("aigateway:api_keys", self.registry.resources)
        self.assertIn("aigateway-api-keys", self.registry.resources)
        self.assertTrue(
            check_permissions.resource_is_registered(
                "aigateway-api-keys", self.registry
            )
        )

    def test_registry_actions_include_quoted_scalars(self) -> None:
        self.assertIn("adapter:write", self.registry.action_keys)
        self.assertIn("pay-recon:write", self.registry.action_keys)

    def test_missing_actions_is_a_parse_error(self) -> None:
        with self.assertRaises(check_permissions.RegistryParseError):
            check_permissions.parse_registry("iam:\n  broken:\n    scenario: x\n")


class ActionValidationTests(unittest.TestCase):
    def test_registered_action_passes(self) -> None:
        result = validate(
            '<PermissionGuard action="adapter:write" resource="adapters" fallback={null}>'
        )
        self.assertEqual([], result.violations)

    def test_unregistered_action_fails(self) -> None:
        result = validate('<PermissionGuard action="user:write">')
        self.assertEqual(1, len(result.violations))
        self.assertEqual("action", result.violations[0].kind)
        self.assertEqual("user:write", result.violations[0].key)

    def test_crud_aliases_pass(self) -> None:
        for action in ("get", "head", "post", "put", "patch", "delete"):
            with self.subTest(action=action):
                result = validate(f'<PermissionGuard action="adapter:{action}">')
                self.assertEqual([], result.violations, action)

    def test_non_colon_action_fails(self) -> None:
        result = validate('<PermissionGuard action="view-dashboard">')
        self.assertEqual(1, len(result.violations))
        self.assertEqual("action", result.violations[0].kind)


class ScanSurfaceTests(unittest.TestCase):
    def test_jsx_slots_and_helpers_are_collected(self) -> None:
        usages, _ = scan(
            '<PermissionGuard action="adapter:write" resource="adapters" fallback={null}>\n'
            'bulkDeletePermission: { permission: "adapter:delete", resource: "adapters" },\n'
            'submitPermission={{ permission: "adapter:write", resource: "adapters" }}\n'
            'const allowed = hasAction("adapter:read");\n'
            'const visible = hasResource("adapters");\n'
        )
        kinds = [usage.kind for usage in usages]
        self.assertEqual(
            [
                "PermissionGuard",
                "bulkDeletePermission",
                "submitPermission",
                "hasAction",
                "hasResource",
            ],
            kinds,
        )

    def test_multiline_shared_slot_is_collected(self) -> None:
        usages, _ = scan(
            "bulkDeletePermission: {\n"
            '  permission: "aigateway-api-key:delete",\n'
            '  resource: "aigateway-api-keys",\n'
            "},\n"
        )
        self.assertEqual(1, len(usages))
        self.assertEqual("aigateway-api-key:delete", usages[0].action)
        self.assertEqual("aigateway-api-keys", usages[0].resource)

    def test_fallback_jsx_does_not_truncate_tag(self) -> None:
        usages, _ = scan(
            '<PermissionGuard action="adapter:write" fallback={<span>a > b</span>}>'
        )
        self.assertEqual(1, len(usages))
        self.assertEqual("adapter:write", usages[0].action)

    def test_comments_are_ignored(self) -> None:
        usages, undecidable = scan(
            '// <PermissionGuard action="user:write">\n'
            '/* <PermissionGuard action="user:delete"> */\n'
            'const ok = "not a usage";\n'
        )
        self.assertEqual([], usages)
        self.assertEqual([], undecidable)

    def test_string_expression_is_literal(self) -> None:
        usages, _ = scan('<PermissionGuard action={"adapter:write"} resource={"adapters"}>')
        self.assertEqual("adapter:write", usages[0].action)
        self.assertEqual("adapters", usages[0].resource)

    def test_dynamic_action_is_undecidable_not_a_failure(self) -> None:
        usages, undecidable = scan("<PermissionGuard action={permissionKey}>")
        self.assertEqual(1, len(usages))
        self.assertIsNone(usages[0].action)
        self.assertEqual(1, len(undecidable))
        result = validate("<PermissionGuard action={permissionKey}>")
        self.assertEqual([], result.violations)

    def test_template_interpolation_is_undecidable(self) -> None:
        _, undecidable = scan("<PermissionGuard action={`adapter:${verb}`}>")
        self.assertEqual(1, len(undecidable))
        self.assertIn("adapter:", undecidable[0].expression)


class ResourceValidationTests(unittest.TestCase):
    def test_shared_slot_resource_must_be_registered(self) -> None:
        result = validate(
            'submitPermission={{ permission: "adapter:write", resource: "mystery" }}'
        )
        self.assertEqual(1, len(result.violations))
        self.assertEqual("resource", result.violations[0].kind)
        self.assertEqual("mystery", result.violations[0].key)

    def test_shared_slot_missing_resource_fails(self) -> None:
        result = validate(
            'bulkDeletePermission: { permission: "adapter:delete" }'
        )
        self.assertEqual(1, len(result.violations))
        self.assertEqual("<missing>", result.violations[0].key)

    def test_derived_casbin_resource_passes(self) -> None:
        result = validate(
            'submitPermission={{ permission: "aigateway-api-key:write", '
            'resource: "aigateway-api-keys" }}'
        )
        self.assertEqual([], result.violations)

    def test_unknown_resource_on_inline_guard_fails(self) -> None:
        result = validate('<PermissionGuard action="adapter:read" resource="unknown">')
        self.assertEqual(1, len(result.violations))
        self.assertEqual("resource", result.violations[0].kind)


class AllowlistTests(unittest.TestCase):
    def test_allowlisted_action_suppresses_violation(self) -> None:
        result = check_permissions.validate_usages(
            scan('<PermissionGuard action="user:write">')[0],
            check_permissions.parse_registry(SAMPLE_REGISTRY),
            [allow("action", "user:write")],
        )
        self.assertEqual([], result.violations)
        self.assertEqual(1, result.allowed_instances)
        self.assertIn(("action", "user:write"), result.used_allowlist)

    def test_canonical_key_matches_allowlist_entry(self) -> None:
        result = check_permissions.validate_usages(
            scan('<PermissionGuard action="user:get">')[0],
            check_permissions.parse_registry(SAMPLE_REGISTRY),
            [allow("action", "user:read")],
        )
        self.assertEqual([], result.violations)
        self.assertIn(("action", "user:read"), result.used_allowlist)

    def test_stale_entry_is_detected(self) -> None:
        stale = check_permissions.stale_allowlist_entries(
            [allow("action", "user:write")], frozenset()
        )
        self.assertEqual(1, len(stale))
        self.assertEqual("user:write", stale[0].key)

    def test_malformed_allowlist_is_rejected(self) -> None:
        with self.assertRaises(check_permissions.RegistryParseError):
            check_permissions.parse_allowlist("action|user:write\n")

    def test_duplicate_allowlist_is_rejected(self) -> None:
        text = (
            "action|user:write|sample.tsx:1|r\n"
            "action|user:write|sample.tsx:2|r\n"
        )
        with self.assertRaises(check_permissions.RegistryParseError):
            check_permissions.parse_allowlist(text)


MIRROR_SAMPLE = """
const ADMIN_FULL_PERMISSIONS: UserPermissionSet = {
  menus: ["/"],
  actions: [
    "syncer:delete",
    "user:write",
    "adapter:read",
  ],
  resources: ["syncers", "adapters"],
};
"""


class MirrorTests(unittest.TestCase):
    def test_mirror_parser_and_registered_prefix_drift(self) -> None:
        mirror = check_permissions.parse_mirror(MIRROR_SAMPLE)
        self.assertEqual(
            ["syncer:delete", "user:write", "adapter:read"],
            [key for key, _line in mirror.actions],
        )
        self.assertEqual(
            ["syncers", "adapters"],
            [key for key, _line in mirror.resources],
        )

        drifts = check_permissions.mirror_drifts(
            mirror, check_permissions.parse_registry(SAMPLE_REGISTRY)
        )
        # user:write 的前缀未登记（G-2 覆盖缺口）不按镜像漂移处理；
        # syncer:delete 的 syncer 前缀已登记但 action 不在注册表 → 漂移。
        self.assertEqual(["syncer:delete"], [drift.key for drift in drifts])

    def test_page_key_missing_from_mirror_is_reported(self) -> None:
        mirror = check_permissions.parse_mirror(MIRROR_SAMPLE)
        usages, _ = scan('<PermissionGuard action="adapter:delete" resource="adapters">')
        violations = check_permissions.mirror_coverage_violations(usages, mirror)
        self.assertEqual(["adapter:delete"], [violation.key for violation in violations])


if __name__ == "__main__":
    unittest.main()
