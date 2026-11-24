#!/usr/bin/env bash
# check-lint.sh - golangci-lint 静态分析门禁
#
# 用法：
#   bash scripts/check-lint.sh              # 提示模式：打印增量命中与存量，永不阻断
#   bash scripts/check-lint.sh --strict     # 门禁模式：阻断"本次改动引入的新问题"
#   bash scripts/check-lint.sh --all        # 全量模式：对全仓阻断（存量为 0 时再启用）
#
# 为什么默认是增量而不是全量：全仓历史存量（errcheck/staticcheck/nilerr/lll 等）有
# 数百条，全量阻断会让门禁长期红灯、进而被绕过——那等于没有门禁。--strict 用
# `--new-from-rev` 保证"只许减少、不许新增"，同时把存量以提示方式输出，清理进度可见。
# CI 与 scripts/verify-docs.sh 使用 --strict。
#
# 环境变量：
#   BASE_REF=origin/main      指定增量比较基线（默认自动推断）
#   REQUIRE_GOLANGCI_LINT=1   未安装 golangci-lint 时视为失败（CI 使用）

# 输入：internal/**/*.go、BASE_REF 指定的增量比较基线
# 输出：默认模式输出增量命中 + 存量；--strict 模式阻断新增问题
# --check 行为：--strict 为 verify-docs.sh/CI 使用的阻断模式
# 消费方：verify-docs.sh(RUN_CODE_CHECKS=1, --strict)、CI
set -euo pipefail

# `go install` 把二进制放进 GOBIN/GOPATH/bin，而该目录通常不在 PATH 上（本仓库的
# 开发机就是这种情况：v2.13.2 已在 GOPATH/bin，但 `command -v` 找不到，门禁会以
# "工具缺失" 失败并误导为环境问题）。
if [ -n "${GOBIN:-}" ]; then
    PATH="$GOBIN:$PATH"
elif command -v go >/dev/null 2>&1; then
    PATH="$(go env GOPATH)/bin:$PATH"
fi
export PATH

MODE="advisory"
ROOT=""
for arg in "$@"; do
    case "$arg" in
        --strict) MODE="strict" ;;
        --all) MODE="all" ;;
        --advisory) MODE="advisory" ;;
        *) ROOT="$arg" ;;
    esac
done
ROOT="${ROOT:-$(pwd)}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=========================================="
echo "  golangci-lint 静态分析门禁（模式：${MODE}）"
echo "=========================================="
echo ""

cd "$ROOT"

# 受限环境（文件沙箱）中默认 GOCACHE 位于用户缓存目录，可能不可写，导致 go list/typecheck
# 直接失败并伪装成“无 Go 文件可分析”。统一落到仓库内 gitignored 缓存目录（CI 亦可复用）。
export GOCACHE="${GOCACHE:-$ROOT/.gotmp/gocache}"
mkdir -p "$GOCACHE"

# 版本与配置自检。v1 语法的 .golangci.yml 配上 v2 二进制会直接报
# "unsupported version of the configuration"；没有这道自检，作业会以一个与代码无关
# 的原因失败，容易长期被当作环境问题忽略。
if ! command -v golangci-lint >/dev/null 2>&1; then
    # 工具缺失是可恢复的环境问题，不是代码问题：本地只在显式要求时失败
    # （REQUIRE_GOLANGCI_LINT=1，CI 设置），否则降级为警告，避免门禁因环境变成
    # 永久红灯而被整体绕过。
    if [ "${REQUIRE_GOLANGCI_LINT:-0}" = "1" ]; then
        echo -e "${RED}FAIL: 未找到 golangci-lint（REQUIRE_GOLANGCI_LINT=1）${NC}"
        echo "  安装：go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0"
        exit 1
    fi
    echo -e "${YELLOW}WARN: 未安装 golangci-lint，跳过静态分析检查（不阻断）${NC}"
    echo "  安装：go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0"
    exit 0
fi

if ! golangci-lint config verify >/dev/null 2>&1; then
    echo -e "${RED}FAIL: .golangci.yml 无法被当前 golangci-lint 加载${NC}"
    golangci-lint config verify 2>&1 | head -5 | sed 's/^/    /'
    golangci-lint version 2>&1 | head -1 | sed 's/^/    /'
    exit 1
fi
echo -e "${GREEN}配置自检通过：$(golangci-lint version 2>&1 | head -1 | cut -c1-60)${NC}"

BASE=""
if [ -n "${BASE_REF:-}" ]; then
    BASE="$BASE_REF"
elif git -C "$ROOT" rev-parse --verify --quiet origin/main >/dev/null 2>&1; then
    BASE="$(git -C "$ROOT" merge-base HEAD origin/main 2>/dev/null || echo HEAD)"
else
    BASE="HEAD"
fi

count_issues() { grep -cE '^[^ ].*\.go:[0-9]+:' "$1" || true; }

STATUS=0
if [ "$MODE" = "all" ]; then
    set +e
    golangci-lint run --timeout 10m ./... > /tmp/golangci-strict.log 2>&1
    STATUS=$?
    set -e
    echo "全量扫描：$(count_issues /tmp/golangci-strict.log) 条"
else
    set +e
    golangci-lint run --timeout 10m --new-from-rev="$BASE" ./... > /tmp/golangci-strict.log 2>&1
    STATUS=$?
    set -e
    echo "增量扫描（基线 ${BASE}）：$(count_issues /tmp/golangci-strict.log) 条"
fi

# 环境问题与代码问题必须分开：golangci-lint 二进制内嵌的 go/types 版本若落后于
# 当前 go 工具链（例如二进制 built with go1.27.1、工具链 go1.27.2），会报
# "export data version N is greater than maximum supported version"——这是
# 工具链版本错配，不是代码缺陷。CI 用匹配版本安装，本地环境按 WARN 跳过，
# 否则门禁会因环境问题长期红灯并被绕过。设置 REQUIRE_LINT_TOOLCHAIN=1 可强制失败。
if grep -qE 'export data version|maximum supported version|could not import .*/internal/|no go files to analyze|context loading failed' /tmp/golangci-strict.log 2>/dev/null; then
    if [ "${REQUIRE_LINT_TOOLCHAIN:-0}" = "1" ]; then
        echo -e "${RED}FAIL: golangci-lint 与当前 Go 工具链版本不匹配（REQUIRE_LINT_TOOLCHAIN=1）${NC}"
        exit 1
    fi
    echo -e "${YELLOW}WARN: golangci-lint 与当前 Go 工具链版本不匹配，export data 无法解码——跳过本次静态分析（环境问题，非代码问题）${NC}"
    echo "  golangci-lint: $(golangci-lint version 2>&1 | head -1) / go: $(go version | awk '{print $3}')"
    echo "  复现修复：用当前 go 重新 go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0"
    exit 0
fi

if [ "$STATUS" -ne 0 ]; then
    if [ "$MODE" = "advisory" ]; then
        echo -e "${YELLOW}提示：本次改动引入或被判为新增的静态分析问题（不阻断）${NC}"
    else
        echo -e "${RED}FAIL: 本次改动引入静态分析问题${NC}"
    fi
    grep -E '^[^ ].*\.go:[0-9]+:' /tmp/golangci-strict.log 2>/dev/null | head -25 | sed 's/^/    /' || true
    echo "  完整报告：/tmp/golangci-strict.log"
    if [ "$MODE" != "advisory" ]; then exit 1; fi
fi

# 存量：只报告，用于跟踪清理进度。
set +e
golangci-lint run --timeout 10m ./... > /tmp/golangci-backlog.log 2>&1
BACKLOG=$(count_issues /tmp/golangci-backlog.log)
set -e
BACKLOG_LINTERS=$(grep -E '^\* ' /tmp/golangci-backlog.log | sed 's/^\* //' | paste -sd', ' - || true)
if [ "${BACKLOG:-0}" -gt 0 ]; then
    echo -e "${YELLOW}存量（不阻断）：${BACKLOG} 条报告${NC}"
    [ -n "$BACKLOG_LINTERS" ] && echo "  分布：${BACKLOG_LINTERS}"
else
    echo -e "${GREEN}存量：0 条，可切换 --all 全量阻断${NC}"
fi

if [ "$MODE" = "advisory" ]; then
    echo -e "${GREEN}PASS: 提示模式结束${NC}"
else
    echo -e "${GREEN}PASS: 无新增静态分析问题${NC}"
fi
exit 0
