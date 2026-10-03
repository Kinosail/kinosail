import { test } from "@playwright/test";
import { completeHappyPath } from "./happy-path-complete";
import { startHappyPath } from "./happy-path-setup";

// This journey persists Owner/MFA state, so replaying it cannot retry the same setup.
test.describe.configure({ retries: 0 });

test("Owner can set up, create a passkey, find, play, resume, curate, install, and sign back in", { tag: "@smoke" }, async ({ page }, testInfo) => {
  // Full onboarding, playback, accessibility, and offline checks share this budget.
  test.setTimeout(90_000);
  // Keep unload state in retained traces without recording origins, queries, or credentials.
  await page.addInitScript(() => {
    const record = (event: string) => {
      const video = document.querySelector<HTMLVideoElement & { webkitPresentationMode?: string }>("video");
      if (!video) return;
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
  });
  const state = await startHappyPath(page, testInfo);
  await completeHappyPath(page, testInfo, state);
});
