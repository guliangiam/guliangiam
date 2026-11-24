"""check-write-invalidation.py 的单元测试 — 同文件 helper 一层展开的正反例。

输入：check-write-invalidation 模块 + 构造的 TSX 片段
输出：unittest 通过/失败
--check 行为：无 --check；单元测试
消费方：独立测试文件；`python3 scripts/test_check_write_invalidation.py`
"""

from __future__ import annotations

import importlib.util
import tempfile
import unittest
from pathlib import Path

_MODULE_PATH = Path(__file__).resolve().with_name("check-write-invalidation.py")
_SPEC = importlib.util.spec_from_file_location("check_write_invalidation", _MODULE_PATH)
assert _SPEC is not None and _SPEC.loader is not None
check_write = importlib.util.module_from_spec(_SPEC)
_SPEC.loader.exec_module(check_write)

DELETE_BLOCK = """
  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteUser(id),
    onSuccess: async () => {
      __ON_SUCCESS__
    },
    onError: () => message.error("failed"),
  });
"""


def delete_violations_for(source: str):
    with tempfile.TemporaryDirectory() as tmp:
        path = Path(tmp) / "Sample.tsx"
        path.write_text(source)
        return check_write.delete_violations(Path(tmp))


def sample(on_success: str, helpers: str = "") -> str:
    block = DELETE_BLOCK.replace("__ON_SUCCESS__", on_success)
    return f"""
function Sample() {{
{helpers}
{block}
}}
"""


class HelperResolutionTest(unittest.TestCase):
    def test_direct_invalidator_still_passes(self):
        source = sample('await queryClient.invalidateQueries({ queryKey: ["users"] });')
        self.assertEqual(delete_violations_for(source), [])

    def test_one_hop_helper_with_invalidator_passes(self):
        helpers = """
  async function invalidateAfterWrite() {
    await queryClient.invalidateQueries({ queryKey: ["users"] });
  }
"""
        source = sample("await invalidateAfterWrite();", helpers)
        self.assertEqual(delete_violations_for(source), [])

    def test_helper_without_invalidator_still_fails(self):
        helpers = """
  async function afterWrite() {
    message.success("deleted");
  }
"""
        source = sample("await afterWrite();", helpers)
        violations = delete_violations_for(source)
        self.assertEqual(len(violations), 1)
        self.assertIn("删除 onSuccess 缺少缓存失效", violations[0][2])

    def test_two_hop_helper_is_not_expanded(self):
        helpers = """
  function secondHop() {
    queryClient.invalidateQueries({ queryKey: ["users"] });
  }
  function firstHop() {
    secondHop();
  }
"""
        source = sample("firstHop();", helpers)
        violations = delete_violations_for(source)
        self.assertEqual(len(violations), 1, "二跳必须保持红")

    def test_arrow_helper_with_block_invalidator_passes(self):
        helpers = """
  const invalidateAfterWrite = async () => {
    await queryClient.invalidateQueries({ queryKey: ["users"] });
  };
"""
        source = sample("await invalidateAfterWrite();", helpers)
        self.assertEqual(delete_violations_for(source), [])

    def test_missing_on_error_segment_still_checks_block_tail(self):
        helpers = """
  async function afterWrite() {
    message.success("deleted");
  }
"""
        block = """
  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteUser(id),
    onSuccess: async () => {
      await afterWrite();
    },
  });
"""
        source = f"function Sample() {{\n{helpers}\n{block}\n}}"
        violations = delete_violations_for(source)
        self.assertEqual(len(violations), 1)


if __name__ == "__main__":
    unittest.main()
