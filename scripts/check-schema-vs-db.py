#!/usr/bin/env python3
"""check-schema-vs-db.py — 声明式 Schema (contracts/schema/*.yaml) ↔ 真实运行时门禁。

为什么需要本门禁：
  `check-schema-consistency.sh` / `check-schema-parity.py` 只校验 YAML ↔ 生成的
  DDL 文件（sql 文件可能陈旧），从不连接真实 DB。本脚本直接查 pg_tables /
  pg_class / information_schema.columns（PG）或 information_schema.tables /
  information_schema.columns（MySQL），抓 YAML 声明但 DB 漏建的表（功能代码
  已 SELECT/INSERT 会 panic）、DB 有但 YAML 没声明的表、RLS 标记矛盾（仅 PG）、
  列级差异。

  触发场景（典型）：改了 YAML 但忘了跑 migration；migration 跑了一半；新功能
  代码提交但 DB rollout 没跟上。

用法：
  python3 scripts/check-schema-vs-db.py                    # 读 .env / 默认值（PG）
  python3 scripts/check-schema-vs-db.py --dialect postgresql --dsn postgres://...
  python3 scripts/check-schema-vs-db.py --dialect mysql
  python3 scripts/check-schema-vs-db.py --fail-fast        # 遇到第一处差异立即退出

退出码：
  0 — 完美对齐（YAML = DB，零差异）
  1 — 存在差异（表/列/RLS 任一）
  2 — 运行时错误（psql/mysql 不存在、连接不上 DB）

输入：contracts/schema/*.yaml、真实 DB(--dsn / --dialect)
输出：PASS/FAIL + 表/列/RLS 差异
--check 行为：--fail-fast 遇第一处差异立即退出；--dialect mysql 校验 MySQL 方言
消费方：独立脚本、CI
"""

from __future__ import annotations

import argparse
import os
import subprocess
import sys
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parent.parent
SCHEMA_DIR = ROOT / "contracts" / "schema"

sys.path.insert(0, str(Path(__file__).resolve().parent))

# 隐式列规则必须与生成器同源：gen_schema.py 在建 DDL 前会对每张表调用
# _apply_implicit_columns（所有表隐含 created_at；revision: true 隐含 revision +
# updated_at；soft_delete: true 隐含 deleted_at + updated_at）。本门禁若只比对
# YAML 里显式书写的列，就会把生成器自动补出来的列一律判成「DB 有 YAML 缺」——
# 全库固定 40+ 条假差异，真实的漂移被噪声淹没。
from schema_common import _apply_implicit_columns  # noqa: E402

# 非声明式表：由代码内的 infra DDL 创建，不参与 contracts/schema 声明
# （ADR-022：声明层只描述业务表）。出现在 DB 里属预期，不应算漂移。
#   schema_migrations — internal/platform/db 的迁移账本（附带 checksum 与重放记录）
NON_DECLARATIVE_TABLES = {"schema_migrations"}


# --------------------------------------------------------------------------- #
# 1. 读 YAML 声明表                                                         #
# --------------------------------------------------------------------------- #
def load_yaml_tables() -> dict[str, dict]:
    """返回 {short_name: {'rls': bool, 'cols': set[str], 'file': str}}.

    列集合包含生成器隐式补出的审计列（见上文 import 处说明），与真实 DDL 保持同源。
    """
    out: dict[str, dict] = {}
    for f in sorted(SCHEMA_DIR.glob("*.yaml")):
        with open(f) as fp:
            for doc in yaml.safe_load_all(fp):
                if not doc:
                    continue
                tname = doc.get("table_name", "")
                if not tname.startswith("guliangiam."):
                    continue
                _apply_implicit_columns(doc)
                short = tname.replace("guliangiam.", "")
                out[short] = {
                    "rls": bool(doc.get("rls", False)),
                    "cols": {c["name"] for c in doc.get("columns", [])},
                    "file": f.name,
                }
    return out


# --------------------------------------------------------------------------- #
# 2. 调 psql 拿真实 DB 状态                                                 #
# --------------------------------------------------------------------------- #
def _psql(dsn: str, sql: str) -> str:
    try:
        result = subprocess.run(
            ["psql", "-d", dsn, "-At", "-F", "|", "-c", sql],
            capture_output=True,
            text=True,
            check=False,
        )
    except FileNotFoundError:
        print("ERROR: psql 未安装。请先装 postgresql-client。", file=sys.stderr)
        sys.exit(2)
    if result.returncode != 0:
        print(f"ERROR: psql 执行失败:\n{result.stderr}", file=sys.stderr)
        sys.exit(2)
    return result.stdout.strip()


def load_db_state(dsn: str) -> tuple[dict[str, dict], dict[str, set[str]]]:
    """返回 (tables, cols).

    tables: {name: {'rls': bool, 'frls': bool}}  — 用 pg_class 的权威字段
    cols:   {name: {col_names}}
    """
    # pg_class 比 pg_tables.view 准（后者在某些角色下看不到 relforcerowsecurity）
    raw = _psql(
        dsn,
        """
        SELECT c.relname, c.relrowsecurity, c.relforcerowsecurity
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'guliangiam' AND c.relkind = 'r'
        ORDER BY c.relname;
        """,
    )
    tables: dict[str, dict] = {}
    for line in raw.split("\n"):
        if not line.strip():
            continue
        name, rls, frls = line.split("|")
        tables[name] = {"rls": rls == "t", "frls": frls == "t"}

    cols_raw = _psql(
        dsn,
        """
        SELECT table_name, column_name
        FROM information_schema.columns
        WHERE table_schema = 'guliangiam'
        ORDER BY table_name, ordinal_position;
        """,
    )
    cols: dict[str, set[str]] = {}
    for line in cols_raw.split("\n"):
        if not line.strip():
            continue
        t, c = line.split("|")
        cols.setdefault(t, set()).add(c)

    return tables, cols


# --------------------------------------------------------------------------- #
# 2b. MySQL — mysql CLI + information_schema                                #
# --------------------------------------------------------------------------- #
def _mysql(host: str, port: int, user: str, pwd: str, dbname: str, sql: str) -> str:
    try:
        result = subprocess.run(
            [
                "mysql",
                f"--host={host}",
                f"--port={port}",
                f"--user={user}",
                f"--database={dbname}",
                "--batch",
                "--raw",
                "--silent",
                "-e", sql,
            ],
            input=f"{pwd}\n" if pwd else None,
            capture_output=True,
            text=True,
            check=False,
        )
    except FileNotFoundError:
        print("ERROR: mysql 未安装。请先装 mysql-client。", file=sys.stderr)
        sys.exit(2)
    if result.returncode != 0:
        print(f"ERROR: mysql 执行失败:\n{result.stderr}", file=sys.stderr)
        sys.exit(2)
    return result.stdout.strip()


def _mysql_read_passwd(args: argparse.Namespace) -> str:
    pwd = os.environ.get("MYSQL_PASSWORD") or os.environ.get("MYSQL_PWD")
    if not pwd:
        # ~/.my.cnf 优先级低于 env，这里简化
        return ""
    return pwd


def load_db_state_mysql(host: str, port: int, user: str, pwd: str, dbname: str) -> tuple[dict[str, dict], dict[str, set[str]]]:
    """MySQL 版 — 用 information_schema。无 RLS 概念，tables.rls 全 false。"""
    raw = _mysql(
        host, port, user, pwd, dbname,
        """
        SELECT table_name
        FROM information_schema.tables
        WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'
        ORDER BY table_name;
        """,
    )
    tables: dict[str, dict] = {}
    for line in raw.split("\n"):
        name = line.strip()
        if not name:
            continue
        tables[name] = {"rls": False, "frls": False}

    cols_raw = _mysql(
        host, port, user, pwd, dbname,
        """
        SELECT table_name, column_name
        FROM information_schema.columns
        WHERE table_schema = DATABASE()
        ORDER BY table_name, ordinal_position;
        """,
    )
    cols: dict[str, set[str]] = {}
    for line in cols_raw.split("\n"):
        if not line.strip():
            continue
        t, c = line.split("\t", 1)
        cols.setdefault(t, set()).add(c)

    return tables, cols


# --------------------------------------------------------------------------- #
# 3. 对比引擎                                                               #
# --------------------------------------------------------------------------- #
def audit(
    yaml_tables: dict[str, dict],
    db_tables: dict[str, dict],
    db_cols: dict[str, set[str]],
    dialect: str = "postgresql",
):
    yaml_names = set(yaml_tables)
    # 账本等基础设施表由代码 DDL 创建，属豁免项（见 NON_DECLARATIVE_TABLES）
    db_names = set(db_tables) - NON_DECLARATIVE_TABLES

    only_in_yaml = sorted(yaml_names - db_names)
    only_in_db   = sorted(db_names   - yaml_names)
    both         = sorted(yaml_names & db_names)

    issues: list[str] = []

    # 3a. 表级差异
    for t in only_in_yaml:
        issues.append(f"YAML 有但 DB 没建: {t}  [{yaml_tables[t]['file']}]")
    for t in only_in_db:
        issues.append(f"DB 有但 YAML 没声明: {t}")

    # 3b. RLS 标记矛盾（仅 PG 有 FORCE ROW LEVEL SECURITY；MySQL 无 RLS 概念）
    warnings: list[str] = []
    if dialect == "postgresql":
        for t in both:
            yaml_rls = yaml_tables[t]["rls"]
            db_rls = db_tables[t]["rls"] and db_tables[t]["frls"]
            if yaml_rls != db_rls:
                issues.append(
                    f"RLS 矛盾: {t:40s} yaml.rls={yaml_rls}  "
                    f"db.rls={db_tables[t]['rls']} db.frls={db_tables[t]['frls']}"
                )
    else:
        # 非 PG 方言：YAML 声明 rls=true 但 DB 不支持 → 记录为 warning（非阻塞）
        for t in both:
            if yaml_tables[t]["rls"]:
                warnings.append(
                    f"  ⓘ [{dialect}] RLS 降级: {t} yaml.rls=true → 应用层 tenant_isolation 守卫"
                )

    # 3c. 列级差异
    # MySQL 等方言会加 gen_* 生成列（替代 PG 部分索引 predicate），跳过这些
    _skip_extra = {"gen_"} if dialect != "postgresql" else set()
    for t in both:
        ycols = yaml_tables[t]["cols"]
        dcols = db_cols.get(t, set())
        if ycols != dcols:
            missing_in_db = ycols - dcols
            extra_in_db   = {c for c in dcols - ycols
                             if not any(c.startswith(p) for p in _skip_extra)}
            if missing_in_db:
                issues.append(f"列级 [{t}] YAML 有 DB 缺: {sorted(missing_in_db)}")
            if extra_in_db:
                issues.append(f"列级 [{t}] DB 有 YAML 缺: {sorted(extra_in_db)}")

    return issues, warnings, only_in_yaml, only_in_db, both


# --------------------------------------------------------------------------- #
# 4. 入口                                                                   #
# --------------------------------------------------------------------------- #
def main() -> int:
    ap = argparse.ArgumentParser(
        description="YAML schema ↔ runtime consistency check (PG + MySQL)",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    ap.add_argument("--dialect", choices=["postgresql", "mysql"], default="postgresql",
                    help="数据库方言 (默认: postgresql)")
    ap.add_argument("--dsn", default=os.environ.get("DATABASE_URL"),
                    help="PostgreSQL DSN (PG 默认)")
    ap.add_argument("--host", "-H", help="数据库 host (默认: localhost)")
    ap.add_argument("--port", "-p", type=int, help="数据库 port")
    ap.add_argument("--db", "-d", help="数据库名")
    ap.add_argument("--user", "-U", help="数据库 user")
    ap.add_argument("--fail-fast", action="store_true",
                    help="遇到第一处差异立即退出")
    ap.add_argument("--no-color", action="store_true", help="关闭彩色输出")
    args = ap.parse_args()

    dialect = args.dialect

    # ── 按 dialect 组装连接参数 ──
    if dialect == "postgresql":
        host   = args.host   or os.environ.get("PGHOST")   or "localhost"
        port   = args.port   or os.environ.get("PGPORT")   or 5432
        user   = args.user   or os.environ.get("PGUSER")   or os.environ.get("USER")
        dbname = args.db     or os.environ.get("PGDATABASE") or "guliangiam"
        pwd    = os.environ.get("PGPASSWORD")
        dsn    = args.dsn or f"host={host} port={port} user={user} dbname={dbname}"
        if pwd:
            dsn += f" password={pwd}"
        db_tables, db_cols = load_db_state(dsn)
        connect_info = f"DSN: {dsn}"
    else:  # mysql
        host   = args.host   or os.environ.get("MYSQL_HOST")   or "localhost"
        port   = args.port   or os.environ.get("MYSQL_PORT")   or 3306
        user   = args.user   or os.environ.get("MYSQL_USER")   or "root"
        dbname = args.db     or os.environ.get("MYSQL_DATABASE") or "guliangiam"
        pwd    = os.environ.get("MYSQL_PASSWORD") or os.environ.get("MYSQL_PWD") or ""
        db_tables, db_cols = load_db_state_mysql(host, port, user, pwd, dbname)
        connect_info = f"mysql://{user}:***@{host}:{port}/{dbname}"

    # 彩色
    R  = "" if args.no_color else "\033[0;31m"
    G  = "" if args.no_color else "\033[0;32m"
    Y  = "" if args.no_color else "\033[1;33m"
    B  = "" if args.no_color else "\033[1m"
    NC = "" if args.no_color else "\033[0m"

    print("=" * 60)
    print(f"  {B}YAML schema ↔ {dialect} runtime 门禁{NC}")
    print("=" * 60)
    print(f"  Dialect: {dialect}")
    print(f"  {connect_info}")
    print(f"  YAML:  {SCHEMA_DIR}")
    print()

    yaml_tables = load_yaml_tables()

    issues, warnings, only_in_yaml, only_in_db, both = audit(
        yaml_tables, db_tables, db_cols, dialect
    )

    print(f"  YAML 声明表:    {len(yaml_tables)}")
    print(f"  DB 实际表:      {len(db_tables)}")
    print(f"  非声明式表(豁免): {len(set(db_tables) & NON_DECLARATIVE_TABLES)}"
          f"  {sorted(set(db_tables) & NON_DECLARATIVE_TABLES)}")
    print(f"  两边都有:       {len(both)}")
    print(f"  YAML 独有 (DB缺): {len(only_in_yaml)}")
    print(f"  DB 独有   (YAML缺): {len(only_in_db)}")

    yaml_rls_count = sum(1 for v in yaml_tables.values() if v["rls"])
    if dialect == "postgresql":
        db_rls_count = sum(1 for v in db_tables.values() if v["rls"] and v["frls"])
    else:
        db_rls_count = 0  # MySQL 无 RLS
    print(f"  YAML rls=T:     {yaml_rls_count}")
    print(f"  DB RLS 实际启用: {db_rls_count}")
    print()

    # 非阻塞 warnings（RLS 降级等）
    if warnings:
        print(f"  {Y}⚠ 方言降级 warnings（非阻塞）{NC}")
        for w in warnings[:5]:
            print(w)
        if len(warnings) > 5:
            print(f"  ... 还有 {len(warnings) - 5} 条")
        print()

    if not issues:
        print(f"  {G}{B}🎉 完美对齐！零差异{NC}")
        print()
        return 0

    print(f"  {R}{B}发现 {len(issues)} 处差异{NC}")
    print()
    for i, issue in enumerate(issues, 1):
        print(f"  {R}  [{i:02d}]{NC} {issue}")
        if args.fail_fast:
            break
    print()
    print(f"  {Y}修复流程：{NC}")
    print(f"    1. YAML 有 DB 缺 → 写 migration CREATE TABLE + ALTER ADD COLUMN")
    print(f"    2. DB 有 YAML 缺 → 把声明补进 contracts/schema/*.yaml 对应文件")
    if dialect == "postgresql":
        print(f"    3. RLS 矛盾      → 统一 rls: true/false + ALTER ENABLE/DISABLE RLS")
    print(f"    4. 跑 migration → 重跑本脚本确认 zero drift")
    print()
    return 1


if __name__ == "__main__":
    sys.exit(main())
