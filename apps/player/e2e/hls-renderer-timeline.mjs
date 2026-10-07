// One bounded compositor tail. Native source strings stay private in this closure.
export function installPresentationTimeline(proof, media, nativeTime, nativeDuration, copyFrame, pending, stopPulse) {
  proof.frameTimings ??= [];
  proof.presentationAttachments = (proof.presentationAttachments ?? 0) + 1;
  let source, activated = false, generationStable = true;
  let visible = document.visibilityState === 'visible';
  let lastActivity = performance.now(), lastQuality, lastTotal = 0, endStamp;
  const touch = () => {lastActivity = performance.now();};
  const invalidate = () => {generationStable = false; touch();};
  document.addEventListener('visibilitychange', () => {
    visible = visible && document.visibilityState === 'visible'; touch();
  });
  window.addEventListener('pagehide', invalidate);
  document.addEventListener('keydown', event => {
    if (event.key === ' ' && event.isTrusted && !activated) {
      source = media.currentSrc; activated = true; touch();
    }
  }, {capture: true});
  for (const name of ['loadstart', 'emptied', 'seeking', 'seeked', 'playing', 'waiting', 'ended', 'error']) {
    media.addEventListener(name, () => {
      if (activated && ['loadstart', 'emptied', 'error'].includes(name)) invalidate();
      touch();
    });
  }
  const quality = () => {
    const value = media.getVideoPlaybackQuality?.();
    return value ? {total: value.totalVideoFrames, dropped: value.droppedVideoFrames,
      corrupted: value.corruptedVideoFrames ?? null} : null;
  };
  const inspect = () => {
    if (!media.isConnected || document.querySelector('video') !== media || proof.presentationAttachments !== 1) invalidate();
    if (activated && (!source || source !== media.currentSrc)) invalidate();
    visible = visible && document.visibilityState === 'visible';
    const current = quality(), serialized = JSON.stringify(current);
    if (serialized !== lastQuality) {lastQuality = serialized; touch();}
    if (!current || current.total < lastTotal) invalidate();
    lastTotal = current?.total ?? lastTotal;
    return current;
  };
  const terminal = () => {
    const q = inspect(), row = proof.rows.at(-1);
    return {callbacks: proof.rows.length, presentedFrames: row?.[1], pts: row?.[0],
      sha256: row?.[2], quality: q, pendingCopies: pending()};
  };
  const onCallback = (stamp, metadata) => {
    if (proof.frameTimings.length >= 4096) {invalidate(); return;}
    proof.frameTimings.push([metadata.presentedFrames, stamp, metadata.presentationTime, metadata.expectedDisplayTime]);
    inspect(); touch();
  };
  const onEnd = () => {endStamp = performance.now(); touch();};
  const settle = async () => {
    const tail = proof.presentationTail = {settled: false, timedOut: false};
    if (!proof.ended || proof.errorCode || endStamp === undefined) {
      tail.failureClass = 'presentation_end_missing'; stopPulse('presentation_end_missing'); return;
    }
    const started = performance.now();
    let timer, raf, stopped = false, frames = 0, before, finalRow, lastSample, stableSamples = 0;
    const nextFrame = () => new Promise(resolve => {raf = requestAnimationFrame(resolve);});
    const work = async () => {
      while (!stopped) {
        const sample = terminal(), signature = JSON.stringify(sample);
        stableSamples = signature === lastSample ? stableSamples + 1 : 0;
        lastSample = signature;
        const now = performance.now(), quiet = now - lastActivity;
        if (!visible || !generationStable || proof.captureErrors || proof.errorCode) break;
        if (now - endStamp >= 500 && frames >= 8 && quiet >= 250 && pending() === 0 && stableSamples >= 2) {
          if (!before) {
            before = sample;
            let frame;
            try {
              frame = new VideoFrame(media);
              finalRow = [frame.timestamp / 1000000, null, '', frame.timestamp, null, media.playbackRate];
              const copying = copyFrame(frame, finalRow);
              frame = null;
              proof.hashes.push(copying);
              await copying;
            } catch {frame?.close(); invalidate();}
            touch(); stableSamples = 0;
          } else {
            tail.beforeFinalCopy = before;
            tail.afterFinalCopy = sample;
            tail.finalNativeFrame = finalRow;
            tail.quietMilliseconds = quiet;
            tail.settled = JSON.stringify(before) === signature && pending() === 0;
            break;
          }
        }
        if (stopped) break;
        await nextFrame(); frames++;
        if (frames >= 1024) {invalidate(); break;}
      }
    };
    try {
      await Promise.race([work(), new Promise(resolve => {
        timer = setTimeout(() => {tail.timedOut = true; stopped = true; resolve();}, 2000);
      })]);
    } finally {
      stopped = true; clearTimeout(timer); cancelAnimationFrame(raf);
      tail.elapsedMilliseconds = performance.now() - started;
      tail.animationFrames = frames;
      tail.visibilityStable = visible;
      tail.generationStable = generationStable && activated;
      tail.attachments = proof.presentationAttachments;
      tail.nativeTime = nativeTime.call(media);
      tail.nativeDuration = nativeDuration.call(media);
      tail.ended = media.ended; tail.paused = media.paused;
      if (tail.timedOut) tail.settled = false;
      stopPulse(tail.settled ? 'presentation_settled' : 'presentation_unqualified');
    }
  };
  return {touch, onCallback, onEnd, settle};
}
