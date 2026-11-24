#!/usr/bin/env python3
"""P3 runtime guard + P2 AST 工具 联合验证脚本。

测试目标:
  1. sqlguard Go 单元测试: go test ./internal/platform/sqlguard/... 全部 PASS
  2. AST 扫描器对故意注入的 fmt.Sprintf("... WHERE x=%s", DTO.Field) → FATAL
  3. check-schema-evolution-audit.py 对注入代码 → exit code 1
  4. 清理后恢复 → 全量门禁 PASS

设计: 脚本在 repository_list.go 里注入一行 bad SQL, 跑 AST + audit 验证 FATAL,
     然后恢复文件。整个过程无人工干预。

用法: python3 scripts/verify-sqlguard-gate.py
"""

import subprocess
import sys
import pathlib
import re

ROOT = pathlib.Path(__file__).resolve().parent.parent

GREEN = "\033[92m"
RED = "\033[91m"
YELLOW = "\033[93m"
RESET = "\033[0m"


def run(cmd: list[str], cwd: pathlib.Path = ROOT, timeout: int = 120) -> tuple[int, str, str]:
    result = subprocess.run(
        cmd, cwd=str(cwd), capture_output=True, text=True, timeout=timeout
    )
    return result.returncode, result.stdout, result.stderr


def step1_unit_tests() -> bool:
    """跑 sqlguard 单元测试。"""
    print(f"\n{YELLOW}[Step 1] Go unit tests for sqlguard{RESET}")
    code, out, err = run(["go", "test", "./internal/platform/sqlguard/...", "-v"])
    if code != 0:
        print(f"  {RED}FAIL{RESET}: go test exited {code}")
        print(err[-600:] if err else out[-600:])
        return False
    passed = out.count("--- PASS")
    failed = out.count("--- FAIL")
    print(f"  {GREEN}PASS{RESET}: {passed} passed, {failed} failed")
    return failed == 0


def step2_inject_and_verify() -> bool:
    """在 repository_list.go 里注入 bad SQL → 跑 AST + audit → 恢复。"""
    print(f"\n{YELLOW}[Step 2] Inject bad SQL + verify gates catch it{RESET}")

    target = ROOT / "internal/infra/postgres/users/repository_list.go"
    content = target.read_text()

    marker = "// P3_TEST_INJECTION: DANGER - intentionally unsafe SQL"
    injection_line = (
        "\t// P3_TEST_INJECTION: DANGER - intentionally unsafe SQL\n"
        '\tdangerSQL := fmt.Sprintf("SELECT * FROM users WHERE username = %s", query.Keyword)\n'
        "\t_ = dangerSQL\n"
    )

    # 如果已经注入过（上次恢复失败），先恢复
    if marker in content:
        print(f"  {YELLOW}note{RESET}: previous injection marker found, cleaning first")
        _restore(target, marker)
        content = target.read_text()

    # 在 buildColumnSearch 调用后面注入
    inject_after = "columnSearchClause, columnSearchValue := buildColumnSearch(query.SearchField, query.SearchValue)"
    if inject_after not in content:
        print(f"  {RED}FAIL{RESET}: injection anchor not found in target file")
        return False

    content = content.replace(inject_after, inject_after + "\n" + injection_line)
    target.write_text(content)
    print(f"  injected bad SQL into {target.relative_to(ROOT)}")

    all_ok = True
    try:
        # 2a: AST 扫描器应该 FATAL
        code, out, _ = run(["go", "run", "scripts/ast-sql-scanner/main.go"])
        if code != 0:
            # go run 自身可能 exit 0，看 JSON 里有没有 FATAL finding
            try:
                import json
                data = json.loads(out)
                fatals = [f for f in data.get("findings", []) if f.get("level") == "FATAL"]
            except Exception:
                fatals = []
        else:
            import json
            try:
                data = json.loads(out)
                fatals = [f for f in data.get("findings", []) if f.get("level") == "FATAL"]
            except Exception:
                fatals = []

        fatal_hit = any("repository_list.go" in f.get("file", "") and f.get("check") == "E1" for f in fatals)
        if fatal_hit:
            print(f"  {GREEN}PASS{RESET}: AST scanner FATAL on injected bad SQL")
        else:
            print(f"  {RED}FAIL{RESET}: AST scanner did NOT FATAL on injected bad SQL")
            print(f"    FATAL findings: {fatals}")
            all_ok = False

        # 2b: audit 脚本应该 exit 1
        code, out, _ = run(["python3", "scripts/check-schema-evolution-audit.py"])
        if code == 1 and "FAIL" in out and "E" in out[:300]:
            print(f"  {GREEN}PASS{RESET}: audit gate exits 1 on injected bad SQL")
        else:
            print(f"  {RED}FAIL{RESET}: audit gate should exit 1, got {code}")
            print(f"    tail: {out[-300:]}")
            all_ok = False

    finally:
        _restore(target, marker)
        print(f"  restored original file")

    return all_ok


def _restore(target: pathlib.Path, marker: str) -> None:
    """删除注入的 3 行（marker + dangerSQL + blank）。"""
    content = target.read_text()
    lines = content.splitlines(keepends=True)
    cleaned = []
    skip = 0
    for line in lines:
        if marker in line:
            skip = 3  # 跳过 marker + 下两行（dangerSQL + _ = dangerSQL）
            continue
        if skip > 0:
            skip -= 1
            continue
        cleaned.append(line)
    # 也清理残留的孤立 _ = dangerSQL（如果 marker 被手动删了）
    final = "".join(cleaned)
    final = re.sub(r'^\t_ = dangerSQL\n', '', final, flags=re.MULTILINE)
    target.write_text(final)


def step3_clean_verification() -> bool:
    """恢复后跑全量门禁，确认无副作用。"""
    print(f"\n{YELLOW}[Step 3] Post-restore: all gates pass?{RESET}")

    all_ok = True

    # go build
    code, _, _ = run(["go", "build", "./..."])
    if code == 0:
        print(f"  {GREEN}PASS{RESET}: go build ./...")
    else:
        print(f"  {RED}FAIL{RESET}: go build failed")
        all_ok = False

    # audit
    code, out, _ = run(["python3", "scripts/check-schema-evolution-audit.py"])
    if code == 0 and "RESULT: 全 PASS" in out:
        print(f"  {GREEN}PASS{RESET}: audit gate all 5 dimensions pass")
    else:
        print(f"  {RED}FAIL{RESET}: audit gate: code={code}")
        print(f"    tail: {out[-200:]}")
        all_ok = False

    return all_ok


def main():
    print("=" * 60)
    print("P3 SQL Guard + P2 AST Scanner Joint Verification")
    print("=" * 60)

    r1 = step1_unit_tests()
    r2 = step2_inject_and_verify()
    r3 = step3_clean_verification()

    print("\n" + "=" * 60)
    print("SUMMARY")
    print("=" * 60)
    print(f"  Step 1 (Go unit tests):   {'PASS' if r1 else 'FAIL'}")
    print(f"  Step 2 (Inject+FATAL):   {'PASS' if r2 else 'FAIL'}")
    print(f"  Step 3 (Post-restore):   {'PASS' if r3 else 'FAIL'}")

    if r1 and r2 and r3:
        print(f"\n{GREEN}All P3 verification steps passed.{RESET}")
        return 0
    else:
        print(f"\n{RED}Some steps FAILED.{RESET}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
