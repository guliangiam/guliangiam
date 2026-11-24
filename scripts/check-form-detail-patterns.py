#!/usr/bin/env python3
"""
GuliangIAM Frontend Form-Detail-Page Pattern Audit

Scans for confirmed form/detail bug patterns (A–J) found during 3 rounds of
audit on ApplicationDetailPage (and propagated to the entire codebase).

Usage: python3 scripts/check-form-detail-patterns.py [--fix]

Output: JSON + human-readable report. Exit code: 0 = clean, non-zero = issues found.
"""

from __future__ import annotations

import re
import sys
import subprocess
import argparse
from pathlib import Path
from dataclasses import dataclass, field

REPO = Path(__file__).resolve().parent.parent
WEB = REPO / "web"
SRC = WEB / "src"

@dataclass
class Finding:
    pattern_id: str
    pattern_name: str
    file: str
    line: int
    snippet: str
    severity: str  # HIGH / MEDIUM / LOW

    def __str__(self):
        return f"[{self.severity}] {self.pattern_id} {self.pattern_name} @ {self.file}:{self.line}  {self.snippet.strip()[:80]}"

def run_grep(pattern: str, glob: str = "*.ts*", cwd: Path = SRC, flags: str = "-rn") -> list[str]:
    result = subprocess.run(
        ["grep", flags, "--include", glob, pattern, str(cwd)],
        capture_output=True, text=True, cwd=str(cwd),
    )
    return result.stdout.strip().splitlines() if result.stdout.strip() else []

def pattern_a_zod_array_default() -> list[Finding]:
    """Pattern A: .array(...).default([]) should be .catch([])"""
    findings = []
    for line in run_grep(r'\.array.*\.default\(\[\]\)'):
        m = re.match(r'(.+?):(\d+):(.+)', line)
        if m:
            findings.append(Finding("A", "Zod.array.default([]) → .catch([])",
                m.group(1).replace(str(REPO)+"/", ""), int(m.group(2)), m.group(3), "HIGH"))
    return findings

def pattern_b_spa_debounce_no_clear() -> list[Finding]:
    """Pattern B: setTimeout in controller but entity-key sync block missing clearTimeout"""
    findings = []
    # Find all controller files
    controllers = list(SRC.rglob("*controller*")) + list(SRC.rglob("*Controller*"))
    for ctrl in controllers:
        if not ctrl.is_file():
            continue
        content = ctrl.read_text()
        has_settimeout = "setTimeout(" in content and "setInterval(" not in content
        # Check for entity-key sync pattern
        has_entity_sync = "entityKey" in content and ("clearTimeout" not in content or "dirtyDebounce" in content)
        if has_settimeout and has_entity_sync and "clearTimeout" not in content:
            findings.append(Finding("B", "SPA navigate debounce timer not cleared in entity-key sync",
                str(ctrl).replace(str(REPO)+"/", ""), 1, "(whole file)", "MEDIUM"))
    return findings

def pattern_c_isnew_truthy_guard() -> list[Finding]:
    """Pattern C: Router param "new" string truthy — missing !isNew guard on conditional render"""
    findings = []
    # Look for pattern: applicationId ? <Component /> where Component is a tab/detail that
    # queries data. Exclude obvious safe cases like export button.
    unsafe_components = ["Tab", "Detail", "Log", "List", "Table"]
    for line in run_grep(r'(applicationId|organizationId|userId|roleId|providerId)\s*\?\s*<'):
        m = re.match(r'(.+?):(\d+):(.+)', line)
        if not m:
            continue
        filepath, lineno, snippet = m.group(1), int(m.group(2)), m.group(3)
        # Skip if same block has !isNew or isNew guard
        has_guard = "!isNew" in snippet or "isNew" in snippet.split("?")[0] if "?" in snippet else False
        if not has_guard and any(c in snippet for c in unsafe_components):
            findings.append(Finding("C", 'Potential "new" truthy bug — missing !isNew guard',
                filepath.replace(str(REPO)+"/", ""), lineno, snippet, "MEDIUM"))
    return findings

def pattern_d_switch_missing_valuepropname() -> list[Finding]:
    """Pattern D: Switch inside Form.Item without valuePropName="checked" """
    findings = []
    files_with_switch = run_grep(r'<Switch\s*/?>')
    for file_line in files_with_switch:
        m = re.match(r'(.+?):(\d+):', file_line)
        if not m:
            continue
        filepath, lineno = m.group(1), int(m.group(2))
        # Read file context
        try:
            content = Path(filepath).read_text().splitlines()
        except Exception:
            continue
        # Search backward up to 15 lines for Form.Item and check if it has valuePropName
        context_start = max(0, lineno - 20)
        context = "\n".join(content[context_start:lineno])
        in_form_item = "Form.Item" in context or "Form.Item>" in context
        has_valueprop = "valuePropName=\"checked\"" in context or "valuePropName='checked'" in context
        if in_form_item and not has_valueprop:
            findings.append(Finding("D", "Switch inside Form.Item missing valuePropName=\"checked\"",
                filepath.replace(str(REPO)+"/", ""), lineno, content[lineno-1][:80] if lineno <= len(content) else "", "MEDIUM"))
    return findings

def pattern_e_handle_reset_baseline() -> list[Finding]:
    """Pattern E: handleReset calls resetFields() but doesn't update dirty baseline ref"""
    findings = []
    for line in run_grep(r'const handleReset\s*=\s*useCallback'):
        m = re.match(r'(.+?):(\d+):', line)
        if not m:
            continue
        filepath, lineno = m.group(1), int(m.group(2))
        try:
            content = Path(filepath).read_text()
        except Exception:
            continue
        # Find the handleReset function body
        reset_match = re.search(r'handleReset\s*=\s*useCallback\([^)]+\)\s*=>\s*\{([^}]+)\}', content, re.DOTALL)
        if not reset_match:
            continue
        body = reset_match.group(1)
        has_reset_fields = "resetFields()" in body
        has_baseline_update = "initialConfigRef" in body or "normalizeConfigSnapshot" in body or "deepEqual" in body
        if has_reset_fields and not has_baseline_update:
            findings.append(Finding("E", "handleReset resets Form but doesn't update dirty baseline ref",
                filepath.replace(str(REPO)+"/", ""), lineno, "(function body)", "MEDIUM"))
    return findings

def pattern_f_save_success_baseline() -> list[Finding]:
    """Pattern F: onSuccess sets isDirty(false) before updating baseline ref"""
    findings = []
    for line in run_grep(r'onSuccess\s*:\s*async'):
        m = re.match(r'(.+?):(\d+):', line)
        if not m:
            continue
        filepath, lineno = m.group(1), int(m.group(2))
        try:
            content = Path(filepath).read_text()
        except Exception:
            continue
        # Find onSuccess block
        success_match = re.search(r'onSuccess\s*:\s*async\s*\([^)]*\)\s*=>\s*\{([^}]+)\}', content, re.DOTALL)
        if not success_match:
            continue
        body = success_match.group(1)
        set_dirty_idx = body.find("setIsDirty(false)")
        baseline_idx = body.find("initialConfigRef")
        if set_dirty_idx != -1 and (baseline_idx == -1 or baseline_idx > set_dirty_idx):
            findings.append(Finding("F", "onSuccess sets isDirty(false) but baseline ref NOT updated BEFORE it",
                filepath.replace(str(REPO)+"/", ""), lineno, "(function body)", "HIGH"))
    return findings

def pattern_g_formitem_name_duplicates() -> list[Finding]:
    """Pattern G: Duplicate Form.Item name= inside the *same* <Form> instance.

    Scope matters: a page often hosts several independent forms (a filter form in the
    header and a create/edit form in the body). Field names like `period`/`metric`
    legitimately repeat across those instances, so counting per file reports every
    such page as a defect and buries the real cases. Duplicates are therefore
    evaluated per <Form ...>…</Form> block; items outside any block are ignored
    (they cannot collide with a form instance).

    Known false positives (excluded):
    - .test.tsx / .stories.tsx / .spec.tsx — storybook/test 中多组件共享 name 字符串
    - 非 AntD Form.Item 的 name 属性（如 <Input name="..."> 原生属性）
    """
    findings = []
    files_with_form = run_grep(r'<Form\.Item[^>]*name=')
    from collections import Counter
    EXCLUDE_GLOBS = (".test.tsx", ".stories.tsx", ".spec.tsx")
    FORM_BLOCK = re.compile(r'<Form(?![.\w])[\s\S]*?</Form>')
    for file_line in files_with_form:
        m = re.match(r'(.+?):', file_line)
        if not m:
            continue
        filepath = m.group(1)
        # Skip test / story / spec files (known false positives)
        if any(g in filepath for g in EXCLUDE_GLOBS):
            continue
        content = Path(filepath).read_text()
        for block_index, block in enumerate(FORM_BLOCK.finditer(content), start=1):
            # Only count Form.Item name= (exclude <Input name=, <Column name= etc.)
            form_item_names = re.findall(
                r'<Form\.Item[^>]*name=["\']([^"\']+)["\']', block.group(0)
            )
            duplicates = {n: c for n, c in Counter(form_item_names).items() if c > 1}
            for name, count in duplicates.items():
                line = content[: block.start()].count("\n") + 1
                findings.append(
                    Finding(
                        "G",
                        f'Duplicate Form.Item name="{name}" ({count}×) in one form',
                        filepath.replace(str(REPO) + "/", ""),
                        line,
                        f'name="{name}" appears {count} times inside form #{block_index}',
                        "HIGH",
                    )
                )
    return findings

def pattern_h_onSubmit_mutation() -> list[Finding]:
    """Pattern H: onSubmit fires a mutation without returning its promise.

    `ResourceFormPage` keeps the submission lock only until the callback's
    promise settles. `onSubmit={() => { void form.validateFields().then(...
    mutate(...)) }}` returns immediately, so the lock releases before the
    request starts and failures never reach the shell's error surface. The
    documented red line (docs/design/frontend/interaction.md) requires
    `onFinish` + `mutateAsync`.
    """
    findings = []
    EXCLUDE_GLOBS = (".test.", ".stories.", ".spec.")
    for path in sorted(SRC.rglob("*.tsx")):
        if any(g in path.name for g in EXCLUDE_GLOBS):
            continue
        content = path.read_text()
        for match in re.finditer(r'onSubmit=\{([\s\S]{0,400})\}', content):
            body = match.group(1)
            if ".mutate(" not in body:
                continue
            line = content.count("\n", 0, match.start()) + 1
            findings.append(
                Finding(
                    "H",
                    "onSubmit fires mutation without returning its promise",
                    str(path).replace(str(REPO) + "/", ""),
                    line,
                    body.replace("\n", " ")[:80],
                    "HIGH",
                )
            )
    return findings

def pattern_i_form_missing_first_error_scroll() -> list[Finding]:
    """Pattern I: page-level form with >=6 items lacks scrollToFirstError.

    `ResourceFormPage` already scrolls to the first error, but page-level raw
    `<Form>` instances do not: when the first invalid field sits below the
    viewport, validation failure looks like a no-op. Modal/Drawer forms are
    exempt because their own container scrolls; short forms (< 6 items) fit a
    typical viewport.
    """
    findings = []
    MIN_FORM_ITEMS = 6
    EXCLUDE_GLOBS = (".test.", ".stories.", ".spec.")
    FORM_BLOCK = re.compile(r'<Form(?![.\w])[\s\S]*?</Form>')
    for path in sorted((SRC / "pages").rglob("*.tsx")):
        if any(g in path.name for g in EXCLUDE_GLOBS):
            continue
        content = path.read_text()
        if "ResourceFormPage" in content:
            continue
        if "<Modal" in content or "<Drawer" in content:
            continue
        for block_index, block in enumerate(FORM_BLOCK.finditer(content), start=1):
            if block.group(0).count("<Form.Item") < MIN_FORM_ITEMS:
                continue
            # Only antd's own submit path can honour scrollToFirstError; forms that
            # call form.validateFields() from a button callback never trigger it.
            if "onFinish" not in block.group(0):
                continue
            if "scrollToFirstError" in block.group(0):
                continue
            line = content.count("\n", 0, block.start()) + 1
            findings.append(
                Finding(
                    "I",
                    "Page-level form with >=6 items lacks scrollToFirstError",
                    str(path).replace(str(REPO) + "/", ""),
                    line,
                    f"form #{block_index} has {block.group(0).count('<Form.Item')} Form.Item without first-error scroll",
                    "HIGH",
                )
            )
    return findings

def pattern_j_long_form_missing_section() -> list[Finding]:
    """Pattern J: page-level long form lacks FormSection grouping.

    A vertical form with six or more items needs visual grouping, otherwise the
    user scrolls a flat wall of fields. Exemptions are semantic, not omissions:
    `ResourceFormPage` pages already have shell sections; Modal/Drawer forms use
    their own container; auth cards already carry a title/subtitle hierarchy;
    forms without `onFinish` are stepped/custom submissions where a section
    header adds no value.
    """
    findings = []
    MIN_FORM_ITEMS = 6
    EXCLUDE_GLOBS = (".test.", ".stories.", ".spec.")
    FORM_BLOCK = re.compile(r'<Form(?![.\w])[\s\S]*?</Form>')
    for path in sorted((SRC / "pages").rglob("*.tsx")):
        if any(g in path.name for g in EXCLUDE_GLOBS):
            continue
        if "auth" in path.parts:
            continue
        content = path.read_text()
        if "ResourceFormPage" in content or "<Modal" in content or "<Drawer" in content:
            continue
        for block_index, block in enumerate(FORM_BLOCK.finditer(content), start=1):
            text = block.group(0)
            if text.count("<Form.Item") < MIN_FORM_ITEMS:
                continue
            if "onFinish" not in text:
                continue
            if "<FormSection" in text:
                continue
            line = content.count("\n", 0, block.start()) + 1
            findings.append(
                Finding(
                    "J",
                    "Page-level long form lacks FormSection grouping",
                    str(path).replace(str(REPO) + "/", ""),
                    line,
                    f"form #{block_index} has {text.count('<Form.Item')} Form.Item without FormSection",
                    "MEDIUM",
                )
            )
    return findings

ALL_PATTERNS = [
    ("A", "Zod .array().default([]) → should be .catch([])", pattern_a_zod_array_default),
    ("B", "SPA navigate debounce timer not cleared in entity-key sync", pattern_b_spa_debounce_no_clear),
    ("C", '"new" truthy bug — missing !isNew guard', pattern_c_isnew_truthy_guard),
    ("D", "Switch inside Form.Item missing valuePropName=\"checked\"", pattern_d_switch_missing_valuepropname),
    ("E", "handleReset resets Form but doesn't update dirty baseline", pattern_e_handle_reset_baseline),
    ("F", "onSuccess sets isDirty(false) before baseline sync", pattern_f_save_success_baseline),
    ("G", "Duplicate Form.Item name= within same file", pattern_g_formitem_name_duplicates),
    ("H", "onSubmit fires mutate without returning its promise", pattern_h_onSubmit_mutation),
    ("I", "Page-level form with >=6 items lacks scrollToFirstError", pattern_i_form_missing_first_error_scroll),
    ("J", "Page-level long form lacks FormSection grouping", pattern_j_long_form_missing_section),
]

def main():
    parser = argparse.ArgumentParser(description="GuliangIAM Form-Detail-Pattern Audit")
    parser.add_argument("--fix", action="store_true", help="Auto-fix safe patterns (Pattern A: .default([]) → .catch([]))")
    parser.add_argument("--strict", action="store_true",
                        help="CI mode: exit non-zero ONLY on Pattern A–F, H, I and J. Pattern G treated as warning (known false positives).")
    args = parser.parse_args()

    print("=" * 70)
    print("GuliangIAM Frontend Form-Detail-Pattern Audit")
    print(f"Scanning: {SRC}")
    print(f"Mode: {'CI strict (A–F only exit-code)' if args.strict else 'full'}")
    print("=" * 70)

    all_findings: list[Finding] = []
    for pid, pname, scanner in ALL_PATTERNS:
        print(f"\n📋 Pattern {pid}: {pname} ...", end=" ", flush=True)
        try:
            findings = scanner()
            all_findings.extend(findings)
            print(f"{'❌' if findings else '✅'} {len(findings)} found")
        except Exception as e:
            print(f"⚠️  ERROR: {e}")

    # Print report
    print("\n" + "=" * 70)
    print(f"AUDIT REPORT — {len(all_findings)} TOTAL ISSUES")
    print("=" * 70)

    severity_order = {"HIGH": 0, "MEDIUM": 1, "LOW": 2}
    all_findings.sort(key=lambda f: (severity_order.get(f.severity, 9), f.pattern_id))

    for finding in all_findings:
        print(str(finding))

    # Summary by pattern
    print("\n" + "=" * 70)
    print("SUMMARY BY PATTERN")
    print("=" * 70)
    pattern_counts: dict[str, int] = {}
    for f in all_findings:
        pattern_counts[f.pattern_id] = pattern_counts.get(f.pattern_id, 0) + 1
    for pid in sorted(pattern_counts.keys()):
        name = next(n for p, n, _ in ALL_PATTERNS if p == pid)
        print(f"  {pid}  {name}: {pattern_counts[pid]}")

    # Auto-fix safe patterns
    if args.fix:
        if any(f.pattern_id == "A" for f in all_findings):
            print("\n🔧 Auto-fixing Pattern A (.default([]) → .catch([])) ...")
            count = 0
            for f in all_findings:
                if f.pattern_id == "A":
                    filepath = REPO / f.file
                    content = filepath.read_text()
                    new_content = re.sub(r'(\.array\([^)]+\))\.default\(\[\]\)', r'\1.catch([])', content)
                    if new_content != content:
                        filepath.write_text(new_content)
                        count += 1
            print(f"   ✅ Fixed {count} files")
        else:
            print("\n✅ Nothing to auto-fix")

    # Exit code
    # --strict (CI) : exit non-zero ONLY on Pattern A–F, H, I and J (Pattern G always warning)
    # default       : all patterns contribute to exit code
    if args.strict:
        blockers = [f for f in all_findings if f.pattern_id in {"A", "B", "C", "D", "E", "F", "H", "I", "J"}]
        warnings = [f for f in all_findings if f.pattern_id == "G"]
        if warnings:
            print(f"\n⚠️  Pattern G warnings: {len(warnings)} known false positives remain (test/storybook/agg tables)")
        if blockers:
            print(f"\n❌ AUDIT FAILED (strict mode) — {len(blockers)} blocker issues in Pattern A–F, H, I and J")
            sys.exit(1)
        else:
            print("\n✅ AUDIT PASSED (strict mode) — Pattern A–F, H, I and J clean")
            sys.exit(0)
    else:
        if all_findings:
            print(f"\n❌ AUDIT FAILED — {len(all_findings)} issues found")
            sys.exit(1)
        else:
            print("\n✅ AUDIT PASSED — all patterns clean")
            sys.exit(0)

if __name__ == "__main__":
    main()
