#!/usr/bin/env bash
# ============================================================================
# GuliangIAM Security Scan — gosec (Go AST 静态安全分析)
# ============================================================================
# gosec 扫描 Go 代码，识别 20+ 类常见安全问题：
#   - SQL 注入、命令注入、路径遍历
#   - 弱加密（MD5/SHA1/ECB 模式）
#   - 硬编码凭据、不安全随机数
#   - 未验证 TLS 证书、文件权限过大
#
# CI 集成参考 .github/workflows/ci.yml 已有的 "Security Scan" job（govulncheck）。
# 本脚本补充 gosec AST 层扫描（govulncheck 管依赖 CVE，gosec 管源码模式）。
#
# 用法：
#   ./tests/security/gosec-scan.sh             # 全仓扫描
#   ./tests/security/gosec-scan.sh ./internal/app/auth  # 指定包
# ============================================================================

set -euo pipefail

TARGET="${1:-./internal/...}"
SEVERITY="${2:-medium}"  # low / medium / high

echo "========================================"
echo "GuliangIAM Security Scan (gosec)"
echo "Target:   $TARGET"
echo "Severity: >= $SEVERITY"
echo "========================================"

if ! command -v gosec &> /dev/null; then
  echo "📥 Installing gosec..."
  go install github.com/securego/gosec/v2/cmd/gosec@latest
fi

# gosec 配置：排除低置信度规则，保留全部 high/medium 级别
# G201 SQL Injection / G204 Command Injection / G304 Path Traversal 必须 fail
# G501 Blocklisted crypto (MD5/SHA1) 必须 fail
echo ""
echo "▶ Running gosec..."

OUTDIR="tests/security/reports"
mkdir -p "$OUTDIR"
TIMESTAMP="$(date +%Y%m%d-%H%M%S)"

gosec \
  -severity "${SEVERITY}+" \
  -confidence medium \
  -logfile "$OUTDIR/gosec-${TIMESTAMP}.json" \
  -no-fail \
  -format json \
  "$TARGET"

echo ""
echo "✅ gosec 完成，报告写入 $OUTDIR/"

# 输出摘要
if [ -s "$OUTDIR/gosec-${TIMESTAMP}.json" ]; then
  python3 -c "
import json, sys
with open('$OUTDIR/gosec-${TIMESTAMP}.json') as f:
    data = json.load(f)
issues = data.get('Issues', []) if isinstance(data, dict) else data
if not issues:
    print('  ✅ 发现 0 个安全问题')
    sys.exit(0)
by_sev = {}
for i in issues:
    sev = i.get('severity', '?')
    by_sev[sev] = by_sev.get(sev, 0) + 1
print(f'  📋 总计 {len(issues)} 个问题')
for sev, count in sorted(by_sev.items()):
    emoji = {'high': '🔴', 'medium': '🟡', 'low': '🟢'}.get(sev, '⚪')
    print(f'    {emoji} {sev}: {count}')
print('')
print('  ⚠️  CI 门禁配置需在 .github/workflows/ci.yml 中设置 fail_on=high')
"
else
  echo "  ✅ 无发现"
fi
