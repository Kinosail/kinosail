const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { JSDOM } = require('jsdom');

const root = `${__dirname}/../..`;
const script = readFileSync(`${root}/apps/player/docs/assets/js/platform-install.js`, 'utf8');
const templates = Object.fromEntries(['player', 'subtitles', 'both'].map(app => [app,
  readFileSync(`${root}/apps/${app === 'both' ? 'player' : app}/packaging/platform-compose${app === 'both' ? '-both' : ''}.yaml`, 'utf8')]));
const settle = () => new Promise(resolve => setImmediate(resolve));

function setup(fetch = async url => ({ ok: true, text: async () => templates[url.split('/').at(-1).replace('.yaml', '')] })) {
  const dom = new JSDOM(`<section hidden data-install-builder data-player-template="/assets/install/player.yaml" data-subtitles-template="/assets/install/subtitles.yaml" data-both-template="/assets/install/both.yaml">
    <select data-install-app><option value="player">Player</option><option value="subtitles">Subtitles</option><option value="both">Both</option></select>
    <input data-install-media><label><span data-install-port-label>HTTPS port on the server</span><input data-install-port value="38127"></label><label data-install-subtitles-port-field hidden>Subtitles HTTPS port on the server<input data-install-subtitles-port value="38128"></label><button data-install-create type="button">Make file</button>
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

test('makes one file for both apps with isolated volumes, ports, and media access', async () => {
  const ui = setup();
  ui.element('app').value = 'both';
  ui.element('app').dispatchEvent(new ui.window.Event('change'));
  assert.equal(ui.element('port').value, '38127');
  assert.equal(ui.element('subtitles-port-field').hidden, false);
  ui.element('media').value = '/srv/Movies & TV';
  ui.element('port').value = '49127';
  ui.element('subtitles-port').value = '49128';
  await ui.create();
  const yaml = ui.element('preview').textContent;
  assert.match(yaml, /name: kinosail-both/);
  assert.match(yaml, /ghcr.io\/kinosail\/kinosail-player:latest/);
  assert.match(yaml, /ghcr.io\/kinosail\/kinosail-subtitles:latest/);
  assert.match(yaml, /"49127:38127"/);
  assert.match(yaml, /"49128:38128"/);
  assert.equal((yaml.match(/source: "\/srv\/Movies & TV"/g) || []).length, 2);
  assert.match(yaml, /player-config:/);
  assert.match(yaml, /subtitles-config:/);
  assert.equal(ui.element('download').download, 'kinosail-both-compose.yaml');
  assert.deepEqual(ui.requests.map(request => request.url), ['/assets/install/both.yaml']);
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

  for (const [playerPort, subtitlesPort] of [['38127', '38127'], ['38127', '80'], ['65536', '38128'], ['38127', ''], ['38127', '38128x']]) {
    const both = setup();
    both.element('app').value = 'both';
    both.element('app').dispatchEvent(new both.window.Event('change'));
    both.element('media').value = '/media';
    both.element('port').value = playerPort;
    both.element('subtitles-port').value = subtitlesPort;
    await both.create();
    assert.equal(both.element('result').hidden, true);
    assert.equal(both.requests.length, 0);
    assert.equal(both.blobs.length, 0);
    both.window.close();
  }
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

test('rejects an altered combined template before creating a download', async () => {
  for (const changed of [
    templates.both.replace('read_only: true\n        bind:', 'read_only: false\n        bind:'),
    templates.both.replace('ghcr.io/kinosail/kinosail-subtitles:latest', 'other:latest'),
    templates.both.replace('create_host_path: false', 'create_host_path: true'),
  ]) {
    const ui = setup(async () => ({ ok: true, text: async () => changed }));
    ui.element('app').value = 'both';
    ui.element('app').dispatchEvent(new ui.window.Event('change'));
    ui.element('media').value = '/media';
    await ui.create();
    assert.equal(ui.element('result').hidden, true);
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
