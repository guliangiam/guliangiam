#!/usr/bin/env bash
# ────────────────────────────────────────────────────────────────────────────
# check-error-response.sh — handler 错误体格式门禁
#
# 规则：internal/transport/http/api/v1/**/handler.go 中所有 handler
# 必须使用 route.WriteError() / route.WriteProtocolError() 输出错误。
# 不允许手写 w.WriteHeader(4xx|5xx) + w.Write(错误体文本)。
#
# 豁免清单（协议端点，返回格式非 JSON 错误体）：
#   cas/chi.go        — CAS XML Service Ticket / PGT / Validate
#   saml/chi.go       — SAML Metadata XML / LogoutResponse XML / HTML
#   scim/handler.go   — SCIM 204 No Content / partial success
#   kerberos/chi.go   — WWW-Authenticate: Negotiate 必须先写 header
#   wellknown/chi.go  — .well-known/saml.xml 元数据 XML
#   auth/chi_protocol.go — OIDC Discovery 200 + Kerberos 401
#   auth/cas_handler.go  — CAS XML + Unauthorized
#   entries/otlp.go   — OTLP protobuf 二进制
#   payments/notify.go — 第三方支付 "success"/"fail" 纯文本
#   rebac/chi.go      — DELETE 成功 204 No Content（正确行为，无 body）
#   ldapsync/handler.go — DELETE 成功 204 NoContent（正确行为）
#   idv/chi.go        — Device Auth 204（正确行为）
#   i18n/handler.go   — 文件内私有 writeJSON helper（非 handler）
#   internal/transport/http/ 中间件层 — 自有 writeHTTPError 等工具
#
# 退出码：0=全通过，1=发现违规
# ────────────────────────────────────────────────────────────────────────────


# 输入：internal/transport/http/api/v1/**/handler.go
# 输出：PASS/FAIL + 绕过写入器的 handler
# --check 行为：无 --check；失败即 exit 1
# 消费方：独立脚本、CI（check-errors.py 为 Python 版）
set -euo pipefail

ROOT="${ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"
cd "$ROOT"

VIOLATIONS=()

# 豁免文件列表（相对路径）
EXEMPT_FILES=(
    "internal/transport/http/api/v1/cas/chi.go"
    "internal/transport/http/api/v1/saml/chi.go"
    "internal/transport/http/api/v1/scim/handler.go"
    "internal/transport/http/api/v1/kerberos/chi.go"
    "internal/transport/http/api/v1/wellknown/chi.go"
    "internal/transport/http/api/v1/auth/chi_protocol.go"
    "internal/transport/http/api/v1/auth/cas_handler.go"
    "internal/transport/http/api/v1/entries/otlp.go"
    "internal/transport/http/api/v1/payments/notify.go"
    "internal/transport/http/api/v1/rebac/chi.go"
    "internal/transport/http/api/v1/ldapsync/handler.go"
    "internal/transport/http/api/v1/idv/chi.go"
    "internal/transport/http/api/v1/i18n/handler.go"
)

is_exempt() {
    local file="$1"
    for exempt in "${EXEMPT_FILES[@]}"; do
        if [[ "$file" == *"$exempt" ]]; then
            return 0
        fi
    done
    return 1
}

# 扫描 handler.go 中的手写错误响应（4xx/5xx WriteHeader 后跟 Write）
while IFS=: read -r file line content; do
    # 跳过豁免文件
    if is_exempt "$file"; then
        continue
    fi

    # 跳过测试文件
    [[ "$file" == *_test.go ]] && continue

    # 检查当前行是不是 WriteHeader(4xx|5xx)
    if echo "$content" | grep -qE 'WriteHeader\(.*(4[0-9][0-9]|5[0-9][0-9])'; then
        # 检查 handler 内有没有 route.WriteError / WriteProtocolError 调用
        # 如果整文件 handler 有手写错误体 + 没用 WriteError → 违规
        if ! grep -q 'route\.WriteError\|route\.WriteProtocolError' "$file" 2>/dev/null; then
            VIOLATIONS+=("$file:$line $content")
        fi
    fi
done < <(grep -rn 'WriteHeader' internal/transport/http/api/v1/ --include='*.go' | grep -v _test.go | grep -v 'EXEMPT')

if [ "${#VIOLATIONS[@]}" -gt 0 ]; then
    echo "❌ handler 错误体门禁失败 — 发现 ${#VIOLATIONS[@]} 处手写 4xx/5xx 响应未用 route.WriteError："
    echo ""
    for v in "${VIOLATIONS[@]}"; do
        echo "  $v"
    done
    echo ""
    echo "修复：用 route.WriteError(w, r, err) 替代手写 w.WriteHeader + w.Write"
    echo "或加入 scripts/check-error-response.sh 豁免清单"
    exit 1
else
    echo "✅ handler error response 全通过 — 所有 4xx/5xx 错误响应使用 route.WriteError"
    echo "   豁免 ${#EXEMPT_FILES[@]} 个协议端点文件（CAS/SAML/SCIM/Kerberos/OTLP/Webhook 等）"
    exit 0
fi
