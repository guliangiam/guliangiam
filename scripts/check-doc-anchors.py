#!/usr/bin/env python3
"""文档锚点门禁：代码与文档中形如「《设计报告》14.6.1」的章节引用必须真实存在。

背景（设计报告 13.1 风险 7/8）：章节重编号后引用会静默失准，修订者会按错误锚点
找到无关内容，或误判「规范未定义」而自行发明规则。本脚本把该风险变成可执行判定。

判定范围：正式文档的章节号（`## N.` / `### N.M` / `#### N.M.K`）与附录号
（`## 附录 X` / `### X.N`）。引用文档别名后跟的编号必须落在对应文档的编号集合内。

不改动任何文件；发现悬空引用时以非零码退出。

输入：docs/ 下所有 .md、scripts/ 源码中的文档引用
输出：PASS/FAIL + 悬空引用列表
--check 行为：无 --check；失败即 exit 1
消费方：verify-docs.sh(4/4)、CI
"""

from __future__ import annotations

import os
import pathlib
import re
import sys

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# 别名 → 默认文档相对路径。正则交替按**插入顺序**优先，因此同一文档的长别名必须写在
# 短别名之前，否则「设计报告」会抢先匹配「项目开发设计报告」的尾串。
DOCS: dict[str, str] = {
    "项目开发设计报告": "docs/design/README.md",
    "开发设计报告": "docs/design/README.md",
    "设计报告": "docs/design/README.md",
    "开发规范手册": "docs/GuliangIAM-开发规范.md",
    "规范手册": "docs/GuliangIAM-开发规范.md",
    "手册": "docs/GuliangIAM-开发规范.md",
    "项目需求分析报告": "docs/requirements/functional.md",
    "需求分析报告": "docs/requirements/functional.md",
    "需求分析": "docs/requirements/functional.md",
    "SRS": "docs/requirements/functional.md",
    "架构设计": "docs/architecture/README.md",
    "控制台设计": "docs/design/frontend/README.md",
    "验收条件": "docs/acceptance-criteria.md",
    "需求追溯矩阵": "docs/traceability-matrix.md",
    "追溯矩阵": "docs/traceability-matrix.md",
    "部署与运维指南": "docs/operations/README.md",
    "部署指南": "docs/operations/README.md",
    "性能测量基线": "docs/architecture/observability.md",
    "性能基线": "docs/architecture/observability.md",
    "Secret 轮换策略文档": "docs/architecture/security-model.md",
    "Secret 轮换策略": "docs/architecture/security-model.md",
    "Secret 轮换": "docs/architecture/security-model.md",
    "文档体系": "docs/README.md",
    "README.md": "docs/README.md",
    "错误码注册表": "docs/generated/errors-registry.md",
    "环境变量注册表": "docs/generated/env-registry.md",
}

# 旧文档拆分后，原章节号分布到多个子文档。此字段为每个别名提供按章节号前缀路由到
# 具体子文档的能力。key 是别名，value 是 (章节号前缀, 子文档路径) 列表，匹配顺序：
# 前缀越长越优先。未命中任何前缀时回退到 DOCS 中的默认路径。
DOCS_ROUTES: dict[str, list[tuple[str, str]]] = {
    # 旧设计报告拆到 design/protocol/, design/auth/, design/commerce/ 等子目录
    "项目开发设计报告": [
        ("14.", "docs/design/protocol/oauth2.md"),
    ],
    "开发设计报告": [
        ("14.", "docs/design/protocol/oauth2.md"),
    ],
    "设计报告": [
        ("14.", "docs/design/protocol/oauth2.md"),
    ],
    # 架构设计 6.6 已迁移到 DOCS_TRANSFORMS（旧前缀变换 + 路径覆盖），
    # 此处 ROUTES 不再保留 TRANSFORMS 已覆盖的条目，避免双重路由。
    # 旧部署指南：Kubernetes 部署章节拆到 deployment.md（deployment.md 仅保留 1-4 四个顶级
    # 章节号，无法做旧→新章节号一一变换，故继续用 ROUTES 只改路径）
    "部署与运维指南": [
        ("3.1", "docs/operations/deployment.md"),
        ("3.2", "docs/operations/deployment.md"),
        ("3.3", "docs/operations/deployment.md"),
        ("3.4", "docs/operations/deployment.md"),
        ("3.5", "docs/operations/deployment.md"),
        ("3.6", "docs/operations/deployment.md"),
        ("3.7", "docs/operations/deployment.md"),
        ("3.8", "docs/operations/deployment.md"),
    ],
}

# 旧文档拆分后，目标子文档的章节号体系也变了。此字段提供"引用章节号 → 目标文档
# 实际章节号"的变换规则。key 是别名，value 是 (旧前缀, 新前缀, 目标文档路径) 列表。
# 匹配后：
#   1. 引用章节号 ref 如果以 旧前缀 开头，去掉旧前缀，加上 新前缀 得到新 ref
#   2. 使用 目标文档路径（覆盖 DOCS_ROUTES 和 DOCS 的默认值）
# 变换也按旧前缀长度降序匹配。
DOCS_TRANSFORMS: dict[str, list[tuple[str, str, str]]] = {
    # SRS 5.X.Y → functional.md 的 X.Y 系列（5.1.2 → 1.2, 5.9 → 9, 5.11.1 → 11.1）
    "SRS": [
        ("5.", "", "docs/requirements/functional.md"),
    ],
    "需求分析报告": [
        ("5.", "", "docs/requirements/functional.md"),
    ],
    "项目需求分析报告": [
        ("5.", "", "docs/requirements/functional.md"),
    ],
    # 架构设计 → 各 architecture 子文档。旧章节号 → 新章节号有完整的一一映射：
    #   6.6 安全红线 → security-model.md 的 1
    #   8.x 安全架构 → security-model.md 的 2.x 通配（8.1 → 2.1, 8.8 → 2.8, 8.19 → 2.19）
    #   6.14 风险引擎 → security-model.md 的 3（ADR-027）
    #   6.13 身份 Actions 沙箱 → system-overview.md 的 3（ADR-029 所在行）
    #   6.15 Workload 身份平面 → security-model.md 的 4（ADR-026）
    #   7.8 / 7.8.2 持久化架构 → data-model.md 的 3 / 3.2
    #   12.x 多区域 → multi-region.md 的 x（12.4 → 4, 12.5 → 5）
    #   12 第 12 章 → multi-region.md 的 1（区域桶模型）
    "架构设计": [
        ("8.", "2.", "docs/architecture/security-model.md"),
        ("6.6", "1", "docs/architecture/security-model.md"),
        ("6.15", "4", "docs/architecture/security-model.md"),
        ("6.14", "3", "docs/architecture/security-model.md"),
        ("6.13", "3", "docs/architecture/system-overview.md"),
        ("7.8.", "3.", "docs/architecture/data-model.md"),
        ("7.8", "3", "docs/architecture/data-model.md"),
        ("12.", "", "docs/architecture/multi-region.md"),
        ("12", "1", "docs/architecture/multi-region.md"),
        ("6.4.2", "", "docs/architecture/layers.md"),
        ("6.4.1", "", "docs/architecture/layers.md"),
        ("6.2.1", "", "docs/architecture/system-overview.md"),
        ("6.1", "3", "docs/architecture/system-overview.md"),
    ],
}


# 旧设计报告的章节编号在代码注释中仍以「设计报告 N.M」出现；design/README.md 的旧编号索引
# 已下线（docs/README.md §1 禁止不可解析引用），此处固化为机器表。值为 None 表示目标文档使用
# 命名章节（无编号），门禁只校验目标文档存在，不再做数字锚点校验。
LEGACY_REFS: dict[tuple[str, str], tuple[str, str | None]] = {
    ("设计报告", "13.1"): ("docs/architecture/layers.md", None),
    ("设计报告", "15.3"): ("docs/design/auth/saml.md", None),
    ("设计报告", "28.1"): ("docs/design/frontend/README.md", None),
    ("设计报告", "28.2"): ("docs/design/iam/applications.md", None),
    ("设计报告", "28.10.5"): ("docs/design/auth/saml.md", None),
    ("设计报告", "42.2"): ("docs/design/iam/applications.md", None),
    ("设计报告", "42.3.3"): ("docs/design/commerce/order.md", None),
    ("设计报告", "42.3.7"): ("docs/design/commerce/payment.md", None),
    ("设计报告", "42.3.11"): ("docs/design/commerce/order.md", None),
    ("设计报告", "42.3.13"): ("docs/design/commerce/payment.md", None),
}


HEADING_RE = re.compile(r"^#{2,6}\s+(.*)$")
# 「3.1」「4.1.3」「D.4」「I.1」——附录小节与普通小节共用一套编号形态
NUMBER_HEADING_RE = re.compile(r"^([0-9]+(?:\.[0-9]+)*|[A-Z](?:\.\d+)+)[\s.、]")
APPENDIX_HEADING_RE = re.compile(r"^([A-Z])(?:\.([0-9]+))?")
# 同文件重复章节号会制造两个可解析但语义不同的锚点；别名解析无法判断该跳到哪一节。
DUPLICATE_NUMBER_RE = re.compile(r"^#{2,6}\s+([0-9]+(?:\.[0-9]+)*|[A-Z](?:\.\d+)+)[\s.、]")

ALIAS_RE = re.compile(
    r"(?:《(" + "|".join(re.escape(a) for a in DOCS) + r")》|"
    r"(" + "|".join(re.escape(a) for a in DOCS) + r")"
    r")(?:\s*§\s*(\d+(?:\.\d+)*)|\s*(附录\s*[A-Z](?:\.[0-9]+)?|\d+(?:\.\d+)+|[A-Z]\.\d+))"
)


def resolve_ref(alias: str, ref: str) -> tuple[str, str | None] | None:
    """根据别名和引用章节号，解析出 (目标文档路径, 变换后的章节号)。

    解析优先级：
    1. LEGACY_REFS 中固化的旧设计报告编号（章节号可能为 None）
    2. DOCS_TRANSFORMS 中的变换规则（按旧前缀长度降序）
    3. DOCS_ROUTES 中的路由规则（按章节号前缀长度降序）
    4. DOCS 中的默认路径

    返回 None 时表示路径逃逸仓库边界（防御性兜底）。
    """
    legacy = LEGACY_REFS.get((alias, ref))
    if legacy is not None:
        resolved_path, new_ref = legacy
    else:
        resolved_path = ""
        new_ref = ref

        # 1. 先尝试变换规则
        transforms = DOCS_TRANSFORMS.get(alias, [])
        sorted_transforms = sorted(transforms, key=lambda x: -len(x[0]))
        for old_prefix, np, subpath in sorted_transforms:
            if ref.startswith(old_prefix):
                tail = ref[len(old_prefix):]  # 去掉旧前缀
                new_ref = np + tail if np else tail
                resolved_path = subpath
                break

        # 2. 再尝试路由规则（只改变路径，不改变章节号）
        if not resolved_path:
            routes = DOCS_ROUTES.get(alias, [])
            sorted_routes = sorted(routes, key=lambda x: -len(x[0]))
            for prefix, subpath in sorted_routes:
                if ref.startswith(prefix):
                    resolved_path = subpath
                    break

        # 3. 回退到默认路径
        if not resolved_path:
            resolved_path = DOCS[alias]

    # 防御性边界校验：硬编码字典不应触发，但保留以防配置漂移
    try:
        (pathlib.Path(REPO) / resolved_path).resolve().relative_to(pathlib.Path(REPO).resolve())
    except ValueError:
        return None

    return resolved_path, new_ref


def heading_numbers(path: str) -> set[str]:
    """收集文档中所有可用于引用的编号（章节号与附录号）。"""
    full = os.path.join(REPO, path)
    if not os.path.exists(full):
        # 文档被删除/改名但别名表未同步时，此前会抛 FileNotFoundError——一个没有
        # 任何指向的堆栈，读者无法知道「哪个引用指向了一份不存在的文档」。返回空集
        # 会让每处引用都判定为悬空，报错因此直接指出问题文档与引用位置。
        return set()
    found: set[str] = set()
    with open(full, encoding="utf-8") as fh:
        for line in fh:
            m = HEADING_RE.match(line)
            if not m:
                continue
            title = m.group(1).strip()
            if title.startswith("附录"):
                # 「附录 C. 错误码全量清单」→ C、C.3
                sub = APPENDIX_HEADING_RE.match(title[2:].strip())
                if sub:
                    found.add(sub.group(1))
                    if sub.group(2):
                        found.add(f"{sub.group(1)}.{sub.group(2)}")
                continue
            sub = NUMBER_HEADING_RE.match(title)
            if sub:
                found.add(sub.group(1))
    return found


def duplicate_numbered_headings() -> list[str]:
    """检测 docs/ Markdown 内重复的章节编号（如两个 §2.4）。"""
    problems: list[str] = []
    for dirpath, dirnames, filenames in os.walk(os.path.join(REPO, "docs")):
        dirnames[:] = [d for d in dirnames if d != "generated"]
        for name in filenames:
            if not name.endswith(".md"):
                continue
            abspath = os.path.join(dirpath, name)
            rel = os.path.relpath(abspath, REPO)
            seen: dict[str, int] = {}
            try:
                lines = open(abspath, encoding="utf-8").read().split("\n")
            except UnicodeDecodeError:
                continue
            for lineno, line in enumerate(lines, 1):
                m = DUPLICATE_NUMBER_RE.match(line)
                if not m:
                    continue
                number = m.group(1)
                if number in seen:
                    problems.append(
                        f"{rel}:{lineno}: 重复章节号「{number}」"
                        f"（首次出现在 :{seen[number]}）"
                    )
                else:
                    seen[number] = lineno
    return problems


def scan_files() -> list[str]:
    paths: list[str] = []
    for root in ("docs", "internal", "scripts", "db", "web/src", "openapi"):
        for dirpath, dirnames, filenames in os.walk(os.path.join(REPO, root)):
            dirnames[:] = [d for d in dirnames if d != "node_modules"]
            for name in filenames:
                if name.endswith(
                    (".md", ".go", ".sh", ".py", ".sql", ".ts", ".tsx", ".yaml", ".yml")
                ):
                    paths.append(os.path.join(dirpath, name))
    return sorted(set(paths))


def main() -> int:
    cache: dict[str, set[str]] = {}
    problems: list[str] = duplicate_numbered_headings()
    duplicate_count = len(problems)
    missing: dict[str, list[str]] = {}
    total = 0

    for abspath in scan_files():
        rel = os.path.relpath(abspath, REPO)
        try:
            text = open(abspath, encoding="utf-8").read()
        except UnicodeDecodeError:
            continue
        if "》" not in text and "手册" not in text and "设计报告" not in text:
            continue
        for lineno, line in enumerate(text.split("\n"), 1):
            for m in ALIAS_RE.finditer(line):
                alias = m.group(1) or m.group(2)
                ref = (m.group(3) or m.group(4)).replace(" ", "")
                resolved = resolve_ref(alias, ref)
                if resolved is None:
                    problems.append(
                        f"{rel}:{lineno}: 路径逃逸「{alias} {ref}」（resolve_ref 返回 None）"
                    )
                    continue
                doc, resolved_ref = resolved
                if doc not in cache:
                    cache[doc] = heading_numbers(doc)
                total += 1
                if not os.path.exists(os.path.join(REPO, doc)):
                    missing.setdefault(doc, []).append(f"{rel}:{lineno}: 「{alias} {ref}」")
                    continue
                if resolved_ref is None:
                    # 旧编号映射到使用命名章节的现行文档：只校验文档存在
                    continue
                check_ref = resolved_ref
                if check_ref.startswith("附录"):
                    check_ref = check_ref[2:]
                if check_ref not in cache[doc]:
                    if ref != resolved_ref:
                        problems.append(
                            f"{rel}:{lineno}: 悬空锚点 「{alias} {ref}」(变换→{resolved_ref})（{doc}）"
                        )
                    else:
                        problems.append(f"{rel}:{lineno}: 悬空锚点 「{alias} {ref}」（{doc}）")

    if missing:
        print("文档锚点门禁失败：以下别名指向的正式文档不存在（文档被删除/改名，别名表未同步）：")
        for doc in sorted(missing):
            print(f"  {doc} —— 被 {len(missing[doc])} 处引用，例如 {missing[doc][0]}")
        print()

    if problems:
        print("文档锚点门禁失败：")
        for p in problems:
            print("  " + p)
        print()

    if problems or missing:
        print(
            f"共 {len(problems)} 处问题（其中重复章节号 {duplicate_count} 处）"
            f" + {sum(len(v) for v in missing.values())} 处"
            f"指向不存在文档的引用（扫描 {total} 处引用）。"
        )
        return 1

    print(f"文档锚点门禁通过：{total} 处章节引用全部可解析，且无重复章节号。")
    return 0


if __name__ == "__main__":
    sys.exit(main())
