#!/usr/bin/env python3
"""check-schema-evolution-audit.py 解析器回归测试（G1d / G1d-2）。

覆盖：
  * [C-1] INSERT ... SELECT 不得借用后续 INSERT 的 VALUES（语句边界）
  * [C-1] 真正的超长 VALUES 仍必须 FAIL（临时 fixture 反向控制）
  * [E-1/E-2] 错误文案 / 审计 reason 里的 from/order 英文单词不得被判为 SQL
  * [E-2] 真实 registry 文本逐条 triage：含 SQL 结构锚点的短片段保持
          TRUE_SQL，Go 调用/散文描述列入逐条 FALSE_POSITIVE 清单；
          判定不依赖长度阈值，也不产生运行时豁免。

运行：python3 -m unittest scripts/test_check_schema_evolution_audit.py -v
"""

from __future__ import annotations

import importlib.util
import json
import tempfile
import unittest
from pathlib import Path
from unittest import mock

ROOT = Path(__file__).resolve().parent.parent
SCRIPT = ROOT / "scripts" / "check-schema-evolution-audit.py"
REGISTRY_PATH = ROOT / "contracts" / "security" / "sql-injection-registry.json"

_spec = importlib.util.spec_from_file_location("check_schema_evolution_audit", SCRIPT)
assert _spec and _spec.loader
audit = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(audit)

# C-1 临时 seed fixture：第一条是 INSERT ... SELECT（没有 VALUES），第二条才是
# 真正的 VALUES。旧实现的越界读取会把第二条的长值挂到第一条的 currency 列上。
_BILLING_INSERT_SELECT = (
    "INSERT INTO guliangiam.organization_billing\n"
    "    (organization_id, balance, currency, default_currency)\n"
    "SELECT o.id, 0, 'USD', 'USD'\n"
    "FROM guliangiam.organizations o\n"
    "WHERE o.slug = 'built-in' AND o.deleted_at IS NULL\n"
    "ON CONFLICT DO NOTHING;\n"
)

_C1_TABLES = {
    "guliangiam.organization_billing": {
        "columns": {
            "organization_id": "UUID",
            "balance": "DECIMAL(19,4)",
            "currency": "VARCHAR(3)",
            "default_currency": "VARCHAR(3)",
        },
        "col_defs": {},
    },
    "guliangiam.products": {
        "columns": {
            "id": "UUID",
            "name": "VARCHAR(255)",
            "tag": "VARCHAR(255)",
        },
        "col_defs": {},
    },
}


def _seed_sql(tag: str) -> str:
    return _BILLING_INSERT_SELECT + (
        "INSERT INTO guliangiam.products (id, name, tag)\n"
        f"VALUES ('p1', 'demo', '{tag}');\n"
    )


def _run_c1_audit(sql: str) -> list[str]:
    """在临时 seed 目录里跑 C-1，不改仓库 fixture。

    quick=True 跳过 C-5；ROOT 指向临时目录后 C-4 也无文件可扫。
    """
    with tempfile.TemporaryDirectory() as tmp:
        root = Path(tmp)
        seed_dir = root / "seed_sql"
        seed_dir.mkdir()
        (seed_dir / "V1__fixture.sql").write_text(sql, encoding="utf-8")
        with mock.patch.object(audit, "ROOT", root), \
                mock.patch.object(audit, "BOOTSTRAP_SEED", seed_dir):
            return audit.audit_seed_value_lengths(_C1_TABLES, quick=True)


class InsertValuesBoundaryTests(unittest.TestCase):
    def test_insert_select_does_not_borrow_later_values(self) -> None:
        sql = (
            "INSERT INTO guliangiam.organization_billing\n"
            "    (organization_id, balance, currency, default_currency)\n"
            "SELECT o.id, 0, 'USD', 'USD'\n"
            "FROM guliangiam.organizations o\n"
            "WHERE o.slug = 'built-in' AND o.deleted_at IS NULL\n"
            "ON CONFLICT DO NOTHING;\n"
            "INSERT INTO guliangiam.products (id, name, tag)\n"
            "VALUES ('x', 'Built-in Demo Product', 'Seed fixture for commerce tests');"
        )
        self.assertIsNone(audit._split_insert_values(sql))

    def test_values_are_read_from_current_statement_only(self) -> None:
        sql = (
            "INSERT INTO guliangiam.products (id, name, tag)\n"
            "VALUES ('p1', 'one', 'demo');\n"
            "INSERT INTO guliangiam.other (id, name)\n"
            "VALUES ('p2', 'a-value-longer-than-the-first-statement');"
        )
        cols, vals = audit._split_insert_values(sql)
        self.assertEqual("id, name, tag", cols)
        self.assertEqual(["'p1'", "'one'", "'demo'"], audit._parse_sql_value_list(vals))

    def test_later_values_belong_to_later_statement(self) -> None:
        sql = _seed_sql("x" * 300)
        self.assertIsNone(audit._split_insert_values(sql))
        second = sql[sql.index("INSERT INTO guliangiam.products"):]
        cols, vals = audit._split_insert_values(second)
        self.assertEqual("id, name, tag", cols)
        parsed = audit._parse_sql_value_list(vals)
        self.assertEqual(3, len(parsed))
        self.assertEqual((300, "x" * 80), audit._extract_string_len(parsed[2]))


class SeedValueLengthTests(unittest.TestCase):
    def test_c1_does_not_charge_long_value_to_prior_insert_select(self) -> None:
        errors = _run_c1_audit(_seed_sql("x" * 300))
        self.assertFalse(
            any("organization_billing" in e for e in errors),
            f"长值被误挂到 INSERT ... SELECT: {errors}",
        )
        self.assertTrue(
            any("[C-1] guliangiam.products 列 tag" in e for e in errors),
            f"后一条 INSERT 的真实超长值未被检出: {errors}",
        )

    def test_c1_true_long_value_still_fails(self) -> None:
        errors = _run_c1_audit(_seed_sql("x" * 300))
        hits = [e for e in errors if "[C-1]" in e and "guliangiam.products" in e]
        self.assertEqual(1, len(hits), errors)
        self.assertIn("实际 300 chars > 上限 255", hits[0])

    def test_c1_short_value_passes(self) -> None:
        self.assertEqual([], _run_c1_audit(_seed_sql("demo")))


class ParseValueListTests(unittest.TestCase):
    def test_subquery_and_escaped_quote_are_not_split(self) -> None:
        vals = (
            "(SELECT id FROM guliangiam.organizations WHERE slug = 'built-in'), "
            "'it''s, fine'"
        )
        parsed = audit._parse_sql_value_list(vals)
        self.assertEqual(2, len(parsed))
        self.assertEqual("'it''s, fine'", parsed[1])
        self.assertEqual((10, "it's, fine"), audit._extract_string_len(parsed[1]))


class SqlStructureClassifierTests(unittest.TestCase):
    def test_prose_with_from_or_order_is_not_sql(self) -> None:
        self.assertFalse(audit._looks_like_sql_fragment(
            "payment callback currency %s does not match order currency %s"
        ))
        self.assertFalse(audit._looks_like_sql_fragment(
            "audit chain algorithm changed from %s to %s at chain_seq %d"
        ))
        self.assertFalse(audit._looks_like_sql_fragment(" from "))

    def test_real_sql_fragments_still_match(self) -> None:
        self.assertTrue(audit._looks_like_sql_fragment(
            "SELECT * FROM users WHERE username = %s"
        ))
        self.assertTrue(audit._looks_like_sql_fragment(
            "FROM guliangiam.users WHERE id = $1"
        ))
        self.assertTrue(audit._looks_like_sql_fragment(
            "UPDATE guliangiam.users SET name = %s WHERE id = %s"
        ))


# ── E-2 短片段 triage：2026-11-03 快照（registry 29 条 = 14 TRUE_SQL + 15 非 SQL 描述）──
# 14 条 pattern 含 SQL 结构锚点，必须继续判为 TRUE_SQL。
_REGISTRY_TRUE_SQL_PATTERNS = [
    ("internal/infra/postgres/iga/repository.go", "31",
     "qListCampaignsBase + extra + ' ORDER BY ...' + fmt.Sprintf('LIMIT $%d OFFSET $%d', ...)"),
    ("internal/infra/postgres/iga/repository.go", "123",
     'qListLifecycleBase + extra + ORDER BY + LIMIT/OFFSET'),
    ("internal/infra/postgres/actions/repository.go", "126",
     "selectSQL = queryListVersionsSelect + queryListVersionsBase + ' ORDER BY version DESC'"),
    ("internal/infra/postgres/actions/repository.go", "191",
     "selectSQL = ...Base + extra + ' ORDER BY created_at DESC'"),
    ("internal/infra/postgres/metering/repository.go", "47",
     "selectSQL = queryListEventsSelect + extra + ' ORDER BY occurred_at DESC'"),
    ("internal/infra/postgres/metering/repository.go", "137",
     "selectSQL = queryListAggregatesSelect + extra + ' ORDER BY period DESC'"),
    ("internal/infra/postgres/risk/repository.go", "46",
     "selectSQL = queryListSignalsSelect + queryListSignalsBase + extra + ' ORDER BY created_at DESC'"),
    ("internal/infra/mysql/iga/sql.go", "40",
     "countSQL = `SELECT COUNT(*) FROM ` + btCampaigns + ` WHERE organization_id = ?` + extra"),
    ("internal/platform/sorting/sorting.go", "111",
     "return 'ORDER BY ' + column + ' ' + direction + ', ' + s.tiebreaker() + ' ASC'"),
    ("internal/infra/postgres/shared/bulk.go", "50",
     "sb.WriteString(fmt.Sprintf('INSERT INTO %s (%s) VALUES ', table, colList))"),
    ("internal/infra/dialect/postgres.go", "63",
     "fmt.Sprintf('ON CONFLICT (%s) DO UPDATE SET %s', conflict, set)"),
    ("internal/infra/syncer/database_syncer.go", "239",
     "fmt.Sprintf('SELECT %s FROM %s', strings.Join(fields), table)"),
    ("internal/platform/crypto/reencrypt_job.go", "239",
     "fmt.Sprintf(`UPDATE %s SET %s = $1 WHERE %s = $2`, col.Table, col.Column, col.PKColumn)"),
    ("internal/infra/mysql/*/sql.go", "N/A (~100 hits)",
     "`SELECT ... FROM ` + btXXX + ` WHERE ...`"),
]

# 15 条 pattern 是 Go 调用 / 简写 / 散文描述，作为 SQL 字面量文本是 FALSE_POSITIVE。
# 逐条列出 file + excerpt + reason + owner，禁止通配符与目录级豁免；它们不是运行时
# 豁免，只是锁定「E-2 不应把这些文本当 SQL」的取证表。真实代码路径的覆盖由
# contracts/security/sql-injection-registry.json 的 E-5 文件级登记承担。
_REGISTRY_FALSE_POSITIVE_TRIAGE = [
    {"file": "internal/infra/postgres/records/repository.go", "line": "90",
     "excerpt": "strings.Replace(queryListRecords, '{ORDER_BY}', orderBy, 1)",
     "reason": "Go 调用文本，无 SQL 结构锚点；orderBy 由 recordSortSpec 白名单解析",
     "owner": "sec"},
    {"file": "internal/infra/postgres/records/repository.go", "line": "128",
     "excerpt": "strings.Replace(querySearchRecords, '{ORDER_BY}', orderBy, 1)",
     "reason": "同上；token 替换点已在 registry 登记为 L2 白名单构造",
     "owner": "sec"},
    {"file": "internal/infra/mysql/records/repository.go", "line": "87",
     "excerpt": "strings.Replace(queryListRecords, '{ORDER_BY}', orderBy, 1)",
     "reason": "同 postgres records；MySQL 侧 ORDER BY 也走平台白名单 Resolve()",
     "owner": "sec"},
    {"file": "internal/infra/mysql/records/repository.go", "line": "154",
     "excerpt": "strings.Replace(querySearchRecords, '{ORDER_BY}', orderBy, 1)",
     "reason": "同上；registry 已登记 runtime_guard",
     "owner": "sec"},
    {"file": "internal/infra/postgres/users/repository_list.go", "line": "24",
     "excerpt": "fmt.Sprintf('%s %s %s LIMIT $5 OFFSET $6', queryListUsers, columnSearchClause, orderBy)",
     "reason": "Go 调用文本；参数是常量查询、白名单列搜索与白名单排序，无 SQL 表/列锚点",
     "owner": "sec"},
    {"file": "internal/infra/postgres/users/repository_list.go", "line": "77",
     "excerpt": "fmt.Sprintf('%s %s', queryCountUsers, columnSearchClause)",
     "reason": "Go 调用文本；columnSearchClause 由 buildColumnSearch 白名单构造",
     "owner": "sec"},
    {"file": "internal/infra/postgres/risk/repository.go", "line": "91",
     "excerpt": "queryListDecisions* + extra",
     "reason": "registry 简写，不是源码字面量；实际拼接由常量查询与白名单 helper 组成",
     "owner": "sec"},
    {"file": "internal/infra/postgres/risk/repository.go", "line": "158",
     "excerpt": "queryListGovernance* + extra",
     "reason": "同上；ORDER BY 为常量后缀，extra 由 buildListExtra 白名单构造",
     "owner": "sec"},
    {"file": "internal/infra/postgres/scim/user_repository.go", "line": "71",
     "excerpt": "countQuery = `SELECT COUNT(*) ` + baseQuery",
     "reason": "Go 赋值表达式而非字面量；baseQuery 是同函数内硬编码常量，短 SELECT 片段不单独进 E-2",
     "owner": "sec"},
    {"file": "internal/infra/postgres/scim/user_repository.go", "line": "72",
     "excerpt": "selectQuery = `SELECT ... ` + baseQuery",
     "reason": "同上；`SELECT ... ` 为 registry 省略写法，列清单来自同函数常量",
     "owner": "sec"},
    {"file": "internal/infra/postgres/scim/group_repository.go", "line": "45",
     "excerpt": "countQuery + selectQuery 拼接 baseQuery",
     "reason": "中文描述句，无 SQL 结构；实际 baseQuery 是函数内常量",
     "owner": "sec"},
    {"file": "internal/infra/postgres/scim/schema_repository.go", "line": "42",
     "excerpt": "countQuery + selectQuery 拼接 baseQuery",
     "reason": "同上；该文件三个查询均以局部常量 baseQuery 收口",
     "owner": "sec"},
    {"file": "internal/infra/mysql/iga/sql.go", "line": "92",
     "excerpt": "countSQL + selectSQL lifecycle",
     "reason": "缩写描述，无 SQL 结构；对应常量由 btLifecycle 编译期表名拼接",
     "owner": "sec"},
    {"file": "internal/infra/postgres/regions/repository.go", "line": "99",
     "excerpt": "fullSQL = fmt.Sprintf('... IN (%s) ...', ph, ph2, ph2)",
     "reason": "Go 调用文本；%s 由内部占位符 ph 生成，registry guardian 已登记",
     "owner": "sec"},
    {"file": "internal/infra/dialect/mysql.go", "line": "76",
     "excerpt": "fmt.Sprintf('ON DUPLICATE KEY UPDATE %s', set)",
     "reason": "Go 调用文本；set 由列清单常量构造，E-1 已有 ON DUPLICATE KEY 安全规则",
     "owner": "sec"},
]


class RegistryFragmentTriageTests(unittest.TestCase):
    def test_true_sql_registry_patterns_stay_true_sql(self) -> None:
        self.assertEqual(14, len(_REGISTRY_TRUE_SQL_PATTERNS))
        for file, line, excerpt in _REGISTRY_TRUE_SQL_PATTERNS:
            with self.subTest(file=file, line=line):
                self.assertEqual(
                    audit.SQL_FRAGMENT_TRUE,
                    audit._classify_sql_fragment(excerpt),
                )

    def test_false_positive_entries_are_explicit_and_non_sql(self) -> None:
        self.assertEqual(15, len(_REGISTRY_FALSE_POSITIVE_TRIAGE))
        for row in _REGISTRY_FALSE_POSITIVE_TRIAGE:
            with self.subTest(file=row["file"], line=row["line"]):
                self.assertTrue(row["file"] and row["excerpt"])
                self.assertTrue(row["reason"] and row["owner"])
                self.assertNotIn("*", row["file"])
                self.assertFalse(row["file"].endswith("/"))
                self.assertEqual(
                    audit.SQL_FRAGMENT_FALSE_POSITIVE,
                    audit._classify_sql_fragment(row["excerpt"]),
                )

    def test_triage_covers_every_registry_pattern(self) -> None:
        entries = json.loads(REGISTRY_PATH.read_text(encoding="utf-8"))["entries"]
        actual = {(e["file"], str(e["line"]), e["pattern"]) for e in entries}
        expected = set(_REGISTRY_TRUE_SQL_PATTERNS) | {
            (r["file"], r["line"], r["excerpt"]) for r in _REGISTRY_FALSE_POSITIVE_TRIAGE
        }
        self.assertEqual(expected, actual)

    def test_documented_non_sql_entries_are_false_positives(self) -> None:
        registry = json.loads(REGISTRY_PATH.read_text(encoding="utf-8"))
        for entry in registry.get("false_positives_filtered_out", []):
            text = f"{entry.get('file', '')} {entry.get('reason', '')}"
            self.assertEqual(
                audit.SQL_FRAGMENT_FALSE_POSITIVE,
                audit._classify_sql_fragment(text),
            )


class ShortFragmentVerdictTests(unittest.TestCase):
    def test_short_true_sql_fragments_are_not_dropped_by_length(self) -> None:
        for text in (
            "ORDER BY ",
            " WHERE id = ",
            " ON CONFLICT ",
            "DELETE FROM users WHERE id = ",
            "SELECT * FROM ",
            "DELETE FROM ",
            "INSERT INTO ",
        ):
            self.assertEqual(audit.SQL_FRAGMENT_TRUE, audit._classify_sql_fragment(text))

    def test_prose_and_audit_reasons_are_false_positives(self) -> None:
        for text in (
            " from ",
            "order currency",
            "payment callback currency %s does not match order currency %s",
            "audit chain algorithm changed from %s to %s at chain_seq %d",
            "HTTP API URL, not SQL",
        ):
            self.assertEqual(
                audit.SQL_FRAGMENT_FALSE_POSITIVE,
                audit._classify_sql_fragment(text),
            )


class ScannerWiringTests(unittest.TestCase):
    def _scan(self, source: str, detector):
        with tempfile.TemporaryDirectory() as tmp:
            fp = Path(tmp) / "sample.go"
            fp.write_text(source, encoding="utf-8")
            original_walk, original_rel = audit._walk_go_files, audit._rel
            audit._walk_go_files = lambda: [fp]
            audit._rel = lambda p: str(p)
            try:
                return detector()
            finally:
                audit._walk_go_files, audit._rel = original_walk, original_rel

    def test_e1_skips_prose_and_keeps_sql(self) -> None:
        hits = self._scan(
            "func f() {\n"
            '    _ = fmt.Sprintf("payment callback currency %s does not match '
            'order currency %s", actual, expected)\n'
            '    _ = fmt.Sprintf("SELECT * FROM users WHERE username = %s", user)\n'
            "}\n",
            audit._e1_detect_fmt_sprintf_sql_risks,
        )
        self.assertEqual(1, len(hits))
        self.assertIn("SELECT * FROM users", hits[0]["snippet"])

    def test_e2_skips_error_message_and_keeps_sql_concat(self) -> None:
        hits = self._scan(
            "func f() {\n"
            '    msg := "cannot advance payment "+payment.ID+" from "+payment.State\n'
            '    q := "SELECT * FROM users WHERE id = " + userID\n'
            "}\n",
            audit._e2_detect_concat_risks,
        )
        self.assertEqual(1, len(hits))
        self.assertIn("SELECT * FROM users", hits[0]["snippet"])

    def test_e2_short_true_sql_fragments_still_fail(self) -> None:
        hits = self._scan(
            "func f() {\n"
            '    q := " WHERE id = " + userID\n'
            '    q2 := "DELETE FROM users WHERE id = " + id\n'
            "}\n",
            audit._e2_detect_concat_risks,
        )
        self.assertEqual(2, len(hits), hits)

    def test_e2_trailing_sql_anchor_still_fails(self) -> None:
        hits = self._scan(
            "func f() {\n"
            '    q := "SELECT * FROM " + table\n'
            "}\n",
            audit._e2_detect_concat_risks,
        )
        self.assertEqual(1, len(hits), hits)
        self.assertIn("SELECT * FROM", hits[0]["snippet"])


if __name__ == "__main__":
    unittest.main()
