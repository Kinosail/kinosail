#!/usr/bin/env python3
"""Prepare a disposable MFA Owner, then run existing real settings journeys."""
import argparse
import base64
import hashlib
import hmac
import json
import os
from pathlib import Path
import struct
import ssl
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request


parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--url', required=True)
parser.add_argument('--output', type=Path, required=True)
parser.add_argument('--required-title', action='append', help='Require this exact populated journey title; repeat for each selected journey')
parser.add_argument('command', nargs=argparse.REMAINDER)
args = parser.parse_args()
if len(args.url) > 2048 or any(ord(character) <= 32 or ord(character) == 127 for character in args.url):
    parser.error('invalid loopback test Server URL')
try:
    url = urllib.parse.urlsplit(args.url)
    port = url.port
except ValueError:
    parser.error('invalid loopback test Server URL')
if (url.scheme not in ('http', 'https') or
        url.hostname not in ('localhost', '127.0.0.1') or url.username or url.password or
        url.path or '?' in args.url or '#' in args.url or port is None or not 1 <= port <= 65535):
    parser.error('requires the fresh supported loopback test Server')
args.url = f'{url.scheme}://{url.hostname}:{port}'
tls_context = None
if url.scheme == 'https':
    certificate = os.environ.get('NODE_EXTRA_CA_CERTS', '')
    helper = Path(__file__).with_name('browser-fixture-tls.sh')
    valid = subprocess.run(['bash', '-c',
        'source "$1"; browser_fixture_uses_tls && validate_browser_fixture_tls && validate_browser_fixture_ca "$2"',
        'fixture', str(helper), certificate], capture_output=True)
    if valid.returncode != 0:
        parser.error('HTTPS requires the validated disposable WebKit public CA')
    tls_context = ssl.create_default_context(cafile=certificate)
command = args.command[1:] if args.command[:1] == ['--'] else args.command
if not command:
    parser.error('requires a browser command')
if args.required_title and any(not title.strip() or len(title) > 240 or '\n' in title or '\r' in title for title in args.required_title):
    parser.error('required journey titles must be bounded nonempty single lines')
args.output.mkdir(parents=True, exist_ok=False)
receipt = {'command': command, 'urlScheme': url.scheme, 'ownerSetup': 'pending',
           'sourceRevision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
           'helperSHA256': hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
           'browserCertificateBypasses': 'disabled', 'selection': 'two existing populated search journeys'}
if args.required_title:
    receipt['selection'] = 'explicit required populated journeys'
    receipt['requiredTitles'] = args.required_title


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, file, code, message, headers, new_url):
        return None


opener = urllib.request.build_opener(NoRedirect, urllib.request.HTTPSHandler(context=tls_context))


def call(path, method, body=None, token='', expected=200):
    headers = {'Content-Type': 'application/json'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    request = urllib.request.Request(args.url + path, data=json.dumps(body).encode() if body is not None else None,
                                     headers=headers, method=method)
    try:
        with opener.open(request, timeout=10) as response:
            if response.status != expected:
                raise RuntimeError('Unexpected Owner preparation HTTP status')
            content = response.read()
            return json.loads(content) if response.headers.get_content_type() == 'application/json' else None
    except urllib.error.HTTPError as error:
        if expected == error.code == 303 and error.headers.get('Location') == '/':
            error.close()
            return None
        raise RuntimeError(f'Owner preparation {path} returned HTTP {error.code}') from None


def verify_results(path, required_titles=None):
    results = json.loads(path.read_text())
    wanted = set(required_titles) if required_titles is not None else {
        'settings search crosses levels and preserves unsaved preferences',
        'Owner settings search finds a setting across task families'}
    found = []

    def walk(suite):
        for spec in suite.get('specs', []):
            if spec['title'] in wanted:
                tests = spec['tests']
                if not tests or any(test['status'] != 'expected' or not test['results'] or
                                    any(result['status'] != 'passed' for result in test['results']) for test in tests):
                    raise RuntimeError('A required populated journey did not pass')
                found.append(spec['title'])
        for child in suite.get('suites', []):
            walk(child)

    for suite in results['suites']:
        walk(suite)
    if not wanted or set(found) != wanted or len(found) != len(wanted):
        raise RuntimeError('Every required populated journey must execute exactly once')
    return {'passed': found, 'resultsSHA256': hashlib.sha256(path.read_bytes()).hexdigest()}


exit_code = 1
try:
    owner = call('/api/v1/setup', 'POST', {'name': 'Owner', 'password': 'test-instance-password', 'totp': True}, expected=201)
    secret = owner['totp']['secret']
    digest = hmac.new(base64.b32decode(secret), struct.pack('>Q', int(time.time() / 30)), hashlib.sha1).digest()
    offset = digest[-1] & 15
    code = f'{(struct.unpack(">I", digest[offset:offset + 4])[0] & 0x7fffffff) % 1000000:06d}'
    if call('/api/v1/me/mfa', 'PUT', {'code': code}, owner['token']) != {'enabled': True}:
        raise RuntimeError('Owner MFA confirmation was not enabled')
    call('/onboarding/finish', 'GET', token=owner['token'], expected=303)
    receipt['ownerSetup'] = 'API MFA confirmed; onboarding complete'
    project = os.environ.get('KINOSAIL_BROWSER_PROJECT', 'chromium')
    env = dict(os.environ, KINOSAIL_TEST_INSTANCE='1', KINOSAIL_TEST_TOTP_SECRET=secret,
               KINOSAIL_E2E_URL=args.url, KINOSAIL_E2E_ARTIFACT_DIR=str(args.output),
               KINOSAIL_E2E_OUTPUT_DIR=str(args.output / 'browser-results'), KINOSAIL_BROWSER_WORKERS='1',
               PLAYWRIGHT_HTML_OUTPUT_DIR=str(args.output / f'html-{project}'),
               PLAYWRIGHT_JSON_OUTPUT_FILE=str(args.output / f'results-{project}.json'))
    exit_code = subprocess.run(command, env=env, check=False).returncode
    if exit_code == 0:
        receipt['journeys'] = verify_results(args.output / f'results-{project}.json', args.required_title)
except (RuntimeError, KeyError, OSError, ValueError) as error:
    # Setup response bodies and credentials are deliberately absent from diagnostics.
    receipt['errorClass'] = type(error).__name__
    print('Populated settings preparation or verification failed; inspect the private E2E artifact.', flush=True)
    exit_code = 1
finally:
    receipt['exitCode'] = exit_code
    (args.output / 'setup-and-run.json').write_text(json.dumps(receipt, indent=2) + '\n')
raise SystemExit(exit_code)
