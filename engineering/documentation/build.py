#!/usr/bin/env python3
"""Build only public documentation into a new output directory."""
import argparse
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
from urllib.parse import urlparse

ROOT = Path(__file__).resolve().parents[2]
DOCS = ROOT / 'engineering/documentation'


def settings(argv=None):
    parser = argparse.ArgumentParser(description=__doc__, allow_abbrev=False)
    parser.add_argument('--output', required=True)
    parser.add_argument('--baseurl', default='/kinosail')
    parser.add_argument('--url', default='https://kinosail.github.io')
    args = parser.parse_args(argv)
    if len(args.baseurl) > 200 or not re.fullmatch(r'(?:/[A-Za-z0-9_-]+)*', args.baseurl):
        parser.error('baseurl must be empty or slash-separated URL segments without a trailing slash')
    try:
        origin = urlparse(args.url)
    except ValueError:
        parser.error('url must be a valid HTTPS origin')
    if len(args.url) > 253 or origin.scheme != 'https' or args.url != f'https://{origin.hostname}' or not origin.hostname or origin.netloc != origin.hostname or origin.path or origin.query or origin.fragment or not re.fullmatch(r'[a-z0-9]+(?:[.-][a-z0-9]+)*', origin.hostname):
        parser.error('url must be a lowercase HTTPS origin without credentials, port, or path')
    if len(args.output) > 4096 or not args.output.strip():
        parser.error('output must name a new directory')
    args.output = Path(args.output).resolve()
    if args.output.exists() or args.output.is_relative_to(ROOT):
        parser.error('output must not exist and must be outside the source checkout')
    return args


def build(args):
    env = os.environ | {'BUNDLE_GEMFILE': str(DOCS / 'Gemfile')}
    with tempfile.TemporaryDirectory(prefix='kinosail-docs-') as temporary:
        staging = Path(temporary)
        source = staging / 'source'
        artifact = staging / 'output'
        shutil.copytree(ROOT / 'apps/player/docs', source, ignore=shutil.ignore_patterns('research'))
        fonts = source / 'assets/fonts'
        fonts.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / 'packages/webassets/static/fonts/manrope.woff2', fonts / 'manrope.woff2')
        shutil.copyfile(ROOT / 'packages/webassets/static/fonts/OFL-Manrope.txt', fonts / 'OFL-Manrope.txt')
        configuration = {'url': args.url, 'baseurl': args.baseurl, 'docs_root': args.baseurl, 'product': 'Player'}
        config = staging / 'deployment.yml'
        config.write_text(json.dumps(configuration))
        subprocess.run(['bundle', 'exec', 'jekyll', 'build', '--source', str(source), '--destination', str(artifact), '--config', f'{source / "_config.yml"},{config}', '--strict_front_matter'], env=env, check=True)
        index = json.loads((artifact / 'search.json').read_text())
        (artifact / '.nojekyll').touch()
        urls = sorted({args.url + page['url'] for page in index})
        (artifact / 'sitemap.xml').write_text('<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">' + ''.join(f'<url><loc>{url}</loc></url>' for url in urls) + '</urlset>')
        shutil.copytree(artifact, args.output)
    print(f'Built {len(index)} searchable Player pages at {args.output}')


if __name__ == '__main__':
    build(settings())
