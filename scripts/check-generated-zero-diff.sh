#!/usr/bin/env bash
# 生成物零差异门禁

# 输入：db/generated/ 四方言、db/<dialect>/V1__*.sql、openapi/openapi.yaml、docs/generated/*.md、web/src/generated/*、.env.example、docs/traceability-matrix.md、web/src/app/theme/brand-palette.ts、internal/transport/http/api/v1/auth/password_change_exempt_gen.go
# 输出：PASS: 全部零差异；FAIL: 逐项打印 diff 原因 + 需运行的生成器命令
# --check 行为：无 --check；DIFFS>0 即 exit 1
# 消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
set -euo pipefail
ROOT="${1:-$(pwd)}"

# 各生成器的中间日志必须按进程隔离：固定 /tmp/*.log 会让并行会话（或并行门禁）互相覆盖
# 诊断内容，虽然 PASS/FAIL 取退出码不受影响，但失败时 tail 的可能不是自己的输出。
LOG_DIR="$(mktemp -d "${TMPDIR:-/tmp}/giam-zero-diff.XXXXXX")"
trap 'rm -rf "$LOG_DIR"' EXIT

echo "=== check-generated-zero-diff.sh ==="

DIFFS=0

# OpenAPI
# 生成器或产物缺失必须硬失败：此前条件写成「文件存在才校验」，生成器从 docs/ 迁到
# scripts/ 后条件恒假，整个零差异校验静默跳过——门禁报 PASS 但什么都没校验。
if [ ! -f "$ROOT/scripts/gen_openapi.py" ]; then
    echo "FAIL: 缺少 OpenAPI 生成器 scripts/gen_openapi.py（无法判定零差异）"
    DIFFS=$((DIFFS + 1))
elif [ ! -f "$ROOT/openapi/openapi.yaml" ]; then
    echo "FAIL: 缺少 OpenAPI 产物 openapi/openapi.yaml（运行 python3 scripts/gen_openapi.py）"
    DIFFS=$((DIFFS + 1))
else
    # --check compares without writing: the previous form passed `--out`, which the
    # generator ignored (it always wrote the default path), so the comparison file
    # never appeared and this block silently reported nothing.
    if python3 "$ROOT/scripts/gen_openapi.py" --check > "$LOG_DIR/openapi_check.log" 2>&1; then
        echo "PASS: openapi/openapi.yaml 与生成结果一致"
    else
        echo "FAIL: openapi/openapi.yaml 与生成结果不一致 — 运行 python3 scripts/gen_openapi.py"
        tail -3 "$LOG_DIR/openapi_check.log"
        DIFFS=$((DIFFS + 1))
    fi
fi

# 迁移链产物分两层（ADR-022）：
#   1) 冻结基线 db/postgresql/V1__*.sql 与 go:embed 副本 internal/bootstrap/schema_sql/V1__*.sql
#      必须逐字节一致（V1 由活动快照 history/active/V1 渲染，不随当前声明变化）；
#   2) 当前态渲染 db/generated/<dialect>/current.up.sql 与增量迁移 V2+ 由 gen_schema --check
#      统一校验零差异（本节只保留两副本一致 + 四方言基线存在性）。
DDL1="$ROOT/db/postgresql/V1__guliangiam_schema_init.sql"
DDL2="$ROOT/internal/bootstrap/schema_sql/V1__guliangiam_schema_init.sql"
if [ ! -f "$DDL1" ] || [ ! -f "$DDL2" ]; then
    echo "FAIL: DDL 两副本缺失（$DDL1 / $DDL2）"
    DIFFS=$((DIFFS + 1))
elif ! diff -q "$DDL1" "$DDL2" > /dev/null 2>&1; then
    echo "FAIL: DDL 两副本逐字节不一致"
    DIFFS=$((DIFFS + 1))
else
    echo "PASS: DDL 两副本一致"
fi

# 结构 V1 SQL 冻结基线存在性（四方言 — MySQL/V1/V3/KingbaseES/GaussDB 各有自己的链）
# V1 基线由 gen_schema.py 产出，四个方言的 V1 文件必须存在且与声明一致。
for dialect in postgresql mysql gaussdb kingbasees; do
    up="$ROOT/db/${dialect}/V1__guliangiam_schema_init.sql"
    if [ ! -f "$up" ]; then
        echo "FAIL: 缺少结构 V1 迁移 db/${dialect}/V1__guliangiam_schema_init.sql"
        DIFFS=$((DIFFS + 1))
    fi
done

# gen_schema.py --check（声明式 Schema → 生成物 逐字节可复现性，四方言）
# 覆盖：当前态 current.up.sql、冻结基线 V1、全部增量迁移 V2+ 与回滚、schema_types.{go,ts}。
# 上面的 V1 存在性 + 两副本一致是叠加的强约束。
if [ ! -f "$ROOT/scripts/gen_schema.py" ]; then
    echo "FAIL: 缺少声明式 Schema 生成器 scripts/gen_schema.py"
    DIFFS=$((DIFFS + 1))
else
    for dialect in postgresql mysql gaussdb kingbasees; do
        if python3 "$ROOT/scripts/gen_schema.py" --dialect "$dialect" --check > "$LOG_DIR/gen_schema_check.log" 2>&1; then
            echo "PASS: ${dialect} 声明式 Schema 与产物零差异"
        else
            echo "FAIL: ${dialect} 产物与 contracts/schema/*.yaml 不一致 — 运行 python3 scripts/gen_schema.py --all"
            tail -3 "$LOG_DIR/gen_schema_check.log"
            DIFFS=$((DIFFS + 1))
        fi
    done
fi

# 注册表 md 渲染层（`*-registry.md` + `security-readiness.md`，落点 docs/generated/）：
# YAML 是唯一事实源，md 是 gen_registry_md.py 生成的人类可读视图。
# 有人手改 md、YAML 改了没跑生成器、或生成器逻辑变更，这里都会拦截。
if [ ! -f "$ROOT/scripts/gen_registry_md.py" ]; then
    echo "FAIL: 缺少注册表渲染层生成器 scripts/gen_registry_md.py"
    DIFFS=$((DIFFS + 1))
else
    if python3 "$ROOT/scripts/gen_registry_md.py" --check; then
        : # 生成器自身已打印「全部 N 个注册表渲染层零差异」，此处不重复计数
    else
        echo "FAIL: 注册表 md 渲染层与 YAML 不一致 — 运行 python3 scripts/gen_registry_md.py"
        DIFFS=$((DIFFS + 1))
    fi
fi

# 控制台设计的菜单表与页面功能注册表（生成块）：必须等于重新生成的结果，否则新增
# 菜单项/页面只改了磁盘、没进文档，或有人手工编辑了生成块。
if [ ! -f "$ROOT/scripts/gen_frontend_registry.py" ]; then
    echo "FAIL: 缺少控制台注册表生成器 scripts/gen_frontend_registry.py"
    DIFFS=$((DIFFS + 1))
else
    if python3 "$ROOT/scripts/gen_frontend_registry.py" --check > "$LOG_DIR/frontend_registry_check.log" 2>&1; then
        echo "PASS: docs/design/frontend/registry.md 生成块（菜单表 + 页面功能注册表）零差异"
    else
        echo "FAIL: docs/design/frontend/registry.md 的生成块与生成结果不一致 — 运行 python3 scripts/gen_frontend_registry.py"
        tail -3 "$LOG_DIR/frontend_registry_check.log"
        DIFFS=$((DIFFS + 1))
    fi
fi

# 加密能力位生成物（contracts/registries/crypto.yaml → internal/platform/crypto/capabilities_gen.go）：
# YAML 是唯一事实源；手改生成物、或改了 crypto.yaml 没重生成，都在这里拦截。
# 生成器缺失/产物漂移均 fail-closed，避免「文件存在才校验」的静默跳过。
if [ ! -f "$ROOT/scripts/gen_crypto_capabilities.py" ]; then
    echo "FAIL: 缺少能力位生成器 scripts/gen_crypto_capabilities.py"
    DIFFS=$((DIFFS + 1))
elif python3 "$ROOT/scripts/gen_crypto_capabilities.py" --check > "$LOG_DIR/crypto_capabilities_check.log" 2>&1; then
    echo "PASS: internal/platform/crypto/capabilities_gen.go 与 contracts/registries/crypto.yaml 零差异"
else
    echo "FAIL: 能力位生成物与 crypto.yaml 不一致 — 运行 python3 scripts/gen_crypto_capabilities.py"
    tail -3 "$LOG_DIR/crypto_capabilities_check.log"
    DIFFS=$((DIFFS + 1))
fi

# ── 其余生成物：领域常量 / 前端错误码 / 前端契约 / .env.example / 追溯矩阵 ────
# 脚本头注释长期声称覆盖 web/src/generated/*、.env.example、docs/traceability-matrix.md，
# 但实现只跑了 OpenAPI/DDL/注册表/前端注册表——「头注释承诺、实现没做」比没有门禁更危险。
# 逐项 fail-closed：生成器缺失或产物漂移都计入 DIFFS。
run_generator_check() {
    local label="$1" script="$2"
    if [ ! -f "$script" ]; then
        echo "FAIL: 缺少生成器 ${script#$ROOT/}（无法判定 $label 零差异）"
        DIFFS=$((DIFFS + 1))
        return
    fi
    if python3 "$script" --check > "$LOG_DIR/gen_check.log" 2>&1; then
        echo "PASS: $label 生成物零差异"
    else
        echo "FAIL: $label 生成物与声明不一致 — 运行 python3 ${script#$ROOT/}"
        tail -3 "$LOG_DIR/gen_check.log"
        DIFFS=$((DIFFS + 1))
    fi
    rm -f "$LOG_DIR/gen_check.log"
}

run_generator_check "领域常量（Go + 前端状态机 TS）" "$ROOT/scripts/gen_domain_constants.py"
run_generator_check "前端错误码 TS" "$ROOT/scripts/gen_error_codes.py"
run_generator_check "前端契约 TS" "$ROOT/scripts/gen_api_contracts.py"
run_generator_check ".env.example" "$ROOT/scripts/gen_env_vars.py"
run_generator_check "追溯矩阵" "$ROOT/scripts/gen_traceability.py"
# 此前这两个生成器不在门禁内：删除契约注册表/改样式令牌后门禁仍 PASS（假绿）。
run_generator_check "强制改密豁免路由集（Go）" "$ROOT/scripts/gen_password_change_exempt.py"
run_generator_check "品牌色板镜像（antd 主题 TS）" "$ROOT/scripts/gen_brand_palette.py"
run_generator_check "OAuth 授权类型闭集（Go + 前端 TS）" "$ROOT/scripts/gen_grant_types.py"

# 生成物不得嵌入易变日期：日期跨日变化会让零差异门禁在没有任何内容变更时变红。
if grep -q '生成日期' "$ROOT/docs/traceability-matrix.md"; then
    echo "FAIL: docs/traceability-matrix.md 含易变「生成日期」——零差异门禁会跨日必红"
    DIFFS=$((DIFFS + 1))
fi

if [ $DIFFS -eq 0 ]; then
    echo "PASS: 全部零差异"
    exit 0
else
    echo "FAIL: $DIFFS 个不一致"
    exit 1
fi
