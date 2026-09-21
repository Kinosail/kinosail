const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { JSDOM } = require('jsdom');
const script = readFileSync(`${__dirname}/../../apps/player/docs/assets/js/docs.js`, 'utf8');
const entry = { title: 'Backups', description: 'Keep state safe', url: '/kinosail/owner-guide/backups/', content: 'encrypted recovery archive', product: 'Player' };
const settle = () => new Promise(resolve => setImmediate(resolve));
function setup(fetch, mobile = false) {
  const dom = new JSDOM(`<button data-theme-toggle></button><button data-menu-toggle></button><aside data-sidebar><form data-search-form data-index-url="/kinosail/search.json"><input data-search-input><div data-search-results hidden></div></form></aside><main><a href="/">Content</a></main>`, { url: 'https://kinosail.github.io/kinosail/', runScripts: 'outside-only' });
  const { window } = dom;
  window.matchMedia = () => ({ matches: mobile, addEventListener() {} });
  window.fetch = fetch;
  window.eval(script);
  const input = window.document.querySelector('input');
  return { window, input, results: window.document.querySelector('[data-search-results]'), search: async value => { input.value = value; input.dispatchEvent(new window.Event('input')); await settle(); } };
}
const response = data => ({ ok: true, text: async () => JSON.stringify(data) });

test('full text search caches the index, uses literal text, and supports no results', async () => {
  let requests = 0;
  const ui = setup(async () => { requests++; return response([entry, { ...entry, title: '<img src=x onerror=alert(1)>', url: '/kinosail/test/' }]); });
  await ui.search('encrypted');
  assert.equal(ui.results.querySelectorAll('a').length, 2);
  assert.equal(ui.results.querySelectorAll('img').length, 0);
  await ui.search('no-matching-page');
  assert.match(ui.results.textContent, /No matching pages/);
  assert.equal(requests, 1);
  await ui.search('a');
  assert.equal(ui.results.hidden, true);
  ui.window.close();
});

test('invalid indexes never create navigable results', async () => {
  const cases = [null, {}, [null], [{ ...entry, title: 4 }], [{ ...entry, description: undefined }], [{ ...entry, content: 'x'.repeat(100000) }], Array(1001).fill(entry), ...['javascript:alert(1)', '//evil.test/', '/elsewhere/', '/kinosail/../../escape', '/kinosail/\\evil.test'].map(url => [{ ...entry, url }])];
  for (const data of cases) {
    const ui = setup(async () => response(data));
    await ui.search('backup');
    assert.match(ui.results.textContent, /Search could not load/);
    assert.equal(ui.results.querySelectorAll('a').length, 0);
    ui.window.close();
  }
});

test('HTTP failure, malformed JSON, and oversized response fail safely; retry works', async () => {
  for (const bad of [{ ok: false }, { ok: true, text: async () => '{' }, { ok: true, text: async () => 'x'.repeat(3000001) }]) {
    let tries = 0;
    const ui = setup(async () => ++tries === 1 ? bad : response([entry]));
    await ui.search('backup');
    assert.match(ui.results.textContent, /Search could not load/);
    await ui.search('backups');
    assert.equal(ui.results.querySelectorAll('a').length, 1);
    ui.window.close();
  }
});

test('clearing or escaping while loading prevents stale results from reopening', async () => {
  for (const clear of [true, false]) {
    let finish;
    const ui = setup(() => new Promise(resolve => { finish = resolve; }));
    await ui.search('backup');
    if (clear) await ui.search('');
    else ui.window.document.dispatchEvent(new ui.window.KeyboardEvent('keydown', { key: 'Escape' }));
    finish(response([entry]));
    await settle();
    assert.equal(ui.results.hidden, true);
    ui.window.close();
  }
});

test('mobile menu focuses search, prevents obscured main focus, and restores on Escape', () => {
  const ui = setup(async () => response([entry]), true);
  const button = ui.window.document.querySelector('[data-menu-toggle]');
  button.click();
  assert.equal(ui.window.document.activeElement, ui.input);
  assert.equal(ui.window.document.querySelector('main').inert, true);
  ui.window.document.dispatchEvent(new ui.window.KeyboardEvent('keydown', { key: 'Escape' }));
  assert.equal(button.getAttribute('aria-expanded'), 'false');
  assert.equal(ui.window.document.querySelector('main').inert, false);
  assert.equal(ui.window.document.activeElement, button);
  ui.window.close();
});
