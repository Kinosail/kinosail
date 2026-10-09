"""Written-first actual owned codec input/capture controls; no Go worker."""
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import time
from types import SimpleNamespace
import unittest
from hls_aac_v1_stage_public import settle
from hls_aac_v2_public_http import ActualServer
from hls_aac_v2_source_identity import regular_identity
from hls_followon_public import bounded_bytes, check, encoder_count
from hls_remaining_nonkey_deadline import DiagnosticDeadline
from hls_timeline_fixture import fixture
from hls_timeline_http import source_state

ROOT = Path(__file__).resolve().parents[3]


class ActualFDObserverControls(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        check(os.name == 'posix' and hasattr(os, 'WNOWAIT'), 'fd_live_linux_required')
        cls.guard = DiagnosticDeadline(120)
        cls.guard.__enter__()
        cls.run = ROOT / '.verification/hls-aac-fd-observer/live'
        cls.run.mkdir(parents=True)
        regular, _ = fixture(cls.run, 'regular', 48, ','.join(str(v) for v in range(0,32,2)), frames=768)
        cls.source = cls.run / 'Fixture.mp4'
        result = subprocess.run(['ffmpeg', '-nostdin', '-v', 'error', '-i', str(regular),
            '-map', '0:v:0', '-map', '0:a:0', '-c', 'copy', str(cls.source)],
            capture_output=True, timeout=30)
        check(result.returncode == 0, 'fd_live_fixed_fixture_mux')
        cls.before = source_state(cls.source)
        check(cls.before['sha256'] == '198a6808092cea2a5481afc24e79481e4966ae577cbb8c46c214737e2173a3ed',
            'fd_live_fixed_fixture_identity')
        cls.expected = regular_identity(cls.source)
        cls.owners, cls.retained, cls.cases = [], [cls.source.open('rb')], []
        cls.server = SimpleNamespace(pid=os.getpid(), _copiedSourceWitness=dict(cls.expected))

    def setUp(self):
        check(not self.owners, 'fd_live_previous_owner_unresolved')
        self.row = {'case': self._testMethodName, 'result': 'failed'}
        self.cases.append(self.row)
        self.directory = self.run / self._testMethodName
        self.directory.mkdir()

    def alias(self, retained):
        return '/proc/' + str(os.getpid()) + '/fd/' + str(retained.fileno())

    def other(self):
        path = self.directory / 'Other.mp4'
        shutil.copyfile(self.source, path)
        retained = path.open('rb')
        self.retained.append(retained)
        check(source_state(path)['sha256'] == self.before['sha256']
            and regular_identity(path) != self.expected, 'fd_live_other_identity_qualification')
        return retained

    def alive(self, process):
        check(os.waitid(os.P_PID, process.pid, os.WEXITED | os.WNOHANG | os.WNOWAIT) is None,
            'fd_live_encoder_must_be_alive')

    def spawn(self, input_value, directory=None, startup=False, adapter=None, env=None):
        directory = directory or self.directory
        args = ['-nostdin', '-v', 'error', '-readrate', '0.1' if startup else '0.25',
            '-stream_loop', '-1', '-i', input_value, '-map', '0:v:0', '-map', '0:a:0',
            '-c:v', 'copy', '-c:a', 'aac' if startup else 'copy']
        args += ['-frames:v', '24', '-shortest'] if startup else ['-t', '10']
        args += ['-f', 'hls', '-hls_time', '1' if startup else '0.1', '-hls_list_size', '0',
            '-hls_segment_type', 'fmp4', '-hls_fmp4_init_filename', 'init.mp4',
            '-hls_segment_filename', str(directory / 'segment-%05d.m4s'),
            str(directory / 'index.m3u8')]
        error = (self.directory / 'codec-private.log').open('wb')
        process = subprocess.Popen([str(adapter or shutil.which('ffmpeg')), *args],
            stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=error,
            start_new_session=True, env=env)
        owned = {'startedAt': time.monotonic(), 'directory': directory, 'error': error}
        self.owners.append((process, self.row, owned))
        deadline = time.monotonic() + min(5, self.guard.check(20))
        while not (directory / 'init.mp4').exists():
            self.alive(process)
            check(time.monotonic() < deadline, 'fd_live_encoder_readiness_bound')
            time.sleep(0.02)
        self.alive(process)
        actual = bounded_bytes(Path('/proc') / str(process.pid) / 'cmdline',
            65536, 'fd_live_actual_argv_bound').rstrip(bytes([0])).split(bytes([0]))
        actual = [v.decode() for v in actual]
        check(actual[0] == shutil.which('ffmpeg') and actual[1:] == args,
            'fd_live_actual_pinned_argv')
        status = bounded_bytes(Path('/proc') / str(process.pid) / 'status', 65536,
            'fd_live_status_bound').decode()
        parents = [line.split()[1] for line in status.splitlines() if line.startswith('PPid:')]
        check(parents == [str(os.getpid())], 'fd_live_actual_owned_parent')
        self.row.update(actualLeaderAlive=True, actualPinnedArguments=True,
            actualArgumentSHA256=hashlib.sha256(repr(actual).encode()).hexdigest(),
            actualInputWitness=regular_identity(input_value))
        return process

    def observe(self, process):
        self.alive(process)
        try:
            count = encoder_count(self.server, self.source)
        except RuntimeError as failure:
            message = str(failure)
            self.row['liveClassificationFailureClass'] = message if re.fullmatch(r'[a-z0-9_]{1,96}', message) else 'fd_live_other_classification_failure'
            self.alive(process)
            raise
        self.alive(process)
        self.row['observedOwnedHLSCount'] = count
        return count

    def test_live_parent_retained_fd_is_counted(self):
        process = self.spawn(self.alias(self.retained[0]))
        self.assertEqual(self.observe(process), 1)
        self.row['result'] = 'observed'

    def test_live_wrong_byte_identical_fd_is_rejected_instead_of_zero(self):
        process = self.spawn(self.alias(self.other()))
        with self.assertRaisesRegex(RuntimeError, '^v2_live_input_identity_unqualified$'):
            self.observe(process)
        self.row['result'] = 'observed'

    def test_known_startup_codec_fixture_counts_as_owned_hls_activity(self):
        directory = Path(tempfile.mkdtemp(prefix='kinosail-transcoder-check-observer-'))
        source = directory / 'source.mp4'
        shutil.copyfile(self.source, source)
        process = self.spawn(str(source), directory, startup=True)
        self.assertEqual(self.observe(process), 1)
        self.row['result'] = 'observed'

    def test_wrapper_records_actual_other_fd_target_identity(self):
        retained = self.other()
        owner = ActualServer(ROOT, self.directory, self.source, Path('/unused-binary'))
        owner.owned_pids.add(os.getpid())
        self.spawn(self.alias(retained), adapter=self.directory / 'owned-ffmpeg', env=owner.env)
        rows = owner.invocation_rows()
        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0].get('inputWitness'), regular_identity(self.alias(retained)))
        self.assertNotEqual(rows[0]['inputWitness'], self.expected)
        self.assertNotIn('inputWitnessFailureClass', rows[0])
        self.row.update(wrapperActualTargetWitnessed=True, result='observed')

    def tearDown(self):
        with self.guard.cleanup():
            for process, row, owned in list(self.owners):
                try:
                    check(time.monotonic() - owned['startedAt'] <= 30, 'fd_live_command_elapsed_bound')
                    paths = list(owned['directory'].iterdir())
                    check(len(paths) <= 8 and all(p.is_file() and not p.is_symlink() for p in paths)
                        and sum(p.stat().st_size for p in paths) <= 8 << 20, 'fd_live_output_bound')
                    check(os.fstat(owned['error'].fileno()).st_size <= 2 << 20, 'fd_live_log_bound')
                except Exception:
                    row.update(result='failed', resourceFailureClass='fd_live_resource_guard_failed')
                    raise
                finally:
                    try:
                        settle(process, row)
                    except Exception:
                        row.update(result='failed', cleanupFailureClass='fd_live_owned_group_unqualified')
                        raise
                    self.owners.remove((process, row, owned))
                    owned['error'].close()
            self.row['sourceUnchanged'] = source_state(self.source) == self.before
            check(self.row['sourceUnchanged'], 'fd_live_source_changed')

    @classmethod
    def tearDownClass(cls):
        with cls.guard.cleanup():
            for process, row, owned in list(cls.owners):
                try:
                    settle(process, row)
                    cls.owners.remove((process, row, owned))
                    owned['error'].close()
                except Exception:
                    row['cleanupFailureClass'] = 'fd_live_final_owner_unresolved'
            if not cls.owners:
                for retained in cls.retained:
                    retained.close()
            evidence = {'revision': subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip(),
                'tree': subprocess.check_output(['git','rev-parse','HEAD^{tree}'],text=True).strip(),
                'sourceSHA256': cls.before['sha256'], 'initialSourceWitness': cls.expected,
                'sourceUnchanged': source_state(cls.source) == cls.before,
                'unresolvedOwners': len(cls.owners),
                'retainedSourcesClosedAfterJoin': all(v.closed for v in cls.retained),
                'cases': cls.cases, 'workerAcceptance': False, 'publicGETAcceptance': False,
                'executedScriptSHA256': hashlib.sha256(Path(__file__).read_bytes()).hexdigest()}
            (cls.run.parent / 'live-receipt.json').write_text(json.dumps(evidence, sort_keys=True) + chr(10))
            print(json.dumps({'liveContractProjection': evidence}, sort_keys=True), flush=True)
        cls.guard.__exit__()
        check(not cls.owners and evidence['sourceUnchanged']
            and evidence['retainedSourcesClosedAfterJoin'], 'fd_live_final_resource_guard')
