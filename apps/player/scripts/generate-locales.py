#!/usr/bin/env python3
"""Keep every Kinosail locale aligned with the canonical English catalog."""

from __future__ import annotations

import json
from pathlib import Path
import re


LOCALES = Path(__file__).resolve().parents[1] / "internal" / "server" / "locales"
PLACEHOLDER = re.compile(r"{{[^{}]+}}|\{[A-Za-z][A-Za-z0-9_]*}")


def read(path: Path) -> dict[str, str]:
    return {message["id"]: message["other"] for message in json.loads(path.read_text())}


def main() -> None:
    english = read(LOCALES / "active.en.json")
    for path in sorted(LOCALES.glob("active.*.json")):
        values = read(path)
        missing = [message for message in english if not values.get(message)]
        catalog = []
        for message, source in english.items():
            translated = values.get(message, source)
            if PLACEHOLDER.findall(translated) != PLACEHOLDER.findall(source):
                raise SystemExit(f"{path.name}: placeholders changed for {message!r}")
            catalog.append({"id": message, "other": translated})
        path.write_text(json.dumps(catalog, ensure_ascii=False, indent=2) + "\n")
        print(f"{path.name}: {len(catalog)} messages, {len(missing)} new English fallbacks")


if __name__ == "__main__":
    main()
