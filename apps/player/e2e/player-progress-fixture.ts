import {type Page} from '@playwright/test';

export const progressFixtureHTML = `
  <!doctype html><html lang="en"><head><meta charset="utf-8"><title>R03 fixture</title></head>
  <body data-viewer-profile="qa-viewer"><main class="player-shell">
    <video data-progress="/progress/movie?playbackToken=synthetic" data-start="0"></video>
    <div class="player-progress-notice" data-progress-notice hidden>
      <span role="status" aria-live="polite" data-progress-status></span>
      <button class="quiet" type="button" data-progress-retry>Retry saving position</button>
      <button class="quiet" type="button" data-progress-continue hidden>Continue without saving</button>
    </div>
    <label>Audio track <select data-audio-track><option value="0">Original</option><option value="1">Other</option></select></label><small data-audio-status></small>
  </main></body></html>`;

export async function establishPlayedThenPaused(page: Page) {
  await page.locator('video').evaluate(media => {
    const decoder = window as Window & {setPaused: (value: boolean) => void};
    decoder.setPaused(false);
    media.dispatchEvent(new Event('playing'));
    decoder.setPaused(true);
  });
}
