const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { JSDOM } = require('jsdom');

const script = readFileSync(`${__dirname}/../../apps/subtitles/docs/assets/js/docs.js`, 'utf8');
const entry = { title: 'Backups', description: 'Protect subtitle state', url: '/subtitles/owner-guide/backups/', content: 'recovery archive', product: 'Subtitles' };
const response = data => ({ ok: true, text: async () => JSON.stringify(data), json: async () => data });
const settle = () => new Promise(resolve => setImmediate(resolve));

function setup(fetch) {
  const dom = new JSDOM(`<button data-theme-toggle></button><button data-menu-toggle></button><aside data-sidebar><form data-search-form data-index-url="/subtitles/search.json"><input data-search-input><div data-search-results hidden></div></form></aside><main><a href="/subtitles/">Content</a><article class="prose"><pre><code>example command</code></pre></article></main>`, { url: 'https://kinosail.com/subtitles/', runScripts: 'outside-only' });
  const { window } = dom;
  window.matchMedia = () => ({ matches: true, addEventListener() {} });
  window.fetch = fetch;
  Object.defineProperty(window.navigator, 'clipboard', { value: { writeText: async () => {} } });
  window.eval(script);
  const input = window.document.querySelector('input');
  return { window, input, results: window.document.querySelector('[data-search-results]'), search: async value => { input.value = value; input.dispatchEvent(new window.Event('input')); await settle(); } };
}

test('Subtitles search rejects unsafe or malformed index entries before linking', async () => {
  for (const data of [[{ ...entry, url: 'javascript:alert(1)' }], [{ ...entry, title: 7 }], Array(1001).fill(entry)]) {
    const ui = setup(async () => response(data));
    await ui.search('backup');
    assert.equal(ui.results.querySelectorAll('a').length, 0);
    assert.match(ui.results.textContent, /Search could not load/);
    ui.window.close();
  }
});

test('clearing Subtitles search while loading cannot reopen stale results', async () => {
  let finish;
  const ui = setup(() => new Promise(resolve => { finish = resolve; }));
  await ui.search('backup');
  await ui.search('');
  finish(response([entry]));
  await settle();
  assert.equal(ui.results.hidden, true);
  ui.window.close();
});

test('Subtitles mobile menu focuses search and protects background focus', () => {
  const ui = setup(async () => response([entry]));
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

test('Subtitles code examples expose a labelled copy action', () => {
  const ui = setup(async () => response([entry]));
  assert.equal(ui.window.document.querySelector('.prose pre button')?.getAttribute('aria-label'), 'Copy code');
  ui.window.close();
});
