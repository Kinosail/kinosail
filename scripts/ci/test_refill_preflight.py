"""Protect complete race acceptance when a focused diagnostic is added."""
from pathlib import Path
import unittest

WORKFLOWS = Path(__file__).resolve().parents[2] / ".github/workflows"


class RefillPreflightTests(unittest.TestCase):
    def test_refill_preflight_cannot_replace_full_race_acceptance(self):
        # Failure model: a narrow or zero-test preflight becomes full acceptance,
        # its diagnostics overwrite complete receipts, or a failure becomes green.
        source = (WORKFLOWS / 'layout-stability.yml').read_text().split('  hls-followon:', 1)[1]
        focus = source.split('      - name: Probe restarted transcode refill\n', 1)[1].split('      - name:', 1)[0]
        self.assertIn('id: refill_preflight', focus)
        self.assertIn("-run '^TestRealTranscodedHLSRefillPreservesFrameClock$/^320x180-restarted-late-audio$'", focus)
        self.assertIn('KINOSAIL_TRANSCODE_REFILL_PREFLIGHT: "1"', focus)
        self.assertIn('/transcode-refill-preflight', focus)
        self.assertIn('assert len(completed) == 1', source)
        self.assertIn('player-transcode-refill-preflight-${{ github.sha }}', source)
        full = source.split('      - name: Run strict public transcode refill race suite\n', 1)[1].split('      - name:', 1)[0]
        self.assertIn("steps.refill_preflight.outcome == 'success'", full)
        self.assertIn('KINOSAIL_GO_TEST_MODE: deep', full)
        self.assertIn('run: ./scripts/ci/test-go.sh player', full)
        self.assertIn('player-transcode-refill-${{ github.sha }}', source)
        self.assertNotIn('continue-on-error', source)

