import {expect, type Page, type Response} from '@playwright/test';

// Observe native trace delivery; never inject or replay admission state.
export function observeStartupPlaybackExits(page: Page, id: (name: string) => string, record: (value: object) => Promise<void>) {
  const establishedSessions = new Set<string>();
  const sequences = new Map<string, number>();
  const observations = new Map<string, object[]>();
  const events = new Set(['play', 'playing', 'first-moving-frame', 'pause', 'session-end', 'heartbeat', 'waiting', 'stalled', 'canplay', 'seeking', 'seeked']);
  const observe = (path: string, body: {session: string; sequence: number; event?: unknown; paused?: unknown}, status?: number) => {
    const key = `${path}:${body.session}`, history = observations.get(key) || [];
    history.push({event: typeof body.event === 'string' && events.has(body.event) ? body.event : 'other', sequence: body.sequence, paused: body.paused === true, ...(status === undefined ? {} : {status})});
    observations.set(key, history.slice(-32));
  };
  page.context().on('request', request => {
    const path = new URL(request.url()).pathname;
    if (request.method() !== 'POST' || !path.endsWith('/playback-events')) return;
    try { const body = request.postDataJSON(); if (typeof body.session === 'string' && Number.isSafeInteger(body.sequence)) { sequences.set(`${path}:${body.session}`, Math.max(sequences.get(`${path}:${body.session}`) || 0, body.sequence)); observe(path, body); } } catch {}
  });
  page.context().on('response', response => {
    const request = response.request();
    if (request.method() !== 'POST' || !new URL(response.url()).pathname.endsWith('/playback-events')) return;
    try {
      const body = request.postDataJSON();
      if (typeof body.session !== 'string' || !Number.isSafeInteger(body.sequence)) return;
      observe(new URL(response.url()).pathname, body, response.status());
      if (response.status() === 204 && ['playing', 'first-moving-frame'].includes(body.event) && body.paused === false) establishedSessions.add(`${new URL(response.url()).pathname}:${body.session}`);
    } catch {}
  });
  return async (name: string, navigate: () => Promise<Response | null>) => {
    const video = page.locator('video'), session = await video.getAttribute('data-playback-session');
    const tracePath = `/api/v1/items/${id(name)}/playback-events`;
    expect(session).toBeTruthy();
    await expect.poll(() => establishedSessions.has(`${tracePath}:${session}`), {timeout: 10_000}).toBe(true);
    const key = `${tracePath}:${session}`, watermark = sequences.get(key) || 0;
    const terminal = (event: string, after: number) => page.context().waitForEvent('response', {predicate: response => {
      if (response.status() !== 204 || response.request().method() !== 'POST' || new URL(response.url()).pathname !== tracePath) return false;
      try { const body = response.request().postDataJSON(); return body.session === session && body.event === event && body.paused === true && body.sequence > after; } catch { return false; }
    }, timeout: 10_000});
    await expect.poll(() => video.evaluate(media => media.paused)).toBe(false);
    await page.locator('.media-stage').focus();
    const started = Date.now(); let stage = 'pause-http-ack', pauseAckMS = 0;
    try {
      const paused = terminal('pause', watermark);
      await page.keyboard.press('k');
      await paused;
      expect(await video.evaluate(media => media.paused)).toBe(true);
      pauseAckMS = Date.now() - started;
      stage = 'public-navigation';
      await navigate();
    } catch (error) {
      const media = await page.evaluate(() => {const n=document.querySelector('video');return n ? {paused:n.paused,readyState:n.readyState,errorCode:n.error?.code||0} : {currentMediaUnavailable:true};}).catch(() => ({currentMediaUnavailable: true}));
      await record({name: 'playback-exit-http-failure', journey: name, stage, elapsedMS: Date.now() - started, media, observations: observations.get(key) || []});
      throw error;
    }
    const current = page.locator('video'), currentMediaCount = await current.count();
    expect(currentMediaCount).toBe(new URL(page.url()).pathname.startsWith('/watch/') ? 1 : 0);
    if (currentMediaCount) { expect(await current.getAttribute('data-playback-session')).toBeTruthy(); expect(await current.getAttribute('data-playback-session')).not.toBe(session); }
    const sessionEndHTTPObserved = (observations.get(key) || []).some(value => {const event = value as {event?: string; status?: number; sequence?: number};return event.event === 'session-end' && event.status === 204 && (event.sequence || 0) > watermark;});
    await record({name: 'playback-exit-http-acknowledgements', journey: name, input: 'keyboard-k', event: 'pause', status: 204, deliveredEvents: 1, sameSession: true, newerThanObservedRequests: true, pauseAckMS, navigationMS: Date.now() - started - pauseAckMS, sessionEndHTTPObserved, currentMediaCount, oldCurrentMediaPresent: false});
  };
}
