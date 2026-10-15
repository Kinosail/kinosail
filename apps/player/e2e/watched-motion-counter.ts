export type FrameSample = {seconds: number; total: number; dropped: number};
export type DisplayedMotionReceipt = {baseline: FrameSample; samples: FrameSample[]; resets: number; invalid: boolean};
export type DisplayedMotionHost = {createDisplayedMotionTracker?: (start: FrameSample) => {
  receipt: DisplayedMotionReceipt; observe(sample: FrameSample): void;
}};

export function installDisplayedMotionTracker(target: DisplayedMotionHost = window as DisplayedMotionHost) {
  target.createDisplayedMotionTracker = start => {
    const valid = (sample: FrameSample) => Number.isFinite(sample.seconds) && sample.seconds >= 0 &&
      [sample.total, sample.dropped].every(value => Number.isInteger(value) && value >= 0 && value <= 0xffffffff) &&
      sample.dropped <= sample.total;
    const displayed = (sample: FrameSample) => sample.total - sample.dropped;
    let previous = {...start};
    const receipt: DisplayedMotionReceipt = {baseline: {...start}, samples: [], resets: 0, invalid: !valid(start)};
    return {receipt, observe(sample) {
      if (receipt.invalid) return;
      if (!valid(sample)) {receipt.invalid = true; receipt.samples = []; return;}
      const reset = sample.total < previous.total || sample.dropped < previous.dropped ||
        displayed(sample) < displayed(previous) || sample.seconds < previous.seconds;
      previous = {...sample};
      if (reset) {
        receipt.baseline = {...sample}; receipt.samples = []; receipt.resets++;
        return;
      }
      const last = receipt.samples.at(-1) ?? receipt.baseline;
      if (receipt.samples.length < 3 && displayed(sample) > displayed(last) && sample.seconds > last.seconds) {
        receipt.samples.push({...sample});
      }
    }};
  };
}
