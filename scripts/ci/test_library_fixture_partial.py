"""Creation/proof failure regressions using the actual command-peer owner fixture."""
from pathlib import Path
import shutil
import unittest
import test_library_fixture_owner as owner


class LibraryFixturePartialTests(unittest.TestCase):
    setUp = owner.LibraryFixtureOwnerTests.setUp
    run_caller = owner.LibraryFixtureOwnerTests.run_caller
    rows = owner.LibraryFixtureOwnerTests.rows

    def test_partial_creation_with_transient_proof_failure_rediscovered_for_cleanup(self):
        for kind in ('network', 'volume', 'container'):
            if self.events.exists(): self.events.unlink()
            if self.output.exists(): shutil.rmtree(self.output)
            with self.subTest(kind=kind):
                result = self.run_caller(CONTROL_PROOF_FAIL_KIND=kind)
                rows = self.rows()
                workspace = Path(next(args[0] for event, args in rows if event == 'workspace'))
                self.addCleanup(lambda path=workspace: shutil.rmtree(path) if path.exists() else None)
                self.assertEqual(result.returncode, 2, result.stderr)
                self.assertFalse(any(event == 'owner' for event, _ in rows))
                self.assertFalse(workspace.exists(), 'fresh matching proof should permit owned cleanup')
                proofs = [args for event, args in rows if event == 'resource-proof' and args[0] == kind]
                self.assertGreaterEqual(len(proofs), 2)
                engines = [args for event, args in rows if event == 'engine']
                removal = ['rm', '--force', proofs[-1][3]] if kind == 'container' else [kind, 'rm', proofs[-1][1] if kind == 'volume' else proofs[-1][3]]
                self.assertIn(removal, engines)
                self.events.unlink()
                if self.output.exists(): shutil.rmtree(self.output)

    def test_unverifiable_partial_creation_retains_workspace_and_resource(self):
        for kind in ('network', 'volume', 'container'):
            if self.events.exists(): self.events.unlink()
            if self.output.exists(): shutil.rmtree(self.output)
            with self.subTest(kind=kind):
                result = self.run_caller(CONTROL_PROOF_FAIL_KIND=kind, CONTROL_PROOF_ALWAYS='1')
                rows = self.rows()
                workspace = Path(next(args[0] for event, args in rows if event == 'workspace'))
                self.addCleanup(lambda path=workspace: shutil.rmtree(path) if path.exists() else None)
                self.assertEqual(result.returncode, 2, result.stderr)
                self.assertTrue(workspace.exists(), 'unknown creation outcome must retain owned evidence')
                self.assertIn('Owned library fixture cleanup failed', result.stderr)
                self.assertFalse(any(event == 'owner' for event, _ in rows))
                engines = [args for event, args in rows if event == 'engine']
                prefix = ['rm'] if kind == 'container' else [kind, 'rm']
                self.assertFalse(any(args[:len(prefix)] == prefix for args in engines))
                self.events.unlink()
                if self.output.exists(): shutil.rmtree(self.output)

    def test_foreign_collision_keeps_unverifiable_workspace_without_removal(self):
        for variable in ('CONTROL_CONTAINER_COLLISION', 'CONTROL_VOLUME_COLLISION'):
            if self.events.exists(): self.events.unlink()
            if self.output.exists(): shutil.rmtree(self.output)
            with self.subTest(variable=variable):
                result = self.run_caller(**{variable: '1'})
                rows = self.rows()
                workspace = Path(next(args[0] for event, args in rows if event == 'workspace'))
                self.addCleanup(lambda path=workspace: shutil.rmtree(path) if path.exists() else None)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(any(event in ('foreign-removed', 'owner') for event, _ in rows))
                self.assertTrue(workspace.exists())
                self.assertIn('Owned library fixture cleanup failed', result.stderr)


if __name__ == '__main__':
    unittest.main()
