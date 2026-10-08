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
