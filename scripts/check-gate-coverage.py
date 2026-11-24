#!/usr/bin/env python3
"""门禁接线覆盖门禁：防止「文档/脚本声明了门禁，但从未被调度或根本不存在」。

Why this exists
---------------
两个真实缺陷被本门禁登记为存在理由：

  1. **孤儿门禁**：`scripts/check-*` 中有 7 个从未接入 `verify-docs.sh` / CI / Makefile，
     其中 `check-rls.sh`（租户隔离唯一结构防线）、`check-cache-registry.sh` 等还被
     `docs/README.md` §2.4 登记为「治理工具」。门禁存在、文档承诺、CI 却从不执行，
     比没有门禁更危险——它让评审者以为覆盖已经发生。
  2. **幽灵门禁**：文档中出现过 `check-domain-distribution.sh`、`check-rls-policies.sh`、
     `slo-budget-gate.sh` 三个仓库中不存在的脚本名；读者按文档复现会得到
     command not found，门禁承诺无法兑现。

调度证据口径（R1 的唯一判定依据）
--------------------------------
只承认**真实执行**证据，不承认「文件里出现过这个字符串」：

  a) `scripts/verify-docs.sh` 的 `run_gate <rule> bash|python3 [...]/scripts/x` 行，
     或同一文件中的直接 `bash|python3 [...]/scripts/x` 调用；
  b) `.github/workflows/*.yml` 中 `run:` 脚本块对门禁的实际执行（含 `VAR=... bash` 前缀）；
  c) `Makefile` recipe 行（制表符开头）中的实际执行；
  d) `INDIRECT_GATES` 白名单：仅用于确由某个 dispatcher 间接调用的门禁，条目必须写成
     `<dispatcher 文件>::<锚点>`；dispatcher 自身必须已被接线，且锚点在 dispatcher
     的非注释行中真实存在。锚点失效即 FAIL。

明确排除：被检查脚本自身、注释行、docstring/文档正文、workflow 的 `paths:` 列表、
`echo` 文本。此前 `dispatch_sources()` 把 `scripts/` 下所有 .sh/.py 都当调度源，
而每个门禁的 usage/docstring 都写了自己的文件名，于是自指匹配让孤儿门禁永远不可见。

判定规则：
  R1. 每个 `scripts/check-*` 必须命中上述 a/b/c/d 之一，或登记在 RETIRED_GATES 并写明
      退役理由；未接线门禁逐条点名。
  R2. 调度入口（verify-docs / CI run / Makefile recipe）引用的 `scripts/*` 必须真实存在。
  R3. 文档正文中反引号包裹的 `scripts/*.{py,sh,mjs}` 路径必须真实存在
      （目标态门禁不得写成既有路径，见开发规范附录 I.2「待接入门禁」）。
  R4. 文档正文中反引号包裹的任何仓库相对路径（contracts/docs/internal/db/web/
      openapi/... 的 yaml/md/go/sql/ts/... 文件）必须真实存在——事实源搬迁或文件
      删除后，文档按「既有路径」引用会误导读者。目标态产物（尚未创建的文件）在路径
      反引号后紧跟 `（待建）` 标记，例如 `` `internal/domain/tokens/subject.go`（待建） ``；
      标记只豁免该单条引用，不按文档/目录整体豁免。若路径已存在却保留标记，判为
      「标记失效」并 FAIL，防止把标记当作通用豁免。

输入：scripts/、scripts/verify-docs.sh、.github/workflows/*.yml、Makefile、docs/**/*.md
输出：PASS/FAIL + 实际接线数/未接线门禁清单 + 幽灵脚本引用清单
--check 行为：无 --check；失败即非零退出
消费方：scripts/verify-docs.sh
"""

from __future__ import annotations

import glob
import os
import re
import sys

try:
    import yaml
except ImportError:  # pragma: no cover - 运行环境缺 PyYAML 时显式失败，不静默降级
    yaml = None

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

SCRIPTS_DIR = os.path.join(REPO, "scripts")
VERIFY = os.path.join(SCRIPTS_DIR, "verify-docs.sh")
CI_GLOB = os.path.join(REPO, ".github", "workflows", "*.yml")
MAKEFILE = os.path.join(REPO, "Makefile")
DOC_GLOB = os.path.join(REPO, "docs", "**", "*.md")

# 已退役门禁：脚本仍在仓库中但不再被调度时必须在这里写明理由（禁止无理由保留）。
RETIRED_GATES: dict[str, str] = {}

# 间接调度白名单：仅用于「确由某个 dispatcher 间接调用、且 dispatcher 自身已被接线」的门禁。
# 格式：gate → "<dispatcher 仓库相对路径>::<dispatcher 中的真实执行锚点>"。
# 锚点必须出现在 dispatcher 的非注释行中；锚点失效、dispatcher 未接线或不存在均 FAIL。
INDIRECT_GATES: dict[str, str] = {
    "check-active-schema-history.py": (
        "scripts/check-migrations.sh::python3 scripts/check-active-schema-history.py"
    ),
}

SCRIPT_REF_RE = re.compile(r"scripts/([A-Za-z0-9_.-]+\.(?:py|sh|mjs))")
INLINE_CODE_RE = re.compile(r"`([^`\n]+)`")
# 目标态产物标记：必须紧跟在路径反引号之后（允许空白），只豁免该单条引用。
TARGET_STATE_MARKER = "（待建）"

ROOT_PREFIX = r'(?:(?:"?\$ROOT"?|"?\$\{ROOT\}"?)/|\./)?'
ENV_ASSIGN = r'(?:[A-Za-z_][A-Za-z0-9_]*=(?:"[^"]*"|\'[^\']*\'|\S+)\s+)*'
RUNNER = r'(?:bash|python3)\s+'
GATE_PATH = r"scripts/([A-Za-z0-9_.-]+\.(?:py|sh))"

# a) verify-docs.sh: run_gate / 直接调用（命令必须出现在行首，注释行先被剔除）
VERIFY_RUN_GATE_RE = re.compile(
    r"^run_gate\s+\S+\s+" + RUNNER + ROOT_PREFIX + GATE_PATH
)
VERIFY_DIRECT_RE = re.compile(
    r"^(?:if\s+!?\s*)?(?:exec\s+)?" + ENV_ASSIGN + RUNNER + ROOT_PREFIX + GATE_PATH
)
# b) workflow run: 块中的真实执行（允许 PATH=... / PGHOST=... 等环境变量前缀）
WORKFLOW_INVOCATION_RE = re.compile(
    r"(?:^|[;&|]\s*|\bthen\s+|\bdo\s+|\belse\s+)" + ENV_ASSIGN + RUNNER + ROOT_PREFIX + GATE_PATH
)
# c) Makefile recipe 行（调用方负责先确认行首是制表符）
MAKEFILE_INVOCATION_RE = re.compile(
    r"^(?:[@+-]\s*)?(?:[A-Za-z_][A-Za-z0-9_]*=\S+\s+)*" + RUNNER + ROOT_PREFIX + GATE_PATH
)


def read(path: str) -> str:
    with open(path, encoding="utf-8") as handle:
        return handle.read()


def effective_lines(text: str) -> list[tuple[int, str]]:
    """返回非注释行（行首空白后以 # 开头即视为注释）。"""
    return [
        (lineno, line)
        for lineno, line in enumerate(text.splitlines(), 1)
        if not line.lstrip().startswith("#")
    ]


def non_comment_text(path: str) -> str:
    return "\n".join(line for _, line in effective_lines(read(path)))


def collect_dispatch_evidence(
    gates: set[str],
) -> tuple[dict[str, list[str]], list[str], int]:
    """返回 (gate → 证据列表, problems, 扫描的调度文件数)。"""
    evidence: dict[str, list[str]] = {gate: [] for gate in gates}
    problems: list[str] = []
    scanned = 0

    # a) verify-docs.sh: run_gate 与直接调用
    if os.path.isfile(VERIFY):
        scanned += 1
        for lineno, line in effective_lines(read(VERIFY)):
            stripped = line.strip()
            for regex in (VERIFY_RUN_GATE_RE, VERIFY_DIRECT_RE):
                match = regex.match(stripped)
                if match and match.group(1) in evidence:
                    evidence[match.group(1)].append(f"scripts/verify-docs.sh:{lineno}")
    else:
        problems.append("缺少调度入口 scripts/verify-docs.sh")

    # b) .github/workflows/*.yml：只读 jobs.*.steps[].run 的实际执行
    if yaml is None:
        problems.append("缺少 PyYAML，无法解析 .github/workflows/*.yml 的 run: 证据（pip install pyyaml）")
    else:
        for path in sorted(glob.glob(CI_GLOB)):
            scanned += 1
            relpath = os.path.relpath(path, REPO)
            try:
                data = yaml.safe_load(read(path)) or {}
            except yaml.YAMLError as exc:
                problems.append(f"{relpath}: YAML 解析失败，无法判定 run: 证据：{exc}")
                continue
            for job_name, job in (data.get("jobs") or {}).items():
                if not isinstance(job, dict):
                    continue
                for step in job.get("steps") or []:
                    if not isinstance(step, dict):
                        continue
                    run = step.get("run")
                    if not isinstance(run, str):
                        continue
                    for lineno, line in enumerate(run.splitlines(), 1):
                        if not line.strip() or line.lstrip().startswith("#"):
                            continue
                        for match in WORKFLOW_INVOCATION_RE.finditer(line):
                            if match.group(1) in evidence:
                                evidence[match.group(1)].append(
                                    f"{relpath}::{job_name} run:{lineno}"
                                )

    # c) Makefile：只认 recipe 行（制表符开头），变量/注释/目标名不算调度
    if os.path.isfile(MAKEFILE):
        scanned += 1
        for lineno, line in enumerate(read(MAKEFILE).splitlines(), 1):
            if not line.startswith("\t"):
                continue
            body = line[1:].strip()
            if body.startswith("#"):
                continue
            match = MAKEFILE_INVOCATION_RE.match(body)
            if match and match.group(1) in evidence:
                evidence[match.group(1)].append(f"Makefile:{lineno}")
    else:
        problems.append("缺少调度入口 Makefile")

    return evidence, problems, scanned


def validate_indirect_gates(
    gates: set[str], direct: dict[str, list[str]], problems: list[str]
) -> dict[str, str]:
    """校验 INDIRECT_GATES 的格式、dispatcher 存在性、锚点有效性与 dispatcher 接线。"""
    valid: dict[str, str] = {}
    for gate, spec in INDIRECT_GATES.items():
        if gate not in gates:
            problems.append(f"INDIRECT_GATES 登记了不存在的门禁: {gate}")
            continue
        dispatcher_rel, sep, anchor = spec.partition("::")
        if not sep or not dispatcher_rel or not anchor:
            problems.append(
                f"INDIRECT_GATES[{gate}] 格式必须为 '<dispatcher 文件>::<锚点>'，实际 {spec!r}"
            )
            continue
        dispatcher_path = os.path.join(REPO, dispatcher_rel)
        if not os.path.isfile(dispatcher_path):
            problems.append(f"INDIRECT_GATES[{gate}] 的 dispatcher 不存在: {dispatcher_rel}")
            continue
        if anchor not in non_comment_text(dispatcher_path):
            problems.append(
                f"INDIRECT_GATES[{gate}] 锚点在 {dispatcher_rel} 的非注释行中不存在（锚点失效）: {anchor!r}"
            )
            continue
        valid[gate] = spec

    def dispatcher_is_wired(name: str, seen: set[str]) -> bool:
        if direct.get(name):
            return True
        if name in seen:
            return False
        spec = valid.get(name)
        if not spec:
            return False
        return dispatcher_is_wired(
            os.path.basename(spec.partition("::")[0]), seen | {name}
        )

    for gate in list(valid):
        dispatcher_name = os.path.basename(valid[gate].partition("::")[0])
        if dispatcher_name == "verify-docs.sh" or dispatcher_is_wired(dispatcher_name, {gate}):
            continue
        problems.append(
            f"INDIRECT_GATES[{gate}] 的 dispatcher {dispatcher_name} 自身未接线，间接登记无效"
        )
        del valid[gate]

    return valid


def main() -> int:
    problems: list[str] = []

    gates = sorted(
        os.path.basename(path)
        for path in glob.glob(os.path.join(SCRIPTS_DIR, "check-*"))
        if os.path.isfile(path)
    )
    if not gates:
        print("FAIL: scripts/check-* 未找到任何门禁脚本", file=sys.stderr)
        return 1
    gate_set = set(gates)

    evidence, evidence_problems, scanned = collect_dispatch_evidence(gate_set)
    problems.extend(evidence_problems)
    indirect = validate_indirect_gates(gate_set, evidence, problems)

    wired = [gate for gate in gates if evidence[gate] or gate in indirect]
    retired = [gate for gate in gates if gate in RETIRED_GATES]
    unwired = [
        gate for gate in gates if gate not in wired and gate not in RETIRED_GATES
    ]

    # R1：孤儿门禁逐条点名（不再用「门禁脚本总数 - 1」的算术冒充接线数）
    for gate in unwired:
        problems.append(
            f"未接线门禁 {gate}：未在 verify-docs.sh（run_gate/直接调用）、CI run:、"
            "Makefile recipe 中找到执行证据；如确由 dispatcher 间接调用，登记 INDIRECT_GATES"
        )
    retired_missing = [gate for gate in RETIRED_GATES if gate not in gate_set]
    if retired_missing:
        problems.append(f"RETIRED_GATES 登记了不存在的门禁: {retired_missing}")
    retired_but_wired = [gate for gate in retired if gate in wired]
    if retired_but_wired:
        problems.append(
            f"RETIRED_GATES 登记的 {retired_but_wired} 仍被实际调度：请删除退役登记或移除调度"
        )

    # R2：调度入口引用的脚本必须存在（只扫真实执行行，注释不算引用）
    source_texts: dict[str, str] = {}
    if os.path.isfile(VERIFY):
        source_texts["scripts/verify-docs.sh"] = "\n".join(
            line for _, line in effective_lines(read(VERIFY))
        )
    if os.path.isfile(MAKEFILE):
        source_texts["Makefile"] = "\n".join(
            line[1:]
            for line in read(MAKEFILE).splitlines()
            if line.startswith("\t") and not line[1:].lstrip().startswith("#")
        )
    if yaml is not None:
        for path in sorted(glob.glob(CI_GLOB)):
            relpath = os.path.relpath(path, REPO)
            try:
                data = yaml.safe_load(read(path)) or {}
            except yaml.YAMLError:
                continue
            run_lines: list[str] = []
            for job in (data.get("jobs") or {}).values():
                if not isinstance(job, dict):
                    continue
                for step in job.get("steps") or []:
                    if isinstance(step, dict) and isinstance(step.get("run"), str):
                        run_lines.extend(
                            line
                            for line in step["run"].splitlines()
                            if not line.lstrip().startswith("#")
                        )
            source_texts[relpath] = "\n".join(run_lines)

    for source_name, text in source_texts.items():
        for match in SCRIPT_REF_RE.finditer(text):
            referenced = match.group(1)
            if not os.path.isfile(os.path.join(SCRIPTS_DIR, referenced)):
                problems.append(f"{source_name} 引用不存在的脚本 scripts/{referenced}")

    # R3：文档中反引号里的脚本路径必须存在
    for path in sorted(glob.glob(DOC_GLOB, recursive=True)):
        relpath = os.path.relpath(path, REPO)
        for lineno, line in enumerate(read(path).split("\n"), 1):
            for match in INLINE_CODE_RE.finditer(line):
                for script_match in SCRIPT_REF_RE.finditer(match.group(1)):
                    referenced = script_match.group(1)
                    if not os.path.isfile(os.path.join(SCRIPTS_DIR, referenced)):
                        problems.append(
                            f"{relpath}:{lineno}: 引用不存在的门禁脚本 scripts/{referenced}"
                        )

    # R4：文档中反引号里的仓库相对路径必须存在。事实源/实现文件被搬迁、删除或改名后，
    # 文档若继续以「既有路径」引用，读者按图索骥必然落空；目标态产物不得写成既有路径，
    # 应改为「（目标态，待接入）+ 开发规范附录 I.2」的表述。
    path_ref_re = re.compile(
        r"^(contracts|docs|scripts|internal|db|web|openapi|cmd|deploy|tests|cred)/"
        r"[^`\s]+\.(?:yaml|yml|md|go|sql|json|ts|tsx|sh|py|mjs|css|html|mod|sum)$"
    )
    for path in sorted(glob.glob(DOC_GLOB, recursive=True)):
        relpath = os.path.relpath(path, REPO)
        for lineno, line in enumerate(read(path).split("\n"), 1):
            for match in INLINE_CODE_RE.finditer(line):
                candidate = match.group(1).strip()
                if (
                    "*" in candidate
                    or "{" in candidate
                    or "<" in candidate
                    or " " in candidate
                    or candidate.endswith("/")
                ):
                    continue
                if not path_ref_re.match(candidate):
                    continue
                exists = os.path.exists(os.path.join(REPO, candidate))
                marked_target = line[match.end():].lstrip().startswith(TARGET_STATE_MARKER)
                if exists and marked_target:
                    problems.append(
                        f"{relpath}:{lineno}: {candidate} 标记为{TARGET_STATE_MARKER}"
                        "但路径已存在；标记失效，请移除"
                    )
                    continue
                if not exists and not marked_target:
                    problems.append(
                        f"{relpath}:{lineno}: 引用不存在的仓库路径 {candidate}（事实源已搬迁/删除，或目标态产物写成了既有路径）"
                    )

    if problems:
        print(f"=== check-gate-coverage.py: FAIL（{len(problems)} 处）===", file=sys.stderr)
        for problem in problems:
            print(f"  - {problem}", file=sys.stderr)
        print(
            "接线口径：verify-docs.sh 的 run_gate/直接调用、CI workflow 的 run:、Makefile recipe；"
            "确由 dispatcher 间接调用的门禁登记 INDIRECT_GATES（文件::锚点）并保证锚点有效。",
            file=sys.stderr,
        )
        return 1

    evidence_count = sum(len(items) for items in evidence.values())
    print(
        f"PASS: 门禁接线完整（{len(gates)} 个门禁脚本：实际接线 {len(wired)} 个 / "
        f"未接线 {len(unwired)} 个 / 已退役登记 {len(retired)} 个；"
        f"调度证据 {evidence_count} 条（扫描 {scanned} 个调度文件）；"
        f"文档脚本与仓库路径引用全部可解析）"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
