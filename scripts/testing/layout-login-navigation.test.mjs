import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
import {runInNewContext} from 'node:vm';
import {EventEmitter} from 'node:events';
import {navigationDiagnostics} from './navigation-diagnostics.mjs';

const source = stripTypeScriptTypes(readFileSync(new URL('../../apps/player/e2e/layout-audit-helpers.ts', import.meta.url), 'utf8')).replace(/^import .*;\n/gm, '').replace(/^export /gm, '');
const origin = 'http://localhost:39060';
function fixture({attachment = 'ok', success = false, raw = origin + '/login?private=secret', type = 'document', status = 200} = {}) {
  const cause = new Error('NS_ERROR_NET_RESET private-secret');
  const attachments = [], effects = [];
  const info = {project: {use: {baseURL: origin}}, attach: async (name, value) => {
    if (attachment === 'reject') throw new Error('private attachment failure');
    if (attachment === 'stall') return new Promise(() => {});
    attachments.push({name, value});
  }};
  const locator = {fill: async () => effects.push('fill'), count: async () => 0, click: async () => effects.push('click'), isVisible: async () => false};
  class Page extends EventEmitter {
    url() {return origin + '/login?private=secret';}
    mainFrame() {return this;}
    async evaluate() {throw new Error('private destroyed context');}
    async goto(path, options) {
      effects.push({path, options});
      const request = {url: () => raw, resourceType: () => type, isNavigationRequest: () => true, frame: () => this};
      this.emit('request', request);
      this.emit('response', {request: () => request, url: () => raw, status: () => status, fromServiceWorker: () => false});
      this.emit(success ? 'requestfinished' : 'requestfailed', request);
      if (!success) throw cause;
    }
    getByLabel() {return locator;}
    getByRole() {return locator;}
    async waitForURL() {effects.push('waitForURL');}
  }
  const page = new Page();
  const login = runInNewContext(`(()=>{${source};return login;})()`, {test: {info: () => info}, navigationDiagnostics, Buffer, setTimeout, clearTimeout});
  return {run: () => login(page), page, cause, attachments, effects};
}
function facts(peer) {
  assert.equal(peer.attachments.length, 1);
  const {name, value} = peer.attachments[0];
  assert.equal(name, 'login-navigation-failure'); assert.equal(value.contentType, 'application/json');
  assert.ok(Buffer.byteLength(value.body) <= 16384);
  assert.doesNotMatch(value.body, /https?:|private|secret|credential|header|body/);
  return JSON.parse(value.body);
}

test('actual shared login callback records a reset without changing goto or its error', async () => {
  const peer = fixture(); await assert.rejects(peer.run(), error => error === peer.cause);
  const value = facts(peer);
  assert.equal(value.errorCategory, 'reset'); assert.equal(value.path, '/login');
  assert.equal(value.counts.requests, 1); assert.equal(value.counts.responses, 1); assert.equal(value.counts.failed, 1);
  assert.equal(value.mainFrameResponses[0].status, 200);
  assert.deepEqual(peer.effects, [{path: '/login', options: undefined}]); assert.equal(peer.page.eventNames().length, 0);
});

test('foreign unknown malformed oversized paths/types/statuses never disclose untrusted values', async () => {
  for (const raw of [null, 7, '', 'not-url', 'https://foreign.invalid/private-secret', origin + '/unknown/private-secret', origin + '/login/extra', 'x'.repeat(2049), 'http://Owner:secret@localhost:39060/login']) {
    const peer = fixture({raw, type: 'private-secret', status: Infinity});
    await assert.rejects(peer.run(), error => error === peer.cause);
    const value = facts(peer); assert.equal(value.failed[0].path, 'other'); assert.equal(value.failed[0].type, 'other');
    assert.equal(value.mainFrameResponses.length, 0); assert.equal(peer.page.eventNames().length, 0);
  }
});

for (const attachment of ['reject', 'stall']) test('diagnostic ' + attachment + ' preserves original cause and cleans listeners', async () => {
  const peer = fixture({attachment}); await assert.rejects(peer.run(), error => error === peer.cause);
  assert.equal(peer.attachments.length, 0); assert.equal(peer.page.eventNames().length, 0);
  assert.deepEqual(peer.effects, [{path: '/login', options: undefined}]);
});

test('successful shared login keeps original actions and emits no failure attachment', async () => {
  const peer = fixture({success: true}); await peer.run();
  assert.equal(peer.attachments.length, 0); assert.equal(peer.page.eventNames().length, 0);
  assert.deepEqual(peer.effects, [{path: '/login', options: undefined}, 'fill', 'fill', 'click', 'waitForURL']);
});

test('owned static resources retain only the category, not an arbitrary basename', async () => {
  const peer = fixture({raw: origin + '/static/private-secret.js', type: 'script'});
  await assert.rejects(peer.run(), error => error === peer.cause);
  const value = facts(peer); assert.equal(value.failed[0].path, '/static');
  assert.equal(value.failed[0].type, 'script'); assert.equal(peer.page.eventNames().length, 0);
});

const librarySource = stripTypeScriptTypes(readFileSync(new URL('../../apps/player/e2e/layout-audit-library.spec.ts', import.meta.url), 'utf8'));
const geometryBlock = librarySource.slice(librarySource.indexOf('\t\tif (route === "/quick-connect")'), librarySource.indexOf('\t\tfor (const selector of selectors)'));
function quickConnectPeer({passes = false, capture = 'ok', attachment = 'ok', image} = {}) {
  const cause = new Error('actual Quick Connect bottom exceeds dock');
  const effects = [], writes = [], attachments = [];
  const png = image ?? Buffer.alloc(24);
  if (!image) {Buffer.from([137,80,78,71,13,10,26,10]).copy(png); png.writeUInt32BE(720,16); png.writeUInt32BE(450,20);}
  const page = {evaluate: async () => ({headerBottom: 60, dockTop: 385, digits: Array.from({length: 6}, () => ({top: 300, bottom: passes ? 380 : 398}))}),
    screenshot: async options => {effects.push(options); if (capture === 'reject') throw new Error('capture'); if (capture === 'stall') return new Promise(() => {}); return png;}};
  const info = {outputPath: name => '/owned/' + name, attach: async (name, value) => {
    if (attachment === 'reject') throw new Error('attachment'); if (attachment === 'stall') return new Promise(() => {});
    attachments.push({name, value});}};
  const expect = value => ({toHaveLength: size => assert.equal(value.length, size), toBeGreaterThanOrEqual: minimum => assert.ok(value >= minimum),
    toBeLessThanOrEqual: maximum => {if (value > maximum) throw cause;}});
  const context = {test: {}, expect, Buffer, setTimeout, clearTimeout, writeFile: async (path, body, options) => writes.push({path, body, options})};
  const helper = runInNewContext(`(()=>{${source};return typeof quickConnectFailureEvidence === 'function' ? quickConnectFailureEvidence : undefined;})()`, context);
  const run = () => runInNewContext(`(async()=>{${geometryBlock}})()`, {...context, route: '/quick-connect', page, testInfo: info, quickConnectFailureEvidence: helper});
  return {run, cause, effects, writes, attachments};
}

test('actual Quick Connect geometry failure captures one named private PNG and preserves the original error', async () => {
  const peer = quickConnectPeer(); await assert.rejects(peer.run(), error => error === peer.cause);
  assert.equal(peer.writes.length, 1); assert.equal(peer.writes[0].path, '/owned/720-quick-connect-failure.png');
  assert.deepEqual({...peer.writes[0].options}, {flag: 'wx', mode: 0o600});
  assert.equal(peer.attachments.length, 1); assert.equal(peer.attachments[0].name, '720-quick-connect-failure.png');
  assert.equal(peer.attachments[0].value.path, peer.writes[0].path); assert.equal(peer.attachments[0].value.contentType, 'image/png');
  assert.deepEqual({...peer.effects[0]}, {type: 'png', fullPage: false, timeout: 1000});
});

test('successful actual Quick Connect geometry captures and writes nothing', async () => {
  const peer = quickConnectPeer({passes: true}); await peer.run();
  assert.equal(peer.effects.length, 0); assert.equal(peer.writes.length, 0); assert.equal(peer.attachments.length, 0);
});

for (const options of [{capture: 'reject'}, {capture: 'stall'}, {attachment: 'reject'}, {attachment: 'stall'}])
  test('Quick Connect diagnostic ' + JSON.stringify(options) + ' keeps the exact geometry cause within bounds', async () => {
    const peer = quickConnectPeer(options); const started = Date.now();
    await assert.rejects(peer.run(), error => error === peer.cause); assert.ok(Date.now() - started < 1800);
    assert.equal(peer.attachments.length, 0); assert.equal(peer.writes.length, options.capture ? 0 : 1);
  });

test('malformed oversized or other-dimension PNG produces no file or attachment', async () => {
  const wrong = Buffer.alloc(24); Buffer.from([137,80,78,71,13,10,26,10]).copy(wrong); wrong.writeUInt32BE(390,16); wrong.writeUInt32BE(844,20);
  for (const image of [null, 'private', Buffer.alloc(0), Buffer.alloc(24), Buffer.alloc(2097153), wrong]) {
    const peer = quickConnectPeer({image: image ?? 'not-buffer'}); await assert.rejects(peer.run(), error => error === peer.cause);
    assert.equal(peer.writes.length, 0); assert.equal(peer.attachments.length, 0);
  }
});
