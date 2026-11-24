#!/usr/bin/env bash
# bench-gate.sh — CI benchmark regression gate
# Thin wrapper around bench_gate.py

# 输入：透传 $@ 给 bench_gate.py
# 输出：同 bench_gate.py
# --check 行为：薄包装，自身无 --check
# 消费方：独立脚本、CI（不纳入 verify-docs.sh）
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
exec python3 "${SCRIPT_DIR}/bench_gate.py" "$@"
