#!/usr/bin/env bash
# ────────────────────────────────────────────────────────────────────────────
# check-goroutine-safety.sh — goroutine panic recovery 门禁
#
# 机制：
#   1. `scripts/tests/testdata/goroutine_scan.go` 用 Go AST 找出 internal/ 下
#      每个 `go` 语句（`go func(...)`、`go f(...)`、`go recv.m(...)`、
#      `go pkg.F(...)` 一视同仁），输出 file:line:col、形态与源码行。
#   2. 排除 `_test.go`、`testdata/`、`vendor/` 与 SafeGo 自身实现。
#   3. 未命中豁免清单的 `go` 语句一律违规——长期运行的 Worker / cleanup /
#      refresh / scheduler goroutine 必须用 recover.SafeGo /
#      SafeGoWithLogger 包裹，否则一次 panic 会击穿整个进程。
#
# 豁免（一次性/短生命周期，panic 不击穿系统级安全；逐条写明理由）：
#   - internal/platform/recover/recover.go      —— SafeGo 自身实现
#   - internal/app/auth/cas.go                  —— CAS PGT URL 一次性回调
#   - internal/app/mcpservers/intranet_scan.go  —— 一次性并发扫描
#   - internal/platform/workpool/pool.go:33/80-99 —— 固定 worker 池与 Shutdown wg.Wait helper
#   - internal/transport/http/api/v1/batch/handler.go —— 单请求内 WaitGroup 并发批处理
#
# 否定测试：`bash scripts/check-goroutine-safety.sh --self-test`
#   对 scripts/tests/testdata/goroutine_cases.go.txt 断言：
#   `go recv.m()` / `go pkg.F()` / `go func()` / `go plain()` 共 4 处被检出；
#   `defer`、字符串、原始字符串、注释里的 go 不被检出。fixture 扩展名为
#   `.go.txt`，不进入 `go build ./...`。
#
# 退出码：0=全通过，1=发现未包裹 goroutine，2=扫描器/解析失败
# ────────────────────────────────────────────────────────────────────────────

# 输入：internal/**/*.go（AST 解析；不含 _test.go / testdata / vendor）
# 输出：PASS/FAIL + 未包裹 goroutine 的 file:line:col 与源码行
# --check 行为：无 --check；失败即 exit 1；--self-test 只跑负向 fixture
# 消费方：scripts/verify-docs.sh（run_gate check-goroutine-safety）
set -euo pipefail

ROOT="${ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"
cd "$ROOT"

SCANNER="scripts/tests/testdata/goroutine_scan.go"
FIXTURE="scripts/tests/testdata/goroutine_cases.go.txt"

command -v go >/dev/null 2>&1 || {
    echo "FAIL: 缺少 go 工具链，无法运行 AST 扫描器" >&2
    exit 2
}

# 从 AST 扫描输出中提取 file/line；豁免判定只看路径与行号。
is_exempt() {
    local file="$1" line="$2"
    case "$file:$line" in
        internal/platform/recover/recover.go:*)                   return 0 ;;
        internal/app/auth/cas.go:*)                                return 0 ;;
        internal/app/mcpservers/intranet_scan.go:*)                return 0 ;;
        internal/platform/workpool/pool.go:33)                     return 0 ;;
        internal/platform/workpool/pool.go:8[0-9]|internal/platform/workpool/pool.go:9*) return 0 ;;
        internal/transport/http/api/v1/batch/handler.go:*)         return 0 ;;
    esac
    return 1
}

# collect_violations <dir-or-file>：填充全局 TOTAL / EXEMPTED / VIOLATIONS。
collect_violations() {
    local target="$1" findings loc kind text file rest line
    findings="$(go run "$SCANNER" "$target")"
    VIOLATIONS=()
    TOTAL=0
    EXEMPTED=0
    while IFS=$'\t' read -r loc kind text; do
        [ -z "$loc" ] && continue
        TOTAL=$((TOTAL + 1))
        file="${loc%%:*}"
        rest="${loc#*:}"
        line="${rest%%:*}"
        if is_exempt "$file" "$line"; then
            EXEMPTED=$((EXEMPTED + 1))
            continue
        fi
        VIOLATIONS+=("$loc	$kind	$text")
    done <<< "$findings"
}

run_self_test() {
    local findings count selectors funcs calls old_count
    findings="$(go run "$SCANNER" "$FIXTURE")"
    count="$(printf '%s\n' "$findings" | grep -c . || true)"
    selectors="$(printf '%s\n' "$findings" | awk -F'\t' '$2 == "selector"' | wc -l | tr -d ' ')"
    funcs="$(printf '%s\n' "$findings" | awk -F'\t' '$2 == "func"' | wc -l | tr -d ' ')"
    calls="$(printf '%s\n' "$findings" | awk -F'\t' '$2 == "call"' | wc -l | tr -d ' ')"

    if [ "$count" -ne 4 ] || [ "$selectors" -ne 2 ] || [ "$funcs" -ne 1 ] || [ "$calls" -ne 1 ]; then
        echo "SELFTEST FAIL: 期望 4 处（selector=2/func=1/call=1），实际 count=$count selector=$selectors func=$funcs call=$calls" >&2
        printf '%s\n' "$findings" >&2
        exit 1
    fi

    collect_violations "$FIXTURE"
    if [ "$TOTAL" -ne 4 ] || [ "$EXEMPTED" -ne 0 ] || [ "${#VIOLATIONS[@]}" -ne 4 ]; then
        echo "SELFTEST FAIL: 负向 fixture 应产生 4 个未豁免违规，实际 TOTAL=$TOTAL EXEMPTED=$EXEMPTED VIOLATIONS=${#VIOLATIONS[@]}" >&2
        exit 1
    fi

    # 旧发现正则：只认 `go func(` 与 `go identifier(`，方法/包选择器漏检，
    # 且会把原始字符串里的 `go plain()` 当成 goroutine。
    old_count="$(grep -cE '\bgo\s+(func\(|[a-zA-Z_]\w*\()' "$FIXTURE" || true)"
    echo "SELFTEST PASS: AST 检出 4 处（selector=2/func=1/call=1；defer/string/comment 0 处）；门禁在 fixture 上会产生 4 个未豁免违规（exit 1 路径）；旧发现正则命中 $old_count 处（漏检 2 个 selector，且含 1 个原始字符串误报）"
}

if [ "${1:-}" = "--self-test" ]; then
    run_self_test
    exit 0
fi

collect_violations internal

if [ "${#VIOLATIONS[@]}" -gt 0 ]; then
    echo "❌ goroutine panic recovery 门禁失败 — 发现 ${#VIOLATIONS[@]} 处未包裹 SafeGo："
    echo ""
    for v in "${VIOLATIONS[@]}"; do
        echo "  $v"
    done
    echo ""
    echo "修复：在 goroutine 启动处替换为 recover.SafeGo / SafeGoWithLogger"
    echo "  import \"github.com/guliangiam/guliangiam/internal/platform/recover\""
    echo "  recover.SafeGo(\"<component-name>\", func() { <body> })"
    exit 1
else
    echo "✅ goroutine safety 全通过 — 扫描 internal/ 共 $TOTAL 个 go 语句，$EXEMPTED 个豁免，0 个未包裹"
    exit 0
fi
