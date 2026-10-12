#!/usr/bin/env python3
"""Real HTTP controls for optional source-clock proof and owned process settlement.

Failures covered: normal gaps/delays mistaken for process errors, optional scans
consuming the preparation budget, incomplete scans certified after cancellation,
and genuine probe failures hidden by fallback. Fault modes wrap only the selected
audio-frame probe; ordinary media, Server, authentication and HLS remain real.
"""
import argparse
import json
import os
from pathlib import Path
import platform
import re
import shlex
import shutil
import socket
import subprocess
import sys
import threading
import time

from hls_followon_public import bounded_bytes, check
from hls_timeline_http import PublicServer, sha, source_state

ROOT = Path(__file__).resolve().parents[3]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--cases', default='gap,delay,coarse-gap,timeout,cancel,probe-error')
parser.add_argument('--movie-source', type=Path)
args = parser.parse_args()
cases = args.cases.split(',')
check(all(case in ['gap', 'delay', 'coarse-gap', 'timeout', 'cancel', 'probe-error', 'movie'] for case in cases), 'case_name')
run = ROOT / '.verification/hls-audio-source' / time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
run.mkdir(parents=True)
receipt = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip(),
           'command': shlex.join([sys.executable, str(Path(__file__).resolve()), *sys.argv[1:]]),
           'environment': platform.platform(), 'result': 'failed', 'cases': [], 'commands': [],
           'boundary': 'Real disposable Go Server and HTTP. Gap/delay/movie use real FFmpeg. Timeout/cancel/error inject faults only at the selected audio-frame probe. No browser or physical-device claim.'}


def command(argv, timeout=120):
    receipt['commands'].append(argv)
    subprocess.run(argv, cwd=ROOT, check=True, capture_output=True, timeout=timeout)


def processes(server):
    rows = subprocess.check_output(['ps', '-eo', 'pid=,ppid=,rss='], text=True, timeout=5).splitlines()
    table = [tuple(map(int, line.split())) for line in rows if len(line.split()) == 3]
    owned = {server.pid}
    while True:
        expanded = owned | {pid for pid, parent, _ in table if parent in owned}
        if expanded == owned:
            break
        owned = expanded
    return len(owned) - 1, sum(rss * 1024 for pid, _, rss in table if pid in owned)


def monitor(server, stop, facts):
    while not stop.is_set():
        try:
            children, rss = processes(server)
            facts['peakOwnedChildren'] = max(facts['peakOwnedChildren'], children)
            facts['peakTreeRSSBytes'] = max(facts['peakTreeRSSBytes'], rss)
            facts['samples'] += 1
        except (OSError, subprocess.SubprocessError):
            facts['samplingErrors'] += 1
        stop.wait(.05)


def records(path, position=0):
    raw = bounded_bytes(path, 8 << 20, 'private_log_bound')[position:].decode()
    result = []
    for line in raw.splitlines():
        if not line.endswith('}'):
            continue
        try:
            row = json.loads(line)
        except ValueError:
            continue
        result.append((row, line))
    return result


def perform(case, base):
    folder = run / case
    media = folder / 'media'
    media.mkdir(parents=True)
    source = media / ('Proof Sample.mkv' if case in ['movie', 'coarse-gap'] else 'Proof Sample.mp4')
    row = {'case': case, 'passed': False}
    receipt['cases'].append(row)
    if case == 'movie':
        check(args.movie_source is not None and args.movie_source.is_file(), 'movie_source_required')
        shutil.copyfile(args.movie_source, source)
    elif case in ['gap', 'delay', 'coarse-gap']:
        extra = ['-af', 'asetpts=PTS+if(gte(T\\,1)\\,0.5/TB\\,0)'] if case == 'gap' else ['-af', 'asetpts=PTS+0.5/TB']
        if case == 'coarse-gap':
            # One timestamp tick must remain a real discontinuity, even when
            # a remux changes the phase of the container's rounding rule.
            extra = ['-af', 'asetpts=PTS+if(gte(T\\,1)\\,0.001/TB\\,0)']
        command(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(base), '-map', '0:v:0', '-c:v', 'copy',
                 '-map', '0:a:0', '-c:a', 'ac3', '-ar', '44100' if case == 'coarse-gap' else '48000', *extra, '-t', '74.5', str(source)])
    else:
        shutil.copyfile(base, source)
    before = source_state(source)
    row['sourceBefore'] = before
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        port = listener.getsockname()[1]
    origin = f'http://localhost:{port}'
    env = dict(os.environ, KINOSAIL_LISTEN=f'127.0.0.1:{port}', KINOSAIL_AUTH_URL=origin,
               KINOSAIL_TLS_ENABLED='false', KINOSAIL_DATA_DIR=str(folder / 'config'),
               KINOSAIL_MEDIA_DIR=str(media), KINOSAIL_CACHE_DIR=str(folder / 'cache'),
               KINOSAIL_BACKUP_DIR=str(folder / 'backups'), KINOSAIL_LOG_LEVEL='debug',
               KINOSAIL_BACKUP_KEY='synthetic-audio-source-key')
    marker = folder / 'audio-proof-started'
    if case in ['timeout', 'cancel', 'probe-error']:
        wrapper = folder / 'probe-wrapper.py'
        wrapper.write_text('#!' + sys.executable + '\nimport os, sys, time\nfrom pathlib import Path\n'
            + "if '-show_frames' in sys.argv and '-select_streams' in sys.argv and sys.argv[sys.argv.index('-select_streams')+1] == 'a:0':\n"
            + f'    Path({str(marker)!r}).write_text("started")\n'
            + ('    raise SystemExit(23)\n' if case == 'probe-error' else '    time.sleep(20)\n')
            + f'os.execv({shutil.which("ffprobe")!r}, [{shutil.which("ffprobe")!r}, *sys.argv[1:]])\n')
        wrapper.chmod(0o755)
        row['probeWrapperSHA256'] = sha(wrapper)
        env['KINOSAIL_FFPROBE'] = str(wrapper)
    log_path = folder / 'server-private.log'
    with log_path.open('w') as log:
        server = subprocess.Popen([str(run / 'player')], cwd=ROOT, env=env, stdout=log, stderr=log)
        stop = threading.Event()
        resources = {'peakOwnedChildren': 0, 'peakTreeRSSBytes': 0, 'samples': 0, 'samplingErrors': 0}
        watcher = threading.Thread(target=monitor, args=(server, stop, resources))
        watcher.start()
        try:
            api = PublicServer(origin)
            api.authorize()
            item = next(item for item in api.call('/api/v1/library')['items'] if item['title'] == 'Proof Sample')
            plan = api.call('/api/v1/items/' + item['id'] + '/playback?videoCodecs=h264&audioCodecs=aac')
            hls = plan['compatible'].replace('/index.m3u8', '-o12500/index.m3u8')
            prepare = '/api/v1/items/' + item['id'] + '/playback-prepare'
            position, started = log_path.stat().st_size, time.monotonic()
            status, data, headers = api.http(prepare, 'POST', {'source': hls})
            request_id = next((v for k, v in headers.items() if k.lower() == 'x-request-id'), '')
            check(status == 202 and json.loads(data).get('state') == 'queued', 'prepare_queued')
            check(re.fullmatch(r'[a-zA-Z0-9_-]{1,80}', request_id), 'request_id')
            if case == 'cancel':
                while not marker.exists() and time.monotonic() - started < 8:
                    time.sleep(.01)
                check(marker.exists(), 'cancel_reached_audio_proof')
                row['cancelStatus'] = api.http(prepare, 'DELETE')[0]
                check(row['cancelStatus'] == 204, 'cancel_status')
            completion, settled = None, 0
            while time.monotonic() - started < 20:
                entries = records(log_path, position)
                states = [r['state'] for r, _ in entries if r.get('msg') == 'HLS startup preparation'
                          and r.get('request_id') == request_id and r.get('state') in ['ready', 'cancelled', 'unavailable', 'bounded']]
                completion = states[-1] if states else None
                settled = settled + 1 if completion and processes(server)[0] == 0 else 0
                if settled >= 3:
                    break
                time.sleep(.05)
            row.update(completionState=completion, elapsedSeconds=time.monotonic() - started,
                       joinedSamples=settled, requestID=request_id)
            check(settled >= 3, 'owned_process_not_joined')
            expected = ['cancelled', 'unavailable'] if case == 'cancel' else ['ready']
            check(completion in expected, 'preparation_completion')
            diagnostics = []
            for entry, line in entries:
                if entry.get('msg') not in ['HLS audio source clock', 'copied probe completion failed']:
                    continue
                check(entry.get('request_id') == request_id, 'diagnostic_correlation')
                check(len(line) <= 2048 and api.token not in line and str(source) not in line
                      and 'synthetic-audio-source-key' not in line, 'diagnostic_secret_or_bound')
                diagnostics.append({key: entry[key] for key in ['msg', 'level', 'outcome', 'failure_class', 'request_id'] if key in entry})
            row['diagnostics'] = diagnostics
            errors = [entry for entry in diagnostics if entry['level'] == 'ERROR']
            clocks = [entry for entry in diagnostics if entry['msg'] == 'HLS audio source clock']
            check(len(errors) == (1 if case == 'probe-error' else 0), 'unexpected_probe_errors')
            if case != 'cancel':
                check(len(clocks) == 1 and clocks[0]['level'] == ('DEBUG' if case == 'movie' else 'WARN'), 'clock_diagnostic')
                check(clocks[0]['outcome'] == ('certified' if case == 'movie' else 'legacy-seek'), 'clock_outcome')
                timeline_file = next((folder / 'cache').glob('*/.copy-timeline'))
                timeline = json.loads(timeline_file.read_text())
                row['audioSource'] = timeline.get('AudioSource')
                check(bool(row['audioSource']) == (case == 'movie'), 'clock_certification')
                check(api.http(hls)[0] == 200, 'fallback_master')
                variant = next(timeline_file.parent.glob('*p/index.m3u8'))
                fragment = variant.with_name('segment-00000.m4s')
                check(fragment.exists() and fragment.stat().st_size > 0, 'fallback_initial_fragment')
                row['initialFragmentSHA256'] = sha(fragment)
                if case == 'timeout':
                    check(row['elapsedSeconds'] < 10, 'optional_proof_budget')
            row['passed'] = True
        finally:
            stop.set()
            watcher.join(timeout=10)
            server.terminate()
            try:
                server.wait(timeout=15)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait()
            row.update(resources=resources, sourceUnchanged=before == source_state(source), serverSettled=server.poll() is not None)
            row['privateLogSHA256'] = sha(log_path)
            check(row['sourceUnchanged'] and row['serverSettled'] and not watcher.is_alive(), 'fixture_settlement')


try:
    command(['go', 'build', '-p', '2', '-o', str(run / 'player'), './apps/player/cmd/kinosail'], 180)
    receipt['binarySHA256'] = sha(run / 'player')
    base = run / 'base.mp4'
    command(['ffmpeg', '-nostdin', '-v', 'error', '-f', 'lavfi', '-i', 'testsrc2=s=320x180:r=24:d=74.5',
             '-f', 'lavfi', '-i', 'sine=frequency=440:sample_rate=48000:duration=74.5',
             '-c:v', 'libx264', '-threads', '2', '-preset', 'ultrafast', '-g', '144',
             '-c:a', 'ac3', '-ac', '2', str(base)])
    for case in cases:
        perform(case, base)
    receipt['result'] = 'passed'
except Exception as error:
    receipt['failureClass'] = str(error) if isinstance(error, RuntimeError) else type(error).__name__
finally:
    (run / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(json.dumps({'receipt': str(run / 'receipt.json'), 'result': receipt['result']}), flush=True)
raise SystemExit(0 if receipt['result'] == 'passed' else 1)
