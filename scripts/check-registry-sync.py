#!/usr/bin/env python3
"""注册表与代码事实源一致性门禁

消费 YAML 注册表（contracts/registries/*.yaml），与代码事实源做双向校验。
每个检查函数独立 PASS/FAIL，全部通过后脚本退出码为 0。

--strict：额外要求 11 个注册表逐一被门禁消费（本脚本解析，或由 DELEGATED_CONSUMERS
登记的脚本解析且该脚本真实引用注册表文件名），任一注册表无消费方即 FAIL。

输入：contracts/registries/*.yaml、internal/错误码/唯一键/... 事实源
输出：PASS/FAIL + 双向不一致项；--strict 追加消费覆盖校验
--check 行为：--strict 追加「注册表必须有消费方」校验
消费方：verify-docs.sh(RUN_CODE_CHECKS=1, --strict)、CI
"""
import re, sys, yaml
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
PASS = True
STRICT = "--strict" in sys.argv

# contracts/registries/*.yaml 全集（--strict 要求逐一有消费方）
REGISTRIES = [
    "surfaces.yaml", "cache.yaml", "env.yaml", "errors.yaml",
    "limits.yaml", "naming.yaml", "open-items.yaml", "outbox.yaml",
    "security-readiness.yaml", "slo.yaml", "state-machines.yaml",
    "short-lived-state.yaml", "known-red.yaml",
    "events.yaml", "lifecycle.yaml", "permissions.yaml", "dr.yaml",
    "referential-integrity.yaml", "crypto.yaml",
    "password-change-exempt-routes.yaml",
]

# 由其他门禁脚本消费的注册表：本脚本不重复解析，但严格模式校验消费方真实存在且引用该文件
DELEGATED_CONSUMERS = {
    "security-readiness.yaml": "scripts/check-security-readiness.py",
    "short-lived-state.yaml": "scripts/check-short-lived-state.py",
    "known-red.yaml": "scripts/verify-docs.sh",
}

# 本门禁实际解析过的注册表（_load_yaml 自动记录）
CONSUMED: set[str] = set()


# ── 辅助函数 ──────────────────────────────────────────────────────────────

def _load_yaml(relpath: str) -> dict:
    """安全加载 YAML 注册表；同时记录消费事实。

    输入缺失即失败：本函数的所有调用点都指向 REGISTRIES 登记的注册表，缺失说明仓库
    结构被破坏（而不是「这个检查按设计可选」），strict 模式下不能当作通过。
    """
    p = ROOT / relpath
    if not p.exists():
        _check(f"必需输入 {relpath}", False,
               "注册表不存在——输入缺失说明仓库结构被破坏，strict 模式下不能当作通过")
        return {}
    CONSUMED.add(Path(relpath).name)
    return yaml.safe_load(p.read_text()) or {}


def _go_env_vars() -> set[str]:
    """扫描 internal/**/*.go 的环境变量读取点（跳过测试文件）。

    识别三种读取形态，避免把「经常量读取」的变量误判为注册表冗余：
      1. 字面量：os.Getenv("CORS_ALLOWED_ORIGINS") / envOrEmpty("X")
      2. os.LookupEnv(...)
      3. 常量间接：os.Getenv(EnvCORSAllowedOrigins)，常量在同仓声明为字符串字面量
    """
    go_vars: set[str] = set()
    go_dir = ROOT / "internal"
    if not go_dir.exists():
        return go_vars

    texts: list[str] = []
    for path in go_dir.rglob("*.go"):
        if path.name.endswith("_test.go"):
            continue
        try:
            texts.append(path.read_text())
        except (OSError, UnicodeDecodeError):
            continue

    # 常量名 → 字符串值（仅纳唯一映射；重名歧义直接丢弃，宁可漏判不可误判）
    const_values: dict[str, set[str]] = {}
    for text in texts:
        for name, val in re.findall(
            r'\b([A-Z][A-Za-z0-9_]*)\s*(?:string\s*)?(?:=|:=)\s*"([A-Z][A-Z0-9_]+)"', text
        ):
            const_values.setdefault(name, set()).add(val)

    # 仅登记「首参为环境变量名」的读取助手；envTruthy(raw) 之类接收值而非
    # key 的函数不得加入，否则会把普通字符串误判成环境变量读取点。
    helpers = [
        "envOrEmpty", "envFloat", "envInt", "getEnvInt", "envIntOr",
        "envDuration", "getEnvBool", "EnvTruthy", "parseFloatKey", "parseIntKey",
    ]
    reader = "(?:os\\.Getenv|os\\.LookupEnv|" + "|".join(re.escape(h) for h in helpers) + ")"

    for text in texts:
        go_vars.update(re.findall(reader + r'\(\s*"([A-Z][A-Z0-9_]+)"', text))
        for ident in re.findall(reader + r'\(\s*([A-Z][A-Za-z0-9_]*)\s*[,)]', text):
            vals = const_values.get(ident)
            if vals and len(vals) == 1:
                go_vars.update(vals)
    return go_vars


def _go_test_env_vars() -> set[str]:
    """扫描仓库内 *_test.go 的环境变量读取点（用于核验 test_only 登记）。

    test_only 变量不参与运行时配置，不能要求 internal/ 非测试代码读取；但登记必须
    对应真实测试消费方，否则等同凭空登记。只认 os.Getenv/os.LookupEnv 字面量。
    """
    go_vars: set[str] = set()
    for path in ROOT.rglob("*_test.go"):
        if {"node_modules", "vendor"}.intersection(path.parts):
            continue
        try:
            text = path.read_text()
        except (OSError, UnicodeDecodeError):
            continue
        go_vars.update(
            re.findall(r'(?:os\.Getenv|os\.LookupEnv)\s*\(\s*"([A-Z][A-Z0-9_]+)"', text)
        )
    return go_vars


def _check(name: str, ok: bool, detail: str):
    global PASS
    mark = "PASS" if ok else "FAIL"
    print(f"{mark}: {name} — {detail}")
    if not ok:
        PASS = False


# ── 1. 错误码注册表 ↔ errors.go ↔ OpenAPI enum ────────────────────────────

def check_error_codes():
    """errors.yaml ↔ internal/platform/errors/errors.go

    注意：协议面错误码（protocol.oauth2 / protocol.scim）与应用错误码是两个命名空间，
    Go 代码中协议错误码在协议适配层处理，不经过 errors.go 的 Code 常量。
    因此只对比 application 节 ↔ errors.go。
    """
    data = _load_yaml("contracts/registries/errors.yaml")
    if not data:
        return

    # 只取 application 节与 errors.go 对比
    app_codes = set()
    for entry in data.get("application", []) or []:
        if isinstance(entry, dict) and entry.get("code"):
            app_codes.add(entry["code"])

    # protocol 节只验证有登记，不对比 Go 常量
    proto_codes = set()
    for entry in data.get("protocol", []) or []:
        if isinstance(entry, dict) and entry.get("code"):
            proto_codes.add(entry["code"])

    _check("errors.yaml 含 protocol OAuth+SCIM 登记",
           len(proto_codes) > 0,
           f"protocol 节 {len(proto_codes)} 个")

    go_path = ROOT / "internal/platform/errors/errors.go"
    if not go_path.exists():
        _check("应用错误码 ↔ errors.go", False, "errors.go 不存在")
        return

    go_src = go_path.read_text()
    go_consts = set(re.findall(r'Code\s+=\s+"(\w+)"', go_src))

    missing = app_codes - go_consts
    extra = go_consts - app_codes

    _check("应用错误码（application）↔ errors.go", not missing,
           f"注册表 {len(app_codes)} / Go 常量 {len(go_consts)}；"
           f"缺失 {missing or 'none'}；多余 {extra or 'none'}（WARN 级）")


# ── 2. 加密信封唯一约束（SQL DDL 直接检查，无 YAML） ───────────────────────

def check_encryption_keys_unique():
    ddl_path = ROOT / "db/generated/postgresql/current.up.sql"
    if not ddl_path.exists():
        _check("encryption_keys 唯一约束", False,
               "必需输入 db/generated/postgresql/current.up.sql 不存在——输入缺失说明仓库结构被破坏，"
               "strict 模式下不能当作通过")
        return

    ddl = ddl_path.read_text()
    _check("encryption_keys 唯一约束 = (purpose, key_id, key_version)",
           "uq_encryption_keys_purpose_key_version" in ddl,
           f"{'存在' if 'uq_encryption_keys_purpose_key_version' in ddl else '缺失'}")


# ── 3. Outbox max_attempts ↔ DDL 默认值 ↔ Worker 配置 ─────────────────────

def check_outbox_params():
    """outbox.yaml.worker.max_attempts ↔ DDL DEFAULT ↔ outbox 状态机"""
    data = _load_yaml("contracts/registries/outbox.yaml")
    if not data:
        return

    # 注册表声明的 max_attempts
    declared_max = data.get("worker", {}).get("max_attempts")
    _check("outbox.yaml 登记 max_attempts", declared_max == 12,
           f"值={declared_max}（应为 12）")

    # DDL 中的 outbox 表
    ddl_path = ROOT / "db/generated/postgresql/current.up.sql"
    if ddl_path.exists():
        ddl = ddl_path.read_text()
        outbox_section = ""
        if "CREATE TABLE.*outbox" in ddl or "CREATE TABLE outbox" in ddl:
            # 提取 CREATE TABLE outbox ... 到 CREATE INDEX 之间
            m = re.search(r"CREATE TABLE outbox[\s\S]*?(?=\nCREATE)", ddl, re.IGNORECASE)
            if m:
                outbox_section = m.group(0)

        _check("outbox DDL max_attempts 默认值",
               "DEFAULT 12" in outbox_section or "DEFAULT 12" in ddl,
               f"DEFAULT 12 {'存在' if 'DEFAULT 12' in outbox_section or 'DEFAULT 12' in ddl else '缺失'}")

    # 状态机注册表一致性
    sm_data = _load_yaml("contracts/registries/state-machines.yaml")
    if sm_data:
        outbox_sm = sm_data.get("outbox", {})
        transitions = outbox_sm.get("transitions", {})
        _check("outbox 状态机 transitions 含 failed→pending",
               "failed" in transitions and "pending" in transitions.get("failed", []),
               f"transitions.failed = {transitions.get('failed')}")
        _check("outbox 状态机终态含 succeeded + dead",
               set(outbox_sm.get("terminal", [])) == {"succeeded", "dead"},
               f"terminal = {outbox_sm.get('terminal')}")


# ── 4. 开放项标识必须登记 ──────────────────────────────────────────────────

# ── 4.5 状态机注册表 ↔ 声明式 Schema ────────────────────────────────────────

def check_state_machine_schema():
    """注册表声明的 check_constraint 必须真实存在于声明式 Schema。

    ADR-022 规定 contracts/schema/*.yaml 是唯一结构源；状态机注册表不是约束定义的来源，
    但它驱动领域状态常量生成。两者曾长期分岔：sessions.session_state 与
    user_identities.link_state 在注册表里有约束定义，Schema 里却既没有列也没有
    约束，且没有任何门禁能发现。此检查把「注册表声明的约束必须落到 Schema」
    变成硬约束：要么补齐 Schema，要么从注册表移除幽灵条目。
    """
    sm_data = _load_yaml("contracts/registries/state-machines.yaml")
    if not sm_data:
        return

    tables: dict[str, dict] = {}
    for fp in sorted((ROOT / "contracts" / "schema").glob("*.yaml")):
        with open(fp, encoding="utf-8") as fh:
            for doc in yaml.safe_load_all(fh):
                if doc and doc.get("table_name"):
                    tables[doc["table_name"].split(".")[-1]] = doc

    problems: list[str] = []
    declared_count = 0
    for key, entry in sm_data.items():
        if not isinstance(entry, dict):
            continue
        table = entry.get("table")
        constraint = entry.get("check_constraint")
        column = entry.get("column")
        if not table or not constraint:
            continue
        declared_count += 1
        constraints = constraint if isinstance(constraint, list) else [constraint]
        columns = column if isinstance(column, list) else [column]
        target = tables.get(str(table).split(".")[-1])
        if target is None:
            problems.append(f"{key}: 表 {table} 不在声明式 Schema")
            continue
        declared = {c.get("name") for c in (target.get("constraints") or []) if isinstance(c, dict)}
        schema_columns = {c.get("name") for c in (target.get("columns") or []) if isinstance(c, dict)}
        for name in constraints:
            if name not in declared:
                problems.append(f"{key}: 约束 {name} 未在 contracts/schema 声明")
        for col in columns:
            if col and col not in schema_columns:
                problems.append(f"{key}: 列 {col} 不在 {table}")

    _check("状态机注册表 check_constraint 全部落到声明式 Schema",
           not problems,
           "; ".join(problems[:4]) if problems else f"{declared_count} 条约束声明全部命中")


def check_open_items():
    """open-items.yaml 中登记的 OPEN-* ↔ 代码注释中的引用"""
    data = _load_yaml("contracts/registries/open-items.yaml")
    if not data:
        return

    items = data.get("items", [])
    registered = {item["id"] for item in items if item.get("id")}
    _check("开放项注册表登记", len(registered) > 0, f"{len(registered)} 条登记")

    if not registered:
        return

    # 全仓扫描 OPEN-* 引用
    found: dict[str, list[str]] = {}
    skip_dirs = {"node_modules", ".git", "casdoor-master", "__pycache__"}
    skip_exts = {".sum", ".png", ".jpg", ".ico", ".pdf", ".lock", ".sqlite"}

    for rel in ("internal", "scripts", "db", "web/src", "openapi", "docs"):
        base = ROOT / rel
        if not base.exists():
            continue
        for path in base.rglob("*"):
            if not path.is_file():
                continue
            if any(d in skip_dirs for d in path.parts):
                continue
            if path.suffix in skip_exts:
                continue
            try:
                text = path.read_text(encoding="utf-8")
            except (UnicodeDecodeError, OSError):
                continue
            for ident in re.findall(r"\b(OPEN-[A-Z]+-\d+)\b", text):
                found.setdefault(ident, []).append(str(path.relative_to(ROOT)))

    unregistered = sorted(set(found) - registered)
    for ident in unregistered:
        _check(f"开放项 {ident} 已登记", False,
               f"未登记引用点: {found[ident][:3]}{'...' if len(found[ident]) > 3 else ''}")
    if not unregistered:
        _check("所有 OPEN-* 引用均已登记", True,
               f"{len(set(found))} 个标识在代码中被引用")

    unused = sorted(registered - set(found))
    if unused:
        print(f"  WARN: 已登记但无代码引用的开放项: {unused}")


# ── 5. 环境变量注册表 ↔ os.Getenv 调用点 ────────────────────────────────────

def check_env_vars():
    """env.yaml 登记的变量名 ↔ internal/**/*.go 中的读取点"""
    data = _load_yaml("contracts/registries/env.yaml")
    if not data:
        return

    # 收集注册表中所有登记的变量名
    registry_vars: set[str] = set()
    frontend_vars: set[str] = set()  # D10_frontend_build：浏览器/构建期变量，Go 运行时本就不读取
    exempt_vars: set[str] = set()    # 显式标注 retired（已退役无消费方）或 pending_implementation（实现待落）的变量
    test_only_vars: set[str] = set() # 显式标注 test_only：仅 *_test.go 读取，不参与运行时配置
    for group_name, group in (data.get("groups") or {}).items():
        for var in group.get("vars", []) or []:
            name = var.get("name")
            if not name:
                continue
            registry_vars.add(name)
            if group_name == "D10_frontend_build":
                frontend_vars.add(name)
            if var.get("retired") or var.get("pending_implementation"):
                exempt_vars.add(name)
            if var.get("test_only"):
                test_only_vars.add(name)

    _check("env.yaml 变量名已收集", len(registry_vars) > 0,
           f"注册表登记 {len(registry_vars)} 个变量")

    # 扫描 Go 代码中的环境变量读取点
    go_vars = _go_env_vars()
    if not go_vars:
        return

    # 代码中有但注册表没登记 — 与「登记了但无读取点」同为单一事实源缺口，硬性失败。
    # 环境变量只允许一个事实源（env.yaml）：未登记变量在部署时无人知道要注入什么，
    # .env.example 也不会生成它。确属平台注入/构建期变量时也必须显式登记并注明理由。
    unregistered = go_vars - registry_vars
    _check("Go 读取的环境变量均已在 env.yaml 登记",
           not unregistered,
           f"未登记 {len(unregistered)} 个: {sorted(unregistered)[:10]}"
           f"{'...' if len(unregistered) > 10 else ''}")

    # 注册表有但代码没用到的 — 硬性缺口
    # 豁免：D10 前端构建变量（由 web/ 读取，非 Go 运行时）；显式 retired（已退役无消费方）；
    # 显式 pending_implementation（实现待落，登记意图但 Go 读取点尚未落地）；
    # 显式 test_only（仅 *_test.go 读取，下方单独核验测试读取点存在）。
    unused = registry_vars - go_vars - frontend_vars - exempt_vars - test_only_vars
    _check("env.yaml 登记变量均有 Go 读取点（D10/retired/pending/test_only 豁免）",
           not unused,
           f"无读取点 {len(unused)} 个: {sorted(unused)[:10]}{'...' if len(unused) > 10 else ''}"
           if unused else
           f"{len(registry_vars) - len(frontend_vars) - len(exempt_vars) - len(test_only_vars)} 个运行时变量均有读取点"
           f"（豁免 {len(frontend_vars)} 个前端构建 + {len(exempt_vars)} 个 retired/pending"
           f" + {len(test_only_vars)} 个 test_only）")

    test_reads = _go_test_env_vars()
    unverified_test_only = sorted(test_only_vars - test_reads)
    _check("env.yaml test_only 变量均有 *_test.go 读取点",
           not unverified_test_only,
           f"无测试读取点 {len(unverified_test_only)} 个: {unverified_test_only[:10]}"
           if unverified_test_only else
           f"{len(test_only_vars)} 个 test_only 变量均有测试读取点")


# ── 6. 命名注册表：禁用拼写检查脚本存在 ────────────────────────────────────

def check_naming_gate():
    """naming.yaml 结构完整 + check-naming.sh 门禁存在"""
    naming_gate = ROOT / "scripts/check-naming.sh"

    data = _load_yaml("contracts/registries/naming.yaml")
    if data:
        conflicts = data.get("conflicts") or []
        _check("naming.yaml conflicts 逐条含 conflict/ruling/reason",
               conflicts and all(c.get("conflict") and c.get("ruling") and c.get("reason") for c in conflicts),
               f"{len(conflicts)} 条")

        concepts = data.get("concepts") or {}
        thin = [name for name, c in concepts.items()
                if not c.get("zh") or not c.get("en")
                or not any(c.get(k) for k in ("json", "sql", "url", "claim", "json_header"))]
        _check("naming.yaml concepts 逐条含中英文名与至少一种落点（json/sql/url/claim）",
               concepts and not thin, f"{len(concepts)} 个概念；不完整 {thin or 'none'}")

        protocols = data.get("protocol_exceptions") or []
        _check("naming.yaml protocol_exceptions 逐条含 style 与 examples",
               protocols and all(p.get("style") and p.get("examples") for p in protocols),
               f"{len(protocols)} 个协议")

    _check("check-naming.sh 门禁存在", naming_gate.exists(),
           f"{'存在' if naming_gate.exists() else '缺失'}")


# ── 7. API 契约注册表 ↔ route.SurfaceFor 唯一分类器 ────────────────────────

def _norm_prefix(p: str) -> str:
    """把注册表前缀归一为根前缀：去 {占位符}、去通配符、去斜杠。"""
    p = re.sub(r"\{[^}]*\}", "", p)
    return p.replace("*", "").strip("/")


def check_api_contracts():
    """surfaces.yaml ↔ internal/platform/registry/surface.go（三面分类唯一事实源）

    三面常量与协议面家族前缀必须与 SurfaceFor 的唯一分类器同源：
    根级前缀集合 = protocolPathPrefixes ∪ {authPrefix}。
    `internal/transport/http/route/surface.go` 只是 Type-Alias Bridge（转发到 registry），
    故真实常量与协议前缀须在 platform/registry 校验。
    """
    data = _load_yaml("contracts/registries/surfaces.yaml")
    if not data:
        return

    src_path = ROOT / "internal/platform/registry/surface.go"
    if not src_path.exists():
        _check("surfaces.yaml ↔ surface.go", False, "platform/registry/surface.go 不存在")
        return
    src = src_path.read_text()

    # 三面常量集合
    declared = {s.get("surface") for s in data.get("surfaces", []) or [] if s.get("surface")}
    code_surfaces = set(re.findall(r'Surface\w+\s+Surface\s*=\s*"([a-z]+)"', src))
    _check("surfaces.yaml surfaces ↔ surface.go 三面常量",
           declared and declared == code_surfaces,
           f"注册表 {sorted(declared)} / 代码 {sorted(code_surfaces)}")

    # 根级前缀集合
    fam_prefixes = {_norm_prefix(f["prefix"]) for f in data.get("protocol_families", []) or [] if f.get("prefix")}
    m = re.search(r"protocolPathPrefixes\s*=\s*\[\]string\{([\s\S]*?)\n\}", src)
    code_seq = re.findall(r'"([^"]+)"', m.group(1)) if m else []
    code_prefixes = {_norm_prefix(p) for p in code_seq} | {"api/v1/auth"}
    _check("surfaces.yaml protocol_families ↔ protocolPathPrefixes ∪ authPrefix",
           fam_prefixes and fam_prefixes == code_prefixes,
           f"注册表 {sorted(fam_prefixes)} / 代码 {sorted(code_prefixes)}")

    # 家族编号顺序必须与分类器声明顺序一致（⑤ = auth 内协议端点，⑥ = SAML，⑦ = AI 协议面）
    order_ids = "①②③④⑤⑥⑦"
    families = [f for f in data.get("protocol_families", []) or [] if f.get("id") in order_ids]
    seq = [_norm_prefix(f["prefix"]) for f in sorted(families, key=lambda f: order_ids.index(f["id"]))]
    expected = [_norm_prefix(p) for p in code_seq[:4]] + ["api/v1/auth"] + [_norm_prefix(p) for p in code_seq[4:]]
    _check("surfaces.yaml 家族编号顺序 ↔ surface.go 家族声明顺序",
           seq == expected,
           f"注册表 {seq} / 期望 {expected}")

    # 三面契约要素：登记的错误体 / 成功 envelope / 认证方案 / 分页组件名
    # 必须在生成的 openapi/openapi.yaml 中真实存在（组件形状是生成物，不在此重复）。
    ce = data.get("contract_elements")
    oa_path = ROOT / "openapi/openapi.yaml"
    if not ce:
        _check("surfaces.yaml 三面契约要素块", False, "缺 contract_elements")
    elif not oa_path.exists():
        _check("surfaces.yaml 三面契约要素 ↔ openapi/openapi.yaml", False, "openapi/openapi.yaml 不存在")
    else:
        oa = oa_path.read_text()
        eb = ce.get("error_body") or {}
        env = ce.get("success_envelope") or {}
        components = [
            eb.get("management"), eb.get("authentication"), eb.get("protocol"),
            env.get("detail"), env.get("list"), env.get("status"),
            (ce.get("pagination") or {}).get("response_meta"),
            *(ce.get("auth_schemes") or []),
        ]
        missing = [c for c in components if c and f"    {c}:" not in oa]
        _check("surfaces.yaml 三面契约组件 ↔ openapi/openapi.yaml components",
               not missing, f"注册表声明但 OpenAPI 缺失：{missing}")

        params = (ce.get("pagination") or {}).get("request_params") or []
        missing_params = [p for p in params if f"\n      name: {p}\n" not in oa]
        _check("surfaces.yaml 分页请求参数 ↔ openapi/openapi.yaml parameters",
               not missing_params, f"注册表声明但 OpenAPI 缺失：{missing_params}")


# ── 8. 缓存注册表 ↔ 进程内缓存实现符号 ────────────────────────────────────

def _go_symbol_present(symbol: str) -> bool:
    for path in ROOT.joinpath("internal").rglob("*.go"):
        if path.name.endswith("_test.go"):
            continue
        try:
            if symbol in path.read_text():
                return True
        except (OSError, UnicodeDecodeError):
            continue
    return False


def check_cache_registry():
    """cache.yaml：进程内缓存上限 + storage 符号可解析 + 键命名与红线结构完整"""
    data = _load_yaml("contracts/registries/cache.yaml")
    if not data:
        return

    layers = data.get("layers") or []
    incomplete = [l.get("name") for l in layers
                  if not all(l.get(k) for k in ("name", "source", "ttl", "failure_behavior"))]
    _check("cache.yaml layers 逐条含 source/ttl/failure_behavior",
           layers and not incomplete, f"{len(layers)} 层；缺字段 {incomplete or 'none'}")

    inproc = data.get("in_process") or []
    _check("cache.yaml in_process 仅登记 2 项（进程内缓存上限）",
           len(inproc) == 2, f"登记 {len(inproc)} 项")

    unresolved = []
    for entry in inproc:
        storage = (entry.get("storage") or "").strip()
        # 取 "A + B" 中最后一个具体类型作为代码锚点
        symbol = storage.split("+")[-1].strip()
        if not symbol or not _go_symbol_present(symbol):
            unresolved.append(f"{entry.get('name')}: {symbol or '空'}")
    _check("cache.yaml in_process storage 符号在 Go 代码中可解析",
           not unresolved, f"不可解析 {unresolved or 'none'}")

    rules = data.get("key_naming_rules") or []
    tenant = [r for r in rules
              if "org:{organization_id}:" in f"{r.get('rule', '')}{r.get('example', '')}"]
    _check("cache.yaml 键命名规则含租户维度前置（org:{organization_id}:）",
           len(tenant) == 1, f"命中 {len(tenant)} 条 / 共 {len(rules)} 条规则")

    redlines = data.get("redlines") or []
    _check("cache.yaml 红线 4 条且逐条有文本",
           len(redlines) == 4 and all(r.get("text") for r in redlines),
           f"{len(redlines)} 条")


# ── 9. SLO 注册表 ↔ 窗口算术 ↔ Prometheus 指标注册名 ───────────────────────

def _metric_name(expr: str) -> str:
    """从 PromQL 表达式或「名称（中文备注）」注释中取出裸指标名。"""
    if "（" in expr:
        expr = expr.split("（")[0]
    return expr.split("{")[0].strip()


def check_slo_registry():
    """slo.yaml：窗口算术自洽 + 错误预算算术自洽 + 指标名在 prometheus.go 可解析"""
    data = _load_yaml("contracts/registries/slo.yaml")
    if not data:
        return

    days = data.get("window_days")
    mins = data.get("window_minutes")
    _check("slo.yaml 窗口算术自洽（window_minutes = window_days × 1440）",
           bool(days) and mins == days * 24 * 60,
           f"{days} 天 → {mins} 分钟（应 {days * 24 * 60 if days else '?'}）")

    # Prometheus 注册名（含可选的 Namespace 前缀；Namespace 可以是字面量或常量）
    prom_path = ROOT / "internal/platform/middleware/prometheus.go"
    registered: set[str] = set()
    if prom_path.exists():
        prom = prom_path.read_text()
        prefix = ""
        ns_literal = re.search(r'Namespace:\s*"([a-z][a-z0-9_]*)"', prom)
        if ns_literal:
            prefix = ns_literal.group(1) + "_"
        else:
            ns_ref = re.search(r"Namespace:\s*(\w+)", prom)
            if ns_ref:
                ns_const = re.search(re.escape(ns_ref.group(1)) + r'\s*=\s*"([a-z][a-z0-9_]*)"', prom)
                if ns_const:
                    prefix = ns_const.group(1) + "_"
        registered = {prefix + n for n in re.findall(r'Name:\s*"([a-z][a-z0-9_]*)"', prom)}

    metrics = []
    for sli in data.get("slis") or []:
        for key in ("metric", "metric_denom", "metric_errors"):
            value = sli.get(key)
            if isinstance(value, str) and value.strip():
                metrics.append((sli.get("id"), _metric_name(value)))
    for slo in data.get("slos") or []:
        value = slo.get("metric")
        if isinstance(value, str) and value.strip():
            metrics.append((slo.get("surface"), _metric_name(value)))

    unknown = sorted({f"{owner}: {name}" for owner, name in metrics if registered and name not in registered})
    _check("slo.yaml 全部 SLI/SLO 指标名在 prometheus.go 中已注册",
           registered and not unknown,
           f"注册名 {sorted(registered)}；无法解析 {unknown or 'none'}")

    # 错误预算算术：window_minutes ×(100−availability)/100
    bad_budget = []
    for slo in data.get("slos") or []:
        avail = slo.get("availability")
        budget = slo.get("error_budget_minutes")
        if not isinstance(avail, (int, float)) or not isinstance(budget, (int, float)) or not mins:
            continue
        expected = mins * (100 - avail) / 100
        if abs(budget - expected) > 0.01:
            bad_budget.append(f"{slo.get('surface')}: {budget} ≠ {round(expected, 2)}")
    _check("slo.yaml 每面 error_budget_minutes = window_minutes ×(100−availability)/100",
           not bad_budget, f"不符 {bad_budget or 'none'}")

    sli_ids = {s.get("id") for s in data.get("slis") or []}
    dangling = [s.get("surface") for s in data.get("slos") or [] if s.get("sli") not in sli_ids]
    _check("slo.yaml 每条 slo.sli 指向已登记 SLI", not dangling, f"悬空 {dangling or 'none'}")


# ── 10. 限流参数注册表 ↔ 环境变量注册表 ↔ limiter 默认值 ───────────────────

def check_limits_registry():
    """limits.yaml：env_rate/env_burst 双登记 + policy_id 齐备 + 响应头覆盖 IETF 契约"""
    data = _load_yaml("contracts/registries/limits.yaml")
    if not data:
        return

    env_data = _load_yaml("contracts/registries/env.yaml") or {}
    registry_vars: set[str] = set()
    exempt_vars: set[str] = set()
    for group in (env_data.get("groups") or {}).values():
        for var in group.get("vars", []) or []:
            if var.get("name"):
                registry_vars.add(var["name"])
                if var.get("retired") or var.get("pending_implementation"):
                    exempt_vars.add(var["name"])
    go_vars = _go_env_vars()

    rls = data.get("ratelimiters") or []
    gaps = []
    for rl in rls:
        if rl.get("type") == "business_counter":
            # 业务计数器非中间件限流器：不读限流环境变量
            continue
        for key in ("env_rate", "env_burst"):
            name = rl.get(key)
            if not name:
                gaps.append(f"{rl.get('name')}.{key} 缺登记")
                continue
            if name not in registry_vars:
                gaps.append(f"{name} 未登记 env.yaml")
            elif name not in go_vars and name not in exempt_vars:
                gaps.append(f"{name} 无 Go 读取点")
    _check("limits.yaml env_rate/env_burst ↔ env.yaml 且 Go 有读取点",
           rls and not gaps, f"{len(rls)} 个限流器；缺口 {gaps or 'none'}")

    # 中间件限流器必须登记 policy_id（进入 RateLimit-Policy comment 参数）
    no_policy = [rl.get("name") for rl in rls
                 if rl.get("type") != "business_counter" and not rl.get("policy_id")]
    _check("limits.yaml 每个中间件限流器登记 policy_id",
           not no_policy, f"缺 {no_policy or 'none'}")

    want_headers = {"RateLimit-Limit", "RateLimit-Remaining", "RateLimit-Reset",
                    "RateLimit-Policy", "Retry-After"}
    got_headers = set((data.get("response_headers") or {}).keys())
    _check("limits.yaml response_headers 覆盖 IETF 五头",
           want_headers <= got_headers, f"缺 {sorted(want_headers - got_headers) or 'none'}")

    retry_after = (data.get("response_headers") or {}).get("Retry-After") or {}
    _check("limits.yaml Retry-After 仅 429 出现",
           str(retry_after.get("only_on")) == "429",
           f"only_on={retry_after.get('only_on')}")


# ── 11b. events.yaml — 事件域命名空间基本结构 ────────────────────────────

ALLOWED_REGISTERED_WITHOUT_LITERAL = {
    "actions.execution.blocked", "actions.execution.failed", "actions.execution.succeeded",
    "audit.chain_anchored", "audit.chain_broken", "control.policy_changed",
    "control.region_registered", "control.tenant_region_changed", "coupon.redemption_conflict",
    "credential.added", "credential.removed", "dsar.request.fulfilled", "dsar.request.rejected",
    "mfa.credential.rebound", "mfa.enrolled", "mfa.recovery_code.exhausted", "mfa.recovery_code.used",
    "payment.reconciliation.mismatch", "payment.reconciliation.resolved", "policy.changed",
    "privacy.consent.granted", "privacy.consent.withdrawn", "privacy.data_erased",
    "protocol.saml.assertion_consumed", "provider.ai.activated", "provider.ai.created",
    "provider.ai.deleted", "provider.ai.disabled", "provider.ai.secret.rotated", "provider.ai.updated",
    "provider.identity.deleted", "provider.identity.disabled", "provider.identity.updated",
    "provider.notification.activated", "provider.notification.created", "provider.notification.deleted",
    "provider.notification.disabled", "provider.notification.secret.rotated", "provider.notification.updated",
    "provider.payment.activated", "provider.payment.created", "provider.payment.deleted",
    "provider.payment.disabled", "provider.payment.secret.rotated", "provider.payment.updated",
    "role.assigned", "role.revoked", "security.whitelist_miss", "service_account.disabled",
    "service_account.rotated", "session.hijack_suspected", "session.idle_expired",
    "subscription.renewed", "user.impersonation.started", "user.login_failed",
    "user.login_succeeded", "webauthn.credential.added",
    "webauthn.credential.registration_failed",
}

_EVENT_FIELD_RE = re.compile(r'(?:Action|EventType|eventType|action):\s*"([a-z][a-z0-9_]*\.[a-z0-9_.]+)"')
_EVENT_CONST_RE = re.compile(r'\bEvent[A-Za-z0-9_]*\s*=\s*"([a-z][a-z0-9_]*\.[a-z0-9_.]+)"')
# 位置参数加固：embedAudit*(...) 把 action/eventType 作为裸位置字符串字面量传入，
# 不带 `Action:`/`EventType:` 字段前缀，上面两条正则漏检（Q1 批复的核心域 11 个之外
# 还有 ~40 个域外事件经此通道逃过门禁）。在调用括号内提取点号事件名字面量即可覆盖；
# resourceType/aggregateType/surface 等位置参数是无点单 token，不会误匹配。
_EVENT_DOT_RE = re.compile(r'"([a-z][a-z0-9_]*\.[a-z0-9_.]+)"')
_EMBED_AUDIT_CALL_RE = re.compile(r'embedAudit[A-Za-z0-9_]*\s*\(')


def _balanced_call_args(text: str, open_paren_idx: int) -> str:
    """返回从 open_paren_idx 处 '(' 起、括号配平的实参子串（含首尾括号）。"""
    depth = 0
    for j in range(open_paren_idx, len(text)):
        if text[j] == "(":
            depth += 1
        elif text[j] == ")":
            depth -= 1
            if depth == 0:
                return text[open_paren_idx:j + 1]
    return text[open_paren_idx:]


def _event_literals(domains: set[str]) -> set[str]:
    found: set[str] = set()
    for path in (ROOT / "internal").rglob("*.go"):
        if path.name.endswith("_test.go"):
            continue
        text = path.read_text(encoding="utf-8", errors="ignore")
        for pattern in (_EVENT_FIELD_RE, _EVENT_CONST_RE):
            for match in pattern.finditer(text):
                event = match.group(1)
                if event.split(".", 1)[0] in domains and not event.endswith("."):
                    found.add(event)
        for call in _EMBED_AUDIT_CALL_RE.finditer(text):
            args = _balanced_call_args(text, call.end() - 1)
            for match in _EVENT_DOT_RE.finditer(args):
                event = match.group(1)
                if event.split(".", 1)[0] in domains and not event.endswith("."):
                    found.add(event)
    return found


def check_events_registry():
    """events.yaml 结构、代码事件名登记、命名策略与兼容别名。"""
    data = _load_yaml("contracts/registries/events.yaml")
    if data is None:
        return
    domains = data.get("domains") or {}
    _check("events.yaml domains 非空",
           isinstance(domains, dict) and len(domains) > 0,
           f"domains={list(domains.keys())}")
    bad = [k for k, v in domains.items()
           if not isinstance(v, list) or not all(isinstance(e, str) for e in v)]
    _check("events.yaml 各域事件名均为字符串列表", not bad, f"异常域: {bad or 'none'}")

    registered = {e for values in domains.values() for e in values}
    code_events = _event_literals(set(domains))
    missing = sorted(code_events - registered)
    _check("代码事件名已登记 events.yaml", not missing, f"未登记: {missing or 'none'}")
    corpus = "".join(path.read_text(encoding="utf-8", errors="ignore")
                     for path in (ROOT / "internal").rglob("*.go"))
    absent = sorted(e for e in registered
                    if e not in corpus and e not in ALLOWED_REGISTERED_WITHOUT_LITERAL)
    _check("events.yaml 事件名在代码有落点（动态/兼容白名单除外）",
           not absent, f"无落点且不在白名单: {absent or 'none'}")

    policy = data.get("naming_policy") or {}
    pattern = policy.get("pattern") or ""
    aliases = policy.get("legacy_aliases") or {}
    malformed = []
    if pattern:
        import re as _re
        try:
            compiled = _re.compile(pattern)
        except _re.error as exc:
            malformed.append(f"pattern invalid: {exc}")
        else:
            malformed.extend(sorted(e for e in registered if not compiled.fullmatch(e)))
    alias_bad = []
    for alias, target in aliases.items():
        if alias not in registered:
            alias_bad.append(f"alias {alias} 未登记")
        if target not in registered:
            alias_bad.append(f"target {target} 未登记")
        if alias == target:
            alias_bad.append(f"alias {alias} 指向自身")
    _check("events.yaml 命名策略 pattern 覆盖全部事件", not malformed, f"异常: {malformed or 'none'}")
    _check("events.yaml 兼容别名登记且目标存在", not alias_bad, f"异常: {alias_bad or 'none'}")


# ── 11c. lifecycle.yaml — 数据生命周期基本结构 ────────────────────────────

def check_lifecycle_registry():
    """lifecycle.yaml 基本结构：version + tables 映射"""
    data = _load_yaml("contracts/registries/lifecycle.yaml")
    if data is None:
        return
    tables = data.get("tables") or {}
    _check("lifecycle.yaml tables 非空",
           isinstance(tables, dict) and len(tables) > 0,
           f"tables={list(tables.keys())}")
    # 每个表必须有某种生命周期数值字段或 description（规则/元数据条目豁免）
    def _has_lifecycle_field(spec: dict) -> bool:
        lifecycle_kw = ("retention", "ttl", "cleanup", "completion",
                        "deletion", "rotation", "risk", "max_active", "pending")
        has_num = any(any(kw in k.lower() for kw in lifecycle_kw) for k in spec)
        # 元数据条目（如 cleanup_constraints）只有规则文本
        return has_num or "description" in spec

    bad = []
    for name, spec in tables.items():
        if not isinstance(spec, dict):
            bad.append(f"{name}: not a dict")
            continue
        if not _has_lifecycle_field(spec):
            bad.append(f"{name}: 无生命周期数值字段")
    _check("lifecycle.yaml 每个表条目有生命周期数值字段",
           not bad, f"异常: {bad or 'none'}")


# ── 11d. permissions.yaml — Casbin 权限点基本结构 ─────────────────────────

def check_permissions_registry():
    """permissions.yaml 基本结构：业务域 → resource → actions 列表"""
    data = _load_yaml("contracts/registries/permissions.yaml")
    if data is None:
        return
    _check("permissions.yaml 有顶层业务域",
           isinstance(data, dict) and len(data) > 0,
           f"顶层 keys={list(data.keys()) if isinstance(data, dict) else 'N/A'}")
    # 每个域的 resource 必须含 actions 列表
    bad = []
    for domain, resources in data.items():
        if not isinstance(resources, dict):
            bad.append(f"{domain}: not a dict")
            continue
        for res, spec in resources.items():
            if not isinstance(spec, dict) or "actions" not in spec:
                bad.append(f"{domain}.{res}: 缺 actions")
                continue
            actions = spec["actions"]
            if not isinstance(actions, list) or not all(isinstance(a, str) for a in actions):
                bad.append(f"{domain}.{res}.actions: 非字符串列表")
    _check("permissions.yaml 每条 resource 有 actions 字符串列表",
           not bad, f"异常: {bad or 'none'}")


# ── 11e. known-red.yaml — 问题与风险清单结构 ──────────────────────────────

def check_known_red_registry():
    """known-red.yaml：每条风险必须登记等级/影响范围/量化基线/出口条件/责任面。"""
    data = _load_yaml("contracts/registries/known-red.yaml")
    if data is None:
        return
    rules = data.get("rules") or []
    # 允许为空：当所有门禁真实通过后基线会清空。这里只校验类型（列表），
    # 防止误写成标量/空文档；历史关闭记录保存在文件头注释与 CHANGELOG。
    _check("known-red.yaml rules 为列表（允许为空 = 门禁全部真实通过）",
           isinstance(rules, list), f"type={type(data.get("rules")).__name__}")
    allowed = {"P0", "P1", "P2"}
    seen = set()
    duplicated = []
    bad = []
    for rule in rules:
        rid = rule.get("rule_id")
        if not rid:
            bad.append("缺 rule_id")
            continue
        if rid in seen:
            duplicated.append(rid)
        seen.add(rid)
        if rule.get("severity") not in allowed:
            bad.append(f"{rid}: severity={rule.get('severity')}")
        scope = rule.get("impact_scope")
        if not isinstance(scope, list) or not scope or not all(isinstance(s, str) and s for s in scope):
            bad.append(f"{rid}: impact_scope 非非空字符串列表")
        baseline = rule.get("baseline_failures")
        if not isinstance(baseline, int) or baseline < 0:
            bad.append(f"{rid}: baseline_failures 非非负整数")
        if not rule.get("exit_condition"):
            bad.append(f"{rid}: 缺 exit_condition")
        if not rule.get("owner"):
            bad.append(f"{rid}: 缺 owner")
    _check("known-red.yaml 每条风险含 severity/impact_scope/baseline_failures/exit_condition/owner",
           not bad, f"异常: {bad or 'none'}")
    _check("known-red.yaml rule_id 唯一", not duplicated, f"重复: {duplicated or 'none'}")


# ── 11f. referential-integrity.yaml — 引用完整性裁决结构 ─────────────────

def check_referential_integrity_registry():
    """referential-integrity.yaml：policy 四要素 + logical/pending 条目字段完整。"""
    data = _load_yaml("contracts/registries/referential-integrity.yaml")
    if data is None:
        return
    policy = data.get("policy") or {}
    missing = [key for key in ("uuid_identifier", "external_identifier", "naming_anomaly", "on_delete_policy")
               if not policy.get(key)]
    _check("referential-integrity.yaml policy 四要素完整", not missing, f"缺 {missing or 'none'}")
    bad = []
    for collection, required in (("logical_references", ("table", "column", "kind", "reason")),
                                 ("pending_physical_fks", ("table", "column", "target", "on_delete", "on_update", "open_item", "reason"))):
        for entry in data.get(collection) or []:
            if not isinstance(entry, dict):
                bad.append(f"{collection}: 非映射条目")
                continue
            lack = [field for field in required if not entry.get(field)]
            if lack:
                bad.append(f"{collection}:{entry.get('table')}.{entry.get('column')} 缺 {lack}")
    _check("referential-integrity.yaml 每条引用分类字段完整", not bad, f"异常: {bad or 'none'}")


# ── 12. 加密能力位：crypto.yaml ↔ CryptoSuite 常量与档位策略 ───────────────

def _go_capability_constants(source: str) -> dict[str, str]:
    """解析 `CapabilityXxx = "sm2_sign"` 形式的常量。"""
    return dict(re.findall(r'(Capability[A-Za-z0-9]+)\s*=\s"([a-z0-9_]+)"', source))


def _go_capability_list(source: str, name: str, constants: dict[str, str]) -> set[str]:
    """把 `gmRequiredCapabilities = []string{CapabilityA, CapabilityB}` 解析成 ID 集合。"""
    match = re.search(rf"{name}\s*=\s*\[\]string\{{(.*?)\}}", source, re.S)
    if not match:
        return set()
    referenced = re.findall(r"Capability[A-Za-z0-9]+", match.group(1))
    return {constants[item] for item in referenced if item in constants}


def _go_capability_sequence(source: str, name: str, constants: dict[str, str]) -> list[str]:
    """把 `registryCapabilityOrder = []string{...}` 解析成保持声明顺序的 ID 列表。"""
    match = re.search(rf"{name}\s*=\s*\[\]string\{{(.*?)\}}", source, re.S)
    if not match:
        return []
    referenced = re.findall(r"Capability[A-Za-z0-9]+", match.group(1))
    return [constants[item] for item in referenced if item in constants]


def check_crypto_registry():
    """crypto.yaml 是 CRYPTO_PROFILE 与能力位的唯一事实源。"""
    data = _load_yaml("contracts/registries/crypto.yaml")
    if not data:
        return

    profile = data.get("profile") or {}
    _check(
        "crypto.yaml profile.env=CRYPTO_PROFILE",
        profile.get("env") == "CRYPTO_PROFILE",
        f"env={profile.get('env')}",
    )
    _check(
        "crypto.yaml profile.values=[default, gm]",
        profile.get("values") == ["default", "gm"],
        f"values={profile.get('values')}",
    )
    _check(
        "crypto.yaml profile.default=default",
        profile.get("default") == "default",
        f"default={profile.get('default')}",
    )

    expected = {"sm2_sign", "sm2_encrypt", "sm3", "sm4", "gm_tls"}
    capabilities = {c.get("id"): c for c in (data.get("capabilities") or []) if c.get("id")}
    _check(
        "crypto.yaml 能力位完整",
        set(capabilities) == expected,
        f"capabilities={sorted(capabilities)}",
    )

    refuse_ids = {
        capability_id
        for capability_id, capability in capabilities.items()
        if capability.get("failure_mode") == "refuse"
    }
    degrade_ids = {
        capability_id
        for capability_id, capability in capabilities.items()
        if capability.get("failure_mode") == "degrade"
    }
    unknown_modes = sorted(
        capability_id
        for capability_id, capability in capabilities.items()
        if capability.get("failure_mode") not in {"refuse", "degrade"}
    )
    _check("crypto.yaml failure_mode 取值合法", not unknown_modes, f"异常={unknown_modes}")
    _check(
        "crypto.yaml refuse 集合",
        refuse_ids == {"sm2_sign", "sm2_encrypt", "sm3", "sm4"},
        f"refuse={sorted(refuse_ids)}",
    )
    _check(
        "crypto.yaml degrade 集合",
        degrade_ids == {"gm_tls"},
        f"degrade={sorted(degrade_ids)}",
    )

    # gm_tls 不允许退回「开启即可用」：probe 必须描述真实材料校验，
    # 所需 env 必须逐项登记，降级原因必须可读。
    env_data = _load_yaml("contracts/registries/env.yaml") or {}
    env_names = {
        var.get("name")
        for group in (env_data.get("groups") or {}).values()
        for var in (group.get("vars") or [])
        if var.get("name")
    }
    gm_tls = capabilities.get("gm_tls") or {}
    env_toggle = gm_tls.get("env_toggle")
    material_env = gm_tls.get("env_material") or []
    expected_material = {
        "GM_TLS_SIGN_CERT_FILE",
        "GM_TLS_SIGN_KEY_FILE",
        "GM_TLS_ENC_CERT_FILE",
        "GM_TLS_ENC_KEY_FILE",
        "GM_TLS_CA_FILE",
    }
    probe = str(gm_tls.get("probe") or "")
    self_attested_probes = {"", "GM_TLS_ENABLED", "GM_TLS_ENABLED=true"}
    _check(
        "crypto.yaml gm_tls env_toggle 已登记",
        env_toggle in env_names,
        f"env_toggle={env_toggle}",
    )
    _check(
        "crypto.yaml gm_tls 材料 env 集合",
        set(material_env) == expected_material,
        f"env_material={sorted(material_env)}",
    )
    _check(
        "crypto.yaml gm_tls 材料 env 均已在 env.yaml 登记",
        set(material_env) <= env_names,
        f"未登记={sorted(set(material_env) - env_names)}",
    )
    _check(
        "crypto.yaml gm_tls probe 是材料校验而非 env 开关",
        probe not in self_attested_probes and "证书" in probe and "CA" in probe,
        f"probe={probe}",
    )
    _check(
        "crypto.yaml gm_tls degrade_reason 已登记",
        bool(str(gm_tls.get("degrade_reason") or "").strip()),
        f"degrade_reason={gm_tls.get('degrade_reason')}",
    )

    # 能力位常量与派生清单已迁移到生成物（crypto.yaml 仍是唯一事实源）。
    # 本门禁按新消费路径校验，并断言 suite.go 不再保留手写副本、且真实消费生成物。
    generated = ROOT / "internal/platform/crypto/capabilities_gen.go"
    if not generated.exists():
        _check(
            "crypto.yaml 生成物存在",
            False,
            "internal/platform/crypto/capabilities_gen.go 不存在"
            "（运行 python3 scripts/gen_crypto_capabilities.py）",
        )
        return

    generated_source = generated.read_text()
    constants = _go_capability_constants(generated_source)
    _check(
        "crypto.yaml 能力位 ID ↔ 生成物 Capability 常量",
        set(constants.values()) == expected,
        f"go={sorted(constants.values())}",
    )
    required_go = _go_capability_list(generated_source, "gmRequiredCapabilities", constants)
    degradable_go = _go_capability_list(generated_source, "gmDegradableCapabilities", constants)
    _check(
        "crypto.yaml refuse ↔ gmRequiredCapabilities",
        required_go == refuse_ids,
        f"go={sorted(required_go)}",
    )
    _check(
        "crypto.yaml degrade ↔ gmDegradableCapabilities",
        degradable_go == degrade_ids,
        f"go={sorted(degradable_go)}",
    )

    declared_order = [
        capability.get("id")
        for capability in (data.get("capabilities") or [])
        if capability.get("id")
    ]
    order_go = _go_capability_sequence(generated_source, "registryCapabilityOrder", constants)
    _check(
        "crypto.yaml 声明顺序 ↔ registryCapabilityOrder",
        order_go == declared_order,
        f"go={order_go}",
    )

    suite = ROOT / "internal/platform/crypto/suite.go"
    if not suite.exists():
        _check("crypto.yaml 消费方存在", False, "internal/platform/crypto/suite.go 不存在")
        return
    suite_source = suite.read_text()
    handwritten = _go_capability_constants(suite_source)
    _check(
        "能力位常量只在生成物中声明（suite.go 无手写副本）",
        not handwritten,
        f"suite.go 手写常量={sorted(handwritten)}",
    )
    generated_lists = ("registryCapabilityOrder", "gmRequiredCapabilities", "gmDegradableCapabilities")
    missing_consumers = [name for name in generated_lists if name not in suite_source]
    _check(
        "suite.go 消费生成的能力位清单",
        not missing_consumers,
        f"未消费={missing_consumers}",
    )


# ── 13. --strict：注册表消费覆盖 ───────────────────────────────────────────

def check_password_change_exempt_registry():
    """password-change-exempt-routes.yaml ↔ 生成 Go 文件双向一致。

    真实路由是否存在由 Go 编译期与路由注册代码保证：生成物位于 auth 包内，
    路由字符串本身不在这里做跨文件猜测（/healthz 等运维路径不在 /api/v1 树中）。
    """
    data = _load_yaml("contracts/registries/password-change-exempt-routes.yaml")
    if not data:
        return

    registry_keys = set()
    for entry in data.get("routes") or []:
        method = str(entry.get("method") or "").strip().upper()
        path = str(entry.get("path") or "").strip()
        if not method or not path:
            _check("password-change-exempt 条目字段完整", False, f"entry={entry!r}")
            continue
        registry_keys.add(f"{method} {path}")

    _check("password-change-exempt 注册表非空", bool(registry_keys), f"entries={len(registry_keys)}")
    _check(
        "password-change-exempt 含 OPTIONS 说明记录",
        any(key.startswith("OPTIONS ") for key in registry_keys),
        "OPTIONS * 由中间件无条件放行，仍须在注册表留说明记录",
    )

    generated_path = ROOT / "internal/transport/http/api/v1/auth/password_change_exempt_gen.go"
    if not generated_path.exists():
        _check("password-change-exempt 生成物存在", False, f"missing {generated_path.relative_to(ROOT)}")
        return

    text = generated_path.read_text()
    _check(
        "password-change-exempt 生成物含 DO NOT EDIT 头",
        "Code generated by scripts/gen_password_change_exempt.py" in text,
        str(generated_path.relative_to(ROOT)),
    )
    generated_keys = set(
        re.findall(r'^\s*"([A-Z]+ [^"]+)":\s*\{\},$', text, re.MULTILINE)
    )
    missing = sorted(registry_keys - generated_keys)
    extra = sorted(generated_keys - registry_keys)
    _check(
        "password-change-exempt 注册表 ↔ 生成物 双向一致",
        not missing and not extra,
        f"missing={missing} extra={extra}",
    )


def check_registry_consumption():
    """--strict：REGISTRIES 登记的每个注册表都必须有门禁消费方"""
    if not STRICT:
        uncovered = [r for r in REGISTRIES if r not in CONSUMED and r not in DELEGATED_CONSUMERS]
        for name in uncovered:
            print(f"  WARN: 注册表 {name} 未被本门禁消费（--strict 下为 FAIL）")
        return

    gaps = []
    for name in REGISTRIES:
        if name in CONSUMED:
            continue
        delegate = DELEGATED_CONSUMERS.get(name)
        if not delegate:
            gaps.append(f"{name} 无消费方")
            continue
        delegate_path = ROOT / delegate
        if not delegate_path.exists():
            gaps.append(f"{name} 的消费方 {delegate} 不存在")
        elif name not in delegate_path.read_text():
            gaps.append(f"{name} 的消费方 {delegate} 未引用该文件名")
    _check(f"--strict 注册表消费覆盖（{len(REGISTRIES)} 个）",
           not gaps, f"缺口 {gaps or 'none'}")


# ── 11f. dr.yaml — RTO/RPO 机器可读口径 ──────────────────────────────────

def check_dr_registry():
    data = _load_yaml("contracts/registries/dr.yaml")
    if data is None:
        return
    tiers = data.get("tiers") or []
    required = {"L1", "L2", "L3"}
    ids = {str(tier.get("id")) for tier in tiers if isinstance(tier, dict)}
    _check("dr.yaml 覆盖 L1/L2/L3 三档", required <= ids, f"tiers={sorted(ids)}")
    bad = []
    for tier in tiers:
        if not isinstance(tier, dict):
            bad.append("tier not a dict")
            continue
        for key in ("scenario", "rto", "rpo", "mechanisms", "drill_frequency"):
            if not tier.get(key):
                bad.append(f"{tier.get('id')}: missing {key}")
    _check("dr.yaml 每档含场景/RTO/RPO/机制/演练频率", not bad, f"异常: {bad or 'none'}")
    _check("dr.yaml 要求人工签署", data.get("manual_signoff_required") is True,
           f"manual_signoff_required={data.get('manual_signoff_required')}")


# ── main ───────────────────────────────────────────────────────────────────

if __name__ == "__main__":
    print("=== check-registry-sync.py (YAML 版) ===")
    if STRICT:
        print("模式：--strict（注册表消费覆盖零缺口）")
    print()

    checks = [
        check_error_codes,
        check_encryption_keys_unique,
        check_outbox_params,
        check_state_machine_schema,
        check_open_items,
        check_env_vars,
        check_naming_gate,
        check_api_contracts,
        check_cache_registry,
        check_slo_registry,
        check_limits_registry,
        check_events_registry,
        check_dr_registry,
        check_lifecycle_registry,
        check_permissions_registry,
        check_known_red_registry,
        check_referential_integrity_registry,
        check_crypto_registry,
        check_password_change_exempt_registry,
        check_registry_consumption,
    ]

    for fn in checks:
        try:
            fn()
        except FileNotFoundError as e:
            print(f"FAIL: 必需输入文件缺失: {e}——输入缺失说明仓库结构被破坏，strict 模式下不能当作通过")
            PASS = False
        except Exception as e:
            print(f"ERROR: {fn.__name__} 崩溃: {e}")
            import traceback
            traceback.print_exc()
            PASS = False
        print()

    print(f"=== 结果: {'PASS' if PASS else 'FAIL'} ===")
    sys.exit(0 if PASS else 1)
