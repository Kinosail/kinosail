import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
import {runInContext, createContext} from 'node:vm';
import {EventEmitter} from 'node:events';
import test from 'node:test';

// Actual registered conversion case; no browser, auth, media or PNG success is simulated.
// Stop immediately before conversion once the actual function precondition is satisfied.
const source = stripTypeScriptTypes(readFileSync(new URL('../../apps/player/e2e/test-instance-supporter.spec.ts', import.meta.url), 'utf8'))
  .replace(/^import .*;\n/gm, '');
const origin = 'https://localhost:38127';
const stop = new Error('conversion-boundary');
function fixture({available = true, delayed = true, status = 200, brokenAttachment = false, stalledAttachment = false} = {}) {
  const cases = new Map(), effects = [], attachments = [];
  const pw = (title, callback) => cases.set(title, callback);
  for (const name of ['skip', 'use', 'beforeEach']) pw[name] = () => {};
  const expect = value => ({toBeLessThan: maximum => assert.ok(value < maximum), toBe: expected => assert.equal(value, expected)});
  expect.poll = (callback, options) => ({toBe: async expected => {
    effects.push('wait-function'); assert.equal(options.timeout, 10_000);
    for (let attempt = 0; attempt < 30; attempt++) {
      if (await callback() === expected) return;
      await new Promise(resolve => setTimeout(resolve, 2));
    }
    throw new Error('actual function unavailable');
  }});
  const scope = createContext({process: {env: {KINOSAIL_UI_FIXTURE_DIR: '/owned/synthetic'}},
    configureProviderProfile() {}, configureTestInstance() {}, isolateProvider() {},
    login: async () => {effects.push('login');}, providerRoute: async () => {},
    readFile: async () => '<svg/>', writeFile: async () => {throw new Error('no artifact write before readiness');},
    join: (...parts) => parts.join('/'), test: pw, expect, URL, Buffer, setTimeout, clearTimeout,
    window: {}, document: {readyState: 'complete', querySelector: () => ({})}});
  runInContext(source, scope);
  class Page extends EventEmitter {
    async goto(target) {
      effects.push('goto'); assert.equal(target, '/supporter');
      this.emit('response', {url: () => origin + '/static/supporter.js?v=19-htmx4', status: () => status});
      if (available) {
        if (delayed) setTimeout(() => {scope.window.supporterShareFile = () => {};}, 10);
        else scope.window.supporterShareFile = () => {};
      }
    }
    url() {return origin + '/supporter';}
    async evaluate(callback) {
      const text = String(callback);
      if (text.includes('const share =')) {effects.push('convert'); if (typeof scope.window.supporterShareFile !== 'function') throw new Error('premature real-function conversion'); throw stop;}
      if (text.includes('const gallery') || text.includes('supporter-badge')) return 1;
      return runInContext('(' + text + ')()', scope);
    }
  }
  const page = new Page();
  const browser = {newContext: async () => ({newPage: async () => page, close: async () => {}})};
  const info = {project: {name: 'webkit', use: {baseURL: origin}}, attach: async (name, value) => {
    if (brokenAttachment) throw new Error('diagnostic-only'); if (stalledAttachment) return new Promise(() => {}); attachments.push({name, value});
  }};
  const run = () => cases.get('Supporter badge rendering and share conversion remain responsive')({browser}, info);
  return {run, page, effects, attachments};
}
function facts(control) {
  assert.equal(control.attachments.length, 1);
  const row = control.attachments[0]; assert.equal(row.name, 'supporter-share-script-availability');
  assert.equal(row.value.contentType, 'application/json'); assert.ok(Buffer.byteLength(row.value.body) < 2048);
  assert.doesNotMatch(row.value.body, /https?:|synthetic|certificate|v=|private/);
  return JSON.parse(row.value.body);
}
test('actual conversion case waits for a delayed real script function without substituting it', async () => {
  const control = fixture(); await assert.rejects(control.run(), error => error === stop);
  assert.deepEqual(control.effects, ['login', 'goto', 'wait-function', 'convert']);
  const value = facts(control); assert.equal(value.functionType, 'function'); assert.equal(value.responseStatus, 200);
  assert.equal(value.responseObserved, true); assert.equal(value.ownedResponse, true);
  assert.equal(control.page.eventNames().length, 0);
});
for (const status of [404, 200]) test(`unavailable real function with script HTTP${status} stops before conversion and retains diagnostics`, async () => {
  const control = fixture({available: false, status}); await assert.rejects(control.run(), /actual function unavailable/);
  assert.equal(control.effects.includes('convert'), false);
  const value = facts(control); assert.equal(value.functionType, 'undefined'); assert.equal(value.responseStatus, status);
  assert.equal(control.page.eventNames().length, 0);
});
test('diagnostic attachment rejection preserves the original missing-function failure and removes listeners', async () => {
  const control = fixture({available: false, brokenAttachment: true}); await assert.rejects(control.run(), /actual function unavailable/);
  assert.equal(control.effects.includes('convert'), false); assert.equal(control.page.eventNames().length, 0);
});

test('a stalled diagnostic attachment cannot hide missing real function or retain listeners', async () => {
  const control = fixture({available: false, stalledAttachment: true});
  await assert.rejects(control.run(), /actual function unavailable/);
  assert.equal(control.effects.includes('convert'), false); assert.equal(control.page.eventNames().length, 0);
});
