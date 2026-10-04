"""Retain exact failed deep coverage without weakening its gate or upload scope."""
from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]


class CoverageEvidenceTests(unittest.TestCase):
    def test_manual_evidence_keeps_original_deep_gate_and_only_profiles(self):
        ci = (ROOT / '.github/workflows/ci.yml').read_text()
        app = (ROOT / '.github/workflows/app.yml').read_text()
        race = app.split('  race:\n', 1)[1].split('\n  security:', 1)[0]
        self.assertEqual(race.count('run: ./scripts/ci/test-go.sh "$APP"'), 1)
        self.assertNotIn('continue-on-error', race)
        self.assertIn("always() && github.event_name == 'workflow_dispatch' && inputs.coverage_diagnostic && fromJSON(inputs.plan).deep", race)
        self.assertIn('go tool cover -func=.verification/coverage.out > .verification/coverage-functions.txt', race)
        self.assertIn('apps/${{ inputs.app }}/.verification/coverage.out', race)
        self.assertIn('apps/${{ inputs.app }}/.verification/coverage-functions.txt', race)
        self.assertNotIn('path: apps/${{ inputs.app }}/.verification\n', race)
        self.assertEqual(ci.count("coverage_diagnostic: ${{ github.event_name == 'workflow_dispatch' && inputs.coverage_diagnostic }}"), 2)
        script = (ROOT / 'scripts/ci/test-go.sh').read_text()
        self.assertIn('player|subtitles) directory="apps/$1"; minimum=89', script)
        self.assertIn('go test -race -count=1 -covermode=atomic -coverprofile="$profile" ./...', script)


if __name__ == '__main__':
    unittest.main()
