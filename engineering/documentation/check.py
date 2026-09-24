#!/usr/bin/env python3
"""Check rendered docs: local links, anchors, assets, headings, and search."""
from html.parser import HTMLParser
from seo_check import validate
import json
import re
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
    for stylesheet in root.rglob('*.css'):
        for asset in re.findall(r'url\([\"\']?([^\"\')]+)', stylesheet.read_text()):
            if not urlsplit(asset).scheme and not (stylesheet.parent / asset).resolve().is_file():
                errors.append(f'{stylesheet.relative_to(root)}: missing CSS asset {asset}')
    if not (root / 'assets/fonts/OFL-Manrope.txt').is_file():
        errors.append('bundled font license missing')
    robots = root / 'robots.txt'
    homepage = (root / 'index.html').read_text()
    canonical = re.search(r'<link rel="canonical" href="(https://[^/]+)', homepage)
    expected_sitemap = f'Sitemap: {canonical.group(1)}{base}/sitemap.xml' if canonical else ''
    if not canonical or not robots.is_file() or expected_sitemap not in robots.read_text().splitlines():
        errors.append('robots.txt must name this build\'s sitemap')
    entries = json.loads((root / 'search.json').read_text())
    for entry in entries:
        if not all(isinstance(entry.get(key), str) for key in ('title', 'description', 'url', 'content', 'product')):
            errors.append('invalid search schema')
        if not entry['url'].startswith(base + '/'):
            errors.append('search URL escapes base')
    for entry in entries:
        relative = entry['url'][len(base) + 1:]
        document = root / relative / 'index.html' if entry['url'].endswith('/') else root / relative
        if document.is_file():
            # The canonical origin is supplied by the build, including preview builds.
            source = document.read_text()
            canonical = re.search(r'<link rel="canonical" href="(https://[^/]+)', source)
            origin = canonical.group(1) if canonical else ''
            errors.extend(f'{relative}: {error}' for error in validate(source, origin + entry['url']))
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
    check(Path(sys.argv[1]).resolve(), sys.argv[2] if len(sys.argv) > 2 else '')
