import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
import {runInNewContext} from 'node:vm';

// Isolated failure matrix: encoder/read/listen failures leave an owned temp
// directory or listener; cleanup replaces the original failure; normal close
// changes. No encoder, server, socket or filesystem mutation runs here.
const source = stripTypeScriptTypes(readFileSync(new URL('../../apps/player/e2e/player-hls-navigation-fixture.ts', import.meta.url), 'utf8'))
  .replace(/^import .*;\n/gm, '').replace(/^export /gm, '');
async function fixture(failureAt) {
  const failure = new Error('controlled failure'), calls = [];
  const listeners = new Map();
  const server = {listening: false,
    once(name, callback) {listeners.set(name, callback);},
    removeListener(name) {listeners.delete(name);},
    listen(_port, _host, ready) {
      if (failureAt === 'listen') throw failure;
      if (failureAt === 'listen-event') {queueMicrotask(() => {const fail = listeners.get('error'); listeners.delete('error'); fail?.(failure);}); return;}
      this.listening = true; calls.push('listen'); ready();
    },
    address: () => failureAt === 'address' ? null : {port: 30001},
    closeAllConnections: () => calls.push('connections-closed'),
    close(callback) {this.listening = false; calls.push('listener-closed'); callback();}};
  const context = {readStaticSource: async () => '', playerSource: '',
    mkdtemp: async () => '/controlled-owned-temp', tmpdir: () => '/controlled', join: (...parts) => parts.join('/'),
    execFileSync: (_command, args) => {
      if (args[0] === '-version') return 'synthetic codec version\n';
      calls.push('encoder'); if (failureAt === 'encoder') throw failure;
    },
    readdir: async () => {if (failureAt === 'readdir') throw failure; return ['segment00.ts'];},
    readFile: async () => {if (failureAt === 'readFile') throw failure; return Buffer.from('controlled');},
    rm: async path => {assert.equal(path, '/controlled-owned-temp'); calls.push('temp-removed');},
    createHash: () => ({update: () => ({digest: () => 'controlled-source-hash'})}),
    createServer: () => server};
  const api = await runInNewContext(`(async()=>{${source}\nreturn {hlsNavigationPeer};})()`, context);
  return {failure, calls, server, listeners, run: () => api.hlsNavigationPeer()};
}
for (const failureAt of ['encoder', 'readdir', 'readFile', 'listen']) test(`owned peer cleanup preserves ${failureAt} failure`, async () => {
  const owner = await fixture(failureAt);
  await assert.rejects(owner.run(), error => error === owner.failure);
  assert.equal(owner.calls.filter(value => value === 'temp-removed').length, 1);
  assert.equal(owner.calls.includes('listen'), false);
});
test('asynchronous listen error rejects and removes owned temp within the control bound', async () => {
  const owner = await fixture('listen-event');
  let timer;
  try {
    await assert.rejects(Promise.race([owner.run(), new Promise((_, reject) => {
      timer = setTimeout(() => reject(new Error('control exceeded bound')), 250);
    })]), error => error === owner.failure);
  } finally {clearTimeout(timer);}
  assert.equal(owner.calls.filter(value => value === 'temp-removed').length, 1);
  assert.equal(owner.listeners.size, 0);
});
test('invalid post-listen address closes its owned listener and temp', async () => {
  const owner = await fixture('address');
  await assert.rejects(owner.run(), /missing isolated HLS address/);
  assert.equal(owner.server.listening, false);
  assert.deepEqual(owner.calls, ['encoder', 'listen', 'connections-closed', 'listener-closed', 'temp-removed']);
});
test('normal peer lifetime keeps temp until explicit close', async () => {
  const owner = await fixture();
  const peer = await owner.run();
  assert.equal(owner.calls.includes('temp-removed'), false);
  assert.equal(owner.server.listening, true);
  await peer.close();
  assert.equal(owner.server.listening, false);
  assert.deepEqual(owner.calls, ['encoder', 'listen', 'connections-closed', 'listener-closed', 'temp-removed']);
});
