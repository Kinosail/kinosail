"""Retain exact failed deep coverage without weakening its gate or upload scope."""
from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]


class CoverageEvidenceTests(unittest.TestCase):
    def test_manual_evidence_keeps_original_deep_gate_and_only_profiles(self):
        deep = (ROOT / '.github/workflows/deep.yml').read_text()
        race = deep.split('  race:\n', 1)[1].split('\n  browser:', 1)[0]
        self.assertEqual(race.count('run: ./scripts/ci/test-go.sh "$APP"'), 1)
        self.assertIn('KINOSAIL_GO_TEST_MODE: deep', race)
        self.assertNotIn('continue-on-error', race)
        self.assertEqual(race.count("always() && github.event_name == 'workflow_dispatch' && inputs.coverage_diagnostic"), 2)
        self.assertIn('go tool cover -func=.verification/coverage.out > .verification/coverage-functions.txt', race)
        self.assertIn('${{ matrix.directory }}/.verification/coverage.out', race)
        self.assertIn('${{ matrix.directory }}/.verification/coverage-functions.txt', race)
        self.assertNotIn('path: ${{ matrix.directory }}/.verification\n', race)
        self.assertNotIn('coverage_diagnostic', (ROOT / '.github/workflows/ci.yml').read_text())
        script = (ROOT / 'scripts/ci/test-go.sh').read_text()
        self.assertIn('player|subtitles) directory="apps/$1"; minimum=89', script)
        self.assertIn('go test -race -count=1 -coverpkg=./... -covermode=atomic -coverprofile="$profile" ./...', script)


if __name__ == '__main__':
    unittest.main()
