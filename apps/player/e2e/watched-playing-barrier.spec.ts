import {expect, test} from "@playwright/test";
import {installWatchedPlayingBarrier} from "./watched-playing-barrier";

// Isolated gap: populated playback cannot deterministically prove registration
// identity, removal and once semantics for the fixture's delayed notifications.
test("watched barrier preserves library callbacks and owned listener identity", {tag: "@smoke"}, async ({page}, info) => {
  await page.addInitScript(installWatchedPlayingBarrier);
  await page.route("**/watched-barrier-proof", route => route.fulfill({contentType: "text/html", body: `
    <video></video><script>
      window.barrierCounts = {outside: 0, removed: 0, duplicate: 0, once: 0};
      document.querySelector('video').addEventListener('playing', () => barrierCounts.outside++);
    </script><script src="/static/player.js"></script><script>
      document.querySelector('video').addEventListener('playing', () => barrierCounts.outside++);
    </script>`}));
  await page.route("**/static/player.js", route => route.fulfill({contentType: "text/javascript", body: `
    const video = document.querySelector('video');
    const removed = () => barrierCounts.removed++;
    video.addEventListener('playing', removed);
    video.removeEventListener('playing', removed);
    window.barrierDuplicate = () => barrierCounts.duplicate++;
    video.addEventListener('playing', barrierDuplicate);
    video.addEventListener('playing', barrierDuplicate);
    video.addEventListener('playing', () => barrierCounts.once++, {once: true});
  `}));
  await page.goto("/watched-barrier-proof");
  const receipt = await page.evaluate(() => {
    const state = window as Window & {
      barrierCounts: {outside: number; removed: number; duplicate: number; once: number};
      barrierDuplicate: EventListener;
      watchedStartup: {pending: unknown[]; release(): number};
    };
    const video = document.querySelector("video")!;
    video.addEventListener("playing", state.barrierDuplicate);
    video.dispatchEvent(new Event("playing"));
    const before = {...state.barrierCounts, queued: state.watchedStartup.pending.length};
    const released = state.watchedStartup.release();
    const after = {...state.barrierCounts};
    video.removeEventListener("playing", state.barrierDuplicate);
    video.dispatchEvent(new Event("playing"));
    return {before, released, after, final: {...state.barrierCounts, queued: state.watchedStartup.pending.length}};
  });
  expect(receipt.before).toEqual({outside: 2, removed: 0, duplicate: 0, once: 0, queued: 2});
  expect(receipt.released).toBe(2);
  expect(receipt.after).toEqual({outside: 2, removed: 0, duplicate: 1, once: 1});
  expect(receipt.final).toEqual({outside: 4, removed: 0, duplicate: 1, once: 1, queued: 0});
  await info.attach("watched-playing-listener-control", {body: JSON.stringify({receipt,
    boundary: "Isolated native DOM event control with served scripts; no decoded media or populated Server proof"}), contentType: "application/json"});
});
