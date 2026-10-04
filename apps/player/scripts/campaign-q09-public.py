#!/usr/bin/env python3
"""Run three bounded Q09 groups; publish only exact safe admission evidence."""
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import time

ROOT = Path(__file__).resolve().parents[3]
MODES = ('isolated', 'server', 'phone')


def titles(mode):
    if mode not in MODES:
        raise ValueError('unknown proof group')
    if mode == 'phone':
        return ['actual Go phone: fictional download controls and ready Play link remain reachable after explicit scrolling']
    prefix = 'real Server' if mode == 'server' else 'isolated native browser'
    rows = [f'{prefix}: Pause retains verified {storage} blocks and Resume continues missing ranges at {width}px'
            for storage in ('opfs', 'indexeddb')
            for width in ([390, 1440, 1920] if mode == 'server' else [390 if storage == 'opfs' else 1440])]
    rows += ['same Viewer Profile in another tab preserves the transfer owner until explicit Pause',
             'navigation interrupts the transfer and reload offers explicit Resume with verified data']
    if mode == 'isolated':
        rows += ['isolated native profile boundary: changing Viewer Profile still cancels the old owner']
    return rows


def verify_results(path, mode):
    if path.is_symlink() or not 0 < path.stat().st_size <= 32 * 1024 * 1024:
        raise ValueError('invalid browser report size')
    data = json.loads(path.read_text())
    if not isinstance(data, dict) or data.get('errors'):
        raise ValueError('global browser error')
    rows = []
    def walk(suites, depth=0):
        if not isinstance(suites, list) or depth > 10 or len(suites) > 100:
            raise ValueError('invalid suite tree')
        for suite in suites:
            if not isinstance(suite, dict) or not isinstance(suite.get('specs', []), list):
                raise ValueError('invalid suite')
            rows.extend(suite.get('specs', []))
            walk(suite.get('suites', []), depth + 1)
    walk(data.get('suites'))
    expected = titles(mode)
    if len(rows) != len(expected) or sorted(row.get('title', '') for row in rows) != sorted(expected):
        raise ValueError('case selection mismatch')
    safe = []
    for title in expected:
        row = next(row for row in rows if row['title'] == title)
        cases = row.get('tests')
        if not isinstance(cases, list) or len(cases) != 1:
            raise ValueError('unexpected project count')
        case = cases[0]
        attempts = case.get('results')
        if (case.get('projectName') != 'chromium' or case.get('status') not in ('expected', 'unexpected')
                or not isinstance(attempts, list) or len(attempts) != 1):
            raise ValueError('wrong project, skip, flaky or retry')
        attempt = attempts[0]
        status = attempt.get('status')
        if (status not in ('passed', 'failed', 'timedOut', 'interrupted')
                or type(attempt.get('retry')) is not int or attempt['retry'] != 0
                or (case['status'] == 'expected') != (status == 'passed')):
            raise ValueError('invalid result admission')
        safe.append({'case': len(safe) + 1, 'status': status, 'retry': 0})
    return {'mode': mode, 'count': len(safe), 'passed': all(row['status'] == 'passed' for row in safe), 'cases': safe}


def digest(path):
    hasher = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            hasher.update(block)
    return hasher.hexdigest()


def git(*args):
    return subprocess.run(['git', *args], cwd=ROOT, check=True, capture_output=True, timeout=10).stdout


def snapshot():
    names = git('ls-files', '-z', '--', 'apps/player', 'packages', 'scripts/ci', 'scripts/tooling',
                'go.work', 'go.work.sum').decode().split('\0')[:-1]
    extensions = {'.go', '.mod', '.sum', '.work', '.js', '.ts', '.css', '.html', '.json', '.yaml', '.py', '.sh'}
    selected = sorted(name for name in names if Path(name).suffix in extensions
                      and '/engineering/' not in name and '/docs/' not in name)
    if not selected or len(selected) > 5000:
        raise ValueError('invalid source inventory')
    return {name: {'bytes': (ROOT / name).stat().st_size, 'sha256': digest(ROOT / name)} for name in selected}


def execute(command, cwd, environment, log, bound):
    started = time.monotonic()
    timed_out = False
    with log.open('xb') as stream:
        process = subprocess.Popen(command, cwd=cwd, env=environment, stdout=stream,
                                   stderr=subprocess.STDOUT, start_new_session=True)
        try:
            code = process.wait(timeout=bound)
        except subprocess.TimeoutExpired:
            timed_out = True
            os.killpg(process.pid, signal.SIGTERM)
            try:
                code = process.wait(timeout=3)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                code = process.wait(timeout=3)
    return {'exitCode': code, 'timedOut': timed_out, 'seconds': round(time.monotonic() - started, 3),
            'externalBoundSeconds': bound, 'privateLogSHA256': digest(log)}


def main():
    os.umask(0o077)
    output = ROOT / '.verification/campaign-proof/Q09'
    private = ROOT / '.verification/campaign-private/Q09'
    output.mkdir(parents=True, exist_ok=False)
    private.mkdir(parents=True, exist_ok=False)
    receipt = {'schemaVersion': 1, 'item': 'Q09', 'startedUTC': datetime.now(timezone.utc).isoformat(),
               'scope': '5 isolated storage + 8 real Go Server + 1 real phone hit-target; no media decoding',
               'normalProtectedValidation': 'separate, required before merge', 'groups': []}
    results, inputs = [], {}
    accepted = False
    try:
        revision = git('rev-parse', 'HEAD').decode().strip()
        if not re.fullmatch('[a-f0-9]{40}', revision) or os.environ.get('GITHUB_SHA') != revision:
            raise ValueError('checkout revision mismatch')
        git('diff', '--quiet', 'HEAD', '--')
        inputs = snapshot()
        receipt['revision'] = revision
        tools = {}
        for name in ('go', 'node', 'pnpm'):
            binary = shutil.which(name)
            if not binary:
                raise ValueError('tool missing')
            tools[name] = {'binarySHA256': digest(Path(binary).resolve())}
            args = ['go', 'version'] if name == 'go' else [name, '--version']
            version = subprocess.run(args, capture_output=True, text=True, check=True, timeout=10).stdout.strip()
            if not re.fullmatch(r'[A-Za-z0-9 ._/-]{1,120}', version):
                raise ValueError('invalid tool version')
            tools[name]['version'] = version
        receipt['tools'] = tools
        browser_paths = sorted((Path.home() / '.cache/ms-playwright').glob('chromium-*/chrome-linux*/chrome'))
        browser_paths += sorted((Path.home() / '.cache/ms-playwright').glob('chromium_headless_shell-*/chrome-linux*/headless_shell'))
        browser_paths += sorted((Path.home() / '.cache/ms-playwright').glob('chromium_headless_shell-*/chrome-headless-shell-linux*/chrome-headless-shell'))
        if not browser_paths:
            raise ValueError('installed Chromium binary unavailable')
        receipt['browserBinaries'] = [{'file': path.name, 'sha256': digest(path)} for path in browser_paths]
        base = {key: value for key, value in os.environ.items() if not key.startswith('KINOSAIL_') and key != 'GOFLAGS'}
        base.update(GOMAXPROCS='2', GOPROXY='off', GOTOOLCHAIN='local', PLAYWRIGHT_CHANNEL='',
                    KINOSAIL_BROWSER_PROJECT='chromium', KINOSAIL_BROWSER_WORKERS='1', KINOSAIL_E2E_VIDEO='off')
        for mode in MODES:
            group = private / mode
            group.mkdir()
            environment = base | {'KINOSAIL_E2E_OUTPUT_DIR': str(group / 'browser'),
                                  'KINOSAIL_E2E_ARTIFACT_DIR': str(group / 'report')}
            if mode == 'isolated':
                environment['KINOSAIL_DOWNLOAD_PAUSE_ISOLATED'] = '1'
                command = ['node', 'node_modules/@playwright/test/cli.js', 'test', 'download-pause.spec.ts',
                           'download-pause-ownership.spec.ts', '--workers=1', '--project=chromium', '--retries=0', '--forbid-only']
                cwd, bound = ROOT / 'apps/player/e2e', 60
            else:
                environment.update(KINOSAIL_DOWNLOAD_PAUSE_BROWSER='1',
                                   KINOSAIL_DOWNLOAD_PAUSE_HIT_TARGETS='1' if mode == 'phone' else '0')
                command = ['../../scripts/tooling/with-go-module.sh', 'go', 'test', '-json', '-p', '1',
                           './internal/server', '-run', '^TestDownloadPauseBrowserJourney$', '-count=1', '-timeout=70s']
                cwd, bound = ROOT / 'apps/player', 80
            record = {'mode': mode, 'command': command, 'cwd': str(cwd.relative_to(ROOT)),
                      **execute(command, cwd, environment, group / 'command.log', bound)}
            receipt['groups'].append(record)
            if record['timedOut']:
                raise ValueError('incomplete timed-out proof')
            result = verify_results(group / 'report/results-chromium.json', mode)
            result['reportSHA256'] = digest(group / 'report/results-chromium.json')
            results.append(result)
            if mode != 'isolated':
                events = []
                for line in (group / 'command.log').read_text(errors='replace').splitlines():
                    try:
                        event = json.loads(line)
                    except json.JSONDecodeError:
                        continue
                    if isinstance(event, dict) and event.get('Test') == 'TestDownloadPauseBrowserJourney':
                        events.append(event.get('Action'))
                record['goTopLevelPass'] = events.count('run') == 1 and events.count('pass') == 1 and not set(events) & {'fail', 'skip'}
            record['accepted'] = record['exitCode'] == 0 and result['passed'] and record.get('goTopLevelPass', True)
        git('diff', '--quiet', 'HEAD', '--')
        receipt['sourceUnchanged'] = inputs == snapshot() and git('rev-parse', 'HEAD').decode().strip() == revision
        accepted = receipt['sourceUnchanged'] and all(group['accepted'] for group in receipt['groups']) and len(results) == 3
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        receipt['failureClass'] = type(error).__name__
    finally:
        receipt.update(accepted=accepted, finishedUTC=datetime.now(timezone.utc).isoformat())
        for name, data in [('receipt.json', receipt), ('results.json', {'groups': results}),
                           ('source-manifest.json', {'inputs': inputs})]:
            (output / name).write_text(json.dumps(data, indent=2) + '\n')
        safe = {path.name: digest(path) for path in output.iterdir() if path.is_file()}
        (output / 'artifact-manifest.json').write_text(json.dumps({'safeFiles': safe}, indent=2) + '\n')
    print('Q09 bounded proof: ' + ('PASS (5 + 8 + 1)' if accepted else 'FAIL or incomplete; safe evidence retained'))
    return 0 if accepted else 1


if __name__ == '__main__':
    raise SystemExit(main())
