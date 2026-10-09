"""Written-first captured FD controls for calls completed before a live sample."""
import copy
import os
from pathlib import Path
import stat
import tempfile
import unittest
from hls_aac_v2_public_http import source_invocations


class CapturedFDInvocationControls(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix='kinosail-fd-contract-')
        self.source = Path(self.directory.name) / 'Fixture.mp4'
        self.source.write_bytes(b'fixed-public-identity-contract')
        self.retained = self.source.open('rb')
        info = os.fstat(self.retained.fileno())
        self.witness = {'device': info.st_dev, 'inode': info.st_ino, 'size': info.st_size,
            'mtimeNs': info.st_mtime_ns, 'regular': stat.S_ISREG(info.st_mode)}
        self.parent = os.getpid()
        self.alias = '/proc/' + str(self.parent) + '/fd/' + str(self.retained.fileno())
        self.row = {'parent': self.parent, 'args': ['-i', self.alias, '-hls_time', '0.1'],
            'inputWitness': dict(self.witness)}

    def tearDown(self):
        self.retained.close()
        self.directory.cleanup()

    def classify(self, row):
        return source_invocations([row], self.source, {self.parent})

    def reject(self, row):
        with self.assertRaises(RuntimeError):
            self.classify(row)

    def test_owned_retained_fd_identity_is_counted(self):
        self.assertEqual(self.classify(self.row), [self.row])

    def test_captured_owned_identity_is_counted_after_join(self):
        self.retained.close()
        self.assertEqual(self.classify(self.row), [self.row])

    def test_alias_pid_matches_owned_parent(self):
        row = copy.deepcopy(self.row)
        row['args'][1] = '/proc/' + str(self.parent + 1) + '/fd/' + str(self.retained.fileno())
        self.reject(row)

    def test_complete_witness_is_required(self):
        for name in self.witness:
            with self.subTest(field=name):
                row = copy.deepcopy(self.row)
                del row['inputWitness'][name]
                self.reject(row)

    def test_every_identity_field_must_match(self):
        for name in ['device', 'inode', 'size', 'mtimeNs']:
            with self.subTest(field=name):
                row = copy.deepcopy(self.row)
                row['inputWitness'][name] += 1
                self.reject(row)

    def test_integer_witness_fields_exclude_booleans_and_strings(self):
        for name in ['device', 'inode', 'size', 'mtimeNs']:
            for value in [True, str(self.witness[name])]:
                with self.subTest(field=name, valueType=type(value).__name__):
                    row = copy.deepcopy(self.row)
                    row['inputWitness'][name] = value
                    self.reject(row)

    def test_regular_input_is_required(self):
        row = copy.deepcopy(self.row)
        row['inputWitness']['regular'] = False
        self.reject(row)

    def test_alias_must_be_canonical_owned_and_bounded(self):
        for value in ['/proc/self/fd/3', '/dev/fd/3', '/proc/0001/fd/3',
            '/proc/' + str(self.parent) + '/fd/0003',
            '/proc/' + str(self.parent) + '/fd/65536']:
            with self.subTest(alias=value):
                row = copy.deepcopy(self.row)
                row['args'][1] = value
                self.reject(row)

    def test_extra_input_is_rejected(self):
        row = copy.deepcopy(self.row)
        row['args'] += ['-i', str(self.source)]
        self.reject(row)

    def test_fd_source_calls_retain_cardinality_bound(self):
        with self.assertRaises(RuntimeError):
            source_invocations([self.row] * 17, self.source, {self.parent})

    def test_present_wrong_literal_witness_is_rejected(self):
        row = copy.deepcopy(self.row)
        row['args'][1] = str(self.source)
        row['inputWitness']['inode'] += 1
        self.reject(row)
