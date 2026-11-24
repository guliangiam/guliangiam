"""check-paths-consistency.py 的单元测试 — 注释豁免与真实字面量检测

输入：check-paths-consistency 模块 + 构造的 TS/TSX 片段
输出：unittest 通过/失败
--check 行为：无 --check；单元测试
消费方：独立测试文件；`python3 -m unittest scripts.test_check_paths_consistency`
"""

from __future__ import annotations

import importlib.util
import unittest
from pathlib import Path

_MODULE_PATH = Path(__file__).resolve().with_name("check-paths-consistency.py")
_SPEC = importlib.util.spec_from_file_location("check_paths_consistency", _MODULE_PATH)
assert _SPEC is not None and _SPEC.loader is not None
check_paths = importlib.util.module_from_spec(_SPEC)
_SPEC.loader.exec_module(check_paths)


def violations(text: str) -> list[tuple[int, str]]:
    return [
        (lineno, literal)
        for lineno, literal, _ in check_paths.find_violations(text)
    ]


class CommentExemptionTests(unittest.TestCase):
    def test_line_comment_path_is_ignored(self) -> None:
        text = (
            '// 旧实现请求 "/api/v1/auth/login"\n'
            "export const ready = true;\n"
        )
        self.assertEqual([], violations(text))

    def test_jsdoc_and_block_comment_paths_are_ignored(self) -> None:
        text = (
            "/**\n"
            " * The old implementation posted to `/api/v1/auth/login`,\n"
            " * whose request type has no such field.\n"
            " */\n"
            '/* fetch("/api/v1/providers"); */\n'
        )
        self.assertEqual([], violations(text))

    def test_commented_out_code_is_ignored(self) -> None:
        text = (
            '// const oldUrl = "/api/v1/users";\n'
            "/*\n"
            '  fetch("/api/v1/providers");\n'
            "  const tpl = `/api/v1/applications/${id}`;\n"
            "*/\n"
            "const ok = 1;\n"
        )
        self.assertEqual([], violations(text))

    def test_comment_and_real_string_in_same_file(self) -> None:
        text = (
            '// 迁移前的调用点：fetch("/api/v1/legacy")\n'
            'export const url = "/api/v1/users"; // 曾用 "/api/v1/old-users"\n'
        )
        self.assertEqual([(2, "/api/v1/users")], violations(text))

    def test_code_after_block_comment_is_still_scanned(self) -> None:
        text = '/* old */ const url = "/api/v1/users";\n'
        self.assertEqual([(1, "/api/v1/users")], violations(text))


class RealLiteralTests(unittest.TestCase):
    def test_real_string_and_template_literals_are_reported(self) -> None:
        text = (
            'const a = "/api/v1/users";\n'
            "const b = '/api/v1/users/1';\n"
            "const c = `/api/v1/users/${encodeURIComponent(name)}`;\n"
        )
        self.assertEqual(
            [
                (1, "/api/v1/users"),
                (2, "/api/v1/users/1"),
                (3, "/api/v1/users/${encodeURIComponent(name)}"),
            ],
            violations(text),
        )

    def test_template_interpolation_is_scanned_as_code(self) -> None:
        text = (
            'const hidden = `${/* "/api/v1/hidden" */ ""}`;\n'
            'const quoted = `${fetch("/api/v1/users")}`;\n'
        )
        self.assertEqual([(2, "/api/v1/users")], violations(text))

    def test_multiline_template_path_is_not_reported(self) -> None:
        # 已知边界：跨行模板按行匹配；路径字面量与闭合反引号不在同一行时不报（与旧行为一致）
        text = (
            "const url = `/api/v1/users/${\n"
            "  encodeURIComponent(name)\n"
            "}`;\n"
        )
        self.assertEqual([], violations(text))

    def test_slashes_inside_string_do_not_start_comment(self) -> None:
        text = (
            'const base = "https://example.com";\n'
            'const url = "/api/v1/users";\n'
        )
        self.assertEqual([(2, "/api/v1/users")], violations(text))

    def test_comment_marker_inside_template_is_not_masked(self) -> None:
        text = 'const s = `// "/api/v1/users"`;\n'
        self.assertEqual([(1, "/api/v1/users")], violations(text))

    def test_regex_and_division_do_not_swallow_real_literals(self) -> None:
        text = (
            "const re = /[\"']/;\n"
            "const ratio = total / count;\n"
            'const url = "/api/v1/users";\n'
        )
        self.assertEqual([(3, "/api/v1/users")], violations(text))

    def test_increment_division_treated_as_division(self) -> None:
        text = (
            'const half = i++ / 2; // 旧调用 "/api/v1/legacy"\n'
            'const url = "/api/v1/users";\n'
        )
        self.assertEqual([(2, "/api/v1/users")], violations(text))

    def test_escaped_quote_does_not_end_string(self) -> None:
        text = 'const msg = "she said \\"hi\\""; const url = "/api/v1/users";\n'
        self.assertEqual([(1, "/api/v1/users")], violations(text))


class ExcludeSemanticsTests(unittest.TestCase):
    def test_exclude_sets_are_unchanged(self) -> None:
        self.assertEqual({"generated", "e2e", "app"}, check_paths.EXCLUDE_DIRS)
        self.assertEqual({"paths.ts"}, check_paths.EXCLUDE_FILES)

    def test_should_exclude_matches_existing_semantics(self) -> None:
        self.assertTrue(check_paths.should_exclude(["generated"], "client.ts"))
        self.assertTrue(check_paths.should_exclude(["e2e"], "flow.ts"))
        self.assertTrue(check_paths.should_exclude(["app", "router"], "routes.ts"))
        self.assertTrue(check_paths.should_exclude(["shared", "api"], "paths.ts"))
        self.assertTrue(check_paths.should_exclude(["pages"], "Page.test.tsx"))
        self.assertFalse(
            check_paths.should_exclude(
                ["pages", "auth"], "useSamlCallbackPageController.tsx"
            )
        )


if __name__ == "__main__":
    unittest.main()
