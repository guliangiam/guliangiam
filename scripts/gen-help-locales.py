#!/usr/bin/env python3
"""Merge the help description source of truth into every locale bundle.

Source: web/src/shared/i18n/help/descriptions.json
Target: web/src/shared/i18n/locales/<lang>/help.json

Every locale receives the same key tree (required by check-i18n-keys.py).
Locales that do not have an authored translation yet fall back to English;
`python3 scripts/audit-i18n-coverage.py` reports those as same-as-English so
the remaining translation work stays visible.
"""
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
LOCALES = ROOT / "web/src/shared/i18n/locales"
SOURCE = ROOT / "web/src/shared/i18n/help/descriptions.json"
LANGS = ["zh-CN", "zh-TW", "en", "ja", "ko", "de", "fr", "es", "pt", "vi", "pl", "tr", "uk", "ru"]


def set_path(tree, path, value):
    parts = path.split(".")
    node = tree
    for part in parts[:-1]:
        node = node.setdefault(part, {})
    node[parts[-1]] = value


def main():
    source = json.loads(SOURCE.read_text(encoding="utf-8"))
    stats = {}
    for lang in LANGS:
        path = LOCALES / lang / "help.json"
        tree = json.loads(path.read_text(encoding="utf-8"))
        authored = pending = 0
        for key, translations in source.items():
            value = translations.get(lang)
            if value is None:
                value = translations["en"]
                pending += 1
            else:
                authored += 1
            set_path(tree, key, value)
        path.write_text(json.dumps(tree, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        stats[lang] = {"authored": authored, "pending": pending}
    print(json.dumps(stats, ensure_ascii=False))


if __name__ == "__main__":
    main()
