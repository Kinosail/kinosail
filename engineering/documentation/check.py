#!/usr/bin/env python3
"""Check rendered docs: local links, anchors, assets, headings, and search."""
from html.parser import HTMLParser
import json
from pathlib import Path
import sys
from urllib.parse import unquote, urljoin, urlsplit


class Page(HTMLParser):
    def __init__(self, source):
        super().__init__()
        self.ids = set()
        self.links = []
        self.h1 = 0
        self.feed(source)

    def handle_starttag(self, tag, attributes):
        attrs = dict(attributes)
        if attrs.get('id'):
            self.ids.add(attrs['id'])
        if tag == 'h1':
            self.h1 += 1
        for key in ('href', 'src'):
            if attrs.get(key):
                self.links.append(attrs[key])


def check(root, base):
    pages = {p: Page(p.read_text()) for p in root.rglob('*.html')}
    errors = []
    for path, page in pages.items():
        if page.h1 != 1:
            errors.append(f'{path.relative_to(root)}: expected one h1, found {page.h1}')
        public = base + '/' + path.relative_to(root).as_posix()
        for link in page.links:
            parsed = urlsplit(urljoin(public, link))
            if parsed.scheme or parsed.netloc:
                continue
            if not parsed.path.startswith(base + '/'):
                errors.append(f'{path.relative_to(root)}: link escapes base: {link}')
                continue
            target = root / unquote(parsed.path[len(base) + 1:])
            if target.is_dir():
                target /= 'index.html'
            if not target.exists():
                errors.append(f'{path.relative_to(root)}: missing {link}')
            elif parsed.fragment and target in pages and unquote(parsed.fragment) not in pages[target].ids:
                errors.append(f'{path.relative_to(root)}: missing anchor {link}')
    entries = json.loads((root / 'search.json').read_text())
    for entry in entries:
        if not all(isinstance(entry.get(key), str) for key in ('title', 'description', 'url', 'content', 'product')):
            errors.append('invalid search schema')
        if not entry['url'].startswith(base + '/'):
            errors.append('search URL escapes base')
    if len({entry['url'] for entry in entries}) != len(entries):
        errors.append('duplicate search URLs')
    if {'Player'} != {entry['product'] for entry in entries}:
        errors.append('search must contain only Player docs')
    for forbidden in ('research', '.env', '.git', 'Gemfile'):
        if any(p.name == forbidden for p in root.rglob('*')):
            errors.append(f'non-public build input copied: {forbidden}')
    if errors:
        raise SystemExit('\n'.join(errors))
    print(f'PASS: {len(pages)} HTML pages, all local links/anchors/assets, single headings, {len(entries)} search entries, public-only output')


if __name__ == '__main__':
    check(Path(sys.argv[1]).resolve(), sys.argv[2] if len(sys.argv) > 2 else '/kinosail')
