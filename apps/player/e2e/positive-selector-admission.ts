import type {Page, Request} from '@playwright/test';

export const selectorNegotiationMatch = (request: Request, page: Page, watch: string, base: URL) => {
  const facts = {get: request.method() === 'GET', mainFrame: false, activeWatch: false,
    sameOriginReferer: false, watchReferer: false, codecsPresent: false};
  try {
    const referer = new URL(request.headers().referer || ''), endpoint = new URL(request.url());
    facts.mainFrame = request.frame() === page.mainFrame(); facts.activeWatch = new URL(request.frame().url()).pathname === watch;
    facts.sameOriginReferer = referer.origin === base.origin; facts.watchReferer = referer.pathname === watch;
    facts.codecsPresent = Boolean(endpoint.searchParams.get('videoCodecs'));
  } catch { /* No raw URL/referrer retained. */ }
  return facts;
};

// Fixed read-only failure facts; this cannot admit or alter negotiation.
export const selectorAdmissionFacts = (page: Page) => page.evaluate(() => {
  const video = document.querySelector('video'); if (!video) return null;
  const bounded = (value: number) => Number.isFinite(value) && value >= 0 && value <= 31622400 ? value : null;
  return {paused: video.paused, readyState: video.readyState, renderedStart: bounded(Number(video.dataset.start)),
    reportedPosition: bounded(video.currentTime), transcode: video.dataset.compatibilityMode === 'transcode',
    nativeHLS: video.canPlayType('application/vnd.apple.mpegurl') !== '',
    ordinaryCanPlayType: ['av01.0.08M.08', 'hvc1.1.6.L123.B0', 'vp09.00.10.08', 'avc1.64002a']
      .map(codec => video.canPlayType(`video/mp4; codecs="${codec}"`) !== '')};
}).catch(() => null);
