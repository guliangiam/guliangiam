#!/usr/bin/env bash
#
# 文档事实门禁（目标态版）。
#
# 背景：docs/ 正式文档是「纯目标态设计规范」（见 docs/README.md §1）。文档治理
# 三条红线由本脚本自动拦截：
#   1. 三态状态标注（[Current]/[Target]/[Open]）不得出现于 docs/ 正文 —— 文档即
#      目标态规范，现状偏差登记已废除；docs/adr/ 按 ADR 自身治理规范保留状态词，
#      为唯一例外目录。
#   2. 已废弃的规模数字面量不得复活 —— 文档不得手写计数（表数/路由数/端点数/
#      环境变量数/用例数），引用事实只写「事实源路径 + 复现命令」。
#   3. 批次记录、收敛记录与结项叙事不得出现于 docs/ —— 一切历史唯一写入仓库根
#      CHANGELOG.md。
#
# 另校验：开发规范手册头部「文档版本」必须等于 CHANGELOG「文档版本沿革」最新行。
#
# 退出码：0 = 全部通过；非零 = 存在任一失败。

# 输入：docs/*.md、CHANGELOG.md
# 输出：PASS/FAIL + 命中行
# --check 行为：无 --check；失败即 exit 1
# 消费方：verify-docs.sh(4/4)、CI
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

SPEC="docs/GuliangIAM-开发规范.md"
CHANGELOG="CHANGELOG.md"

failures=0
fail() { printf 'FAIL: %s\n' "$*" >&2; failures=$((failures + 1)); }
pass() { printf 'PASS: %s\n' "$*"; }

# ---------------------------------------------------------------------------
# 检查 1：docs/ 正文禁止三态状态标注（ADR 目录例外）。
#
# docs/ 正式文档为目标态规范，不维护现状三态登记；状态词只允许出现在
# docs/adr/（其自身治理规范保留 [Current]/[Target]/[Open]）。任何正文残留
# 三态标注都意味着现状治理内容被错误保留。
# ---------------------------------------------------------------------------
echo "== 三态状态标注（docs/adr/ 例外）=="
# README 的治理规则文本自身需要引用状态词（说明禁令内容），其全部行豁免。
state_hits="$(grep -rInE --include='*.md' '\[(Current|Target|Open)\]' docs 2>/dev/null |
  grep -vE '^docs/adr/' |
  grep -vE '^docs/README\.md:[0-9]+:' || true)"
if [[ -n "$state_hits" ]]; then
  fail "docs/ 正文出现三态状态标注（文档即目标态规范，状态词仅允许于 docs/adr/）："
  printf '%s\n' "$state_hits" >&2
else
  pass "docs/ 正文无 [Current]/[Target]/[Open]"
fi

# ---------------------------------------------------------------------------
# 检查 2：已废弃的规模数字面量不得复活。
#
# 文档禁止手写计数；下列字面量全部有过「与正确值并存」的历史，任何一次
# 结构/契约变化都会让复写值变成陈旧值。改为引用生成物路径 + 复现命令。
# ---------------------------------------------------------------------------
deprecated_literals=(
  '79 张表'
  '78 张表'
  '194 条索引'
  '195 条索引'
  '172 个普通索引'
  '507 operations'
  '507 个操作'
  '509 operations'
  '511 operations'
  '511 个操作'
  '511 条'
  '511 个 route.Definition'
  '359 路径'
  '359 paths'
  '361 路径'
  '361 paths'
  '501 条 OperationID'
  '501 个 OperationID'
  '429 条语句'
  '433 条语句'
  '66 个 handler 参数'
  '74 个 handler 参数'
  '137 个环境变量'
  '1274 个'
  '1274 个用例'
  '256 条'
  '133 条 AC'
  '69 处'
  '70 处'
  '71 处'
  # 《架构优化方案》分层与目录规模曾被逐处手写；这些字面量在任何一次模块增删、
  # 页面新增、语言包补齐后都会变成陈旧值，改由该文档 3.4「规模事实源」承载。
  '22中间件'
  '22个: CORS'
  '3个API面'
  '60个应用服务'
  '12个UoW子接口'
  '76个领域模块'
  '28个基础设施模块'
  '28个平台模块'
  '58个页面目录'
  '~130路由'
  '~70个API模块'
  '~75个共享组件'
  '14语言, 3命名空间'
  '4方言SQL迁移'
  '6个页面目录'
  '7+文档'
  '7个新文档'
  '11个MD文件'
  '5个UTF-8文件'
  '12种语言包'
  '26语言支持'
)

echo
echo "== 废弃规模数字面量 =="
# docs/adr/ 豁免：ADR 治理规范允许其「验证证据」携带命令 + 实测结果（含计数），
# ADR 是唯一允许书写实测数值的文档域。
for literal in "${deprecated_literals[@]}"; do
  hits="$(grep -rIn --include='*.md' -F "$literal" docs --exclude-dir=adr 2>/dev/null || true)"
  if [[ -n "$hits" ]]; then
    fail "废弃字面量「${literal}」仍被引用（文档禁止手写计数，改为引用生成物路径 + 复现命令）："
    printf '%s\n' "$hits" >&2
  else
    pass "无「${literal}」"
  fi
done

# 批次/结项叙事与实施进度词已迁移至 `scripts/check-doc-redlines.py`
# （该脚本做逐行上下文判定，避免「实现状态」「规划中含」等术语被词面误伤）。

if [[ -d docs/archive ]]; then
  fail "docs/archive/ 仍存在；历史归档不属于 docs/（唯一历史载体为 ${CHANGELOG}）"
else
  pass "无 docs/archive/"
fi

# ---------------------------------------------------------------------------
# 检查 4：文档版本号必须等于 CHANGELOG「文档版本沿革」的最新行。
#
# 版本沿革唯一位于 CHANGELOG；规范正文头部保留一个版本标记，两处不一致时
# 读者会按过期版本理解正文的修订代次。
# ---------------------------------------------------------------------------
echo
echo "== 文档版本号 =="
doc_version="$(
  grep -m1 -oE '\*\*文档版本\*\*：V[0-9]+\.[0-9]+' "$SPEC" 2>/dev/null |
    grep -oE 'V[0-9]+\.[0-9]+' || true
)"
changelog_version="$(
  awk '
    /^## 文档版本沿革/ { in_section = 1; next }
    in_section && /^## / { exit }
    in_section && /^\| \*\*V[0-9]+\.[0-9]+\*\*/ { last = $0 }
    END {
      if (last ~ /V[0-9]+\.[0-9]+/) {
        match(last, /V[0-9]+\.[0-9]+/)
        print substr(last, RSTART, RLENGTH)
      }
    }' "$CHANGELOG"
)"

if [[ -z "$doc_version" ]]; then
  fail "${SPEC} 头部未声明「**文档版本**：Vx.y」"
elif [[ -z "$changelog_version" ]]; then
  fail "${CHANGELOG} 的「文档版本沿革」节未找到版本行（节标题或表行格式已变）"
elif [[ "$doc_version" == "$changelog_version" ]]; then
  pass "文档版本 ${doc_version} 与 ${CHANGELOG} 沿革最新行一致"
else
  fail "文档版本不一致：${SPEC} 声明 ${doc_version}，${CHANGELOG} 沿革最新行为 ${changelog_version}"
fi

# ---------------------------------------------------------------------------
echo
if (( failures > 0 )); then
  echo "文档事实门禁失败：${failures} 项" >&2
  exit 1
fi
echo "文档事实门禁通过。"
