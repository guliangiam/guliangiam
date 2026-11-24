#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

# 前置工具检查：缺少工具时给出可执行指引，而不是让门禁以 exit 127 / traceback 崩溃。
missing_tools=()
for tool in python3 git grep; do
  command -v "$tool" >/dev/null 2>&1 || missing_tools+=("$tool")
done
if [ ${#missing_tools[@]} -gt 0 ]; then
  echo "缺少必需工具：${missing_tools[*]}（安装后重跑）" >&2
  exit 2
fi

# ── known-red 基线机制 ────────────────────────────────────────────────────
# contracts/registries/known-red.yaml 登记「已知红项」的 rule_id 与其基线失败数。
# 约定：门禁失败时，若该 rule_id 已登记且 baseline_failures > 0，则把**实测失败数**
# 与基线比较：actual > baseline 硬 FAIL（已知红项不是无限预算）；actual <= baseline
# 才降级为 WARN。未登记或基线为 0 的失败仍为硬 FAIL。基线只增不减；任何基线调整
# 须同步 CHANGELOG。
known_red_baseline() {
  python3 - "$1" <<'PY' 2>/dev/null || true
import sys, yaml
rid = sys.argv[1]
try:
    d = yaml.safe_load(open("contracts/registries/known-red.yaml", encoding="utf-8")) or {}
    for r in d.get("rules", []) or []:
        if r.get("rule_id") == rid:
            print(int(r.get("baseline_failures", 0)))
            break
except Exception:
    pass
PY
}

# 无法计数规则清单：`rule_id|理由`，一行一条。仅登记「输出不含稳定失败计数」的已知红项；
# 命中时 actual 取 1 并显式告警。未登记且解析不到计数的 rule 按「未验证失败」硬 FAIL，
# 防止有人用 baseline_failures=1 永久豁免一个从不输出计数的门禁。
UNCOUNTABLE_RULES=(
  # 示例格式：check-xxx|该门禁逐行打印命中项、无汇总计数，只能按 1 保守比较
)

uncountable_reason() {
  local rule_id="$1" entry
  for entry in ${UNCOUNTABLE_RULES[@]+"${UNCOUNTABLE_RULES[@]}"}; do
    if [[ "${entry%%|*}" == "$rule_id" ]]; then
      printf '%s' "${entry#*|}"
      return 0
    fi
  done
  return 1
}

# 从门禁输出解析实测失败数。优先使用显式计数（FAIL_COUNT / RESULT / === FAIL（N 处）），
# 其次汇总分类计数。解析不到或计数为 0 时返回非零，由 run_gate 走「无法计数」分支。
gate_failure_count() {
  python3 - "$1" <<'PY'
import re
import sys

text = open(sys.argv[1], encoding="utf-8", errors="replace").read()
text = re.sub(r"\x1b\[[0-9;]*m", "", text)
lines = text.splitlines()

def first_match(pattern, flags=0):
    for line in lines:
        found = re.search(pattern, line, flags)
        if found:
            value = int(found.group(1))
            if value > 0:
                return value
    return None

# 1) 显式标准输出：FAIL_COUNT=N
value = first_match(r"FAIL_COUNT\s*[=:]\s*(\d+)", re.IGNORECASE)
if value:
    print(value)
    sys.exit(0)

# 2) 汇总行：RESULT: 3 PASS / 7 FAIL 或 RESULT: 7 FAIL
value = first_match(r"RESULT:.*?(\d+)\s+FAIL", re.IGNORECASE)
if value:
    print(value)
    sys.exit(0)

# 3) 门禁汇总行：=== check-x.py: FAIL（N 处）===
value = first_match(r"^\s*(?:===|##).*?FAIL[（(]\s*(\d+)\s*[处项个類类问题致命]")
if value:
    print(value)
    sys.exit(0)

# 4) 分类计数求和：FAIL (2 致命) + FAIL (5 致命) → 7
category_counts = [
    int(m.group(1))
    for line in lines
    for m in [re.search(r"FAIL[（(]\s*(\d+)\s*(?:处|项|个|类问题|類問題|致命)", line)]
    if m
]
if category_counts and sum(category_counts) > 0:
    print(sum(category_counts))
    sys.exit(0)

# 5) 行内计数：FAIL: 发现 N 处 / 共 N 项（必须同句出现失败语义）
value = first_match(r"(?:FAIL|失败|问题).*?(?:共\s*)?(\d+)\s*[处项个]")
if value:
    print(value)
    sys.exit(0)

sys.exit(1)
PY
}

# run_gate <rule_id> <command...>：运行门禁；失败时按 known-red 基线决定 WARN 或 FAIL。
#
# 崩溃与「已知债务」必须分开：门禁自身 Traceback / panic / 命令不存在（exit 127）说明
# 它根本没有执行校验——此前 check-security-readiness.py（旧路径 FileNotFoundError）与
# check-http-methods.py（LEGACY_PATH NameError）都以退出码 1 被 known-red 当成普通
# 「已知红项」豁免，门禁实际上从未校验过任何东西。崩溃不是债务，禁止基线豁免。
run_gate() {
  local rule_id="$1"; shift
  # CI 去重：这 6 项由 backend / frontend / linter 专用作业执行；PR 的
  # Documentation gate 设置 SKIP_DUPLICATE_CODE_CHECKS=1 时跳过重复执行。
  # 默认（本地）仍全量执行，不改变任何判定逻辑。
  if [[ "${SKIP_DUPLICATE_CODE_CHECKS:-0}" == "1" ]]; then
    case "$rule_id" in
      go-build|go-vet|go-test|check-lint|web-frontend|check-bundle-budget)
        echo "[code checks] skip duplicate gate: $rule_id (owned by dedicated CI job)"
        return 0
        ;;
    esac
  fi
  local log status baseline actual reason
  log="$(mktemp)"
  set +e
  "$@" 2>&1 | tee "$log"
  status="${PIPESTATUS[0]}"
  set -e
  if [ "$status" -eq 0 ]; then
    rm -f "$log"
    return 0
  fi
  if [ "$status" -eq 127 ] || grep -qE 'Traceback \(most recent call last\)|^panic:|command not found' "$log"; then
    echo "FAIL: [$rule_id] 门禁自身崩溃/不可执行（exit $status）——崩溃不是 known-red 债务，必须修复；完整输出见上方" >&2
    rm -f "$log"
    exit 1
  fi
  baseline="$(known_red_baseline "$rule_id")"
  if [ -n "$baseline" ] && [ "$baseline" -gt 0 ]; then
    actual="$(gate_failure_count "$log" || true)"
    if [ -z "$actual" ]; then
      reason="$(uncountable_reason "$rule_id" || true)"
      if [ -n "$reason" ]; then
        echo "WARN: [$rule_id] 该 rule 无法计数，按 actual=1 保守比较（UNCOUNTABLE_RULES 理由：$reason）" >&2
        actual=1
      else
        echo "FAIL: [$rule_id] 该 rule 无法计数，且未登记 UNCOUNTABLE_RULES——无法证明失败数未超基线，阻断失败" >&2
        rm -f "$log"
        exit 1
      fi
    fi
    if [ "$actual" -gt "$baseline" ]; then
      echo "FAIL: [$rule_id] 失败数超出已知基线（实测 $actual > 基线 $baseline）——已知红项不是无限预算，必须收敛；出口条件见 contracts/registries/known-red.yaml" >&2
      rm -f "$log"
      exit 1
    fi
    echo "WARN: [$rule_id] 已登记为已知红项（实测 $actual / 基线 $baseline），本次非阻断；出口条件见 contracts/registries/known-red.yaml" >&2
    rm -f "$log"
    return 0
  fi
  echo "FAIL: [$rule_id] 未登记 known-red 基线，阻断失败" >&2
  rm -f "$log"
  exit 1
}

# gofmt / goimports 格式检查（供 run_gate 调用；未安装 goimports 时降级为仅 gofmt）。
_check_gofmt() {
  local unformatted
  unformatted="$(gofmt -l . | tr -d '[:space:]')"
  if [ -z "$unformatted" ] && command -v goimports >/dev/null 2>&1; then
    unformatted="$(goimports -l . | tr -d '[:space:]')"
  fi
  if [ -n "$unformatted" ]; then
    gofmt -l . >&2
    command -v goimports >/dev/null 2>&1 && goimports -l . >&2
    return 1
  fi
  return 0
}

search_stale_paths() {
  # docs/optimized/ 已退役（B 方案单轨收敛）：任何残留引用都失败，防止「第二宿主」复活。
  # scripts/gen_openapi.py 与 contracts/api/ 也是迁移后不应再出现的旧位置——生成器已从
  # docs/ 迁到 scripts/；OpenAPI 生成物按 ADR-012「生成物物理位置裁决」留在 openapi/，
  # contracts/ 只承载声明层。排除本脚本自身：它就是该模式的声明处，否则门禁永远匹配到自己。
  grep -RInE 'docs/(ARCHITECTURE|DEPLOYMENT|secret-rotation|performance-baseline|规范文件)\.md|docs/optimized/|docs/gen_openapi\.py|contracts/api/' \
    docs scripts openapi .github --exclude="verify-docs.sh" 2>/dev/null || true
}

# Drift patterns for values that have a single fact source in code/config. Each pattern
# must stay empty; a hit means the doc cites a path, image tag, or default that no longer
# matches the repository.
#
# The first alternative catches bare `sql/V1` / `sql/V2` (the pre-migration directory,
# no `__` suffix) as well as the obsolete `sql/V1__...` form. The leading
# `(^|[^A-Za-z0-9_])` guard is what keeps the correct `db/postgresql/V1__...` path from
# matching: the character before `sql` there is `l` (from "postgresql"), which is
# alphanumeric.
search_stale_values() {
  grep -RInE '(^|[^A-Za-z0-9_])(sql/V[12]__|sql/V[12]([^_A-Za-z0-9]|$)|V1__schema_init|V2__seed_init|golang:1\.2[0-9] AS |FROM alpine:3\.(1[0-9]|2[01])|AUTO_MIGRATE[^|]*\| *No *\| *false)' docs --exclude='*.py' || true
}

# 代码/CI 侧的迁移前旧路径：ADR-022 明确「不存在 db/migrations/」，但 CI 的 schema
# 重放循环、RLS 策略静态扫描与 dev 回滚检查脚本仍以 db/migrations/ 为输入——glob 不匹配
# 任何文件时这些步骤静默变成 no-op（CI 里 psql 还会直接失败）。本检查只扫 scripts/ 与
# .github/（docs/adr/ADR-022 的「不存在 db/migrations/」是规范表述，不是残留引用）。
search_stale_code_paths() {
  grep -RInE 'db/migrations/|docs/gen_openapi\.py|contracts/api/|db/schema/|docs/registries/[A-Za-z0-9_-]+\.yaml' \
    scripts .github --exclude="verify-docs.sh" 2>/dev/null || true
}

# docs/ 下的规范/设计/需求/运维/验收类文档只保存正文与当前状态（docs/README.md §4 规则 10）。
# 变更记录（版本表、修订历史、变更日志）唯一写入仓库根目录 CHANGELOG.md；此处拦截
# 「变更记录」章节标题、「| 版本 | 日期 |」式版本表，以及实际存在过的另一类载体——
# 以「收敛记录」「落地记录」「第 N 批」为标题的逐批叙事：它们不是变更日志的名字，
# 但作用完全一样，只拦标题词会全部漏过。
search_inline_changelog() {
  grep -RInE '^#+[[:space:]]*(变更记录|修订记录|版本历史|更新历史|变更历史|收敛记录|落地记录|Change ?Log|Revision History)[[:space:]]*$|^\|[[:space:]]*版本[[:space:]]*\|[[:space:]]*日期|^#+[[:space:]]*.*第[一二三四五六七八九十百零0-9]+批' docs --include='*.md' |
    grep -v '批准' |
    grep -vE '^docs/README\.md:[0-9]+:.*(禁止写入|不得写入|必须写作|规则 10)' || true
}

echo "[1/4] checking stale documentation paths"
if [[ -n "$(search_stale_paths)" ]]; then
  search_stale_paths
  echo "stale documentation path found" >&2
  exit 1
fi

echo "[1/4] checking stale code/CI paths"
if [[ -n "$(search_stale_code_paths)" ]]; then
  search_stale_code_paths
  echo "stale code/CI path found" >&2
  exit 1
fi

echo "[2/4] checking stale documented values"
if [[ -n "$(search_stale_values)" ]]; then
  search_stale_values
  echo "stale documented value found" >&2
  exit 1
fi

echo "[3/4] checking inline change records (must live in CHANGELOG.md only)"
if [[ -n "$(search_inline_changelog)" ]]; then
  search_inline_changelog
  echo "变更记录只允许写入仓库根目录 CHANGELOG.md；docs/ 下的规范/设计文档只保存正文与当前状态（README §4 规则 10）" >&2
  exit 1
fi

echo "[code checks] whitespace"
git diff --check -- docs scripts/verify-docs.sh

echo "[4/4] checking formal documentation files"
test -f docs/README.md
test -f docs/GuliangIAM-开发规范.md
test -f docs/acceptance-criteria.md
test -f docs/traceability-matrix.md
test -f docs/architecture/README.md
test -f docs/architecture/system-overview.md
test -f docs/architecture/api-surfaces.md
test -f docs/architecture/data-model.md
test -f docs/architecture/security-model.md
test -f docs/architecture/layers.md
test -f docs/architecture/observability.md
test -f docs/architecture/multi-region.md
test -f docs/architecture/technology-selection.md
test -f docs/design/README.md
test -f docs/design/frontend/README.md
test -f docs/requirements/functional.md
test -f docs/requirements/non-functional.md
test -f docs/operations/README.md
test -f docs/operations/deployment.md
test -f docs/registries/README.md
test -f docs/adr/README.md
test -f CHANGELOG.md
test -f SECURITY.md
test -f CONTRIBUTING.md

# 文档事实门禁：①docs/ 正文禁止三态状态标注（目标态规范，docs/adr/ 例外）；
# ②废弃规模数字面量不得复活（文档禁止手写计数）；
# ③手册文档版本与 CHANGELOG 沿革一致。文档引用事实只写「事实源路径 + 复现命令」。
echo "[4/4] checking documentation facts"
bash scripts/check-doc-facts.sh

# 文档红线门禁：批次/结项叙事与实施进度词逐行拦截；ADR 仅扫批次标记，
# `docs/generated/` 渲染层排除。上下文豁免见脚本头部说明。
echo "[4/4] checking documentation redlines"
python3 scripts/check-doc-redlines.py

# AC 需求锚门禁：需求编号断档/重复，以及 AC family 的需求前缀映射与逐条锚点。
echo "[4/4] checking acceptance criteria requirement anchors"
python3 scripts/check-doc-ac-anchors.py

# 悬空章节锚点门禁：`《文档名》N.M` 式引用必须命中目标文档真实存在的章节号。
# 章节被删除/重编号后引用会静默指向不存在的位置，读者无法回指，此前只能靠人工发现。
echo "[4/4] checking documentation section anchors"
python3 scripts/check-doc-anchors.py

# 相对链接门禁：事实源搬迁（db/schema→contracts/schema、docs/registries→contracts/registries、
# docs/gen_openapi.py→scripts/gen_openapi.py）后指向旧位置的 Markdown 链接会静默 404，
# 而章节锚点门禁看不到「链接目标文件不存在」这类失效。
echo "[4/4] checking documentation links"
python3 scripts/check-doc-links.py

# docs 根目录布局门禁：根目录只允许 4 份正式入口文档；过程报告/审计快照
# 必须写入 .trae/documents/，否则 B1 收尾后的根目录会再次被新增报告污染。
echo "[4/4] checking docs root layout"
python3 scripts/check-docs-root.py

# 数据生命周期覆盖：命中 deleted_at / expires_at / 追加型命名的表必须被生命周期注册表
# 显式覆盖（保留/归档/清理策略），否则「数据全生命周期完整性」只有注册表而没有覆盖。
echo "[4/4] checking data lifecycle coverage"
python3 scripts/check-lifecycle-coverage.py

# 门禁接线覆盖：每个 scripts/check-* 必须被 verify-docs.sh / CI / Makefile 引用，
# 文档中出现的脚本路径必须真实存在。此前有 7 个孤儿门禁（含 RLS 结构门禁）从未被调度，
# 文档还引用过 3 个不存在的门禁脚本名——「文档承诺、CI 不跑」是比没有门禁更危险的状态。
echo "[4/4] checking gate wiring coverage"
python3 scripts/check-gate-coverage.py

if [[ "${RUN_CODE_CHECKS:-0}" == "1" ]]; then
  echo "[code checks] running repository quality commands"

  # Layering is still above its historic baseline (see ADR-005 / 开发规范附录 I).
  # Run it for evidence; baseline failures are tracked via known-red.yaml.
  echo "[code checks] layering (informational, non-blocking)"
  run_gate check-layers bash scripts/check-layers.sh
  # SQL 字面量只允许在 internal/infra/**。domain/records 曾持有 6 条审计链 SQL
  # 常量，而 check-layers 只查 import、不扫字符串，门禁绿但事实违规。本门禁
  # 扫描 app/domain/platform/transport 的 Go 字符串字面量（排除注释与测试）。
  run_gate check-sql-location python3 scripts/check-sql-location.py

  # 迁移链完整性：结构迁移与回滚脚本一一配对、版本号连续、结构/种子类别纯净、
  # 种子幂等、双副本一致、账本单一写入者。此前种子脚本放在 db/postgresql/ 并占用
  # 结构版本号 V2，任何把所有 V{n}__*.sql 一律当结构迁移的跑法都会把种子注入生产；
  # 运行时也没有迁移账本，无法回答"该库处于哪个结构版本"（实际入口为 cmd/migrate）。
  run_gate check-migrations bash scripts/check-migrations.sh
  run_gate check-db-conventions bash scripts/check-db-conventions.sh
  # 默认值的方言可移植性：TEXT 携带默认值在 MySQL 不可表达，只减不增；
  # LONGTEXT 显式「无上界」故禁止默认值（OPEN-API-12）。
  run_gate check-schema-defaults python3 scripts/check-schema-defaults.py
  # 命名唯一裁决：禁止字面量扫描（散落的 `tenant_id`/`org_id`、大小写变体、缩写）。
  run_gate check-naming bash scripts/check-naming.sh
  # 注册表一致性：错误码、唯一键、DEFAULT 值等可枚举事实必须与 contracts/registries/ 一致。
  # --strict 追加注册表消费覆盖：contracts/registries/ 下每个 YAML 都必须有门禁消费方
  # （本脚本解析，或 DELEGATED_CONSUMERS 登记的消费方脚本真实引用该文件名）。
  run_gate check-registry-sync python3 scripts/check-registry-sync.py --strict
  # 三面分类注册表完整性：surfaces.yaml 的显式端点/公开豁免/家族与 gate 引用自洽，
  # 且公开豁免不得命中管理面（AC-ARCH-003/004）。
  run_gate check-surface-registry python3 scripts/check-surface-registry.py
  # 短期状态矩阵登记完整性：每条须登记 存储/多区域策略/失败姿态/清理（AC 目标态）。
  run_gate check-short-lived-state python3 scripts/check-short-lived-state.py
  # 缓存注册表 ↔ 代码实现一致性：失效策略/租户键前缀/四条红线必须与 cache.yaml 渲染层一致。
  # 该门禁在 docs/README.md §2.4 被登记为治理工具，却从未接入 verify-docs——文档承诺与
  # 实际调度不一致，等于没有门禁。
  run_gate check-cache-registry bash scripts/check-cache-registry.sh
  # DDL 必需表清单（schema.sh 是粗粒度存在性检查，等价性由 check-schema-parity.py 强制）。
  run_gate check-schema bash scripts/check-schema.sh
  # 目标域表完整性：ADR-024~029 的 16 张目标域表必须同时存在于声明式 Schema 与生成 SQL。
  run_gate check-target-schema bash scripts/check-target-schema.sh
  # RLS 强制隔离（ADR-016 / 安全就绪度 S12）：含 organization_id 的租户表必须启用 RLS，
  # 生成 SQL 必须 ENABLE + FORCE，应用角色不得是 owner。该门禁是租户隔离的唯一结构防线，
  # 此前只在文档里被登记为 CI 门禁却从未被调度；且规则用行正则只认 `rls: true`，
  # 映射写法 `rls: {force: true}` 会被误报（AI 网关 8 张表即属此类）——已改为 YAML 语义解析。
  run_gate check-rls bash scripts/check-rls.sh
  # 租户隔离分类：每张表必须 direct/indirect/self_scoped/platform_scoped/global 之一，
  # global 仅允许 tenant-isolation.yaml 登记项；新表未归类即失败。
  run_gate check-tenant-isolation python3 scripts/check-tenant-isolation.py
  # 创建/退役组织的租户接线：CreateOrganization 必须在同一事务写身份与主拓扑，
  # DeleteOrganization 必须状态流转 + 软删；防止回退成「只建组织不建租户」。
  run_gate check-tenant-provisioning python3 scripts/check-tenant-provisioning.py
  # 表间引用完整性：UUID *_id 列必须物理外键，或在 referential-integrity.yaml 分类
  # （logical_references 永久逻辑引用 / pending_physical_fks 待补并指向开放项）。没有这道
  # 门禁，删掉一个 fk_constraints 或新增 *_id 列忘记外键都不会被任何检查发现。
  run_gate check-fk-coverage python3 scripts/check-fk-coverage.py
  # 声明式 Schema ↔ 产物 DDL 的结构等价性（表/列/主键/约束/索引/RLS/扩展/函数/触发器）。
  # gen_schema.py --check 只证明「生成器可复现」，无法发现有人手工改产物或生成器漏对象；
  # check-sql-columns.py 只反向校验 Go 仓储的列引用。两者之间的缺口由此门禁补齐。
  # 生成器可复现性必须覆盖**全部方言**：此前只校验 postgresql，于是 GaussDB /
  # KingBaseES / MySQL 三个产物自加入声明式 Schema 后从未被重新生成，表名变形、
  # 列改名、RLS 缺失等 145 项漂移长期存活而无门禁可发现（与 check-schema-parity.py
  # 互补：后者比对「产物 ↔ 声明」，前者证明「生成器能逐字节重放产物」）。
  for dialect in postgresql mysql gaussdb kingbasees; do
    run_gate gen_schema python3 scripts/gen_schema.py --dialect "$dialect" --check
  done
  run_gate check-schema-parity python3 scripts/check-schema-parity.py
  # ADR-022 命名门禁（PostgreSQL 方言）：contracts/schema/*.yaml 与 DDL 生成物必须逐字节
  # 同源。它与上面的 gen_schema --dialect postgresql --check 同源，但保留独立入口，
  # 防止方言循环被删改后 PostgreSQL 侧失去校验。
  run_gate check-schema-consistency bash scripts/check-schema-consistency.sh
  # Schema 演化增量审计：seed 值长度 ↔ 列类型、CHECK/predicate cast、动态 SQL 风险。
  # 此前从未被任何调度入口执行（实测 3 PASS / 7 FAIL）。7 项中 [C-1] 是 INSERT…SELECT
  # 跨语句误匹配 VALUES、[E-2]/[E-4] 是错误文案被当作 SQL 拼接的误报，尚未修复；
  # 按 known-red 基线 7 记账（actual > baseline 硬失败），出口条件见注册表。
  run_gate check-schema-evolution-audit python3 scripts/check-schema-evolution-audit.py
  # 索引经济性（D9）：普通索引列被唯一约束最左前缀覆盖即冗余。
  run_gate check-redundant-index python3 scripts/check-redundant-index.py
  run_gate check-sql-portability bash scripts/check-sql-portability.sh
  # SQL↔DDL 列漂移：仓储 SQL 字面量引用的表/列必须在 V1 DDL 中存在。这是集成测试
  # 之外的唯一防线——`//go:build integration` 的用例默认不跑，列改名会静默变成
 # 运行时错误（详见 13.1 风险条目）。
  run_gate check-sql-columns python3 scripts/check-sql-columns.py
  # 语句级契约：静态检查看不到类型不匹配、未使用参数序号、多表 UPDATE 同名列歧义与
  # 指向不存在表的仓储；PREPARE 能在有 DSN 时一次覆盖这四类（无 DSN 时跳过并告警）。
  run_gate check-sql-runtime python3 scripts/check-sql-runtime.py
  # 调用点参数个数：pgx 在运行期拒绝 `wrong number of parameters`，而 PREPARE 看不到
  # 调用点——语句绑三个参数、调用只传两个，PREPARE 完全通过，首个请求才失败。给
  # adapters/models 补 is_enabled 谓词时只改了 SQL，两个列表接口因此每次请求必败，
  # 而当时 15 项门禁全绿。
  run_gate check-sql-args python3 scripts/check-sql-args.py
  run_gate check-scan-columns python3 scripts/check-scan-columns.py
  # goroutine 安全：长期运行的后台 goroutine 必须由 recover.SafeGo 包裹，否则一次 panic
  # 会击穿整个进程（Worker/cleanup loop/refresh loop 均属长期运行）。
  run_gate check-goroutine-safety bash scripts/check-goroutine-safety.sh
  # 前后端查询参数契约：web 发出的每个查询参数必须真的被某个 handler 读取。此前
  # 列表页一律发 `search`/`sortBy`，而后端只读 `keyword`/`sortField`，搜索与排序
  # 全部静默失效。
  run_gate check-api-params python3 scripts/check-api-params.py
  # 错误体必须走统一写入器（管理/认证面 route.WriteError，协议面 WriteOAuthError /
  # WriteSCIMError）。手写 {"error":…} 会丢失 requestId/traceId，并发明附录 C 之外
  # 的错误码（i18n handler 曾因此写 `bad_request`）。
  run_gate check-errors python3 scripts/check-errors.py
  # 语义失败不得回 400：可解析可绑定但违反规则的请求（格式/枚举闭集/跨字段/
  # 状态机/越界）必须 422 + meta，客户端才能区分「重试无用」与「改字段即可成功」
  # （手册 4.14.1）。尚未转换的站点按批次登记在脚本 PENDING 中，只增不减。
  run_gate check-error-semantics python3 scripts/check-error-semantics.py
  # 手写错误响应门禁：禁止 handler 直接 w.WriteHeader(4xx/5xx)+w.Write 绕过写入器。
  # check-errors.py 只看 route.WriteJSON 绕过路径，本门禁补「裸 WriteHeader」路径。
  run_gate check-error-response bash scripts/check-error-response.sh
  # 数据库约束错误映射覆盖率：只报告仍裸返回驱动错误的仓储写路径，基线由
  # `scripts/check-db-error-mapping.py --write-baseline` 生成。本批只接线四个域，
  # 剩余域按基线收敛；该项只报告，`|| true` 保证不阻断 CI。
  python3 scripts/check-db-error-mapping.py || true
  # 结构化日志红线（E3-b）：日志 key 必须 snake_case，禁止敏感 key 与完整请求体 key。
  # 当前全仓基线仍有 157 条存量 violations（口径见 scripts/check-logs.py 头部说明），
  # 未清零前只报告不阻塞；基线清零后把本行改为 run_gate 硬失败。本批新增行为的回归
  # 由 internal/platform/logging/logging_test.go 与 access_log_test.go 承担。
  python3 scripts/check-logs.py || true
  # 安全就绪度矩阵：contracts/registries/security-readiness.yaml 里标记「已实现」的能力
  # 必须带一个在仓库里真实存在的验收锚点，且 status 值不得拼写漂移。该注册表此前
  # 无任何门禁消费，「标记 = 实现」无人核对（docs/README.md §3.1 注册表治理「安全能力就绪度」行）。
  run_gate check-security-readiness python3 scripts/check-security-readiness.py
  # 读接口凭据不得回显明文：读侧结构体的凭据字段必须配 `XxxSet` + `XxxMasked`，
  # 否则要在脚本内 ALLOWLIST 里写明理由。faceidproviders / captchaproviders /
  # adapters / radiusconfigs 四个详情接口曾把 accessKeySecret / secretKey /
  # dbPassword / sharedSecret 原样返回。
  run_gate check-credential-masking python3 scripts/check-credential-masking.py
  # 加密列注册表 ↔ 仓储 mapper ↔ 声明式 Schema：回填列必须在轮换注册表内，
  # 每个注册列必须在 Schema 中真实存在，providers/notificationproviders 的
  # PG/MySQL mapper 必须仍引用 NewEncryptedFieldMapper。此前组织敏感列登记在
  # 错误的表名/PK 下，轮换作业会扫描一个没有这些列的表。
  run_gate check-encrypted-columns python3 scripts/check-encrypted-columns.py
  # 枚举闭集跨语言一致性：服务端的 enumset.NewSet 与控制台的 @enumset 清单必须
  # 逐值一致。此前控制台发 `Default`/`reCAPTCHA`/`Turnstile` 而工厂注册的是
  # `default`/`recaptcha`/`cloudflare`，六个值无一匹配，控制台创建的验证码
  # provider 全部不可用（生成验证码时 500）。
  run_gate check-enums python3 scripts/check-enums.py
  # Provider kind 路由完整性：identityKinds、provider-kinds.yaml 与运行时 pending
  # 路由必须逐值一致；routable=false 的 kind 必须带子系统批次，避免回到「声明即可用」。
  run_gate check-provider-kinds python3 scripts/check-provider-kinds.py
  # HTTP 方法与路径契约：控制台发出的每个请求必须命中一条已声明的路由。方法不匹配
  # 不是软失败——路由没有该动词的处理器，请求 404 而页面永远空白。MCP 应用商店曾
  # POST 而处理器注册 GET，页面因此从不可加载。
  run_gate check-http-methods python3 scripts/check-http-methods.py
  # 自检：paths.ts 解析、动词不匹配与覆盖率下限三类样本必须按预期返回 1/0。
  # 没有这一步，门禁自身退化（例如重新只解析字面量）时仍可能安静通过。
  run_gate check-http-methods-self-test python3 scripts/check-http-methods.py --self-test
  # 审计主体类型：`records.actor_id` 是 uuid 列，而 21 处审计调用点曾把 user.Username
  # 当主体传入——用户名不是 UUID，INSERT 报 invalid input syntax，审计与业务写同事务，
  # 于是「改资料/换 MFA/删凭证」整条链一起回滚成 500。编译、评审、单测都看不见，
  # 因为值只有在落库那一刻才非法。
  run_gate check-audit-actor python3 scripts/check-audit-actor.py
  # UI 接线：带筛选状态的页面必须把值交给请求；传 onSortChange 的页面必须有可排序列。
  # 11 个页面持着 filterValues 却从不转发（另外 12 个同组件同 prop 的兄弟页面转发
  # 了），7 个页面接了排序回调却没有任何可排序列——两者都是「控件看着能用、实际无效」。
  run_gate check-ui-wiring python3 scripts/check-ui-wiring.py
  # 前端请求体键 ↔ Go json tag：`route.DecodeJSON` 使用 DisallowUnknownFields，
  # 前端按另一套字段名发送时不是「忽略」，而是整条写路径 400。tsc 只校验页面
  # 字面量与前端 payload 类型，看不到 Go struct，因此这里对显式登记的
  # (TS 类型, Go DTO) 对做静态比对：前端多出的键即失败。
  run_gate check-payload-contract python3 scripts/check-payload-contract.py
  # 详情页宽列未声明表格横向滚动：`ResourceListCard` 给列表页统一加了
  # `scroll={{ x: 1600 }}`，详情页/tab 直接渲染的 Table 没有这层保护，列宽一旦
  # 显式声明，窄视口下会把页面容器撑出横向滚动条。tsc、lint 与 mock 了 HTTP 的
  # 组件测试都看不见，只在该视口的布局阶段出现。
  run_gate check-table-overflow python3 scripts/check-table-overflow.py
  # Descriptions 固定多列/固定跨列：`column` 与 `Item span` 必须成对响应式，否则
  # 窄视口降为单列后 span=2 会越界错行；tsc、lint 与组件测试都看不见，只在渲染时暴露。
  run_gate check-descriptions-responsive python3 scripts/check-descriptions-responsive.py
  # 写路径 → 列表缓存同步：删除 mutation 与创建页 onSuccess 必须失效查询缓存，
  # 否则返回列表仍显示已删除行 / 看不到新建行（tsc、lint 与组件测试都看不见）。
  run_gate check-write-invalidation python3 scripts/check-write-invalidation.py
  # 主显示区宽度：控制台页面只能使用 .gl-content__inner 三档语义类，禁止页面级
  # max-width/第二套容器（否则同一壳层下页面宽度跳变，窄视口样式分叉）。
  run_gate check-content-width python3 scripts/check-content-width.py
  # 子菜单卡片化：一个标签页/分区只允许一层卡片，禁止 TabCard / gl-section-anchor /
  # gl-tabs-card 内再嵌套 Card（否则出现双层堆叠，组件测试与 tsc 都看不见）。
  run_gate check-card-patterns python3 scripts/check-card-patterns.py
  # 前端架构门禁：所有 HTTP 请求必须走 shared/api/http.ts 统一入口，禁止直调
  # fetch/XMLHttpRequest/axios（否则绕过 credentials/CSRF/Accept-Language/错误映射/401 登出）。
  run_gate check-frontend-architecture python3 scripts/check-frontend-architecture.py
  # API 路径唯一事实源：web/src 生产代码禁止硬编码 `/api/v1/...` 字面量，必须从
  # shared/api/paths.ts 取常量。此前该门禁存在但从未被调度，硬编码回归无人拦截。
  run_gate check-paths-consistency python3 scripts/check-paths-consistency.py
  # 前端权限 key 必须能在 contracts/registries/permissions.yaml 中找到。未登记的
  # key 不会报错也不会回 403，PermissionGuard 只是静默隐藏控件——构建、lint 与
  # 组件测试全绿。动态构造逐条列入「不可判定清单」，不静默放过也不误杀。
  run_gate check-permission-keys python3 scripts/check-permission-keys.py
  # 路由 Permission 声明 vs 运行期 resourceFromPath 派生。中间件根本不读声明，
  # 只按路径首段裁决，因此声明可以长期静默漂移：`/users/{id}/orders` 声明
  # `orders:read`，实际却由 `users:read` 放行。warn 阶段只阻断本次变更集命中的
  # R1/R2 失配；嵌套高危（首段后的 {var} + 声明资源 ≠ 派生资源）永不可豁免，
  # 始终阻断。运行时函数指纹内嵌在脚本里，Go 侧改动未同步会直接判红。
  run_gate check-route-resource-derivation python3 scripts/check-route-resource-derivation.py
  # 设计令牌一致性：`styles.css` 的 `--gl-*` 颜色令牌与 antd 主题必须用的 JS 镜像
  # （`brand-palette.ts`）是同一事实的两处表述。antd 无法读取 CSS 变量，所以镜像
  # 必然存在；两份手写值一旦漂移，CSS 照常渲染而 antd 半侧悄悄换色，可访问性断言
  # 也只剩一半生效。
  run_gate check-token-parity python3 scripts/check-token-parity.py
  # i18n 键完整性：i18next 找不到键时不报错，把键名当文案显示，所以「未注册的命名空间」
  # 与「从错误命名空间取键」都能一路通过 tsc / oxlint / 组件测试。`/device` 整页曾因
  # `useTranslation("device")` 而显示 `deviceAuth`、`authorizeDevice`；另有页面从
  # `common.auth.*` 与 `auth.json` 两棵互相重叠的键树中取键。本门禁失败即阻断，键解析
  # 存量以基线文件记账（只减不增），语言包键全量一致为硬性条件。
  run_gate check-i18n-keys python3 scripts/check-i18n-keys.py
  # Go 格式：历史上没有任何 CI 门禁强制过 `gofmt`（唯一包含该检查的工作流挂在
  # `master` 分支上，而本仓库唯一分支是 main），全仓一度有三分之一的文件未格式化。
  # 格式不是审美问题——未格式化的代码会让后续每一个 diff 混入无关行，评审与追溯
  # 都被噪声掩盖。`goimports` 一并检查（手册 1.1.4 要求标准库 → 第三方 → 项目内分组），
  # 未安装时降级为仅 gofmt，不让门禁因缺工具而红灯。
  run_gate check-gofmt _check_gofmt
  run_gate go-build go build ./...
  run_gate go-vet go vet ./...
  run_gate go-test go test ./...
  # 静态分析：golangci-lint（配置为 v2 语法）。--strict 只阻断本次改动引入的新问题；
  # 存量以提示形式输出，避免"永久红"的假门禁。.golangci.yml 曾长期停留在 v1 语法而 CI
  # 用 version: latest，配置根本无法加载，等于从未做过静态分析。
  run_gate check-lint bash scripts/check-lint.sh --strict
  # OpenAPI 门禁：生成器必须确定性运行，且 openapi/openapi.yaml 必须已是生成结果。
  # 与 git HEAD 比较在工作区未提交时不可靠（规范文件或生成器本身尚未提交都会报差异），
  # 因此生成器自带的 --check 只比「生成结果 ↔ 磁盘文件」——这才是「文件已随路由更新」
  # 的真实判定。
  run_gate gen_openapi python3 scripts/gen_openapi.py --check
  run_gate check-openapi-contract bash scripts/check-openapi-contract.sh
  # 生成物零差异：DDL 双副本、菜单表与页面功能注册表（《控制台设计》生成块）、OpenAPI 等所有生成物必须与生成器一致。
  run_gate check-generated-zero-diff bash scripts/check-generated-zero-diff.sh
  # AC ↔ 验收标准双向追溯：需求编号不得有「无 AC 的需求」或「无需求的 AC」。
  run_gate check-ac-traceability python3 scripts/check-ac-traceability.py
  # 前端覆盖闭合：路由 ↔ 页面 ↔ 菜单 ↔ 《控制台设计》菜单表四者必须互相闭合，且没有孤岛页面。
  run_gate check-frontend-coverage python3 scripts/check-frontend-coverage.py
  # 数据流闭环：写路径的事务边界、审计同事务、outbox 投递规则。
  run_gate check-data-flow bash scripts/check-data-flow.sh
  # 路由清单契约：openapi/openapi.yaml 的 operationId 集合必须与已声明路由一致。
  run_gate check-api-contract bash scripts/check-api-contract.sh
  # format:check is part of the CI frontend job; running it here keeps the local
  # gate at parity with CI, which otherwise lets a formatting regression through
  # until the pull request is opened.
  run_gate web-frontend bash -c 'cd web && pnpm run lint && pnpm run format:check && pnpm run build && pnpm run test -- --run'
  # 首屏预算：`dist/assets/index-*.js` 只有 113KB，而 index.html 真正 preload 的
  # 资源一度是 1719KB——`manualChunks` 给「只有 lazy route 能到达」的 Scalar 子图
  # 命名后，rolldown 把整组提回入口静态图（1025KB，占首屏 60%）。门禁改为量
  # index.html 引用的全部资源，并要求每个首屏 vendor chunk 都在脚本里声明过理由，
  # 因此「不该在首屏的东西被提上来」会指名道姓地失败。需先构建（上一行已构建）。
  run_gate check-bundle-budget python3 scripts/check-bundle-budget.py
  # 首屏防回归：app shell/entry 必须深导入 antd 组件，禁止 barrel 把整包拖入入口静态图。
  run_gate check-app-barrel-imports python3 scripts/check-app-barrel-imports.py
else
  echo "code checks skipped; run RUN_CODE_CHECKS=1 bash scripts/verify-docs.sh to execute them"
fi