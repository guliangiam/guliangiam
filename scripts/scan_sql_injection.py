#!/usr/bin/env python3
"""P0 scanner — dynamic SQL construction point collector.

扫 Go 代码里绕过 pgx 参数化的动态 SQL 构造模式，输出 JSON 供人工分类 + 检测子模块 E-1~E-5 用。
规则见 P0 方案文档 — 只抓真正的动态（非 compile-time 常量）构造。

扫描 4 类模式：
  whitelist_token   strings.Replace(const, "{TOKEN}", dynamic, 1) + dynamic 必须经 whitelist
  internal_const    fmt.Sprintf/SB.WriteString 拼 SQL + 参数来源可确定是内部常量
  concat_risk       `+` 运算符拼 SQL 片段（过滤掉编译期常量 + 内部常量）
  variable_arg      Exec/Query 首参是变量名（不是 const queryXxx）

P1 检测子模块会：
  1. 跑这个扫描器拿到所有点
  2. 和 contracts/security/sql-injection-registry.json 对比
  3. 不一致 → FATAL（有人加了新动态 SQL 没登记）
"""
import os, re, json, sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
INTERNAL = os.path.join(ROOT, "internal")

hits = {k: [] for k in ("whitelist_token", "internal_const", "concat_risk", "variable_arg")}

SQL_KW = r"(SELECT|INSERT|UPDATE|DELETE|FROM|WHERE|ORDER\s+BY|GROUP\s+BY|SET)"

def rel(p):
    return os.path.relpath(p, ROOT)

def walk_go_files():
    for dirpath, _, files in os.walk(INTERNAL):
        if any(s in dirpath for s in ["/vendor/", "/node_modules/"]):
            continue
        for fn in files:
            if fn.endswith(".go") and not fn.endswith("_test.go"):
                yield os.path.join(dirpath, fn)

for path in walk_go_files():
    with open(path) as f:
        lines = f.read().splitlines()
    for i, stripped in enumerate(lines, 1):
        if stripped.lstrip().startswith("//"):
            continue
        spf = re.search(r"fmt\.Sprintf\s*\(\s*([`\"])([^\1]*)(?:SELECT|INSERT|UPDATE|DELETE|FROM|WHERE)[^\1]*\1", stripped, re.IGNORECASE)
        if spf and re.search(r"%[sqv]", spf.group(2)):
            hits["internal_const"].append({"file": rel(path), "line": i, "snippet": stripped[:200]})
            continue
        if "strings.Replace(" in stripped and re.search(r"\{[A-Z_]+\}", stripped):
            hits["whitelist_token"].append({"file": rel(path), "line": i, "snippet": stripped[:200]})
            continue
        if ".WriteString(" in stripped and "fmt.Sprintf" in stripped and re.search(SQL_KW, stripped, re.IGNORECASE):
            hits["internal_const"].append({"file": rel(path), "line": i, "snippet": stripped[:200]})
            continue

QUERY_FIRST_ARG = re.compile(r"\.(Exec|Query|QueryRow)\s*\(\s*ctx\s*,\s*(\w+)")
SQL_CONST_PREFIXES = ("query", "Query", "sql", "SQL", "q", "countSQL", "selectSQL", "baseQuery", "Receipt", "Refund", "Attempt", "Recon")

for path in walk_go_files():
    with open(path) as f:
        lines = f.read().splitlines()
    for i, stripped in enumerate(lines, 1):
        if stripped.lstrip().startswith("//"):
            continue
        if any(x in stripped for x in ["set_tenant_context", "set_config(", "guliangiam.set_"]):
            continue
        if "Batch" in stripped:
            continue
        m = QUERY_FIRST_ARG.search(stripped)
        if m:
            arg = m.group(2)
            if not arg.startswith(SQL_CONST_PREFIXES):
                hits["variable_arg"].append({"file": rel(path), "line": i, "arg": arg, "snippet": stripped[:200]})

CONCAT_SQL = re.compile(
    r"(`[^`]*(?:WHERE|ORDER|FROM|JOIN|SELECT|UPDATE|DELETE)[^`]*`\s*\+\s*\w+)"
    r"|(\w+\s*\+\s*`[^`]*(?:WHERE|ORDER|FROM|JOIN)[^`]*`)",
    re.IGNORECASE
)
CONCAT_SQL2 = re.compile(
    r"(?:\"[^\"]*(?:WHERE|ORDER|FROM|JOIN|SELECT|UPDATE|DELETE)[^\"]*\"\s*\+\s*\w+)"
    r"|(\w+\s*\+\s*\"[^\"]*(?:WHERE|ORDER|FROM|JOIN)[^\"]*\")",
    re.IGNORECASE
)
for path in walk_go_files():
    with open(path) as f:
        lines = f.read().splitlines()
    for i, stripped in enumerate(lines, 1):
        if stripped.lstrip().startswith("//"):
            continue
        if CONCAT_SQL.search(stripped) or CONCAT_SQL2.search(stripped):
            if not any(noise in stripped for noise in ["smtp_sender", "cannot transition", "in current state", "HTTP URL", "apiURL"]):
                hits["concat_risk"].append({"file": rel(path), "line": i, "snippet": stripped[:200]})

registry_path = os.path.join(ROOT, "contracts/security/sql-injection-registry.json")
if os.path.exists(registry_path):
    with open(registry_path) as f:
        reg = json.load(f)
    reg_files = {e["file"] for e in reg.get("entries", [])}
    scan_files = {h["file"] for cat in hits.values() for h in cat}
    new_points = scan_files - reg_files
    missing_from_reg = reg_files - scan_files
    print(f"\n=== vs registry ===")
    print(f"  new scan points not in registry: {len(new_points)} → FATAL")
    for f in sorted(new_points):
        print(f"    + {f}")
    print(f"  registry entries with no scan hit: {len(missing_from_reg)}")
    for f in sorted(missing_from_reg):
        print(f"    - {f}")

print("\n=== SCAN SUMMARY ===")
for cat, items in hits.items():
    print(f"  {cat}: {len(items)}")

json.dump(hits, sys.stdout, indent=2, ensure_ascii=False)
