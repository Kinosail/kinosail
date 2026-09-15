import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {test} from 'node:test';
import {runInNewContext} from 'node:vm';

const source = readFileSync(new URL('./static/appearance.js', import.meta.url), 'utf8');
function loadTheme(saved, dark = false, storageError = false) {
  const root = {dataset: {}};
  const writes = [];
  const control = {value: saved, addEventListener(name, listener) { this.change = listener; }};
  const meta = {setAttribute(name, value) { this[name] = value; }};
  const media = {matches: dark, addEventListener(name, listener) { this.change = listener; }};
  runInNewContext(source, {
    document: {documentElement: root, readyState: 'complete', querySelector: () => meta, querySelectorAll: () => [control]},
    localStorage: {getItem() { if (storageError) throw new Error('Unavailable'); return saved; }, setItem(key, value) { writes.push([key, value]); }},
    HTMLInputElement: class {},
    matchMedia: () => media,
    addEventListener() {},
  });
  return {root, meta, media, control, writes};
}

test('system appearance resolves to an explicit CSS theme and follows changes', () => {
  const {root, meta, media} = loadTheme('system');
  assert.equal(root.dataset.theme, 'light');
  assert.equal(meta.content, '#f5f7fb');
  media.matches = true;
  media.change();
  assert.equal(root.dataset.theme, 'dark');
  assert.equal(meta.content, '#10151e');
});

test('explicit appearance is stable when system appearance changes', () => {
  const {root, media} = loadTheme('light', true);
  media.change();
  assert.equal(root.dataset.theme, 'light');
});

for (const value of [undefined, '', 'unknown', 'LIGHT', 'x'.repeat(4096)]) {
  test(`invalid or missing persisted theme falls back safely (${String(value).slice(0,12)})`, () => {
    assert.equal(loadTheme(value).root.dataset.theme, 'dark');
  });
}
test('unavailable storage does not prevent rendering', () => {
  assert.equal(loadTheme(null, false, true).root.dataset.theme, 'dark');
});

test('invalid appearance changes do not alter the page or persisted preference', () => {
  const {root, control, writes} = loadTheme('light');
  for (const value of ['', 'unknown', 'LIGHT', 'x'.repeat(4096)]) {
    control.value = value;
    control.change();
    assert.equal(root.dataset.theme, 'light');
    assert.deepEqual(writes, []);
  }
});
test('a supported appearance applies and persists immediately', () => {
  const {root, control, writes} = loadTheme('light');
  control.value = 'dark';
  control.change();
  assert.equal(root.dataset.theme, 'dark');
  assert.deepEqual(writes, [['kinosail-theme', 'dark']]);
});
