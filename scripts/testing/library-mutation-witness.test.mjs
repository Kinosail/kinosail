import assert from 'node:assert/strict';
import {EventEmitter} from 'node:events';
import {test} from 'node:test';
import {libraryMutation} from './library-mutation-witness.mjs';

const base = 'http://127.0.0.1:38127', id = '0123456789abcdef';
function peer(options = {}) {
  const page = new EventEmitter(), frame = {}, log = [], attachments = [];
  const destination = options.destination ?? '/settings';
  let current = base + (options.current ?? '/settings#profiles');
  const post = {url: () => base + (options.action ?? '/settings/profiles'), method: () => 'POST',
    resourceType: () => 'document', isNavigationRequest: () => true, frame: () => frame};
  const get = {url: () => base + destination, method: () => 'GET',
    resourceType: () => 'document', isNavigationRequest: () => true, frame: () => options.childFrame ? {} : frame,
    redirectedFrom: () => options.wrongChain ? {...post} : post};
  const response = (request, status, location) => ({request: () => request, status: () => status,
    headerValue: async () => {
      if (options.heldHeader) await options.heldHeader;
      if (options.delayedHeader) await new Promise(resolve => setTimeout(resolve, 60));
      log.push('location-ready'); return location;
    }});
  Object.assign(page, {url: () => current, mainFrame: () => frame,
    evaluate: async () => {log.push('evaluate'); return 100;},
    waitForFunction: async (callback, args) => {
      log.push('ready'); assert.equal(args.origin, base); assert.equal(args.path, destination);
      assert.equal(args.previous, 100); assert.equal(current, base + destination);
      const old = Object.getOwnPropertyDescriptors(globalThis);
      try {
        Object.defineProperties(globalThis, {
          location: {configurable:true, value:new URL(current)},
          document: {configurable:true, value:{readyState:'interactive'}},
          performance: {configurable:true, value:{timeOrigin:options.oldDocument ? 100 : options.nonfinite ? NaN : 200,
            getEntriesByType: () => [{type:options.bfcache ? 'back_forward' : 'navigate'}]}}
        });
        assert.equal(callback(args), true, 'fresh committed document required');
      } finally {
        for (const key of ['location', 'document', 'performance']) {
          if (old[key]) Object.defineProperty(globalThis,key,old[key]); else delete globalThis[key];
        }
      }
    }});
  const button = {evaluate: async () => {log.push('form'); return {
    action: options.formAction ?? base + (options.action ?? '/settings/profiles'), method: options.formMethod ?? 'post'};},
    click: async () => {
      log.push('click'); if (options.clickError) throw options.clickError;
      page.emit('request', post);
      if (options.stall) return;
      page.emit('response', response(post, options.postStatus ?? 303, options.location ?? destination));
      if (options.afterResponseError) throw options.afterResponseError;
      const unrelated = {...get, redirectedFrom: () => null};
      page.emit('response', response(unrelated, 200));
      await new Promise(resolve => setTimeout(resolve, 15));
      log.push('redirect'); page.emit('request', get);
      page.emit('response', response(get, options.getStatus ?? 200));
      await new Promise(resolve => setTimeout(resolve, 15));
      current = base + destination; log.push('commit'); page.emit('framenavigated', frame);
    }};
  const info = {attach: async (name, data) => {
    log.push('attach'); if (options.attachError) throw options.attachError;
    assert.equal(name, 'library-mutation.json'); assert.ok(data.body.length <= 16384);
    attachments.push(JSON.parse(data.body.toString()));
  }};
  return {page, button, info, log, attachments, setURL: value => {current = value;}};
}
const descriptor = {kind: 'profile-add'};
function clean(p) {
  for (const event of ['request', 'response', 'requestfailed', 'framenavigated'])
    assert.equal(p.page.listenerCount(event), 0, event);
}
test('real callback ordering requires POST303, its document GET200 and new commit', async () => {
  const p = peer(); await libraryMutation(p.page, base, descriptor, p.button, p.info);
  assert.ok(p.log.indexOf('ready') > p.log.indexOf('commit'));
  assert.deepEqual(p.attachments[0], {version: 1, kind: 'profile-add', postRequests: 1,
    postStatus: 303, redirectedRequests: 1, redirectedStatus: 200, failures: 0,
    documentCommitted: true, documentReady: true, route: 'settings', outcome: 'complete'});
  clean(p);
});
test('delayed Location validation must finish before qualifying the new document', async () => {
  const p = peer({delayedHeader:true}); await libraryMutation(p.page, base, descriptor, p.button, p.info);
  assert.ok(p.log.indexOf('location-ready') >= 0);
  assert.ok(p.log.indexOf('ready') > p.log.indexOf('location-ready'));
  clean(p);
});
test('all five owned forms use their exact POST and public destination', async () => {
  for (const kind of ['profile-save', 'profile-password', 'playlist-add', 'collection-add']) {
    const curation = kind.endsWith('-add'), name = kind === 'playlist-add' ? 'E2E playlist chromium-1760000000000' : 'E2E Collection chromium-1760000000000';
    const action = curation ? '/' + kind.split('-')[0] + '/' + encodeURIComponent(name) + '/' + id :
      '/settings/profiles/' + (kind === 'profile-save' ? 'permissions' : 'password');
    const p = peer({action, current:curation ? '/watch/' + id : '/settings#profiles', destination:curation ? '/watch/' + id : '/settings'});
    await libraryMutation(p.page, base, curation ? {kind,name} : {kind}, p.button, p.info);
    assert.equal(p.attachments[0].outcome, 'complete'); clean(p);
  }
});
test('authority and closed descriptor rejection has no DOM, listener, click or attachment effects', async () => {
  for (const value of [undefined, '', 'http://127.0.0.1:0', 'http://127.0.0.1:80',
    'http://127.0.0.1:65536', 'http://user@127.0.0.1:38127', base + '/settings',
    base + '/?x=1', base + '/#x', 'https://example.com:38127', 'x'.repeat(2049)]) {
    const p = peer(); await assert.rejects(libraryMutation(p.page, value, descriptor, p.button, p.info));
    assert.deepEqual(p.log, []); clean(p);
  }
  for (const value of [null, {}, {kind:'unknown'}, {kind:'profile-add', extra:true},
    {kind:'playlist-add'}, {kind:'playlist-add', name:'../foreign'},
    {kind:'collection-add', name:'x'.repeat(129)}]) {
    const p = peer(); await assert.rejects(libraryMutation(p.page, base, value, p.button, p.info));
    assert.deepEqual(p.log, []); clean(p);
  }
  for (const value of ['http://127.0.0.1:38128/settings', 'https://example.com/settings', base + '/%73ettings']) {
    const p = peer(); p.setURL(value);
    await assert.rejects(libraryMutation(p.page, base, descriptor, p.button, p.info));
    assert.deepEqual(p.log, []); clean(p);
  }
});
test('foreign form action and nonPOST conflict reject before click', async () => {
  for (const options of [{formAction: 'http://127.0.0.1:38128/settings/profiles'},
    {formAction: base + '/settings/profiles?extra=1'}, {formMethod:'get'}]) {
    const p = peer(options); await assert.rejects(libraryMutation(p.page, base, descriptor, p.button, p.info));
    assert.ok(!p.log.includes('click')); assert.equal(p.attachments.length, 0); clean(p);
  }
});
test('rejected transport and stale document never qualify completed mutation', async () => {
  for (const options of [{postStatus:409}, {getStatus:403}, {oldDocument:true}, {nonfinite:true}, {bfcache:true},
    {location:'http://127.0.0.1:38128/settings'}]) {
    const p = peer(options); await assert.rejects(libraryMutation(p.page, base, descriptor, p.button, p.info));
    assert.equal(p.attachments[0].outcome, 'failed'); assert.equal(p.attachments[0].documentReady, false); clean(p);
  }
});
test('original action failure wins over attachment error with joined listener cleanup', async () => {
  const cause = new Error('original action'), p = peer({clickError:cause, attachError:new Error('attachment')});
  await assert.rejects(libraryMutation(p.page, base, descriptor, p.button, p.info), error => error === cause);
  clean(p);
});
test('wrong redirect identity or child-frame document cannot qualify a mutation', {timeout:23000}, async () => {
  for (const options of [{wrongChain:true}, {childFrame:true}]) {
    const p = peer(options); await assert.rejects(libraryMutation(p.page, base, descriptor, p.button, p.info), /mutation deadline/);
    assert.equal(p.attachments[0].redirectedRequests, 0);
    assert.equal(p.attachments[0].documentCommitted, false); clean(p);
  }
});
test('held callback cannot overwrite failure-state attachment after owned listener cleanup', async () => {
  let release;
  const heldHeader = new Promise(resolve => {release = resolve;}), cause = new Error('action after response');
  const p = peer({heldHeader, afterResponseError:cause});
  await assert.rejects(libraryMutation(p.page, base, descriptor, p.button, p.info), error => error === cause);
  const snapshot = JSON.stringify(p.attachments); clean(p);
  release(); await new Promise(resolve => setImmediate(resolve));
  assert.equal(JSON.stringify(p.attachments), snapshot);
  assert.equal(p.attachments[0].outcome, 'failed');
  assert.ok(!p.log.includes('ready'));
});
test('missing response times out without qualifying success', {timeout:12000}, async () => {
  const p = peer({stall:true}); await assert.rejects(libraryMutation(p.page, base, descriptor, p.button, p.info), /mutation deadline/);
  assert.equal(p.attachments[0].postRequests, 1); assert.equal(p.attachments[0].postStatus, null);
  assert.equal(p.attachments[0].outcome, 'failed'); clean(p);
});
