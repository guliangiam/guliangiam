#!/usr/bin/env python3
"""Reject read-side DTOs that expose a credential as plaintext.

Why this exists
---------------
The read APIs answer detail/list responses straight from the app-layer DTOs in
`internal/app/**`. A DTO field that mirrors a stored secret (an OAuth client
secret, a RADIUS shared secret, a database password, an access-key secret …) is
marshalled verbatim: the plaintext leaves the process on every GET. The
authentication code never has to be wrong for this to happen — the leak is the
DTO shape itself, so no unit test that exercises a handler will notice.

The project already has one sanctioned shape for this, adopted by
`internal/app/providers/dto.go` and `internal/app/certs/dto.go`:

    ClientSecretSet    bool   `json:"clientSecretSet"`
    ClientSecretMasked string `json:"clientSecretMasked,omitempty"`

i.e. a read DTO stops emitting the plaintext and emits an *is-set* boolean plus
a *masked* display string; the write path (Create/Patch commands and the
transport request DTOs) keeps accepting plaintext and is not touched. This gate
makes that convention enforceable: a read-side struct that carries a credential
field must carry the paired `XxxSet` / `XxxMasked` fields. It is how
faceidproviders / captchaproviders / adapters / radiusconfigs were caught still
returning `accessKeySecret` / `secretKey` / `dbPassword` / `sharedSecret`.

Read-side structs are those whose name ends in `Detail`, `ListItem` or
`Response`, plus any struct embedded (nested) inside one — an embedded struct's
fields are flattened into the parent's JSON, so its credentials leak too.

Some read payloads are built as `map[string]any` literals instead of structs
(`internal/app/users/mapper.go` 的 Profile/属性面)。struct 扫描对它们完全
不可见，因此追加一条 map 字面量键扫描：规范 9.5 的 PII 键（idCard/
passportNo/driverLicenseNo）必须以 `XxxSet`/`XxxMasked` 配对形式出现，
裸键即 FAIL。

A deliberate exception is registered in ALLOWLIST below as `Struct.field`
(or `Struct.*`) with a reason; map-key exceptions go in MAP_KEY_ALLOWLIST
keyed by relative path. A reasoned exception beats a silent pass: the
next reader can see *why* a credential-shaped field is exempt.

Usage: python3 scripts/check-credential-masking.py
Exit code 0 = every read-side credential field is masked or allowlisted,
            1 = an unmasked read-side credential field was found.

输入：internal/app/**/dto.go、response.go
输出：PASS/FAIL + 未脱敏字段列表
--check 行为：无 --check；失败即 exit 1
消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
"""

import os
import re
import sys

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
APP_DIR = os.path.join(REPO_ROOT, "internal", "app")

# Structs whose JSON is returned to a client. Substring before the suffix so
# `ProviderConfigDetail` / `WebhookEventListItem` are covered too.
READ_SIDE_SUFFIX = re.compile(r"(Detail|ListItem|Response)$")

STRUCT_DECL = re.compile(r"^type\s+(\w+)\s+struct\s*\{", re.MULTILINE)

# A field line: `Name Type `… json:"tag" …`` (single line only).
FIELD = re.compile(r"^([A-Z]\w*)\s+\S")
# An embedded (anonymous) field: a bare type name with no tag.
EMBEDDED = re.compile(r"^([A-Z]\w*)$")
JSON_TAG = re.compile(r"json:\"([^\"]*)\"")

# Credential nouns. A tag whose *final* identifier word is one of these is a
# credential (accessKeySecret, clientSecret, dbPassword, accessToken, …), as is
# a two-word compound where the credential noun is the leading modifier and the
# head is `key` (secretKey). Metadata fields that merely mention a noun
# (passwordType, tokenUrl, passwordOptions, tokenFormat …) are not credentials:
# their head word is not a credential noun.
CRED_WORDS = {
    "secret",
    "password",
    "passwd",
    "token",
    "privatekey",
    "apikey",
    "credential",
}

# Fields already in the paired form are the remedy, not the leak.
PAIRED_SUFFIX = re.compile(r"(Set|Masked)$")

# 规范 9.5 PII 键：证件号族永不返回原文。归一化（小写、去分隔符）后精确
# 匹配 map[string]any 字面量键；命中而没有同文件 `<key>Set`/`<key>Masked`
# 配对即违规。
PII_MAP_KEYS = {"idcard", "passportno", "driverlicenseno"}

MAP_LITERAL_KEY = re.compile(r'^\s*"([A-Za-z][A-Za-z0-9_]*)"\s*:', re.MULTILINE)

# Reviewed map-key exceptions, keyed by repo-relative path, each with a reason.
MAP_KEY_ALLOWLIST = {
    "internal/app/users/custom_attributes.go": (
        "自定义属性保留字段名集合（map[string]struct{} 的键），用于拒绝与档案"
        "列同名的自定义属性，不是响应载荷"
    ),
}

# Reviewed exceptions, keyed by `Struct.field` (or `Struct.*`), each with a
# reason. Prefer a reason over a silent skip.
ALLOWLIST = {
    "ApplicationDetail.EnablePassword": (
        "布尔功能开关（是否允许密码登录），字段名含 password 但非凭据"
    ),
    "LoginResponse.*": (
        "OIDC/OAuth 登录响应，令牌即响应载荷本身（accessToken/idToken/"
        "refreshToken），按协议必须下发，非管理面回显存储凭据"
    ),
    "AuthorizeResponse.*": (
        "OIDC 隐式流授权响应，令牌即响应载荷本身，按协议必须下发"
    ),
    "ExchangeAuthorizationCodeResponse.*": (
        "OIDC 授权码换取令牌响应，令牌即响应载荷本身，按协议必须下发"
    ),
    "TokenExchangeResponse.*": (
        "RFC 8693 令牌交换响应，令牌即响应载荷本身，按协议必须下发"
    ),
    "DCRResponse.ClientSecret": (
        "RFC 7591 动态客户端注册响应：client_secret 为一次性下发的注册产物，"
        "协议要求必须在响应中回传，非存储凭据回显"
    ),
    "DCRResponse.RegistrationAccessToken": (
        "RFC 7591 动态客户端注册响应：registration_access_token 为一次性下发的"
        "注册凭据，协议要求必须在响应中回传"
    ),
    "AgentListItem.Token": (
        "应用层已就地脱敏（first4****last4），非明文泄露；脱敏机制不同于 "
        "XxxSet/XxxMasked 范式，统一为独立工作项"
    ),
}


def camel_words(tag):
    """Split a json tag name into lowercase identifier words."""
    tag = re.sub(r"\d+$", "", tag)  # clientSecret2 -> clientSecret
    words = []
    for part in re.split(r"[_\-]+", tag):
        words += re.findall(r"[A-Z]+(?![a-z])|[A-Z]?[a-z]+", part)
    return [w.lower() for w in words if w]


def is_credential(tag):
    """Whether a json tag names a stored credential."""
    if tag.lower() in CRED_WORDS:  # single token: apikey, privatekey
        return True
    words = camel_words(tag)
    if not words:
        return False
    if words[-1] in CRED_WORDS:
        return True
    # secretKey: the credential noun is the leading modifier, `key` is the head.
    return len(words) == 2 and words[0] in CRED_WORDS and words[1] == "key"


def parse_structs(source):
    """Return {struct_name: {"fields": {name: json_tag}, "embedded": [names]}}."""
    structs = {}
    for match in STRUCT_DECL.finditer(source):
        name = match.group(1)
        body = source[match.end():]
        close = re.search(r"^\}", body, re.MULTILINE)
        if close:
            body = body[: close.start()]
        fields = {}
        embedded = []
        for raw in body.splitlines():
            code = raw.split("//")[0]
            stripped = code.strip()
            if not stripped:
                continue
            if "`" not in code:
                emb = EMBEDDED.match(stripped)
                if emb:
                    embedded.append(emb.group(1))
                continue
            field = FIELD.match(stripped)
            if not field:
                continue
            tag = JSON_TAG.search(code)
            if not tag:
                continue
            fields[field.group(1)] = tag.group(1).split(",")[0]
        structs[name] = {"fields": fields, "embedded": embedded}
    return structs


def resolved_fields(structs, name, seen=None):
    """Fields of a struct plus those of every struct it embeds (flattened).

    A read-side embed is skipped: it is checked under its own name, so pulling
    its fields up here would only report the same leak twice (e.g. `AgentDetail`
    embeds `AgentListItem`). Non-read-side embeds (shared bases) are flattened
    because nothing else inspects them.
    """
    seen = seen or set()
    if name in seen or name not in structs:
        return {}
    seen.add(name)
    entry = structs[name]
    fields = dict(entry["fields"])
    for emb in entry["embedded"]:
        if READ_SIDE_SUFFIX.search(emb):
            continue
        fields.update(resolved_fields(structs, emb, seen))
    return fields


def allowlisted(struct_name, field_name):
    return (
        f"{struct_name}.{field_name}" in ALLOWLIST
        or f"{struct_name}.*" in ALLOWLIST
    )


def normalize_map_key(key):
    return re.sub(r"[_\-]", "", key).lower()


def check_map_literal_keys(rel, source, violations):
    """Flag bare PII keys in map literals without the Set/Masked pairing."""
    if rel in MAP_KEY_ALLOWLIST:
        return
    for match in MAP_LITERAL_KEY.finditer(source):
        key = match.group(1)
        if normalize_map_key(key) not in PII_MAP_KEYS:
            continue
        if f'"{key}Set"' in source and f'"{key}Masked"' in source:
            continue
        line = source[: match.start()].count("\n") + 1
        violations.append(
            f'{rel}:{line} (map literal key "{key}": PII 原文键，'
            f'必须改为 "{key}Set" + "{key}Masked" 配对)'
        )


def main():
    violations = []
    allowlisted_hits = 0

    for dirpath, _dirs, files in os.walk(APP_DIR):
        for name in sorted(files):
            if not name.endswith(".go") or name.endswith("_test.go"):
                continue
            path = os.path.join(dirpath, name)
            with open(path, encoding="utf-8") as handle:
                source = handle.read()
            structs = parse_structs(source)
            rel = os.path.relpath(path, REPO_ROOT)

            check_map_literal_keys(rel, source, violations)

            for struct_name in structs:
                if not READ_SIDE_SUFFIX.search(struct_name):
                    continue
                fields = resolved_fields(structs, struct_name)
                for field_name, tag in sorted(fields.items()):
                    if PAIRED_SUFFIX.search(field_name):
                        continue
                    if not is_credential(tag):
                        continue
                    if f"{field_name}Set" in fields and f"{field_name}Masked" in fields:
                        continue
                    if allowlisted(struct_name, field_name):
                        allowlisted_hits += 1
                        continue
                    line = _line_of(source, struct_name, field_name)
                    violations.append(
                        f"{rel}:{line} ({struct_name}.{field_name} json:\"{tag}\")"
                    )

    if violations:
        print(
            f"Credential-masking check FAILED ({len(violations)} unmasked read-side "
            "credential field(s) found):"
        )
        for item in violations:
            print(f"  - {item}")
        print(
            "  A read DTO must not emit a credential as plaintext. Replace the "
            "field with `<Name>Set bool` + `<Name>Masked string` (see "
            "internal/app/providers/dto.go), keep the write path plaintext, or "
            "register the field in ALLOWLIST with a reason.",
            file=sys.stderr,
        )
        return 1

    print(
        "Credential-masking check OK: every read-side credential field is "
        f"masked (`XxxSet` + `XxxMasked`) or allowlisted "
        f"({allowlisted_hits} allowlisted hit(s), {len(ALLOWLIST)} registered "
        "exception(s))"
    )
    return 0


def _line_of(source, struct_name, field_name):
    struct_at = re.search(rf"^type\s+{re.escape(struct_name)}\s+struct", source, re.M)
    start = struct_at.start() if struct_at else 0
    field_at = re.search(rf"^\s*{re.escape(field_name)}\s+\S", source[start:], re.M)
    if not field_at:
        return source[:start].count("\n") + 1
    return source[: start + field_at.start()].count("\n") + 1


if __name__ == "__main__":
    sys.exit(main())
