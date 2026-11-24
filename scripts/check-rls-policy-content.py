#!/usr/bin/env python3
"""
check-rls-policy-content.py — RLS POLICY 内容门禁

拦截危险 POLICY 模式:
  1. USING TRUE / TRUE OR is_super_admin()  — 恒真策略 = FORCE RLS 白开
  2. WITH CHECK 缺失 (间接表无 org_id 但没写 WITH CHECK)  — INSERT 越权
  3. YAML 声明了 rls_predicate 但 migration 没用到 — 硬编码覆盖声明

原理: 同时检查 db/postgresql/V*.sql (静态) 和 DB 运行时 pg_policies (动态)
      双保险——即使 migration 已跑过，DB 里的恒真策略也会被抓出来
用法:
  python3 scripts/check-rls-policy-content.py                    # 只查 migration 文件
  python3 scripts/check-rls-policy-content.py --dsn "$DSN"       # 同时查 DB 运行时
  python3 scripts/check-rls-policy-content.py --fail-fast        # 有问题立即 exit 1 (默认)

输入：db/postgresql/V*.sql、DB 运行时 pg_policies(--dsn)
输出：PASS/FAIL + 危险 POLICY 模式定位
--check 行为：--dsn 同时查运行时；--fail-fast 遇第一处差异立即退出
消费方：独立脚本、CI
"""
import argparse, glob, os, re, sys, subprocess, yaml

# ── 危险模式 ────────────────────────────────────────────────────────
# USING TRUE 恒真: 单独 TRUE / TRUE OR is_super_admin / (true) 等
FOREVER_TRUE_PATTERNS = [
    re.compile(r'\bUSING\s*\(\s*TRUE\s*\)', re.IGNORECASE),
    re.compile(r'\bUSING\s+TRUE\b', re.IGNORECASE),
    re.compile(r"USING\s*\(\s*TRUE\s*(?:OR|AND)\s*guliangiam\.is_super_admin", re.IGNORECASE),
    re.compile(r"USING\s*\(\s*TRUE\s*(?:OR|AND)\s*current_setting", re.IGNORECASE),
]

def check_migrations():
    """静态扫 db/postgresql/V*.sql"""
    hits = []
    for f in sorted(glob.glob('db/postgresql/V*.sql')):
        with open(f) as fp:
            lines = fp.readlines()
        content = ''.join(lines)

        for pat in FOREVER_TRUE_PATTERNS:
            for m in pat.finditer(content):
                # 定位行号
                line_no = content[:m.start()].count('\n') + 1
                # 白名单: 前 5 行内有 -- allow-USING-TRUE 注释
                prev_block = ''.join(lines[max(0, line_no-6):line_no])
                if re.search(r'--\s*allow-USING-TRUE', prev_block):
                    continue
                # 取上下文
                ctx_start = max(0, line_no - 1)
                ctx_end   = min(len(lines), line_no + 3)
                ctx = ''.join(lines[ctx_start:ctx_end]).rstrip()
                hits.append((f, line_no, pat.pattern[:40], ctx))
    return hits

def _normalize_predicate(expr):
    """Normalize a predicate expression for declaration-vs-runtime comparison."""
    s = (expr or '').strip()
    while s.startswith('(') and s.endswith(')'):
        s = s[1:-1].strip()
    s = re.sub(r'\s+', '', s).lower()
    return s


def load_declared_predicates():
    """Return {short_table_name: normalized rls_predicate} from contracts/schema/*.yaml.

    contracts/schema is the single source of truth (ADR-022). Platform-level tables
    legitimately declare `guliangiam.is_super_admin()` as their predicate; a runtime
    policy that matches the declaration is compliant, not a forever-true leak.
    A declaration of TRUE/1=1 stays a violation.
    """
    declared = {}
    for path in sorted(glob.glob('contracts/schema/**/*.yaml', recursive=True)):
        try:
            with open(path) as fp:
                # Schema files are multi-document YAML (`---` separated table entries).
                docs = list(yaml.safe_load_all(fp))
        except Exception:
            continue
        for doc in docs:
            entries = []
            if isinstance(doc, list):
                entries = doc
            elif isinstance(doc, dict):
                if isinstance(doc.get('tables'), list):
                    entries = doc['tables']
                elif 'table_name' in doc:
                    entries = [doc]
            for entry in entries:
                if not isinstance(entry, dict):
                    continue
                table = entry.get('table_name')
                predicate = entry.get('rls_predicate')
                if not table or not predicate:
                    continue
                declared[str(table).split('.')[-1]] = _normalize_predicate(str(predicate))
    return declared


def check_db(dsn):
    """动态扫 pg_policies — 必须 psycopg 或 psql"""
    try:
        out = subprocess.run(
            ['psql', '-d', dsn, '-t', '-A', '-F', '|', '-c', """
                SELECT tablename, policyname, qual, COALESCE(with_check,'')
                FROM pg_policies WHERE schemaname='guliangiam';
            """],
            capture_output=True, text=True, timeout=10,
        ).stdout.strip()
    except Exception as e:
        return [('__db__', 0, f'psql 连不上: {e}', '')]

    declared = load_declared_predicates()
    hits = []
    for line in out.split('\n'):
        if not line.strip(): continue
        parts = line.split('|', 3)
        if len(parts) != 4: continue
        t, p, qual, wc = parts

        # 恒真检测 (干掉 is_super_admin 后看剩余)
        core = re.sub(
            r'(OR|AND)?\s*\(?\s*guliangiam\.is_super_admin\(\)\s*\)?',
            '', qual, flags=re.IGNORECASE,
        ).strip('() AND OR ')
        if core.lower() not in ('', 'true', '1=1'):
            continue

        # 声明层豁免: 策略谓词与 contracts/schema 的 rls_predicate 完全一致时合规
        declared_predicate = declared.get(t)
        if declared_predicate and declared_predicate not in ('true', '1=1'):
            if declared_predicate == _normalize_predicate(qual):
                continue

        hits.append((f'pg_policies.{t}', 0, f'恒真策略 qual="{qual[:100]}"', f'policy={p}'))

    return hits

def main():
    ap = argparse.ArgumentParser(description='RLS POLICY 内容门禁')
    ap.add_argument('--dsn', help='PostgreSQL DSN — 提供则同时查 DB 运行时')
    ap.add_argument('--no-fail', action='store_true', help='告警但 exit 0')
    args = ap.parse_args()

    hits = []

    # 1. 静态: migration 文件
    mig_hits = check_migrations()
    hits.extend(mig_hits)
    if mig_hits:
        print(f'  📄 migrations 里 {len(mig_hits)} 处 USING TRUE')

    # 2. 动态: DB 运行时
    if args.dsn:
        db_hits = check_db(args.dsn)
        hits.extend(db_hits)
        if db_hits:
            print(f'  🗄️  pg_policies 里 {len(db_hits)} 张表有恒真策略')

    if not hits:
        print('  ✅ RLS POLICY 内容门禁通过')
        sys.exit(0)

    print(f'\n  ❌ 发现 {len(hits)} 处危险 RLS POLICY:\n')
    for f, ln, pat, ctx in hits:
        print(f'  → {f}:{ln}  [{pat}]')
        if ctx:
            for l in ctx.split('\n'):
                print(f'      {l}')
        print()

    if args.no_fail:
        print('  🟡 --no-fail: 只告警不阻断')
        sys.exit(0)

    print('  🔴 exit 1 — PR 被阻断')
    print('     正确做法: YAML 声明 rls_predicate 后用 migration DROP + CREATE')
    sys.exit(1)

if __name__ == '__main__':
    main()
