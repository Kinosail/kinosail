"""Safe source-drift diagnostics; fictional data only, no Go or browser calls."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
HELPER = ROOT / 'apps/player/scripts/campaign_source_admission.py'


class SourceAdmissionTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        spec = importlib.util.spec_from_file_location('campaign_source_admission', HELPER)
        cls.module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(cls.module)

    def admission(self, changed, missing=False):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            if not missing:
                (root / 'go.work.sum').write_text('PRIVATE-FICTIONAL-CURRENT')
            calls = []
            def git(*args):
                calls.append(args)
                if args == ('diff', '--name-only', '-z', 'HEAD', '--'):
                    return changed
                if args == ('show', 'HEAD:go.work.sum'):
                    return b'PRIVATE-FICTIONAL-BASE'
                raise AssertionError('unexpected command')
            result = self.module.source_admission(root, git)
            self.assertNotIn('PRIVATE-FICTIONAL', json.dumps(result))
            return result, calls

    def test_clean_checkout_has_explicit_admission(self):
        result, calls = self.admission(b'')
        self.assertTrue(result['clean'])
        self.assertEqual(result['changedTrackedCount'], 0)
        self.assertEqual(len(calls), 1)

    def test_known_manifest_drift_exports_only_identity_and_hashes(self):
        result, _ = self.admission(b'go.work.sum\0')
        self.assertFalse(result['clean'])
        self.assertEqual(result['changedTrackedCount'], 1)
        row = result['knownManifests'][0]
        self.assertEqual(row['file'], 'go.work.sum')
        self.assertEqual(row['head']['bytes'], 22)
        self.assertEqual(row['working']['bytes'], 25)
        self.assertNotEqual(row['head']['sha256'], row['working']['sha256'])

    def test_unknown_private_path_is_counted_without_reading_or_export(self):
        result, calls = self.admission(b'private/PRIVATE-FICTIONAL-token.txt\0')
        self.assertFalse(result['clean'])
        self.assertEqual(result['otherChangedTrackedCount'], 1)
        self.assertEqual(result['knownManifests'], [])
        self.assertEqual(len(calls), 1)

    def test_deleted_manifest_rejects_clean_without_raw_errors(self):
        result, _ = self.admission(b'go.work.sum\0', missing=True)
        self.assertFalse(result['clean'])
        self.assertEqual(result['knownManifests'][0]['working'], {'missing': True})


if __name__ == '__main__':
    unittest.main()
