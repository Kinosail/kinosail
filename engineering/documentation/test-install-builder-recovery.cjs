// Isolated JSDOM fault controls; real generated-docs/browser proof is separate.
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { JSDOM } = require('jsdom');
const root = __dirname + '/../..';
const script = readFileSync(root + '/apps/player/docs/assets/js/platform-install.js', 'utf8');
const templates = Object.fromEntries(['player', 'subtitles', 'both'].map(app => [app,
  readFileSync(root + '/apps/' + (app === 'both' ? 'player' : app) + '/packaging/platform-compose' + (app === 'both' ? '-both' : '') + '.yaml', 'utf8')]));
const originals = [
  'https://raw.githubusercontent.com/Kinosail/kinosail/main/apps/player/packaging/platform-compose.yaml',
  'https://raw.githubusercontent.com/Kinosail/kinosail/main/apps/subtitles/packaging/platform-compose.yaml',
  'https://raw.githubusercontent.com/Kinosail/kinosail/main/apps/player/packaging/platform-compose-both.yaml',
];
const settle = () => new Promise(resolve => setImmediate(resolve));
function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
const response = app => ({ ok: true, text: async () => templates[app] });
function setup(fetch) {
  const dom = new JSDOM(`<section hidden data-install-builder data-player-template="/assets/install/player.yaml" data-subtitles-template="/assets/install/subtitles.yaml" data-both-template="/assets/install/both.yaml">
    <select data-install-app><option value="player">Player</option><option value="subtitles">Subtitles</option><option value="both">Both</option></select>
    <input data-install-media><span data-install-port-label>HTTPS port on the server</span><input data-install-port value="38127">
    <label data-install-subtitles-port-field hidden><input data-install-subtitles-port value="38128"></label>
    <button data-install-create type="button">Make Compose file</button><p data-install-error role="alert" hidden></p>
    <section data-install-result hidden><code data-install-preview></code><a data-install-download>Download</a><button data-install-copy type="button">Copy</button><p data-install-status></p></section>
    </section><ol><li>${originals.map((href, i) => '<a href="' + href + '">' + ['Player', 'Subtitles', 'Both'][i] + ' Compose file</a>').join(', ')}</li></ol>`,
  { url: 'https://kinosail.com/getting-started/platforms/', runScripts: 'outside-only' });
  const { window } = dom;
  const requests = [], blobs = [], revoked = [], copied = [], timers = new Map();
  let now = 0, next = 0;
  window.setTimeout = (callback, delay) => { const id = ++next; timers.set(id, { callback, at: now + delay }); return id; };
  window.clearTimeout = id => timers.delete(id);
  window.performance.now = () => now;
  window.fetch = (url, options) => {
    requests.push({ url, options });
    return fetch ? fetch(url, options) : Promise.resolve(response(url.split('/').at(-1).replace('.yaml', '')));
  };
  window.URL.createObjectURL = blob => { blobs.push(blob); return 'blob:fictional-' + blobs.length; };
  window.URL.revokeObjectURL = url => revoked.push(url);
  Object.defineProperty(window.navigator, 'clipboard', { value: { writeText: async text => copied.push(text) } });
  window.eval(script);
  const element = name => window.document.querySelector('[data-install-' + name + ']');
  const clock = {
    pending: () => [...timers.values()].map(timer => timer.at - now),
    elapse: ms => { now += ms; },
    tick: async ms => {
      now += ms;
      for (const [id, timer] of [...timers]) if (timer.at <= now && timers.delete(id)) timer.callback();
      await settle();
    },
  };
  const input = (name, value) => { element(name).value = value; element(name).dispatchEvent(new window.Event('input')); };
  const choose = app => { element('app').value = app; element('app').dispatchEvent(new window.Event('change')); };
  const create = async () => { element('create').click(); await settle(); };
  input('media', '/fictional/q47/media');
  return { window, element, clock, input, choose, create, requests, blobs, revoked, copied };
}
function failed(ui) {
  assert.equal(ui.element('create').disabled, false);
  assert.equal(ui.element('create').textContent, 'Retry');
  assert.equal(ui.element('error').hidden, false);
  assert.equal(ui.element('result').hidden, true);
  assert.equal(ui.element('download').getAttribute('href'), null);
  assert.equal(ui.element('fallback').hidden, false);
  assert.deepEqual([...ui.element('fallback').querySelectorAll('a')].map(link => link.href), originals);
  assert.equal(ui.element('media').value, '/fictional/q47/media');
  assert.equal(ui.clock.pending().length, 0);
}
function retained(ui, app, port, second = '38128') {
  assert.equal(ui.element('app').value, app);
  assert.equal(ui.element('port').value, port);
  assert.equal(ui.element('subtitles-port').value, second);
}
function published(ui, app, media, port) {
  assert.equal(ui.element('result').hidden, false);
  assert.equal(ui.element('create').disabled, false);
  assert.equal(ui.element('create').textContent, 'Make Compose file');
  assert.equal(ui.element('fallback').hidden, true);
  assert.equal(ui.element('download').download, 'kinosail-' + app + '-compose.yaml');
  assert.equal(ui.element('download').href, 'blob:fictional-' + ui.blobs.length);
  assert.ok(ui.element('preview').textContent.includes('source: ' + JSON.stringify(media)));
  assert.ok(ui.element('preview').textContent.includes('"' + port + ':' + (app === 'subtitles' ? '38128' : '38127') + '"'));
}

test('held headers settle at one 15-second deadline even when fetch ignores abort, then Retry works', async () => {
  const first = deferred();
  const ui = setup(() => ui.requests.length === 1 ? first.promise : Promise.resolve(response('player')));
  ui.input('port', '49127');
  await ui.create();
  assert.deepEqual(ui.clock.pending(), [15000]);
  await ui.clock.tick(14999);
  assert.equal(ui.element('create').disabled, true);
  await ui.clock.tick(1);
  failed(ui); retained(ui, 'player', '49127');
  assert.equal(ui.requests[0].options.credentials, 'omit');
  assert.equal(ui.requests[0].options.signal.aborted, true);
  first.resolve(response('player')); await settle();
  failed(ui); assert.equal(ui.blobs.length, 0);
  await ui.create();
  published(ui, 'player', '/fictional/q47/media', '49127');
  assert.equal(ui.requests.length, 2);
  assert.equal(ui.blobs.length, 1);
  assert.equal(ui.clock.pending().length, 0);
  assert.equal(ui.window.document.querySelectorAll('a[href^="https://raw.githubusercontent.com/"]').length, 6);
  ui.window.close();
});

test('held body shares the original deadline and retains both ports through Retry', async () => {
  const body = deferred();
  const ui = setup(() => ui.requests.length === 1 ? Promise.resolve({ ok: true, text: () => body.promise }) : Promise.resolve(response('both')));
  ui.choose('both'); ui.input('port', '49127'); ui.input('subtitles-port', '49128');
  await ui.create();
  await ui.clock.tick(5000);
  assert.deepEqual(ui.clock.pending(), [10000]);
  await ui.clock.tick(9999);
  assert.equal(ui.element('create').disabled, true);
  await ui.clock.tick(1);
  failed(ui); retained(ui, 'both', '49127', '49128');
  assert.equal(ui.requests[0].options.signal.aborted, true);
  body.resolve(templates.both); await settle();
  failed(ui); assert.equal(ui.blobs.length, 0);
  await ui.create();
  published(ui, 'both', '/fictional/q47/media', '49127');
  assert.ok(ui.element('preview').textContent.includes('"49128:38128"'));
  assert.equal(ui.requests.length, 2);
  ui.window.close();
});

test('late body cannot publish when the deadline callback has not yet been scheduled', async () => {
  const body = deferred();
  const ui = setup(() => Promise.resolve({ ok: true, text: () => body.promise }));
  await ui.create();
  ui.clock.elapse(15001);
  body.resolve(templates.player); await settle();
  failed(ui);
  assert.equal(ui.blobs.length, 0);
  assert.equal(ui.requests[0].options.signal.aborted, true);
  ui.window.close();
});

test('network and HTTP errors expose safe Retry and original links with retained inputs', async () => {
  for (const initial of [() => Promise.reject(new Error('fictional-private-canary')), () => Promise.resolve({ ok: false })]) {
    const ui = setup(() => ui.requests.length === 1 ? initial() : Promise.resolve(response('player')));
    ui.input('port', '49127'); await ui.create();
    failed(ui); retained(ui, 'player', '49127');
    assert.equal(ui.requests[0].options.signal.aborted, true);
    assert.doesNotMatch(ui.element('error').textContent, /fictional-private-canary/);
    await ui.create(); published(ui, 'player', '/fictional/q47/media', '49127');
    ui.window.close();
  }
});

test('editing cancels old headers while a new request owns all output and status', async () => {
  const old = deferred();
  const ui = setup(() => ui.requests.length === 1 ? old.promise : Promise.resolve(response('player')));
  await ui.create();
  ui.input('media', '/fictional/q47/changed'); ui.input('port', '50127');
  assert.equal(ui.requests[0].options.signal.aborted, true);
  assert.equal(ui.clock.pending().length, 0);
  await ui.create();
  const status = ui.element('status').textContent;
  old.resolve(response('player')); await settle();
  published(ui, 'player', '/fictional/q47/changed', '50127');
  assert.equal(ui.blobs.length, 1); assert.equal(ui.element('status').textContent, status);
  ui.window.close();
});

test('changing app cancels old body and preserves the new writable Subtitles contract', async () => {
  const body = deferred();
  const ui = setup(() => ui.requests.length === 1 ? Promise.resolve({ ok: true, text: () => body.promise }) : Promise.resolve(response('subtitles')));
  await ui.create();
  ui.choose('subtitles'); ui.input('port', '49128');
  assert.equal(ui.requests[0].options.signal.aborted, true);
  await ui.create();
  body.resolve(templates.player); await settle();
  published(ui, 'subtitles', '/fictional/q47/media', '49128');
  assert.ok(ui.element('preview').textContent.includes('read_only: false'));
  assert.equal(ui.blobs.length, 1);
  ui.window.close();
});

test('pagehide aborts headers and body without late Retry or Blob side effects', async () => {
  for (const phase of ['headers', 'body']) {
    const pending = deferred();
    const ui = setup(() => phase === 'headers' ? pending.promise : Promise.resolve({ ok: true, text: () => pending.promise }));
    await ui.create();
    ui.window.dispatchEvent(new ui.window.Event('pagehide'));
    assert.equal(ui.requests[0].options.signal.aborted, true);
    assert.equal(ui.clock.pending().length, 0);
    pending.resolve(phase === 'headers' ? response('player') : templates.player);
    await ui.clock.tick(20000);
    assert.equal(ui.element('create').textContent, 'Make Compose file');
    assert.equal(ui.element('error').hidden, true);
    assert.equal(ui.element('result').hidden, true);
    assert.equal(ui.blobs.length, 0);
    ui.window.close();
  }
  const ready = setup(); await ready.create();
  ready.window.dispatchEvent(new ready.window.Event('pagehide'));
  assert.deepEqual(ready.revoked, ['blob:fictional-1']);
  assert.equal(ready.element('download').getAttribute('href'), null);
  ready.window.close();
});

test('stale clipboard success or failure cannot overwrite a newer file status', async () => {
  for (const reject of [false, true]) {
    const copied = deferred(), ui = setup();
    await ui.create();
    ui.window.navigator.clipboard.writeText = () => copied.promise;
    ui.element('copy').click(); await settle();
    ui.input('media', '/fictional/q47/changed'); await ui.create();
    const status = ui.element('status').textContent;
    if (reject) copied.reject(new Error('fictional-private-canary')); else copied.resolve();
    await settle();
    assert.equal(ui.element('status').textContent, status);
    assert.ok(ui.element('preview').textContent.includes('/fictional/q47/changed'));
    ui.window.close();
  }
});

test('reentrant invalidation at Blob creation revokes the unowned URL without publishing', async () => {
  const ui = setup();
  ui.window.URL.createObjectURL = blob => {
    ui.blobs.push(blob);
    ui.input('media', '/fictional/q47/changed');
    return 'blob:fictional-1';
  };
  await ui.create();
  assert.equal(ui.element('result').hidden, true);
  assert.equal(ui.element('download').getAttribute('href'), null);
  assert.deepEqual(ui.revoked, ['blob:fictional-1']);
  assert.equal(ui.element('preview').textContent, '');
  ui.window.close();
});
