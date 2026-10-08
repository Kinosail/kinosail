// A moving video in the previous document is not evidence for the selected public mode.
export async function selectCompatibilityDocument(page, link, started) {
  const current = page.url(), href = await link.getAttribute('href');
  if (typeof current !== 'string' || current.length > 2048 || !['?compatible=1', '?direct=1'].includes(href)
      || typeof started !== 'number' || !Number.isFinite(started) || started <= 0
      || started > Date.now() || Date.now() - started >= 10000) throw Error('invalid compatibility navigation');
  const source = new URL(current);
  if (!['http:', 'https:'].includes(source.protocol) || !['localhost', '127.0.0.1'].includes(source.hostname)
      || source.username || source.password || !/^\/watch\/[A-Za-z0-9_-]{1,128}$/.test(source.pathname)) throw Error('unowned compatibility destination');
  const destination = new URL(href, source).href;
  const previous = await page.evaluate(() => performance.timeOrigin);
  if (typeof previous !== 'number' || !Number.isFinite(previous) || previous <= 0) throw Error('invalid previous document identity');
  const remaining = () => Math.max(1, 10000 - (Date.now() - started));
  await link.click();
  await page.waitForURL(destination, {timeout: remaining(), waitUntil: 'commit'});
  await page.waitForFunction(({destination, previous}) => location.href === destination
    && performance.timeOrigin !== previous && document.readyState === 'complete'
    && Array.isArray(window.mediaEvents), {destination, previous}, {timeout: remaining()});
}

// Document facts only; missing instrumentation never fabricates a playing event.
export async function attachCompatibilityDocumentState(page, info, phase, expected = null) {
  if (!['before-clock', 'failure'].includes(phase) ||
      expected !== null && (typeof expected !== 'number' || !Number.isFinite(expected) || expected <= 0 || expected > 1e14))
    throw Error('invalid compatibility observation');
  let timer, state = null;
  try {
    const value = await Promise.race([page.evaluate(() => ({timeOrigin: performance.timeOrigin,
      observerPresent: Array.isArray(window.mediaEvents), documentReady: document.readyState === 'complete'})).catch(() => null),
      new Promise(resolve => {timer = setTimeout(() => resolve(null), 500);})]);
    clearTimeout(timer);
    if (value && Object.keys(value).sort().join(',') === 'documentReady,observerPresent,timeOrigin' &&
        typeof value.timeOrigin === 'number' && Number.isFinite(value.timeOrigin) && value.timeOrigin > 0 && value.timeOrigin <= 1e14 &&
        typeof value.observerPresent === 'boolean' && typeof value.documentReady === 'boolean') state = value;
    const body = JSON.stringify(state ? {schemaVersion: 1, phase, timeOrigin: state.timeOrigin,
      observerPresent: state.observerPresent, documentReady: state.documentReady,
      sameDocument: expected === null ? null : state.timeOrigin === expected} : {schemaVersion: 1, phase, unavailable: true});
    if (Buffer.byteLength(body) <= 1024) await Promise.race([info.attach('compatibility-document-state', {body, contentType:'application/json'}),
      new Promise(resolve => {timer = setTimeout(resolve, 500);})]);
  } catch { /* Observation cannot replace the original media assertion. */ }
  finally {clearTimeout(timer);}
  return state;
}
