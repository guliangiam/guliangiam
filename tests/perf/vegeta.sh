#!/usr/bin/env bash
# ============================================================================
# GuliangIAM Performance Regression Test
# ============================================================================
# 使用 vegeta（Go 原生 load tester）对核心端点做 30s 持续压测，
# 输出 p50 / p95 / p99 / 错误率 指标，用于 CI 中建立性能基线。
#
# 前置条件：
#   - vegeta 已安装（go install github.com/tsenart/vegeta/v12@latest）
#   - 目标服务可访问（默认 http://localhost:8000）
#
# 用法：
#   ./tests/perf/vegeta.sh [target-url] [duration] [rate]
#   ./tests/perf/vegeta.sh http://prod.example.com 60s 500
# ============================================================================

set -euo pipefail

TARGET="${1:-http://localhost:8000}"
DURATION="${2:-30s}"
RATE="${3:-100}"  # requests per second per endpoint

echo "========================================"
echo "GuliangIAM Performance Test"
echo "Target:   $TARGET"
echo "Duration: $DURATION"
echo "Rate:     ${RATE} req/s (per endpoint)"
echo "========================================"

if ! command -v vegeta &> /dev/null; then
  echo "❌ vegeta not found. Install with:"
  echo "   go install github.com/tsenart/vegeta/v12@latest"
  exit 1
fi

OUTDIR="tests/perf/reports"
mkdir -p "$OUTDIR"
TIMESTAMP="$(date +%Y%m%d-%H%M%S)"

# ── 端点 1：/healthz（liveness，最高优先级，必须 P99 < 50ms） ──────────────
echo ""
echo "▶ Endpoint: GET /healthz (liveness probe)"
vegeta attack \
  -targets=<(echo "GET ${TARGET}/healthz") \
  -rate="${RATE}" \
  -duration="$DURATION" \
  -timeout=5s \
  -name="healthz" | \
  vegeta report -type=json > "$OUTDIR/healthz-${TIMESTAMP}.json"
cat "$OUTDIR/healthz-${TIMESTAMP}.json" | python3 -c "
import json, sys
d = json.load(sys.stdin)
print(f'  requests={d[\"requests\"]}  success={d[\"success\"]:.2%}')
print(f'  latencies  p50={d[\"latencies\"][\"50\"]/1e6:.1f}ms  p95={d[\"latencies\"][\"95\"]/1e6:.1f}ms  p99={d[\"latencies\"][\"99\"]/1e6:.1f}ms')
print(f'  errors: {len(d.get(\"errors\", []))}')
"

# ── 端点 2：GET /api/v1/organizations（典型列表查询，P99 < 300ms） ────────
echo ""
echo "▶ Endpoint: GET /api/v1/organizations (list query)"
vegeta attack \
  -targets=<(echo "GET ${TARGET}/api/v1/organizations") \
  -rate="$((RATE / 5))" \
  -duration="$DURATION" \
  -timeout=10s \
  -name="organizations-list" | \
  vegeta report -type=json > "$OUTDIR/org-list-${TIMESTAMP}.json"
cat "$OUTDIR/org-list-${TIMESTAMP}.json" | python3 -c "
import json, sys
d = json.load(sys.stdin)
print(f'  requests={d[\"requests\"]}  success={d[\"success\"]:.2%}')
print(f'  latencies  p50={d[\"latencies\"][\"50\"]/1e6:.1f}ms  p95={d[\"latencies\"][\"95\"]/1e6:.1f}ms  p99={d[\"latencies\"][\"99\"]/1e6:.1f}ms')
print(f'  errors: {len(d.get(\"errors\", []))}')
"

# ── 端点 3：GET /api/v1/applications/{id}（典型详情查询，P99 < 200ms） ─────
echo ""
echo "▶ Endpoint: GET /api/v1/applications/{id} (detail)"
vegeta attack \
  -targets=<(echo "GET ${TARGET}/api/v1/applications/app-stub-id") \
  -rate="$((RATE / 5))" \
  -duration="$DURATION" \
  -timeout=10s \
  -name="application-detail" | \
  vegeta report -type=json > "$OUTDIR/app-detail-${TIMESTAMP}.json"
cat "$OUTDIR/app-detail-${TIMESTAMP}.json" | python3 -c "
import json, sys
d = json.load(sys.stdin)
print(f'  requests={d[\"requests\"]}  success={d[\"success\"]:.2%}')
print(f'  latencies  p50={d[\"latencies\"][\"50\"]/1e6:.1f}ms  p95={d[\"latencies\"][\"95\"]/1e6:.1f}ms  p99={d[\"latencies\"][\"99\"]/1e6:.1f}ms')
print(f'  errors: {len(d.get(\"errors\", []))}')
"

echo ""
echo "✅ 测试完成，报告写入 $OUTDIR/"
echo ""
echo "性能基线阈值（§9.6 验收准则）："
echo "  /healthz              P99 < 50ms   错误率 < 0.1%"
echo "  /api/v1/organizations P99 < 300ms  错误率 < 1%"
echo "  /api/v1/applications  P99 < 200ms  错误率 < 1%"
