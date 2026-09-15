import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import fs from 'node:fs';
const context = vm.createContext({ document: { querySelector: () => null } });
vm.runInContext(fs.readFileSync(new URL('./static/mobile-tabs.js', import.meta.url), 'utf8'), context);
const parse = raw => JSON.parse(JSON.stringify(context.parseMobileTabs(raw)));
test('tab choices preserve order and enforce the four-tab limit', () => {
  assert.deepEqual(parse('["shows","all","movies","list"]'), ['shows','all','movies','list']);
  assert.deepEqual(parse('["movies"]'), ['movies']);
  for (const raw of ['', 'null', '{}', '[]', '["movies",]', '["movies","movies"]', '["unknown"]', '[1]', '[" movies"]', '["movies","shows","all","list","books"]', ' '.repeat(257)]) assert.throws(() => parse(raw));
});

test('upgrades the old default while preserving personal choices and later two-tab layouts', () => {
  const restore = (raw, legacy) => JSON.parse(JSON.stringify(context.restoreMobileTabs(raw, legacy)));
  const defaults = ['all', 'shows', 'movies', 'search'];
  assert.deepEqual(restore(null, null), defaults);
  assert.deepEqual(restore(null, '["movies","shows"]'), defaults);
  assert.deepEqual(restore(null, '["shows","movies","all"]'), ['shows', 'movies', 'all']);
  assert.deepEqual(restore('["movies","shows"]', null), ['movies', 'shows']);
  assert.deepEqual(restore(JSON.stringify(defaults), null), defaults);
  assert.throws(() => restore('["unknown"]', '["movies","shows"]'));
  assert.throws(() => restore(null, '["unknown"]'));
});
