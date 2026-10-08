// Public form submission evidence for the disposable Library Owner journeys.
// No form values, response bodies, headers or raw routes enter the attachment.
export async function libraryMutation(page, baseURL, descriptor, button, testInfo) {
  const reject = () => {throw new Error('invalid owned Library mutation');};
  if (typeof baseURL !== 'string' || baseURL.length > 2048) reject();
  let base;
  try {base = new URL(baseURL);} catch {reject();}
  if (!['http:', 'https:'].includes(base.protocol) || !['127.0.0.1', 'localhost'].includes(base.hostname) ||
    !/^[1-9][0-9]{3,4}$/.test(base.port) || +base.port < 1024 || +base.port > 65535 ||
    base.username || base.password || base.pathname !== '/' || base.search || base.hash ||
    ![base.origin, base.origin + '/'].includes(baseURL)) reject();
  const kinds = {'profile-add': '/settings/profiles', 'profile-save': '/settings/profiles/permissions',
    'profile-password': '/settings/profiles/password', 'playlist-add': 'playlist', 'collection-add': 'collection'};
  if (!descriptor || Object.getPrototypeOf(descriptor) !== Object.prototype || !Object.hasOwn(kinds, descriptor.kind)) reject();
  const curation = ['playlist-add', 'collection-add'].includes(descriptor.kind);
  const keys = Object.keys(descriptor).sort().join(',');
  if (keys !== (curation ? 'kind,name' : 'kind')) reject();
  if (curation && (typeof descriptor.name !== 'string' || Buffer.byteLength(descriptor.name, 'utf8') > 128 ||
    !(descriptor.kind === 'playlist-add' ? /^E2E playlist (chromium|firefox|webkit)-[0-9]{13}$/ :
      /^E2E Collection (chromium|firefox|webkit)-[0-9]{13}$/).test(descriptor.name))) reject();
  const ownedURL = value => {
    if (typeof value !== 'string' || value.length > 2048) return;
    try {
      const url = new URL(value);
      if (url.origin === base.origin && !url.username && !url.password) return url;
    } catch {}
  };
  const current = ownedURL(page.url());
  if (!current || current.search || (curation ? !/^\/watch\/[a-f0-9]{16}$/.test(current.pathname) || current.hash :
    current.pathname !== '/settings' || !['', '#profiles'].includes(current.hash))) reject();
  const destination = curation ? current.pathname : '/settings';
  const fragment = curation ? '' : '#profiles';
  const action = curation ? `/${kinds[descriptor.kind]}/${encodeURIComponent(descriptor.name)}/${current.pathname.split('/')[2]}` : kinds[descriptor.kind];
  const form = await button.evaluate((element, settings) => ({action: element.form?.action, method: element.form?.method,
    section: settings ? element.form?.closest('section[id]')?.id : null}), !curation);
  if (!form || Object.keys(form).sort().join(',') !== 'action,method,section' || form.method !== 'post' ||
    form.action !== base.origin + action || form.section !== (curation ? null : 'profiles')) reject();
  const previous = await page.evaluate(() => performance.timeOrigin);
  if (!Number.isFinite(previous) || previous <= 0) reject();
  const state = {version: 1, kind: descriptor.kind, postRequests: 0, postStatus: null,
    redirectedRequests: 0, redirectedStatus: null, failures: 0,
    documentCommitted: false, documentReady: false, route: 'unavailable', outcome: 'failed'};
  let post, redirected, resolve, fail, failure, timer, postAccepted = false, active = true;
  const complete = new Promise((yes, no) => {resolve = yes; fail = no;});
  complete.catch(() => {});
  const failed = error => {if (active) {failure ??= error; fail(failure);}};
  const qualify = () => {
    if (postAccepted && state.redirectedStatus === 200 && state.documentCommitted) resolve();
  };
  const documentRequest = request => {
    try {return request.isNavigationRequest() && request.resourceType() === 'document' && request.frame() === page.mainFrame();}
    catch {return false;}
  };
  const request = candidate => {
    try {
      if (!documentRequest(candidate)) return;
      if (candidate.method() === 'POST' && candidate.url() === base.origin + action) {
        state.postRequests = Math.min(8, state.postRequests + 1);
        if (state.postRequests !== 1) throw new Error('duplicate Library mutation POST');
        post = candidate;
      } else if (candidate.method() === 'GET' && candidate.url() === base.origin + destination &&
        post && candidate.redirectedFrom() === post) {
        state.redirectedRequests = Math.min(8, state.redirectedRequests + 1);
        if (state.redirectedRequests !== 1) throw new Error('duplicate Library mutation redirect');
        redirected = candidate;
      }
    } catch (error) {failed(error);}
  };
  const response = candidate => {
    void (async () => {
      const source = candidate.request();
      if (!source || (source !== post && source !== redirected)) return;
      const status = candidate.status();
      if (!Number.isInteger(status) || status < 100 || status > 599) throw new Error('invalid Library mutation status');
      if (source === post) {
        state.postStatus = status;
        const location = await candidate.headerValue('location');
        if (!active) return;
        if (status !== 303 || location !== destination) throw new Error('Library mutation POST was not accepted');
        postAccepted = true;
      } else {
        state.redirectedStatus = status;
        if (status !== 200) throw new Error('Library mutation document was not accepted');
      }
      qualify();
    })().catch(failed);
  };
  const requestFailed = candidate => {
    if (candidate === post || candidate === redirected) {
      state.failures = Math.min(8, state.failures + 1); failed(new Error('Library mutation request failed'));
    }
  };
  const committed = frame => {
    if (frame === page.mainFrame() && redirected) {
      const url = ownedURL(page.url());
      if (!url || url.pathname !== destination || url.search || !['', fragment].includes(url.hash)) {
        failed(new Error('unapproved Library mutation destination')); return;
      }
      state.documentCommitted = true; qualify();
    }
  };
  const listeners = {request, response, requestfailed: requestFailed, framenavigated: committed};
  for (const [event, callback] of Object.entries(listeners)) page.on(event, callback);
  const started = performance.now();
  const remaining = () => Math.max(1, Math.ceil(10000 - (performance.now() - started)));
  timer = setTimeout(() => failed(new Error('Library mutation deadline')), 10000);
  let original;
  try {
    await button.click({timeout: remaining()});
    await complete;
    await page.waitForFunction(({origin, path, fragment, previous}) => location.origin === origin && location.pathname === path &&
      !location.search && ['', fragment].includes(location.hash) && Number.isFinite(performance.timeOrigin) && performance.timeOrigin > 0 && performance.timeOrigin !== previous &&
      performance.getEntriesByType('navigation')[0]?.type !== 'back_forward' && document.readyState !== 'loading',
      {origin: base.origin, path: destination, fragment, previous}, {timeout: remaining()});
    if (failure) throw failure;
    state.documentReady = true; state.outcome = 'complete';
  } catch (error) {original = error;}
  finally {
    active = false;
    clearTimeout(timer);
    for (const [event, callback] of Object.entries(listeners)) page.off(event, callback);
    const final = ownedURL(page.url());
    state.route = final?.pathname === '/settings' ? 'settings' : /^\/watch\/[a-f0-9]{16}$/.test(final?.pathname ?? '') ? 'watch' : final ? 'other' : 'unavailable';
    let attachmentTimer;
    try {
      const body = Buffer.from(JSON.stringify(state));
      if (body.length > 16384) throw new Error('Library mutation witness size');
      await Promise.race([testInfo.attach('library-mutation.json', {body, contentType: 'application/json'}),
        new Promise((_, no) => {attachmentTimer = setTimeout(() => no(new Error('Library mutation attachment deadline')), 500);})]);
    } catch (error) {original ??= error;}
    finally {clearTimeout(attachmentTimer);}
  }
  if (original) throw original;
}
