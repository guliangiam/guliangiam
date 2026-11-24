"""
gen_schema.py 的单元测试 — 验证新增列等可加性变更支持

输入：gen_schema 模块 + 构造的 schema 片段
输出：pytest/unittest 测试通过/失败
--check 行为：无 --check；单元测试
消费方：独立测试文件；不纳入 verify-docs.sh
"""

from __future__ import annotations

import copy
import tempfile
import unittest
from pathlib import Path

from scripts import gen_schema


class UnsupportedMigrationChangesTests(unittest.TestCase):
    def test_additive_column_is_supported(self) -> None:
        previous = [{
            "table_name": "guliangiam.accounts",
            "pk": "id",
            "columns": [{"name": "id", "type": "UUID", "nullable": False}],
            "indexes": [],
        }]
        current = [{
            "table_name": "guliangiam.accounts",
            "pk": "id",
            "columns": [
                {"name": "id", "type": "UUID", "nullable": False},
                {"name": "display_name", "type": "TEXT", "nullable": True},
            ],
            "indexes": [],
        }]

        self.assertEqual([], gen_schema.unsupported_migration_changes(previous, current))

    def test_additive_index_is_supported(self) -> None:
        previous = [{
            "table_name": "guliangiam.accounts",
            "pk": "id",
            "columns": [{"name": "id", "type": "UUID", "nullable": False}],
            "indexes": [],
        }]
        current = [{
            "table_name": "guliangiam.accounts",
            "pk": "id",
            "columns": [{"name": "id", "type": "UUID", "nullable": False}],
            "indexes": [{"name": "idx_accounts_id", "columns": ["id"]}],
        }]

        self.assertEqual([], gen_schema.unsupported_migration_changes(previous, current))
        diff = gen_schema.diff_schemas(previous, current)
        self.assertEqual("idx_accounts_id", diff["added_indexes"]["accounts"]["added"][0]["name"])

    def test_additive_check_constraint_is_supported(self) -> None:
        previous = [{
            "table_name": "guliangiam.accounts",
            "pk": "id",
            "columns": [{"name": "id", "type": "UUID", "nullable": False}],
            "constraints": [],
        }]
        current = [{
            "table_name": "guliangiam.accounts",
            "pk": "id",
            "columns": [{"name": "id", "type": "UUID", "nullable": False}],
            "constraints": [{"name": "ck_accounts_id", "type": "check", "expr": "id IS NOT NULL"}],
        }]

        self.assertEqual([], gen_schema.unsupported_migration_changes(previous, current))
        diff = gen_schema.diff_schemas(previous, current)
        self.assertEqual("ck_accounts_id", diff["added_checks"]["accounts"]["added"][0]["name"])
        up = gen_schema.render_migration_up(diff, 12, 11, current, {}, None)
        down = gen_schema.render_migration_down(diff, 12, 11, None)
        self.assertIn("ADD CONSTRAINT ck_accounts_id CHECK (id IS NOT NULL)", up)
        self.assertIn("SELECT 1 FROM pg_constraint", up)
        self.assertIn("conrelid = 'guliangiam.accounts'::regclass", up)
        self.assertIn("END IF;", up)
        self.assertIn("DROP CONSTRAINT IF EXISTS ck_accounts_id", down)

    def test_replaced_check_constraint_is_supported_and_rendered(self) -> None:
        previous = [{
            "table_name": "guliangiam.providers",
            "pk": "id",
            "columns": [{"name": "id", "type": "UUID", "nullable": False}],
            "constraints": [{
                "name": "ck_providers_kind",
                "type": "check",
                "expr": "kind IN ('github', 'custom')",
            }],
        }]
        current = [{
            "table_name": "guliangiam.providers",
            "pk": "id",
            "columns": [{"name": "id", "type": "UUID", "nullable": False}],
            "constraints": [{
                "name": "ck_providers_kind",
                "type": "check",
                "expr": "kind IN ('github', 'custom', 'sms')",
            }],
        }]

        self.assertEqual([], gen_schema.unsupported_migration_changes(previous, current))
        diff = gen_schema.diff_schemas(previous, current)
        self.assertEqual("ck_providers_kind", diff["changed_checks"]["providers"]["replaced"][0]["name"])
        up = gen_schema.render_migration_up(diff, 2, 1, current, {}, None)
        down = gen_schema.render_migration_down(diff, 2, 1, None)
        # 正向：先 DROP 再 ADD 到新定义
        self.assertIn("DROP CONSTRAINT IF EXISTS ck_providers_kind", up)
        self.assertIn(
            "ADD CONSTRAINT ck_providers_kind CHECK (kind IN ('github', 'custom', 'sms'))", up
        )
        # 回滚：恢复旧定义
        self.assertIn(
            "ADD CONSTRAINT ck_providers_kind CHECK (kind IN ('github', 'custom'))", down
        )

    def test_non_check_constraint_addition_is_supported(self) -> None:
        previous = [{"table_name": "guliangiam.accounts", "columns": [], "constraints": []}]
        current = [{
            "table_name": "guliangiam.accounts",
            "columns": [],
            "constraints": [{"name": "uq_accounts_id", "type": "unique", "columns": ["id"]}],
        }]

        self.assertEqual([], gen_schema.unsupported_migration_changes(previous, current))
        diff = gen_schema.diff_schemas(previous, current)
        self.assertEqual("uq_accounts_id", diff["added_uniques"]["accounts"]["added"][0]["name"])
        up = gen_schema.render_migration_up(diff, 12, 11, current, {}, None)
        down = gen_schema.render_migration_down(diff, 12, 11, None)
        self.assertIn("ADD CONSTRAINT uq_accounts_id UNIQUE (id)", up)
        self.assertIn("SELECT 1 FROM pg_constraint", up)
        self.assertIn("conrelid = 'guliangiam.accounts'::regclass", up)
        self.assertIn("DROP CONSTRAINT IF EXISTS uq_accounts_id", down)

    def test_modified_deleted_or_reordered_existing_lists_are_rejected(self) -> None:
        previous = [{
            "table_name": "guliangiam.accounts",
            "columns": [],
            "constraints": [
                {"name": "ck_accounts_one", "type": "check", "expr": "id IS NOT NULL"},
                {"name": "ck_accounts_two", "type": "check", "expr": "id IS NOT NULL"},
            ],
            "indexes": [
                {"name": "idx_accounts_one", "columns": ["id"]},
                {"name": "idx_accounts_two", "columns": ["id"]},
            ],
            "fk_constraints": [
                {"name": "fk_accounts_one", "column": "id", "ref_table": "guliangiam.organizations"},
                {"name": "fk_accounts_two", "column": "id", "ref_table": "guliangiam.organizations"},
            ],
        }]
        modifications = {
            "constraints": ("expr", "id IS NULL"),
            "indexes": ("columns", ["other_id"]),
            "fk_constraints": ("on_delete", "CASCADE"),
        }

        for field in modifications:
            # 索引允许「纯删除」（见 test_pure_index_deletion_is_supported_and_rendered），
            # 但重排或以同名就地改定义仍判为不支持；CHECK 的「同名改定义」由
            # test_replaced_check_constraint_is_supported_and_rendered 覆盖（DROP+ADD）。
            if field == "indexes":
                operations = ("modified", "reordered")
            elif field == "constraints":
                # 约束按名称比对：重排不产生 DDL，视为无变更；删除仍不支持。
                operations = ("deleted",)
            else:
                operations = ("modified", "deleted", "reordered")
            for operation in operations:
                with self.subTest(field=field, operation=operation):
                    current = copy.deepcopy(previous)
                    items = current[0][field]
                    if operation == "modified":
                        key, value = modifications[field]
                        items[0][key] = value
                    elif operation == "deleted":
                        items.pop(0)
                    else:
                        items.reverse()

                    self.assertIn(
                        "accounts: primary key, indexes, constraints, foreign keys, RLS, "
                        "or other table metadata changed",
                        gen_schema.unsupported_migration_changes(previous, current),
                    )

    def test_pure_index_deletion_is_supported_and_rendered(self) -> None:
        previous = [{
            "table_name": "guliangiam.accounts",
            "pk": "id",
            "columns": [{"name": "id", "type": "UUID", "nullable": False}],
            "constraints": [
                {"name": "uq_accounts_user_id", "type": "unique", "columns": ["user_id"]},
            ],
            "indexes": [
                {"name": "idx_accounts_user_id", "columns": ["user_id"]},
                {"name": "idx_accounts_created_at", "columns": ["created_at"]},
            ],
        }]
        # 只保留被唯一约束最左前缀覆盖的那一个索引被删除，另一个索引原样保留。
        current = copy.deepcopy(previous)
        current[0]["indexes"] = [{"name": "idx_accounts_created_at", "columns": ["created_at"]}]

        self.assertEqual([], gen_schema.unsupported_migration_changes(previous, current))

        diff = gen_schema.diff_schemas(previous, current)
        self.assertEqual(
            "idx_accounts_user_id", diff["dropped_indexes"]["accounts"]["dropped"][0]["name"]
        )
        self.assertFalse(diff["added_indexes"])

        up = gen_schema.render_migration_up(diff, 14, 13, current, {}, None)
        down = gen_schema.render_migration_down(diff, 14, 13, None)
        self.assertIn("DROP INDEX IF EXISTS idx_accounts_user_id;", up)
        self.assertNotIn("idx_accounts_created_at", up)
        self.assertIn(
            "CREATE INDEX IF NOT EXISTS idx_accounts_user_id ON guliangiam.accounts (user_id)",
            down,
        )

    def test_comment_only_change_is_supported(self) -> None:
        previous = [{
            "table_name": "guliangiam.accounts",
            "pk": "id",
            "columns": [{"name": "id", "type": "UUID", "nullable": False, "comment": "old"}],
        }]
        current = [{
            "table_name": "guliangiam.accounts",
            "pk": "id",
            "columns": [{"name": "id", "type": "UUID", "nullable": False, "comment": "new"}],
        }]

        self.assertEqual([], gen_schema.unsupported_migration_changes(previous, current))


class AdditiveMigrationRenderingTests(unittest.TestCase):
    def test_mixed_additive_migration_up_and_down(self) -> None:
        previous = [
            {
                "table_name": "guliangiam.organizations",
                "pk": "id",
                "columns": [{"name": "id", "type": "UUID", "nullable": False}],
            },
            {
                "table_name": "guliangiam.accounts",
                "pk": "id",
                "columns": [{"name": "id", "type": "UUID", "nullable": False}],
                "constraints": [],
                "indexes": [],
                "fk_constraints": [],
            },
        ]
        current = copy.deepcopy(previous)
        account = current[1]
        account["columns"].extend([
            {"name": "external_id", "type": "TEXT", "nullable": True},
            {"name": "organization_id", "type": "UUID", "nullable": True},
        ])
        account["constraints"].extend([
            {
                "name": "ck_accounts_external_id",
                "type": "check",
                "expr": "external_id <> ''",
            },
            {
                "name": "uq_accounts_external_id",
                "type": "unique",
                "columns": ["external_id"],
            },
            {
                "name": "uq_accounts_active_external_id",
                "type": "unique_partial",
                "columns": ["external_id"],
                "predicate": "external_id <> ''",
            },
        ])
        account["indexes"].extend([
            {"name": "idx_accounts_organization_id", "columns": ["organization_id"]},
            {
                "name": "uq_accounts_organization_external_id",
                "columns": ["organization_id", "external_id"],
                "unique": True,
            },
        ])
        account["fk_constraints"].append({
            "column": "organization_id",
            "ref_table": "guliangiam.organizations",
            "ref_column": "id",
            "on_delete": "CASCADE",
        })

        self.assertEqual([], gen_schema.unsupported_migration_changes(previous, current))
        diff = gen_schema.diff_schemas(previous, current)
        self.assertEqual(1, len(diff["added_checks"]["accounts"]["added"]))
        self.assertEqual(1, len(diff["added_uniques"]["accounts"]["added"]))
        self.assertEqual(1, len(diff["added_unique_partials"]["accounts"]["added"]))
        self.assertEqual(2, len(diff["added_indexes"]["accounts"]["added"]))
        self.assertEqual(1, len(diff["added_fks"]["accounts"]["added"]))

        up = gen_schema.render_migration_up(diff, 13, 12, current, {}, None)
        down = gen_schema.render_migration_down(diff, 13, 12, None)
        self.assertEqual(3, up.count("SELECT 1 FROM pg_constraint"))

        up_fragments = [
            "ALTER TABLE guliangiam.accounts ADD COLUMN IF NOT EXISTS external_id TEXT;",
            "ADD CONSTRAINT ck_accounts_external_id CHECK (external_id <> '')",
            "ADD CONSTRAINT uq_accounts_external_id UNIQUE (external_id)",
            "CREATE UNIQUE INDEX IF NOT EXISTS uq_accounts_active_external_id "
            "ON guliangiam.accounts (external_id) WHERE external_id <> '';",
            "CREATE INDEX IF NOT EXISTS idx_accounts_organization_id "
            "ON guliangiam.accounts (organization_id);",
            "CREATE UNIQUE INDEX IF NOT EXISTS uq_accounts_organization_external_id "
            "ON guliangiam.accounts (organization_id, external_id);",
            "ADD CONSTRAINT fk_accounts_organization_id_organizations FOREIGN KEY "
            "(organization_id) REFERENCES guliangiam.organizations(id) ON DELETE CASCADE",
        ]
        up_positions = [up.index(fragment) for fragment in up_fragments]
        self.assertEqual(sorted(up_positions), up_positions)

        down_fragments = [
            "ALTER TABLE guliangiam.accounts DROP CONSTRAINT IF EXISTS "
            "fk_accounts_organization_id_organizations;",
            "DROP INDEX IF EXISTS uq_accounts_organization_external_id;",
            "DROP INDEX IF EXISTS idx_accounts_organization_id;",
            "DROP INDEX IF EXISTS uq_accounts_active_external_id;",
            "ALTER TABLE guliangiam.accounts DROP CONSTRAINT IF EXISTS uq_accounts_external_id;",
            "ALTER TABLE guliangiam.accounts DROP CONSTRAINT IF EXISTS ck_accounts_external_id;",
            "ALTER TABLE guliangiam.accounts DROP COLUMN IF EXISTS organization_id;",
            "ALTER TABLE guliangiam.accounts DROP COLUMN IF EXISTS external_id;",
        ]
        down_positions = [down.index(fragment) for fragment in down_fragments]
        self.assertEqual(sorted(down_positions), down_positions)

    def test_longtext_logical_type_renders_pg_text_in_increment(self) -> None:
        # A-6.3A 在声明层新增 LONGTEXT 列。增量渲染器必须把逻辑类型别名成
        # PostgreSQL 的 TEXT，否则 V13 会在 ADD COLUMN / ALTER TYPE 路径上
        # 直接产出 PostgreSQL 不认识的 `LONGTEXT`（建表/改列即失败）。全量
        # 渲染走 render_column，增量路径必须复用同一映射（pg_column_type）。
        previous = [{
            "table_name": "guliangiam.accounts",
            "pk": "id",
            "columns": [
                {"name": "id", "type": "UUID", "nullable": False},
                {"name": "notes", "type": "TEXT", "nullable": True},
            ],
            "indexes": [],
            "constraints": [],
            "fk_constraints": [],
        }]
        current = copy.deepcopy(previous)
        current[0]["columns"] = [
            {"name": "id", "type": "UUID", "nullable": False},
            {"name": "notes", "type": "LONGTEXT", "nullable": True},
            {"name": "payload", "type": "LONGTEXT", "nullable": True},
        ]

        self.assertEqual([], gen_schema.unsupported_migration_changes(previous, current))
        diff = gen_schema.diff_schemas(previous, current)
        up = gen_schema.render_migration_up(diff, 14, 13, current, {}, None)

        self.assertNotIn("LONGTEXT", up)
        self.assertIn(
            "ALTER TABLE guliangiam.accounts ADD COLUMN IF NOT EXISTS payload TEXT;", up
        )
        self.assertIn(
            "ALTER TABLE guliangiam.accounts ALTER COLUMN notes TYPE TEXT "
            "USING (notes::TEXT);",
            up,
        )

    def test_plain_index_and_fk_rendering(self) -> None:
        previous = [
            {
                "table_name": "guliangiam.organizations",
                "pk": "id",
                "columns": [{"name": "id", "type": "UUID", "nullable": False}],
            },
            {
                "table_name": "guliangiam.accounts",
                "pk": "id",
                "columns": [
                    {"name": "id", "type": "UUID", "nullable": False},
                    {"name": "organization_id", "type": "UUID", "nullable": True},
                ],
                "indexes": [],
                "fk_constraints": [],
            },
        ]
        current = copy.deepcopy(previous)
        current[1]["indexes"].append({
            "name": "idx_accounts_organization_id",
            "columns": ["organization_id"],
            "using": "btree",
            "predicate": "organization_id IS NOT NULL",
        })
        current[1]["fk_constraints"].append({
            "column": "organization_id",
            "ref_table": "guliangiam.organizations",
            "on_delete": "CASCADE",
            "on_update": "SET_NULL",
        })

        self.assertEqual([], gen_schema.unsupported_migration_changes(previous, current))
        diff = gen_schema.diff_schemas(previous, current)
        self.assertTrue(gen_schema.migration_diff_has_changes(diff))
        up = gen_schema.render_migration_up(diff, 13, 12, current, {}, None)
        down = gen_schema.render_migration_down(diff, 13, 12, None)

        self.assertIn(
            "CREATE INDEX IF NOT EXISTS idx_accounts_organization_id ON guliangiam.accounts "
            "USING btree (organization_id) WHERE organization_id IS NOT NULL;",
            up,
        )
        self.assertIn(
            "ADD CONSTRAINT fk_accounts_organization_id_organizations FOREIGN KEY "
            "(organization_id) REFERENCES guliangiam.organizations(id) "
            "ON DELETE CASCADE ON UPDATE SET NULL;",
            up,
        )
        self.assertIn("SELECT 1 FROM pg_constraint", up)
        self.assertIn("conrelid = 'guliangiam.accounts'::regclass", up)
        self.assertLess(
            down.index("DROP CONSTRAINT IF EXISTS fk_accounts_organization_id_organizations"),
            down.index("DROP INDEX IF EXISTS idx_accounts_organization_id"),
        )


class DiffSchemasTests(unittest.TestCase):
    def test_table_changes_preserve_declaration_order(self) -> None:
        previous = [
            {"table_name": "guliangiam.old_parent", "columns": []},
            {"table_name": "guliangiam.old_child", "columns": []},
            {"table_name": "guliangiam.shared", "columns": []},
        ]
        current = [
            {"table_name": "guliangiam.pay_refunds", "columns": []},
            {"table_name": "guliangiam.pay_attempts", "columns": []},
            {"table_name": "guliangiam.shared", "columns": []},
        ]

        diff = gen_schema.diff_schemas(previous, current)

        self.assertEqual(
            ["pay_refunds", "pay_attempts"],
            [gen_schema.short_name(t["table_name"]) for t in diff["added_tables"]],
        )
        self.assertEqual(
            ["old_parent", "old_child"],
            [gen_schema.short_name(t["table_name"]) for t in diff["dropped_tables"]],
        )


class AddedRlsMigrationTests(unittest.TestCase):
    """D3: 「已有表新增 RLS」必须可增量渲染，且反向关闭 RLS 必须被拒绝。"""

    def _tables(self, *, rls: bool | None = None, predicate: str | None = None) -> tuple[list[dict], list[dict]]:
        previous = [{
            "table_name": "guliangiam.provider_secrets",
            "pk": "id",
            "columns": [{"name": "id", "type": "UUID", "nullable": False}],
            "indexes": [],
            "constraints": [],
            "fk_constraints": [],
        }]
        current = copy.deepcopy(previous)
        if rls is not None:
            current[0]["rls"] = rls
        if predicate is not None:
            current[0]["rls_predicate"] = predicate
        return previous, current

    def test_added_rls_is_supported_and_rendered(self) -> None:
        predicate = (
            "guliangiam.is_super_admin() OR EXISTS (SELECT 1 FROM guliangiam.providers p "
            "WHERE p.id = provider_id AND p.organization_id = guliangiam.current_organization_id())"
        )
        previous, current = self._tables(rls=True, predicate=predicate)

        self.assertEqual([], gen_schema.unsupported_migration_changes(previous, current))
        diff = gen_schema.diff_schemas(previous, current)
        self.assertIn("provider_secrets", diff["added_rls"])
        self.assertTrue(gen_schema.migration_diff_has_changes(diff))

        up = gen_schema.render_migration_up(diff, 14, 13, current, {}, None)
        down = gen_schema.render_migration_down(diff, 14, 13, None)
        self.assertIn("ALTER TABLE guliangiam.provider_secrets ENABLE ROW LEVEL SECURITY;", up)
        self.assertIn("ALTER TABLE guliangiam.provider_secrets FORCE ROW LEVEL SECURITY;", up)
        self.assertIn("CREATE POLICY tenant_isolation ON guliangiam.provider_secrets", up)
        self.assertIn(predicate.replace("\n", " "), up)
        self.assertIn("DROP POLICY IF EXISTS tenant_isolation ON guliangiam.provider_secrets;", down)
        self.assertIn("ALTER TABLE guliangiam.provider_secrets DISABLE ROW LEVEL SECURITY;", down)

    def test_added_rls_without_predicate_renders_organization_id_default(self) -> None:
        previous, current = self._tables(rls=True)
        current[0]["columns"].append({"name": "organization_id", "type": "UUID", "nullable": False})

        diff = gen_schema.diff_schemas(previous, current)
        up = gen_schema.render_migration_up(diff, 14, 13, current, {}, None)
        self.assertIn("organization_id = guliangiam.current_organization_id()", up)

    def test_disabling_rls_is_rejected(self) -> None:
        previous, current = self._tables(rls=True)
        current[0]["rls"] = False
        self.assertTrue(gen_schema.unsupported_migration_changes(previous, current))

    def test_changing_predicate_of_enabled_rls_is_rejected(self) -> None:
        previous = [{
            "table_name": "guliangiam.provider_secrets",
            "pk": "id",
            "columns": [{"name": "id", "type": "UUID", "nullable": False}],
            "indexes": [],
            "constraints": [],
            "fk_constraints": [],
            "rls": True,
            "rls_predicate": "organization_id = guliangiam.current_organization_id()",
        }]
        current = copy.deepcopy(previous)
        current[0]["rls_predicate"] = "TRUE"
        self.assertTrue(gen_schema.unsupported_migration_changes(previous, current))


class SnapshotTests(unittest.TestCase):
    def test_freeze_snapshot_is_atomic_and_refuses_overwrite(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source = root / "schema.yaml"
            source.write_text("table_name: guliangiam.accounts\n", encoding="utf-8")
            target = root / "history/V1"

            gen_schema.freeze_snapshot(target, [source])

            self.assertEqual(source.read_bytes(), (target / source.name).read_bytes())
            with self.assertRaises(FileExistsError):
                gen_schema.freeze_snapshot(target, [source])


if __name__ == "__main__":
    unittest.main()
