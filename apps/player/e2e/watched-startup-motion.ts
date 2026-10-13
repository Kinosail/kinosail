type MotionState = {seconds: number | null; frames: number | null; duration: number | null; paused: boolean; ended: boolean; seeking: boolean;
  readyState: number; networkState: number; errorCode: number; visibility: string; focused: boolean;
  inViewport: boolean; buffered: {count: number; scanTruncated: boolean; current: (number | null)[] | null; last: (number | null)[] | null}};
export type WatchedObservation = {event: string; paused: boolean; failure?: string;
  motion?: {start: MotionState | null; current: MotionState | null; frameTimes: number[]; rawFrameTimes: (number | null)[];
    callbacksSeen: number; playResult: string; rejection: string};
  before: {seconds: number; frames: number; startSeconds: number; frameTimes: number[]; callbacksReleased: number};
  seconds: number; frames: number};

export function observeHeldWatchedMotion({watched, cancelledAgain}: {watched: boolean; cancelledAgain: boolean}) {
    const relay = (window as Window & {qaWatchedRelay: Window}).qaWatchedRelay;
    addEventListener("message", async event => {
      if (event.source !== relay || event.origin !== location.origin || event.data !== "qa-watched-commit") return;
      const video = document.querySelector("video")!;
      if (cancelledAgain) {
        const form = document.querySelector<HTMLFormElement>('form[action^="/watched/"]')!;
        document.addEventListener("submit", event => event.preventDefault(), {capture: true, once: true});
        if (form.dispatchEvent(new SubmitEvent("submit", {bubbles: true, cancelable: true, submitter: form.querySelector("button")}))) throw new Error("Second submission was not cancelled");
      }
      // Native navigation can pause WebKit media before the response arrives.
      // Establish fresh decoded motion inside the held request before releasing
      // the page's queued real notifications and requesting the late native pause.
      const finite = (value: number) => Number.isFinite(value) ? value : null;
      const state = (): MotionState | null => { try {
        const bounds = video.getBoundingClientRect(), ranges = video.buffered;
        const range = (index: number) => [finite(ranges.start(index)), finite(ranges.end(index))];
        let current: (number | null)[] | null = null;
        for (let index = 0; index < Math.min(ranges.length, 32); index++) {
          if (ranges.start(index) <= video.currentTime && ranges.end(index) >= video.currentTime) {current = range(index); break;}
        }
        return {seconds: finite(video.currentTime), frames: finite(video.getVideoPlaybackQuality().totalVideoFrames), duration: finite(video.duration),
          paused: video.paused, ended: video.ended, seeking: video.seeking, readyState: video.readyState,
          networkState: video.networkState, errorCode: video.error?.code ?? 0, visibility: document.visibilityState,
          focused: document.hasFocus(), inViewport: bounds.bottom > 0 && bounds.right > 0 && bounds.top < innerHeight && bounds.left < innerWidth,
          buffered: {count: ranges.length, scanTruncated: ranges.length > 32, current, last: ranges.length ? range(ranges.length - 1) : null}};
      } catch {return null;} };
      const start = {seconds: video.currentTime, frames: video.getVideoPlaybackQuality().totalVideoFrames};
      // WebKit can reset its quality counter when resuming. Count freshly
      // presented frames instead of treating totalVideoFrames as monotonic.
      const frameTimes: number[] = [];
      const rawFrameTimes: (number | null)[] = [];
      let callbacksSeen = 0;
      let frame = video.requestVideoFrameCallback(function presented(_now, metadata) {
        callbacksSeen = Math.min(callbacksSeen + 1, 255);
        if (rawFrameTimes.length < 3) rawFrameTimes.push(finite(metadata.mediaTime));
        if (metadata.mediaTime > (frameTimes.at(-1) ?? start.seconds)) frameTimes.push(metadata.mediaTime);
        if (frameTimes.length < 3) frame = video.requestVideoFrameCallback(presented);
      });
      let rejected = false, rejection = "none", playResult = "pending";
      const startState = state();
      void video.play().then(() => {playResult = "fulfilled";}, (error: unknown) => {
        rejected = true;
        playResult = "rejected";
        rejection = error instanceof DOMException && ["AbortError", "NotAllowedError", "NotSupportedError"].includes(error.name) ? error.name : "other";
      });
      const deadline = performance.now() + 8000;
      while (video.paused || frameTimes.length < 3 || video.currentTime <= start.seconds + 0.2) {
        if (rejected || performance.now() >= deadline) {
          video.cancelVideoFrameCallback(frame);
          relay.postMessage({kind: "qa-watched-paused", failure: "native motion did not resume during held transport",
            motion: {start: startState, current: state(), frameTimes, rawFrameTimes, callbacksSeen, playResult, rejection}}, location.origin);
          return;
        }
        await new Promise(resolve => setTimeout(resolve, 20));
      }
      const before = {seconds: video.currentTime, frames: video.getVideoPlaybackQuality().totalVideoFrames,
        startSeconds: start.seconds, frameTimes};
      const callbacksReleased = (window as Window & {watchedStartup: {release(): number}}).watchedStartup.release();
      const report = (event: Event) => relay.postMessage({kind: "qa-watched-paused", event: event.type, paused: video.paused,
        before: {...before, callbacksReleased}, seconds: video.currentTime, frames: video.getVideoPlaybackQuality().totalVideoFrames}, location.origin);
      if (watched) {video.addEventListener("pause", report, {once: true}); return video.pause();}
      video.addEventListener("ended", report, {once: true});
      // Decode the admitted buffered ending; departure intentionally stopped loading.
      void video.play().catch(() => {});
    });
}

export function recordWatchedNativeEvents() {
    const add = EventTarget.prototype.addEventListener;
    const native: Array<Record<string, unknown>> = [];
    const mediaTime = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, "currentTime");
    for (const event of ["loadstart", "loadedmetadata", "canplay", "play", "playing", "pause", "waiting", "stalled", "error", "ended", "seeking", "seeked", "timeupdate"]) {
      add.call(document, event, event => {
        if (!(event.target instanceof HTMLVideoElement)) return;
        const video = event.target;
        native.push({event: event.type, at: performance.now(), seconds: video.currentTime,
          nativeSeconds: mediaTime?.get?.call(video) ?? null, paused: video.paused, seeking: video.seeking,
          readyState: video.readyState, networkState: video.networkState, errorCode: video.error?.code ?? null,
          decodedFrames: video.getVideoPlaybackQuality().totalVideoFrames, pendingNotifications: (window as Window & {watchedStartup?: {pending: unknown[]}}).watchedStartup?.pending.length ?? 0});
        if (native.length > 96) native.shift();
      }, true);
    }
    Object.assign(window, {watchedNativeStartup: native});
}
