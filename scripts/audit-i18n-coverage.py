#!/usr/bin/env python3
"""i18n 覆盖度审计（只读，不修改语言包）。

统计每个语言包中「与英文源相同」的文案（视为未翻译），并按源码实际引用的
键给出渲染面覆盖情况。用于持续度量翻译波次进度，不阻断构建。

用法：
  python3 scripts/audit-i18n-coverage.py                 # 控制台摘要
  python3 scripts/audit-i18n-coverage.py --json out.json # 机器可读结果
  python3 scripts/audit-i18n-coverage.py --md out.md     # Markdown 报告
  python3 scripts/audit-i18n-coverage.py --list zh-CN --min-words 2
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
WEB = ROOT / "web" / "src"
LOCALES = WEB / "shared" / "i18n" / "locales"
NAMESPACES = ("common", "auth", "resource", "help")
SOURCE_LANG = "en"

# 产品名与技术术语不算未翻译。
TERM_WHITELIST = {
    "api key", "api keys", "jwt", "jws", "jwks", "oidc", "oauth", "oauth2", "saml", "scim",
    "ldap", "mfa", "otp", "totp", "webauthn", "passkey", "passkeys", "base dn", "bind dn",
    "liquid auth", "evm", "keccak256", "sha-256", "sha256", "rsa", "ecdsa", "pem", "csr",
    "url", "uri", "http", "https", "json", "yaml", "csv", "xlsx", "pdf", "ui", "api", "sdk",
    "idp", "sp", "acs", "sso", "rbac", "abac", "rebac", "iga", "pam", "llm", "ai", "gpu",
    "openapi", "swagger", "postgresql", "mysql", "redis", "kafka", "docker", "kubernetes",
    "github", "google", "microsoft", "apple", "wechat", "alipay", "dingtalk", "feishu",
    "sms", "totp", "hotp", "radius", "eap", "peap", "tls", "ssl", "dns", "ip", "cidr",
    # 品牌 / 协议 / 标准名称：保留英文原文
    "aws s3", "azure acs", "azure ad", "azure ad b2c", "azure blob", "amazon ses",
    "amazon sns", "google cloud storage", "microsoft teams", "zoho mail",
    "recaptcha v2", "recaptcha v3", "cloudflare turnstile", "lemon squeezy",
    "bird sms", "clickatell sms", "plivo sms", "sinch sms", "telnyx sms",
    "vonage sms", "africa's talking sms", "gopay (gc)", "kerberos keytab",
    "c14n 1.0", "content-type: application/x-protobuf", "v0 (sub)", "v1 (dom/obj)",
    "v2 (obj/act)", "saml 2.0", "oidc / oauth", "api token", "json mapping of ldap attributes",
}

KEY_CALL = re.compile(r"\b(?:t|tc|tr|ta|ti)\(\s*[\"']([A-Za-z][\w.]*)[\"']")
LATIN = re.compile(r"[A-Za-z][A-Za-z0-9 ,.'’/&()%+—–:_-]{1,}")


def flatten(node: object, prefix: str = "") -> dict[str, str]:
    result: dict[str, str] = {}
    if isinstance(node, dict):
        for key, value in node.items():
            result.update(flatten(value, f"{prefix}.{key}" if prefix else key))
    elif isinstance(node, str):
        result[prefix] = node
    return result


def load_locale(lang: str) -> dict[str, dict[str, str]]:
    bundle: dict[str, dict[str, str]] = {}
    for namespace in NAMESPACES:
        path = LOCALES / lang / f"{namespace}.json"
        if not path.exists():
            continue
        bundle[namespace] = flatten(json.loads(path.read_text(encoding="utf-8")))
    return bundle


def referenced_keys() -> set[str]:
    keys: set[str] = set()
    for path in WEB.rglob("*.ts*"):
        if ".test." in path.name or ".stories." in path.name:
            continue
        for match in KEY_CALL.finditer(path.read_text(encoding="utf-8")):
            keys.add(match.group(1))
    return keys


def is_multiword_english(value: str) -> bool:
    if not LATIN.fullmatch(value) or " " not in value:
        return False
    return value.strip().lower() not in TERM_WHITELIST


def analyze() -> dict:
    english = load_locale(SOURCE_LANG)
    reference = referenced_keys()
    langs = sorted(path.name for path in LOCALES.iterdir() if path.is_dir())
    matrix: dict[str, dict] = {}

    for lang in langs:
        bundle = load_locale(lang)
        per_ns: dict[str, dict] = {}
        for namespace, values in bundle.items():
            source = english.get(namespace, {})
            total = len(values)
            same = multi = referenced = untranslated_referenced = 0
            for key, value in values.items():
                source_value = source.get(key)
                identical = isinstance(source_value, str) and value == source_value
                if identical and LATIN.fullmatch(value):
                    same += 1
                    if is_multiword_english(value):
                        multi += 1
                if key in reference:
                    referenced += 1
                    if identical and is_multiword_english(value):
                        untranslated_referenced += 1
            per_ns[namespace] = {
                "total": total,
                "sameAsEnglish": same,
                "multiwordEnglish": multi,
                "referenced": referenced,
                "referencedUntranslated": untranslated_referenced,
            }
        matrix[lang] = {
            "namespaces": per_ns,
            "total": sum(item["total"] for item in per_ns.values()),
            "sameAsEnglish": sum(item["sameAsEnglish"] for item in per_ns.values()),
            "multiwordEnglish": sum(item["multiwordEnglish"] for item in per_ns.values()),
            "referenced": sum(item["referenced"] for item in per_ns.values()),
            "referencedUntranslated": sum(
                item["referencedUntranslated"] for item in per_ns.values()
            ),
        }

    return {"sourceLanguage": SOURCE_LANG, "referencedKeys": len(reference), "languages": matrix}


def render_markdown(data: dict) -> str:
    lines = [
        "# i18n 覆盖度矩阵",
        "",
        f"- 英文源引用键：{data['referencedKeys']}",
        "- 「与 en 相同」视为未翻译；术语白名单不计入多词英文。",
        "",
        "| 语言 | 总键 | 与 en 相同 | 多词英文 | 已引用 | 已引用且未翻译 |",
        "|---|---|---|---|---|---|",
    ]
    for lang, item in sorted(data["languages"].items()):
        lines.append(
            f"| {lang} | {item['total']} | {item['sameAsEnglish']} | "
            f"{item['multiwordEnglish']} | {item['referenced']} | "
            f"{item['referencedUntranslated']} |"
        )
    return "\n".join(lines) + "\n"


def list_untranslated(lang: str, min_words: int) -> list[dict]:
    english = load_locale(SOURCE_LANG)
    bundle = load_locale(lang)
    rows: list[dict] = []
    for namespace, values in bundle.items():
        source = english.get(namespace, {})
        for key, value in values.items():
            source_value = source.get(key)
            if not isinstance(source_value, str) or value != source_value:
                continue
            if value.strip().lower() in TERM_WHITELIST:
                continue
            if len(value.split()) < min_words:
                continue
            rows.append({"namespace": namespace, "key": key, "value": value})
    return rows


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--json")
    parser.add_argument("--md")
    parser.add_argument("--list", dest="list_lang")
    parser.add_argument("--min-words", type=int, default=2)
    args = parser.parse_args()

    data = analyze()
    if args.json:
        Path(args.json).write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    if args.md:
        Path(args.md).write_text(render_markdown(data), encoding="utf-8")
    if args.list_lang:
        for row in list_untranslated(args.list_lang, args.min_words):
            print(f"{row['namespace']}\t{row['key']}\t{row['value']}")
        return 0

    print(f"referenced keys: {data['referencedKeys']}")
    print(f"{'lang':6} {'multiword':>10} {'sameAsEn':>9} {'refUntranslated':>16}")
    for lang, item in sorted(data["languages"].items()):
        print(
            f"{lang:6} {item['multiwordEnglish']:10} {item['sameAsEnglish']:9} "
            f"{item['referencedUntranslated']:16}"
        )
    return 0


if __name__ == "__main__":
    sys.exit(main())
