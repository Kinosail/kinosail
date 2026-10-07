import {test} from "node:test";
import assert from "node:assert/strict";
import {EventEmitter} from "node:events";
import {measureFlows} from "./layout-stability-flows.mjs";

for (const playback of ['autoplay-between-read-and-click', 'paused-needs-gesture', 'rejected', 'stalled']) test(`theater explicit Play preparation ${playback}`, async t => {
  const calls = [], contexts = [], results = [], probe = {};
  const failure = Object.assign(new Error('synthetic Play rejection'), {name: 'NotAllowedError'});
  let gesture = false, plays = 0, pauses = 0, signalPlay, rejectLate;
  const playStarted = new Promise(resolve => {signalPlay = resolve;});
  const media = {
    paused: true, ended: false, readyState: 4, currentTime: 0.07, error: null,
    canPlayType: () => 'probably',
    async play() {
      plays++; assert.equal(gesture, true, 'explicit evaluation or button dispatch retains user gesture');
      signalPlay();
      if (playback === 'stalled') return new Promise((_, reject) => {rejectLate = reject;});
      if (playback === 'rejected') throw failure;
      media.paused = false;
    },
    pause() {pauses++; media.paused = true;},
  };
  class Page extends EventEmitter {
    mainFrame() {return this;}
    url() {return 'https://owned.fixture/watch/0123456789abcdef';}
    async goto() {}
    async evaluate(callback) {
      if (String(callback).includes('libraryMarker')) return {readyState: 'complete'};
      return {documentReady: 'complete', paused: media.paused, ended: false, readyState: 4, errorCode: 0, currentTime: media.currentTime, theaterActive: calls.includes('Theater'), theaterPressed: calls.includes('Theater') ? 'true' : 'false', toolbarHidden: !media.paused};
    }
    locator(selector) {
      if (selector === '.app-header input[name=q]') return {count: async () => 0};
      if (selector === '[data-theater]') return {isVisible: async () => true, click: async () => {calls.push('Theater');}};
      if (selector === '.media-stage') return {boundingBox: async () => ({x: 16, y: 200, width: 358, height: 272})};
      if (selector === '.player-stage-toolbar') return {evaluate: async () => !media.paused, isVisible: async () => true};
      assert.equal(selector, 'video');
      return {evaluate: async callback => {
        // Evaluation uses the pinned Playwright user-gesture seam. Interpose
        // autoplay after the old paused observation, or before explicit Play.
        if (playback === 'autoplay-between-read-and-click' && String(callback).includes('.play()')) media.paused = false;
        gesture = true;
        try {
          const value = await callback(media);
          if (playback === 'autoplay-between-read-and-click' && value === true) media.paused = false;
          return value;
        } finally {gesture = false;}
      }};
    }
    getByRole(role, options) {
      assert.equal(role, 'button'); assert.equal(options.name, 'Play');
      return {first: () => ({click: async () => {
        calls.push('toggle'); gesture = true;
        try {if (media.paused) await media.play(); else media.pause();}
        finally {gesture = false;}
      }})};
    }
    mouse = {move: async (x, y) => {calls.push(['mouse', x, y]);}};
    keyboard = {press: async key => {calls.push(key);}};
    async waitForTimeout(value) {calls.push(['wait', value]);}
  }
  const browser = {newContext: async () => {
    const page = new Page();
    const context = {closed: false, page, newPage: async () => page, close: async () => {context.closed = true;}};
    contexts.push(context); return context;
  }};
  if (playback === 'stalled') {
    t.mock.timers.enable({apis: ['setTimeout']});
    const operation = measureFlows(browser, {baseURL: 'https://owned.fixture'}, '/watch/0123456789abcdef', undefined, results, probe);
    const settled = operation.then(() => ({passed: true}), error => ({error}));
    await playStarted;
    t.mock.timers.tick(30000);
    const value = await Promise.race([settled, new Promise(resolve => setImmediate(() => resolve({unsettled: true})))]);
    assert.equal(value.unsettled, undefined, 'Play must settle as failure within the original 30-second action budget');
    assert.equal(value.error?.name, 'TimeoutError'); assert.equal(value.error?.message, 'Theater Play preparation timed out');
    const unhandled = [], listener = error => {unhandled.push(error);};
    process.on('unhandledRejection', listener);
    try {rejectLate(failure); await new Promise(resolve => setImmediate(resolve)); assert.deepEqual(unhandled, []);}
    finally {process.off('unhandledRejection', listener);}
    assert.equal(results.some(value => value.flow === 'theater-idle-exit'), false);
    assert.equal(calls.includes('Theater'), false);
    assert.equal(calls.some(value => Array.isArray(value) && value[0] === 'wait'), false);
  } else if (playback === 'rejected') {
    await assert.rejects(measureFlows(browser, {baseURL: 'https://owned.fixture'}, '/watch/0123456789abcdef', undefined, results, probe), error => error === failure);
    assert.equal(results.some(value => value.flow === 'theater-idle-exit'), false);
    assert.equal(calls.includes('Theater'), false);
    assert.equal(calls.some(value => Array.isArray(value) && value[0] === 'wait'), false);
  } else {
    const pending = new Set(), schedule = globalThis.setTimeout, cancel = globalThis.clearTimeout;
    let preparationTimers = 0;
    t.mock.method(globalThis, 'setTimeout', (callback, milliseconds, ...args) => {
      const timer = schedule(callback, milliseconds, ...args);
      if (milliseconds === 30000) {pending.add(timer); preparationTimers++;}
      return timer;
    });
    t.mock.method(globalThis, 'clearTimeout', timer => {pending.delete(timer); return cancel(timer);});
    await measureFlows(browser, {baseURL: 'https://owned.fixture'}, '/watch/0123456789abcdef', undefined, results, probe);
    assert.equal(preparationTimers, 1); assert.equal(pending.size, 0);
    assert.equal(results.find(value => value.flow === 'theater-idle-exit').stable, true);
    assert.equal(media.paused, false); assert.equal(pauses, 0);
    assert.deepEqual(calls.filter(value => Array.isArray(value)), [['mouse', 0, 0], ['wait', 2700], ['wait', 100], ['mouse', 31, 215], ['wait', 200]]);
  }
  assert.equal(plays, 1);
  assert.ok(contexts.every(context => context.closed));
  assert.ok(contexts.every(context => context.page.eventNames().length === 0));
});
