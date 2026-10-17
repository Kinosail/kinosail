"""Keep one long browser frame from failing the title-filter layout flow.

The WebKit layout job failed three times on one revision because a single
frame gap of about 1.1 s left only one sample after 550 ms. The page never
navigated or scrolled. The browser flow cannot inject a frame stall on demand,
so these cases replay recorded frame timings through the real sampler and
verdict with only the browser globals replaced.
"""
import json
from pathlib import Path
import subprocess
import unittest


MODULE = Path(__file__).resolve().parents[2] / "scripts/testing/layout-stability-navigation.mjs"
REPLAY = """
const [modulePath, frames, documentChanged] = JSON.parse(process.argv[1]);
const {sampleTitleFilterFrames, titleFilterOutcome, titleFilterSampling} = await import(modulePath);
let now = 0, y = 0, index = 0;
Object.defineProperty(globalThis, "performance", {value: {now: () => now, timeOrigin: 1}, configurable: true});
globalThis.requestAnimationFrame = callback => {
  const [at, scroll] = frames[index] ?? [(frames.at(-1)?.[0] ?? 0) + 16 * (index - frames.length + 1), 0];
  index++;
  now = at;
  y = scroll;
  queueMicrotask(() => callback(now));
};
Object.defineProperty(globalThis, "scrollY", {get: () => y, configurable: true});
globalThis.innerHeight = 650;
globalThis.innerWidth = 390;
globalThis.getComputedStyle = () => ({scrollBehavior: "smooth"});
globalThis.matchMedia = () => ({matches: false});
globalThis.document = {documentElement: {scrollHeight: 1439, scrollWidth: 390}, querySelectorAll: () => ({length: 4})};
const state = await sampleTitleFilterFrames(titleFilterSampling);
console.log(JSON.stringify(titleFilterOutcome(state, documentChanged ? 2 : 1, titleFilterSampling)));
"""


def replay(frames, document_changed=False):
    completed = subprocess.run(["node", "--input-type=module", "-e", REPLAY,
                                json.dumps([MODULE.as_uri(), frames, document_changed])],
                               capture_output=True, text=True, timeout=60, check=False)
    if completed.returncode != 0:
        raise AssertionError(completed.stderr)
    return json.loads(completed.stdout)


def steady(start, end, scroll=0):
    return [[at, scroll] for at in range(start, end, 16)]


ATTEMPT_ONE = [[1, 0], [103, 0], [136, 0], [863, 0]]
ATTEMPT_TWO = [[1, 0], [14, 0], [117, 0], [154, 0], [188, 0], [1325, 0]]
ATTEMPT_THREE = [[0, 0], [13, 0], [115, 0], [188, 0], [1345, 0]]


class TitleFilterSamplingTest(unittest.TestCase):
    def test_recorded_webkit_frame_stalls_pass_once_three_settled_frames_arrive(self):
        for frames, gap in [(ATTEMPT_ONE, 727), (ATTEMPT_TWO, 1137), (ATTEMPT_THREE, 1157)]:
            with self.subTest(gap=gap):
                outcome = replay(frames)
                self.assertTrue(outcome["stable"], outcome)
                self.assertGreaterEqual(outcome["settledSamples"], 3)
                self.assertEqual(outcome["longestFrameGap"], gap)
                self.assertEqual(outcome["sampleCount"], len(frames) + 2)

    def test_steady_frames_still_sample_the_whole_late_window(self):
        outcome = replay(steady(16, 900))
        self.assertTrue(outcome["stable"], outcome)
        self.assertGreaterEqual(outcome["settledSamples"], 16)
        self.assertEqual(outcome["longestFrameGap"], 16)

    def test_scroll_after_a_long_frame_still_fails(self):
        outcome = replay(ATTEMPT_TWO + [[1341, 40], [1357, 40]])
        self.assertFalse(outcome["stable"], outcome)

    def test_late_scroll_inside_the_original_window_still_fails(self):
        outcome = replay(steady(16, 690) + [[700, 12]] + steady(716, 900))
        self.assertFalse(outcome["stable"], outcome)

    def test_document_change_still_fails(self):
        outcome = replay(ATTEMPT_TWO, document_changed=True)
        self.assertFalse(outcome["stable"], outcome)

    def test_stall_past_the_cap_fails_without_enough_settled_frames(self):
        outcome = replay([[1, 0], [20, 0], [3100, 0]])
        self.assertFalse(outcome["stable"], outcome)
        self.assertEqual(outcome["settledSamples"], 1)
        self.assertEqual(outcome["sampleCount"], 3)


if __name__ == "__main__":
    unittest.main()
