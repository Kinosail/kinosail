import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const source = readFileSync(new URL('./static/player-status.js', import.meta.url), 'utf8');

function startup({ready = 0, network = 2, error = null, native = true} = {}) {
  const listeners = new Map(), attrs = new Map(), classes = new Set();
  const classList = {contains: name => classes.has(name), remove: name => classes.delete(name),
    toggle(name, value) {if (value) classes.add(name); else classes.delete(name);}};
  const skeleton = {hidden: false};
  const status = {hidden: false, dataset: {}, classList, closest: () => ({classList}),
    querySelector: () => skeleton, setAttribute: (name, value) => attrs.set(name, value), removeAttribute: name => attrs.delete(name)};
  const player = {tagName: 'VIDEO', readyState: ready, networkState: network, error, currentTime: 0, duration: ready ? 12 : NaN,
    paused: true, dataset: {}, currentSrc: '/media/fixture', buffered: {length: ready ? 1 : 0, end: () => 12},
    hasAttribute: name => native && name === 'data-native-controls', addEventListener: (name, handler) => listeners.set(name, handler)};
  vm.runInNewContext(source, {player, playerStatus: status, playerMessage: {textContent: ''},
    bufferedProgress: {setAttribute() {}}, bufferedAhead: () => ready ? 12 : 0,
    navigator: {userAgent: 'Chromium', platform: 'Linux', maxTouchPoints: 0},
    HTMLMediaElement: {HAVE_NOTHING: 0, HAVE_CURRENT_DATA: 2, HAVE_FUTURE_DATA: 3, NETWORK_EMPTY: 0, NETWORK_IDLE: 1, NETWORK_LOADING: 2},
    appleNativePlayback: false, appleTouch: false, playbackPreparation: undefined, setTimeout, clearTimeout});
  return {player, status, attrs, skeleton, classes, emit: name => listeners.get(name)()};
}

for (const native of [false, true]) {
  test(`pending initial media is announced when loadstart preceded script initialization (native=${native})`, () => {
    const f = startup({native});
    assert.equal(f.status.hidden, false);
    assert.equal(f.attrs.get('aria-busy'), 'true');
    assert.equal(f.status.dataset.state, 'loading');
    assert.equal(f.skeleton.hidden, false);
    assert.equal(f.classes.has('is-busy'), true);
    f.player.readyState = 4;
    f.player.duration = 12;
    f.player.buffered = {length: 1, end: () => 12};
    f.emit('playing'); // A paused media event must not pretend playback started.
    assert.equal(f.attrs.get('aria-busy'), 'true');
    f.player.paused = false;
    f.emit('playing');
    assert.equal(f.status.hidden, true);
    assert.equal(f.attrs.has('aria-busy'), false);
  });
}

for (const state of [{network: 0}, {network: 1}, {ready: 4, network: 1}, {error: {code: 4}}]) {
  test(`initialization does not announce new work for ${JSON.stringify(state)}`, () => {
    const f = startup(state);
    assert.notEqual(f.attrs.get('aria-busy'), 'true');
    assert.equal(f.classes.has('is-busy'), false);
  });
}

test('an ordinary later loadstart still announces loading and a terminal error stops it', () => {
  const f = startup({network: 1});
  f.emit('loadstart');
  assert.equal(f.attrs.get('aria-busy'), 'true');
  f.player.error = {code: 4};
  f.emit('error');
  assert.equal(f.attrs.get('aria-busy'), 'false');
  assert.equal(f.skeleton.hidden, true);
});
