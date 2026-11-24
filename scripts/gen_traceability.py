#!/usr/bin/env python3

"""扫描全仓文档锚点，生成 docs/traceability-matrix.md 追溯矩阵

输入：docs/requirements/**/*.md（需求表格行定义与引用）、docs/design/**/*.md [DESIGN]、
      docs/acceptance-criteria.md [AC]、docs/adr/*.md [ADR]、contracts/registries/open-items.yaml [OPEN]
输出：docs/traceability-matrix.md；--check 模式零差异 + 未定义引用门禁
--check 行为：比较当前 matrix 与重新扫描生成结果的差异；校验显式需求引用是否都有逐号定义；
             并校验 ADR 区间与 docs/adr 实际文件集合一致，无差异且无 UNDEFINED 时 PASS
消费方：check-generated-zero-diff.sh、CI

定义模型（唯一事实源）：
  需求 ID 的逐号定义只允许是 docs/requirements/**/*.md 的表格行 `| ID | 描述 | 优先级 |`。
  章节锚点 `[REQ:FAM-001 ~ FAM-00N]` 只声明编号空间，不逐号展开；其他文档中的 `[REQ:ID]`、
  `### AC-*` 标题括号引用、`SRS` 引用一律是引用，必须命中定义集，否则 --check 失败。"""

from __future__ import annotations

import argparse
import re
import sys
from collections import defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
TRACEABILITY_OUTPUT = ROOT / "docs/traceability-matrix.md"
REQUIREMENTS_DIR = ROOT / "docs" / "requirements"
DESIGN_DIR = ROOT / "docs" / "design"
ADR_DIR = ROOT / "docs" / "adr"
AC_FILE = ROOT / "docs" / "acceptance-criteria.md"
ADR_FILE_RE = re.compile(r'^ADR-(\d{3})-[a-z0-9-]+\.md$')
ADR_COUNT_CELL_RE = re.compile(r'^\*\*\d+\*\*$')


class AdrRangeError(RuntimeError):
    """docs/adr 无法推导 ADR 编号区间时抛出，拒绝回退到写死区间。"""


# 扫描目录（相对 ROOT）
SCAN_DIRS = [
    "docs/requirements",
    "docs/design",
    "docs",               # acceptance-criteria.md 在 docs 根目录
    "docs/adr",
    # "docs/registries", — 渲染层 md 不扫描（会污染追溯矩阵）；仅 open-items.yaml 独立处理
    # docs/generated/（注册表渲染层）同理由 EXCLUDE_DIRS 排除
]

# 需求编号：支持多段族（DATA-WR-001、DB-REQ-005）与含数字前缀（B2B-001）。
# 结尾 \b 防止把 `FE-0nn` 截成 `FE-0`。
REQ_ID_RE = re.compile(r'([A-Z0-9]+(?:-[A-Z0-9]+)*-\d+)\b')

# AC 编号：AC-TOKEN-001 / AC-AUTH-001 / AC-DATA-001；无括号也可识别。
AC_ID_RE = re.compile(r'AC-[A-Z0-9]+(?:-[A-Z0-9]+)*-\d+\b')

# 表格行定义：只认以 ID 开头的首列行。
ROW_DEF_RE = re.compile(r'^\s*\|[ \t]*([A-Z0-9]+(?:-[A-Z0-9]+)*-\d+)[ \t]*\|')

# 锚点正则（设计路径允许连字符：ai-gateway、casdoor-parity-checklist）
ANCHOR_PATTERNS = {
    "DESIGN": re.compile(r'\[DESIGN:([A-Za-z0-9_./-]+)\]'),
    "AC":     re.compile(r'\[AC:([A-Z0-9]+(?:-[A-Z0-9]+)*-\d+)\]'),
    "ADR":    re.compile(r'\[ADR:(\d{3})\]'),
    "OPEN":   re.compile(r'\[OPEN:([\w-]+)\]'),
}
REQ_ANCHOR_RE = re.compile(r'\[REQ:([^\]]+)\]')

# 行内式正则（从正文文本中提取）
INLINE_PATTERNS = {
    "AC":     AC_ID_RE,
    "ADR":    re.compile(r'\b(ADR-\d{3})\b'),
    "OPEN":   re.compile(r'\b(OPEN-[\w-]+)\b'),
    "DESIGN": re.compile(r'\bDESIGN\s+(\d+(?:\.\d+)*)\b'),
}

# 非需求前缀：避免把算法名、章节号、审计发现号当成未定义需求引用。
NON_REQ_PREFIXES = {
    "AC", "ADR", "OPEN", "API", "DESIGN", "ARCH", "RLS", "AES", "SHA", "UTF",
    "EIP", "RIP", "RFC", "TLS", "SPIFFE", "SCRAM", "SSE", "CAEP", "RISC",
    "JWT", "JSON", "HTTP", "HTTPS", "URL", "URI", "IP",
}

# 范围声明：AUTH-001 ~ AUTH-009 / AUTH-094~098（第二个族名可省略）
RANGE_RE = re.compile(
    r'([A-Z0-9]+(?:-[A-Z0-9]+)*)-(\d+)\s*[~～]\s*'
    r'(?:([A-Z0-9]+(?:-[A-Z0-9]+)*)-)?(\d+)\b'
)
# 斜杠枚举：AUTH-094/098、ORG-016/019/020、RISK-013/014
SLASH_RE = re.compile(
    r'([A-Z0-9]+(?:-[A-Z0-9]+)*)-(\d+)/(\d+)(?:/(\d+))?\b'
)


def _iter_markdown_files() -> list[Path]:
    """收集所有要扫描的 .md 文件（去重）。

    排除：
      - traceability-matrix.md（生成物，循环自引用）
      - docs/generated/*.md（注册表渲染层，不含正式锚点定义；会污染追溯矩阵）
      - docs/technical-notes/**（一次性历史审计文档）
    """
    seen: set[Path] = set()
    files: list[Path] = []
    # CODE_WIKI.md is non-formal and rejected by the docs-root governance gate.
    EXCLUDE_NAMES = {"traceability-matrix.md", "CODE_WIKI.md"}
    EXCLUDE_DIRS = {"registries", "technical-notes", "generated"}
    for dir_rel in SCAN_DIRS:
        base = ROOT / dir_rel
        if not base.exists():
            continue
        for p in base.rglob("*.md"):
            if p.name in EXCLUDE_NAMES:
                continue
            if p.parent.parent.name == "docs" and p.parent.name in EXCLUDE_DIRS:
                continue
            if "technical-notes" in p.parts:
                continue
            if p in seen:
                continue
            seen.add(p)
            files.append(p)
    return sorted(files)


def _collect_definitions() -> tuple[dict[str, str], list[str]]:
    """收集需求逐号定义：requirements 目录内的表格行。

    返回 ({id: 定义文件}, 重复定义错误列表)。
    """
    defs: dict[str, str] = {}
    duplicates: list[str] = []
    for p in sorted(REQUIREMENTS_DIR.rglob("*.md")):
        rel = str(p.relative_to(ROOT))
        for lineno, line in enumerate(p.read_text(encoding="utf-8", errors="replace").splitlines(), 1):
            m = ROW_DEF_RE.match(line)
            if not m:
                continue
            ident = m.group(1)
            if ident in defs:
                duplicates.append(f"{ident} 重复定义：{defs[ident]} 与 {rel}:{lineno}")
                continue
            defs[ident] = rel
    return defs, duplicates


def _expand_line_forms(line: str, expand_ranges: bool = True) -> tuple[set[str], str]:
    """展开一行中的范围/斜杠引用，返回 (ID 集合, 掩码后的行)。

    范围默认按引用展开；expand_ranges=False 时仅掩码（视为编号空间声明）。
    """
    ids: set[str] = set()
    masked = line
    for m in RANGE_RE.finditer(line):
        fam1, start, fam2, end = m.group(1), int(m.group(2)), m.group(3) or m.group(1), int(m.group(4))
        if expand_ranges and fam1 == fam2 and start <= end:
            ids.update(f"{fam1}-{n:03d}" for n in range(start, end + 1))
        masked = masked.replace(m.group(0), " " * len(m.group(0)))
    for m in SLASH_RE.finditer(masked):
        fam, first, second, third = m.group(1), int(m.group(2)), int(m.group(3)), m.group(4)
        nums = [first, second] + ([int(third)] if third else [])
        ids.update(f"{fam}-{n:03d}" for n in nums)
        masked = masked.replace(m.group(0), " " * len(m.group(0)))
    return ids, masked


def _is_ignored_req_token(ident: str) -> bool:
    prefix = ident.split("-")[0]
    if prefix.isdigit():
        return True
    return prefix in NON_REQ_PREFIXES


def _collect_strict_problems(defs: dict[str, str]) -> list[str]:
    """校验三类显式引用上下文中的需求编号都有逐号定义。

    A. 任意文档中的 [REQ:单号] 锚点（requirements 内的范围声明跳过）
    B. `### AC-*` 标题行括号中的编号（含斜杠枚举）
    C. 含 `SRS` 的行中的编号（范围跳过，单号与斜杠枚举校验）
    """
    problems: list[str] = []
    for p in _iter_markdown_files():
        rel = str(p.relative_to(ROOT))
        in_requirements = p.is_relative_to(REQUIREMENTS_DIR)
        for lineno, line in enumerate(p.read_text(encoding="utf-8", errors="replace").splitlines(), 1):
            loc = f"{rel}:{lineno}"

            # A. [REQ:...] 锚点
            for m in REQ_ANCHOR_RE.finditer(line):
                content = m.group(1)
                for seg in content.split(","):
                    seg = seg.strip()
                    if "~" in seg or "～" in seg:
                        continue  # 编号空间声明，不逐号校验
                    sm = REQ_ID_RE.fullmatch(seg)
                    if not sm:
                        continue  # MODULE-* 等非逐号锚点
                    ident = sm.group(1)
                    if ident not in defs:
                        problems.append(f"UNDEFINED: {ident} <- {loc} ([REQ:] 锚点)")

            # B. AC 标题行
            if re.match(r'^###\s+AC-', line):
                masked = AC_ID_RE.sub(lambda m: " " * len(m.group(0)), line)
                forms, masked = _expand_line_forms(masked, expand_ranges=False)
                singles = [m.group(1) for m in REQ_ID_RE.finditer(masked)]
                for ident in sorted(forms) + singles:
                    if _is_ignored_req_token(ident):
                        continue
                    if ident not in defs:
                        problems.append(f"UNDEFINED: {ident} <- {loc} (AC 标题引用)")

            # C. SRS 引用行
            if "SRS" in line:
                masked = AC_ID_RE.sub(lambda m: " " * len(m.group(0)), line)
                forms, masked = _expand_line_forms(masked, expand_ranges=False)
                singles = [m.group(1) for m in REQ_ID_RE.finditer(masked)]
                for ident in sorted(forms) + singles:
                    if _is_ignored_req_token(ident):
                        continue
                    if ident not in defs:
                        problems.append(f"UNDEFINED: {ident} <- {loc} (SRS 引用)")
    return problems


def _validate_design_anchors() -> list[str]:
    """校验设计锚点：一份设计文档恰好一个 [DESIGN:path]，且值等于相对路径（去 .md）。

    docs/design/README.md 是索引，不得携带锚点；锚点不得重复或指向不存在的设计文档。
    """
    problems: list[str] = []
    if not DESIGN_DIR.exists():
        return [f"MISSING: 设计目录 {DESIGN_DIR.relative_to(ROOT)} 不存在"]

    index_file = DESIGN_DIR / "README.md"
    expected: dict[str, Path] = {}
    for p in sorted(DESIGN_DIR.rglob("*.md")):
        if p == index_file:
            continue
        expected[str(p.relative_to(DESIGN_DIR).with_suffix(""))] = p

    seen: dict[str, list[str]] = defaultdict(list)
    for p in sorted(DESIGN_DIR.rglob("*.md")):
        rel = str(p.relative_to(DESIGN_DIR))
        text = p.read_text(encoding="utf-8", errors="replace")
        anchors = ANCHOR_PATTERNS["DESIGN"].findall(text)
        if p == index_file:
            if anchors:
                problems.append(f"{rel}: 索引文档不得携带设计锚点（发现 {len(anchors)} 个）")
            continue
        if len(anchors) != 1:
            problems.append(f"{rel}: 设计锚点数量为 {len(anchors)}（必须恰好 1 个）")
            continue
        anchor = anchors[0]
        want = str(p.relative_to(DESIGN_DIR).with_suffix(""))
        if anchor != want:
            problems.append(f"{rel}: 设计锚点值 `{anchor}` 与路径 `{want}` 不一致")
        seen[anchor].append(rel)

    for anchor, files in sorted(seen.items()):
        if len(files) > 1:
            problems.append(f"设计锚点 `{anchor}` 重复出现在: {', '.join(files)}")
        if anchor not in expected:
            problems.append(f"设计锚点 `{anchor}` 未指向 docs/design 下的设计文档")
    for anchor in sorted(set(expected) - set(seen)):
        problems.append(f"MISSING: 设计文档 `{expected[anchor].relative_to(ROOT)}` 缺少 [DESIGN:{anchor}] 锚点")
    return problems


def _extract_anchors(text: str, file_rel: str, defs: dict[str, str]) -> dict[str, set[str]]:
    """从单文件文本中提取锚点：REQ 只保留定义集内条目，AC/ADR/OPEN/DESIGN 保持原语义。"""
    anchors: dict[str, set[str]] = {
        "REQ": set(), "DESIGN": set(), "AC": set(), "ADR": set(), "OPEN": set()
    }

    # 1. 括号式锚点（DESIGN/AC/ADR/OPEN）
    for kind, pat in ANCHOR_PATTERNS.items():
        for m in pat.finditer(text):
            anchors[kind].add(m.group(1).strip())

    # 2. 行内 AC / ADR / DESIGN / OPEN
    for m in INLINE_PATTERNS["AC"].finditer(text):
        anchors["AC"].add(m.group(0))
    for m in INLINE_PATTERNS["ADR"].finditer(text):
        anchors["ADR"].add(m.group(1).split("-")[1])  # 只存编号 "004"
    for m in INLINE_PATTERNS["DESIGN"].finditer(text):
        anchors["DESIGN"].add(m.group(1))
    for m in INLINE_PATTERNS["OPEN"].finditer(text):
        anchors["OPEN"].add(m.group(1))

    # 3. 行内需求引用：先剥离 AC 片段，避免 AC-TOKEN-001 被截成 TOKEN-001
    stripped = AC_ID_RE.sub(lambda m: " " * len(m.group(0)), text)
    for m in REQ_ID_RE.finditer(stripped):
        ident = m.group(1)
        if ident in defs:
            anchors["REQ"].add(ident)

    return anchors


def scan_all() -> tuple[dict[str, dict[str, list[str]]], dict[str, str], list[str]]:
    """扫描全仓，返回 ({kind: {anchor_id: [file_rel, ...]}}, 定义集, 重复定义错误)。"""
    defs, duplicates = _collect_definitions()
    index: dict[str, dict[str, list[str]]] = {k: defaultdict(list) for k in ("REQ", "DESIGN", "AC", "ADR", "OPEN")}

    # 定义文件先行登记，保证矩阵「出现文件」首列就是定义源
    for ident, rel in sorted(defs.items()):
        index["REQ"][ident].append(rel)

    for md_file in _iter_markdown_files():
        rel = str(md_file.relative_to(ROOT))
        text = md_file.read_text(encoding="utf-8", errors="replace")
        anchors = _extract_anchors(text, rel, defs)
        for kind, ids in anchors.items():
            for aid in sorted(ids):
                if rel not in index[kind][aid]:
                    index[kind][aid].append(rel)

    # 也扫描 open-items.yaml 中的 OPEN-* 条目
    open_items_yaml = ROOT / "contracts" / "registries" / "open-items.yaml"
    if open_items_yaml.exists():
        try:
            import yaml
            data = yaml.safe_load(open_items_yaml.read_text()) or {}
            for item in data.get("items", []) or []:
                iid = item.get("id")
                if iid:
                    index["OPEN"][iid].append("contracts/registries/open-items.yaml")
        except Exception:
            pass

    return {k: dict(v) for k, v in index.items()}, defs, duplicates


def _build_ac_links(defs: dict[str, str]) -> dict[str, list[str]]:
    """建立 需求 → AC 的真实关联：按 AC 块作用域解析。

    规则：以 AC 标题行切块，块内（标题 + 正文）出现的已定义需求编号全部归到
    该 AC；支持 [REQ:x] 锚点、正文裸编号、范围 A-001 ~ A-003 与斜杠 A-041/043
    形式。跨块引用不扩散，避免把“引用了另一条 AC”误算成本条覆盖。
    """
    links: dict[str, set[str]] = defaultdict(set)
    if not AC_FILE.exists():
        return {}

    blocks: list[tuple[str, list[str]]] = []
    cur_ac: str | None = None
    cur_lines: list[str] = []
    for line in AC_FILE.read_text(encoding="utf-8", errors="replace").splitlines():
        if re.match("^### +AC-", line):
            if cur_ac is not None:
                blocks.append((cur_ac, cur_lines))
            found = AC_ID_RE.search(line)
            cur_ac = found.group(0) if found else None
            cur_lines = [line]
            continue
        if cur_ac is None:
            continue
        if line.startswith("## ") or line.startswith("# "):
            blocks.append((cur_ac, cur_lines))
            cur_ac = None
            cur_lines = []
            continue
        cur_lines.append(line)
    if cur_ac is not None:
        blocks.append((cur_ac, cur_lines))

    for ac, block_lines in blocks:
        for line in block_lines:
            masked = AC_ID_RE.sub(lambda m: " " * len(m.group(0)), line)
            forms, masked = _expand_line_forms(masked, expand_ranges=False)
            singles = [m.group(1) for m in REQ_ID_RE.finditer(masked)]
            for ident in set(forms) | set(singles):
                if ident in defs:
                    links[ident].add(ac)
    # 显式映射：AC 文件「附录 A：需求覆盖索引」表格
    text = AC_FILE.read_text(encoding="utf-8", errors="replace")
    marker = "## 附录 A：需求覆盖索引"
    pos = text.find(marker)
    if pos != -1:
        tail = text[pos:]
        nxt = tail.find("\n## ", len(marker))
        if nxt != -1:
            tail = tail[:nxt]
        for line in tail.splitlines():
            if not line.startswith("| "):
                continue
            cells = [c.strip() for c in line.strip().strip("|").split("|")]
            if len(cells) < 2:
                continue
            req_cell = cells[0].strip().strip(chr(96))
            if not REQ_ID_RE.fullmatch(req_cell):
                continue
            acs = AC_ID_RE.findall(cells[1])
            if acs:
                links[req_cell].update(acs)
    return {k: sorted(v) for k, v in links.items()}


def _adr_range() -> str:
    """从 docs/adr/ADR-\\d{3}-*.md 文件名推导编号区间。

    只认 `ADR-<三位序号>-<kebab>.md`，README/模板自然排除；按整数排序。
    目录缺失或没有匹配文件时抛 AdrRangeError，不退回写死区间。
    编号连续时渲染 `ADR-001 ~ ADR-035`；有缺口时列实际编号，不伪装连续。
    """
    adr_rel = ADR_DIR.relative_to(ROOT) if ADR_DIR.is_relative_to(ROOT) else ADR_DIR
    if not ADR_DIR.is_dir():
        raise AdrRangeError(f"目录不存在：{adr_rel}")
    ids = sorted({
        int(m.group(1))
        for p in ADR_DIR.iterdir()
        if p.is_file() and (m := ADR_FILE_RE.match(p.name))
    })
    if not ids:
        raise AdrRangeError(f"{adr_rel} 下没有 ADR-\\d{{3}}-*.md 文件")
    if ids == list(range(ids[0], ids[-1] + 1)):
        return f"ADR-{ids[0]:03d} ~ ADR-{ids[-1]:03d}"
    return "、".join(f"ADR-{i:03d}" for i in ids)


def _declared_adr_range(matrix_text: str) -> str | None:
    """读取生成物「锚点统计」表中 ADR 行声明的区间（去反引号）。

    仅认第二列为 `**数字**` 的统计行，避免命中「锚点格式约定」表的 ADR 示例行。
    """
    for line in matrix_text.splitlines():
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        if len(cells) >= 3 and cells[0] == "ADR" and ADR_COUNT_CELL_RE.match(cells[1]):
            return cells[2].strip("`") or None
    return None


def render(index: dict, defs: dict[str, str]) -> str:
    """把扫描到的锚点索引渲染为 Markdown 追溯矩阵。"""
    lines: list[str] = []
    # 刻意不写生成日期：零差异门禁必须与时钟无关。日期每天都变，会让
    # check-generated-zero-diff 在跨日后无端变红（同 gen_registry_md.py 的取舍）。
    lines.append("# GuliangIAM 需求追溯矩阵")
    lines.append("")
    lines.append("**生成方式**：`python3 scripts/gen_traceability.py`（自动扫描全仓文档锚点）")
    lines.append("")
    lines.append("## 锚点格式约定")
    lines.append("")
    lines.append("| 类型 | 格式 | 示例 |")
    lines.append("|------|------|------|")
    lines.append("| 需求（定义） | `docs/requirements/**` 表格行（首列为需求编号） | `AUTH-001` 行 |")
    lines.append("| 需求（引用） | `[REQ:FAM-XXX]` / AC 标题括号 / `SRS FAM-XXX` | `[REQ:AUTH-001]` |")
    lines.append("| 设计 | `[DESIGN:path]` | `[DESIGN:auth/oauth2]` |")
    lines.append("| 验收条件 | `[AC:MODULE-XXX]` | `[AC:AUTH-001]` |")
    lines.append("| ADR | `[ADR:NNN]` | `[ADR:004]` |")
    lines.append("| 开放项 | `[OPEN:xxx-XXX]` | `[OPEN:OPEN-API-05]` |")
    lines.append("")
    lines.append("> 逐号定义只来自 `docs/requirements/**/*.md` 的表格行；章节 `[REQ:… ~ …]` 仅声明编号空间。")
    lines.append("")

    # 统计概览
    lines.append("## 锚点统计")
    lines.append("")
    lines.append("| 类型 | 数量 | 锚点前缀 |")
    lines.append("|------|------|---------|")
    kind_labels = {
        "REQ": "需求（已定义）",
        "DESIGN": "设计",
        "AC": "验收条件",
        "ADR": "ADR",
        "OPEN": "开放项",
    }
    kind_prefix = {
        "REQ": "AUTH/ORG/USER/APP/PROV/PERM/COMM/INFRA/SEC/FE/REBAC/IGA/RISK/B2B/WLI/EXT/METER/PRIV/DR/DATA/DB-REQ",
        "DESIGN": "path 或 section 编号 (如 auth/oauth2, 14.6.3)",
        "AC": "AC-AUTH/AC-TOKEN/AC-PROTO/AC-JWKS/AC-SESSION/AC-...",
        "ADR": _adr_range(),
        "OPEN": "OPEN-API/OPEN-DESIGN/OPEN-...",
    }
    for kind in ("REQ", "DESIGN", "AC", "ADR", "OPEN"):
        count = len(index.get(kind, {}))
        lines.append(f"| {kind_labels.get(kind, kind)} | **{count}** | `{kind_prefix.get(kind, '')}` |")
    lines.append("")

    # 按类型列出完整锚点清单（方便核对）
    for kind in ("REQ", "DESIGN", "AC", "ADR", "OPEN"):
        items = sorted(index.get(kind, {}).keys())
        label = kind_labels.get(kind, kind)
        if not items:
            continue
        lines.append(f"## {label}清单（{len(items)} 项）")
        lines.append("")
        lines.append("| 锚点 | 出现文件 |")
        lines.append("|------|---------|")
        for aid in items:
            files = index[kind][aid]
            file_list = ", ".join(f"`{f}`" for f in files[:5])
            if len(files) > 5:
                file_list += f" ... (+{len(files) - 5})"
            lines.append(f"| `{aid}` | {file_list} |")
        lines.append("")

    # 交叉索引：需求 → AC（同一行共现）
    lines.append("## 需求 ↔ 验收条件 交叉索引")
    lines.append("")
    lines.append("| 需求编号 | 关联 AC | 需求定义文件 |")
    lines.append("|---------|---------|-------------|")
    ac_links = _build_ac_links(defs)
    req_items = sorted(index.get("REQ", {}).keys())
    for req_id in req_items:
        acs = ac_links.get(req_id, [])
        ac_text = "、".join(f"`{a}`" for a in acs) if acs else "—"
        def_file = index["REQ"][req_id][0] if index["REQ"].get(req_id) else defs.get(req_id, "")
        lines.append(f"| `{req_id}` | {ac_text} | `{def_file}` |")
    lines.append("")

    # 自生成标记
    lines.append("---")
    lines.append("")
    lines.append(f"> **本文件由 `scripts/gen_traceability.py` 自动生成，禁止手改。**")
    lines.append(f"> 重新生成：`python3 scripts/gen_traceability.py`")
    lines.append(f"> 零差异门禁：`python3 scripts/gen_traceability.py --check`")
    lines.append("")

    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser(description="扫描全仓文档锚点生成追溯矩阵")
    parser.add_argument("--check", action="store_true", help="零差异门禁 + 未定义需求引用校验")
    args = parser.parse_args()

    print("=== gen_traceability.py ===")
    print("扫描范围：docs/requirements, docs/design, docs, docs/adr, contracts/registries")
    print()

    index, defs, duplicates = scan_all()
    if duplicates:
        for problem in duplicates:
            print(f"  DUPLICATE: {problem}")
        print("FAIL: 需求编号存在重复定义（逐号定义必须唯一）")
        return 1

    try:
        content = render(index, defs)
    except AdrRangeError as exc:
        print(f"  ADR: 无法推导编号区间 — {exc}")
        print("FAIL: docs/adr 无法推导 ADR 区间（不写死；检查目录与 ADR-\\d{3}-*.md 命名）")
        return 1

    if args.check:
        design_problems = _validate_design_anchors()
        if design_problems:
            for problem in design_problems:
                print(f"  {problem}")
            print(f"FAIL: {len(design_problems)} 处设计锚点问题（每份设计文档必须恰好一个 [DESIGN:path]）")
            return 1

        problems = _collect_strict_problems(defs)
        if problems:
            for problem in problems:
                print(f"  {problem}")
            print(f"FAIL: {len(problems)} 处需求引用未命中逐号定义（先在 docs/requirements 表格行登记）")
            return 1

        if not TRACEABILITY_OUTPUT.exists():
            print(f"  NEW  docs/traceability-matrix.md")
            print("FAIL: 需要运行 python3 scripts/gen_traceability.py")
            return 1

        existing = TRACEABILITY_OUTPUT.read_text()

        declared_range = _declared_adr_range(existing)
        expected_range = _adr_range()
        if declared_range != expected_range:
            print(f"  ADR 区间不一致：矩阵声明 `{declared_range}`，docs/adr 实际 `{expected_range}`")
            print("FAIL: traceability-matrix.md 的 ADR 区间与 docs/adr 文件集合不一致 — 运行 python3 scripts/gen_traceability.py")
            return 1

        if existing.strip() != content.strip():
            import difflib
            print(f"  DIFF docs/traceability-matrix.md")
            diff = list(difflib.unified_diff(
                existing.splitlines(), content.splitlines(),
                fromfile="existing", tofile="generated",
                lineterm=""
            ))
            for line in diff[:15]:
                print(f"    {line}")
            if len(diff) > 15:
                print(f"    ... (+{len(diff) - 15} more lines)")
            print("FAIL: traceability-matrix.md 与扫描结果不一致 — 运行 python3 scripts/gen_traceability.py")
            return 1

        for kind, count in sorted({k: len(v) for k, v in index.items()}.items()):
            print(f"  SCAN  {kind}: {count} 个锚点")
        print(f"  OK   docs/traceability-matrix.md（未定义引用 0）")
        print("PASS: traceability-matrix.md 零差异")
        return 0

    TRACEABILITY_OUTPUT.write_text(content)
    for kind, count in sorted({k: len(v) for k, v in index.items()}.items()):
        print(f"  SCAN  {kind}: {count} 个锚点")
    print(f"GEN  {TRACEABILITY_OUTPUT.relative_to(ROOT)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
