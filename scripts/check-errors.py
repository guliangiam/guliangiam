#!/usr/bin/env python3
"""Reject error responses that bypass the unified writers.

Why this exists
---------------
The manual defines exactly two error wire formats (附录 C): the application
envelope produced by `presenter.PresentError` (via `route.WriteError`) and the
protocol formats produced by `route.WriteOAuthError` / `route.WriteSCIMError`.
A handler that hand-rolls `{"error": …}` with `route.WriteJSON` produces a third,
undocumented shape: it omits `requestId`/`traceId`, invents codes that are not in
the appendix C list, and — because the frontend switches on the shape — surfaces
as a generic "Request failed" message. That is exactly how `bad_request` leaked
into the i18n handler (错误码注册表 §3 row 1).

The second rule catches the same leak through the other door: a type declared in
a handler package that implements `Error() string` but neither `ProtocolCode()`
nor `HTTPStatus()`. Such an error carries no status, so `route.WriteError`'s
`StatusCodeFromError` falls through to the 500 default and the protocol face
renders it as `server_error`. That is how ten SAML/wellknown 400s and the three
RADIUS control-plane 409/400s were answering 500 (错误码注册表 §1).

Allowed exceptions are declared below with a reason, so an intentional
provider-facing body (a payment gateway that expects the literal string `fail`)
is a documented decision rather than a silent bypass.

Usage: python3 scripts/check-errors.py
Exit code 0 = every error body goes through a writer, 1 = a bypass was found.

输入：internal/transport/http/ 下 handler 包
输出：PASS/FAIL + 绕过写入器的 handler / 缺失方法的 error 类型
--check 行为：无 --check；失败即 exit 1
消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
"""

import os
import re
import sys

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
TRANSPORT_DIR = os.path.join(REPO_ROOT, "internal/transport/http")

# `route.WriteJSON(w, <status>, map[string]…{"error": …})` — an error body that
# skipped the writers. Matches both the map literal and the nested-map form.
BYPASS = re.compile(
    r"route\.WriteJSON\(\s*\w+\s*,\s*([^,]+),\s*(?:map\[string\][\w\.\[\]]+\{)?"
    r"[^)]{0,400}?\"error\"\s*:",
    re.DOTALL,
)

# Statuses that are not errors: a 2xx body may legitimately carry an `error`
# field (a partial-success report, for example).
NON_ERROR_STATUS = re.compile(r"http\.Status(?:OK|Created|Accepted|NoContent)\b")

# A struct declared in a handler package…
ERROR_TYPE_DECL = re.compile(r"^type\s+(\w+)\s+struct\s*\{", re.MULTILINE)


# …whose Error() method is what route.WriteError would receive.
def _method(type_name, signature):
    return re.compile(
        r"^func\s+\(\w+\s+\*?" + re.escape(type_name) + r"\)\s+" + signature,
        re.MULTILINE,
    )


# Give the writer a status and/or a protocol code. Either is enough: an
# *apperrors.Error is recognised by StatusCodeFromError, and a ProtocolCodeError
# is recognised by OAuthErrorFrom.
STATUS_METHOD = r"HTTPStatus\(\)\s+int"
PROTOCOL_METHOD = r"ProtocolCode\(\)\s+string"
ERROR_METHOD = r"Error\(\)\s*string"

# Reviewed exceptions. Each entry is (path suffix, reason).
ALLOWED = [
    (
        "api/v1/payments/notify.go",
        "payment-gateway callback: the body is dictated by the gateway "
        "(alipay/wechat expect the literal `fail`), not by our API contract",
    ),
]


def allowed(path):
    rel = os.path.relpath(path, REPO_ROOT)
    for suffix, _reason in ALLOWED:
        if rel.endswith(suffix):
            return True
    return False


def main():
    violations = []
    stateless = []
    for dirpath, _dirs, files in os.walk(TRANSPORT_DIR):
        for name in sorted(files):
            if not name.endswith(".go") or name.endswith("_test.go"):
                continue
            path = os.path.join(dirpath, name)
            if allowed(path):
                continue
            with open(path, encoding="utf-8") as handle:
                source = handle.read()
            rel = os.path.relpath(path, REPO_ROOT)

            for match in BYPASS.finditer(source):
                if NON_ERROR_STATUS.search(match.group(1)):
                    continue
                line = source[: match.start()].count("\n") + 1
                violations.append(f"{rel}:{line}")

            for match in ERROR_TYPE_DECL.finditer(source):
                type_name = match.group(1)
                if not _method(type_name, ERROR_METHOD).search(source):
                    continue
                if _method(type_name, STATUS_METHOD).search(source):
                    continue
                if _method(type_name, PROTOCOL_METHOD).search(source):
                    continue
                line = source[: match.start()].count("\n") + 1
                stateless.append(f"{rel}:{line} ({type_name})")

    failed = False

    if violations:
        failed = True
        print(
            f"Error-writer check FAILED ({len(violations)} hand-rolled error "
            "body/bodies found):"
        )
        for item in violations:
            print(f"  - {item}")
        print(
            "  Use route.WriteError for the management/authentication faces and "
            "route.WriteOAuthError / route.WriteSCIMError for the protocol face.",
            file=sys.stderr,
        )

    if stateless:
        failed = True
        print(
            f"Error-writer check FAILED ({len(stateless)} error type(s) carrying no "
            "status):"
        )
        for item in stateless:
            print(f"  - {item}")
        print(
            "  A type with Error() but neither HTTPStatus() int nor "
            "ProtocolCode() string makes route.WriteError answer 500: "
            "StatusCodeFromError falls through to the internal default and the "
            "protocol face renders server_error. Return an *apperrors.Error "
            "(apperrors.NewInvalidArgument(...) etc.) instead of a local type.",
            file=sys.stderr,
        )

    if failed:
        return 1

    print(
        "Error-writer check OK: every error response goes through "
        "route.WriteError / WriteOAuthError / WriteSCIMError, and no handler "
        "package declares a status-less error type"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
