import { test } from "@playwright/test";
import { completeHappyPath } from "./happy-path-complete";
import { startHappyPath } from "./happy-path-setup";
import { navigationDiagnostics } from "../../../scripts/testing/navigation-diagnostics.mjs";

// This journey persists Owner/MFA state, so replaying it cannot retry the same setup.
test.describe.configure({ retries: 0 });

test("Owner can set up, create a passkey, find, play, resume, curate, install, and sign back in", { tag: "@smoke" }, async ({ page }, testInfo) => {
  // Full onboarding, playback, accessibility, and offline checks share this budget.
  test.setTimeout(90_000);
  const diagnostics = testInfo.project.use.defaultBrowserType === "webkit"
    ? navigationDiagnostics(page, testInfo.project.use.baseURL) : undefined;
  diagnostics?.observePlayback();
  let failure: unknown;
  try {
    // Keep unload state in retained traces without recording origins, queries, or credentials.
    await page.addInitScript((observePlayback: boolean) => {
      const guard = Symbol.for("kinosail:e2e:playback-observer");
      const state = window as unknown as Record<symbol, boolean>;
      if (state[guard]) return;
      state[guard] = true;
      let records = 0;
      const record = (event: string) => {
        const video = document.querySelector<HTMLVideoElement & { webkitPresentationMode?: string }>("video");
        if (!video || records === 64) return;
        records++;
        const raw = video.currentSrc || video.getAttribute("src") || "";
        let source = "";
        try {
          if (raw) {
            const url = new URL(raw, location.href);
            source = ["http:", "https:"].includes(url.protocol) ? url.pathname.slice(0, 256) : url.protocol;
          }
        } catch {}
        console.debug("kinosail-playback-lifecycle", JSON.stringify({
          event, source, visible: document.visibilityState, position: video.currentTime,
          ready: video.readyState, network: video.networkState, paused: video.paused,
          pip: document.pictureInPictureElement === video || video.webkitPresentationMode === "picture-in-picture",
        }));
      };
      addEventListener("pagehide", () => record("pagehide-capture"), { capture: true });
      // Register after initial scripts so the second observation follows their handlers.
      addEventListener("load", () => {
        addEventListener("pagehide", () => record("pagehide-after-load"));
      }, { once: true });
      if (observePlayback) {
        addEventListener("kinosail:navigation", () => record("navigation"));
        document.addEventListener("DOMContentLoaded", () => {
          for (const event of ["play", "pause", "playing", "seeking", "seeked", "loadedmetadata", "emptied", "error"]) {
            document.addEventListener(event, value => {if (value.target instanceof HTMLVideoElement) record(event);}, {capture: true});
          }
        }, {once: true});
      }
    }, Boolean(diagnostics));
    const state = await startHappyPath(page, testInfo);
    await completeHappyPath(page, testInfo, state);
  } catch (error) {
    failure = error;
    throw error;
  } finally {
    if (diagnostics) {
      let timer: ReturnType<typeof setTimeout> | undefined;
      try {
        const value = await diagnostics.snapshot(failure);
        await Promise.race([
          testInfo.attach("playback-navigation-diagnostics", {body: Buffer.from(JSON.stringify(value)), contentType: "application/json"}),
          new Promise(resolve => {timer = setTimeout(resolve, 500);}),
        ]);
      } catch { /* Retained observations never replace the original journey outcome. */ }
      finally {clearTimeout(timer); diagnostics.stop();}
    }
  }
});
