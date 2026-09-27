const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { JSDOM } = require('jsdom');

const root = `${__dirname}/../..`;
const script = readFileSync(`${root}/apps/player/docs/assets/js/platform-install.js`, 'utf8');
const templates = Object.fromEntries(['player', 'subtitles'].map(app => [app,
  readFileSync(`${root}/apps/${app}/packaging/platform-compose.yaml`, 'utf8')]));
const settle = () => new Promise(resolve => setImmediate(resolve));

function setup(fetch = async url => ({ ok: true, text: async () => templates[url.endsWith('/player.yaml') ? 'player' : 'subtitles'] })) {
  const dom = new JSDOM(`<section hidden data-install-builder data-player-template="/assets/install/player.yaml" data-subtitles-template="/assets/install/subtitles.yaml">
    <select data-install-app><option value="player">Player</option><option value="subtitles">Subtitles</option></select>
    <input data-install-media><input data-install-port value="38127"><button data-install-create type="button">Make file</button>
    <p data-install-error hidden></p><section data-install-result hidden><code data-install-preview></code>
    <a data-install-download>Download</a><button data-install-copy type="button">Copy</button><p data-install-status></p></section>
  </section>`, { url: 'https://kinosail.com/getting-started/platforms/', runScripts: 'outside-only' });
  const { window } = dom;
  const requests = [], blobs = [], revoked = [], copied = [];
  window.fetch = async (url, options) => { requests.push({ url, options }); return fetch(url, options); };
  window.URL.createObjectURL = blob => { blobs.push(blob); return `blob:kinosail-${blobs.length}`; };
  window.URL.revokeObjectURL = url => revoked.push(url);
  Object.defineProperty(window.navigator, 'clipboard', { value: { writeText: async text => copied.push(text) } });
  window.eval(script);
  const element = name => window.document.querySelector(`[data-install-${name}]`);
  const create = async () => { element('create').click(); await settle(); };
  return { window, element, create, requests, blobs, revoked, copied };
}

test('makes a self-contained Player file without sending the media path', async () => {
  const ui = setup();
  assert.equal(ui.element('builder').hidden, false);
  ui.element('media').value = '/mnt/tank/Movies & TV';
  await ui.create();
  const yaml = ui.element('preview').textContent;
  assert.match(yaml, /source: "\/mnt\/tank\/Movies & TV"/);
  assert.match(yaml, /ghcr.io\/kinosail\/kinosail-player:latest/);
  assert.match(yaml, /read_only: true/);
  assert.doesNotMatch(yaml, /KINOSAIL_MEDIA_PATH/);
  assert.equal(ui.element('download').download, 'kinosail-player-compose.yaml');
  assert.equal(ui.element('download').href, 'blob:kinosail-1');
  assert.equal(ui.element('result').hidden, false);
  assert.deepEqual(ui.requests.map(request => request.url), ['/assets/install/player.yaml']);
  assert.equal(ui.requests[0].options.credentials, 'omit');
  ui.element('copy').click();
  await settle();
  assert.deepEqual(ui.copied, [yaml]);
  ui.window.close();
});

test('switches app and host port while keeping Subtitles media writable', async () => {
  const ui = setup();
  ui.element('app').value = 'subtitles';
  ui.element('app').dispatchEvent(new ui.window.Event('change'));
  assert.equal(ui.element('port').value, '38128');
  ui.element('media').value = '/srv/media';
  ui.element('port').value = '49128';
  await ui.create();
  const yaml = ui.element('preview').textContent;
  assert.match(yaml, /"49128:38128"/);
  assert.match(yaml, /source: "\/srv\/media"/);
  assert.match(yaml, /read_only: false/);
  assert.equal(ui.element('download').download, 'kinosail-subtitles-compose.yaml');
  ui.window.close();
});

test('rejects invalid input before fetch or download', async () => {
  const paths = ['', 'relative/path', '/', '/mnt/../media', '/mnt/$MEDIA', '/mnt/\u007fnext', ' /media', '/media ', '/' + 'a'.repeat(4096), '/' + ' '.repeat(4096) + 'media'];
  for (const path of paths) {
    const ui = setup();
    ui.element('media').value = path;
    await ui.create();
    assert.equal(ui.element('result').hidden, true, path);
    assert.equal(ui.element('error').hidden, false, path);
    assert.equal(ui.requests.length, 0, path);
    assert.equal(ui.blobs.length, 0, path);
    ui.window.close();
  }
  for (const port of ['', '0', '80', '01024', '65536', 'abc', '38127x']) {
    const ui = setup();
    ui.element('media').value = '/media';
    ui.element('port').value = port;
    await ui.create();
    assert.equal(ui.element('result').hidden, true, port);
    assert.equal(ui.requests.length, 0, port);
    ui.window.close();
  }
  const ui = setup();
  ui.element('app').append(new ui.window.Option('Unknown', 'unknown'));
  ui.element('app').value = 'unknown';
  ui.element('media').value = '/media';
  await ui.create();
  assert.equal(ui.requests.length, 0);
  assert.equal(ui.element('result').hidden, true);
  ui.window.close();
});

test('rejects unavailable, oversized, or unexpected templates without a download', async () => {
  for (const fetch of [
    async () => { throw new Error('network'); },
    async () => ({ ok: false }),
    async () => ({ ok: true, text: async () => 'x'.repeat(20000) }),
    async () => ({ ok: true, text: async () => templates.player.replace('kinosail-player:latest', 'other:latest') }),
    async () => ({ ok: true, text: async () => templates.player.replace('KINOSAIL_MEDIA_PATH', 'MEDIA_PATH') }),
    async () => ({ ok: true, text: async () => templates.player.replace('create_host_path: false', 'create_host_path: true') }),
  ]) {
    const ui = setup(fetch);
    ui.element('media').value = '/media';
    await ui.create();
    assert.equal(ui.element('result').hidden, true);
    assert.equal(ui.element('error').hidden, false);
    assert.equal(ui.blobs.length, 0);
    ui.window.close();
  }
});

test('editing a generated file clears stale output and revokes its download', async () => {
  const ui = setup();
  ui.element('media').value = '/media';
  await ui.create();
  ui.element('media').value = '/another';
  ui.element('media').dispatchEvent(new ui.window.Event('input'));
  assert.equal(ui.element('result').hidden, true);
  assert.deepEqual(ui.revoked, ['blob:kinosail-1']);
  assert.equal(ui.element('download').getAttribute('href'), null);
  ui.window.close();
});

test('editing while the template loads cannot publish an obsolete file', async () => {
  let finish;
  const ui = setup(() => new Promise(resolve => { finish = resolve; }));
  ui.element('media').value = '/first';
  ui.element('create').click();
  await settle();
  assert.equal(ui.element('create').disabled, true);
  assert.equal(ui.element('result').hidden, true);
  ui.element('media').value = '/second';
  ui.element('media').dispatchEvent(new ui.window.Event('input'));
  finish({ ok: true, text: async () => templates.player });
  await settle();
  assert.equal(ui.element('result').hidden, true);
  assert.equal(ui.blobs.length, 0);
  ui.window.close();
});
