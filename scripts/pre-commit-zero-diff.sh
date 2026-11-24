#!/usr/bin/env bash
# 可选提交前零差异校验（默认关闭；不安装到 .git/hooks）
#
# 用法：
#   GATED_ZERO_DIFF=1 bash scripts/pre-commit-zero-diff.sh
#
# 语义：
#   - 未设置 GATED_ZERO_DIFF=1 时立即 exit 0，不产生任何副作用。
#   - 启用后按当前 index（GIT_INDEX_FILE 优先）物化暂存树到临时目录，在副本中运行
#     scripts/check-generated-zero-diff.sh；工作树里他人未提交的 WIP 不会被当成事实源。
#   - 只读：不写仓库文件、不改 index、不改 refs。
#
# 为什么必须 cd 到物化根：scripts/schema_common.py 的 SCHEMA_DIR 是相对路径
# `contracts/schema`，cwd 不是物化根时会读错声明目录（实测会打印「未找到任何表定义」）。
set -euo pipefail

if [ "${GATED_ZERO_DIFF:-0}" != "1" ]; then
    exit 0
fi

REPO_ROOT="$(git rev-parse --show-toplevel)"
CHANGED="$(git -C "$REPO_ROOT" diff --cached --name-only --diff-filter=ACMRD)"

# 触发面 = 生成器输入 ∪ 生成物 ∪ 生成器实现（宁可宽，多跑约 6 秒好过漏检）。
TRIGGER_RE='(^(contracts/(schema|registries)/|db/|openapi/|docs/(generated|design|requirements|adr)/|docs/(acceptance-criteria|traceability-matrix)\.md|\.env\.example|web/src/(generated|pages|shared/api|shared/types/generated)/|web/src/app/router/router\.tsx|web/src/features/navigation/menu\.tsx|web/src/app/theme/brand-palette\.ts|scripts/gen_[^/]*\.py|scripts/frontend_common\.py|web/oxfmt\.json)|routes\.go$|/schema_sql/|/types/generated/|capabilities_gen\.go$|state_generated\.go$|password_change_exempt_gen\.go$)'

if ! printf '%s\n' "$CHANGED" | grep -qE "$TRIGGER_RE"; then
    exit 0
fi

STAGE="$(mktemp -d "${TMPDIR:-/tmp}/giam-staged.XXXXXX")"
trap 'rm -rf "$STAGE"' EXIT

git -C "$REPO_ROOT" checkout-index -a --prefix="$STAGE/"
# 3 个生成器依赖 web/node_modules 里的 oxfmt；只读软链，不复制、不安装。
ln -sfn "$REPO_ROOT/web/node_modules" "$STAGE/web/node_modules"

cd "$STAGE"
env -u GULIANGIAM_API_VERSION bash "$STAGE/scripts/check-generated-zero-diff.sh" "$STAGE"
