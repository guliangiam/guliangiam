#!/usr/bin/env python3

"""从 internal/transport/http/api/v1/**/routes.go 生成 openapi/openapi.yaml

输入：internal/transport/http/api/v1/**/routes.go
输出：openapi/openapi.yaml；--check 模式 diff 并输出 PASS/FAIL
--check 行为：--check 模式比较当前 openapi.yaml 与生成结果的差异，无差异时 PASS
消费方：check-generated-zero-diff.sh、verify-docs.sh(RUN_CODE_CHECKS=1, 调用 scripts/gen_openapi.py)、CI"""

import os
import re
import sys
from collections import OrderedDict

BASE_DIR = "internal/transport/http/api/v1"
OUTPUT_FILE = "openapi/openapi.yaml"

# API contract version. Independent from the product/release version: it changes only
# when the generated contract changes incompatibly. Override with GULIANGIAM_API_VERSION.
API_VERSION = os.environ.get("GULIANGIAM_API_VERSION", "1.0.0")

# There used to be a KNOWN_DUPLICATES allowlist here for WebAuthn routes declared
# both in auth/routes.go and webauthn/routes.go. Those duplicated declarations were
# removed and the route contract test now pins the declaration set against runtime
# registrations with zero delta (手册 DB-055), so the allowlist would only ever mask
# a new duplicate: it is gone, and *any* duplicate path+method pair fails the
# generator. Per-route counts are not written here on purpose (docs/README.md §1).

# API face (手册 4.1.3) to OpenAPI tag prefix. The surface is read from the
# route declaration itself, so the tag can never disagree with the surface
# contract gate.
SURFACE_PREFIX = {
    "SurfaceManagement": "management",
    "SurfaceAuthentication": "authentication",
    "SurfaceProtocol": "protocol",
}

# Browser-facing protocol callbacks consume an HTML form instead of JSON. The
# success status and response headers come from route.Definition; this set only
# selects the request media type, because the struct has no media-type field.
FORM_URLENCODED_OPERATIONS = {"SamlSpAcs"}
# Multipart routes that are not named "*Upload*": they carry a bare file part
# only, so they get the upload shape minus the group `tag` property.
MULTIPART_FILE_ONLY_OPERATIONS = {"RecognizeUserIdentityDocument"}
# These publish the manager's JWK/JWKS types verbatim; a generic "object" would
# hide the algorithm family (kty/alg/crv) an RP has to dispatch on.
JWKS_OPERATIONS = {"GetJWKS", "GetJWKSByApplication", "GetGMJWKS"}

# Header and redirect descriptions stay here (they are presentation, not
# contract); the names themselves come from route.Definition.ResponseHeaders.
HEADER_DESCRIPTIONS = {
    "Location": "URL of the created resource or redirect target.",
    "Retry-After": "Seconds to wait before retrying the request.",
    "Set-Cookie": "Opaque session cookie.",
    "X-CLI-SHA256": "SHA-256 checksum of the downloaded CLI artifact.",
    "X-CLI-Filename": "Filename of the downloaded CLI artifact.",
}
REDIRECT_DESCRIPTIONS = {
    301: "Moved Permanently; follow the Location header.",
    302: "Found; follow the Location header.",
    303: "See Other; follow the Location header.",
    307: "Temporary Redirect; repeat the request at the Location.",
    308: "Permanent Redirect; repeat the request at the Location.",
}

# Module name to tag display name mapping
TAG_MAP = {
    "face_id_providers": "face-id-providers",
    "captcha_providers": "captcha-providers",
    "notification_providers": "notification-providers",
    "radius_configs": "radius-configs",
    "radiusaccountings": "radius-accountings",
    "mcpservers": "mcp-servers",
    "ldapsync": "ldaps",
}

# Tag descriptions
TAG_DESCRIPTIONS = {
    "system": "Runtime and health endpoints.",
    "auth": "Session and account context APIs.",
    "applications": "Application management APIs.",
    "organizations": "Organization management APIs.",
    "providers": "Provider management APIs.",
    "permissions": "Permission management APIs.",
    "roles": "Role management APIs.",
    "sessions": "Session management APIs.",
    "tokens": "Token management APIs.",
    "users": "User management APIs.",
    "groups": "Group management APIs.",
    "enforcers": "Casbin enforcer management and policy enforcement APIs.",
    "syncers": "Identity syncer management APIs.",
    "scim": "SCIM v2 protocol endpoints (RFC 7644).",
    "cas": "CAS (Central Authentication Service) protocol endpoints.",
    "dcr": "OAuth 2.0 Dynamic Client Registration (RFC 7591).",
    "payments": "Payment management APIs.",
    "messages": "Messaging and notification APIs.",
    "certs": "X.509 certificate management APIs.",
    "keys": "Cryptographic key management APIs.",
    "webhooks": "Webhook management and event delivery APIs.",
    "ldaps": "LDAP directory configuration and sync APIs.",
    "agents": "Agent management APIs.",
    "forms": "Form definition and management APIs.",
    "gateway": "API gateway server and site management APIs.",
    "invitations": "User invitation management APIs.",
    "orders": "Order management APIs.",
    "plans": "Subscription plan management APIs.",
    "pricings": "Pricing management APIs.",
    "products": "Product management APIs.",
    "subscriptions": "Subscription management APIs.",
    "coupons": "Coupon and discount management APIs.",
    "resources": "File and resource management APIs.",
    "records": "Audit record management APIs.",
    "entries": "Observability entry and OTLP ingestion APIs.",
    "rules": "Authorization rule management APIs.",
    "dashboard": "Dashboard statistics and analytics APIs.",
    "face_id_providers": "Face ID provider configuration APIs.",
    "captcha_providers": "CAPTCHA provider configuration APIs.",
    "notification_providers": "Notification provider configuration APIs.",
    "notifications": "Notification management APIs.",
    "radius_configs": "RADIUS configuration management APIs.",
    "radiusaccountings": "RADIUS accounting record APIs.",
    "verifications": "Verification code management APIs.",
    "tickets": "Support ticket management APIs.",
    "transactions": "Transaction management APIs.",
    "mcpservers": "MCP (Model Context Protocol) server management APIs.",
    "wellknown": "Well-known discovery and configuration endpoints.",
    "adapters": "Adapter management APIs.",
    "models": "AI model management APIs.",
    "i18n": "Internationalization APIs.",
    "saml": "SAML protocol endpoints.",
    "radius": "RADIUS server management APIs.",
    "kerberos": "Kerberos authentication APIs.",
    "webauthn": "WebAuthn authentication APIs.",
    "batch": "Batch operation APIs.",
    "ldapsync": "LDAP sync configuration APIs.",
}


# 每个路由声明是单层 struct literal（字段之间无嵌套结构），按声明块解析再逐字段取值。
# 旧实现用一个"按固定字段顺序贯穿"的大正则：任何声明只要缺 Summary 或字段顺序不同，
# 整条声明就被静默丢弃——aigateway 的 50 条声明（管理面 CRUD + 协议面 /ai/faces）
# 因未写 Summary 而从未进入 OpenAPI，门禁却全绿。字段提取与完整性断言因此拆开：
# 逐字段独立匹配，且解析条数必须等于源文件中的 Method 字面量数。
#
# 声明块形态只有两种（gofmt 后稳定）：独占一行的 "{" 起始多行块，或 "{Method: ...}" 单行。
# 按行切块而不是用花括号正则，是因为 Pattern 字符串本身含 "{id}" 等花括号。
ROUTE_FIELDS = {
    "method": re.compile(r'Method:\s*"([^"]*)"'),
    "pattern": re.compile(r'Pattern:\s*"([^"]*)"'),
    "operationId": re.compile(r'OperationID:\s*"([^"]*)"'),
    "summary": re.compile(r'Summary:\s*"([^"]*)"'),
    "permission": re.compile(r'Permission:\s*"([^"]*)"'),
    # 包别名不固定：多数模块 import route，aigateway 使用 rthttp 别名。
    "surface": re.compile(r"Surface:\s*[A-Za-z_]\w*\.(Surface\w+)"),
    # SuccessStatus is either a numeric literal or a net/http-style constant;
    # ResponseHeaders is a []string literal. Both are optional and default to
    # 200 / no headers, so existing declarations need no change.
    "successStatus": re.compile(r"SuccessStatus:\s*([A-Za-z0-9_.]+)"),
    "responseHeaders": re.compile(r"ResponseHeaders:\s*\[\]string\{([^}]*)\}"),
    # ErrorResponses is an []int literal of non-default error statuses the
    # handler actually returns. Optional and empty by default, so existing
    # declarations keep their generated contract unchanged.
    "errorResponses": re.compile(r"ErrorResponses:\s*\[\]int\{([^}]*)\}"),
}
ROUTE_REQUIRED_FIELDS = ("method", "pattern", "operationId", "permission", "surface")

# net/http status names accepted in SuccessStatus / ErrorResponses declarations.
HTTP_STATUS_NAMES = {
    "StatusOK": 200,
    "StatusCreated": 201,
    "StatusAccepted": 202,
    "StatusNonAuthoritativeInfo": 203,
    "StatusNoContent": 204,
    "StatusResetContent": 205,
    "StatusPartialContent": 206,
    "StatusMultiStatus": 207,
    "StatusAlreadyReported": 208,
    "StatusIMUsed": 226,
    "StatusMultipleChoices": 300,
    "StatusMovedPermanently": 301,
    "StatusFound": 302,
    "StatusSeeOther": 303,
    "StatusNotModified": 304,
    "StatusUseProxy": 305,
    "StatusTemporaryRedirect": 307,
    "StatusPermanentRedirect": 308,
    "StatusConflict": 409,
    "StatusUnprocessableEntity": 422,
}


def parse_success_status(raw, filepath):
    """Resolve a SuccessStatus literal to its integer value."""
    token = raw.strip()
    if token.isdigit():
        status = int(token)
    else:
        name = token.rsplit(".", 1)[-1]
        if name not in HTTP_STATUS_NAMES:
            raise ValueError(f"{filepath}: 无法解析 SuccessStatus {token!r}")
        status = HTTP_STATUS_NAMES[name]
    if not 100 <= status <= 599:
        raise ValueError(f"{filepath}: SuccessStatus {status} 不是合法 HTTP 状态码")
    return status


def parse_response_headers(raw):
    """Extract header names from a []string literal body."""
    return re.findall(r'"([^"]+)"', raw)


def parse_error_responses(raw, filepath):
    """Resolve an ErrorResponses []int literal to a sorted, de-duplicated list.

    Ordering is normalised so the generated YAML is stable regardless of how the
    declaration lists the statuses; that keeps `gen_openapi.py --check` free of
    false failures caused by key order.
    """
    statuses = set()
    for token in raw.split(","):
        token = token.strip()
        if not token:
            continue
        if token.isdigit():
            statuses.add(int(token))
            continue
        name = token.rsplit(".", 1)[-1]
        if name not in HTTP_STATUS_NAMES:
            raise ValueError(f"{filepath}: 无法解析 ErrorResponses {token!r}")
        statuses.add(HTTP_STATUS_NAMES[name])
    for status in statuses:
        if not 400 <= status <= 599:
            raise ValueError(f"{filepath}: ErrorResponses {status} 不是合法错误状态码")
    return sorted(statuses)


def humanize_operation_id(operation_id):
    """Summary 缺省时的可读兜底：ChatCompletions -> "Chat Completions"."""
    return re.sub(r"(?<=[a-z0-9])(?=[A-Z])", " ", operation_id)


def iter_route_blocks(content):
    """按行切出每个 route.Definition 声明块（多行 "{" 块 / "{Method: ...}" 单行）。"""
    lines = content.split("\n")
    index = 0
    while index < len(lines):
        stripped = lines[index].strip()
        if stripped == "{" or "{Method:" in stripped:
            start = index
            depth = stripped.count("{") - stripped.count("}")
            index += 1
            while index < len(lines) and depth > 0:
                depth += lines[index].count("{") - lines[index].count("}")
                index += 1
            block = "\n".join(lines[start:index])
            if ROUTE_FIELDS["method"].search(block):
                yield block
        else:
            index += 1


def parse_routes_file(filepath, module_name):
    """Parse a routes.go file and extract route definitions.

    完整性契约：返回条目数必须等于源文件中 Method 字面量数；不一致说明解析器无法
    覆盖某种声明形态，此时抛错而不是静默丢端点（丢端点会让 OpenAPI 契约缺面，
    前端 SDK/契约测试与外部集成方全部看不到该 API）。
    """
    with open(filepath, "r") as f:
        content = f.read()

    defs = []
    for block in iter_route_blocks(content):
        fields = {}
        for name, rx in ROUTE_FIELDS.items():
            found = rx.search(block)
            if found:
                fields[name] = found.group(1)
        missing = [name for name in ROUTE_REQUIRED_FIELDS if name not in fields]
        if missing:
            raise ValueError(
                f"{filepath}: 路由声明缺少必填字段 {missing}: {block.strip()[:160]}"
            )
        fields.setdefault("summary", humanize_operation_id(fields["operationId"]))
        if "successStatus" in fields:
            fields["successStatus"] = parse_success_status(fields["successStatus"], filepath)
        else:
            fields["successStatus"] = 200
        fields["responseHeaders"] = parse_response_headers(fields.get("responseHeaders", ""))
        fields["errorResponses"] = parse_error_responses(
            fields.get("errorResponses", ""), filepath
        )
        fields["module"] = module_name
        defs.append(fields)

    declared = len(re.findall(r'Method:\s*"', content))
    if declared != len(defs):
        raise ValueError(
            f"{filepath}: 声明 {declared} 条路由，解析出 {len(defs)} 条——"
            "解析器无法覆盖该声明形态；拒绝静默丢失端点，请修正声明或解析器"
        )
    return defs


def extract_all_routes():
    """Extract all route definitions from all modules."""
    all_defs = []

    for entry in sorted(os.listdir(BASE_DIR)):
        dirpath = os.path.join(BASE_DIR, entry)
        if not os.path.isdir(dirpath):
            continue
        routes_file = os.path.join(dirpath, "routes.go")
        if not os.path.exists(routes_file):
            continue
        defs = parse_routes_file(routes_file, entry)
        all_defs.extend(defs)



    return all_defs


def get_tag_name(module, surface):
    """Get the display tag name for a module, prefixed with its API face.

    The face prefix (manual 4.1.3 item 7) makes the OpenAPI tag list a
    per-face grouping: a module that serves more than one face (auth serves
    both the authentication face and protocol family 5) produces one tag per
    face instead of a single ambiguous tag.
    """
    base = TAG_MAP.get(module, module)
    prefix = SURFACE_PREFIX.get(surface)
    if prefix is None:
        return base
    return f"{prefix}: {base}"


def get_tag_description(module):
    """Get the description for a module tag."""
    return TAG_DESCRIPTIONS.get(module, f"{module} management APIs.")


def extract_path_params(pattern):
    """Extract path parameter names from a URL pattern."""
    return re.findall(r"\{(\w+)\}", pattern)


def is_list_endpoint(op_id, pattern):
    """Check if an endpoint is a list endpoint."""
    return op_id.startswith("List") or op_id.startswith("ScimList")


# --- Query parameter contract -------------------------------------------------
#
# The spec must document the parameters the server actually honours, and nothing
# else. It used to document a fixed set on every list endpoint: `filter`, which
# only the SCIM handlers read (2 of 61), and `sortField`/`sortOrder`, which only
# `users` and `records` read (2 of 61) — while `keyword`, read by 44 handlers and
# sent by every console search box, appeared nowhere in the spec. A documented
# parameter that no handler reads is not a harmless extra: the client that follows
# the document sends it, the server ignores it, and an unknown query parameter is
# not an error, so the feature fails silently.
#
# The parameters are therefore derived from the implementation, using the same
# accessor set `scripts/check-api-params.py` verifies against the web app.
GLOBAL_QUERY_PARAMS = ("page", "pageSize")

QUERY_ACCESSORS = (
    re.compile(r'\.Query\(\)\.Get\(\s*"([^"]+)"\s*\)'),
    re.compile(r'\.Query\(\)\[\s*"([^"]+)"\s*\]'),
    re.compile(r'route\.QueryInt\(\s*r\s*,\s*"([^"]+)"'),
    re.compile(r'route\.QueryOptionalBool\(\s*r\s*,\s*"([^"]+)"'),
    re.compile(r'route\.QueryOptionalString\(\s*r\s*,\s*"([^"]+)"'),
)

BOOL_QUERY_PARAMS = re.compile(r'route\.QueryOptionalBool\(\s*r\s*,\s*"([^"]+)"')

# Path parameters, framework parameters and protocol-fixed names are documented
# through a shared component so their description lives in one place.
QUERY_COMPONENTS = {
    "page": "Page",
    "pageSize": "PageSize",
    "sortField": "SortField",
    "sortOrder": "SortOrder",
    "keyword": "Keyword",
    "organizationId": "OrganizationID",
    "filter": "Filter",
}

# RFC 7644 §3.4.2 fixes the SCIM list parameter names. The SCIM handlers read
# startIndex/count through parseScimListParams (a dynamic accessor the
# literal-based index cannot see), so they are declared here as protocol-fixed.
PROTOCOL_QUERY_PARAMS = {
    "/scim/v2/Users": ("startIndex", "count", "filter", "sortBy", "sortOrder"),
    "/scim/v2/Groups": ("startIndex", "count", "filter", "sortBy", "sortOrder"),
}

# SCIM sortOrder is ascending/descending (RFC 7644 §3.4.2.3), not the REST
# asc/desc; sortBy is an attribute path, not a camelCase API field. The SCIM
# list operations must not reuse the REST components.
SCIM_QUERY_COMPONENTS = {
    "startIndex": "ScimStartIndex",
    "count": "ScimCount",
    "sortBy": "ScimSortBy",
    "sortOrder": "ScimSortOrder",
}


def query_component_for(pattern, name):
    if pattern.startswith("/scim/"):
        override = SCIM_QUERY_COMPONENTS.get(name)
        if override is not None:
            return override
    return QUERY_COMPONENTS.get(name)

_QUERY_INDEX_CACHE = {}


def query_param_index():
    """Map every registered pattern to the query parameters its handlers read.

    A handler is any non-test Go file under `internal/` that names the pattern as
    a literal, because a route is registered in one file and served by another.
    The union is taken, so a route split across a `routes.go`, a `chi.go` and a
    request mapper is still fully described.
    """
    if _QUERY_INDEX_CACHE:
        return _QUERY_INDEX_CACHE

    root = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "internal")
    sources = {}
    for dirpath, _dirs, files in os.walk(root):
        for name in files:
            if not name.endswith(".go") or name.endswith("_test.go"):
                continue
            with open(os.path.join(dirpath, name), encoding="utf-8") as handle:
                sources[os.path.join(dirpath, name)] = handle.read()

    patterns = set()
    for module in sorted(os.listdir(BASE_DIR)):
        routes_file = os.path.join(BASE_DIR, module, "routes.go")
        if not os.path.exists(routes_file):
            continue
        with open(routes_file, encoding="utf-8") as handle:
            patterns.update(re.findall(r'Pattern:\s*"([^"]+)"', handle.read()))

    for pattern in patterns:
        literal = f'"{pattern}"'
        params, booleans = set(GLOBAL_QUERY_PARAMS), set()
        for source in sources.values():
            if literal not in source:
                continue
            for accessor in QUERY_ACCESSORS:
                params.update(accessor.findall(source))
            booleans.update(BOOL_QUERY_PARAMS.findall(source))
        params.update(PROTOCOL_QUERY_PARAMS.get(pattern, ()))
        _QUERY_INDEX_CACHE[pattern] = (params, booleans)
    return _QUERY_INDEX_CACHE


def infer_request_schema(op):
    """Infer the request schema name for an operation."""
    mapping = {
        "CreateApplication": "CreateApplicationRequest",
        "PatchApplication": "PatchApplicationRequest",
        "ReplaceApplicationProviderBindings": "ReplaceApplicationProviderBindingsRequest",
        "PutApplicationClientCertificates": "ReplaceApplicationClientCertificatesRequest",
        "ImportApplication": "ImportApplicationRequest",
        "PostAuthLogin": "AuthLoginRequest",
        "PostAuthAuthorize": "AuthAuthorizeRequest",
        "PostAuthConsentCheck": "AuthConsentEvaluationRequest",
        "PostAuthConsentGrant": "AuthConsentRequest",
        "PostAuthConsentRevoke": "AuthConsentRequest",
        "PostAuthCaptchaVerify": "CaptchaVerifyRequest",
        "PostAuthResetPassword": "ResetPasswordRequest",
        "PostAuthSignup": "AuthSignupRequest",
        "PostAuthPasswordReset": "PasswordResetRequest",
        "PostAuthPasswordCheck": "PasswordCheckRequest",
        "PostAuthSocialLogin": "SocialLoginRequest",
        "PostAuthSocialAuthorize": "SocialAuthorizeRequest",
        "PostAuthNativeSsoComplete": "NativeSsoCompleteRequest",
        "PostAuthVerifyCode": "VerifyCodeRequest",
        "PostAuthQrcode": "QrcodeCreateRequest",
        "PostAuthQrcodeScan": "QrcodeScanRequest",
        "PostAuthQrcodeConfirm": "QrcodeConfirmRequest",
        "PostAuthQrcodeExchange": "QrcodeExchangeRequest",
        "PostAuthTokenExchange": "TokenExchangeRequest",
        "PostAuthJwtBearerToken": "JwtBearerTokenRequest",
        "PostAuthTokenExchangeRFC8693": "TokenExchangeRFC8693Request",
        "PostAuthTokenIntrospect": "TokenIntrospectRequest",
        "PostAuthTokenClientCredentials": "ClientCredentialsRequest",
        "PostAuthOAuth2Revoke": "OAuth2RevokeRequest",
        "PostAuthOAuth2Introspect": "OAuth2IntrospectRequest",
        "PostAuthBackchannelLogout": "BackchannelLogoutRequest",
        "PostAuthDPoPValidate": "DPoPValidateRequest",
        "PostAuthClientAuthenticate": "ClientAuthenticateRequest",
        "PostAuthDeviceAuthorize": "DeviceAuthorizeRequest",
        "PostAuthDeviceToken": "DeviceTokenRequest",
        "PostAuthDeviceVerify": "DeviceVerifyRequest",
        "PostAuthDeviceCancel": "DeviceCancelRequest",
        "PostAuthFaceId": "FaceIdInitiateRequest",
        "PostAuthFaceIdVerify": "FaceIdVerifyRequest",
        "PostAuthFaceIdComplete": "FaceIdCompleteRequest",
        "PostAuthWeb3": "Web3InitiateRequest",
        "PostAuthWeb3Verify": "Web3VerifyRequest",
        "PostAuthMfaPushInitiate": "MfaPushInitiateRequest",
        "PostAuthMfaPushVerify": "MfaPushVerifyRequest",
        "PostAuthMfaPushEnable": "MfaPushEnableRequest",
        "PostAuthMfaPushDisable": "MfaPushDisableRequest",
        "WebAuthnRegistrationBegin": "WebAuthnRegistrationBeginRequest",
        "WebAuthnRegistrationFinish": "WebAuthnRegistrationFinishRequest",
        "WebAuthnAuthenticationBegin": "WebAuthnAuthenticationBeginRequest",
        "WebAuthnAuthenticationFinish": "WebAuthnAuthenticationFinishRequest",
        "PostAuthCasLogout": "CasLogoutRequest",
        "PostAuthLoginLdap": "LdapLoginRequest",
        "CreateUser": "CreateUserRequest",
        "PatchUser": "PatchUserRequest",
        "UpdateUserProfile": "UpdateUserProfileRequest",
        "ReplaceUserRoleBindings": "ReplaceUserRoleBindingsRequest",
        "ReplaceUserGroupMemberships": "ReplaceUserGroupMembershipsRequest",
        "UpdateUserCredentials": "UpdateUserCredentialsRequest",
        "ReplaceUserIdentities": "ReplaceUserIdentitiesRequest",
        "ReplaceUserMfaMethods": "ReplaceUserMfaMethodsRequest",
        "UpdateUserPreferredMfa": "UpdateUserPreferredMfaRequest",
        "InitiateUserMfaSetup": "InitiateMfaSetupRequest",
        "VerifyUserMfaSetup": "VerifyMfaSetupRequest",
        "EnableUserMfaSetup": "EnableMfaSetupRequest",
        "ImpersonateUser": "ImpersonateUserRequest",
        # Real-name verification takes the SDK-collected metaInfo/returnUrl
        # body (management and self-service routes share one DTO); the OCR
        # recognize route is multipart.
        "VerifyUserIdentification": "VerifyIdentificationRequest",
        "VerifySelfIdentification": "VerifyIdentificationRequest",
        "RecognizeUserIdentityDocument": "RecognizeIdentityDocumentRequest",
        "UnlinkUserIdentity": "UnlinkIdentityRequest",
        "SetUserPassword": "SetUserPasswordRequest",
        "ImportUsers": "ImportUsersRequest",
        "CreateOrganization": "CreateOrganizationRequest",
        "PatchOrganization": "PatchOrganizationRequest",
        "UpdateOrganizationSettings": "UpdateOrganizationSettingsRequest",
        "UpdateOrganizationTheme": "UpdateOrganizationThemeRequest",
        "UpdateOrganizationSecurityPolicy": "UpdateOrganizationSecurityPolicyRequest",
        "CreateProvider": "CreateProviderRequest",
        "PatchProvider": "PatchProviderRequest",
        "UpdateProviderConfig": "UpdateProviderConfigRequest",
        "CreatePermission": "CreatePermissionRequest",
        "PatchPermission": "PatchPermissionRequest",
        "EnforcePermission": "EnforcePermissionRequest",
        "BatchEnforcePermission": "BatchEnforcePermissionRequest",
        "UpdatePermissionPolicy": "UpdatePermissionPolicyRequest",
        "BulkImportPermissions": "BulkImportPermissionsRequest",
        "CreateRole": "CreateRoleRequest",
        "PatchRole": "PatchRoleRequest",
        "BulkImportRoles": "BulkImportRolesRequest",
        "CreateGroup": "CreateGroupRequest",
        "PatchGroup": "PatchGroupRequest",
        "BulkImportGroups": "BulkImportGroupsRequest",
        "PatchSession": "PatchSessionRequest",
        "CreateToken": "CreateTokenRequest",
        "CreateEnforcer": "CreateEnforcerRequest",
        "PatchEnforcer": "PatchEnforcerRequest",
        "AddEnforcerPolicy": "AddEnforcerPolicyRequest",
        "UpdateEnforcerPolicy": "UpdateEnforcerPolicyRequest",
        "ImportPolicies": "ImportPoliciesRequest",
        "CreateAdapter": "CreateAdapterRequest",
        "PatchAdapter": "PatchAdapterRequest",
        "AddAdapterPolicy": "AddAdapterPolicyRequest",
        "UpdateAdapterPolicy": "UpdateAdapterPolicyRequest",
        "CreateModel": "CreateModelRequest",
        "PatchModel": "PatchModelRequest",
        "CreateSyncer": "CreateSyncerRequest",
        "PatchSyncer": "PatchSyncerRequest",
        "CreateWebhook": "CreateWebhookRequest",
        "PatchWebhook": "PatchWebhookRequest",
        "CreateLdap": "CreateLdapRequest",
        "PatchLdap": "PatchLdapRequest",
        "CreateCert": "CreateCertRequest",
        "PatchCert": "PatchCertRequest",
        "CreateKey": "CreateKeyRequest",
        "PatchKey": "PatchKeyRequest",
        "CreateAgent": "CreateAgentRequest",
        "PatchAgent": "PatchAgentRequest",
        "CreateForm": "CreateFormRequest",
        "PatchForm": "PatchFormRequest",
        "CreateGatewayServer": "CreateGatewayServerRequest",
        "UpdateGatewayServer": "UpdateGatewayServerRequest",
        "CreateGatewaySite": "CreateGatewaySiteRequest",
        "UpdateGatewaySite": "UpdateGatewaySiteRequest",
        "CreateInvitation": "CreateInvitationRequest",
        "CreateOrder": "CreateOrderRequest",
        "PatchOrder": "PatchOrderRequest",
        "PlaceOrder": "PlaceOrderRequest",
        "CreatePlan": "CreatePlanRequest",
        "PatchPlan": "PatchPlanRequest",
        "CreatePricing": "CreatePricingRequest",
        "PatchPricing": "PatchPricingRequest",
        "CreateProduct": "CreateProductRequest",
        "PatchProduct": "PatchProductRequest",
        "CreateSubscription": "CreateSubscriptionRequest",
        "PatchSubscription": "PatchSubscriptionRequest",
        "CreateCoupon": "CreateCouponRequest",
        "PatchCoupon": "PatchCouponRequest",
        "CreateResource": "CreateResourceRequest",
        "PatchResource": "PatchResourceRequest",
        "CreateRecord": "CreateRecordRequest",
        "SearchRecords": "SearchRecordsRequest",
        "CreateEntry": "CreateEntryRequest",
        "PatchEntry": "PatchEntryRequest",
        "CreateRule": "CreateRuleRequest",
        "PatchRule": "PatchRuleRequest",
        "CreateFaceIdProvider": "CreateFaceIdProviderRequest",
        "PatchFaceIdProvider": "PatchFaceIdProviderRequest",
        "CreateCaptchaProvider": "CreateCaptchaProviderRequest",
        "PatchCaptchaProvider": "PatchCaptchaProviderRequest",
        "CreateNotificationProvider": "CreateNotificationProviderRequest",
        "PatchNotificationProvider": "PatchNotificationProviderRequest",
        "CreateNotification": "CreateNotificationRequest",
        "CreateRadiusConfig": "CreateRadiusConfigRequest",
        "PatchRadiusConfig": "PatchRadiusConfigRequest",
        "CreateRadiusAccounting": "CreateRadiusAccountingRequest",
        "PatchRadiusAccounting": "PatchRadiusAccountingRequest",
        "SendVerificationCode": "SendVerificationCodeRequest",
        "VerifyVerificationCode": "VerifyVerificationCodeRequest",
        "CreateTicket": "CreateTicketRequest",
        "PatchTicket": "PatchTicketRequest",
        "AddTicketMessage": "AddTicketMessageRequest",
        "CreateTransaction": "CreateTransactionRequest",
        "PatchTransaction": "PatchTransactionRequest",
        "CreateMcpServer": "CreateMcpServerRequest",
        "PatchMcpServer": "PatchMcpServerRequest",
        "ScimCreateUser": "ScimCreateUserRequest",
        "ScimReplaceUser": "ScimReplaceUserRequest",
        "ScimPatchUser": "ScimPatchUserRequest",
        "ScimCreateGroup": "ScimCreateGroupRequest",
        "ScimReplaceGroup": "ScimReplaceGroupRequest",
        "ScimPatchGroup": "ScimPatchGroupRequest",
        "RegisterOAuthClient": "RegisterOAuthClientRequest",
        "UpdateOAuthClient": "UpdateOAuthClientRequest",
        "SamlAcs": "SamlAcsRequest",
        "SamlSpAcs": "SamlSpAcsRequest",
        "UpdateTranslations": "UpdateTranslationsRequest",
        "TranslateText": "TranslateTextRequest",
        "CreateLdapSyncConfig": "CreateSyncConfigRequest",
        "PatchLdapSyncConfig": "PatchSyncConfigRequest",
        "BatchUsers": "BatchRequest",
        "BatchOrganizations": "BatchRequest",
        "BatchApplications": "BatchRequest",
        "CreatePayment": "CreatePaymentRequest",
        "PatchPayment": "PatchPaymentRequest",
        "InvoicePayment": "InvoicePaymentRequest",
        "SendEmail": "SendEmailRequest",
        "SendSms": "SendSmsRequest",
        "SendNotification": "SendNotificationRequest",
        "StartRadiusServer": "StartRadiusServerRequest",
        "StopRadiusServer": "StopRadiusServerRequest",
        "KerberosNegotiate": "KerberosNegotiateRequest",
        "KerberosLogin": "KerberosLoginRequest",
        "PostAuthTrustedDeviceRegister": "TrustedDeviceRegisterRequest",
        "PostAuthStepUpBegin": "StepUpBeginRequest",
        "PostAuthStepUpFinish": "StepUpFinishRequest",
        "PostAuthRememberMeIssue": "RememberMeIssueRequest",
        "PostAuthRememberMeExchange": "RememberMeExchangeRequest",
        "PostAuthRememberMeRevoke": "RememberMeRevokeRequest",
        "PostPlatformUserTOTPAdminRebind": "AdminRebindTOTPRequest",
    }
    return mapping.get(op["operationId"], "")


def infer_response_schema(op, is_create):
    """Infer the response schema name for an operation."""
    auth_responses = {
        "PostAuthLogin": "AuthLoginResponse",
        "PostAuthLoginLdap": "AuthLoginResponse",
        "PostAuthAuthorize": "AuthAuthorizeResponse",
        "PostAuthConsentCheck": "AuthConsentEvaluationResponse",
        "PostAuthConsentGrant": "AuthConsentResponse",
        "PostAuthConsentRevoke": "AuthConsentResponse",
        "GetAuthAccount": "AuthAccountContext",
        "GetAuthAppLogin": "AppLoginResult",
        "GetAuthCaptcha": "CaptchaResponse",
        "GetAuthCaptchaStatus": "CaptchaStatusResponse",
        "PostAuthCaptchaVerify": "CaptchaVerifyResponse",
        "GetAuthContact": "ContactResponse",
        "PostAuthTokenExchange": "TokenResponse",
        "PostAuthJwtBearerToken": "TokenResponse",
        "PostAuthTokenExchangeRFC8693": "TokenResponse",
        "PostAuthTokenClientCredentials": "TokenResponse",
        "PostAuthDeviceToken": "TokenResponse",
        "PostAuthQrcode": "QrcodeCreateResponse",
        "GetAuthQrcodeStatus": "QrcodeStatusResponse",
        "PostAuthQrcodeExchange": "TokenResponse",
        "PostAuthFaceId": "FaceIdInitiateResponse",
        "PostAuthFaceIdVerify": "FaceIdVerifyResponse",
        "PostAuthFaceIdComplete": "AuthLoginResponse",
        "PostAuthWeb3": "Web3InitiateResponse",
        "PostAuthWeb3Verify": "AuthLoginResponse",
        "GetAuthPermissions": "AuthPermissionsResponse",
        "PostAuthTrustedDeviceBegin": "TrustedDeviceBeginResponse",
        "PostAuthTrustedDeviceRegister": "TrustedDeviceView",
        "GetAuthTrustedDevices": "TrustedDeviceListResponse",
        "ListUserTrustedDevices": "TrustedDeviceListResponse",
        "PostAuthStepUpBegin": "StepUpBeginResponse",
        "PostAuthStepUpFinish": "StepUpFinishResponse",
        "PostAuthRememberMeIssue": "RememberMeTokenResponse",
        "PostAuthRememberMeExchange": "RememberMeTokenResponse",
        "PostAuthDPoPValidate": "DPoPValidateResponse",
        "PostAuthClientAuthenticate": "ClientAuthenticateResponse",
        "PostAuthTokenIntrospect": "TokenIntrospectResponse",
        "PostAuthOAuth2Introspect": "TokenIntrospectResponse",
        "PostAuthDeviceAuthorize": "DeviceAuthorizeResponse",
        "GetAuthLoginChallenge": "LoginChallengeResponse",
        "PostAuthSocialAuthorize": "SocialAuthorizeResponse",
        "VerifyInvitation": "InvitationsVerifyResponse",
        "GetApplicationDeleteSummary": "ApplicationDeleteSummary",
        # Client credential lifecycle: create returns the one-time plaintext,
        # rotation returns the one-time plaintext plus the grace window.
        "CreateApplication": "CreateApplicationResponse",
        "PostApplicationClientSecretRotate": "RotateClientSecretResponse",
        # Client certificate bindings: the read projection never carries the PEM.
        "GetApplicationClientCertificates": "ApplicationClientCertificateList",
        # The application detail read is the only endpoint with the masked
        # client-secret projection; every other Get* stays an opaque ResourceDetail.
        "GetApplication": "ApplicationDetail",
        # Anonymous pre-login discovery: both routes serve the same closed
        # public projection (internal/app/applications/dto.go ApplicationPublicDetail).
        "GetApplicationByClientID": "ApplicationPublicDetail",
        "ResolveApplication": "ApplicationPublicDetail",
        # Cursor-paginated read: a `Get*` default would describe it as an opaque
        # object and a `List*` default as offset paginated with a total, and the
        # transcript has neither a page index nor a total (42.3.13 条 1).
        "GetEntryTranscript": "EntryTranscriptPage",
    }
    if op["operationId"] in auth_responses:
        return auth_responses[op["operationId"]]

    if op["operationId"].startswith("List") or op["operationId"].startswith("ScimList"):
        return "PaginatedResponse"

    if op["operationId"].startswith("Get"):
        return "ResourceDetail"

    return "OperationStatusResponse"


def generate_openapi(defs):
    """Generate the complete OpenAPI YAML specification."""
    lines = []

    # Header
    lines.append("openapi: 3.1.0")
    lines.append("info:")
    lines.append("  title: Guliang IAM API")
    lines.append(f"  version: {API_VERSION}")
    lines.append("  summary: Target-state IAM HTTP API; PostgreSQL is the default baseline")
    lines.append("  description: |")
    lines.append("    This specification documents all implemented endpoints.")
    lines.append("    Auto-generated from routes.go definitions to ensure 100% alignment.")
    lines.append(
        "    The info.version is the API contract version and is independent of the"
    )
    lines.append(
        "    product/release version; override it with GULIANGIAM_API_VERSION when needed."
    )
    lines.append(
        "    Database structure is declared in contracts/schema/*.yaml with version history"
    )
    lines.append(
        "    in contracts/schema/history/; DDL and versioned migrations are generated by"
    )
    lines.append(
        "    scripts/gen_schema.py under ADR-022. Route definitions are generated"
    )
    lines.append(
        "    from routes.go; field-level API schemas are governed by ADR-012."
    )
    lines.append("servers:")
    lines.append("  - url: /")
    lines.append("security:")
    lines.append("  - BearerAuth: []")
    lines.append("  - CookieSession: []")

    # Tags
    seen_tags = set()
    tag_lines = []
    for d in defs:
        tag = get_tag_name(d["module"], d["surface"])
        if tag not in seen_tags:
            seen_tags.add(tag)
            # Quoted: the face prefix contains ": ", which is not a valid plain
            # scalar in YAML.
            tag_lines.append(f'  - name: "{tag}"')
            tag_lines.append(f"    description: {get_tag_description(d['module'])}")

    lines.append("tags:")
    lines.extend(tag_lines)

    # Paths
    lines.append("paths:")

    # Group by path. A path+method pair must be declared exactly once: if two route
    # files declare it, only one chi.go handler is registered, so the other declaration
    # is dead contract that silently overwrites the first. Dedupe only to keep the
    # emitted mapping well-formed while the error below is raised.
    seen_ops = {}
    dropped = []
    path_ops = OrderedDict()
    for d in sorted(defs, key=lambda x: (x["pattern"], x["operationId"])):
        key = (d["pattern"], d["method"].lower())
        if key in seen_ops:
            dropped.append((key, seen_ops[key], d["operationId"]))
            continue
        seen_ops[key] = d["operationId"]
        path_ops.setdefault(d["pattern"], []).append(d)

    if dropped:
        print(
            f"ERROR: {len(dropped)} duplicate path+method declaration(s); the same route "
            f"is declared in more than one route file, so only one handler can ever be "
            f"registered and the rest is dead contract (手册 DB-055):",
            file=sys.stderr,
        )
        for (pattern, method), kept, skipped in dropped:
            print(
                f"  - {method.upper()} {pattern}: kept {kept}, dropped {skipped}",
                file=sys.stderr,
            )
        return None

    for pattern, ops in path_ops.items():
        lines.append(f"  {pattern}:")
        for op in ops:
            method = op["method"].lower()
            tag = get_tag_name(op["module"], op["surface"])
            lines.append(f"    {method}:")
            lines.append(f"      tags:")
            lines.append(f'        - "{tag}"')
            lines.append(f"      operationId: {op['operationId']}")
            lines.append(f"      summary: {op['summary']}")

            if op["permission"]:
                lines.append(f"      x-permission: {op['permission']}")

            # Path parameters
            path_params = extract_path_params(pattern)
            has_params = len(path_params) > 0

            if has_params:
                lines.append("      parameters:")
                for p in path_params:
                    lines.append(f"        - name: {p}")
                    lines.append("          in: path")
                    lines.append("          required: true")
                    lines.append("          schema:")
                    lines.append("            type: string")

            # Query parameters for list endpoints. Each declaration is derived from
            # the handlers that serve the pattern, so the spec cannot document a
            # parameter the server ignores (`query_param_index`).
            is_list = is_list_endpoint(op["operationId"], pattern)
            query_params, bool_params = query_param_index().get(pattern, (set(), set()))
            if is_list and method == "get":
                names = sorted(query_params, key=lambda n: (query_component_for(pattern, n) is None, n))
                if names and not has_params:
                    lines.append("      parameters:")
                for name in names:
                    component = query_component_for(pattern, name)
                    if component is not None:
                        lines.append(f"        - $ref: '#/components/parameters/{component}'")
                        continue
                    lines.append(f"        - name: {name}")
                    lines.append("          in: query")
                    lines.append("          required: false")
                    lines.append("          schema:")
                    lines.append("            type: boolean" if name in bool_params else "            type: string")

            # Request body for POST/PUT/PATCH
            if method in ("post", "put", "patch"):
                req_schema = infer_request_schema(op)
                if req_schema:
                    lines.append("      requestBody:")
                    lines.append("        required: true")
                    lines.append("        content:")
                    if ("upload" in pattern or "Upload" in op["operationId"]
                            or op["operationId"] in MULTIPART_FILE_ONLY_OPERATIONS):
                        lines.append("          multipart/form-data:")
                        lines.append("            schema:")
                        lines.append("              type: object")
                        lines.append("              required:")
                        lines.append("                - file")
                        lines.append("              properties:")
                        lines.append("                file:")
                        lines.append("                  type: string")
                        lines.append("                  format: binary")
                        if op["operationId"] not in MULTIPART_FILE_ONLY_OPERATIONS:
                            lines.append("                tag:")
                            lines.append("                  type: string")
                    else:
                        if op["operationId"] in FORM_URLENCODED_OPERATIONS:
                            lines.append("          application/x-www-form-urlencoded:")
                        else:
                            lines.append("          application/json:")
                        lines.append("            schema:")
                        lines.append(f"              $ref: '#/components/schemas/{req_schema}'")

            # Responses. The success status and headers are declared on the
            # route (route.Definition.SuccessStatus / ResponseHeaders); the
            # generator never infers them from the operationId or the method.
            # 204 and 3xx carry no JSON body by HTTP semantics; every other
            # status documents the schema the handler returns.
            lines.append("      responses:")
            status = op["successStatus"]
            headers = op["responseHeaders"]
            lines.append(f"        '{status}':")
            if status == 204:
                lines.append("          description: No Content.")
            elif 300 <= status < 400:
                lines.append(
                    f"          description: {REDIRECT_DESCRIPTIONS.get(status, 'Redirect response.')}"
                )
            else:
                if status == 201:
                    description = "Resource created."
                else:
                    description = op["summary"]
                lines.append(f"          description: {description}")
                lines.append("          content:")
                lines.append("            application/json:")
                lines.append("              schema:")
                if op["operationId"] in JWKS_OPERATIONS:
                    lines.append("                $ref: '#/components/schemas/JWKS'")
                elif (
                    ".well-known" in pattern
                    or "ServiceProviderConfig" in pattern
                    or "ResourceTypes" in pattern
                    or pattern.endswith("/Schemas")
                ):
                    lines.append("                type: object")
                    lines.append("                additionalProperties: true")
                else:
                    resp_schema = infer_response_schema(op, status == 201)
                    lines.append(
                        f"                $ref: '#/components/schemas/{resp_schema}'"
                    )

            if headers:
                lines.append("          headers:")
                for header in headers:
                    lines.append(f"            {header}:")
                    lines.append(
                        "              description: "
                        + HEADER_DESCRIPTIONS.get(
                            header, f"Value of the {header} response header."
                        )
                    )
                    lines.append("              schema:")
                    lines.append("                type: string")

            if method == "delete":
                lines.append("        '404':")
                lines.append("          $ref: '#/components/responses/ErrorResponse'")

            # Non-default error statuses declared on the route, ascending and
            # de-duplicated. The success status and the generic 500 are already
            # emitted above/below, so skip them to keep YAML keys unique.
            for error_status in op.get("errorResponses", []):
                if error_status in (status, 500):
                    continue
                lines.append(f"        '{error_status}':")
                lines.append("          $ref: '#/components/responses/ErrorResponse'")

            lines.append("        '500':")
            lines.append("          $ref: '#/components/responses/ErrorResponse'")

    # Components
    lines.append("")
    lines.append("components:")
    lines.append("  securitySchemes:")
    lines.append("    BearerAuth:")
    lines.append("      type: http")
    lines.append("      scheme: bearer")
    lines.append("      bearerFormat: JWT")
    lines.append("    CookieSession:")
    lines.append("      type: apiKey")
    lines.append("      in: cookie")
    lines.append("      name: session")
    lines.append("  parameters:")
    lines.append("    Page:")
    lines.append("      name: page")
    lines.append("      in: query")
    lines.append("      required: false")
    lines.append("      schema:")
    lines.append("        type: integer")
    lines.append("        minimum: 1")
    lines.append("        default: 1")
    lines.append("      description: Page number for pagination.")
    lines.append("    PageSize:")
    lines.append("      name: pageSize")
    lines.append("      in: query")
    lines.append("      required: false")
    lines.append("      schema:")
    lines.append("        type: integer")
    lines.append("        minimum: 1")
    lines.append("        maximum: 100")
    lines.append("        default: 20")
    lines.append(
        "      description: Number of items per page (default 20, maximum 100; "
        "a larger value is rejected with 422)."
    )
    lines.append("    SortField:")
    lines.append("      name: sortField")
    lines.append("      in: query")
    lines.append("      required: false")
    lines.append("      schema:")
    lines.append("        type: string")
    lines.append(
        "      description: API field name to sort by, camelCase; an unlisted field is "
        "rejected with 422. Every ORDER BY ends with the id tiebreaker."
    )
    lines.append("    SortOrder:")
    lines.append("      name: sortOrder")
    lines.append("      in: query")
    lines.append("      required: false")
    lines.append("      schema:")
    lines.append("        type: string")
    lines.append("        enum: [asc, desc]")
    lines.append("        default: asc")
    lines.append("      description: Sort direction.")
    lines.append("    Keyword:")
    lines.append("      name: keyword")
    lines.append("      in: query")
    lines.append("      required: false")
    lines.append("      schema:")
    lines.append("        type: string")
    lines.append(
        "      description: Free-text filter applied by every list handler. The name is "
        "`keyword` end to end — the console's search box sends it and the transport "
        "layer reads it; an alias such as `search` is read by nobody and turns the "
        "search box into a silent no-op."
    )
    lines.append("    OrganizationID:")
    lines.append("      name: organizationId")
    lines.append("      in: query")
    lines.append("      required: false")
    lines.append("      schema:")
    lines.append("        type: string")
    lines.append("        format: uuid")
    lines.append(
        "      description: Restrict the result to one organization. It narrows the "
        "row set only; tenant isolation is enforced by RLS and is never delegated to "
        "this parameter."
    )
    lines.append("    Filter:")
    lines.append("      name: filter")
    lines.append("      in: query")
    lines.append("      required: false")
    lines.append("      schema:")
    lines.append("        type: string")
    lines.append("      description: >-")
    lines.append("        SCIM filter expression (RFC 7644 §3.4.2.2); accepted only on")
    lines.append("        /scim/v2/* endpoints. Never interpolated into SQL as-is.")
    lines.append("    ScimStartIndex:")
    lines.append("      name: startIndex")
    lines.append("      in: query")
    lines.append("      required: false")
    lines.append("      schema:")
    lines.append("        type: integer")
    lines.append("        minimum: 1")
    lines.append("        default: 1")
    lines.append("      description: >-")
    lines.append("        1-based row offset (RFC 7644 §3.4.2.4), not a page number;")
    lines.append("        values below 1 are normalized to 1.")
    lines.append("    ScimCount:")
    lines.append("      name: count")
    lines.append("      in: query")
    lines.append("      required: false")
    lines.append("      schema:")
    lines.append("        type: integer")
    lines.append("        minimum: 0")
    lines.append("      description: >-")
    lines.append("        Desired page size (RFC 7644 §3.4.2.4); values below 1 are")
    lines.append("        normalized to 100, values above 100 are rejected with 400")
    lines.append("        scimType=tooMany.")
    lines.append("    ScimSortBy:")
    lines.append("      name: sortBy")
    lines.append("      in: query")
    lines.append("      required: false")
    lines.append("      schema:")
    lines.append("        type: string")
    lines.append("        default: id")
    lines.append("      description: >-")
    lines.append("        SCIM attribute path, case-insensitive (RFC 7643 §2.1).")
    lines.append("        Users whitelist: id, userName, userType, status, isAdmin,")
    lines.append("        isForbidden, score, lastSigninAt, createdAt, updatedAt,")
    lines.append("        meta.created, meta.lastModified. Groups whitelist: id,")
    lines.append("        displayName, createdAt, updatedAt, meta.created,")
    lines.append("        meta.lastModified. An unlisted path is rejected with 400")
    lines.append("        scimType=invalidValue. Every ORDER BY ends with the id")
    lines.append("        tiebreaker.")
    lines.append("    ScimSortOrder:")
    lines.append("      name: sortOrder")
    lines.append("      in: query")
    lines.append("      required: false")
    lines.append("      schema:")
    lines.append("        type: string")
    lines.append("        enum: [ascending, descending]")
    lines.append("        default: ascending")
    lines.append("      description: SCIM sort direction (RFC 7644 §3.4.2.3).")
    lines.append("    ResourceID:")
    lines.append("      name: id")
    lines.append("      in: path")
    lines.append("      required: true")
    lines.append("      schema:")
    lines.append("        type: string")
    lines.append("        format: uuid")
    lines.append("      description: Resource unique identifier.")
    lines.append("  responses:")
    lines.append("    ErrorResponse:")
    lines.append("      description: Application error response (see manual appendix C.1).")
    lines.append("      content:")
    lines.append("        application/json:")
    lines.append("          schema:")
    lines.append("            $ref: '#/components/schemas/ErrorResponse'")
    lines.append("    OAuthErrorResponse:")
    lines.append(
        "      description: OAuth 2.0 protocol error response (RFC 6749 section 5.2)."
    )
    lines.append("      content:")
    lines.append("        application/json:")
    lines.append("          schema:")
    lines.append("            $ref: '#/components/schemas/OAuthErrorResponse'")
    lines.append("  schemas:")
    lines.append("    ErrorResponse:")
    lines.append("      type: object")
    lines.append("      required:")
    lines.append("        - error")
    lines.append("        - requestId")
    lines.append("        - traceId")
    lines.append("      properties:")
    lines.append("        error:")
    lines.append("          type: object")
    lines.append("          required:")
    lines.append("            - code")
    lines.append("            - message")
    lines.append("          properties:")
    lines.append("            code:")
    lines.append("              type: string")
    lines.append("            message:")
    lines.append("              type: string")
    lines.append("            messageKey:")
    lines.append("              type: string")
    lines.append(
        "              description: Optional fine-grained i18n key (messageKey > code > message)."
    )
    lines.append("            details:")
    lines.append("              type: object")
    lines.append("              additionalProperties: true")
    lines.append(
        "              description: Structured field-level detail (canonical key); same payload as the deprecated meta alias."
    )
    lines.append("            meta:")
    lines.append("              type: object")
    lines.append("              additionalProperties: true")
    lines.append("              deprecated: true")
    lines.append(
        "              description: Deprecated compatibility alias of details; always carries the same value. Removed after the announced window."
    )
    lines.append("        requestId:")
    lines.append("          type: string")
    lines.append(
        "          description: Request correlation id echoed from X-Request-Id (generated when absent)."
    )
    lines.append("        traceId:")
    lines.append("          type: string")
    lines.append(
        "          description: W3C trace id; falls back to requestId when no span is recording."
    )
    lines.append("    OAuthErrorResponse:")
    lines.append("      type: object")
    lines.append("      required:")
    lines.append("        - error")
    lines.append("      properties:")
    lines.append("        error:")
    lines.append("          type: string")
    lines.append("        error_description:")
    lines.append("          type: string")
    lines.append("        error_uri:")
    lines.append("          type: string")
    lines.append("    OperationStatusResponse:")
    lines.append("      type: object")
    lines.append("      properties:")
    lines.append("        status:")
    lines.append("          type: string")
    lines.append("        msg:")
    lines.append("          type: string")
    lines.append("        data:")
    lines.append("          type: object")
    lines.append("          additionalProperties: true")
    lines.append("          description: Operation-specific details such as the IDV provider redirect URL.")
    lines.append("    CreateApplicationResponse:")
    lines.append("      type: object")
    lines.append("      required:")
    lines.append("        - id")
    lines.append("        - status")
    lines.append("      properties:")
    lines.append("        id:")
    lines.append("          type: string")
    lines.append("        status:")
    lines.append("          type: string")
    lines.append("        clientSecretPlain:")
    lines.append("          type: string")
    lines.append(
        "          description: One-time client secret plaintext; present only when the selected auth method issues a secret. Never returned by read endpoints."
    )
    lines.append("        clientSecretPrefix:")
    lines.append("          type: string")
    lines.append("          description: Non-secret display prefix (gl_cs_ plus six random characters).")
    lines.append("    RotateClientSecretResponse:")
    lines.append("      type: object")
    lines.append("      required:")
    lines.append("        - clientSecretPlain")
    lines.append("        - clientSecretPrefix")
    lines.append("        - rotatedAt")
    lines.append("        - graceUntil")
    lines.append("      properties:")
    lines.append("        clientSecretPlain:")
    lines.append("          type: string")
    lines.append(
        "          description: One-time new client secret plaintext; read endpoints expose only clientSecretSet/clientSecretMasked."
    )
    lines.append("        clientSecretPrefix:")
    lines.append("          type: string")
    lines.append("        rotatedAt:")
    lines.append("          type: string")
    lines.append("          format: date-time")
    lines.append("        graceUntil:")
    lines.append("          type: string")
    lines.append("          format: date-time")
    lines.append(
        "          description: End of the acceptance window for the previous secret (default 24h, configurable)."
    )
    # Mirrors presenter.ListResponse[T]: {"items": [...], "page": {page, pageSize, total}}
    lines.append("    PaginatedResponse:")
    lines.append("      type: object")
    lines.append("      required:")
    lines.append("        - items")
    lines.append("        - page")
    lines.append("      properties:")
    lines.append("        items:")
    lines.append("          type: array")
    lines.append("          items:")
    lines.append("            type: object")
    lines.append("            additionalProperties: true")
    lines.append("        page:")
    lines.append("          $ref: '#/components/schemas/PageMeta'")
    # Read-side credential projection. The plaintext (and the hash) is deliberately
    # absent: it appears only once in CreateApplicationResponse / RotateClientSecretResponse.
    # Only the application detail response carries ClientSecretSet / ClientSecretMasked
    # (internal/app/applications/dto.go ApplicationDetail), so the fields live on a
    # dedicated schema rather than on the shared opaque ResourceDetail.
    lines.append("    ResourceDetail:")
    lines.append("      type: object")
    lines.append("      additionalProperties: true")
    lines.append("    ApplicationDetail:")
    lines.append("      type: object")
    lines.append("      properties:")
    lines.append("        clientSecretSet:")
    lines.append("          type: boolean")
    lines.append(
        "          description: Whether a client secret is configured for this application; never the secret itself."
    )
    lines.append("        clientSecretMasked:")
    lines.append("          type: string")
    lines.append(
        "          description: Masked display value (secret prefix plus bullets) of the configured client secret; never the plaintext or its hash."
    )
    # Public-key projection of app_client_jwks. The server serializes a fixed
    # whitelist (kty/kid/use/alg/n/e/crv/x/y); private members are never emitted.
    lines.append("        clientJwks:")
    lines.append("          type: object")
    lines.append("          description: Read-only public projection of the client JWKS registered for private_key_jwt; private key members are never returned.")
    lines.append("          required:")
    lines.append("            - kids")
    lines.append("            - keys")
    lines.append("          properties:")
    lines.append("            kids:")
    lines.append("              type: array")
    lines.append("              items:")
    lines.append("                type: string")
    lines.append("            keys:")
    lines.append("              type: array")
    lines.append("              items:")
    lines.append("                type: object")
    lines.append("                additionalProperties: false")
    lines.append("                required:")
    lines.append("                  - kty")
    lines.append("                  - kid")
    lines.append("                properties:")
    for member in ("kty", "kid", "use", "alg", "n", "e", "crv", "x", "y"):
        lines.append(f"                  {member}:")
        lines.append("                    type: string")
    # Remote JWKS location for private_key_jwt (app_client_jwks_uris). Echoes the
    # write-side jwksUri field; absent when the client uses inline JWKS only.
    lines.append("        jwksUri:")
    lines.append("          type: string")
    lines.append("          format: uri")
    lines.append(
        "          description: Remote JWKS location configured for private_key_jwt; absent when only inline JWKS is registered."
    )
    lines.append("      additionalProperties: true")
    # Mirrors app/entries.TranscriptPage: append-order keyset read, so there is a
    # cursor instead of a page index, and no total (a total would have to be
    # recomputed on every read and could disagree with the sequence).
    lines.append("    EntryTranscriptPage:")
    lines.append("      type: object")
    lines.append("      required:")
    lines.append("        - messages")
    lines.append("      properties:")
    lines.append("        messages:")
    lines.append("          type: array")
    lines.append("          items:")
    lines.append("            type: object")
    lines.append("            additionalProperties: true")
    lines.append("        nextCursor:")
    lines.append("          type: string")
    lines.append("          description: Opaque cursor for the next page; absent at the end of the sequence.")
    lines.append("        redactedFields:")
    lines.append("          type: array")
    lines.append("          items:")
    lines.append("            type: string")
    lines.append("    PageMeta:")
    lines.append("      type: object")
    lines.append("      properties:")
    lines.append("        page:")
    lines.append("          type: integer")
    lines.append("        pageSize:")
    lines.append("          type: integer")
    lines.append("        total:")
    lines.append("          type: integer")

    # Add all request/response schemas referenced by the paths
    schemas = generate_component_schemas()
    lines.extend(schemas)

    return "\n".join(lines)


def generate_component_schemas():
    """Generate all component schemas."""
    lines = []
    
    # All schemas used across the API
    schema_defs = {
        # JWKS is published by platform/jwks.Manager: RSA keys carry n/e, SM2
        # keys carry crv=SM2 plus fixed 32-octet x/y coordinates, and no private
        # parameter is ever serialized.
        "ECJWK": """      type: object
      required:
        - kty
        - use
        - kid
        - alg
        - crv
        - x
        - y
      properties:
        kty:
          type: string
          enum:
            - EC
        use:
          type: string
          enum:
            - sig
        kid:
          type: string
        alg:
          type: string
          enum:
            - SM2-SM3
        crv:
          type: string
          enum:
            - SM2
        x:
          type: string
          description: Base64url-encoded SM2 x coordinate; always 32 octets (RFC 7518 §6.2.1.2).
        y:
          type: string
          description: Base64url-encoded SM2 y coordinate; always 32 octets (RFC 7518 §6.2.1.2).
      additionalProperties: false""",
        "RSAJWK": """      type: object
      required:
        - kty
        - use
        - kid
        - alg
        - n
        - e
      properties:
        kty:
          type: string
          enum:
            - RSA
        use:
          type: string
          enum:
            - sig
        kid:
          type: string
        alg:
          type: string
          enum:
            - RS256
        n:
          type: string
          description: Base64url-encoded RSA modulus.
        e:
          type: string
          description: Base64url-encoded RSA public exponent.
      additionalProperties: false""",
        "JWK": """      oneOf:
        - $ref: '#/components/schemas/RSAJWK'
        - $ref: '#/components/schemas/ECJWK'""",
        "JWKS": """      type: object
      required:
        - keys
      properties:
        keys:
          type: array
          items:
            $ref: '#/components/schemas/JWK'
      additionalProperties: false""",
        "AuthLoginRequest": """      type: object
      description: >-
        Login request document. The first factor is either password, a
        one-time verification code (verificationCode + verificationType) or an
        issued MFA challenge (challengeId + passcode/recoveryCode); the
        application layer rejects invalid combinations. captchaToken/captchaId
        are required when the organization enables a captcha provider.
      properties:
        organizationSlug:
          type: string
        username:
          type: string
        password:
          type: string
          format: password
        applicationId:
          type: string
        challengeId:
          type: string
        mfaType:
          type: string
        passcode:
          type: string
        recoveryCode:
          type: string
        verificationCode:
          type: string
        verificationType:
          type: string
          enum:
            - email
            - sms
        captchaToken:
          type: string
        captchaId:
          type: string
        rememberMeToken:
          type: string
        enableMfaRemember:
          type: boolean
        responseType:
          type: string
        redirectUri:
          type: string
        state:
          type: string
        nonce:
          type: string
        scope:
          type: array
          items:
            type: string
        codeChallenge:
          type: string
        codeChallengeMethod:
          type: string
        resource:
          type: string
        samlRequest:
          type: string
        relayState:
          type: string
        casService:
          type: string""",
        "AuthLoginResponse": """      type: object
      description: >-
        Login result document. status discriminates the three outcomes:
        success (account context + optional protocol artifacts), mfa_required
        (challenge fields + available methods) and required_mfa_setup.
      required:
        - status
        - needUpdatePassword
      properties:
        status:
          type: string
          enum:
            - success
            - mfa_required
            - required_mfa_setup
        account:
          type: object
          properties:
            id:
              type: string
            owner:
              type: string
            name:
              type: string
            displayName:
              type: string
            avatar:
              type: string
            isAdmin:
              type: boolean
        organization:
          type: object
          properties:
            id:
              type: string
            name:
              type: string
            displayName:
              type: string
            logoUrl:
              type: string
            faviconUrl:
              type: string
            navItems:
              type: array
              items:
                type: string
            userNavItems:
              type: array
              items:
                type: string
            navGroupOrder:
              type: array
              items:
                type: string
        accessToken:
          type: string
        passwordPolicy:
          type: object
          properties:
            minLength:
              type: integer
            maxLength:
              type: integer
            requireUppercase:
              type: boolean
            requireLowercase:
              type: boolean
            requireDigit:
              type: boolean
            requireSpecial:
              type: boolean
            rejectCommonPasswords:
              type: boolean
            rejectUsername:
              type: boolean
            passwordHistoryCount:
              type: integer
            passwordMaxAgeDays:
              type: integer
        sessionPolicy:
          type: object
          properties:
            idleTimeoutMinutes:
              type: integer
        needUpdatePassword:
          type: boolean
        deviceInfo:
          type: object
          additionalProperties: true
        nextMfa:
          type: string
        challengeId:
          type: string
        challengeState:
          type: string
        challengeExpiresAt:
          type: string
          format: date-time
        requiredMfa:
          type: array
          items:
            type: string
        availableMfaMethods:
          type: array
          items:
            type: object
            properties:
              id:
                type: string
              methodType:
                type: string
              displayName:
                type: string
              target:
                type: string
              countryCode:
                type: string
        code:
          type: string
        redirectTo:
          type: string
        samlResponse:
          type: string
        relayState:
          type: string
        casTicket:
          type: string
        idToken:
          type: string
        tokenType:
          type: string
        expiresIn:
          type: integer
          format: int64
        refreshToken:
          type: string
        passwordExpired:
          type: boolean
        passwordExpiresAt:
          type: string
        rememberMeToken:
          type: string
        rememberMeExpiresAt:
          type: string
          format: date-time""",
        "AuthAuthorizeRequest": """      type: object
      required:
        - client_id
        - redirect_uri
        - response_type
      properties:
        client_id:
          type: string
        redirect_uri:
          type: string
        response_type:
          type: string
        scope:
          type: string
        state:
          type: string
        code_challenge:
          type: string
        code_challenge_method:
          type: string""",
        "AuthAuthorizeResponse": """      type: object
      properties:
        code:
          type: string
        redirect_uri:
          type: string
        consent_required:
          type: boolean""",
        "AuthConsentEvaluationRequest": """      type: object
      required:
        - client_id
        - scope
      properties:
        client_id:
          type: string
        scope:
          type: string""",
        "AuthConsentEvaluationResponse": """      type: object
      properties:
        consent_required:
          type: boolean
        scopes:
          type: array
          items:
            type: string""",
        "AuthConsentRequest": """      type: object
      required:
        - client_id
        - scope
      properties:
        client_id:
          type: string
        scope:
          type: string""",
        "AuthConsentResponse": """      type: object
      properties:
        status:
          type: string
        scope:
          type: string""",
        "AuthAccountContext": """      type: object
      properties:
        userId:
          type: string
        username:
          type: string
        organizationId:
          type: string
        organizationSlug:
          type: string
        applicationId:
          type: string
        permissions:
          type: array
          items:
            type: string
        isAdmin:
          type: boolean
        isOwner:
          type: boolean
        needUpdatePassword:
          type: boolean""",
        "AppLoginResult": """      type: object
      properties:
        organizationSlug:
          type: string
        applicationSlug:
          type: string
        providers:
          type: array
          items:
            type: object
            properties:
              id:
                type: string
              type:
                type: string
              name:
                type: string
        captchaRequired:
          type: boolean
        faceIdEnabled:
          type: boolean
        web3Enabled:
          type: boolean""",
        "CaptchaResponse": """      type: object
      properties:
        captchaId:
          type: string
        captchaImage:
          type: string
        type:
          type: string
        shape:
          type: string
        captchaImageSub:
          type: string""",
        "CaptchaStatusResponse": """      type: object
      properties:
        enabled:
          type: boolean""",
        "CaptchaVerifyRequest": """      type: object
      required:
        - captchaId
        - answer
      properties:
        captchaId:
          type: string
        answer:
          type: string""",
        "VerifyIdentificationRequest": """      type: object
      properties:
        metaInfo:
          type: string
        country:
          type: string
        returnUrl:
          type: string
        action:
          type: string""",
        "CaptchaVerifyResponse": """      type: object
      properties:
        verified:
          type: boolean""",
        "ContactResponse": """      type: object
      properties:
        email:
          type: string
        phone:
          type: string""",
        "TokenResponse": """      type: object
      properties:
        access_token:
          type: string
        token_type:
          type: string
        expires_in:
          type: integer
        refresh_token:
          type: string
        scope:
          type: string
        issued_token_type:
          type: string""",
        "AuthSignupRequest": """      type: object
      required:
        - username
        - password
        - organizationSlug
      properties:
        username:
          type: string
        password:
          type: string
          format: password
        email:
          type: string
        phone:
          type: string
        organizationSlug:
          type: string
        signupAppId:
          type: string
        verificationCode:
          type: string
        invitationCode:
          type: string
        captchaToken:
          type: string
        captchaId:
          type: string""",
        "NativeSsoCompleteRequest": """      type: object
      required:
        - providerId
        - code
      properties:
        providerId:
          type: string
        code:
          type: string
        state:
          type: string
        redirectUri:
          type: string
        organizationSlug:
          type: string
        applicationId:
          type: string""",
        "TokenExchangeRequest": """      type: object
      required:
        - grant_type
        - code
      properties:
        grant_type:
          type: string
        code:
          type: string
        redirect_uri:
          type: string
        client_id:
          type: string
        code_verifier:
          type: string""",
        "JwtBearerTokenRequest": """      type: object
      required:
        - grant_type
        - assertion
      properties:
        grant_type:
          type: string
        assertion:
          type: string
        scope:
          type: string""",
        "TokenExchangeRFC8693Request": """      type: object
      required:
        - grant_type
        - subject_token
        - subject_token_type
      properties:
        grant_type:
          type: string
        subject_token:
          type: string
        subject_token_type:
          type: string
        actor_token:
          type: string
        actor_token_type:
          type: string
        resource:
          type: string
        audience:
          type: string
        scope:
          type: string""",
        "TokenIntrospectRequest": """      type: object
      required:
        - token
      properties:
        token:
          type: string
        token_type_hint:
          type: string""",
        "TokenIntrospectResponse": """      type: object
      properties:
        active:
          type: boolean
        scope:
          type: string
        client_id:
          type: string
        username:
          type: string
        token_type:
          type: string
        exp:
          type: integer
        iat:
          type: integer
        sub:
          type: string
        aud:
          type: string
        iss:
          type: string""",
        "ClientCredentialsRequest": """      type: object
      required:
        - grant_type
        - client_id
        - client_secret
      properties:
        grant_type:
          type: string
        client_id:
          type: string
        client_secret:
          type: string
        scope:
          type: string""",
        "OAuth2RevokeRequest": """      type: object
      required:
        - token
      properties:
        token:
          type: string
        token_type_hint:
          type: string""",
        "OAuth2IntrospectRequest": """      type: object
      required:
        - token
      properties:
        token:
          type: string
        token_type_hint:
          type: string""",
        "BackchannelLogoutRequest": """      type: object
      required:
        - logout_token
      properties:
        logout_token:
          type: string""",
        "DPoPValidateRequest": """      type: object
      required:
        - dpop_proof
        - htm
        - htu
      properties:
        dpop_proof:
          type: string
        htm:
          type: string
        htu:
          type: string
        ath:
          type: string""",
        "DPoPValidateResponse": """      type: object
      properties:
        valid:
          type: boolean
        jkt:
          type: string
        message:
          type: string""",
        "ClientAuthenticateRequest": """      type: object
      required:
        - client_id
      properties:
        client_id:
          type: string
        client_secret:
          type: string
        method:
          type: string
          enum:
            - client_secret_basic
            - client_secret_post
            - private_key_jwt
            - none""",
        "ClientAuthenticateResponse": """      type: object
      properties:
        authenticated:
          type: boolean
        clientId:
          type: string""",
        "DeviceAuthorizeRequest": """      type: object
      required:
        - client_id
      properties:
        client_id:
          type: string
        scope:
          type: string""",
        "DeviceAuthorizeResponse": """      type: object
      properties:
        device_code:
          type: string
        user_code:
          type: string
        verification_uri:
          type: string
        verification_uri_complete:
          type: string
        expires_in:
          type: integer
        interval:
          type: integer""",
        "DeviceTokenRequest": """      type: object
      required:
        - grant_type
        - device_code
      properties:
        grant_type:
          type: string
        device_code:
          type: string
        client_id:
          type: string""",
        "DeviceVerifyRequest": """      type: object
      required:
        - user_code
      properties:
        user_code:
          type: string""",
        "DeviceCancelRequest": """      type: object
      required:
        - device_code
      properties:
        device_code:
          type: string""",
        "FaceIdInitiateRequest": """      type: object
      required:
        - organizationSlug
        - username
      properties:
        organizationSlug:
          type: string
        username:
          type: string
        applicationId:
          type: string""",
        "FaceIdInitiateResponse": """      type: object
      properties:
        sessionId:
          type: string
        challenge:
          type: string
        expiresIn:
          type: integer
          format: int64""",
        "FaceIdVerifyRequest": """      type: object
      required:
        - sessionId
        - imageData
      properties:
        sessionId:
          type: string
        imageData:
          type: string
        deviceInfo:
          type: string""",
        "FaceIdVerifyResponse": """      type: object
      properties:
        verified:
          type: boolean
        livenessPassed:
          type: boolean""",
        "FaceIdCompleteRequest": """      type: object
      required:
        - sessionId
      properties:
        sessionId:
          type: string
        applicationId:
          type: string
        responseType:
          type: string
        redirectUri:
          type: string
        state:
          type: string
        nonce:
          type: string
        scope:
          type: array
          items:
            type: string""",
        "Web3InitiateRequest": """      type: object
      properties:
        organizationSlug:
          type: string
        applicationId:
          type: string""",
        "Web3InitiateResponse": """      type: object
      properties:
        challengeId:
          type: string
        message:
          type: string""",
        "Web3VerifyRequest": """      type: object
      required:
        - challengeId
        - signature
      properties:
        challengeId:
          type: string
        signature:
          type: string""",
        "QrcodeCreateRequest": """      type: object
      properties:
        organizationSlug:
          type: string
        applicationId:
          type: string""",
        "QrcodeCreateResponse": """      type: object
      properties:
        codeId:
          type: string
        qrcodeUrl:
          type: string""",
        "QrcodeScanRequest": """      type: object
      required:
        - codeId
      properties:
        codeId:
          type: string""",
        "QrcodeConfirmRequest": """      type: object
      required:
        - codeId
      properties:
        codeId:
          type: string""",
        "QrcodeExchangeRequest": """      type: object
      required:
        - codeId
      properties:
        codeId:
          type: string""",
        "QrcodeStatusResponse": """      type: object
      properties:
        status:
          type: string
        codeId:
          type: string""",
        "LoginChallengeResponse": """      type: object
      properties:
        challengeId:
          type: string
        type:
          type: string
        status:
          type: string""",
        "SocialAuthorizeResponse": """      type: object
      properties:
        redirectUrl:
          type: string""",
        "AuthPermissionsResponse": """      type: object
      properties:
        permissions:
          type: array
          items:
            type: string""",
        "ResetPasswordRequest": """      type: object
      required:
        - verificationCode
        - newPassword
      properties:
        organizationSlug:
          type: string
        username:
          type: string
        verificationCode:
          type: string
        newPassword:
          type: string
        captchaToken:
          type: string
          description: Human-check response when the organization enables a captcha provider.
        captchaId:
          type: string
          description: Server-issued captcha challenge id for server-generated captcha providers.""",
        "PasswordResetRequest": """      type: object
      required:
        - verificationCode
        - newPassword
      properties:
        organizationSlug:
          type: string
        username:
          type: string
        verificationCode:
          type: string
        newPassword:
          type: string
        captchaToken:
          type: string
          description: Human-check response when the organization enables a captcha provider.
        captchaId:
          type: string
          description: Server-issued captcha challenge id for server-generated captcha providers.""",
        "PasswordCheckRequest": """      type: object
      required:
        - password
      properties:
        password:
          type: string""",
        "SocialLoginRequest": """      type: object
      required:
        - providerId
        - code
      properties:
        providerId:
          type: string
        code:
          type: string
        state:
          type: string
        codeVerifier:
          type: string
        redirectUri:
          type: string""",
        "SocialAuthorizeRequest": """      type: object
      required:
        - providerId
      properties:
        providerId:
          type: string
        redirectUri:
          type: string""",
        "LdapLoginRequest": """      type: object
      required:
        - organizationSlug
        - username
        - password
      properties:
        organizationSlug:
          type: string
        username:
          type: string
        password:
          type: string
          format: password
        ldapId:
          type: string
        applicationId:
          type: string
        captchaToken:
          type: string
        captchaId:
          type: string
        responseType:
          type: string
        redirectUri:
          type: string
        state:
          type: string
        nonce:
          type: string
        scope:
          type: array
          items:
            type: string""",
        "VerifyCodeRequest": """      type: object
      required:
        - dest
        - type
      properties:
        dest:
          type: string
        type:
          type: string
          enum:
            - email
            - sms""",
        "MfaPushInitiateRequest": """      type: object
      properties:
        mfaType:
          type: string""",
        "MfaPushVerifyRequest": """      type: object
      required:
        - challengeId
        - response
      properties:
        challengeId:
          type: string
        response:
          type: string""",
        "MfaPushEnableRequest": """      type: object
      properties:
        mfaType:
          type: string""",
        "MfaPushDisableRequest": """      type: object
      properties:
        mfaType:
          type: string""",
        "WebAuthnRegistrationBeginRequest": """      type: object
      required:
        - organizationSlug
        - username
      properties:
        organizationSlug:
          type: string
        username:
          type: string""",
        "WebAuthnRegistrationFinishRequest": """      type: object
      required:
        - organizationSlug
        - username
        - challengeId
        - credentialId
        - publicKey
      properties:
        organizationSlug:
          type: string
        username:
          type: string
        challengeId:
          type: string
        challenge:
          type: string
        credentialId:
          type: string
        publicKey:
          type: string
        clientDataJSON:
          type: string
        authenticatorData:
          type: string
        attestationObject:
          type: string
        attestationType:
          type: string
        transports:
          type: array
          items:
            type: string""",
        "WebAuthnAuthenticationBeginRequest": """      type: object
      required:
        - organizationSlug
        - username
      properties:
        organizationSlug:
          type: string
        username:
          type: string""",
        "WebAuthnAuthenticationFinishRequest": """      type: object
      required:
        - organizationSlug
        - username
        - challengeId
        - credentialId
        - signature
      properties:
        organizationSlug:
          type: string
        username:
          type: string
        challengeId:
          type: string
        challenge:
          type: string
        credentialId:
          type: string
        signature:
          type: string
        authenticatorData:
          type: string
        clientDataJSON:
          type: string
        applicationId:
          type: string
        responseType:
          type: string
        redirectUri:
          type: string
        state:
          type: string
        nonce:
          type: string
        scope:
          type: array
          items:
            type: string""",
        "CasLogoutRequest": """      type: object
      properties:
        logoutRequest:
          type: string""",
        "ApplicationDeleteSummary": """      type: object
      properties:
        userCount:
          type: integer
        sessionCount:
          type: integer
        tokenCount:
          type: integer""",
        "ApplicationPublicDetail": """      type: object
      description: >-
        Anonymous pre-login discovery projection shared by
        /api/v1/applications/by-client-id/{clientId} and /api/v1/applications/resolve.
        Carries branding, behavior switches and UI customization (CSS/HTML are
        sanitized client-side; script execution is not supported). SAML material,
        callback allowlists, internal override URLs, transport configuration,
        token policy and client secret material are never present.
      additionalProperties: false
      properties:
        id:
          type: string
        organizationId:
          type: string
        slug:
          type: string
        name:
          type: string
        displayName:
          type: string
        description:
          type: string
        homepageUrl:
          type: string
        clientId:
          type: string
        logo:
          type: string
        title:
          type: string
        favicon:
          type: string
        isShared:
          type: boolean
        enableAutoSignin:
          type: boolean
        enableSignup:
          type: boolean
        forcedRedirectOrigin:
          type: string
        signupUrl:
          type: string
        forgetUrl:
          type: string
        termsOfUse:
          type: string
        privacyPolicy:
          type: string
        orgChoiceMode:
          type: string
        signinHtml:
          type: string
        signupHtml:
          type: string
        headerHtml:
          type: string
        footerHtml:
          type: string
        customCss:
          type: string
        customHeader:
          type: string
        formCss:
          type: string
        formCssMobile:
          type: string
        formSideHtml:
          type: string
        formBackgroundUrl:
          type: string
        formBackgroundUrlMobile:
          type: string
        formOffset:
          type: string
        signinMethods:
          type: array
          items:
            type: string
        signinItems:
          type: array
          items:
            type: object
            additionalProperties: true
        signupItems:
          type: array
          items:
            type: object
            additionalProperties: true""",
        "CreateUserRequest": """      type: object
      required:
        - username
        - password
      properties:
        username:
          type: string
        password:
          type: string
          format: password
        email:
          type: string
        phone:
          type: string
        displayName:
          type: string
        organizationId:
          type: string
          format: uuid""",
        "PatchUserRequest": """      type: object
      properties:
        displayName:
          type: string
        email:
          type: string
        phone:
          type: string
        enabled:
          type: boolean
        locked:
          type: boolean""",
        "UpdateUserProfileRequest": """      type: object
      properties:
        firstName:
          type: string
        lastName:
          type: string
        bio:
          type: string
        avatarUrl:
          type: string""",
        "ReplaceUserRoleBindingsRequest": """      type: object
      properties:
        roleBindings:
          type: array
          items:
            type: object
            properties:
              roleId:
                type: string""",
        "ReplaceUserGroupMembershipsRequest": """      type: object
      properties:
        groupIds:
          type: array
          items:
            type: string""",
        "UpdateUserCredentialsRequest": """      type: object
      properties:
        password:
          type: string
          format: password""",
        "ReplaceUserIdentitiesRequest": """      type: object
      properties:
        identities:
          type: array
          items:
            type: object""",
        "ReplaceUserMfaMethodsRequest": """      type: object
      required:
        - revision
        - items
      properties:
        revision:
          type: integer
          format: int64
          description: Expected users.revision from the read API; a stale or missing value is rejected with 409.
        items:
          type: array
          items:
            type: object
            required:
              - methodType
            properties:
              id:
                type: string
                description: Existing factor id returned by the read API; omit it for a new factor.
              methodType:
                type: string
                enum:
                  - app
                  - totp
                  - email
                  - sms
                  - radius
                  - push
                description: Factor type. app and totp are aliases; at most one factor per normalized type.
              enabled:
                type: boolean
                default: false
                description: Whether the factor is enabled. The server rejects enabling a factor that has not completed the setup ceremony.
              displayName:
                type: string
                description: Human-readable label stored with the factor.""",
        "UpdateUserPreferredMfaRequest": """      type: object
      properties:
        preferredMfa:
          type: string""",
        "InitiateMfaSetupRequest": """      type: object
      properties:
        mfaType:
          type: string""",
        "VerifyMfaSetupRequest": """      type: object
      properties:
        mfaType:
          type: string
        code:
          type: string""",
        "EnableMfaSetupRequest": """      type: object
      required:
        - passcode
      properties:
        mfaType:
          type: string
        passcode:
          type: string
          description: Proof-of-possession verified server-side before the factor is enabled.""",
        "ImpersonateUserRequest": """      type: object
      properties: {}""",
        "UnlinkIdentityRequest": """      type: object
      properties:
        providerId:
          type: string""",
        "SetUserPasswordRequest": """      type: object
      required:
        - password
      properties:
        password:
          type: string
          format: password
        reason:
          type: string
          maxLength: 128""",
        "ImportUsersRequest": """      type: object
      properties:
        users:
          type: array
          items:
            type: object""",
        "CreateOrganizationRequest": """      type: object
      required:
        - name
      properties:
        name:
          type: string
        slug:
          type: string
        description:
          type: string""",
        "PatchOrganizationRequest": """      type: object
      properties:
        name:
          type: string
        description:
          type: string
        enabled:
          type: boolean""",
        "UpdateOrganizationSettingsRequest": """      type: object
      properties:
        settings:
          type: object
          additionalProperties: true""",
        "UpdateOrganizationThemeRequest": """      type: object
      properties:
        theme:
          type: object
          additionalProperties: true""",
        # The policy maps are stored as free-form JSONB: these are the keys read by
        # app/auth (password, expiry, MFA) and app/sessionpolicy, while unknown keys
        # are accepted and persisted, so every policy object stays open.
        "UpdateOrganizationSecurityPolicyRequest": """      type: object
      properties:
        passwordPolicy:
          type: object
          additionalProperties: true
          properties:
            minLength:
              type: integer
              minimum: 1
            maxLength:
              type: integer
            requireUppercase:
              type: boolean
            requireLowercase:
              type: boolean
            requireDigit:
              type: boolean
            requireSpecial:
              type: boolean
            rejectCommonPasswords:
              type: boolean
            rejectUsername:
              type: boolean
            passwordHistoryCount:
              type: integer
            passwordMaxAgeDays:
              type: integer
            passwordExpiryEnabled:
              type: boolean
            passwordExpiryDays:
              type: integer
            passwordExpiryWarningDays:
              type: integer
            forceChangeOnExpiry:
              type: boolean
        passwordDeriveAlgorithm:
          type: string
          enum:
            - hmac_sha256
            - hmac_sm3
        mfaPolicy:
          type: object
          additionalProperties: true
          properties:
            required:
              type: boolean
        ipPolicy:
          type: object
          additionalProperties: true
        sessionPolicy:
          type: object
          additionalProperties: true
          properties:
            idleTimeoutMinutes:
              type: integer
              minimum: 1
            bindIp:
              type: boolean
            bindUserAgent:
              type: boolean
            mfaRememberHours:
              type: integer
            mfaRememberInHours:
              type: integer
              deprecated: true
        riskPolicy:
          type: object
          additionalProperties: true""",
        "CreateProviderRequest": """      type: object
      required:
        - type
        - name
      properties:
        type:
          type: string
        name:
          type: string
        category:
          type: string
        config:
          type: object
          additionalProperties: true""",
        "PatchProviderRequest": """      type: object
      properties:
        name:
          type: string
        config:
          type: object
          additionalProperties: true""",
        "UpdateProviderConfigRequest": """      type: object
      properties:
        config:
          type: object
          additionalProperties: true""",
        "CreatePermissionRequest": """      type: object
      required:
        - name
      properties:
        name:
          type: string
        description:
          type: string
        resource:
          type: string
        action:
          type: string
        effect:
          type: string
          enum:
            - allow
            - deny
        sorter:
          type: string""",
        "PatchPermissionRequest": """      type: object
      properties:
        name:
          type: string
        description:
          type: string
        effect:
          type: string
          enum:
            - allow
            - deny
        sorter:
          type: string""",
        "EnforcePermissionRequest": """      type: object
      required:
        - subject
        - object
        - action
      properties:
        subject:
          type: string
        object:
          type: string
        action:
          type: string""",
        "BatchEnforcePermissionRequest": """      type: object
      properties:
        requests:
          type: array
          items:
            type: object
            properties:
              subject:
                type: string
              object:
                type: string
              action:
                type: string""",
        "UpdatePermissionPolicyRequest": """      type: object
      required:
        - policies
      properties:
        policies:
          type: array
          items:
            type: object
            required:
              - pType
              - rules
            properties:
              pType:
                type: string
              rules:
                type: array
                items:
                  type: string""",
        "BulkImportPermissionsRequest": """      type: object
      properties:
        permissions:
          type: array
          items:
            type: object""",
        "CreateRoleRequest": """      type: object
      required:
        - name
      properties:
        name:
          type: string
        description:
          type: string
        permissions:
          type: array
          items:
            type: string""",
        "PatchRoleRequest": """      type: object
      properties:
        name:
          type: string
        description:
          type: string
        permissions:
          type: array
          items:
            type: string""",
        "BulkImportRolesRequest": """      type: object
      properties:
        roles:
          type: array
          items:
            type: object""",
        "CreateGroupRequest": """      type: object
      required:
        - name
      properties:
        name:
          type: string
        description:
          type: string
        type:
          type: string
          enum: [virtual, physical, department]""",
        "PatchGroupRequest": """      type: object
      properties:
        name:
          type: string
        description:
          type: string
        type:
          type: string
          enum: [virtual, physical, department]""",
        "BulkImportGroupsRequest": """      type: object
      properties:
        groups:
          type: array
          items:
            type: object""",
        "PatchSessionRequest": """      type: object
      properties:
        state:
          type: string""",
        "CreateTokenRequest": """      type: object
      properties:
        name:
          type: string
        expiresIn:
          type: integer
        scopes:
          type: array
          items:
            type: string""",
        "CreateEnforcerRequest": """      type: object
      required:
        - name
        - modelId
        - adapterId
      properties:
        name:
          type: string
        modelId:
          type: string
        adapterId:
          type: string
        description:
          type: string""",
        "PatchEnforcerRequest": """      type: object
      properties:
        name:
          type: string
        description:
          type: string""",
        "AddEnforcerPolicyRequest": """      type: object
      properties:
        ptype:
          type: string
        policy:
          type: array
          items:
            type: string""",
        "UpdateEnforcerPolicyRequest": """      type: object
      properties:
        oldPolicy:
          type: array
          items:
            type: string
        newPolicy:
          type: array
          items:
            type: string""",
        "ImportPoliciesRequest": """      type: object
      properties:
        policies:
          type: array
          items:
            type: object""",
        "CreateAdapterRequest": """      type: object
      required:
        - name
        - type
      properties:
        name:
          type: string
        type:
          type: string
        config:
          type: object""",
        "PatchAdapterRequest": """      type: object
      properties:
        name:
          type: string
        config:
          type: object""",
        "AddAdapterPolicyRequest": """      type: object
      properties:
        ptype:
          type: string
        policy:
          type: array
          items:
            type: string""",
        "UpdateAdapterPolicyRequest": """      type: object
      properties:
        oldPolicy:
          type: array
          items:
            type: string
        newPolicy:
          type: array
          items:
            type: string""",
        "CreateModelRequest": """      type: object
      required:
        - name
        - type
      properties:
        name:
          type: string
        type:
          type: string
        provider:
          type: string
        config:
          type: object""",
        "PatchModelRequest": """      type: object
      properties:
        name:
          type: string
        config:
          type: object""",
        "CreateSyncerRequest": """      type: object
      required:
        - name
        - type
      properties:
        name:
          type: string
        type:
          type: string
        config:
          type: object""",
        "PatchSyncerRequest": """      type: object
      properties:
        name:
          type: string
        config:
          type: object""",
        "CreateWebhookRequest": """      type: object
      required:
        - url
      properties:
        url:
          type: string
        events:
          type: array
          items:
            type: string
        secret:
          type: string
        enabled:
          type: boolean""",
        "PatchWebhookRequest": """      type: object
      properties:
        url:
          type: string
        events:
          type: array
          items:
            type: string
        enabled:
          type: boolean""",
        "CreateLdapRequest": """      type: object
      required:
        - name
        - host
        - port
      properties:
        name:
          type: string
        host:
          type: string
        port:
          type: integer
        baseDn:
          type: string
        bindDn:
          type: string
        bindPassword:
          type: string
        useSsl:
          type: boolean
        startTls:
          type: boolean""",
        "PatchLdapRequest": """      type: object
      properties:
        name:
          type: string
        host:
          type: string
        port:
          type: integer
        baseDn:
          type: string
        bindDn:
          type: string
        bindPassword:
          type: string""",
        "CreateCertRequest": """      type: object
      properties:
        domain:
          type: string
        type:
          type: string
        provider:
          type: string""",
        "PatchCertRequest": """      type: object
      properties:
        domain:
          type: string
        provider:
          type: string""",
        "CreateKeyRequest": """      type: object
      properties:
        name:
          type: string
        type:
          type: string
        algorithm:
          type: string""",
        "PatchKeyRequest": """      type: object
      properties:
        name:
          type: string""",
        "CreateAgentRequest": """      type: object
      required:
        - name
      properties:
        name:
          type: string
        type:
          type: string
        config:
          type: object""",
        "PatchAgentRequest": """      type: object
      properties:
        name:
          type: string
        config:
          type: object""",
        "CreateFormRequest": """      type: object
      required:
        - name
      properties:
        name:
          type: string
        fields:
          type: array
          items:
            type: object""",
        "PatchFormRequest": """      type: object
      properties:
        name:
          type: string
        fields:
          type: array
          items:
            type: object""",
        "CreateGatewayServerRequest": """      type: object
      properties:
        name:
          type: string
        type:
          type: string
        config:
          type: object""",
        "UpdateGatewayServerRequest": """      type: object
      properties:
        name:
          type: string
        config:
          type: object""",
        "CreateGatewaySiteRequest": """      type: object
      properties:
        domain:
          type: string
        serverId:
          type: string
        config:
          type: object""",
        "UpdateGatewaySiteRequest": """      type: object
      properties:
        domain:
          type: string
        config:
          type: object""",
        "CreateInvitationRequest": """      type: object
      properties:
        organizationId:
          type: string
        email:
          type: string
        roleIds:
          type: array
          items:
            type: string
        groupIds:
          type: array
          items:
            type: string
        invitedBy:
          type: string
        validDays:
          type: integer
          minimum: 1
          maximum: 365""",
        "InvitationsVerifyResponse": """      type: object
      properties:
        organizationId:
          type: string
        organizationSlug:
          type: string
        email:
          type: string
        roleIds:
          type: array
          items:
            type: string
        groupIds:
          type: array
          items:
            type: string
        expiresAt:
          type: string""",
        "CreateOrderRequest": """      type: object
      properties:
        planId:
          type: string
        productId:
          type: string
        couponCode:
          type: string""",
        "PatchOrderRequest": """      type: object
      properties:
        status:
          type: string""",
        "PlaceOrderRequest": """      type: object
      properties:
        planId:
          type: string
        productId:
          type: string
        couponCode:
          type: string""",
        "CreatePlanRequest": """      type: object
      required:
        - name
      properties:
        name:
          type: string
        description:
          type: string
        price:
          type: number
        currency:
          type: string""",
        "PatchPlanRequest": """      type: object
      properties:
        name:
          type: string
        description:
          type: string
        price:
          type: number""",
        "CreatePricingRequest": """      type: object
      properties:
        name:
          type: string
        amount:
          type: number
        currency:
          type: string""",
        "PatchPricingRequest": """      type: object
      properties:
        name:
          type: string
        amount:
          type: number""",
        "CreateProductRequest": """      type: object
      required:
        - name
      properties:
        name:
          type: string
        description:
          type: string
        price:
          type: number
        currency:
          type: string""",
        "PatchProductRequest": """      type: object
      properties:
        name:
          type: string
        description:
          type: string""",
        "CreateSubscriptionRequest": """      type: object
      properties:
        planId:
          type: string
        userId:
          type: string""",
        "PatchSubscriptionRequest": """      type: object
      properties:
        status:
          type: string""",
        "CreateCouponRequest": """      type: object
      properties:
        code:
          type: string
        discount:
          type: number
        type:
          type: string
        expiresAt:
          type: string
          format: date-time""",
        "PatchCouponRequest": """      type: object
      properties:
        discount:
          type: number
        expiresAt:
          type: string
          format: date-time""",
        "CreateResourceRequest": """      type: object
      properties:
        name:
          type: string
        type:
          type: string""",
        "PatchResourceRequest": """      type: object
      properties:
        name:
          type: string""",
        "CreateRecordRequest": """      type: object
      properties:
        action:
          type: string
        resource:
          type: string
        details:
          type: object""",
        "SearchRecordsRequest": """      type: object
      properties:
        query:
          type: string
        filters:
          type: object
        page:
          type: integer
        pageSize:
          type: integer""",
        "CreateEntryRequest": """      type: object
      properties:
        name:
          type: string
        type:
          type: string
        data:
          type: object""",
        "PatchEntryRequest": """      type: object
      properties:
        name:
          type: string
        data:
          type: object""",
        "CreateRuleRequest": """      type: object
      properties:
        name:
          type: string
        type:
          type: string
        config:
          type: object""",
        "PatchRuleRequest": """      type: object
      properties:
        name:
          type: string
        config:
          type: object""",
        "CreateFaceIdProviderRequest": """      type: object
      properties:
        name:
          type: string
        type:
          type: string
        config:
          type: object""",
        "PatchFaceIdProviderRequest": """      type: object
      properties:
        name:
          type: string
        config:
          type: object""",
        "CreateCaptchaProviderRequest": """      type: object
      properties:
        name:
          type: string
        type:
          type: string
        config:
          type: object""",
        "PatchCaptchaProviderRequest": """      type: object
      properties:
        name:
          type: string
        config:
          type: object""",
        "CreateNotificationProviderRequest": """      type: object
      properties:
        name:
          type: string
        type:
          type: string
        config:
          type: object""",
        "PatchNotificationProviderRequest": """      type: object
      properties:
        name:
          type: string
        config:
          type: object""",
        "CreateNotificationRequest": """      type: object
      properties:
        type:
          type: string
        title:
          type: string
        content:
          type: string
        userId:
          type: string""",
        "CreateRadiusConfigRequest": """      type: object
      properties:
        name:
          type: string
        host:
          type: string
        port:
          type: integer
        secret:
          type: string""",
        "PatchRadiusConfigRequest": """      type: object
      properties:
        name:
          type: string
        host:
          type: string
        port:
          type: integer""",
        "CreateRadiusAccountingRequest": """      type: object
      properties:
        sessionId:
          type: string
        username:
          type: string
        nasIpAddress:
          type: string""",
        "PatchRadiusAccountingRequest": """      type: object
      properties:
        status:
          type: string""",
        "SendVerificationCodeRequest": """      type: object
      required:
        - dest
        - type
      properties:
        dest:
          type: string
        type:
          type: string
          enum:
            - email
            - sms""",
        "VerifyVerificationCodeRequest": """      type: object
      required:
        - dest
        - code
      properties:
        dest:
          type: string
        code:
          type: string""",
        "CreateTicketRequest": """      type: object
      required:
        - title
      properties:
        organizationId:
          type: string
        title:
          type: string
        type:
          type: string
          enum:
            - org_request
            - vendor_feedback
        description:
          type: string
        priority:
          type: string""",
        "PatchTicketRequest": """      type: object
      properties:
        title:
          type: string
        type:
          type: string
          enum:
            - org_request
            - vendor_feedback
        description:
          type: string
        status:
          type: string
        priority:
          type: string
        assigneeId:
          type: string""",
        "AddTicketMessageRequest": """      type: object
      required:
        - content
      properties:
        content:
          type: string""",
        "CreateTransactionRequest": """      type: object
      required:
        - type
        - amount
        - currency
      properties:
        organizationId:
          type: string
        userId:
          type: string
        type:
          type: string
        amount:
          type: number
        currency:
          type: string
        description:
          type: string""",
        "PatchTransactionRequest": """      type: object
      properties:
        state:
          type: string
        description:
          type: string""",
        "CreateMcpServerRequest": """      type: object
      required:
        - name
        - type
      properties:
        organizationId:
          type: string
        name:
          type: string
        slug:
          type: string
        type:
          type: string
        description:
          type: string
        endpoint:
          type: string
        isEnabled:
          type: boolean""",
        "PatchMcpServerRequest": """      type: object
      properties:
        name:
          type: string
        description:
          type: string
        endpoint:
          type: string
        isEnabled:
          type: boolean""",
        "RegisterOAuthClientRequest": """      type: object
      required:
        - client_name
      properties:
        client_name:
          type: string
        redirect_uris:
          type: array
          items:
            type: string
        grant_types:
          type: array
          items:
            type: string
        response_types:
          type: array
          items:
            type: string
        token_endpoint_auth_method:
          type: string
        jwks:
          type: object
          description: Inline JSON Web Key Set for private_key_jwt. Only public key members are accepted. Alternatively set jwks_uri to a remotely hosted JWKS.
        jwks_uri:
          type: string
          format: uri
          maxLength: 2048
          description: Remote JWKS location for private_key_jwt (RFC 7591 jwks_uri). HTTPS only; fetched with SSRF protections (5s timeout, 64KiB cap, 5-minute cache, fail-closed).
        scope:
          type: string""",
        "UpdateOAuthClientRequest": """      type: object
      properties:
        client_name:
          type: string
        redirect_uris:
          type: array
          items:
            type: string
        grant_types:
          type: array
          items:
            type: string
        token_endpoint_auth_method:
          type: string
        jwks:
          type: object
          description: Inline JSON Web Key Set for private_key_jwt. Only public key members are accepted. Alternatively set jwks_uri to a remotely hosted JWKS.
        jwks_uri:
          type: string
          format: uri
          maxLength: 2048
          description: Remote JWKS location for private_key_jwt (RFC 7591 jwks_uri). HTTPS only; fetched with SSRF protections (5s timeout, 64KiB cap, 5-minute cache, fail-closed).
        scope:
          type: string""",
        "SamlAcsRequest": """      type: object
      properties:
        SAMLResponse:
          type: string
        RelayState:
          type: string""",
        "SamlSpAcsRequest": """      type: object
      required:
        - SAMLResponse
        - RelayState
      properties:
        SAMLResponse:
          type: string
        RelayState:
          type: string""",
        "UpdateTranslationsRequest": """      type: object
      properties:
        language:
          type: string
        translations:
          type: object
          additionalProperties: true""",
        "TranslateTextRequest": """      type: object
      properties:
        text:
          type: string
        sourceLanguage:
          type: string
        targetLanguage:
          type: string""",
        "CreateSyncConfigRequest": """      type: object
      properties:
        organizationId:
          type: string
        ldapId:
          type: string
        syncIntervalMinutes:
          type: integer
        baseDn:
          type: string
        userFilter:
          type: string
        groupFilter:
          type: string
        enabled:
          type: boolean""",
        "PatchSyncConfigRequest": """      type: object
      properties:
        syncIntervalMinutes:
          type: integer
        baseDn:
          type: string
        userFilter:
          type: string
        groupFilter:
          type: string
        enabled:
          type: boolean""",
        "BatchRequest": """      type: object
      required:
        - operations
      properties:
        operations:
          type: array
          items:
            type: object
            required:
              - method
              - path
            properties:
              method:
                type: string
              path:
                type: string
              body:
                type: object""",
        "CreatePaymentRequest": """      type: object
      properties:
        orderId:
          type: string
        amount:
          type: number
        currency:
          type: string
        providerType:
          type: string""",
        "PatchPaymentRequest": """      type: object
      properties:
        status:
          type: string""",
        "InvoicePaymentRequest": """      type: object
      properties:
        invoiceType:
          type: string""",
        "SendEmailRequest": """      type: object
      required:
        - to
        - subject
        - body
      properties:
        to:
          type: string
        subject:
          type: string
        body:
          type: string""",
        "SendSmsRequest": """      type: object
      required:
        - to
        - message
      properties:
        to:
          type: string
        message:
          type: string""",
        "SendNotificationRequest": """      type: object
      required:
        - userId
        - title
        - body
      properties:
        userId:
          type: string
        title:
          type: string
        body:
          type: string""",
        "StartRadiusServerRequest": """      type: object
      properties:
        configId:
          type: string""",
        "StopRadiusServerRequest": """      type: object
      properties:
        configId:
          type: string""",
        "KerberosNegotiateRequest": """      type: object
      properties:
        spnegoToken:
          type: string""",
        "KerberosLoginRequest": """      type: object
      properties:
        ticket:
          type: string""",
        "ScimCreateUserRequest": """      type: object
      properties:
        userName:
          type: string
        name:
          type: object
        emails:
          type: array
          items:
            type: object
        active:
          type: boolean""",
        "ScimReplaceUserRequest": """      type: object
      properties:
        userName:
          type: string
        name:
          type: object
        emails:
          type: array
          items:
            type: object
        active:
          type: boolean""",
        "ScimPatchUserRequest": """      type: object
      properties:
        Operations:
          type: array
          items:
            type: object""",
        "ScimCreateGroupRequest": """      type: object
      properties:
        displayName:
          type: string
        members:
          type: array
          items:
            type: object""",
        "ScimReplaceGroupRequest": """      type: object
      properties:
        displayName:
          type: string
        members:
          type: array
          items:
            type: object""",
        "ScimPatchGroupRequest": """      type: object
      properties:
        Operations:
          type: array
          items:
            type: object""",
        "CreateApplicationRequest": """      type: object
      required:
        - name
      properties:
        name:
          type: string
        description:
          type: string
        organizationId:
          type: string
          format: uuid
        tokenSigningMethod:
          type: string
          enum:
            - RS256
            - SM2-SM3
        jwks:
          type: object
          description: Inline JSON Web Key Set used by token_endpoint_auth_method=private_key_jwt. Only public key members are accepted. Alternatively set jwksUri to a remotely hosted JWKS.
        jwksUri:
          type: string
          format: uri
          maxLength: 2048
          description: Remote JWKS location for private_key_jwt. HTTPS only; fetched with SSRF protections (5s timeout, 64KiB cap, 5-minute cache, fail-closed). Either inline jwks or jwksUri satisfies the key-material requirement; both may be set.
        clientCertificates:
          type: array
          maxItems: 8
          description: Initial mTLS client certificates written in the same transaction as the application. Required when tokenEndpointAuthMethod is tls_client_auth or self_signed_tls_client_auth; rejected for every other method.
          items:
            type: object
            required:
              - fingerprintSha256
              - certificate
              - selfSigned
            properties:
              fingerprintSha256:
                type: string
                description: RFC 8705 x5t#S256, a 43-character base64url SHA-256 of the DER certificate. The server recomputes it and rejects a mismatch.
              certificate:
                type: string
                description: One public certificate, PEM CERTIFICATE block or base64 DER.
              selfSigned:
                type: boolean""",
        "PatchApplicationRequest": """      type: object
      properties:
        name:
          type: string
        description:
          type: string
        tokenSigningMethod:
          type: string
          enum:
            - RS256
            - SM2-SM3
        jwks:
          type: object
          description: Replaces the inline JSON Web Key Set when token_endpoint_auth_method=private_key_jwt. Only public key members are accepted.
        jwksUri:
          type: string
          format: uri
          maxLength: 2048
          description: Replaces the remote JWKS location for private_key_jwt. HTTPS only; fetched with SSRF protections (5s timeout, 64KiB cap, 5-minute cache, fail-closed). An empty string removes the stored URI.
        clientCertificates:
          type: array
          maxItems: 8
          description: Replaces the whole mTLS certificate set in the same transaction as the application update. Required when switching to tls_client_auth or self_signed_tls_client_auth; rejected for every other method.
          items:
            type: object
            required:
              - fingerprintSha256
              - certificate
              - selfSigned
            properties:
              fingerprintSha256:
                type: string
                description: RFC 8705 x5t#S256, a 43-character base64url SHA-256 of the DER certificate. The server recomputes it and rejects a mismatch.
              certificate:
                type: string
                description: One public certificate, PEM CERTIFICATE block or base64 DER.
              selfSigned:
                type: boolean""",
        "ApplicationClientCertificateList": """      type: object
      required:
        - items
      properties:
        items:
          type: array
          items:
            type: object
            required:
              - fingerprintSha256
              - selfSigned
              - status
            properties:
              fingerprintSha256:
                type: string
                description: RFC 8705 x5t#S256, a 43-character base64url SHA-256 of the DER certificate.
              notAfter:
                type: string
                format: date-time
              selfSigned:
                type: boolean
              status:
                type: string""",
        "ReplaceApplicationClientCertificatesRequest": """      type: object
      required:
        - items
      properties:
        items:
          type: array
          maxItems: 8
          items:
            type: object
            required:
              - fingerprintSha256
              - certificate
              - selfSigned
            properties:
              fingerprintSha256:
                type: string
                description: Must equal base64url(SHA-256(DER)) of certificate; the server recomputes and rejects a mismatch.
              certificate:
                type: string
                description: One public certificate in PEM (a single CERTIFICATE block) or base64 DER. Private key material is rejected.
              selfSigned:
                type: boolean
                description: Must match token_endpoint_auth_method; true only for self_signed_tls_client_auth.""",
        "ReplaceApplicationProviderBindingsRequest": """      type: object
      properties:
        providerBindings:
          type: array
          items:
            type: object""",
        "ImportApplicationRequest": """      type: object
      properties:
        config:
          type: object""",
        "TrustedDeviceBeginResponse": """      type: object
      required:
        - challengeId
        - nonce
        - expiresAt
      properties:
        challengeId:
          type: string
        nonce:
          type: string
          description: Opaque nonce the device must sign with the registered public key.
        expiresAt:
          type: string
          format: date-time""",
        "TrustedDeviceRegisterRequest": """      type: object
      required:
        - challengeId
        - nonce
        - deviceCredentialId
        - devicePublicKey
        - signature
      properties:
        challengeId:
          type: string
        nonce:
          type: string
        deviceCredentialId:
          type: string
        devicePublicKey:
          type: string
          description: Base64-encoded device public key bound to the credential id.
        signature:
          type: string
          description: Base64-encoded signature over the challenge nonce.""",
        "TrustedDeviceView": """      type: object
      required:
        - id
        - userId
        - deviceCredentialId
        - status
        - createdAt
        - lastSeenAt
        - expiresAt
      properties:
        id:
          type: string
        userId:
          type: string
        deviceCredentialId:
          type: string
        status:
          type: string
          enum:
            - active
            - revoked
            - expired
        createdAt:
          type: string
          format: date-time
        lastSeenAt:
          type: string
          format: date-time
        expiresAt:
          type: string
          format: date-time
        revokedAt:
          type: string
          format: date-time
          description: Present only after revocation.""",
        "TrustedDeviceListResponse": """      type: array
      items:
        $ref: '#/components/schemas/TrustedDeviceView'""",
        "StepUpBeginRequest": """      type: object
      required:
        - operationScope
      properties:
        operationScope:
          type: string
          description: Operation the session is being elevated for; bound to the challenge.""",
        "StepUpFinishRequest": """      type: object
      required:
        - challengeId
        - operationScope
      description: Provide either passcode (TOTP) or the WebAuthn assertion fields, never both.
      properties:
        challengeId:
          type: string
        operationScope:
          type: string
        passcode:
          type: string
          description: TOTP code; never logged. Mutually exclusive with the assertion fields.
        webauthnChallengeId:
          type: string
          description: Challenge id returned by /api/v1/auth/webauthn/authentication/begin.
        challenge:
          type: string
          description: WebAuthn challenge plaintext returned by authentication/begin.
        credentialId:
          type: string
        signature:
          type: string
        authenticatorData:
          type: string
        clientDataJSON:
          type: string""",
        "StepUpBeginResponse": """      type: object
      required:
        - challengeId
        - operationScope
        - expiresAt
      properties:
        challengeId:
          type: string
        operationScope:
          type: string
        expiresAt:
          type: string
          format: date-time
          description: Challenge is valid for five minutes.""",
        "StepUpFinishResponse": """      type: object
      required:
        - assuranceLevel
        - sessionId
        - operationScope
        - elevatedAt
      properties:
        assuranceLevel:
          type: string
          enum:
            - aal1
            - aal2
        sessionId:
          type: string
          description: The elevated session; only this session is raised, login state is unchanged.
        operationScope:
          type: string
        elevatedAt:
          type: string
          format: date-time""",
        "RememberMeIssueRequest": """      type: object
      properties:
        applicationId:
          type: string
          description: Optional application the token family is scoped to.""",
        "RememberMeExchangeRequest": """      type: object
      properties:
        token:
          type: string
          description: Single-use token; when omitted the remember-me cookie is used.""",
        "RememberMeRevokeRequest": """      type: object
      properties:
        familyId:
          type: string
          description: Optional family id; when omitted all families for the current user are revoked.""",
        "RememberMeTokenResponse": """      type: object
      required:
        - token
        - familyId
        - expiresAt
      properties:
        token:
          type: string
          description: One-time plaintext token; the server stores only its hash.
        familyId:
          type: string
        expiresAt:
          type: string
          format: date-time
        userId:
          type: string
        organizationId:
          type: string""",
        "AdminRebindTOTPRequest": """      type: object
      required:
        - confirm
        - reason
      properties:
        confirm:
          type: boolean
          description: Second confirmation; false is rejected with 400.
        reason:
          type: string
          description: Operator-supplied justification recorded in the audit trail.""",
    }

    for name, schema_yaml in sorted(schema_defs.items()):
        lines.append(f"    {name}:")
        lines.append(schema_yaml)

    return lines


def main():
    os.chdir(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

    out_path = OUTPUT_FILE
    check_only = False
    args = sys.argv[1:]
    for index, arg in enumerate(args):
        if arg == "--out":
            if index + 1 >= len(args):
                print("ERROR: --out 需要一个路径参数", file=sys.stderr)
                return 2
            out_path = args[index + 1]
        elif arg == "--check":
            check_only = True
        else:
            print(f"ERROR: 未知参数 {arg}", file=sys.stderr)
            return 2

    try:
        defs = extract_all_routes()
    except ValueError as exc:
        # 解析完整性断言失败：宁可让门禁红，也不生成一份缺端点的契约。
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1
    print(f"Total endpoints extracted: {len(defs)}", file=sys.stderr)

    yaml_content = generate_openapi(defs)
    if yaml_content is None:
        # generate_openapi() returned None because a path+method pair was declared in
        # more than one route file; the spec must not be written with a silently
        # dropped operation.
        return 1

    if check_only:
        existing = ""
        if os.path.isfile(out_path):
            with open(out_path) as f:
                existing = f.read()
        if existing != yaml_content:
            print(
                f"FAIL: {out_path} 与生成结果不一致 — 运行 python3 scripts/gen_openapi.py",
                file=sys.stderr,
            )
            return 1
        print(f"PASS: {out_path} 与生成结果一致", file=sys.stderr)
        return 0

    parent = os.path.dirname(out_path)
    if parent:
        os.makedirs(parent, exist_ok=True)
    with open(out_path, "w") as f:
        f.write(yaml_content)

    print(f"{out_path} generated successfully", file=sys.stderr)

    # Count verification
    op_count = yaml_content.count("operationId:")
    print(f"operationId count in generated file: {op_count}", file=sys.stderr)

    return 0


if __name__ == "__main__":
    sys.exit(main())
