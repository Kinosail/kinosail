#!/usr/bin/env python3
"""Notify IndexNow about public documentation URLs changed by a main push."""
import json
from pathlib import Path
import re
import subprocess
import sys
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

ROOT = Path(__file__).resolve().parents[2]
SOURCE = 'apps/player/docs/'
ORIGIN = 'https://kinosail.com'
GLOBAL = {'apps/player/docs/_config.yml', 'engineering/documentation/build.py'}
ENDPOINT = 'https://api.indexnow.org/indexnow'


def public_key():
    files = [path for path in (ROOT / SOURCE).iterdir()
             if re.fullmatch(r'[0-9a-f]{32}\.txt', path.name)]
    if len(files) != 1 or not files[0].is_file() or files[0].stat().st_size > 129:
        raise ValueError('expected exactly one public IndexNow key file')
    key = files[0].stem
    if files[0].read_text() not in (key, key + '\n'):
        raise ValueError('public IndexNow key file content does not match its name')
    return key


def page_url(path):
    if len(path) > 512:
        raise ValueError('documentation path is too long')
    if not path.startswith(SOURCE):
        return None
    relative = path[len(SOURCE):]
    parts = relative.split('/')
    if (not relative or any(part.startswith('_') for part in parts)
            or parts[0] == 'research' or parts[-1] in {'README.md', '404.md'}):
        return None
    if parts[-1].endswith('.md'):
        parts[-1] = parts[-1][:-3]
    elif parts[-1].endswith('.html'):
        parts[-1] = parts[-1][:-5]
    else:
        return None
    if parts[-1] == 'index':
        parts.pop()
    if any(not re.fullmatch(r'[a-z0-9]+(?:-[a-z0-9]+)*', part) for part in parts):
        raise ValueError(f'unsupported documentation path: {path}')
    return f'{ORIGIN}/{"/".join(parts)}' + ('/' if parts else '')


def changed_urls(before, after):
    if any(not re.fullmatch(r'[0-9a-f]{40}', ref) for ref in (before, after)):
        raise ValueError('both revisions must be full lowercase commit hashes')
    subprocess.run(['git', 'cat-file', '-e', f'{after}^{{commit}}'], cwd=ROOT, check=True)
    if before == '0' * 40:
        paths = []
        all_pages = True
    else:
        subprocess.run(['git', 'cat-file', '-e', f'{before}^{{commit}}'], cwd=ROOT, check=True)
        subprocess.run(['git', 'merge-base', '--is-ancestor', before, after], cwd=ROOT, check=True)
        result = subprocess.run(['git', 'diff', '--no-renames', '--name-only', '-z', before, after,
                                 '--', SOURCE, 'engineering/documentation/build.py'], cwd=ROOT,
                                check=True, stdout=subprocess.PIPE)
        if len(result.stdout) > 1_000_000:
            raise ValueError('changed path list is too large')
        paths = [item.decode('utf-8') for item in result.stdout.split(b'\0') if item]
        all_pages = any(path in GLOBAL or path.startswith((SOURCE + '_includes/',
                                                           SOURCE + '_layouts/', SOURCE + '_data/'))
                        for path in paths)
    if all_pages:
        paths.extend(str(path.relative_to(ROOT)) for path in (ROOT / SOURCE).rglob('*') if path.is_file())
    urls = sorted({url for path in paths if (url := page_url(path))})
    if len(urls) > 10_000:
        raise ValueError('IndexNow accepts at most 10,000 URLs')
    return urls


def notify(urls):
    if not urls:
        print('IndexNow: no public documentation URLs changed')
        return
    if (len(urls) > 10_000 or len(urls) != len(set(urls))
            or any(not isinstance(url, str) or len(url) > 512
                   or not re.fullmatch(r'https://kinosail\.com/(?:[a-z0-9]+(?:-[a-z0-9]+)*/)*', url)
                   for url in urls)):
        raise ValueError('IndexNow URLs must be unique canonical documentation URLs')
    key = public_key()
    payload = json.dumps({'host': 'kinosail.com', 'key': key,
                          'keyLocation': f'{ORIGIN}/{key}.txt', 'urlList': urls}).encode('utf-8')
    if len(payload) > 1_000_000:
        raise ValueError('IndexNow payload is too large')
    request = Request(ENDPOINT, data=payload, headers={'Content-Type': 'application/json; charset=utf-8'})
    try:
        with urlopen(request, timeout=15) as response:
            status = response.status
    except HTTPError as error:
        status = error.code
        error.close()
    except URLError as error:
        raise RuntimeError('IndexNow could not be reached') from error
    if status not in (200, 202):
        raise RuntimeError(f'IndexNow rejected {len(urls)} URLs with HTTP {status}')
    print(f'IndexNow received {len(urls)} changed URLs (HTTP {status})'
          + ('; key validation pending' if status == 202 else ''))


if __name__ == '__main__':
    if len(sys.argv) != 3:
        raise SystemExit('usage: indexnow.py BEFORE_SHA AFTER_SHA')
    try:
        notify(changed_urls(sys.argv[1], sys.argv[2]))
    except (ValueError, subprocess.CalledProcessError, UnicodeError, RuntimeError) as error:
        raise SystemExit(str(error)) from error
