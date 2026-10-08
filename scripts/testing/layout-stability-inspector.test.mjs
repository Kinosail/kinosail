import {test} from 'node:test';
import assert from 'node:assert/strict';
import {measureInspectorEditing} from './layout-stability-flows.mjs';
import * as flows from './layout-stability-flows.mjs';

// Fast actual response delivery can precede completion of a select action.
// Pending measurements must therefore own response release, rather than race it.
class Editor {
  busy = false; requests = []; pending = Promise.resolve(); calls = []; scroll = 0;
  node = {value: 'en', options: [{value: 'en'}, {value: 'fr'}],
    getBoundingClientRect: () => ({top: -40, bottom: -1})};
  document = {activeElement: null, documentElement: {scrollHeight: 1200},
    querySelector: selector => selector === '#inspector-status' ? {getAttribute: () => this.busy ? 'true' : null} : {disabled: this.busy}};
  pendingGeometryFailure = undefined; refreshes = 0;
  refreshFetchDelay = 0; refreshFetchFailure = undefined; responseReturned = false; requireReady = false; stallFetch = false; contextClosed = false; cancelFetch = undefined;
  response = {actual: 'owned-response-object'}; failure = undefined;
  async route(pattern, callback) {assert.equal(pattern, '**/inspect?*'); this.callback = callback;}
  async request(controlled = false) {
    this.busy = true;
    const response = this.response;
    const request = this.callback({request: () => ({resourceType: () => 'fetch'}),
      fetch: async options => {
        this.calls.push('fetch'); if (this.failure) throw this.failure;
        if (controlled) {
          if (this.stallFetch) return new Promise((_, reject) => {this.cancelFetch = reject;});
          if (this.refreshFetchDelay) await new Promise(resolve => setTimeout(resolve, this.refreshFetchDelay));
          if (this.refreshFetchFailure) {assert.equal(options.timeout, 10000); throw this.refreshFetchFailure;}
          this.responseReturned = true; this.calls.push('response-ready');
        }
        return response;
      },
      abort: async () => {this.calls.push('abort'); this.busy = false;},
      fulfill: async value => {assert.equal(value.response, response); this.calls.push('fulfill'); if (controlled) this.calls.push('controlled-release'); this.busy = false;},
      continue: async () => {throw new Error('unexpected non-fetch');}});
    this.requests.push(request); this.pending = request; request.catch(() => {});
  }
  context() {return {close: async () => {this.contextClosed = true; this.cancelFetch?.(new Error('owned context cancelled'));}};}
  async goto() {await this.request();}
  async reload() {this.node.value = 'en'; await this.request(); await this.pending;}
  async waitForTimeout() {await this.pending;}
  async waitForFunction(callback) {
    if (!callback()) await this.pending;
    assert.equal(callback(), true, 'pending/loaded state must remain observable');
  }
  locator(selector) {
    if (selector === 'select[name="language"]') return {
      focus: async () => {this.document.activeElement = this.node;},
      evaluate: async (callback, argument) => callback(this.node, argument),
      selectOption: async value => {this.refreshes++; this.responseReturned = false; this.node.value = value; await this.request(true); await new Promise(resolve => setTimeout(resolve, 1000));},
    };
    return {isDisabled: async () => this.busy, setInputFiles: async () => {}, focus: async () => {},
      boundingBox: async () => {if (this.busy && this.refreshes && this.requireReady) {assert.equal(this.responseReturned, true, 'real fetch must return before pending geometry'); this.calls.push('pending-capture');} if (this.busy && this.refreshes && this.pendingGeometryFailure) throw this.pendingGeometryFailure; return {x: 0, y: 0, width: 100, height: 50};}};
  }
  keyboard = {press: async () => {this.document.activeElement = {kind: 'next-control'};}};
  async evaluate(callback, argument) {return callback(argument);}
  async evaluateHandle(callback) {const value = callback(); value.dispose = async () => {}; return value;}
}
async function withDocument(editor, action) {
  const keys = ['document', 'scrollY', 'innerHeight', 'scrollTo'];
  const previous = keys.map(key => Object.getOwnPropertyDescriptor(globalThis, key));
  Object.defineProperties(globalThis, {
    document: {configurable: true, value: editor.document},
    scrollY: {configurable: true, get: () => editor.scroll},
    innerHeight: {configurable: true, value: 844},
    scrollTo: {configurable: true, value: value => {editor.scroll = value.top;}},
  });
  try {return await action();}
  finally {for (const [index, key] of keys.entries()) {
    if (previous[index]) Object.defineProperty(globalThis, key, previous[index]); else delete globalThis[key];
  }}
}
test('actual inspector callback holds fast refresh responses through pending measurements', async () => {
  const editor = new Editor(), results = [];
  await withDocument(editor, () => measureInspectorEditing(editor, '/inspect/0123456789abcdef', results, {}));
  assert.equal(results.length, 4);
  assert.ok(results[0].pendingLocked && results[0].loadedEnabled);
  assert.ok(results.slice(1).every(row => row.focusRetained && row.scrollRetained));
  assert.equal(editor.calls.filter(value => value === 'fulfill').length, 7);
  assert.equal(editor.busy, false);
});
test('actual upstream response failure remains the original cause without fabricated loaded state', async () => {
  const editor = new Editor(), results = [], failure = new Error('private upstream marker');
  editor.failure = failure;
  await withDocument(editor, () => assert.rejects(measureInspectorEditing(editor, '/inspect/0123456789abcdef', results, {}), error => error === failure));
  assert.equal(editor.calls.includes('fulfill'), false); assert.deepEqual(results, []);
});

test('pending measurement failure releases the owned actual response and preserves its cause', async () => {
  const editor = new Editor(), failure = new Error('private geometry marker'), probe = {};
  editor.pendingGeometryFailure = failure;
  await withDocument(editor, () => assert.rejects(measureInspectorEditing(editor, '/inspect/0123456789abcdef', [], probe), error => error === failure));
  await Promise.all(editor.requests);
  assert.equal(editor.busy, false);
  assert.equal(editor.calls.filter(value => value === 'fulfill').length, 3);
  assert.equal(probe.geometry.inspectorRefresh.handlerEntered, true);
  assert.equal(probe.geometry.inspectorRefresh.responseReleasedBeforeFailure, false);
  assert.equal(probe.geometry.inspectorRefresh.cleanupReleased, true);
  assert.doesNotMatch(JSON.stringify(probe), /private|https?:|0123456789abcdef/);
});

for (const engine of ['chromium', 'firefox', 'webkit']) test(engine + ' isolates the actual routed editing context', async () => {
  let editing;
  const options = {baseURL: 'https://owned.fixture', serviceWorkers: 'allow'};
  await flows.inspectorEditingContext({newContext: async value => {editing = value; return {};}}, options);
  assert.equal(editing.serviceWorkers, 'block'); assert.equal(editing.ignoreHTTPSErrors, false);
  assert.equal(options.serviceWorkers, 'allow');
  assert.deepEqual(editing.viewport, {width: 390, height: 844});
});

test('missing route entry rejects controlled refresh without fabricating pending measurements', async () => {
  const editor = new Editor(), results = [], probe = {};
  editor.locator = function(selector) {
    if (selector === 'select[name="language"]') return {
      focus: async () => {this.document.activeElement = this.node;},
      evaluate: async (callback, argument) => callback(this.node, argument),
      selectOption: async value => {this.node.value = value; this.busy = false;},
    };
    return Editor.prototype.locator.call(this, selector);
  };
  await withDocument(editor, () => assert.rejects(measureInspectorEditing(editor, '/inspect/0123456789abcdef', results, probe),
    /Controlled inspector refresh was not routed/));
  assert.equal(results.length, 1); assert.equal(editor.calls.filter(value => value === 'fetch').length, 2);
  assert.deepEqual(probe.geometry.inspectorRefresh, {handlerEntered: false, responseReturned: false, responseReleasedBeforeFailure: false, cleanupReleased: true, cleanupJoined: true, cleanupCancelled: false});
});

test('delayed real fetch returns before pending capture and response release', async () => {
  const editor = new Editor(), results = [];
  editor.refreshFetchDelay = 1500; editor.requireReady = true;
  await withDocument(editor, () => measureInspectorEditing(editor, '/inspect/0123456789abcdef', results, {}));
  assert.equal(results.length, 4);
  const steps = editor.calls.filter(value => ['response-ready', 'pending-capture', 'controlled-release'].includes(value));
  assert.deepEqual(steps, Array.from({length: 3}, () => ['response-ready', 'pending-capture', 'controlled-release']).flat());
});
test('failed controlled fetch aborts and joins without pending capture or replacement cause', async () => {
  const editor = new Editor(), failure = new Error('private controlled upstream failure'), probe = {};
  editor.refreshFetchDelay = 1200; editor.refreshFetchFailure = failure; editor.requireReady = true;
  await withDocument(editor, () => assert.rejects(measureInspectorEditing(editor, '/inspect/0123456789abcdef', [], probe), error => error === failure));
  assert.equal(editor.calls.includes('pending-capture'), false);
  assert.equal(editor.calls.filter(value => value === 'abort').length, 1);
  assert.equal(editor.busy, false);
  assert.equal(probe.geometry.inspectorRefresh.cleanupJoined, true);
  assert.equal(probe.geometry.inspectorRefresh.responseReturned, false);
  assert.doesNotMatch(JSON.stringify(probe), /private|https?:|0123456789abcdef/);
});

test('stalled owned handler is cancelled after bounded join without replacing the gate failure', async () => {
  const editor = new Editor(), probe = {};
  editor.stallFetch = true; editor.requireReady = true;
  await withDocument(editor, () => assert.rejects(measureInspectorEditing(editor, '/inspect/0123456789abcdef', [], probe),
    /Controlled inspector response was not ready/));
  assert.equal(editor.contextClosed, true);
  await Promise.allSettled(editor.requests);
  assert.equal(editor.calls.includes('pending-capture'), false);
  assert.equal(probe.geometry.inspectorRefresh.cleanupCancelled, true);
  assert.equal(probe.geometry.inspectorRefresh.cleanupJoined, true);
});
