#!/usr/bin/env python3
"""Build native interface catalogs from Player's existing web translations."""

from __future__ import annotations

import json
from pathlib import Path
import re
import sys


ROOT = Path(__file__).resolve().parents[1]
LOCALES = ROOT / "internal/server/locales"
APPLE = ROOT / "apps/native/Resources/Localizable.xcstrings"
ANDROID = ROOT / "apps/android/app/src/main/assets/interface-copy"
ANDROID_LOCALES = ROOT / "apps/android/app/src/main/res/xml/locale_config.xml"
WEAR = ROOT / "apps/android/wear/src/main/assets/interface-copy"
WEAR_LOCALES = ROOT / "apps/android/wear/src/main/res/xml/locale_config.xml"
TRANSLATED = (
    "es", "de", "fr", "pt-BR", "zh-Hans", "it", "nl", "pl", "ru", "ja", "ko",
    "ar", "tr", "uk", "pt-PT", "zh-Hant", "sv",
)
QUOTED = re.compile(r'"([^"\\\n]+)"')


def read(tag: str) -> dict[str, str]:
    return {entry["id"]: entry["other"] for entry in json.loads((LOCALES / f"active.{tag}.json").read_text())}


def emit(path: Path, value: object, check: bool) -> bool:
    content = json.dumps(value, ensure_ascii=False, indent=2, sort_keys=True) + "\n"
    if check:
        return path.is_file() and path.read_text() == content
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content)
    return True


def emit_text(path: Path, content: str, check: bool) -> bool:
    if check:
        return path.is_file() and path.read_text() == content
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content)
    return True


def main() -> int:
    check = sys.argv[1:] == ["--check"]
    if sys.argv[1:] and not check:
        raise SystemExit("usage: generate-native-copy.py [--check]")
    shared = (ROOT.parents[1] / "packages/localization/languages.go").read_text()
    declared = re.search(r"var translatedLanguageTags = map\[string\]struct\{\}\{(.*?)\n\}", shared, re.S)
    if declared is None or set(re.findall(r'"([^"]+)"\s*:', declared[1])) != {"en", *TRANSLATED}:
        raise SystemExit("native locale list differs from the shared translated locales")
    source = read("en")
    used: set[str] = set()
    for folder, suffix in ((ROOT / "apps/native/Sources", ".swift"), (ROOT / "apps/native/Watch", ".swift"),
                           (ROOT / "apps/android/app/src/main/java", ".kt"),
                           (ROOT / "apps/android/wear/src/main/java", ".kt")):
        for path in folder.rglob(f"*{suffix}"):
            used.update(QUOTED.findall(path.read_text()))
    english = {key: value for key, value in source.items() if key in used and key[0].isupper()}
    strings = {key: {"localizations": {}} for key in english}
    ok = True
    for tag in TRANSLATED:
        values = read(tag)
        if values.keys() != source.keys():
            raise SystemExit(f"{tag}: message IDs differ from English")
        translated = {key: values[key] for key in english if values[key] != english[key]}
        ok &= emit(ANDROID / f"{tag}.json", translated, check)
        ok &= emit(WEAR / f"{tag}.json", translated, check)
        for key, value in translated.items():
            strings[key]["localizations"][tag] = {"stringUnit": {"state": "translated", "value": value}}
    ok &= emit(APPLE, {"sourceLanguage": "en", "strings": strings, "version": "1.0"}, check)
    config = '<locale-config xmlns:android="http://schemas.android.com/apk/res/android">\n'
    config += '    <locale android:name="en" />\n'
    config += ''.join(f'    <locale android:name="{tag}" />\n' for tag in TRANSLATED)
    config += '</locale-config>\n'
    ok &= emit_text(ANDROID_LOCALES, config, check)
    ok &= emit_text(WEAR_LOCALES, config, check)
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
