#!/usr/bin/env python3
"""Check that every translated string the console asks for actually exists.

Why this exists
---------------
i18next does not fail loudly. When a key is missing it returns the key itself, so
the UI renders the literal token instead of a sentence — no exception, no crash,
no red build. Three separate ways this shipped in this repository:

  1. **A namespace that was never registered.** `useTranslation("device")` in
     `DeviceVerifyPage` asked for a namespace absent from the `ns` array in
     `web/src/shared/i18n/index.ts`, so the whole device-authorization page
     rendered `deviceAuth`, `enterCode`, `authorizeDevice` as visible text.
     The strings already existed in `common`; only the request was wrong.

  2. **A key referenced from the wrong namespace.** `LdapTable` holds
     `useTranslation("common")` while every `ldap.*` string lives in `resource`,
     so its column headers were raw keys.

  3. **A locale that never received a key.** `dashboard.activityHeatmap` existed
     in two of fourteen language packs. A Brazilian or Turkish operator saw the
     English fallback chain end at the key itself.

None of the three is visible to `tsc`, to `oxlint`, or to a component test that
mocks `useTranslation`. They are all decidable from the source, which is what this
script does.

Rules
-----
  1. **Namespace contract.** Every `useTranslation("x")` argument and every
     `"x:key"` prefix must be declared in the `ns` array of
     `web/src/shared/i18n/index.ts`, which is the single source of truth for
     which namespaces exist. Hard failure.

  2. **Key resolvability.** A call must resolve in the namespace *its own
     translator* is bound to — `const { t: tr } = useTranslation("resource")` makes
     `tr("x")` a `resource` lookup and nothing else, so a key that lives in
     `common` is not reachable through `tr`. Calls that pass no inline fallback
     must resolve; a call with an inline fallback (`t("key", "Text")`) is
     deliberately exempt from the hard failure because i18next renders the
     fallback and the user never sees a raw key. Failures are matched against
     `scripts/i18n-key-baseline.txt` so the existing backlog is a no-regression
     ratchet instead of a blocked build: a *new* offender fails, a *repaired*
     entry must be removed from the baseline (stale entries are reported and
     fail), and the baseline may only shrink.

  3. **Locale parity.** Every key of every namespace must exist in every language
     pack under `web/src/shared/i18n/locales/`. Hard failure.

  4. **Key-tree structure (R1–R3).** i18next treats `.` as a path separator, so a
     bundle whose shape disagrees with its call sites silently loses subtrees:

       * R1 — a key name must not itself contain `.` or `:`; such a name is
         indistinguishable from a path and shadows the real subtree.
       * R2 — no top-level group may be named after a namespace; `common.common`
         made `t("common.foo")` resolve *inside* `common` while call sites meant
         "the `foo` of the `common` namespace".
       * R3 — a path must not be both a leaf and a group; once a prefix is a
         string, everything below it is unreachable.

     All three were present and all three are decidable from the files, so they
     are hard failures.

  5. **Dotted namespace heads (R4).** An inline fallback silently hides a key that
     does not resolve anywhere, so the string never leaves English. The most common
     cause is a dotted reference: `t("common.saved")` looks like a path but the
     dotted head is *not* a namespace selector, only `:` is, so the key is read as
     the literal path `common.saved` inside the bound namespace. A key whose head
     names a declared namespace therefore fails hard, whether or not a fallback is
     present; fallback-carrying misses that are *not* of that shape stay on the
     baseline ratchet.

  6. **Key ownership (R5).** The same key path living in two namespaces is two
     sources of truth that drift apart. Hard failure for new pairs; the existing
     pairs are recorded in `scripts/i18n-key-ownership-baseline.txt` and that file
     may only shrink.

Counts are never written down in `docs/`; the reproduction command is
`python3 scripts/check-i18n-keys.py`.

输入：web/src/ 中 useTranslation + t()、web/src/shared/i18n/index.ts ns 列表、各语言包
输出：PASS/FAIL + 缺失命名空间/缺失键/未覆盖 locale 列表
--check 行为：无 --check；失败即 exit 1
消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
"""

from __future__ import annotations

import argparse
import collections
import json
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
WEB = ROOT / "web/src"
I18N_INIT = WEB / "shared/i18n/index.ts"
LOCALES = WEB / "shared/i18n/locales"
BASELINE = ROOT / "scripts/i18n-key-baseline.txt"
OWNERSHIP_BASELINE = ROOT / "scripts/i18n-key-ownership-baseline.txt"

# `useTranslation("common")` / `useTranslation(["common", "auth"])`
USE_TRANSLATION = re.compile(r"""useTranslation\(\s*(\[[^\]]*\]|"[^"]*"|'[^']*')""")
# `const { t } = useTranslation("resource")` / `const { t: tr } = useTranslation("resource")`
# The translator name is what a call site actually resolves against, so the alias
# — not the file — decides which bundle a key is looked up in.
DESTRUCTURE = re.compile(r"const\s*\{([^}]*)\}\s*=\s*useTranslation\s*\(\s*([^)]*)\)", re.S)
# Any `name("key"` call: the identifier is filtered against the alias map, the
# trailing character tells an inline fallback string from an options object.
T_CALL = re.compile(r"""\b([A-Za-z_$][\w$]*)\s*\(\s*"([^"\\]+)"\s*(.)""", re.S)
NS_ARRAY = re.compile(r"""\bns:\s*\[([^\]]*)\]""")
QUOTED = re.compile(r'"([^"]*)"' + "|" + r"'([^']*)'")


def quoted_literals(text: str) -> list[str]:
    """Return all single/double quoted literals in text (source may use either style)."""
    return [a or b for a, b in QUOTED.findall(text)]


def git_show(relative: str) -> str:
    """Read the staged (index) copy of a path, never the working tree."""
    result = subprocess.run(
        ["git", "show", f":{relative}"],
        cwd=ROOT,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        sys.exit(f"FAIL: 无法读取暂存副本 {relative}: {result.stderr.strip()}")
    return result.stdout


def staged_source_files() -> list[str]:
    """Source files staged for this commit (index copies only)."""
    result = subprocess.run(
        ["git", "diff", "--cached", "--name-only", "--diff-filter=ACMR", "--", "web/src"],
        cwd=ROOT,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        sys.exit(f"FAIL: 无法列出暂存文件: {result.stderr.strip()}")
    files: list[str] = []
    for relative in result.stdout.splitlines():
        if not relative.endswith((".ts", ".tsx")):
            continue
        name = Path(relative).name
        if name.endswith(".d.ts") or ".test." in name or ".spec." in name:
            continue
        files.append(relative)
    return sorted(files)


def declared_namespaces(staged: bool = False) -> list[str]:
    """The namespaces i18next is initialised with, read from the init call."""
    text = git_show(str(I18N_INIT.relative_to(ROOT))) if staged else I18N_INIT.read_text(
        encoding="utf-8"
    )
    match = NS_ARRAY.search(text)
    if not match:
        sys.exit(f"FAIL: no `ns: [...]` array found in {I18N_INIT.relative_to(ROOT)}")
    return quoted_literals(match.group(1))


def flatten(node: dict, prefix: str = "") -> set[str]:
    keys: set[str] = set()
    for name, value in node.items():
        path = f"{prefix}{name}"
        if isinstance(value, dict):
            keys |= flatten(value, f"{path}.")
        else:
            keys.add(path)
    return keys


def locale_files(locales: list[str], namespace: str) -> list[Path]:
    return [LOCALES / lang / f"{namespace}.json" for lang in locales]


def language_dirs() -> list[str]:
    return sorted(child.name for child in LOCALES.iterdir() if child.is_dir())


def load_bundle(language: str, namespace: str, staged: bool = False) -> dict:
    relative = str((LOCALES / language / f"{namespace}.json").relative_to(ROOT))
    if staged:
        return json.loads(git_show(relative))
    return json.loads((LOCALES / language / f"{namespace}.json").read_text(encoding="utf-8"))


def load_locales(staged: bool = False) -> tuple[dict[str, set[str]], dict[str, set[str]]]:
    """Return (keys per namespace, keys per namespace shared by all locales)."""
    languages = language_dirs()
    per_ns: dict[str, set[str]] = {}
    per_ns_common: dict[str, set[str]] = {}
    for namespace in declared_namespaces(staged):
        files = locale_files(languages, namespace)
        if not files or not files[0].exists():
            sys.exit(f"FAIL: no locale files for namespace '{namespace}' under {LOCALES}")
        key_sets = [flatten(load_bundle(lang, namespace, staged)) for lang in languages]
        per_ns[namespace] = set().union(*key_sets)
        per_ns_common[namespace] = set.intersection(*key_sets)
    return per_ns, per_ns_common


def structure_paths(node: dict, prefix: str = "") -> dict[str, str]:
    """Every path in a bundle and whether it holds a string or a subtree."""
    out: dict[str, str] = {}
    for name, value in node.items():
        path = f"{prefix}{name}"
        if isinstance(value, dict):
            out[path] = "group"
            out.update(structure_paths(value, f"{path}."))
        else:
            out[path] = "leaf"
    return out


def structure_violations(staged: bool = False) -> list[str]:
    """R1–R3: the key tree must agree with the separator semantics of i18next."""
    violations: list[str] = []
    declared = set(declared_namespaces(staged))
    for namespace in declared_namespaces(staged):
        for language in language_dirs():
            node = load_bundle(language, namespace, staged)
            where = f"locales/{language}/{namespace}.json"
            shaped = structure_paths(node)
            for path, kind in sorted(shaped.items()):
                name = path.rsplit(".", 1)[-1]
                if "." in name or ":" in name:
                    violations.append(f"{where}: 键名 '{path}' 含分隔符，与路径语法无法区分（R1）")
                if kind == "leaf" and any(other.startswith(f"{path}.") for other in shaped):
                    violations.append(
                        f"{where}: 路径 '{path}' 既是叶子又是组，其子树不可达（R3）"
                    )
            for head, value in sorted(node.items()):
                if head in declared and isinstance(value, dict):
                    violations.append(
                        f"{where}: 顶层组 '{head}' 与命名空间同名，遮蔽同名键（R2）"
                    )
    return violations


def ownership_violations(staged: bool = False) -> list[str]:
    """R5: one key path living in more than one namespace is two sources of truth."""
    owners: dict[str, set[str]] = {}
    for namespace in declared_namespaces(staged):
        for language in language_dirs():
            for key in flatten(load_bundle(language, namespace, staged)):
                owners.setdefault(key, set()).add(namespace)
    return [
        f"{'|'.join(sorted(namespaces))}: {key}"
        for key, namespaces in sorted(owners.items())
        if len(namespaces) > 1
    ]


def source_files() -> list[Path]:
    files: list[Path] = []
    for path in sorted(WEB.rglob("*")):
        if not path.is_file() or path.suffix not in {".ts", ".tsx"}:
            continue
        name = path.name
        if name.endswith(".d.ts") or ".test." in name or ".spec." in name:
            continue
        files.append(path)
    return files


def file_namespaces(
    text: str, declared: set[str], relative: str
) -> tuple[set[str], dict[str, set[str]], list[str]]:
    """Namespaces a file resolves against, its translator aliases, and contract violations.

    The alias map is the crux: `const { t: tr } = useTranslation("resource")` means
    `tr("plan.title")` resolves in `resource` only. Reading call sites by the literal
    name `t` hides every aliased call, which is how an entire class of raw-key
    rendering shipped unnoticed.
    """
    violations: list[str] = []
    namespaces: set[str] = set()
    for match in USE_TRANSLATION.finditer(text):
        for literal in quoted_literals(match.group(1)):
            if literal not in declared:
                violations.append(f"{relative}: useTranslation(\"{literal}\") — 未注册的命名空间")
            namespaces.add(literal)
    if not namespaces:
        namespaces = {"common"}  # i18next defaultNS
    aliases: dict[str, set[str]] = {}
    for destructured, args in DESTRUCTURE.findall(text):
        bound = set(quoted_literals(args)) or {"common"}  # i18next defaultNS
        for part in destructured.split(","):
            left, _, right = part.strip().partition(":")
            if left.strip() != "t":
                continue
            name = right.strip() or "t"
            aliases.setdefault(name, set()).update(bound)
    if not aliases:
        aliases["t"] = set(namespaces)
    return namespaces, aliases, violations


def key_violations(
    text: str, relative: str, aliases: dict[str, set[str]], per_ns: dict[str, set[str]], declared: set[str]
) -> tuple[list[str], list[str], list[str]]:
    """Return (unresolvable keys, R4 offenders behind an inline fallback, hard R4 dotted heads)."""
    violations: list[str] = []
    masked: list[str] = []
    hard: list[str] = []
    for match in T_CALL.finditer(text):
        alias, key, next_char = match.groups()
        candidates = aliases.get(alias)
        if not candidates:
            continue
        rest = text[match.end() : match.end() + 48]
        has_fallback = next_char == "," and rest.lstrip().startswith('"')
        finding, is_hard = _check_key(key, candidates, per_ns, declared)
        if not finding:
            continue
        call = f'{relative}: {alias}("{key}")'
        if is_hard:
            hard.append(f"{call} — {finding}")
        elif has_fallback:
            masked.append(f'{call}, "…" — 内联兜底掩盖了{finding}')
        else:
            violations.append(f"{call} — {finding}")
    return violations, masked, hard


def _check_key(
    key: str, candidates: set[str], per_ns: dict[str, set[str]], declared: set[str]
) -> tuple[str | None, bool]:
    """Resolve one key against the namespaces its own translator is bound to."""
    prefix = key.split(":", 1)[0] if ":" in key else ""
    if prefix and prefix in declared:
        lookup, candidates = key.split(":", 1)[1], {prefix}
    elif prefix and prefix not in declared and prefix.isalnum():
        return f"命名空间前缀 '{prefix}' 未注册", True
    else:
        lookup = key
    head, _, tail = key.partition(".")
    if not prefix and head in declared and tail:
        spelled = f"{head}:{tail}"
        if tail in per_ns.get(head, set()):
            return (
                f'"{key}" 属于命名空间 {head}，但 {", ".join(sorted(candidates))} 不是它：'
                f'点号不是命名空间分隔符，应改用绑定 {head} 的 translator 写作 "{tail}"'
                f'（或显式写作 "{spelled}"）（R4）'
            ), True
        return f'点号不是命名空间分隔符："{key}" 被当作 {", ".join(sorted(candidates))} 中的路径，应写作 "{spelled}"（R4）', True
    if any(lookup in per_ns.get(ns, set()) for ns in candidates):
        return None, False
    homes = sorted(ns for ns, keys in per_ns.items() if lookup in keys)
    where = f"（该键存在于 {', '.join(homes)}）" if homes else "（该键在所有语言包中均不存在）"
    return f"在 {', '.join(sorted(candidates))} 中无法解析{where}", False


def check_locale_parity(
    per_ns: dict[str, set[str]], per_ns_common: dict[str, set[str]], staged: bool = False
) -> list[str]:
    violations: list[str] = []
    languages = language_dirs()
    for namespace, keys in per_ns.items():
        missing = keys - per_ns_common[namespace]
        for key in sorted(missing):
            owners = [
                lang
                for lang in languages
                if key in flatten(load_bundle(lang, namespace, staged))
            ]
            absent = [lang for lang in languages if lang not in owners]
            violations.append(
                f"{namespace}.{key} — 缺 {len(absent)} 个语言包（有：{', '.join(owners)}）"
            )
    return violations


def read_baseline(path: Path) -> list[str]:
    if not path.exists():
        return []
    lines = path.read_text(encoding="utf-8").splitlines()
    return [line.strip() for line in lines if line.strip() and not line.startswith("#")]


def write_baseline(path: Path, header: str, entries: list[str]) -> None:
    path.write_text(
        f"# {header}\n" + "".join(f"{entry}\n" for entry in entries), encoding="utf-8"
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--staged",
        action="store_true",
        help="只检查本次提交暂存的内容（读取 git 索引副本，不读工作树）",
    )
    parser.add_argument(
        "--update-baseline",
        action="store_true",
        help="重写基线文件为当前违规集合（仅在有意收敛/记账时使用）",
    )
    args = parser.parse_args()
    if args.staged and args.update_baseline:
        parser.error("--staged 模式下不允许 --update-baseline")

    declared = declared_namespaces(args.staged)
    per_ns, per_ns_common = load_locales(args.staged)
    hard: list[str] = []
    key_findings: list[str] = []
    r4_findings: list[str] = []

    if args.staged:
        source_texts = [(relative, git_show(relative)) for relative in staged_source_files()]
    else:
        source_texts = [
            (str(path.relative_to(ROOT)), path.read_text(encoding="utf-8"))
            for path in source_files()
        ]

    for relative, text in source_texts:
        namespaces, aliases, ns_violations = file_namespaces(text, set(declared), relative)
        hard.extend(ns_violations)
        found, masked, dotted = key_violations(text, relative, aliases, per_ns, set(declared))
        key_findings.extend(found)
        r4_findings.extend(masked)
        hard.extend(dotted)

    structure = structure_violations(args.staged)
    hard.extend(structure)
    parity = check_locale_parity(per_ns, per_ns_common, args.staged)
    ownership = ownership_violations(args.staged)

    identifiers = sorted({finding.split(" — ")[0] for finding in key_findings})
    r4_identifiers = sorted({finding.split(" — ")[0] for finding in r4_findings})

    if args.update_baseline:
        write_baseline(
            BASELINE,
            "i18n 键解析基线：只允许删除，不允许新增（见 scripts/check-i18n-keys.py）",
            sorted(set(identifiers) | set(r4_identifiers)),
        )
        write_baseline(
            OWNERSHIP_BASELINE,
            "i18n 键归属基线：同一键路径不得新增第二个归属命名空间（见 scripts/check-i18n-keys.py）",
            ownership,
        )
        print(
            f"baseline 已更新：键解析 {len(identifiers) + len(r4_identifiers)} 条 → "
            f"{BASELINE.relative_to(ROOT)}；键归属 {len(ownership)} 条 → "
            f"{OWNERSHIP_BASELINE.relative_to(ROOT)}"
        )
        return 0

    baseline = set(read_baseline(BASELINE))
    ownership_baseline = set(read_baseline(OWNERSHIP_BASELINE))
    tracked = identifiers + r4_identifiers
    new = sorted(set(tracked) - baseline)
    stale = sorted(baseline - set(tracked))
    new_owners = sorted(set(ownership) - ownership_baseline)
    stale_owners = sorted(ownership_baseline - set(ownership))
    if args.staged:
        # Ratchet/staleness invariants are properties of the whole tree; a staged
        # subset cannot observe them, so only "new violation" is enforced here.
        stale = []
        stale_owners = []

    if hard:
        distinct = collections.Counter(hard)
        print(f"FAIL: {len(hard)} 处命名空间契约/键树结构违规（{len(distinct)} 类）：")
        for item, count in sorted(distinct.items()):
            suffix = f"（{count} 处调用）" if count > 1 else ""
            print(f"  - {item}{suffix}")
    if parity:
        print(f"FAIL: {len(parity)} 处语言包键不一致：")
        for item in parity[:20]:
            print(f"  - {item}")
    if new:
        print(f"FAIL: {len(new)} 处新增的键解析失败（基线外）：")
        for item in new:
            print(f"  - {item}")
    if new_owners:
        print(f"FAIL: {len(new_owners)} 个键新增了重复归属（基线外）：")
        for item in new_owners:
            print(f"  - {item}")
    if stale:
        print(f"FAIL: {len(stale)} 条键解析基线条目已不再违规，请执行 --update-baseline 收敛：")
        for item in stale[:20]:
            print(f"  - {item}")
    if stale_owners:
        print(
            f"FAIL: {len(stale_owners)} 条键归属基线条目已不再违规，"
            f"请执行 --update-baseline 收敛："
        )
        for item in stale_owners[:20]:
            print(f"  - {item}")

    if hard or parity or new or new_owners or stale or stale_owners:
        return 1

    print(
        f"PASS: {len(declared)} 个命名空间契约成立；键树结构 R1–R3 全绿；"
        f"语言包键全量一致；内联兜底掩盖的缺失键 R4 记账 {len(r4_identifiers)} 条；"
        f"键解析基线记账 {len(baseline)} 条（新增 0 条）；"
        f"键归属 R5 基线记账 {len(ownership_baseline)} 条（新增 0 条）"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
