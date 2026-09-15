import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const source = readFileSync(new URL('./static/artwork-palette.js', import.meta.url), 'utf8');
test('featured actions stay stable and failed artwork falls back without losing the title', () => {
  const listeners = new Map();
  const removed = [];
  const image = { hidden: false, parentElement: { classList: { remove: name => removed.push(name) } }, addEventListener: (name, handler) => listeners.set(name, handler) };
  const main = { dataset: {}, querySelectorAll: () => [image], addEventListener() { assert.fail('browsing must not replace the feature'); } };
  vm.runInNewContext(source, {
    document: { readyState: 'complete', querySelector: () => main, addEventListener() {} },
    fetch() { assert.fail('artwork fallback must not make a network request'); },
  });
  listeners.get('error')();
  assert.equal(image.hidden, true);
  assert.deepEqual(removed, ['has-media-backdrop']);
});
