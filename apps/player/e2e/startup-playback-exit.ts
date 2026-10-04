import {expect, type Page, type Response} from '@playwright/test';

// Observe native trace delivery; never inject or replay admission state.
export function observeStartupPlaybackExits(page: Page, id: (name: string) => string, record: (value: object) => Promise<void>) {
  const establishedSessions = new Set<string>();
  const sequences = new Map<string, number>();
  page.on('request', request => {
    const path = new URL(request.url()).pathname;
    if (request.method() !== 'POST' || !path.endsWith('/playback-events')) return;
    try { const body = request.postDataJSON(); if (typeof body.session === 'string' && Number.isSafeInteger(body.sequence)) sequences.set(`${path}:${body.session}`, Math.max(sequences.get(`${path}:${body.session}`) || 0, body.sequence)); } catch {}
  });
  page.on('response', response => {
    const request = response.request();
    if (request.method() !== 'POST' || response.status() !== 204 || !new URL(response.url()).pathname.endsWith('/playback-events')) return;
    try {
      const body = request.postDataJSON();
      if (['playing', 'first-moving-frame'].includes(body.event) && body.paused === false && typeof body.session === 'string') establishedSessions.add(`${new URL(response.url()).pathname}:${body.session}`);
    } catch {}
  });
  return async (name: string, navigate: () => Promise<Response | null>) => {
    const video = page.locator('video'), session = await video.getAttribute('data-playback-session');
    const tracePath = `/api/v1/items/${id(name)}/playback-events`;
    expect(session).toBeTruthy();
    await expect.poll(() => establishedSessions.has(`${tracePath}:${session}`), {timeout: 10_000}).toBe(true);
    const key = `${tracePath}:${session}`, watermark = sequences.get(key) || 0;
    const terminal = (event: string, after: number) => page.waitForResponse(response => {
      if (response.status() !== 204 || response.request().method() !== 'POST' || new URL(response.url()).pathname !== tracePath) return false;
      try { const body = response.request().postDataJSON(); return body.session === session && body.event === event && body.paused === true && body.sequence > after; } catch { return false; }
    }, {timeout: 10_000});
    const started = Date.now(), paused = terminal('pause', watermark);
    await video.evaluate(media => media.pause());
    await paused;
    expect(await video.evaluate(media => media.paused)).toBe(true);
    const pauseAckMS = Date.now() - started, ended = terminal('session-end', sequences.get(key) || watermark);
    await navigate();
    await ended;
    const current = page.locator('video'), currentMediaCount = await current.count();
    expect(currentMediaCount).toBe(new URL(page.url()).pathname.startsWith('/watch/') ? 1 : 0);
    if (currentMediaCount) { expect(await current.getAttribute('data-playback-session')).toBeTruthy(); expect(await current.getAttribute('data-playback-session')).not.toBe(session); }
    await record({name: 'playback-exit-http-acknowledgements', journey: name, event: 'pause/session-end', status: 204, deliveredEvents: 2, sameSession: true, newerThanObservedRequests: true, pauseAckMS, terminalAckMS: Date.now() - started, currentMediaCount, oldCurrentMediaPresent: false});
  };
}
