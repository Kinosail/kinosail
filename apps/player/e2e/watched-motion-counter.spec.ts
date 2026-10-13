import {expect, test} from "@playwright/test";
import {installDisplayedMotionTracker, type DisplayedMotionHost, type FrameSample} from "./watched-motion-counter";

// Isolated gap: real playback cannot deterministically produce dropped-only,
// stale, or reset counter sequences. These controls protect admission arithmetic.
const sample = (seconds: number, total: number, dropped = 0): FrameSample => ({seconds, total, dropped});
const controls = [
  {name: "dropped-only increments", samples: [sample(1.1, 11, 1), sample(1.2, 12, 2), sample(1.3, 13, 3)], admitted: false},
  {name: "time without frames", samples: [sample(1.1, 10), sample(1.2, 10), sample(1.3, 10)], admitted: false},
  {name: "frames without time", samples: [sample(1, 11), sample(1, 12), sample(1, 13)], admitted: false},
  {name: "reset without fresh increments", samples: [sample(1.1, 11), sample(1.2, 12), sample(1.3, 2), sample(1.4, 2)], admitted: false},
  {name: "displayed counter regresses", samples: [sample(1.1, 11), sample(1.2, 12), sample(1.3, 13, 5), sample(1.4, 14, 5)], admitted: false},
  {name: "dropped counter resets", samples: [sample(1.1, 12, 1), sample(1.2, 13, 1), sample(1.3, 14), sample(1.4, 15)], admitted: false},
  {name: "time regresses", samples: [sample(1.1, 11), sample(1.2, 12), sample(0.9, 13), sample(1.3, 14)], admitted: false},
  {name: "three fresh displayed observations", samples: [sample(1.1, 11), sample(1.2, 13, 1), sample(1.3, 14, 1)], admitted: true},
  {name: "three fresh observations after reset", samples: [sample(1.1, 11), sample(1.2, 2), sample(1.3, 3), sample(1.4, 4), sample(1.5, 5)], admitted: true},
];

for (const control of controls) {
  test(`held motion rejects false frame credit: ${control.name}`, {tag: "@smoke"}, async ({}, info) => {
    const host: DisplayedMotionHost = {};
    installDisplayedMotionTracker(host);
    const tracker = host.createDisplayedMotionTracker!(sample(1, 10));
    for (const observation of control.samples) tracker.observe(observation);
    expect(tracker.receipt.invalid).toBe(false);
    expect(tracker.receipt.samples.length === 3).toBe(control.admitted);
    await info.attach("displayed-motion-counter-control", {body: JSON.stringify({name: control.name,
      input: control.samples, admitted: control.admitted, receipt: tracker.receipt,
      boundary: "Isolated synthetic counter arithmetic; no browser playback or Server proof"}), contentType: "application/json"});
  });
}

test("held motion rejects invalid counters and keeps failure sticky", {tag: "@smoke"}, async ({}, info) => {
  const invalid = [sample(1.1, NaN), sample(1.1, Infinity), sample(1.1, -1), sample(1.1, 1.5),
    sample(1.1, 2 ** 32), sample(1.1, 11, -1), sample(1.1, 11, 12), sample(NaN, 11)];
  for (const observation of invalid) {
    const host: DisplayedMotionHost = {};
    installDisplayedMotionTracker(host);
    const tracker = host.createDisplayedMotionTracker!(sample(1, 10));
    tracker.observe(observation);
    for (const total of [12, 13, 14]) tracker.observe(sample(total, total));
    expect(tracker.receipt.invalid).toBe(true);
    expect(tracker.receipt.samples).toEqual([]);
  }
  await info.attach("displayed-motion-invalid-controls", {body: JSON.stringify({rejected: invalid.length,
    boundary: "Invalid native counter shapes remain rejected after later valid observations"}), contentType: "application/json"});
});
