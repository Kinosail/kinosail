"""Owned actual Go HTTP process and public cached-delivery controls."""
import json
import os
import re
import shutil
import socket
import subprocess
import threading
import time
from hls_followon_public import bounded_bytes, check, encoder_count, sample_resources
from hls_remaining_process import finish_processes
from hls_timeline_http import PublicServer, sha, source_state
from hls_timeline_packets import manifest_facts
from hls_aac_v2_public_evidence import assert_fixed_timeline
from hls_aac_v2_source_identity import matches_source_input, option, regular_identity, startup_fixture


class ActualServer:
    def __init__(self, root, directory, source, binary):
        self.root, self.directory, self.source, self.binary = root, directory, source, binary
        with socket.socket() as listener:
            listener.bind(('127.0.0.1', 0))
            port = listener.getsockname()[1]
        self.api = PublicServer('http://localhost:' + str(port))
        self.log_path = directory / 'server-private.log'
        self.invocations = directory / 'ffmpeg-private.jsonl'
        real = shutil.which('ffmpeg')
        probe = shutil.which('ffprobe')
        check(real is not None and probe is not None, 'v2_pinned_codec_missing')
        adapter = directory / 'owned-ffmpeg'
        adapter.write_text("#!/usr/bin/python3\n"
            "import json,os,sys\n"
            "args=sys.argv[1:]\n"
            "if \"-hls_time\" in args:\n"
            "    sys.path.insert(0,os.environ[\"V2_OBSERVER_ROOT\"])\n"
            "    from hls_aac_v2_source_identity import captured_input_identity\n"
            "    row={\"args\":args,\"parent\":os.getppid()}\n"
            "    try:\n"
            "        row[\"inputWitness\"]=captured_input_identity(args)\n"
            "    except RuntimeError:\n"
            "        row[\"inputWitnessFailureClass\"]=\"v2_actual_input_identity_unqualified\"\n"
            "    with open(os.environ[\"V2_ARGV_CAPTURE\"],\"a\") as stream:\n"
            "        stream.write(json.dumps(row)+\"\\n\")\n"
            "os.execv(os.environ[\"V2_REAL_FFMPEG\"],[os.environ[\"V2_REAL_FFMPEG\"],*args])\n")
        adapter.chmod(0o700)
        self.env = dict(os.environ, KINOSAIL_LISTEN='127.0.0.1:' + str(port),
            KINOSAIL_AUTH_URL=self.api.url, KINOSAIL_TLS_ENABLED='false',
            KINOSAIL_DATA_DIR=str(directory / 'config'), KINOSAIL_MEDIA_DIR=str(source.parent),
            KINOSAIL_CACHE_DIR=str(directory / 'cache'), KINOSAIL_BACKUP_DIR=str(directory / 'backups'),
            KINOSAIL_BACKUP_KEY='synthetic-v2-public-key', KINOSAIL_FFMPEG=str(adapter),
            KINOSAIL_FFPROBE=probe, V2_ARGV_CAPTURE=str(self.invocations), V2_REAL_FFMPEG=real,
            V2_OBSERVER_ROOT=str(root / 'apps/player/scripts'))
        self.process, self.log, self.sampler = None, None, None
        self.sessions = []
        self.owned_pids = set()
        self.source_witness = regular_identity(source)
        self.before = source_state(source)
        check(regular_identity(source) == self.source_witness, 'v2_initial_source_identity_unqualified')

    def start(self, authorize=False, binary=None):
        check(self.process is None, 'v2_session_already_owned')
        self.log = self.log_path.open('a')
        self.process = subprocess.Popen([str(binary or self.binary)], cwd=self.root, env=self.env,
            stdout=self.log, stderr=self.log, start_new_session=True)
        self.process._copiedSourceWitness = dict(self.source_witness)
        self.owned_pids.add(self.process.pid)
        self.stop_event = threading.Event()
        self.resources = {'samples': 0, 'peakOwnedFFmpeg': 0, 'samplingErrors': 0}
        self.sampler = threading.Thread(target=sample_resources,
            args=(self.process, self.source, self.stop_event, self.resources), daemon=True)
        self.sampler.start()
        if authorize:
            self.api.authorize()
        else:
            deadline = time.monotonic() + 30
            healthy = False
            while time.monotonic() < deadline:
                try:
                    healthy = self.api.http('/healthz', authenticated=False)[0] == 200
                    if healthy:
                        break
                except OSError:
                    pass
                time.sleep(0.1)
            check(healthy, 'v2_server_reopen_health')
        return self

    def stop(self):
        if self.process is None:
            return
        process = self.process
        try:
            result = finish_processes(process, self.source, self.stop_event, self.sampler)
        except Exception as error:
            self.sessions.append({'teardownFailureClass': type(error).__name__, 'ownershipRetained': True})
            raise
        result['resources'] = dict(self.resources)
        result['sourceUnchanged'] = source_state(self.source) == self.before
        self.sessions.append(result)
        joined = result.get('ownedProcessJoin', {})
        settled = (process.poll() is not None and joined.get('confirmedZeroSamples') == 2
                   and not joined.get('qualificationFailures') and not result['cleanupFailures'])
        if settled:
            self.process = None
            self.log.close()
            self.log = None
        check(settled and result['sourceUnchanged'] and self.resources['samples'] > 0
              and self.resources['peakOwnedFFmpeg'] <= 1 and self.resources['samplingErrors'] == 0,
              'v2_owned_server_resource_join')

    def invocation_rows(self):
        if not self.invocations.exists():
            return []
        rows = [json.loads(row) for row in bounded_bytes(self.invocations, 65536, 'v2_argv_bound').splitlines()]
        check(len(rows) <= 64, 'v2_all_hls_invocation_bound')
        for row in rows:
            args = row.get('args')
            check(type(args) is list and 0 < len(args) <= 128
                  and all(type(value) is str and len(value) <= 2048 for value in args),
                  'v2_actual_argv_shape')
        return rows

    def source_invocation_rows(self):
        return source_invocations(self.invocation_rows(), self.source, self.owned_pids)


def source_invocations(rows, source, owned_pids):
    """Keep every actual source call; allow only the identified bounded codec fixture."""
    selected = []
    for row in rows:
        check(type(row.get('parent')) is int and row['parent'] in owned_pids, 'v2_argv_foreign_parent')
        args = row['args']
        inputs = [args[n + 1] for n, value in enumerate(args[:-1]) if value == '-i']
        check(len(inputs) == 1, 'v2_argv_single_input_required')
        if matches_source_input(inputs[0], row, source):
            selected.append(row)
            continue
        check(startup_fixture(inputs[0], args), 'v2_unknown_non_source_hls_invocation')
    check(len(selected) <= 16, 'v2_encoder_invocation_bound')
    return selected


def diagnostic_producer_rows(rows, source, owned_pids):
    """Non-throwing whitelist projection keeps a failed counter's cause."""
    output = []
    for row in rows:
        args = row['args']
        fields = {'ownedParent': row['parent'] in owned_pids,
                  'exactSourceInputs': sum(args[n + 1] == str(source)
                      for n, value in enumerate(args[:-1]) if value == '-i')}
        for name in ['-ss', '-start_number', '-output_ts_offset', '-hls_time']:
            values = [args[n + 1] for n, value in enumerate(args[:-1]) if value == name]
            fields[name.removeprefix('-')] = [value if re.fullmatch(r'-?[0-9]{1,12}(?:\.[0-9]{1,9})?', value)
                                              else 'other' for value in values]
        for name in ['-c:a', '-c:v']:
            values = [args[n + 1] for n, value in enumerate(args[:-1]) if value == name]
            fields[name.removeprefix('-')] = [value if value in ['copy', 'aac', 'libx264', 'libx265'] else 'other'
                                              for value in values]
        fields['videoCopyPriorSS'] = '-copypriorss:v' in args
        fields['globalCopyPriorSS'] = '-copypriorss' in args
        output.append(fields)
    return output

def actual_producer_rows(rows):
    output = []
    for row in rows:
        args = row['args']
        avoid = [n for n, value in enumerate(args[:-1]) if value == '-avoid_negative_ts']
        check(1 <= len(avoid) <= 2 and all(args[n + 1] == 'disabled' for n in avoid)
              and max(avoid) > args.index('-i'), 'v2_output_negative_ts_policy')
        check(option(args, '-c:a') == option(args, '-c:v') == 'copy'
              and '-copypriorss' not in args and '-copypriorss:a' not in args
              and option(args, '-copypriorss:v') == '0'
              and float(option(args, '-hls_time')) == 0.1, 'v2_actual_copy_window_argv')
        output.append({'seek': option(args, '-ss'), 'startNumber': option(args, '-start_number') or '0',
            'muxOffset': option(args, '-output_ts_offset'), 'audioBSF': option(args, '-bsf:a'),
            'segmentOptions': option(args, '-hls_segment_options'), 'parent': row['parent']})
    return output


def idle(api, server, source):
    deadline, zeros = time.monotonic() + 10, 0
    selected = {}
    while time.monotonic() < deadline:
        status, data, _ = api.http('/settings/metrics')
        check(status == 200 and len(data) <= 65536, 'v2_metrics_bound')
        selected = {}
        for line in data.decode().splitlines():
            match = re.fullmatch(r'kinosail_workload_(active|waiting)\{class="(playback|background)"\} ([0-9]{1,8})', line)
            if match:
                selected[match[1] + ':' + match[2]] = int(match[3])
        check(len(selected) == 4, 'v2_workload_projection_shape')
        zeros = zeros + 1 if encoder_count(server, source) == 0 and all(v == 0 for v in selected.values()) else 0
        if zeros >= 2:
            return {'zeroSamples': zeros, 'workloads': selected}
        time.sleep(0.05)
    raise RuntimeError('v2_cached_delivery_not_idle')


def cached_media(owner, selected, directory, source_grid):
    directory.mkdir()
    status, master, _ = owner.api.http(selected)
    check(status == 200 and len(master) <= 65536, 'v2_cached_master')
    names = [name for name in master.decode().splitlines() if re.fullmatch(r'[1-9][0-9]{2,3}p/index\.m3u8', name)]
    check(len(names) == 1, 'v2_cached_rendition')
    prefix = selected.removesuffix('index.m3u8') + names[0].removesuffix('index.m3u8')
    status, manifest, _ = owner.api.http(prefix + 'index.m3u8')
    check(status == 200 and len(manifest) <= 65536, 'v2_cached_variant')
    facts, fragments = manifest_facts(manifest)
    assert_fixed_timeline(facts, fragments, source_grid)
    assets = []
    for name in ['init.mp4', *[name for name, _ in fragments]]:
        status, data, _ = owner.api.http(prefix + name)
        check(status == 200 and 0 < len(data) <= 2 << 20, 'v2_cached_asset')
        path = directory / name
        path.write_bytes(data)
        assets.append(path)
    joined = directory / 'joined.mp4'
    joined.write_bytes(b''.join(bounded_bytes(path, 2 << 20, 'v2_cached_join') for path in assets))
    return joined, manifest, assets, idle(owner.api, owner.process, owner.source)
